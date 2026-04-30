package upgrade

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
)

// archiveNameTemplate is the printf-format reproducing
// `.goreleaser.yaml`'s `archives[0].name_template`. Drift between this
// constant and the goreleaser config produces a "release asset not
// found" failure at upgrade time (RESEARCH.md Pitfall 7). The Go-side
// constant and the YAML config MUST stay in lockstep — see the parity
// test in upgrade_test.go.
//
// Goreleaser template: `helix_v{{ .Version }}_{{ .Os }}_{{ .Arch }}`
// rendered against goreleaser's own (Os, Arch, Version) substitutions.
const archiveNameTemplate = "helix_v%s_%s_%s.tar.gz"

// archiveAssetName returns the per-platform archive filename for the
// given version, OS, and arch. version is expected without the leading
// `v` (the template already includes it).
func archiveAssetName(version, goos, goarch string) string {
	v := strings.TrimPrefix(version, "v")
	return fmt.Sprintf(archiveNameTemplate, v, goos, goarch)
}

// Options drives the Upgrade and Update entrypoints. All fields are
// optional except Current, which the cobra wiring fills with
// cli.CurrentVersion() so the version-compare branch can run.
type Options struct {
	// Prerelease widens the release search to include `-rc*`, `-beta*`,
	// and `-alpha*` tags. Default false (D-11 stable-only).
	Prerelease bool

	// Version pins the upgrade target to a specific tag (e.g. "v1.10.0").
	// Empty means "use Prerelease to pick latest". --version always
	// wins over --prerelease (D-12 precedence rule).
	Version string

	// DryRun runs the full flow up to (but not including) the swap +
	// relaunch step. The stage dir is cleaned up; nothing on disk
	// outside the stage dir is modified.
	DryRun bool

	// Current is the running binary's version, used by the downgrade-
	// refuse check (D-10). Caller passes cli.CurrentVersion(); empty
	// is treated as "dev" and any latest tag wins.
	Current string

	// Stdout receives user-facing status messages. nil means os.Stdout.
	Stdout io.Writer

	// baseURL overrides the GitHub API base for tests. Empty in
	// production code.
	baseURL string
}

// Update runs the read-only check: fetches the latest (or prerelease,
// per Options) release, prints `current/latest/status` lines plus the
// release-notes body, and returns. Mutates nothing on disk and never
// makes network calls beyond the API check.
func Update(ctx context.Context, opts Options) error {
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}
	rel, err := selectRelease(ctx, opts)
	if err != nil {
		return err
	}
	current := opts.Current
	if current == "" {
		current = "dev"
	}
	fmt.Fprintf(out, "current: %s\n", current)
	fmt.Fprintf(out, "latest:  %s\n", rel.TagName)
	if IsDowngrade(current, rel.TagName) {
		fmt.Fprintln(out, "status:  up to date")
		return nil
	}
	fmt.Fprintln(out, "status:  upgrade available")
	if rel.Body != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "--- release notes ---")
		fmt.Fprintln(out, sanitizeReleaseBody(rel.Body))
	}
	return nil
}

