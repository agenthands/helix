//go:build integration || llm || llmjudge

package scenario_test

import (
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// TestScenario_Degraded_FileOpsWithoutLS (D-04, T-20-05) verifies that file-level tools
// continue to work when the language server is unavailable. Uses the Go fixture with
// SkipLS to simulate LS unavailability.
func TestScenario_Degraded_FileOpsWithoutLS(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// read_file should succeed without LS.
	result := harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	require.Contains(t, harness.TextContent(result), "Helper")

	// list_directory should succeed without LS.
	result = harness.CallTool(t, s, "list_directory", map[string]any{"path": "."})
	text := harness.TextContent(result)
	require.Contains(t, text, "main.go")

	// search_in_files should succeed without LS.
	result = harness.CallTool(t, s, "search_in_files", map[string]any{"pattern": "Helper"})
	require.Contains(t, harness.TextContent(result), "Helper")

	// replace_in_file should succeed without LS.
	harness.CallTool(t, s, "replace_in_file", map[string]any{
		"path":        "main.go",
		"pattern":     "Helper function called",
		"replacement": "Helper function invoked",
	})

	// Verify the edit took effect.
	result = harness.CallTool(t, s, "read_file", map[string]any{"path": "main.go"})
	text = harness.TextContent(result)
	require.Contains(t, text, "invoked")
	require.NotContains(t, text, "Helper function called")
}

// TestScenario_Degraded_LSToolsReportHonestly (D-04, T-20-05) verifies that LS-dependent
// tools respond promptly and honestly when the LS readiness wait is skipped.
// Note: SkipLS only skips the readiness wait, it does not prevent the LS from starting.
// If gopls is installed, the LS may still be available. This test verifies tools produce
// meaningful responses (success or clean error) without hanging — the "no infinite waits"
// guarantee. The "LS truly unavailable" path is tested by TestScenario_Unsupported_LSToolsFailCleanly.
func TestScenario_Degraded_LSToolsReportHonestly(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
	})
	s := runner.Session

	// LS-dependent tools should respond promptly regardless of LS state.
	// They may succeed (if gopls started fast) or fail cleanly (if not ready yet).
	// Either way, the response must be meaningful, not a hang or stack trace.

	// search_symbols: verify prompt response with meaningful content.
	result, err := s.CallTool(
		t.Context(),
		&mcp.CallToolParams{
			Name:      "search_symbols",
			Arguments: map[string]any{"query": "Helper"},
		},
	)
	require.NoError(t, err, "search_symbols should not produce a protocol error")
	text := harness.TextContent(result)
	require.NotEmpty(t, text, "response should not be empty")
	require.NotContains(t, text, "goroutine", "response should not be a stack trace")
	if result.IsError {
		t.Logf("search_symbols returned error (LS not ready): %s", text)
	} else {
		t.Logf("search_symbols succeeded (LS was ready): %s", text[:min(len(text), 80)])
	}

	// go_to_definition: same prompt-response guarantee.
	result, err = s.CallTool(
		t.Context(),
		&mcp.CallToolParams{
			Name: "go_to_definition",
			Arguments: map[string]any{
				"path":   "main.go",
				"line":   6,
				"column": 1,
			},
		},
	)
	require.NoError(t, err, "go_to_definition should not produce a protocol error")
	text = harness.TextContent(result)
	require.NotEmpty(t, text, "response should not be empty")
	require.NotContains(t, text, "goroutine", "response should not be a stack trace")
}
