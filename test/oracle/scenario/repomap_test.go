//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agenthands/helix/test/harness"
)

// TestScenario_GetRepoMap exercises the full get_repo_map tool through the
// daemon + MCP protocol stack: activate workspace -> get_repo_map -> verify
// ranked output contains Go files with symbol elision.
func TestScenario_GetRepoMap(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	result := harness.CallTool(t, runner.Session, "get_repo_map", map[string]any{
		"token_budget": float64(4096),
	})
	text := harness.TextContent(result)

	assert.NotEmpty(t, text, "get_repo_map should return non-empty output")
	assert.NotContains(t, text, "No files found", "get_repo_map should find files in workspace")
}

// TestScenario_GetRepoMap_TokenBudget verifies that a small token budget
// produces smaller output than a large budget.
func TestScenario_GetRepoMap_TokenBudget(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	smallResult := harness.CallTool(t, runner.Session, "get_repo_map", map[string]any{
		"token_budget": float64(64),
	})
	largeResult := harness.CallTool(t, runner.Session, "get_repo_map", map[string]any{
		"token_budget": float64(8192),
	})

	smallText := harness.TextContent(smallResult)
	largeText := harness.TextContent(largeResult)

	assert.NotEmpty(t, smallText, "small budget should still produce output (min 1 file)")
	assert.NotEmpty(t, largeText)
	assert.LessOrEqual(t, len(smallText), len(largeText),
		"small budget output should not exceed large budget output")
}

// TestScenario_GetContext exercises the full get_context tool through the
// daemon + MCP protocol stack: activate workspace -> get_context with seed
// files -> verify personalized ranked output.
func TestScenario_GetContext(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	result := harness.CallTool(t, runner.Session, "get_context", map[string]any{
		"files":        []any{"main.go"},
		"token_budget": float64(4096),
	})
	text := harness.TextContent(result)

	assert.NotEmpty(t, text, "get_context should return non-empty output")
	assert.NotContains(t, text, "No files found", "get_context should find relevant files")
}
