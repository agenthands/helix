//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Unsupported_FileOpsWork (SCEN-03, T-20-04) verifies that file-level
// tools work against an unsupported language fixture (.xyz files).
func TestScenario_Unsupported_FileOpsWork(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "unsupported")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// read_file should succeed on .xyz files.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "main.xyz"})
	text := harness.TextContent(result)
	require.Contains(t, text, "function main")

	// list_directory should see .xyz files.
	result = harness.CallTool(t, s, "list_directory", map[string]any{"path": "."})
	text = harness.TextContent(result)
	require.Contains(t, text, "main.xyz")
	require.Contains(t, text, "utils.xyz")

	// search_in_files should find content in .xyz files.
	result = harness.CallTool(t, s, "search_in_files", map[string]any{"pattern": "function"})
	text = harness.TextContent(result)
	require.Contains(t, text, ".xyz")
}

// TestScenario_Unsupported_LSToolsFailCleanly (SCEN-03, T-20-04) verifies that
// LS-dependent tools fail with meaningful errors for unsupported languages.
func TestScenario_Unsupported_LSToolsFailCleanly(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "unsupported")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// search_symbols should fail cleanly -- no LS available.
	result := harness.CallToolExpectError(t, s, "search_symbols", map[string]any{"query": "main"})
	text := harness.TextContent(result)
	// Error should be meaningful, not empty or a raw stack trace.
	require.NotEmpty(t, text, "error message should not be empty")
	require.NotContains(t, text, "goroutine", "error should not be a stack trace")
	require.True(t, result.IsError, "result should be marked as error")
}
