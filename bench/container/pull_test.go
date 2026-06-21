package container

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// a valid 64-char lowercase hex sha256 used across the hermetic pull tests.
const testDigest = "abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789"

const testRepo = "ghcr.io/agenthands/helix-bench"

// TestVerifyThenPullVerifiesBeforeCacheFill (HERMETIC) is the sibling that
// proves the verify-before-fetch decision branch without a live registry
// (Pitfall 1). It asserts:
//   - verifyFn runs BEFORE the puller (pullFn),
//   - a verifyFn error means pullFn is NEVER called,
//   - and a verify failure leaves NO cache dir (no bytes published; Pitfall 3).
func TestVerifyThenPullVerifiesBeforeCacheFill(t *testing.T) {
	// Isolate the cache root so we can assert nothing was published.
	cacheRoot := t.TempDir()
	t.Setenv(cacheDirEnv, cacheRoot)

	var order []string
	verifyErr := errors.New("signature verification FAILED")

	_, _, err := VerifyThenPull(context.Background(), testRepo, testDigest,
		withHostArch("amd64"),
		withArchFn(func(_ context.Context, _ string) (string, error) {
			order = append(order, "arch")
			return "amd64", nil
		}),
		withFetchMetaFn(func(_ context.Context, _ string) ([]byte, []byte, error) {
			order = append(order, "fetchMeta")
			return []byte("manifest"), []byte("bundle"), nil
		}),
		withVerifyFn(func(_, _ []byte) error {
			order = append(order, "verify")
			return verifyErr // force a verify failure
		}),
		withPullFn(func(_ context.Context, _, _ string) error {
			order = append(order, "pull")
			return nil
		}),
	)

	if err == nil {
		t.Fatalf("VerifyThenPull with failing verify = nil err, want verify failure")
	}
	if err.Error() != verifyErr.Error() {
		t.Fatalf("VerifyThenPull err = %q, want canonical %q", err.Error(), verifyErr.Error())
	}

	// Ordering: arch → fetchMeta → verify, and pull NEVER runs.
	want := []string{"arch", "fetchMeta", "verify"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v (pull must not run on verify failure)", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v", order, want)
		}
	}

	// No cache dir published: bench-images/<sha>/ must not exist.
	imgDir := filepath.Join(cacheRoot, imagesSubdir, testDigest)
	if _, statErr := os.Stat(imgDir); !os.IsNotExist(statErr) {
		t.Fatalf("verify failure published a cache dir %s (statErr=%v); want none (Pitfall 3)", imgDir, statErr)
	}
}

// TestVerifyThenPullOrderOnSuccess (HERMETIC) confirms that on the happy path
// verify still runs strictly before pull, and the cache dir IS published with
// the sentinel marker.
func TestVerifyThenPullOrderOnSuccess(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv(cacheDirEnv, cacheRoot)

	var order []string
	hit, dir, err := VerifyThenPull(context.Background(), testRepo, testDigest,
		withHostArch("amd64"),
		withArchFn(func(_ context.Context, _ string) (string, error) {
			order = append(order, "arch")
			return "amd64", nil
		}),
		withFetchMetaFn(func(_ context.Context, _ string) ([]byte, []byte, error) {
			order = append(order, "fetchMeta")
			return []byte("manifest"), []byte("bundle"), nil
		}),
		withVerifyFn(func(_, _ []byte) error {
			order = append(order, "verify")
			return nil
		}),
		withPullFn(func(_ context.Context, _, stageDir string) error {
			order = append(order, "pull")
			// Write a byte into the staging dir to mimic a real layer export.
			return os.WriteFile(filepath.Join(stageDir, "layer.tar"), []byte("x"), 0o644)
		}),
	)
	if err != nil {
		t.Fatalf("VerifyThenPull(success) err = %v, want nil", err)
	}
	if hit {
		t.Fatalf("VerifyThenPull(cold) hit = true, want false")
	}
	want := []string{"arch", "fetchMeta", "verify", "pull"}
	if len(order) != len(want) {
		t.Fatalf("call order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("call order = %v, want %v (verify must precede pull)", order, want)
		}
	}
	// The published dir carries the sentinel marker.
	if _, statErr := os.Stat(filepath.Join(dir, cacheOKMarker)); statErr != nil {
		t.Fatalf("published cache dir missing sentinel marker: %v", statErr)
	}
}

