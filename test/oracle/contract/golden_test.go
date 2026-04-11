//go:build integration || llm || llmjudge

// Golden output tests for every accessible MCP tool (CONT-01).
//
// First run MUST use GOLDEN_UPDATE=1 to generate baseline golden files:
//   GOLDEN_UPDATE=1 go test -tags integration -run TestGolden -count=1 -timeout 5m ./test/oracle/contract/...
//
// Subsequent runs compare against the golden files:
//   go test -tags integration -run TestGolden -count=1 -timeout 5m ./test/oracle/contract/...

package contract_test

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/postfix/serena/test/harness"
)

// normalizeResponse replaces non-deterministic values with stable placeholders (D-04).
func normalizeResponse(text string, workspaceDir string) string {
	// Replace workspace dir first (most specific).
	if workspaceDir != "" {
		text = strings.ReplaceAll(text, workspaceDir, "<WORKSPACE>")
	}
	// Normalize absolute paths containing /testdata/fixtures/.
	text = regexp.MustCompile(`/[^\s"]+/testdata/fixtures/`).ReplaceAllString(text, "<FIXTURE_ROOT>/")
	// Normalize /tmp/ and /var/folders/ paths.
	text = regexp.MustCompile(`(?:/tmp|/var/folders)/[^\s"]+`).ReplaceAllString(text, "<TMPDIR>")
	// Normalize ISO timestamps (e.g. 2026-04-11T19:33:13Z).
	text = regexp.MustCompile(`\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\s"]*`).ReplaceAllString(text, "<TIMESTAMP>")
	// Normalize short date-time in list_directory output (e.g. "2026-04-11 19:33").
	text = regexp.MustCompile(`\d{4}-\d{2}-\d{2}\s+\d{2}:\d{2}`).ReplaceAllString(text, "<DATETIME>")
	// Normalize session IDs with embedded timestamps (e.g. session-2026-04-11T19-33-13).
	text = regexp.MustCompile(`session-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}`).ReplaceAllString(text, "session-<TIMESTAMP>")
	// Normalize UUIDs.
	text = regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`).ReplaceAllString(text, "<UUID>")
	return text
}

// filterWorkspaceLines keeps only lines containing <WORKSPACE> from multi-line
// output. This strips non-deterministic stdlib results from search_symbols output
// that vary by Go version and platform, keeping only workspace-local results.
// Lines are sorted for stability.
func filterWorkspaceLines(text string) string {
	var kept []string
	for _, line := range strings.Split(text, "\n") {
		if strings.Contains(line, "<WORKSPACE>") {
			kept = append(kept, line)
		}
	}
	sort.Strings(kept)
	if len(kept) == 0 {
		return text // fallback: return original if no workspace lines
	}
	return strings.Join(kept, "\n") + "\n"
}

// goldenDir returns the root directory for golden output files (D-05).
func goldenDir() string {
	return filepath.Join(harness.ProjectRoot(), "test", "oracle", "contract", "testdata", "golden")
}

// goldenCase describes one tool invocation for golden output capture.
type goldenCase struct {
	tool             string
	args             map[string]any
	needsWorkspace   bool   // true if tool requires activate_project
	needsLS          bool   // true if tool requires running language server
	needsMemory      string // if non-empty, write this memory name before calling
	workspaceOnly    bool   // if true, filter output to workspace-local lines only
}

// noWorkspaceCases returns tools that need no workspace (memory tools, onboarding tools).
func noWorkspaceCases() []goldenCase {
	return []goldenCase{
		{tool: "list_memories", args: map[string]any{}},
		{tool: "write_memory", args: map[string]any{"name": "golden-test", "content": "test content for golden"}},
		{tool: "read_memory", args: map[string]any{"name": "golden-test"}, needsMemory: "golden-test"},
		{tool: "search_memories", args: map[string]any{"query": "golden"}, needsMemory: "golden-test"},
		{tool: "onboard_project", args: map[string]any{}},
		{tool: "prepare_for_new_conversation", args: map[string]any{}},
		{tool: "switch_mode", args: map[string]any{"target_mode": "read"}},
	}
}

// workspaceNoLSCases returns tools that need a workspace but not a language server.
func workspaceNoLSCases() []goldenCase {
	return []goldenCase{
		// activate_project is tested separately since it creates the workspace.
		{tool: "read_file", args: map[string]any{"path": "main.go"}, needsWorkspace: true},
		{tool: "list_directory", args: map[string]any{"path": "."}, needsWorkspace: true},
		{tool: "find_files", args: map[string]any{"pattern": "*.go"}, needsWorkspace: true},
		{tool: "search_in_files", args: map[string]any{"pattern": "func"}, needsWorkspace: true},
	}
}

// workspaceLSCases returns tools that need a running language server.
// Line/column values are 0-indexed per the tool schemas.
// Line 4, col 5 targets "main" in "func main() {" (fixture main.go).
func workspaceLSCases() []goldenCase {
	return []goldenCase{
		// search_symbols returns stdlib results that vary by Go version; keep workspace-local only.
		{tool: "search_symbols", args: map[string]any{"query": "main"}, needsWorkspace: true, needsLS: true, workspaceOnly: true},
		{tool: "get_symbol_overview", args: map[string]any{"path": "main.go"}, needsWorkspace: true, needsLS: true},
		{tool: "get_hover_info", args: map[string]any{"path": "main.go", "line": 4, "column": 5}, needsWorkspace: true, needsLS: true},
		{tool: "go_to_definition", args: map[string]any{"path": "main.go", "line": 4, "column": 5}, needsWorkspace: true, needsLS: true},
		{tool: "find_references", args: map[string]any{"path": "main.go", "line": 4, "column": 5}, needsWorkspace: true, needsLS: true},
		{tool: "get_call_hierarchy", args: map[string]any{"path": "main.go", "line": 4, "column": 5}, needsWorkspace: true, needsLS: true},
		// get_type_hierarchy and find_implementations need a type, not a function.
		// Line 15, col 5 targets DemoStruct in the fixture (0-indexed).
		{tool: "get_type_hierarchy", args: map[string]any{"path": "main.go", "line": 15, "column": 5}, needsWorkspace: true, needsLS: true},
		{tool: "find_implementations", args: map[string]any{"path": "main.go", "line": 15, "column": 5}, needsWorkspace: true, needsLS: true},
		{tool: "get_diagnostics", args: map[string]any{"path": "main.go"}, needsWorkspace: true, needsLS: true},
		{tool: "format_code", args: map[string]any{"path": "main.go"}, needsWorkspace: true, needsLS: true},
		// get_code_actions uses 1-indexed line/column. Line 5, col 5 targets "main".
		{tool: "get_code_actions", args: map[string]any{"path": "main.go", "line": 5, "column": 5}, needsWorkspace: true, needsLS: true},
	}
}

// TestGolden_ToolOutputs captures golden output for every accessible MCP tool (CONT-01).
// Tools are grouped by their requirements: no-workspace, workspace-no-LS, workspace-LS.
// LS-dependent tools skip cleanly when gopls is unavailable.
func TestGolden_ToolOutputs(t *testing.T) {
	gDir := goldenDir()

	// --- Group 1: No-workspace tools ---
	t.Run("no_workspace", func(t *testing.T) {
		runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

		// Pre-write memory for tools that need it.
		harness.CallTool(t, runner.Session, "write_memory", map[string]any{
			"name": "golden-test", "content": "test content for golden",
		})

		for _, tc := range noWorkspaceCases() {
			tc := tc
			t.Run(tc.tool, func(t *testing.T) {
				result := harness.CallTool(t, runner.Session, tc.tool, tc.args)
				text := harness.TextContent(result)
				normalized := normalizeResponse(text, "")
				goldenPath := filepath.Join(gDir, tc.tool, "success.golden")
				harness.AssertGolden(t, goldenPath, []byte(normalized))
			})
		}
	})

	// --- Group 2: Workspace tools (no LS) ---
	t.Run("workspace_no_ls", func(t *testing.T) {
		fixtureDir := harness.PrepareFixture(t, "go")
		runner := harness.StartRunner(t, harness.RunnerOptions{
			WorkspaceDir: fixtureDir,
			SkipLS:       true,
		})

		// Test activate_project separately (it activates the workspace itself).
		t.Run("activate_project", func(t *testing.T) {
			// Runner already activated the workspace; call it again to get the output.
			result := harness.CallTool(t, runner.Session, "activate_project", map[string]any{
				"repo_path": fixtureDir,
			})
			text := harness.TextContent(result)
			normalized := normalizeResponse(text, fixtureDir)
			goldenPath := filepath.Join(gDir, "activate_project", "success.golden")
			harness.AssertGolden(t, goldenPath, []byte(normalized))
		})

		for _, tc := range workspaceNoLSCases() {
			tc := tc
			t.Run(tc.tool, func(t *testing.T) {
				result := harness.CallTool(t, runner.Session, tc.tool, tc.args)
				text := harness.TextContent(result)
				normalized := normalizeResponse(text, fixtureDir)
				goldenPath := filepath.Join(gDir, tc.tool, "success.golden")
				harness.AssertGolden(t, goldenPath, []byte(normalized))
			})
		}
	})

	// --- Group 3: Workspace tools (with LS) ---
	t.Run("with_ls", func(t *testing.T) {
		harness.RequireGopls(t)

		fixtureDir := harness.PrepareFixture(t, "go")
		runner := harness.StartRunner(t, harness.RunnerOptions{
			WorkspaceDir: fixtureDir,
		})

		for _, tc := range workspaceLSCases() {
			tc := tc
			t.Run(tc.tool, func(t *testing.T) {
				result := harness.CallTool(t, runner.Session, tc.tool, tc.args)
				text := harness.TextContent(result)
				normalized := normalizeResponse(text, fixtureDir)
				if tc.workspaceOnly {
					normalized = filterWorkspaceLines(normalized)
				}
				goldenPath := filepath.Join(gDir, tc.tool, "success.golden")
				harness.AssertGolden(t, goldenPath, []byte(normalized))
			})
		}
	})
}
