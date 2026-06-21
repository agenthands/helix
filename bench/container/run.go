package container

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// errBadMount is returned when a run mount source or destination is not a
// well-formed absolute path. It fail-closes the same argv flag-smuggling /
// path-escape vectors as errBadRepo: a path beginning with '-' (which
// docker/podman would parse as a flag), a non-absolute path, a path carrying
// ".." (escape), or a path containing ':' (which would corrupt the "-v
// <src>:<dst>" pair) can never reach the engine argv (T-85-02-01).
var errBadMount = errors.New("bench/container: mount paths must be clean absolute paths without '..' or ':'")

// isValidMountPath reports whether p is a conservative, clean, absolute path
// safe to place in a "-v <src>:<dst>" argv pair. Mirrors isValidRepo's explicit,
// total, no-regexp fail-close discipline:
//   - non-empty
//   - absolute (filepath.IsAbs) — a relative or '-'-leading value is rejected,
//     closing the flag-smuggling vector (a leading '-' is never absolute)
//   - already-clean (filepath.Clean(p) == p) — rejects "..", trailing-slash and
//     redundant-separator escapes post-canonicalization
//   - contains no ':' — the "-v" pair separator; a ':' in a segment would split
//     the host/container halves and smuggle an extra mount spec
//
// Cleanliness is checked against the SLASH form (ToSlash) so the check is
// host-OS-independent: bench mount specs are container paths, always '/'-rooted.
func isValidMountPath(p string) bool {
	if p == "" {
		return false
	}
	if strings.ContainsRune(p, ':') {
		return false
	}
	// Container/host mount specs are always '/'-rooted; normalize so the
	// IsAbs/Clean checks behave identically on windows cross-compiles.
	s := filepath.ToSlash(p)
	if !strings.HasPrefix(s, "/") {
		return false
	}
	if filepath.ToSlash(filepath.Clean(s)) != s {
		return false
	}
	return true
}

// isValidImageRef reports whether image is a digest-pinned reference
// "<repo>@sha256:<64-hex>" whose repo half passes isValidRepo (rejecting a
// leading '-' flag-smuggling vector) and whose digest half passes isHexSHA256.
// A bare repo with no "@sha256:" pin is ALSO accepted (isValidRepo) so callers
// that already resolved a digest-pinned ImageRef.Repo can pass either form; what
// is rejected is a tag-shaped (":latest") or flag-smuggling ("-x/y") value. This
// mirrors the pullArgs fail-close discipline but spans the full pinned-image
// alphabet, which isValidRepo alone (no '@') cannot express (T-85-02-01).
func isValidImageRef(image string) bool {
	if at := strings.Index(image, "@sha256:"); at >= 0 {
		repo := image[:at]
		digest := image[at+len("@sha256:"):]
		return isValidRepo(repo) && isHexSHA256(digest)
	}
	return isValidRepo(image)
}

// runArgs builds the FIXED argv for a hermetic container run:
//
//	["run", "--rm", ("--network=none")?, ("-v" "<src>:<dst>")*, "--", image, cmdArgs...]
//
// It fail-closes via isValidImageRef (image, rejects leading '-' / tag-shaped
// refs) and isValidMountPath (each mount source AND destination) BEFORE any argv is
// produced, so a crafted image or mount value can never reach the engine
// (T-85-02-01). The "--" terminator ends docker/podman option parsing so the
// image and cmdArgs can never be smuggled as flags (mirrors pullArgs, engine.go).
// Mounts are emitted in sorted-by-source order so the argv is byte-stable for the
// golden assertion. "--network=none" is present IFF netNone — that flag presence
// is exactly the network-isolation policy (T-85-02-02 / SC#3), not always-on.
// NO docker Go SDK is used (the make verify-no-docker-sdk gate enforces it).
func runArgs(image string, cmdArgs []string, mounts map[string]string, netNone bool) ([]string, error) {
	if !isValidImageRef(image) {
		return nil, errBadRepo
	}
	// Validate every mount end before producing any argv (fail-close, never
	// partial). Collect sorted sources for deterministic emission.
	srcs := make([]string, 0, len(mounts))
	for src, dst := range mounts {
		if !isValidMountPath(src) || !isValidMountPath(dst) {
			return nil, errBadMount
		}
		srcs = append(srcs, src)
	}
	sort.Strings(srcs)

	args := make([]string, 0, 4+2*len(srcs)+1+len(cmdArgs))
	args = append(args, "run", "--rm")
	if netNone {
		args = append(args, "--network=none")
	}
	for _, src := range srcs {
		args = append(args, "-v", src+":"+mounts[src])
	}
	// "--" terminates option parsing so image/cmdArgs can never be parsed as
	// flags even if engine flag-parsing changes (belt-and-suspenders with the
	// isValidRepo leading-'-' rejection above).
	args = append(args, "--", image)
	args = append(args, cmdArgs...)
	return args, nil
}

// Run executes a hermetic container run via the resolved engine, mirroring
// PullByDigest EXACTLY: it builds the fixed argv with runArgs (returning that
// error verbatim — fail-close before crossing os/exec), then runs the engine
// with a strict env allowlist (PATH/HOME/HELIX_CACHE_DIR only, never the full
// parent env — T-85-02-03) and process-group ownership (procGroupAttr) so a
// context cancel can group-kill the engine and any descendants. NO shell, NO
// docker Go SDK (T-85-02-04 / SC#1).
func (e *Engine) Run(ctx context.Context, image string, cmdArgs []string, mounts map[string]string, netNone bool) error {
	args, err := runArgs(image, cmdArgs, mounts, netNone)
	if err != nil {
		return err
	}
	// runShim is the hermetic test seam: when set it receives the exact argv that
	// would cross os/exec, proving the boundary argv without a live engine.
	if e.runShim != nil {
		return e.runShim(args)
	}
	cmd := exec.CommandContext(ctx, e.bin, args...)
	cmd.SysProcAttr = procGroupAttr()
	cmd.Env = allowlistEnv()
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("bench/container: %s run %s: %w", e.bin, image, err)
	}
	return nil
}
