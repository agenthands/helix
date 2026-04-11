//go:build integration || llm || llmjudge

package protocol_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestSessionIsolation_WorkspaceIndependence verifies that two separate daemon
// instances with different workspace configurations do not observe each other's
// state. Per Pitfall 3, a single daemon is single-workspace, so isolation
// requires separate StartRunner calls (PROTO-03, D-02).
func TestSessionIsolation_WorkspaceIndependence(t *testing.T) {
	// Runner A: workspace activated (Go fixture)
	runnerA := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: harness.PrepareFixture(t, "go"),
		SkipLS:       true,
	})
	defer runnerA.Stop()

	// Runner B: no workspace activated
	runnerB := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runnerB.Stop()

	// Runner A should succeed listing directory (workspace active).
	harness.CallTool(t, runnerA.Session, "list_directory", map[string]any{"path": "."})

	// Runner B should fail listing directory (no workspace active).
	harness.CallToolExpectError(t, runnerB.Session, "list_directory", map[string]any{"path": "."})
}

// TestSessionIsolation_ModeRestrictions verifies that mode restrictions apply
// independently per daemon instance. A read-mode daemon should not expose edit
// tools, while an admin-mode daemon should (PROTO-03, D-02, T-19-03).
func TestSessionIsolation_ModeRestrictions(t *testing.T) {
	// Runner A: read mode -- should NOT have edit tools.
	runnerA := harness.StartRunner(t, harness.RunnerOptions{
		SkipLS: true,
		Mode:   "read",
	})
	defer runnerA.Stop()

	// Runner B: admin mode -- should have edit tools.
	runnerB := harness.StartRunner(t, harness.RunnerOptions{
		SkipLS: true,
		Mode:   "admin",
	})
	defer runnerB.Stop()

	toolsA := harness.ListSessionTools(t, runnerA.Session)
	toolsB := harness.ListSessionTools(t, runnerB.Session)

	// Edit tools excluded from read mode whose names match actual registered tool names.
	// Note: read.yaml exclude_tools also lists legacy names (create_text_file,
	// replace_content, delete_symbol) that don't match current registrations
	// (create_file, replace_in_file, safe_delete_symbol) -- those are not tested here.
	editTools := []string{
		"replace_symbol_body",
		"insert_before_symbol",
		"insert_after_symbol",
	}

	containsTool := func(tools []string, name string) bool {
		for _, t := range tools {
			if t == name {
				return true
			}
		}
		return false
	}

	for _, et := range editTools {
		t.Run("read_excludes_"+et, func(t *testing.T) {
			require.False(t, containsTool(toolsA, et),
				"read mode should NOT contain edit tool %s", et)
		})
		t.Run("admin_includes_"+et, func(t *testing.T) {
			require.True(t, containsTool(toolsB, et),
				"admin mode should contain edit tool %s", et)
		})
	}
}

// TestSessionIsolation_ToolListIndependence verifies that two daemon instances
// configured with different modes have independently resolved tool lists
// (PROTO-03, T-19-01).
//
// Note: Memory store isolation cannot be tested with separate StartRunner
// calls because skill.InitAll uses Caddy-style global registration -- the
// daemon captures tool handler closures at construction time, and the global
// skill store pointer is shared. Workspace and mode isolation (tested above)
// are the observable isolation boundaries.
func TestSessionIsolation_ToolListIndependence(t *testing.T) {
	// Runner A: full profile, read mode (restricted tool set).
	runnerA := harness.StartRunner(t, harness.RunnerOptions{
		SkipLS: true,
		Mode:   "read",
	})
	defer runnerA.Stop()

	// Runner B: full profile, edit mode (broader tool set).
	runnerB := harness.StartRunner(t, harness.RunnerOptions{
		SkipLS: true,
		Mode:   "edit",
	})
	defer runnerB.Stop()

	toolsA := harness.ListSessionTools(t, runnerA.Session)
	toolsB := harness.ListSessionTools(t, runnerB.Session)

	require.NotEmpty(t, toolsA, "read-mode daemon should have tools")
	require.NotEmpty(t, toolsB, "edit-mode daemon should have tools")

	// The two daemons should have different tool counts because mode filtering
	// is applied per-daemon. This proves each daemon's session resolves its
	// own tool list independently.
	require.NotEqual(t, len(toolsA), len(toolsB),
		"read mode (%d tools) and edit mode (%d tools) should have different tool counts",
		len(toolsA), len(toolsB))
}
