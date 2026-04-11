//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Python_FullCycle exercises a complete agent workflow against the Python fixture:
// activate -> search -> read -> edit -> verify read-back.
func TestScenario_Python_FullCycle(t *testing.T) {
	harness.RequireLS(t, "pyright-langserver")

	fixtureDir := harness.PrepareFixture(t, "python")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (search): find the helper symbol.
	result := harness.CallTool(t, s, "search_symbols", map[string]any{"query": "helper"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 2 (read): read main.py containing helper.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.py"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 3 (edit): change return "hello" to return "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.py",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.py"})
	text := harness.TextContent(result)
	require.Contains(t, text, `"hi"`)
	require.NotContains(t, text, `return "hello"`)
}
