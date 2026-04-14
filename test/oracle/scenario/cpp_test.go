//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Cpp_FullCycle exercises a complete agent workflow against the C++ fixture:
// activate -> read -> edit -> verify read-back.
// Note: clangd (Apple) does not support workspace/symbol for small projects,
// so we skip the search step and focus on file operations + LS-backed edits.
func TestScenario_Cpp_FullCycle(t *testing.T) {
	harness.RequireLS(t, "clangd")

	fixtureDir := harness.PrepareFixture(t, "cpp")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (read): read main.cpp containing helper.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "main.cpp"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 2 (edit): change return "hello" to return "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.cpp",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 3 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.cpp"})
	text := harness.TextContent(result)
	require.Contains(t, text, `"hi"`)
	require.NotContains(t, text, `return "hello"`)
}
