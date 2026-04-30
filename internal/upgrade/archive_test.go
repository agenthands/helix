package upgrade

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestArchiveExtractTarGzHappyPath(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	if err := extractTarGz(testdataArchive, dest); err != nil {
		t.Fatalf("extractTarGz: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(dest, "helix"))
	if err != nil {
		t.Fatalf("reading extracted helix: %v", err)
	}
	want := "helix-stub-binary-v0.0.0-test\n"
	if string(got) != want {
		t.Errorf("extracted content = %q, want %q", string(got), want)
	}
}

// buildTarGzWithEntry produces a minimal .tar.gz with a single entry.
// Used by adversarial tests that construct malformed/zip-slip archives.
func buildTarGzWithEntry(t *testing.T, name, content string, mode int64) string {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	if err := tw.WriteHeader(&tar.Header{
		Name:     name,
		Mode:     mode,
		Size:     int64(len(content)),
		Typeflag: tar.TypeReg,
	}); err != nil {
		t.Fatalf("WriteHeader: %v", err)
	}
	if _, err := tw.Write([]byte(content)); err != nil {
		t.Fatalf("tw.Write: %v", err)
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tw.Close: %v", err)
	}
	if err := gw.Close(); err != nil {
		t.Fatalf("gw.Close: %v", err)
	}
	path := filepath.Join(t.TempDir(), "archive.tar.gz")
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	return path
}

func TestArchiveZipSlipRejection(t *testing.T) {
	t.Parallel()
	bad := buildTarGzWithEntry(t, "../escape.txt", "pwned", 0o644)
	dest := t.TempDir()
	err := extractTarGz(bad, dest)
	if err == nil {
		t.Fatal("zip-slip extraction succeeded; want error")
	}
	if !strings.Contains(err.Error(), "escapes") {
		t.Errorf("expected escape error, got: %v", err)
	}
}

func TestArchiveAbsolutePathRejection(t *testing.T) {
	t.Parallel()
	bad := buildTarGzWithEntry(t, "/etc/passwd-rewrite", "pwned", 0o644)
	dest := t.TempDir()
	err := extractTarGz(bad, dest)
	if err == nil {
		t.Fatal("absolute path extraction succeeded; want error")
	}
	if !strings.Contains(err.Error(), "absolute path") {
		t.Errorf("expected absolute-path error, got: %v", err)
	}
}

func TestArchiveMalformedGzip(t *testing.T) {
	t.Parallel()
	// Write garbage that isn't a valid gzip stream.
	path := filepath.Join(t.TempDir(), "bad.tar.gz")
	if err := os.WriteFile(path, []byte("not-a-gzip-file"), 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	dest := t.TempDir()
	err := extractTarGz(path, dest)
	if err == nil {
		t.Fatal("malformed gzip extraction succeeded; want error")
	}
}

func TestArchiveMissingFile(t *testing.T) {
	t.Parallel()
	dest := t.TempDir()
	err := extractTarGz("testdata/nonexistent.tar.gz", dest)
	if err == nil {
		t.Fatal("missing-file extraction succeeded; want error")
	}
}

func TestStageNewStageDir(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	installPath := filepath.Join(tmp, "helix")

	dir, cleanup, err := NewStageDir(installPath)
	if err != nil {
		t.Fatalf("NewStageDir: %v", err)
	}
	if !strings.HasPrefix(dir, tmp) {
		t.Errorf("stage dir %q not in tmp %q (sibling-of-install invariant)", dir, tmp)
	}
	st, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat stage dir: %v", err)
	}
	if !st.IsDir() {
		t.Errorf("stage dir is not a directory")
	}

	// Cleanup should remove the dir.
	cleanup()
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("stage dir still exists after cleanup: err=%v", err)
	}

	// Idempotent: second call must not panic.
	cleanup()
}

func TestStageNewStageDirReusesExistingPath(t *testing.T) {
	t.Parallel()
	tmp := t.TempDir()
	installPath := filepath.Join(tmp, "helix")

	// Pre-create stale stage dir to verify NewStageDir wipes it.
	stale := filepath.Join(tmp, stageDirName)
	if err := os.Mkdir(stale, 0o755); err != nil {
		t.Fatalf("pre-create stale: %v", err)
	}
	staleFile := filepath.Join(stale, "stale.txt")
	if err := os.WriteFile(staleFile, []byte("old"), 0o644); err != nil {
		t.Fatalf("pre-create stale file: %v", err)
	}

	dir, cleanup, err := NewStageDir(installPath)
	if err != nil {
		t.Fatalf("NewStageDir: %v", err)
	}
	defer cleanup()

	// Stale file must be gone (NewStageDir wipes the existing dir).
	if _, err := os.Stat(staleFile); !os.IsNotExist(err) {
		t.Errorf("stale file survived NewStageDir: err=%v", err)
	}
	if dir != stale {
		t.Errorf("dir = %q, want %q (deterministic sibling path)", dir, stale)
	}
}
