//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_TypeScript_FullCycle exercises a complete agent workflow against the TypeScript fixture:
// activate -> search -> read -> edit -> verify read-back.
func TestScenario_TypeScript_FullCycle(t *testing.T) {
	harness.RequireLS(t, "typescript-language-server")

	fixtureDir := harness.PrepareFixture(t, "typescript")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (search): find the Greeter symbol.
	result := harness.CallTool(t, s, "search_symbols", map[string]any{"query": "Greeter"})
	require.Contains(t, harness.TextContent(result), "Greeter")

	// Step 2 (read): read greeter.ts containing the Greeter class.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "greeter.ts"})
	require.Contains(t, harness.TextContent(result), "Greeter")

	// Step 3 (edit): change "Hello" to "Hey" in the greeting.
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "greeter.ts",
		"pattern":     "Hello",
		"replacement": "Hey",
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "greeter.ts"})
	text := harness.TextContent(result)
	require.Contains(t, text, "Hey")
	require.NotContains(t, text, "Hello")
}
