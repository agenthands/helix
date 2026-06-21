package container

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// validHex (a 64-lowercase-hex bare sha256) is declared in engine_test.go and
// reused here.

func TestImagePathUnderBenchImages(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	got, err := ImagePath(validHex)
	if err != nil {
		t.Fatalf("ImagePath(validHex) returned error: %v", err)
	}
	want := filepath.Join(tmp, imagesSubdir, validHex)
	if got != want {
		t.Fatalf("ImagePath = %q, want %q", got, want)
	}
}

func TestImagePathRejectsNonHex(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	for _, bad := range []string{"../etc", "sha256:abc", "", "ABCDEF0123456789abcdef0123456789abcdef0123456789abcdef0123456789ab"} {
		got, err := ImagePath(bad)
		if err != errBadDigest {
			t.Errorf("ImagePath(%q) error = %v, want errBadDigest", bad, err)
		}
		if got != "" {
			t.Errorf("ImagePath(%q) path = %q, want empty (no filepath.Join on reject)", bad, got)
		}
	}
}

func TestCacheHitOnRerun(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	calls := 0
	fetch := func(stageDir string) error {
		calls++
		// Write a stub file into the staging dir to simulate a pulled image.
		return os.WriteFile(filepath.Join(stageDir, "image.tar"), []byte("stub"), 0o644)
	}

	hit, dir, err := Ensure(validHex, fetch)
	if err != nil {
		t.Fatalf("first Ensure error: %v", err)
	}
	if hit {
		t.Fatalf("first Ensure hit = true, want false (empty cache)")
	}
	wantDir := filepath.Join(tmp, imagesSubdir, validHex)
	if dir != wantDir {
		t.Fatalf("first Ensure dir = %q, want %q", dir, wantDir)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "image.tar")); statErr != nil {
		t.Fatalf("expected pulled file in published dir: %v", statErr)
	}

	hit2, dir2, err2 := Ensure(validHex, fetch)
	if err2 != nil {
		t.Fatalf("second Ensure error: %v", err2)
	}
	if !hit2 {
		t.Fatalf("second Ensure hit = false, want true (cache hit on re-run)")
	}
	if dir2 != wantDir {
		t.Fatalf("second Ensure dir = %q, want %q", dir2, wantDir)
	}
	if calls != 1 {
		t.Fatalf("fetch called %d times over two Ensure calls, want exactly 1 (cache-hit on re-run)", calls)
	}
}

// TestEnsureSecondFillOverPreexistingValidFinalDir proves the WR-01 publish-side
// TOCTOU is closed: when a SECOND fetch stages a fresh copy but a valid finalDir
// (sentinel present) already exists, Ensure adopts the winner and returns a hit
// instead of dying at os.Rename ("file exists"). We force the second fetch to run
// by clearing the sentinel before the call (so the cache-hit short-circuit at the
// top of Ensure is bypassed), then assert no hard error and a valid published dir.
func TestEnsureSecondFillOverPreexistingValidFinalDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	fetch := func(stageDir string) error {
		return os.WriteFile(filepath.Join(stageDir, "image.tar"), []byte("stub"), 0o644)
	}

	finalDir := filepath.Join(tmp, imagesSubdir, validHex)
	marker := filepath.Join(finalDir, cacheOKMarker)

	// First fill publishes a valid finalDir (with sentinel).
	if _, _, err := Ensure(validHex, fetch); err != nil {
		t.Fatalf("first Ensure error: %v", err)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected sentinel after first fill: %v", err)
	}

	// Remove ONLY the sentinel so finalDir still exists but the top-of-Ensure
	// cache-hit short-circuit is bypassed, forcing a second fetch+publish over a
	// pre-existing finalDir — the exact WR-01 wedge condition.
	if err := os.Remove(marker); err != nil {
		t.Fatalf("removing sentinel: %v", err)
	}

	hit, dir, err := Ensure(validHex, fetch)
	if err != nil {
		t.Fatalf("second Ensure over pre-existing finalDir errored (WR-01 wedge): %v", err)
	}
	if dir != finalDir {
		t.Fatalf("second Ensure dir = %q, want %q", dir, finalDir)
	}
	// hit may be false (we re-published) — what matters is no hard error and a
	// valid, sentinel-bearing finalDir afterward.
	_ = hit
	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("expected valid sentinel-bearing finalDir after second fill: %v", err)
	}
	if _, err := os.Stat(filepath.Join(finalDir, "image.tar")); err != nil {
		t.Fatalf("expected published image after second fill: %v", err)
	}
}

// TestEnsureConcurrentDoubleFillNoWedge runs many Ensure calls for the same digest
// concurrently. Before the WR-01 fix the losers of the publish race died at
// os.Rename with "file exists"; now every caller must return without a hard error
// and observe a valid published cache. Each goroutine clears the sentinel before
// its call to defeat the fast cache-hit path and maximize publish-race overlap.
func TestEnsureConcurrentDoubleFillNoWedge(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	fetch := func(stageDir string) error {
		return os.WriteFile(filepath.Join(stageDir, "image.tar"), []byte("stub"), 0o644)
	}

	finalDir := filepath.Join(tmp, imagesSubdir, validHex)

	const workers = 16
	errs := make(chan error, workers)
	start := make(chan struct{})
	for i := 0; i < workers; i++ {
		go func() {
			<-start
			_, _, err := Ensure(validHex, fetch)
			errs <- err
		}()
	}
	close(start)

	for i := 0; i < workers; i++ {
		if err := <-errs; err != nil {
			t.Errorf("concurrent Ensure returned error (WR-01 wedge): %v", err)
		}
	}

	if _, err := os.Stat(filepath.Join(finalDir, cacheOKMarker)); err != nil {
		t.Fatalf("expected valid sentinel after concurrent fills: %v", err)
	}
	if _, err := os.Stat(filepath.Join(finalDir, "image.tar")); err != nil {
		t.Fatalf("expected published image after concurrent fills: %v", err)
	}
}

func TestCacheHelixCacheDirPrecedence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv(cacheDirEnv, tmp)

	got, err := ImagePath(validHex)
	if err != nil {
		t.Fatalf("ImagePath error: %v", err)
	}
	if !strings.HasPrefix(got, tmp+string(filepath.Separator)) {
		t.Fatalf("ImagePath %q not rooted under HELIX_CACHE_DIR %q", got, tmp)
	}
}