// TestVerifyThenPullArchGateRefusesMismatch (HERMETIC): manifest arch amd64,
// host arm64, BENCH_ARCH_MISMATCH_OK unset → refusal BEFORE any verify or fetch.
func TestVerifyThenPullArchGateRefusesMismatch(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv(cacheDirEnv, cacheRoot)
	// Ensure the override is not set in the test environment.
	t.Setenv(archMismatchEnv, "")

	var touched []string
	_, _, err := VerifyThenPull(context.Background(), testRepo, testDigest,
		withHostArch("arm64"),
		withArchFn(func(_ context.Context, _ string) (string, error) {
			touched = append(touched, "arch")
			return "amd64", nil
		}),
		withFetchMetaFn(func(_ context.Context, _ string) ([]byte, []byte, error) {
			touched = append(touched, "fetchMeta")
			return nil, nil, nil
		}),
		withVerifyFn(func(_, _ []byte) error {
			touched = append(touched, "verify")
			return nil
		}),
		withPullFn(func(_ context.Context, _, _ string) error {
			touched = append(touched, "pull")
			return nil
		}),
	)
	if err == nil {
		t.Fatalf("VerifyThenPull(arch mismatch) = nil, want ArchGate refusal")
	}
	// Only the arch inspect ran; fetch/verify/pull never did.
	if len(touched) != 1 || touched[0] != "arch" {
		t.Fatalf("on arch mismatch, touched = %v; want only [arch] (no verify/fetch/pull)", touched)
	}
	// No cache dir published.
	imgDir := filepath.Join(cacheRoot, imagesSubdir, testDigest)
	if _, statErr := os.Stat(imgDir); !os.IsNotExist(statErr) {
		t.Fatalf("arch mismatch published a cache dir; want none")
	}
}

