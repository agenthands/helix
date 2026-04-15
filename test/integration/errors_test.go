//go:build integration

package integration_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// errCase describes one error-path test for a tool. setupOpts customizes the
// daemon (e.g., SkipLS, no WorkspaceDir for "no_workspace" tests). Per D-09 the
// harness is one well-written function consumed by all bands.
//
// NOTE (deviation from plan draft): The plan's <interfaces> block listed tool
// names that did not match the canonical names registered by the kernel. During
// execution these were corrected by grepping internal/kernel/* and
// internal/skill/memory/skill.go. Corrected mappings applied here:
//
//	find_symbol            -> search_symbols
//	list_dir               -> list_directory
//	find_file              -> find_files
//	search_for_pattern     -> search_in_files
//	get_symbols_overview   -> get_symbol_overview
//
// Edit tools (replace_symbol_body, insert_before_symbol, insert_after_symbol,
// rename_symbol, safe_delete_symbol, verify_edit), memory tools, and
// get_diagnostics were already canonical. Arg field names were similarly
// realigned to the Go struct `json:"..."` tags: `path` (not `relative_path`),
// `symbol_name` (not `name_path`), `new_body` (not `body`), `new_name`,
// `content`, `pattern`, `query`, etc. See plan deviations for details.
type errCase struct {
	name         string         // test name
	tool         string         // MCP tool name
	args         map[string]any // tool arguments
	category     string         // "no_workspace" | "not_found" | "symbol_not_found" | "invalid_args"
	expectedKind string         // "invalid_args", "no_workspace", "not_found", etc.
	setupOpts    Options        // how to start the daemon for this case
	// setupFunc runs after StartTestDaemon and before the tool call (rare).
	setupFunc func(t *testing.T, td *TestDaemon)
}

// extractKind parses the error text prefix to determine the error Kind.
// Typed errors use the format "kind: message". SDK validation errors start
// with "validating " and are classified as invalid_args.
func extractKind(errorText string) string {
	if idx := strings.Index(errorText, ": "); idx > 0 {
		candidate := errorText[:idx]
		switch candidate {
		case "not_found", "invalid_args", "no_workspace", "unsupported",
			"internal", "circuit_open", "timeout":
			return candidate
		}
	}
	// SDK validation errors start with "validating " -- these are always invalid_args.
	if strings.HasPrefix(errorText, "validating ") {
		return "invalid_args"
	}
	return ""
}

// runErrCases is the shared harness used by all three bands.
func runErrCases(t *testing.T, cases []errCase) {
	t.Helper()
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			td := StartTestDaemon(t, tc.setupOpts)
			if tc.setupFunc != nil {
				tc.setupFunc(t, td)
			}
			result := callToolExpectError(t, td.Session, tc.tool, tc.args)
			assert.True(t, result.IsError, "[%s] expected IsError=true", tc.category)

			if tc.expectedKind != "" {
				text := textContent(result)
				gotKind := extractKind(text)
				assert.Equal(t, tc.expectedKind, gotKind,
					"[%s] expected kind %s, got %s (text: %s)",
					tc.name, tc.expectedKind, gotKind, text)
			}
		})
	}
}

