package aiderpolyglot

import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// TestPinIsRealHexSHA proves the pinned sha constant is a real 40-hex commit
// (never a branch/tag), validated by isHexSHA1, and the repo URL is the upstream.
func TestPinIsRealHexSHA(t *testing.T) {
	if !isHexSHA1(PinnedSHA) {
		t.Fatalf("PinnedSHA %q is not a 40-char hex sha1", PinnedSHA)
	}
	if !strings.Contains(RepoURL, "Aider-AI/polyglot-benchmark") {
		t.Fatalf("RepoURL = %q, want the upstream Aider-AI/polyglot-benchmark", RepoURL)
	}
	if strings.HasPrefix(RepoURL, "-") {
		t.Fatal("RepoURL must not begin with '-' (argv flag-smuggling)")
	}
}

// TestIsHexSHA1 proves the validator accepts a 40-hex sha and rejects everything
// else (short, long, uppercase, non-hex, branch-shaped).
func TestIsHexSHA1(t *testing.T) {
	good := "7e0611e77b54e2dea774cdc0aa00cf9f7ed6144f"
	if !isHexSHA1(good) {
		t.Fatalf("isHexSHA1(%q) = false, want true", good)
	}
	for _, bad := range []string{
		"",
		"main",
		"7e0611e7", // too short
		good + "00", // too long
		strings.ToUpper(good), // uppercase
		"7e0611e77b54e2dea774cdc0aa00cf9f7ed6144g", // non-hex 'g'
		"v1.0.0",
		"../etc",
	} {
		if isHexSHA1(bad) {
			t.Fatalf("isHexSHA1(%q) = true, want false", bad)
		}
	}
}

// TestCloneArgsFixedArgv proves cloneArgs builds the fixed --depth 1 sha-pinned
// clone/fetch/checkout argv and contains "--depth", "1", the sha, "--", with NO
// branch/tag. This hermetic argv assertion is the AUTHORITATIVE proof (the live
// clone is never the sole proof).
func TestCloneArgsFixedArgv(t *testing.T) {
	dest := t.TempDir()
	steps, err := cloneArgs(RepoURL, PinnedSHA, dest)
	if err != nil {
		t.Fatalf("cloneArgs: %v", err)
	}
	if len(steps) != 3 {
		t.Fatalf("cloneArgs returned %d steps, want 3 (clone/fetch/checkout)", len(steps))
	}
	joinAll := func() string {
		var b strings.Builder
		for _, s := range steps {
			b.WriteString(strings.Join(s, " "))
			b.WriteString("\n")
		}
		return b.String()
	}
	all := joinAll()
	// Every step must invoke git.
	for _, s := range steps {
		if len(s) == 0 || s[0] != "git" {
			t.Fatalf("step %v does not start with git", s)
		}
	}
	// --depth 1 shallow clone.
	if !strings.Contains(all, "--depth 1") {
		t.Fatalf("argv missing --depth 1:\n%s", all)
	}
	// The exact sha must appear (pinned, never a branch/tag).
	if !strings.Contains(all, PinnedSHA) {
		t.Fatalf("argv missing pinned sha:\n%s", all)
	}
	// A "--" end-of-options terminator must guard the fetch/checkout refspec.
	if !strings.Contains(all, "--") {
		t.Fatalf("argv missing -- terminator:\n%s", all)
	}
	// The dest dir must be the clone target.
	if !strings.Contains(all, dest) {
		t.Fatalf("argv missing dest dir %q:\n%s", dest, all)
	}
	// No branch/tag fetch: never "origin main" / "origin HEAD".
	if strings.Contains(all, "origin main") || strings.Contains(all, "origin HEAD") {
		t.Fatalf("argv fetches a mutable ref, must pin by sha:\n%s", all)
	}
}

// TestCloneArgsFailClosed proves cloneArgs fail-closes on a non-hex sha and a
// traversal dest dir BEFORE producing any argv (T-85-07-01 / T-85-07-02).
func TestCloneArgsFailClosed(t *testing.T) {
	if _, err := cloneArgs(RepoURL, "main", t.TempDir()); err == nil {
		t.Fatal("cloneArgs(sha=main) = nil err, want rejection (not a sha)")
	}
	if _, err := cloneArgs(RepoURL, "deadbeef", t.TempDir()); err == nil {
		t.Fatal("cloneArgs(short sha) = nil err, want rejection")
	}
	if _, err := cloneArgs("-flag-url", PinnedSHA, t.TempDir()); err == nil {
		t.Fatal("cloneArgs(flag-shaped url) = nil err, want rejection")
	}
}

