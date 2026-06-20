package ragindex

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeCorpus materializes a map of relative-path -> content under a fresh
// temp dir and returns the root. Subdirectories are created as needed.
func writeCorpus(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", p, err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", p, err)
		}
	}
	return root
}

func TestCorpusSHADeterministic(t *testing.T) {
	files := map[string]string{
		"a.go":        "package a\nfunc A() {}\n",
		"sub/b.go":    "package b\nfunc B() {}\n",
		"sub/c/d.txt": "hello world\n",
	}

	root1 := writeCorpus(t, files)
	sha1, err := CorpusSHA(root1)
	if err != nil {
		t.Fatalf("CorpusSHA(root1): %v", err)
	}
	if sha1 == "" {
		t.Fatal("CorpusSHA returned empty string")
	}

	// Same logical corpus written into a different temp dir, files created in a
	// different order, with different mtimes => identical SHA (content-only).
	root2 := writeCorpus(t, map[string]string{
		"sub/c/d.txt": "hello world\n",
		"sub/b.go":    "package b\nfunc B() {}\n",
		"a.go":        "package a\nfunc A() {}\n",
	})
	// Bump mtimes far into the past to prove mtime is NOT in the hash.
	old := mustParseTime(t)
	for rel := range files {
		if err := os.Chtimes(filepath.Join(root2, rel), old, old); err != nil {
			t.Fatalf("chtimes: %v", err)
		}
	}
	sha2, err := CorpusSHA(root2)
	if err != nil {
		t.Fatalf("CorpusSHA(root2): %v", err)
	}
	if sha1 != sha2 {
		t.Fatalf("CorpusSHA not deterministic across order/mtime: %s != %s", sha1, sha2)
	}

	// Re-hashing the same root twice is stable.
	sha1b, err := CorpusSHA(root1)
	if err != nil {
		t.Fatalf("CorpusSHA(root1) again: %v", err)
	}
	if sha1 != sha1b {
		t.Fatalf("CorpusSHA not stable on repeat: %s != %s", sha1, sha1b)
	}
}

func TestCorpusSHAContentDiscriminates(t *testing.T) {
	base := map[string]string{"a.go": "package a\nfunc A() {}\n"}
	rootA := writeCorpus(t, base)
	shaA, err := CorpusSHA(rootA)
	if err != nil {
		t.Fatalf("CorpusSHA(rootA): %v", err)
	}

	rootB := writeCorpus(t, map[string]string{"a.go": "package a\nfunc A() { return }\n"})
	shaB, err := CorpusSHA(rootB)
	if err != nil {
		t.Fatalf("CorpusSHA(rootB): %v", err)
	}
	if shaA == shaB {
		t.Fatal("CorpusSHA did not change when file content changed")
	}
}

func TestCacheDirPrecedence(t *testing.T) {
	// HELIX_CACHE_DIR set => returned verbatim.
	want := filepath.Join(t.TempDir(), "explicit-cache")
	t.Setenv("HELIX_CACHE_DIR", want)
	if got := cacheDir(); got != want {
		t.Fatalf("HELIX_CACHE_DIR not honored verbatim: got %q want %q", got, want)
	}

	// Unset => a UserCacheDir-rooted path under a helix segment.
	t.Setenv("HELIX_CACHE_DIR", "")
	got := cacheDir()
	if got == "" {
		t.Fatal("cacheDir() returned empty with HELIX_CACHE_DIR unset")
	}
	if !strings.Contains(got, "helix") {
		t.Fatalf("fallback cacheDir not under a helix segment: %q", got)
	}
}

func TestIndexPathLayout(t *testing.T) {
	base := filepath.Join(t.TempDir(), "cache-root")
	t.Setenv("HELIX_CACHE_DIR", base)

	root := writeCorpus(t, map[string]string{"a.go": "package a\n"})
	sha, err := CorpusSHA(root)
	if err != nil {
		t.Fatalf("CorpusSHA: %v", err)
	}

	got, err := IndexPath(root)
	if err != nil {
		t.Fatalf("IndexPath: %v", err)
	}
	want := filepath.Join(base, "bench-rag-index", sha)
	if got != want {
		t.Fatalf("IndexPath layout wrong: got %q want %q", got, want)
	}
	if filepath.Base(got) != sha {
		t.Fatalf("IndexPath does not end in corpus_sha segment: %q", got)
	}
	if !strings.Contains(got, "bench-rag-index") {
		t.Fatalf("IndexPath missing bench-rag-index segment: %q", got)
	}
}
