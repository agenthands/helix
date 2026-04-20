//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_FuzzyEdit exercises the standalone fuzzy_edit tool through the
// daemon + MCP protocol stack: activate workspace -> fuzzy_edit with slightly
// wrong search text -> verify the fuzzy match succeeds and reports strategy.
func TestScenario_FuzzyEdit(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// Step 1: Read the file first to know exact content.
	readResult := harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	original := harness.TextContent(readResult)
	require.Contains(t, original, "Helper")

	// Step 2: fuzzy_edit with whitespace-drifted search block (extra spaces).
	result := harness.CallTool(t, s, "fuzzy_edit", map[string]any{
		"path":        "main.go",
		"search":      "fmt.Println(\"Helper function called\")",
		"replacement": "fmt.Println(\"Helper function modified by fuzzy edit\")",
	})
	text := harness.TextContent(result)
	assert.Contains(t, text, "match_strategy", "response should report match strategy")
	assert.Contains(t, text, "similarity_score", "response should report similarity score")

	// Step 3: Verify the edit was applied.
	readResult = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	modified := harness.TextContent(readResult)
	assert.Contains(t, modified, "modified by fuzzy edit")
	assert.NotContains(t, modified, "Helper function called")
}

// TestScenario_FuzzyEdit_Ambiguity verifies that fuzzy_edit refuses ambiguous
// matches when the search text matches multiple locations.
func TestScenario_FuzzyEdit_Ambiguity(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	// "Helper()" appears at lines 7 and 27 — exact duplicate lines, ambiguous.
	result := harness.CallToolExpectError(t, runner.Session, "fuzzy_edit", map[string]any{
		"path":        "main.go",
		"search":      "\tHelper()",
		"replacement": "\tHelperRenamed()",
	})
	text := harness.TextContent(result)
	assert.Contains(t, text, "ambiguous", "should report ambiguity when search matches multiple locations")
}

// TestScenario_ReplaceInFile_FuzzyFallback verifies that replace_in_file falls
// back to fuzzy matching when exact match fails, through the full daemon stack.
func TestScenario_ReplaceInFile_FuzzyFallback(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// Multi-line search with wrong indentation (spaces instead of tabs).
	// Exact/regex match will fail, but fuzzy indentation-flexible strategy matches.
	result := harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.go",
		"pattern":     "func main() {\n    fmt.Println(\"Hello, Go!\")\n    Helper()\n}",
		"replacement": "func main() {\n\tfmt.Println(\"Hello from fuzzy fallback!\")\n\tHelper()\n}",
	})
	text := harness.TextContent(result)
	assert.Contains(t, text, "match_strategy", "fuzzy fallback should report strategy")

	// Verify the edit was applied.
	readResult := harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	assert.Contains(t, harness.TextContent(readResult), "Hello from fuzzy fallback!")
}
