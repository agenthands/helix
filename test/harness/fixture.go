//go:build integration || llm || llmjudge

package harness

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// PrepareFixture copies testdata/fixtures/{lang}/ to a temp directory and returns
// the destination path. Each test gets its own copy to prevent cross-test contamination.
func PrepareFixture(tb testing.TB, lang string) string {
	tb.Helper()

	srcDir := filepath.Join(ProjectRoot(), "testdata", "fixtures", lang)
	dstDir := filepath.Join(tb.TempDir(), lang)

	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		tb.Fatalf("PrepareFixture(%s): %v", lang, err)
	}

	return dstDir
}

// RequireGopls skips the test if gopls is not installed.
func RequireGopls(tb testing.TB) {
	tb.Helper()
	if _, err := exec.LookPath("gopls"); err != nil {
		tb.Skip("gopls not installed, skipping LSP integration test")
	}
}

// RequireLS skips the test if the given language server binary is not in PATH.
func RequireLS(tb testing.TB, binary string) {
	tb.Helper()
	if _, err := exec.LookPath(binary); err != nil {
		tb.Skipf("%s not installed, skipping integration test", binary)
	}
}
