//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Markdown_FullCycle exercises a complete agent workflow against the Markdown fixture:
// activate -> symbols -> read -> edit -> verify read-back.
// Markdown uses marksman as the language server.
func TestScenario_Markdown_FullCycle(t *testing.T) {
	harness.RequireLS(t, "marksman")

	fixtureDir := harness.PrepareFixture(t, "markdown")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (symbols): get document symbols for main.md.
	// marksman exposes headings as document symbols.
	result := harness.CallTool(t, s, "get_symbol_overview", map[string]any{"path": "main.md"})
	text := harness.TextContent(result)
	require.Contains(t, text, "Project Overview")

	// Step 2 (read): read main.md containing the project overview.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.md"})
	require.Contains(t, harness.TextContent(result), "Getting Started")

	// Step 3 (edit): change "three layers" to "four layers".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.md",
		"pattern":     "three layers",
		"replacement": "four layers",
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.md"})
	readText := harness.TextContent(result)
	require.Contains(t, readText, "four layers")
	require.NotContains(t, readText, "three layers")
}
