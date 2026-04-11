//go:build integration || llm || llmjudge

package harness

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// updateGolden rewrites golden files instead of comparing.
// Usage: go test -tags integration ./test/harness/... -update
// Or:    GOLDEN_UPDATE=1 go test -tags integration ./test/harness/...
var updateGolden = flag.Bool("update", false, "update golden files instead of comparing")

// GoldenStore manages golden file comparisons for a given root directory.
type GoldenStore struct {
	// RootDir is the directory containing golden files.
	// If empty, defaults to filepath.Join(ProjectRoot(), "testdata", "golden").
	RootDir string
}

// NewGoldenStore creates a GoldenStore with the given root directory.
func NewGoldenStore(rootDir string) *GoldenStore {
	return &GoldenStore{RootDir: rootDir}
}

// rootDir returns the effective root directory, applying the default if RootDir is empty.
func (gs *GoldenStore) rootDir() string {
	if gs.RootDir != "" {
		return gs.RootDir
	}
	return filepath.Join(ProjectRoot(), "testdata", "golden")
}

// AssertTools compares actual (a list of tool names) against the golden file
// {RootDir}/{name}.tools.golden. The file format is sorted tool names, one per
// line, LF-terminated. On -update (or GOLDEN_UPDATE=1), the file is rewritten
// instead of compared.
func (gs *GoldenStore) AssertTools(t *testing.T, name string, actual []string) {
	t.Helper()

	dir := gs.rootDir()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	path := filepath.Join(dir, name+".tools.golden")

	sorted := make([]string, len(actual))
	copy(sorted, actual)
	sort.Strings(sorted)
	got := strings.Join(sorted, "\n") + "\n"

	if *updateGolden || os.Getenv("GOLDEN_UPDATE") == "1" {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		t.Logf("updated golden file: %s", path)
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if string(wantBytes) != got {
		t.Errorf("golden %s mismatch.\n--- want:\n%s\n+++ got:\n%s\n(run with -update to accept)",
			name, string(wantBytes), got)
	}
}

// AssertGolden compares got against the golden file at path. On -update (or
// GOLDEN_UPDATE=1), the file is rewritten instead of compared.
func AssertGolden(t *testing.T, path string, got []byte) {
	t.Helper()

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}

	if *updateGolden || os.Getenv("GOLDEN_UPDATE") == "1" {
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("update golden %s: %v", path, err)
		}
		t.Logf("updated golden file: %s", path)
		return
	}

	wantBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with -update to create)", path, err)
	}
	if string(wantBytes) != string(got) {
		t.Errorf("golden %s mismatch.\n--- want:\n%s\n+++ got:\n%s\n(run with -update to accept)",
			path, string(wantBytes), string(got))
	}
}