// Upgrade runs the install flow per the system-architecture diagram in
// RESEARCH.md lines 162-250: daemon-detect → permission probe → API
// fetch → semver compare → download → minisign verify → extract →
// (DryRun bail) → swap → relaunch.
//
// The function returns nil on `up to date` short-circuit, nil after a
// successful relaunch (which actually never returns on Unix because
// syscall.Exec replaces the process image), and a typed error on any
// failure. The relaunch path on Windows calls os.Exit(0) which also
// does not return.
func Upgrade(ctx context.Context, opts Options) error {
	out := opts.Stdout
	if out == nil {
		out = os.Stdout
	}

	// Step 1: daemon-detect short-circuit (D-08).
	if RunningInDaemon() {
		fmt.Fprintln(out, "helix is running as a daemon child process; restart the daemon manually after upgrading from a non-daemon shell")
		return nil
	}

	// Step 2: permission probe (D-09) — BEFORE network I/O.
	exec, err := os.Executable()
	if err != nil {
		return serr.Wrap(serr.Internal, "resolving running executable", err)
	}
	// Resolve symlinks so we probe the real install dir, not a symlink dir.
	if resolved, err := filepath.EvalSymlinks(exec); err == nil {
		exec = resolved
	}
	if err := ProbeWritable(exec); err != nil {
		fmt.Fprintln(out, SudoHint(exec, sudoHintArgs(opts)))
		return serr.New(serr.Unsupported, "install path not writable").
			WithDetail(exec)
	}

	// Step 3: GitHub API fetch.
	rel, err := selectRelease(ctx, opts)
	if err != nil {
		return err
	}

	// Step 4: semver compare (D-10).
	current := opts.Current
	if current == "" {
		current = "dev"
	}
	if current != "dev" && IsDowngrade(current, rel.TagName) {
		fmt.Fprintf(out, "helix is already up to date (current %s, latest %s)\n", current, rel.TagName)
		return nil
	}

	// Step 5: stage dir + downloads.
	stage, cleanup, err := NewStageDir(exec)
	if err != nil {
		return err
	}
	// Cleanup runs on success and on most failure paths. Verification
	// failure deliberately skips cleanup (postmortem inspection per
	// VALIDATION.md State row) — see the verifyKept flag below.
	verifyKept := false
	defer func() {
		if !verifyKept {
			cleanup()
		}
	}()

	archiveName := archiveAssetName(rel.TagName, runtime.GOOS, runtime.GOARCH)
	sigName := archiveName + ".minisig"

	archiveAsset := rel.FindAsset(archiveName)
	if archiveAsset == nil {
		return serr.New(serr.NotFound, "no release archive for current platform").
			WithDetail(fmt.Sprintf("looking for %s in tag %s", archiveName, rel.TagName))
	}
	sigAsset := rel.FindAsset(sigName)
	if sigAsset == nil {
		return serr.New(serr.NotFound, "no archive signature for current platform").
			WithDetail("looking for " + sigName)
	}
	checksumsAsset := rel.FindAsset("checksums.txt")
	checksumsSigAsset := rel.FindAsset("checksums.txt.minisig")

	stageArchive := filepath.Join(stage, archiveName)
	stageSig := filepath.Join(stage, sigName)
	if err := downloadFile(ctx, archiveAsset.BrowserDownloadURL, stageArchive); err != nil {
		return err
	}
	if err := downloadFile(ctx, sigAsset.BrowserDownloadURL, stageSig); err != nil {
		return err
	}

	// Defense-in-depth: if checksums.txt + sig are present, verify them
	// and cross-check the archive sha256. Phase 51 51-05 signs both
	// files; this triple-check matches the INSTALL.md ceremony.
	if checksumsAsset != nil && checksumsSigAsset != nil {
		stageChecksums := filepath.Join(stage, "checksums.txt")
		stageChecksumsSig := filepath.Join(stage, "checksums.txt.minisig")
		if err := downloadFile(ctx, checksumsAsset.BrowserDownloadURL, stageChecksums); err != nil {
			return err
		}
		if err := downloadFile(ctx, checksumsSigAsset.BrowserDownloadURL, stageChecksumsSig); err != nil {
			return err
		}
		if err := VerifyArchive(stageChecksums, stageChecksumsSig); err != nil {
			verifyKept = true
			return err
		}
		if err := crossCheckSha256(stageArchive, stageChecksums, archiveName); err != nil {
			verifyKept = true
			return err
		}
	}

	// Step 6: minisign verify the archive itself.
	if err := VerifyArchive(stageArchive, stageSig); err != nil {
		verifyKept = true
		return err
	}

	// Step 7: extract.
	extracted := filepath.Join(stage, "extracted")
	if err := extractTarGz(stageArchive, extracted); err != nil {
		return err
	}
	newBin := filepath.Join(extracted, "helix")
	if _, err := os.Stat(newBin); err != nil {
		return serr.Wrap(serr.NotFound, "extracted archive missing helix binary", err)
	}

	// Step 8: --dry-run bails before swap (D-12).
	if opts.DryRun {
		fmt.Fprintf(out, "dry-run: downloaded + verified %s; skipping swap\n", rel.TagName)
		return nil
	}

	// Step 9: atomic swap.
	if err := os.Chmod(newBin, 0o755); err != nil {
		// Non-fatal: extraction may have set mode 0o755 already; some
		// filesystems disallow chmod. Continue.
		_ = err
	}
	if err := swap(exec, newBin); err != nil {
		return serr.Wrap(serr.Internal, "atomic swap", err)
	}
	fmt.Fprintf(out, "upgraded helix to %s\n", rel.TagName)

	// Step 10: relaunch with same args minus the "upgrade" verb. On
	// Unix syscall.Exec never returns on success; on Windows
	// relaunch calls os.Exit(0) which also doesn't return.
	relaunchArgs := stripUpgradeVerb(os.Args)
	if err := relaunch(exec, relaunchArgs, os.Environ()); err != nil {
		return serr.Wrap(serr.Internal, "relaunch", err)
	}
	return nil
}

