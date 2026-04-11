//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Go_FullCycle exercises a complete agent workflow against the Go fixture:
// activate -> search -> read -> edit -> verify read-back -> verify search after edit.
func TestScenario_Go_FullCycle(t *testing.T) {
	harness.RequireGopls(t)

	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (search): find the Helper symbol.
	result := harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Helper"})
	require.Contains(t, harness.TextContent(result), "Helper")

	// Step 2 (read): read the file containing Helper.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	require.Contains(t, harness.TextContent(result), "Helper")

	// Step 3 (edit): change "Helper function called" to "Helper function invoked".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.go",
		"pattern":     "Helper function called",
		"replacement": "Helper function invoked",
	})

	// Step 4 (verify read-back): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	text := harness.TextContent(result)
	require.Contains(t, text, "invoked")
	require.NotContains(t, text, "Helper function called")

	// Step 5 (verify search after edit): Helper symbol should still be found.
	result = harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Helper"})
	require.Contains(t, harness.TextContent(result), "Helper")
}
