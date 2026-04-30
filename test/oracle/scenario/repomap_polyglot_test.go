//go:build integration || llm || llmjudge

package scenario_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestScenario_RepoMap_Polyglot guards BUG-01 (Phase 46): on a polyglot
// workspace containing Go sources under pkg/ and a deeply-nested Lua
// testdata fixture, get_repo_map MUST surface Go symbols and MUST NOT be
// monopolized by the Lua fixture. This test pins the fix at the MCP-tool
// boundary; the unit reproduction lives in
// internal/repomap/polyglot_rank_test.go.
//
// Test-environment note: daemon.New() re-runs skill.InitAll with
// $HOME/.helix/default-project as ProjectDir, which in practice points at
// a populated tag cache for the developer's own repo. We point $HOME at an
// isolated tb.TempDir() for the duration of this test so the daemon's
// RepoMapSkill.Init binds to a fresh empty tag cache, and activate_project
// rebinds the workspace root to our polyglot fixture.
func TestScenario_RepoMap_Polyglot(t *testing.T) {
	// Isolate daemon's ~/.helix lookup from the developer's populated cache.
	t.Setenv("HOME", t.TempDir())

	fixtureDir := harness.PrepareFixture(t, "polyglot_lua")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})

	result := harness.CallTool(t, runner.Session, "get_repo_map", map[string]any{
		"token_budget": float64(4096),
	})
	text := harness.TextContent(result)

	require.NotEmpty(t, text, "get_repo_map should return non-empty output")
	assert.NotContains(t, text, "No files found")

	// Must reference at least one Go file under pkg/.
	assert.Contains(t, text, ".go", "rendered map must reference at least one .go file")
	assert.Contains(t, text, "pkg", "rendered map must reference pkg/ Go sources")

	// Must contain at least one Go symbol from the fixture.
	anyGoSym := strings.Contains(text, "Server") ||
		strings.Contains(text, "Store") ||
		strings.Contains(text, "Handler") ||
		(strings.Contains(text, "Logger") && strings.Contains(text, "logger.go"))
	assert.True(t, anyGoSym,
		"rendered map must contain at least one Go symbol from pkg/, got:\n%s", text)

	// Guard against Lua-domination: the number of .go references must be
	// greater than zero and Go must appear whenever Lua appears.
	goCount := strings.Count(text, ".go")
	luaCount := strings.Count(text, ".lua")
	assert.Greater(t, goCount, 0, "at least one .go reference expected")
	if luaCount > 0 {
		assert.GreaterOrEqual(t, goCount, 1,
			"Go must appear when Lua appears (got .go=%d, .lua=%d)\n%s", goCount, luaCount, text)
	}
}
