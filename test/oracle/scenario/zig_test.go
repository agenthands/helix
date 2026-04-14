//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Zig_FullCycle exercises a complete agent workflow against the Zig fixture:
// activate -> read -> edit -> verify read-back.
// Note: zls does not reliably support workspace/symbol in temp workspaces,
// so we skip the search step and focus on file operations.
func TestScenario_Zig_FullCycle(t *testing.T) {
	harness.RequireLS(t, "zls")

	fixtureDir := harness.PrepareFixture(t, "zig")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (read): read main.zig containing helper.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "src/main.zig"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 2 (edit): change "hello" to "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "src/main.zig",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 3 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "src/main.zig"})
	text := harness.TextContent(result)
	require.Contains(t, text, `"hi"`)
	require.NotContains(t, text, `return "hello"`)
}
