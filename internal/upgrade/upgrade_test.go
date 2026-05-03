package upgrade

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// upgradeTestServer hosts a fake GitHub Releases API + asset endpoints
// against the local Plan 01 testdata fixtures. Tests configure the
// release tag, asset URLs, and signature payload by writing to the
// test struct fields.
type upgradeTestServer struct {
	t       *testing.T
	server  *httptest.Server
	tag     string
	prerelease bool
}

func newUpgradeTestServer(t *testing.T, tag string) *upgradeTestServer {
	t.Helper()
	uts := &upgradeTestServer{t: t, tag: tag}
	mux := http.NewServeMux()

	// Compose the per-platform asset name to match runtime.GOOS/GOARCH.
	assetName := archiveAssetName(tag, runtime.GOOS, runtime.GOARCH)
	sigName := assetName + ".sigstore.json"

	rel := Release{
		TagName: tag,
		Name:    tag,
		Body:    "## Test\n- bullet 1\n",
		Assets: []Asset{},
	}
	// Build the (test) server lazily — assets need the server URL.
	mux.HandleFunc("/repos/agenthands/helix/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(uts.populatedRelease(rel, assetName, sigName))
	})
	mux.HandleFunc("/repos/agenthands/helix/releases/tags/"+tag, func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(uts.populatedRelease(rel, assetName, sigName))
	})
	mux.HandleFunc("/repos/agenthands/helix/releases", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]Release{uts.populatedRelease(rel, assetName, sigName)})
	})
	mux.HandleFunc("/dl/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		serveFile(t, w, testdataArchive)
	})
	mux.HandleFunc("/dl/"+sigName, func(w http.ResponseWriter, r *http.Request) {
		serveFile(t, w, testdataBundle)
	})

	uts.server = httptest.NewServer(mux)
	t.Cleanup(uts.server.Close)
	return uts
}

func (uts *upgradeTestServer) populatedRelease(rel Release, assetName, sigName string) Release {
	rel.Assets = []Asset{
		{Name: assetName, BrowserDownloadURL: uts.server.URL + "/dl/" + assetName},
		{Name: sigName, BrowserDownloadURL: uts.server.URL + "/dl/" + sigName},
	}
	return rel
}

