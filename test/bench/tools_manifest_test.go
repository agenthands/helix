package bench_test

// tools_manifest.go enumerates the 53-tool bench manifest locked by D-04.
//
// Every entry is a benchCase with:
//   - name: exact MCP tool name (canonical source: internal/daemon/bootstrap_test.go)
//   - args: a map[string]any tuned to work against testdata/fixtures/go/
//   - needsCopy: true when the tool mutates files, signalling Plan 09-03 that
//     the sub-benchmark must use prepareGoFixtureCopyB(b) instead of
//     prepareGoFixtureB(b) to avoid corrupting the shared read-only fixture
//
// The total count MUST equal 53 — enforced by TestBenchToolsManifestMatchesRegistry
// in main_test.go, which also asserts bidirectional name parity with the live
// MCP registry (no manifest-only names, no registry-only names).
//
// Breakdown (9 + 6 + 7 + 3 + 7 + 2 + 2 + 2 + 3 + 1 + 1 + 4 + 6 = 53):
//   - Symbol (9): 09-RESEARCH.md Pattern 1 symbol retrieval tools
//   - Edit (6): mutating tools — Plan 09-03 must use prepareGoFixtureCopyB
//   - File ops (7): read/list/find/search/create/replace/fuzzy_edit
//   - Diagnostics (3): diagnostics, code actions, formatting
//   - Memory (7): TestMain seeds "bench-manifest" memory for parity coverage
//   - Workflow (2): onboard_project, prepare_for_new_conversation
//   - Profile (2): switch_mode, get_token_budget
//   - RepoMap (2): get_repo_map, get_context
//   - Built-in (3): ping, echo, activate_project
//   - Health (1): get_health
//   - Help (1): get_tool_help
//   - Semantic graph (4, Phase 64): index/refresh/status/context
//   - Semantic deep-context (6, Phase 65): cluster map/explain, symbol deep,
//     related symbols, change-impact graph, validate edge
//
// Hardcoded offsets in testdata/fixtures/go/main.go (verified at plan time):
//
//	line 11 col 6 -> func Helper    (go_to_definition, find_references target)
//	line 16 col 6 -> type DemoStruct
//	line 21 col 13 -> method Value on *DemoStruct
//
// Symbol tools use 0-indexed line/column per internal/kernel/symbols/tools.go
// jsonschema tags. Edit tools (rename_symbol) use 1-indexed per
// internal/kernel/edit/tools.go jsonschema tags.

// benchCase describes a single tool invocation for the bench manifest.
type benchCase struct {
	name      string
	args      map[string]any
	needsCopy bool // true for edit tools that mutate files
}

// benchMemoryName is the memory seeded by TestMain so that read_memory,
// search_memories, rename_memory, edit_memory, and delete_memory all have
// something to operate on when their sub-benches run.
const benchMemoryName = "bench-manifest"

