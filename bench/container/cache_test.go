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
