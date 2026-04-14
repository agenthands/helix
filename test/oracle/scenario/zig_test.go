//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Zig_FullCycle exercises a complete agent workflow against the Zig fixture:
// activate -> search (via read) -> read -> edit -> verify read-back.
// zls does not reliably support workspace/symbol or documentSymbol in temp workspaces,
// so step 1 uses read_file as a search proxy to verify the fixture is accessible.
func TestScenario_Zig_FullCycle(t *testing.T) {
	harness.RequireLS(t, "zls")

	fixtureDir := harness.PrepareFixture(t, "zig")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (search): verify fixture contains helper via file read.
	// zls does not return workspace/symbol results, so we verify via content.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "src/main.zig"})
	text := harness.TextContent(result)
	require.Contains(t, text, "helper")
	require.Contains(t, text, "DemoStruct")

	// Step 2 (read): read greeter.zig to verify cross-file awareness.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "src/greeter.zig"})
	require.Contains(t, harness.TextContent(result), "greet")

	// Step 3 (edit): change "hello" to "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "src/main.zig",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "src/main.zig"})
	readText := harness.TextContent(result)
	require.Contains(t, readText, `"hi"`)
	require.NotContains(t, readText, `return "hello"`)
}