// selectRelease returns the Release tag the upgrade flow targets,
// applying the `--version` precedence rule (D-12: --version wins over
// --prerelease).
func selectRelease(ctx context.Context, opts Options) (*Release, error) {
	base := opts.baseURL
	if base == "" {
		base = defaultAPIBase
	}
	if opts.Version != "" {
		return fetchByTagFrom(ctx, base, opts.Version)
	}
	return fetchReleaseInfoFrom(ctx, base, opts.Prerelease)
}

// stripUpgradeVerb returns args with the first "upgrade" or "update"
// argument removed so the relaunched binary doesn't immediately try to
// upgrade itself again. Preserves the binary path at args[0].
func stripUpgradeVerb(args []string) []string {
	out := make([]string, 0, len(args))
	stripped := false
	for _, a := range args {
		if !stripped && (a == "upgrade" || a == "update") {
			stripped = true
			continue
		}
		out = append(out, a)
	}
	return out
}

// sudoHintArgs reconstructs the user's original flag set so the
// SudoHint message can echo it back. Strips os.Args[0] (program name)
// and the leading "upgrade" verb.
func sudoHintArgs(opts Options) []string {
	args := []string{}
	if opts.Prerelease {
		args = append(args, "--prerelease")
	}
	if opts.Version != "" {
		args = append(args, "--version", opts.Version)
	}
	if opts.DryRun {
		args = append(args, "--dry-run")
	}
	return args
}

// crossCheckSha256 verifies that archivePath's sha256 matches the entry
// for archiveName inside checksumsPath. The format mirrors goreleaser's
// `checksums.txt` (one `<sha256>  <name>\n` per line).
func crossCheckSha256(archivePath, checksumsPath, archiveName string) error {
	want, err := sha256OfFile(archivePath)
	if err != nil {
		return serr.Wrap(serr.Internal, "hashing archive", err)
	}
	body, err := os.ReadFile(checksumsPath)
	if err != nil {
		return serr.Wrap(serr.Internal, "reading checksums.txt", err)
	}
	for _, line := range strings.Split(string(body), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) != 2 {
			continue
		}
		if fields[1] == archiveName {
			if !strings.EqualFold(fields[0], want) {
				return serr.New(serr.Internal, "archive sha256 mismatch against checksums.txt").
					WithDetail(fmt.Sprintf("entry=%s want=%s got=%s", archiveName, fields[0], want))
			}
			return nil
		}
	}
	return serr.New(serr.NotFound, "archive name not found in checksums.txt").
		WithDetail("entry=" + archiveName)
}

// sha256OfFile returns the hex-encoded sha256 of path's contents.
func sha256OfFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sanitizeReleaseBody strips ANSI escape sequences from release-notes
// markdown before printing. RESEARCH.md Security Domain calls this out
// as a terminal-injection mitigation (T-52-04-10).
func sanitizeReleaseBody(body string) string {
	// ANSI escape sequences start with ESC (0x1b). Remove the ESC byte
	// itself; the trailing CSI/SGR bytes are harmless plain text once
	// the escape lead-in is gone.
	return strings.Map(func(r rune) rune {
		if r == 0x1b {
			return -1
		}
		return r
	}, body)
}
