//go:build integration

package integration_test

import (
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// updateGolden rewrites golden files instead of comparing.
// Usage: go test -tags integration ./test/integration/... -update
// Or:    GOLDEN_UPDATE=1 go test -tags integration ./test/integration/...
// Per CONTEXT D-02: diffs must be reviewable in PR — updates always show in git status.
var updateGolden = flag.Bool("update", false, "update golden files instead of comparing")

// goldenDir returns the absolute path to testdata/profiles/ under the repo root.
func goldenDir() string {
	return filepath.Join(projectRoot(), "testdata", "profiles")
}

// assertGoldenTools compares `actual` (a list of tool names) against the golden
// file `testdata/profiles/<name>.tools.golden`. The file format is sorted tool
// names, one per line, LF-terminated. On -update (or GOLDEN_UPDATE=1), the file
// is rewritten instead of compared.
//
// Per D-01/D-02: the golden file is the ORACLE. Tests must not compute expected
// output from `.helix/profiles/*.yaml` — that would share the code-under-test's
// source of truth and mask bad YAML changes.
func assertGoldenTools(t *testing.T, name string, actual []string) {
	t.Helper()
	if err := os.MkdirAll(goldenDir(), 0o755); err != nil {
		t.Fatalf("mkdir golden dir: %v", err)
	}
	path := filepath.Join(goldenDir(), name+".tools.golden")

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
		t.Errorf("golden %s mismatch.\n--- want:\n%s\n+++ got:\n%s\n(run: go test -tags integration ./test/integration/... -update -run <TestName> to accept)",
			name, string(wantBytes), got)
	}
}
