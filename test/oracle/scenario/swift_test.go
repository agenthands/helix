//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Swift_FullCycle exercises a complete agent workflow against the Swift fixture:
// activate -> search -> read -> edit -> verify read-back.
func TestScenario_Swift_FullCycle(t *testing.T) {
	harness.RequireLS(t, "sourcekit-lsp")

	fixtureDir := harness.PrepareFixture(t, "swift")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
	})
	s := runner.Session

	// Step 1 (search): find the helper symbol. sourcekit-lsp needs indexing time.
	text := harness.WaitForSearchResults(t, s, "helper", 60*time.Second)
	require.Contains(t, text, "helper")

	// Step 2 (read): read main.swift containing helper.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "Sources/main.swift"})
	require.Contains(t, harness.TextContent(result), "helper")

	// Step 3 (edit): change return "hello" to return "hi".
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "Sources/main.swift",
		"pattern":     `return "hello"`,
		"replacement": `return "hi"`,
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "Sources/main.swift"})
	readText := harness.TextContent(result)
	require.Contains(t, readText, `"hi"`)
	require.NotContains(t, readText, `return "hello"`)
}
