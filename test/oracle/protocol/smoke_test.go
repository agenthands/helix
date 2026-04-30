//go:build integration || llm || llmjudge

package protocol_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/test/harness"
	"github.com/stretchr/testify/require"
)

// TestSmokeImportHarness validates that the harness package is importable and
// functional from an oracle package. It exercises StartRunner, ListSessionTools,
// and Stop across the package boundary.
func TestSmokeImportHarness(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	tools := harness.ListSessionTools(t, runner.Session)
	require.NotEmpty(t, tools, "should have registered tools")

	t.Logf("found %d tools, first 5:", len(tools))
	for i, name := range tools {
		if i >= 5 {
			break
		}
		t.Logf("  [%d] %s", i, name)
	}
}

// TestSmokeFixture validates that PrepareFixture works cross-package and copies
// the Go fixture with the expected main.go file.
func TestSmokeFixture(t *testing.T) {
	dir := harness.PrepareFixture(t, "go")
	require.DirExists(t, dir)

	mainPath := filepath.Join(dir, "main.go")
	_, err := os.Stat(mainPath)
	require.NoError(t, err, "main.go should exist in go fixture")
}

// TestSmokeProjectRoot validates that ProjectRoot resolves to the repo root
// containing go.mod.
func TestSmokeProjectRoot(t *testing.T) {
	root := harness.ProjectRoot()

	goModPath := filepath.Join(root, "go.mod")
	_, err := os.Stat(goModPath)
	require.NoError(t, err, "go.mod should exist at project root")
}