// TestCloneArgsRejectsHostileLeaf proves the dest-leaf guard fails closed on a
// leading-dot leaf (".git", ".ssh") and on a "." / ".." traversal token, instead
// of silently accepting them (WR-03). These dirs are clean per filepath.Clean
// but must never be a clone destination leaf.
func TestCloneArgsRejectsHostileLeaf(t *testing.T) {
	base := t.TempDir()
	for _, leaf := range []string{".git", ".ssh", ".", ".."} {
		// filepath.Join cleans away a "." or ".." leaf, so build the dest dir
		// string directly to exercise the leaf guard on the unclean form too.
		dest := base + string(filepath.Separator) + leaf
		if _, err := cloneArgs(RepoURL, PinnedSHA, dest); err == nil {
			t.Fatalf("cloneArgs(dest leaf %q) = nil err, want rejection", leaf)
		}
	}
}

// TestClonePathUnderCache proves clonePath roots the clone under cacheDir() with
// the aider-polyglot/<sha>/ sub-segment and is sha-validated.
func TestClonePathUnderCache(t *testing.T) {
	t.Setenv("HELIX_CACHE_DIR", t.TempDir())
	p, err := clonePath(PinnedSHA)
	if err != nil {
		t.Fatalf("clonePath: %v", err)
	}
	want := filepath.Join(os.Getenv("HELIX_CACHE_DIR"), "aider-polyglot", PinnedSHA)
	if p != want {
		t.Fatalf("clonePath = %q, want %q", p, want)
	}
	if _, err := clonePath("main"); err == nil {
		t.Fatal("clonePath(main) = nil err, want rejection")
	}
}

// TestFlagNonHermetic proves an exercise whose config carries non_hermetic, or
// whose language is a known network-at-test-time track, is flagged (SC#3). The
// committed rust `leap` fixture carries the marker.
func TestFlagNonHermetic(t *testing.T) {
	rustDir := filepath.Join("fixtures", "rust", "exercises", "practice", "leap")
	ex, err := loadExercise(rustDir, "rust")
	if err != nil {
		t.Fatalf("loadExercise(rust leap): %v", err)
	}
	if !ex.NonHermetic {
		t.Fatal("rust leap fixture should be flagged NonHermetic (SC#3)")
	}
	// The python wordy fixture is hermetic.
	pyDir := filepath.Join("fixtures", "python", "exercises", "practice", "wordy")
	py, err := loadExercise(pyDir, "python")
	if err != nil {
		t.Fatalf("loadExercise(python wordy): %v", err)
	}
	if py.NonHermetic {
		t.Fatal("python wordy fixture should be hermetic, got NonHermetic")
	}
}

// TestLiveCloneAtPinnedSha is the network-gated live leg. It skips offline (a
// quick reachability probe) or unless HELIX_BENCH_NETWORK is opted in. It is
// NEVER the sole proof — TestCloneArgsFixedArgv is authoritative. When it runs,
// it asserts the resolved checkout HEAD == the pinned sha.
func TestLiveCloneAtPinnedSha(t *testing.T) {
	if os.Getenv("HELIX_BENCH_NETWORK") == "" {
		t.Skip("set HELIX_BENCH_NETWORK=1 to run the live pinned-sha clone (network/git gated)")
	}
	// Quick reachability probe: a 3s dial to github:443. Skip cleanly offline.
	conn, err := net.DialTimeout("tcp", "github.com:443", 3*time.Second)
	if err != nil {
		t.Skipf("github unreachable, skipping live clone: %v", err)
	}
	_ = conn.Close()

	dest := filepath.Join(t.TempDir(), "clone")
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	if err := Clone(ctx, PinnedSHA, dest); err != nil {
		t.Fatalf("Clone: %v", err)
	}
	// Assert the checked-out HEAD is exactly the pinned sha.
	got, err := headSHA(dest)
	if err != nil {
		t.Fatalf("headSHA: %v", err)
	}
	if got != PinnedSHA {
		t.Fatalf("resolved HEAD = %q, want pinned %q", got, PinnedSHA)
	}
}