// TestVerifyThenPullArchMismatchOverride (HERMETIC): with BENCH_ARCH_MISMATCH_OK=1
// the mismatch is tolerated and the pipeline proceeds to verify+pull.
func TestVerifyThenPullArchMismatchOverride(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv(cacheDirEnv, cacheRoot)
	t.Setenv(archMismatchEnv, "1")

	var order []string
	_, _, err := VerifyThenPull(context.Background(), testRepo, testDigest,
		withHostArch("arm64"),
		withArchFn(func(_ context.Context, _ string) (string, error) {
			order = append(order, "arch")
			return "amd64", nil
		}),
		withFetchMetaFn(func(_ context.Context, _ string) ([]byte, []byte, error) {
			order = append(order, "fetchMeta")
			return []byte("m"), []byte("b"), nil
		}),
		withVerifyFn(func(_, _ []byte) error {
			order = append(order, "verify")
			return nil
		}),
		withPullFn(func(_ context.Context, _, _ string) error {
			order = append(order, "pull")
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("VerifyThenPull(override) err = %v, want nil", err)
	}
	want := []string{"arch", "fetchMeta", "verify", "pull"}
	if len(order) != len(want) {
		t.Fatalf("override call order = %v, want %v", order, want)
	}
}

// TestVerifyThenPullCacheHitSkipsVerifyAndPull (HERMETIC): pre-create the cache
// sentinel → VerifyThenPull returns hit without re-verifying or re-fetching.
// (The arch inspect still runs before Ensure; that is a daemon-free HEAD-only
// round-trip and does not touch the network for layers — the invariant under
// test is that verify/fetchMeta/pull are skipped on a hit.)
func TestVerifyThenPullCacheHitSkipsVerifyAndPull(t *testing.T) {
	cacheRoot := t.TempDir()
	t.Setenv(cacheDirEnv, cacheRoot)

	// Pre-populate the cache dir + sentinel for testDigest.
	imgDir := filepath.Join(cacheRoot, imagesSubdir, testDigest)
	if err := os.MkdirAll(imgDir, 0o755); err != nil {
		t.Fatalf("seeding cache dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(imgDir, cacheOKMarker), nil, 0o644); err != nil {
		t.Fatalf("seeding sentinel: %v", err)
	}

	var touched []string
	hit, dir, err := VerifyThenPull(context.Background(), testRepo, testDigest,
		withHostArch("amd64"),
		withArchFn(func(_ context.Context, _ string) (string, error) {
			return "amd64", nil // matches host; gate passes
		}),
		withFetchMetaFn(func(_ context.Context, _ string) ([]byte, []byte, error) {
			touched = append(touched, "fetchMeta")
			return nil, nil, nil
		}),
		withVerifyFn(func(_, _ []byte) error {
			touched = append(touched, "verify")
			return nil
		}),
		withPullFn(func(_ context.Context, _, _ string) error {
			touched = append(touched, "pull")
			return nil
		}),
	)
	if err != nil {
		t.Fatalf("VerifyThenPull(cache hit) err = %v, want nil", err)
	}
	if !hit {
		t.Fatalf("VerifyThenPull(cache hit) hit = false, want true")
	}
	if dir != imgDir {
		t.Fatalf("VerifyThenPull(cache hit) dir = %q, want %q", dir, imgDir)
	}
	if len(touched) != 0 {
		t.Fatalf("cache hit ran %v; want no verify/fetch/pull", touched)
	}
}

// TestVerifyThenPullRejectsBadDigestAndRepo (HERMETIC): the fail-closed guards
// reject crafted digests/repos before any crane round-trip.
func TestVerifyThenPullRejectsBadDigestAndRepo(t *testing.T) {
	if _, _, err := VerifyThenPull(context.Background(), testRepo, "sha256:nope"); !errors.Is(err, errBadDigest) {
		t.Fatalf("bad digest err = %v, want errBadDigest", err)
	}
	if _, _, err := VerifyThenPull(context.Background(), "-flag-smuggle", testDigest); !errors.Is(err, errBadRepo) {
		t.Fatalf("bad repo err = %v, want errBadRepo", err)
	}
}

// TestVerifyThenPullNotConfiguredOnDefaults (HERMETIC, WR-03): a production call
// with NO option seams (the un-wired Plan 04 defaults) must fail closed with the
// single, clearly-labeled ErrNotConfigured BEFORE any verify — never with the
// branch-distinguishing errFetchMetaNotWired/errPullNotWired text — so no oracle
// can tell an un-wired path from a verification outcome. The guard fires only
// after the digest/repo fail-closed checks (those have their own test above).
func TestVerifyThenPullNotConfiguredOnDefaults(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	_, _, err := VerifyThenPull(context.Background(), testRepo, testDigest)
	if !errors.Is(err, ErrNotConfigured) {
		t.Fatalf("VerifyThenPull(un-wired defaults) err = %v, want ErrNotConfigured", err)
	}
	if errors.Is(err, errFetchMetaNotWired) || errors.Is(err, errPullNotWired) {
		t.Fatalf("VerifyThenPull leaked a per-collaborator not-wired sentinel: %v", err)
	}
}

// TestVerifyThenPullLive (GATED) is the SOLE live verify+pull test. It requires
// a container engine (docker/podman) OR network plus a HELIX_BENCH_MIRROR ref,
// and SKIPs cleanly when absent. Its hermetic siblings above
// (TestVerifyThenPullVerifiesBeforeCacheFill in particular) prove the
// verify-before-fetch decision branch WITHOUT the live dependency (Pitfall 1 —
// no skip-only-without-sibling).
func TestVerifyThenPullLive(t *testing.T) {
	mirror := os.Getenv("HELIX_BENCH_MIRROR")
	if _, err := Detect(); err != nil && mirror == "" {
		t.Skip("no engine/network/mirror (set HELIX_BENCH_MIRROR + docker/podman on PATH); skipping live verify+pull")
	}
	if mirror == "" {
		t.Skip("HELIX_BENCH_MIRROR unset; skipping live verify+pull (hermetic siblings cover the decision branch)")
	}
	// HELIX_BENCH_MIRROR is "<repo>@<64-hex-digest>"; split and drive the real
	// crane arch inspect + the production VerifyImage + a crane.Pull-backed
	// puller. When the Plan 04 mirror exists this exercises the full path; until
	// then the gate above SKIPs and the hermetic siblings carry the proof.
	repo, digest, ok := splitMirrorRef(mirror)
	if !ok {
		t.Fatalf("HELIX_BENCH_MIRROR=%q is not <repo>@<64-hex-digest>", mirror)
	}
	_, _, err := VerifyThenPull(context.Background(), repo, digest)
	if errors.Is(err, ErrNotConfigured) {
		// The live fetch/pull collaborators are still the un-wired Plan 04
		// defaults, so VerifyThenPull fails closed with ErrNotConfigured before
		// any verify. That is by design until Plan 04 wires the real
		// crane.Manifest+bundle fetcher and crane.Pull puller (see IN-03 / WR-03).
		t.Skip("VerifyThenPull not configured (Plan 04 fetch/pull wiring absent); skipping live verify+pull")
	}
	if err != nil {
		t.Fatalf("live VerifyThenPull(%s@%s) = %v", repo, digest, err)
	}
}

// splitMirrorRef parses "<repo>@<64-hex-digest>" into its parts. Returns ok=false
// on a malformed ref so the live test fails loudly rather than silently pulling
// a tag.
func splitMirrorRef(ref string) (repo, digest string, ok bool) {
	for i := len(ref) - 1; i >= 0; i-- {
		if ref[i] == '@' {
			repo, digest = ref[:i], ref[i+1:]
			return repo, digest, isHexSHA256(digest) && isValidRepo(repo)
		}
	}
	return "", "", false
}