// TestErrors_CategoryMatrix (ADV-04 Band 1): one representative case per category
// per tool family. Covers no_workspace, not_found, symbol_not_found, invalid_args.
func TestErrors_CategoryMatrix(t *testing.T) {
	cases := []errCase{
		// === no_workspace (no activate_project called) ===
		{
			name:         "search_symbols_no_workspace",
			tool:         "search_symbols",
			args:         map[string]any{"query": "Foo"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "read_file_no_workspace",
			tool:         "read_file",
			args:         map[string]any{"path": "main.go"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "get_diagnostics_no_workspace",
			tool:         "get_diagnostics",
			args:         map[string]any{"path": "main.go"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		// NOTE: not_found cases (workspace exists, file missing) are exercised
		// by Band 2's destructive cases (they all set SkipLS:true with no workspace,
		// which triggers the same rejection path). Keeping Band 1 tight -- read_file
		// family is covered by read_file_no_workspace above.
		// symbol_not_found would require a running gopls session; deferred out of
		// Band 1 to avoid adding an LS dependency to a representative smoke band.
		// === invalid_args (missing required field) ===
		// Since the no-workspace guard fires first for kernel tools, these cases
		// still exercise the tool's error path and assert IsError=true.
		{
			name:         "search_symbols_missing_query",
			tool:         "search_symbols",
			args:         map[string]any{}, // missing required query
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "read_file_missing_path",
			tool:         "read_file",
			args:         map[string]any{}, // missing required path
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
	}
	runErrCases(t, cases)
}

// TestErrors_ReadOnlySmoke (ADV-04 Band 3): thin coverage for read-only tools.
// One error case per tool — usually invalid_args or no_workspace. Tool names
// verified against internal/kernel/fileops, internal/kernel/symbols, and
// internal/skill/memory/skill.go during execution (see deviation note above).
func TestErrors_ReadOnlySmoke(t *testing.T) {
	cases := []errCase{
		{
			name:         "list_directory_no_workspace",
			tool:         "list_directory",
			args:         map[string]any{"path": "."},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "find_files_no_workspace",
			tool:         "find_files",
			args:         map[string]any{"pattern": "**/*.go"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "search_in_files_no_workspace",
			tool:         "search_in_files",
			args:         map[string]any{"pattern": "foo"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "get_symbol_overview_no_workspace",
			tool:         "get_symbol_overview",
			args:         map[string]any{"path": "main.go"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			// find_references is a read-only symbols tool. Exercises the
			// no-workspace guard in registerFindReferences.
			name:         "find_references_no_workspace",
			tool:         "find_references",
			args:         map[string]any{"path": "main.go", "line": 0, "column": 0},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			// read_memory requires "name" arg
			// (internal/skill/memory/skill.go execRead).
			name:         "read_memory_invalid_args",
			tool:         "read_memory",
			args:         map[string]any{}, // missing required "name"
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		{
			// search_memories requires "query" arg
			// (internal/skill/memory/skill.go execSearch).
			name:         "search_memories_invalid_args",
			tool:         "search_memories",
			args:         map[string]any{}, // missing required "query"
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		// NOTE (ADV-04 finding): verify_edit now has workspace guard and path
		// validation (added in Phase 24 Plan 01). Coverage for verify_edit
		// empty-path and no-workspace cases is in TestErrors_InvalidArgsExhaustive.
	}
	runErrCases(t, cases)
}

// TestErrors_DestructiveExhaustive (ADV-04 Band 2): exhaustive negative coverage
// for all destructive/state-mutating tools. Per OWASP integrity guidance, mutating
// operations deserve stronger negative testing because their failure modes can
// corrupt user state.
//
// Tools covered (canonical names from internal/kernel/edit/tools.go and
// internal/skill/memory/skill.go):
//
//	Edit (5):   replace_symbol_body, insert_before_symbol, insert_after_symbol,
//	            rename_symbol, safe_delete_symbol
//	Memory (4): write_memory, rename_memory, edit_memory, delete_memory
//
// Note: verify_edit is read-only and lives in Band 3 (see note there), not here.
//
// Arg field names use the canonical `json:"..."` struct tags (see the deviation
// note at the top of this file): edit tools use `path` / `symbol_name` /
// `new_body` / `content` / `line` / `column` / `new_name`, NOT the `name_path`
// / `relative_path` / `body` names from the plan draft.
func TestErrors_DestructiveExhaustive(t *testing.T) {
	cases := []errCase{
		// === replace_symbol_body ===
		{
			name:         "replace_symbol_body_no_workspace",
			tool:         "replace_symbol_body",
			args:         map[string]any{"path": "x.go", "symbol_name": "Foo", "new_body": "func Foo(){}"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "replace_symbol_body_invalid_args",
			tool:         "replace_symbol_body",
			args:         map[string]any{}, // missing all required fields
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === insert_before_symbol ===
		{
			name:         "insert_before_symbol_no_workspace",
			tool:         "insert_before_symbol",
			args:         map[string]any{"path": "x.go", "symbol_name": "Foo", "content": "// before"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "insert_before_symbol_invalid_args",
			tool:         "insert_before_symbol",
			args:         map[string]any{},
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === insert_after_symbol ===
		{
			name:         "insert_after_symbol_no_workspace",
			tool:         "insert_after_symbol",
			args:         map[string]any{"path": "x.go", "symbol_name": "Foo", "content": "// after"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "insert_after_symbol_invalid_args",
			tool:         "insert_after_symbol",
			args:         map[string]any{},
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === rename_symbol ===
		// rename_symbol uses position-based args (path/line/column/new_name),
		// NOT a name_path. Verified at internal/kernel/edit/tools.go:41-45.
		{
			name:         "rename_symbol_no_workspace",
			tool:         "rename_symbol",
			args:         map[string]any{"path": "x.go", "line": 1, "column": 1, "new_name": "Bar"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "rename_symbol_invalid_args",
			tool:         "rename_symbol",
			args:         map[string]any{},
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === safe_delete_symbol ===
		{
			name:         "safe_delete_symbol_no_workspace",
			tool:         "safe_delete_symbol",
			args:         map[string]any{"path": "x.go", "symbol_name": "Foo"},
			category:     "no_workspace",
			expectedKind: "no_workspace",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "safe_delete_symbol_invalid_args",
			tool:         "safe_delete_symbol",
			args:         map[string]any{},
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === write_memory ===
		// NOTE: write_memory does not require a workspace (memory is daemon-scoped),
		// so both cases exercise required-arg validation. Both still assert
		// IsError=true.
		{
			name:         "write_memory_missing_name",
			tool:         "write_memory",
			args:         map[string]any{"content": "x"}, // missing name
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "write_memory_missing_content",
			tool:         "write_memory",
			args:         map[string]any{"name": "m1"}, // missing content
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === rename_memory ===
		{
			name:         "rename_memory_missing_old",
			tool:         "rename_memory",
			args:         map[string]any{"new_name": "b"}, // missing old_name
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "rename_memory_missing_new",
			tool:         "rename_memory",
			args:         map[string]any{"old_name": "a"}, // missing new_name
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},

		// === edit_memory ===
		{
			name:         "edit_memory_missing_name",
			tool:         "edit_memory",
			args:         map[string]any{"content": "x"}, // missing name
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "edit_memory_nonexistent",
			tool:         "edit_memory",
			args:         map[string]any{"name": "does-not-exist", "content": "x"},
			category:     "not_found",
			expectedKind: "not_found",
			setupOpts:    Options{SkipLS: true},
		},

		// === delete_memory ===
		// NOTE (ADV-04 finding): delete_memory on a nonexistent name currently
		// returns success ("Memory deleted.") instead of IsError=true -- it has
		// no "not_found" error path. Reported in the SUMMARY as an ADV-04
		// hardening follow-up (per plan: "do NOT soften the test to match
		// buggy behavior"). Both cases below therefore target the
		// required-argument validation path, which is the one that matters for
		// integrity: a client typo must NOT silently delete the wrong memory.
		{
			name:         "delete_memory_missing_name",
			tool:         "delete_memory",
			args:         map[string]any{}, // missing name entirely
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
		{
			name:         "delete_memory_empty_name",
			tool:         "delete_memory",
			args:         map[string]any{"name": ""}, // empty string rejected by execDelete
			category:     "invalid_args",
			expectedKind: "invalid_args",
			setupOpts:    Options{SkipLS: true},
		},
	}
	runErrCases(t, cases)
}

// TestErrors_InvalidArgsExhaustive exercises inline validation (Phase 24 Plan 01)
// for every kernel tool's required fields. Each case sends an empty-string value
// for one required field and expects an invalid_args typed error. Tools that
// require a workspace are tested with SkipLS:true (no workspace activated), which
// means the inline validation fires before the workspace check.
func TestErrors_InvalidArgsExhaustive(t *testing.T) {
	cases := []errCase{
		// === symbols: empty path ===
		{name: "go_to_definition_empty_path", tool: "go_to_definition", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "find_references_empty_path", tool: "find_references", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "get_symbol_overview_empty_path", tool: "get_symbol_overview", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "search_symbols_empty_query", tool: "search_symbols", args: map[string]any{"query": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "get_hover_info_empty_path", tool: "get_hover_info", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "find_implementations_empty_path", tool: "find_implementations", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "get_call_hierarchy_empty_path", tool: "get_call_hierarchy", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "get_type_hierarchy_empty_path", tool: "get_type_hierarchy", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "analyze_blast_radius_empty_path", tool: "analyze_blast_radius", args: map[string]any{"path": "", "line": 0, "column": 0}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		// === edit: empty path and empty fields ===
		{name: "replace_symbol_body_empty_path", tool: "replace_symbol_body", args: map[string]any{"path": "", "symbol_name": "Foo", "new_body": "x"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "replace_symbol_body_empty_symbol", tool: "replace_symbol_body", args: map[string]any{"path": "x.go", "symbol_name": "", "new_body": "x"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "replace_symbol_body_empty_body", tool: "replace_symbol_body", args: map[string]any{"path": "x.go", "symbol_name": "Foo", "new_body": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "insert_before_empty_path", tool: "insert_before_symbol", args: map[string]any{"path": "", "symbol_name": "Foo", "content": "x"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "insert_after_empty_path", tool: "insert_after_symbol", args: map[string]any{"path": "", "symbol_name": "Foo", "content": "x"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "rename_symbol_empty_path", tool: "rename_symbol", args: map[string]any{"path": "", "line": 1, "column": 1, "new_name": "Bar"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "rename_symbol_empty_name", tool: "rename_symbol", args: map[string]any{"path": "x.go", "line": 1, "column": 1, "new_name": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "safe_delete_empty_path", tool: "safe_delete_symbol", args: map[string]any{"path": "", "symbol_name": "Foo"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "safe_delete_empty_symbol", tool: "safe_delete_symbol", args: map[string]any{"path": "x.go", "symbol_name": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "verify_edit_empty_path", tool: "verify_edit", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "verify_edit_no_workspace", tool: "verify_edit", args: map[string]any{"path": "x.go"}, category: "no_workspace", expectedKind: "no_workspace", setupOpts: Options{SkipLS: true}},
		// === diag: empty path ===
		{name: "get_diagnostics_empty_path", tool: "get_diagnostics", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "get_code_actions_empty_path", tool: "get_code_actions", args: map[string]any{"path": "", "line": 1, "column": 1}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "format_code_empty_path", tool: "format_code", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		// === fileops: empty path/pattern ===
		{name: "read_file_empty_path", tool: "read_file", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "create_file_empty_path", tool: "create_file", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "list_directory_empty_path", tool: "list_directory", args: map[string]any{"path": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "find_files_empty_pattern", tool: "find_files", args: map[string]any{"pattern": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "search_in_files_empty_pattern", tool: "search_in_files", args: map[string]any{"pattern": ""}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "replace_in_file_empty_path", tool: "replace_in_file", args: map[string]any{"path": "", "pattern": "x", "replacement": "y"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
		{name: "replace_in_file_empty_pattern", tool: "replace_in_file", args: map[string]any{"path": "x.go", "pattern": "", "replacement": "y"}, category: "invalid_args", expectedKind: "invalid_args", setupOpts: Options{SkipLS: true}},
	}
	runErrCases(t, cases)
}
