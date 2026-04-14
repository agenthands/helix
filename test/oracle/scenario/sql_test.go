//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_SQL_FullCycle exercises a complete agent workflow against the SQL fixture:
// activate -> search (content scan) -> read -> edit -> verify read-back.
// SQL has no standard language server, so this tests file operations only.
// This verifies Serena handles SQL files correctly for read/write/search.
func TestScenario_SQL_FullCycle(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "sql")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// Step 1 (search): find helper via file content search.
	result := harness.CallTool(t, s, "search_in_files", map[string]any{
		"pattern": "helper",
	})
	text := harness.TextContent(result)
	require.Contains(t, text, "helper")

	// Step 2 (read): read main.sql containing the helper table.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.sql"})
	require.Contains(t, harness.TextContent(result), "CREATE TABLE helper")

	// Step 3 (edit): change 'hello' to 'hi' in the INSERT.
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.sql",
		"pattern":     "'hello'",
		"replacement": "'hi'",
	})

	// Step 4 (verify): confirm the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.sql"})
	readText := harness.TextContent(result)
	require.Contains(t, readText, "'hi'")
	require.NotContains(t, readText, "'hello'")

	// Step 5 (cross-file): read schema.sql to verify multi-file awareness.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "schema.sql"})
	require.Contains(t, harness.TextContent(result), "greet")
}