// benchTools is the canonical 43-tool manifest. Count and names are locked by
// D-04 and asserted against the live registry in
// TestBenchToolsManifestMatchesRegistry.
var benchTools = []benchCase{
	// === Symbol (9) ==========================================================
	// Target: Helper at main.go line 11 col 6 (0-indexed: line 10 col 5)
	{name: "go_to_definition", args: map[string]any{
		"path":   "main.go",
		"line":   6, // main() body reference to Helper() at line 7 col 2 (0-indexed 6/1)
		"column": 1,
	}},
	{name: "find_references", args: map[string]any{
		"path":   "main.go",
		"line":   10, // 0-indexed line of `func Helper()`
		"column": 5,
	}},
	{name: "get_symbol_overview", args: map[string]any{
		"path": "main.go",
	}},
	{name: "search_symbols", args: map[string]any{
		"query": "Helper",
	}},
	{name: "get_hover_info", args: map[string]any{
		"path":   "main.go",
		"line":   10,
		"column": 5,
	}},
	{name: "find_implementations", args: map[string]any{
		"path":   "main.go",
		"line":   15, // DemoStruct type declaration
		"column": 5,
	}},
	{name: "get_call_hierarchy", args: map[string]any{
		"path":      "main.go",
		"line":      10,
		"column":    5,
		"direction": "both",
	}},
	{name: "get_type_hierarchy", args: map[string]any{
		"path":      "main.go",
		"line":      15,
		"column":    5,
		"direction": "both",
	}},
	{name: "analyze_blast_radius", args: map[string]any{
		"path":   "main.go",
		"line":   10,
		"column": 5,
	}},

	// === Edit (6) ============================================================
	// ALL edit tools MUTATE files — needsCopy=true. Plan 09-03 must call
	// prepareGoFixtureCopyB(b) for every edit sub-benchmark so the mutations
	// land in a tb.TempDir() copy, not the shared read-only fixture.
	{name: "replace_symbol_body", needsCopy: true, args: map[string]any{
		"path":        "main.go",
		"symbol_name": "UnusedFunc",
		"new_body":    "{\n\tfmt.Println(\"replaced\")\n}",
	}},
	{name: "insert_before_symbol", needsCopy: true, args: map[string]any{
		"path":        "main.go",
		"symbol_name": "UnusedFunc",
		"content":     "// Inserted before UnusedFunc.\n",
	}},
	{name: "insert_after_symbol", needsCopy: true, args: map[string]any{
		"path":        "main.go",
		"symbol_name": "UnusedFunc",
		"content":     "\n// Inserted after UnusedFunc.\n",
	}},
	{name: "rename_symbol", needsCopy: true, args: map[string]any{
		// rename_symbol uses 1-indexed line/column per edit/tools.go schema.
		"path":     "main.go",
		"line":     11,
		"column":   6,
		"new_name": "HelperRenamed",
	}},
	{name: "safe_delete_symbol", needsCopy: true, args: map[string]any{
		"path":        "main.go",
		"symbol_name": "UnusedFunc",
		"force":       true,
	}},
	{name: "verify_edit", needsCopy: true, args: map[string]any{
		"path": "main.go",
	}},

	// === File ops (6) ========================================================
	{name: "read_file", args: map[string]any{
		"path": "main.go",
	}},
	{name: "create_file", needsCopy: true, args: map[string]any{
		"path":    "bench_created.txt",
		"content": "bench manifest create_file invocation\n",
	}},
	{name: "list_directory", args: map[string]any{
		"path": ".",
	}},
	{name: "find_files", args: map[string]any{
		"pattern": "**/*.go",
	}},
	{name: "search_in_files", args: map[string]any{
		"pattern":      "Helper",
		"include_glob": "*.go",
		"max_results":  50,
	}},
	{name: "replace_in_file", needsCopy: true, args: map[string]any{
		"path":        "main.go",
		"pattern":     "Hello, Go!",
		"replacement": "Hello, Bench!",
	}},
	{name: "fuzzy_edit", needsCopy: true, args: map[string]any{
		"path":        "main.go",
		"search":      "Hello, Go!",
		"replacement": "Hello, Fuzzy!",
	}},

	// === Diagnostics (3) =====================================================
	{name: "get_diagnostics", args: map[string]any{
		"path": "main.go",
	}},
	{name: "get_code_actions", args: map[string]any{
		"path":   "main.go",
		"line":   11,
		"column": 6,
	}},
	{name: "format_code", needsCopy: true, args: map[string]any{
		"path":       "main.go",
		"tab_size":   4,
		"use_spaces": false,
	}},

	// === Memory (7) ==========================================================
	// TestMain seeds a benchMemoryName memory before sub-benches run so
	// read/search/rename/edit/delete have state to operate on.
	{name: "write_memory", args: map[string]any{
		"name":    benchMemoryName + "-w",
		"content": "bench manifest write_memory payload",
	}},
	{name: "read_memory", args: map[string]any{
		"name": benchMemoryName,
	}},
	{name: "list_memories", args: map[string]any{}},
	{name: "search_memories", args: map[string]any{
		"query": "manifest",
	}},
	{name: "rename_memory", args: map[string]any{
		"old_name": benchMemoryName + "-rename-src",
		"new_name": benchMemoryName + "-rename-dst",
	}},
	{name: "edit_memory", args: map[string]any{
		"name":    benchMemoryName,
		"search":  "payload",
		"replace": "PAYLOAD",
	}},
	{name: "delete_memory", args: map[string]any{
		"name": benchMemoryName + "-delete",
	}},

	// === Workflow (2) ========================================================
	{name: "onboard_project", args: map[string]any{}},
	{name: "prepare_for_new_conversation", args: map[string]any{}},

	// === Profile (2) =========================================================
	// switch_mode: "full" profile disallows read->read self-transitions; use
	// "edit" as a stable target that exists in the allowed-targets set.
	// Plan 09-03 fix: original "read" target triggered
	// "transition from \"read\" to \"read\" not allowed".
	{name: "switch_mode", args: map[string]any{
		"target_mode": "edit",
	}},
	{name: "get_token_budget", args: map[string]any{}},

	// === RepoMap (2) =========================================================
	{name: "get_repo_map", args: map[string]any{}},
	{name: "get_context", args: map[string]any{
		"files": []any{"main.go"},
	}},

	// === Built-in (3) ========================================================
	// ping takes a required Message arg (PingArgs in internal/mcp/server.go).
	// Plan 09-03 fix: original empty args triggered schema validation error.
	{name: "ping", args: map[string]any{
		"message": "bench",
	}},
	// echo takes Text, not Message (EchoArgs in internal/mcp/server.go).
	// Plan 09-03 fix: original "message" key was rejected as additional.
	{name: "echo", args: map[string]any{
		"text": "bench",
	}},
	{name: "activate_project", args: map[string]any{
		// repo_path is supplied at bench time by activateWorkspaceB; the
		// manifest entry is for parity counting only.
		"repo_path": ".",
	}},

	// === Health (1) ==========================================================
	{name: "get_health", args: map[string]any{}},

	// === Help (1) =============================================================
	{name: "get_tool_help", args: map[string]any{
		"tool_name": "ping",
	}},

	// === Semantic graph (4, Phase 64) ========================================
	{name: "index_semantic_graph", args: map[string]any{}},
	{name: "refresh_semantic_graph", args: map[string]any{}},
	{name: "get_semantic_graph_status", args: map[string]any{}},
	{name: "get_semantic_context", args: map[string]any{
		"query": "ping",
	}},

	// === Semantic deep-context (6, Phase 65) =================================
	// Read-only graph queries (no needsCopy). The manifest parity test compares
	// NAMES only and never invokes these via the manifest, so minimal sensible
	// args matching each tool's schema are sufficient.
	{name: "get_cluster_map", args: map[string]any{}},
	{name: "explain_cluster", args: map[string]any{
		"cluster_id": "c0",
	}},
	{name: "explain_symbol_deep", args: map[string]any{
		"seed": map[string]any{"file_path": "main.go", "symbol_name": "Helper"},
	}},
	{name: "find_related_symbols", args: map[string]any{
		"seed": map[string]any{"file_path": "main.go", "symbol_name": "Helper"},
	}},
	{name: "get_change_impact_graph", args: map[string]any{
		"seed": map[string]any{"file_path": "main.go", "symbol_name": "Helper"},
	}},
	{name: "validate_graph_edge", args: map[string]any{}},
}
