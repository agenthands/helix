//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestScenario_Cpp_FullCycle exercises a complete agent workflow against the C++ fixture:
// activate -> search (via symbol overview) -> read -> edit -> verify read-back.
// clangd does not support workspace/symbol for small projects, so we use
// get_symbol_overview (textDocument/documentSymbol) for the discovery step.
func TestScenario_Cpp_FullCycle(t *testing.T) {
	harness.RequireLS(t, "clangd")

	fixtureDir := harness.PrepareFixture(t, "cpp")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (discover): get symbol overview to find helper.
	// clangd needs didOpen (handled by quirk) then responds immediately.
	text := harness.WaitForSearchResults(t, s, "helper", 30*time.Second)
	require.Contains(t, text, "helper")

	// Step 2 (read): read main.cpp containing helper.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "main.cpp"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 3 (edit): change return "hello" to return "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.cpp",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.cpp"})
	readText := harness.TextContent(result)
	require.Contains(t, readText, `"hi"`)
	require.NotContains(t, readText, `return "hello"`)
}