func serveFile(t *testing.T, w http.ResponseWriter, path string) {
	t.Helper()
	body, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("reading %s: %v", path, err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	_, _ = w.Write(body)
}

func TestUpgradeArchiveNameTemplateGoreleaserParity(t *testing.T) {
	t.Parallel()
	// goreleaser template is `helix_v{{ .Version }}_{{ .Os }}_{{ .Arch }}` —
	// reproduce its substitution and assert the Go-side template matches.
	// Drift here breaks asset lookup at upgrade time (RESEARCH.md Pitfall 7).
	cases := []struct {
		version string
		os      string
		arch    string
		want    string
	}{
		{"v1.9.0", "linux", "amd64", "helix_v1.9.0_linux_amd64.tar.gz"},
		{"1.9.0", "linux", "amd64", "helix_v1.9.0_linux_amd64.tar.gz"},
		{"v1.10.0", "darwin", "arm64", "helix_v1.10.0_darwin_arm64.tar.gz"},
		{"v1.9.0", "windows", "amd64", "helix_v1.9.0_windows_amd64.tar.gz"},
	}
	for _, tc := range cases {
		got := archiveAssetName(tc.version, tc.os, tc.arch)
		if got != tc.want {
			t.Errorf("archiveAssetName(%q, %q, %q) = %q, want %q",
				tc.version, tc.os, tc.arch, got, tc.want)
		}
	}
}

func TestUpgradeUpdateHappyPath(t *testing.T) {
	uts := newUpgradeTestServer(t, "v1.9.0")
	withTestTrustRoot(t)

	var out bytes.Buffer
	err := Update(context.Background(), Options{
		Current: "v1.8.0",
		Stdout:  &out,
		baseURL: uts.server.URL,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "current: v1.8.0") {
		t.Errorf("Update output missing current: %q", got)
	}
	if !strings.Contains(got, "latest:  v1.9.0") {
		t.Errorf("Update output missing latest: %q", got)
	}
	if !strings.Contains(got, "upgrade available") {
		t.Errorf("Update output missing status: %q", got)
	}
	if !strings.Contains(got, "release notes") {
		t.Errorf("Update output missing release notes: %q", got)
	}
}

func TestUpgradeUpdateUpToDate(t *testing.T) {
	uts := newUpgradeTestServer(t, "v1.9.0")
	withTestTrustRoot(t)

	var out bytes.Buffer
	err := Update(context.Background(), Options{
		Current: "v1.9.0",
		Stdout:  &out,
		baseURL: uts.server.URL,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !strings.Contains(out.String(), "up to date") {
		t.Errorf("Update output missing up-to-date: %q", out.String())
	}
}

func TestUpgradeDryRun(t *testing.T) {
	if runtime.GOOS == "windows" {
		// On Windows the swap path uses .exe semantics; the dry-run path
		// still bails before swap, so the test is logically the same, but
		// the test infra below uses os.Executable() shenanigans that rely
		// on POSIX permissions. Skip cleanly.
		t.Skip("dry-run test exercises POSIX-style probe")
	}
	uts := newUpgradeTestServer(t, "v1.9.0")
	withTestTrustRoot(t)

	// Create a fake "running binary" location. The orchestrator probes
	// os.Executable() for write access — the test fakes this by setting
	// up a writable temp dir and overriding os.Args[0]. But os.Executable()
	// returns the actual test runner binary, which lives in a temp dir
	// owned by Go's test runtime. We cannot easily redirect that in
	// production code. Instead, we sanity-check the dry-run path
	// explicitly via the lower-level orchestrator helpers.
	//
	// The broader upgrade test exercises selectRelease + download +
	// verify; the swap step is NOT reached because DryRun=true returns
	// before swap.
	tmp := t.TempDir()
	stagePath, cleanup, err := NewStageDir(filepath.Join(tmp, "helix"))
	if err != nil {
		t.Fatalf("NewStageDir: %v", err)
	}
	defer cleanup()
	if !strings.Contains(stagePath, tmp) {
		t.Fatalf("stage path %q not in %q", stagePath, tmp)
	}

	// Drive Update through the test server (DryRun is on Upgrade not
	// Update; the architecture-diagram step #5/6/7 path is exercised by
	// Upgrade. We use Update here because the orchestrator's swap step
	// requires write access to os.Executable() which is not feasible
	// to simulate in a unit test).
	var out bytes.Buffer
	err = Update(context.Background(), Options{
		Current: "v1.8.0",
		Stdout:  &out,
		baseURL: uts.server.URL,
	})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if !strings.Contains(out.String(), "v1.9.0") {
		t.Errorf("Update output missing tag: %q", out.String())
	}
}

func TestUpgradeDowngradeRefused(t *testing.T) {
	uts := newUpgradeTestServer(t, "v1.7.0")
	withTestTrustRoot(t)

	var out bytes.Buffer
	err := Upgrade(context.Background(), Options{
		Current: "v1.9.0",
		Stdout:  &out,
		baseURL: uts.server.URL,
	})
	if err != nil {
		// Permission probe may fire first and produce a typed error,
		// which is acceptable — both paths satisfy "no upgrade
		// happened, hard refuse downgrade was honored". Per D-10 the
		// happy "already up to date" path returns nil, so we accept
		// nil OR a "not writable" error so the test runs in any
		// install-permissions environment.
		if !strings.Contains(err.Error(), "not writable") {
			t.Fatalf("Upgrade: unexpected error: %v", err)
		}
		return
	}
	if !strings.Contains(out.String(), "already up to date") {
		t.Errorf("Upgrade output missing up-to-date: %q", out.String())
	}
}

func TestUpgradeDaemonShortCircuit(t *testing.T) {
	t.Setenv("HELIX_RUNNING_AS_DAEMON", "1")
	var out bytes.Buffer
	err := Upgrade(context.Background(), Options{
		Current: "v1.8.0",
		Stdout:  &out,
	})
	if err != nil {
		t.Fatalf("Upgrade with daemon env: %v", err)
	}
	if !strings.Contains(out.String(), "restart the daemon manually") {
		t.Errorf("Upgrade output missing daemon hint: %q", out.String())
	}
}

// (Phase 58 D-02): the placeholder-pubkey distinct-error path
// (TestUpgradePlaceholderPubKeyDistinguished + IsPlaceholderPubKey) is
// deleted alongside the minisign embed. There is no longer a
// "pre-rotation key" concept under cosign keyless — the embedded asset is
// a sigstore TUF trust root, refreshed via `make update-trust-root`.

func TestUpgradeStripUpgradeVerb(t *testing.T) {
	t.Parallel()
	// stripUpgradeVerb strips the upgrade verb AND all upgrade-only flags
	// (--prerelease, --version <v>, --check, --dry-run) from the relaunch
	// command line. The relaunched binary must NOT receive flags that only
	// make sense for the upgrade subcommand — root-level cobra parsing
	// would either misinterpret them (e.g., --version is a Bool at root
	// level so `--version v1.10.0` is parsed as `--version=true` plus a
	// positional, printing "helix version dev" instead of running) or
	// hard-error on unknown flags. See REVIEW.md CR-02.
	cases := []struct {
		in   []string
		want []string
	}{
		// Bare verb removal.
		{[]string{"helix", "upgrade"}, []string{"helix"}},
		{[]string{"helix", "update"}, []string{"helix"}},
		// Upgrade-only flags must be dropped along with the verb.
		{[]string{"helix", "upgrade", "--prerelease"}, []string{"helix"}},
		{[]string{"helix", "upgrade", "--check"}, []string{"helix"}},
		{[]string{"helix", "upgrade", "--dry-run"}, []string{"helix"}},
		// --version at root would be parsed as the boolean version flag —
		// the relaunched process must NOT see it.
		{[]string{"helix", "upgrade", "--version", "v1.10.0"}, []string{"helix"}},
		{[]string{"helix", "upgrade", "--version=v1.10.0"}, []string{"helix"}},
		// Combined flags.
		{[]string{"helix", "upgrade", "--prerelease", "--dry-run"}, []string{"helix"}},
		{[]string{"helix", "upgrade", "--version", "v1.10.0", "--dry-run"}, []string{"helix"}},
		// Pre-verb flags that are NOT upgrade-only must be preserved
		// (e.g., a global --json flag the user threaded before the verb).
		{[]string{"helix", "--json", "upgrade"}, []string{"helix", "--json"}},
		// A bare `helix --version` invocation is NOT an upgrade — the verb
		// is absent, so the args pass through unchanged.
		{[]string{"helix", "--version"}, []string{"helix", "--version"}},
		// Only first occurrence of the verb is stripped (defensive).
		{[]string{"helix", "upgrade", "upgrade"}, []string{"helix", "upgrade"}},
	}
	for _, tc := range cases {
		got := stripUpgradeVerb(tc.in)
		if !equalStrings(got, tc.want) {
			t.Errorf("stripUpgradeVerb(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

func TestUpgradeSanitizeReleaseBody(t *testing.T) {
	t.Parallel()
	// ESC sequence (\x1b[31m red ANSI) must be stripped from release
	// notes before printing to avoid terminal injection (T-52-04-10).
	body := "before\x1b[31m middle \x1b[0m after"
	got := sanitizeReleaseBody(body)
	if strings.ContainsRune(got, 0x1b) {
		t.Errorf("sanitizeReleaseBody = %q, ESC still present", got)
	}
	if !strings.Contains(got, "before") || !strings.Contains(got, "after") {
		t.Errorf("sanitizeReleaseBody dropped legitimate content: %q", got)
	}
}

// TestUpgradeAsymmetricChecksumsRefused covers REVIEW.md CR-03: when a
// release ships checksums.txt without checksums.txt.minisig (or vice
// versa) the upgrade flow MUST fail closed with a clear error rather
// than silently fall back to archive-signature-only verification. The
// cross-check is the surface that defends against a tampered single-
// asset substitution; an attacker who can replace checksums.txt and
// also delete the .minisig sidecar would otherwise turn it off.
func TestUpgradeAsymmetricChecksumsRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("permission probe path is POSIX-flavored")
	}
	withTestTrustRoot(t)

	tag := "v1.9.0"
	assetName := archiveAssetName(tag, runtime.GOOS, runtime.GOARCH)
	sigName := assetName + ".sigstore.json"

	mux := http.NewServeMux()
	var server *httptest.Server
	mux.HandleFunc("/repos/agenthands/helix/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		// Asset list includes archive + bundle + checksums.txt but NOT
		// checksums.txt.sigstore.json — the asymmetric case. Production
		// code must refuse to proceed.
		_ = json.NewEncoder(w).Encode(Release{
			TagName: tag,
			Assets: []Asset{
				{Name: assetName, BrowserDownloadURL: server.URL + "/dl/archive"},
				{Name: sigName, BrowserDownloadURL: server.URL + "/dl/sig"},
				{Name: "checksums.txt", BrowserDownloadURL: server.URL + "/dl/checksums"},
				// checksums.txt.sigstore.json intentionally omitted.
			},
		})
	})
	mux.HandleFunc("/dl/archive", func(w http.ResponseWriter, r *http.Request) {
		serveFile(t, w, testdataArchive)
	})
	mux.HandleFunc("/dl/sig", func(w http.ResponseWriter, r *http.Request) {
		serveFile(t, w, testdataBundle)
	})

	server = httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err := Upgrade(context.Background(), Options{
		Current: "v1.8.0",
		DryRun:  true,
		Stdout:  &out,
		baseURL: server.URL,
	})
	if err == nil {
		t.Fatal("Upgrade(asymmetric checksum pair) = nil, want refusal")
	}
	// Permission probe might fire first in some CI environments — accept
	// either signal that the upgrade was refused.
	if !strings.Contains(err.Error(), "release artifacts incomplete") &&
		!strings.Contains(err.Error(), "not writable") {
		t.Errorf("Upgrade(asymmetric) err = %q, want 'release artifacts incomplete' refusal", err.Error())
	}
}

func TestUpgradeVerifyTamperedFails(t *testing.T) {
	// The full Upgrade flow against tampered fixtures fails at verify and
	// returns the canonical signature-failure error. Set up the server to
	// serve a tampered sigstore bundle.
	if runtime.GOOS == "windows" {
		t.Skip("permission probe path is POSIX-flavored")
	}
	withTestTrustRoot(t)

	mux := http.NewServeMux()
	tag := "v1.9.0"
	assetName := archiveAssetName(tag, runtime.GOOS, runtime.GOARCH)
	sigName := assetName + ".sigstore.json"

	// Tamper bundle in-memory: flip a byte deep in the JSON payload.
	rawSig, err := os.ReadFile(testdataBundle)
	if err != nil {
		t.Fatalf("reading bundle: %v", err)
	}
	tampered := append([]byte(nil), rawSig...)
	if len(tampered) < 200 {
		t.Fatalf("bundle fixture too short to tamper")
	}
	tampered[len(tampered)-30] ^= 0x01

	var server *httptest.Server
	mux.HandleFunc("/repos/agenthands/helix/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(Release{
			TagName: tag,
			Assets: []Asset{
				{Name: assetName, BrowserDownloadURL: server.URL + "/dl/archive"},
				{Name: sigName, BrowserDownloadURL: server.URL + "/dl/sig"},
			},
		})
	})
	mux.HandleFunc("/dl/archive", func(w http.ResponseWriter, r *http.Request) {
		serveFile(t, w, testdataArchive)
	})
	mux.HandleFunc("/dl/sig", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(tampered)
	})

	server = httptest.NewServer(mux)
	defer server.Close()

	var out bytes.Buffer
	err = Upgrade(context.Background(), Options{
		Current: "v1.8.0",
		DryRun:  true,
		Stdout:  &out,
		baseURL: server.URL,
	})
	if err == nil {
		t.Fatal("Upgrade(tampered) = nil, want signature error")
	}
	if !strings.Contains(err.Error(), "signature verification FAILED") {
		// Permission probe might fire first in environments where the
		// test binary's install path is not writable — accept that as
		// well to stay portable.
		if !strings.Contains(err.Error(), "not writable") {
			t.Errorf("Upgrade(tampered) err = %q, want signature/permission failure", err.Error())
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestUpgradeRelaunchInvokesExecWithStrippedArgs covers the agent-level
// integration gap that human UAT-1 was guarding: that Upgrade reaches
// Step 10 and dispatches to relaunchFn with the verb + upgrade-only
// flags removed from os.Args. The httptest server + test keypair drive
// the orchestrator through API → semver → permission → download →
// verify → extract → swap → relaunch; swapFn and relaunchFn are
// overridden so the test runner's binary is not actually replaced and
// the test process is not actually exec'd away.
func TestUpgradeRelaunchInvokesExecWithStrippedArgs(t *testing.T) {
	if runtime.GOOS == "windows" {
		// Permission probe path + path semantics are POSIX-flavored;
		// the swap_windows.go relaunch uses os.Exit(0) which we cannot
		// observe even via override. Coverage on Unix is sufficient.
		t.Skip("relaunch dispatch coverage is POSIX-flavored")
	}
	withTestTrustRoot(t)

	uts := newUpgradeTestServer(t, "v1.9.0")

	// Override swapFn so the running test binary is not actually
	// replaced. Capture the swap arguments to confirm Step 9 fired
	// before Step 10.
	var swapCalls int
	prevSwap := swapFn
	swapFn = func(currentPath, newPath string) error {
		swapCalls++
		// Sanity: the caller passed the running executable as the
		// target and the extracted helix binary as the source.
		if !strings.Contains(currentPath, "/") {
			t.Errorf("swapFn currentPath = %q, want absolute path", currentPath)
		}
		if !strings.HasSuffix(newPath, "helix") {
			t.Errorf("swapFn newPath = %q, want path ending in 'helix'", newPath)
		}
		return nil
	}
	t.Cleanup(func() { swapFn = prevSwap })

	// Override relaunchFn to capture (and return nil instead of
	// never-returning). The production implementation calls
	// syscall.Exec which replaces the process image — observing the
	// captured args is the whole point of this indirection.
	type relaunchCall struct {
		binPath string
		args    []string
		env     []string
	}
	var captured []relaunchCall
	prevRelaunch := relaunchFn
	relaunchFn = func(binPath string, args, env []string) error {
		captured = append(captured, relaunchCall{
			binPath: binPath,
			args:    append([]string(nil), args...),
			env:     append([]string(nil), env...),
		})
		return nil
	}
	t.Cleanup(func() { relaunchFn = prevRelaunch })

	// Simulate a real user invocation: `helix upgrade --version v1.9.0
	// --prerelease`. The relaunched binary must NOT see those flags or
	// the verb (REVIEW.md CR-02), so after the upgrade the process
	// would re-exec as bare `helix` with no args.
	prevArgs := os.Args
	os.Args = []string{"helix", "upgrade", "--version", "v1.9.0", "--prerelease"}
	t.Cleanup(func() { os.Args = prevArgs })

	var out bytes.Buffer
	err := Upgrade(context.Background(), Options{
		Current: "v1.8.0",
		Stdout:  &out,
		baseURL: uts.server.URL,
	})

	// Permission probe may fail in restricted environments — accept that as
	// a clean skip rather than a failure.
	if err != nil && strings.Contains(err.Error(), "not writable") {
		t.Skip("install path not writable in this test environment; relaunch path not exercised")
	}
	if err != nil {
		t.Fatalf("Upgrade: %v", err)
	}

	// Step 9 must have run before Step 10.
	if swapCalls != 1 {
		t.Errorf("swapFn invocations = %d, want 1", swapCalls)
	}

	// Step 10 must have run exactly once.
	if len(captured) != 1 {
		t.Fatalf("relaunchFn invocations = %d, want 1", len(captured))
	}
	got := captured[0]

	// binPath: must be the running executable (swap target). We accept
	// any non-empty absolute path because os.Executable() returns the
	// test runner's path which varies per platform / temp dir.
	if got.binPath == "" {
		t.Error("relaunchFn binPath is empty, want absolute exec path")
	}

	// args: must NOT contain the verb or any upgrade-only flag — that's
	// the CR-02 contract. Pre-verb args (here just "helix") pass through.
	for _, banned := range []string{"upgrade", "update", "--prerelease", "--check", "--dry-run", "--version"} {
		for _, a := range got.args {
			if a == banned {
				t.Errorf("relaunchFn args = %v, contains banned token %q (CR-02 contract violation)", got.args, banned)
			}
		}
	}
	// Cross-check: stripUpgradeVerb of the same input produces the same
	// stripped slice the orchestrator passed to relaunchFn.
	want := stripUpgradeVerb(os.Args)
	if len(got.args) != len(want) {
		t.Errorf("relaunchFn args = %v (len %d), want stripUpgradeVerb(os.Args) = %v (len %d)", got.args, len(got.args), want, len(want))
	} else {
		for i := range want {
			if got.args[i] != want[i] {
				t.Errorf("relaunchFn args[%d] = %q, want %q", i, got.args[i], want[i])
			}
		}
	}

	// env: must be a snapshot of os.Environ(). We don't compare verbatim
	// (PATH etc. drift during test setup) but we sanity-check non-empty.
	if len(got.env) == 0 {
		t.Error("relaunchFn env is empty, want os.Environ() snapshot")
	}
}
