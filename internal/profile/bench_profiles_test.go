package profile

import (
	"testing"

	"github.com/agenthands/helix/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Blank imports trigger skill.Register() via init() (Caddy-style), exactly as
	// the daemon does in internal/daemon/imports.go. Without these, the global
	// skill registry is empty and skill.ResolveTools(...) returns no tools, so the
	// golden tool-surface assertions below would be vacuous. Importing the skill
	// packages here populates ToolProviders() so ResolveTools resolves the real
	// per-arm tool surface — the exact call the daemon makes at session start.
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/repomap"
	_ "github.com/agenthands/helix/internal/skill/semantic"
	_ "github.com/agenthands/helix/internal/skill/workflow"
)

// resolvedToolNames mirrors the daemon's session-start resolution
// (skill.ResolveTools(profile.Skills, profile.Tools, profile.ExcludeTools)) and
// returns the resolved tool names as a set for golden assertions.
func resolvedToolNames(t *testing.T, p *Profile) map[string]bool {
	t.Helper()
	resolved := skill.ResolveTools(p.Skills, p.Tools, p.ExcludeTools)
	names := make(map[string]bool, len(resolved))
	for _, td := range resolved {
		names[td.Name] = true
	}
	return names
}

// The 10 semantic-store-backed tools excluded by bench-no-semantic (RESEARCH Q4,
// D-11/D-12). get_repo_map/get_context are intentionally NOT in this list.
var semanticStoreTools = []string{
	"index_semantic_graph",
	"refresh_semantic_graph",
	"get_semantic_graph_status",
	"get_semantic_context",
	"explain_symbol_deep",
	"find_related_symbols",
	"validate_graph_edge",
	"get_cluster_map",
	"explain_cluster",
	"get_change_impact_graph",
}

// The LSP-backed symbol-retrieval + diagnostics tools excluded by bench-no-lsp
// (RESEARCH Q3, D-09).
var lspBackedTools = []string{
	"go_to_definition",
	"find_references",
	"get_symbol_overview",
	"search_symbols",
	"get_hover_info",
	"find_implementations",
	"get_call_hierarchy",
	"get_type_hierarchy",
	"get_diagnostics",
	"get_code_actions",
	"format_code",
}

// The 4 structured-edit tools excluded by bench-no-structured-edit (D-04).
var structuredEditTools = []string{
	"replace_symbol_body",
	"fuzzy_edit",
	"insert_before_symbol",
	"insert_after_symbol",
}

// TestBenchProfiles pins the resolved tool surface of each of the 4 bench
// ablation arms (golden tests) and verifies the per-arm kernel disable flags.
// Mitigates T-76-04 (under-excluded arm leaks a disabled-subsystem tool).
func TestBenchProfiles(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	t.Run("bench-full", func(t *testing.T) {
		p, ok := store.Profile("bench-full")
		require.True(t, ok, "bench-full profile must be loaded")

		names := resolvedToolNames(t, p)
		require.NotEmpty(t, names, "bench-full must expose a non-empty tool surface")

		// Control arm: full surface present — structured-edit, symbol, semantic tools.
		assert.True(t, names["replace_symbol_body"], "bench-full must include replace_symbol_body")
		assert.True(t, names["fuzzy_edit"], "bench-full must include fuzzy_edit")
		assert.True(t, names["go_to_definition"], "bench-full must include go_to_definition")
		assert.True(t, names["find_references"], "bench-full must include find_references")
		for _, tool := range semanticStoreTools {
			assert.True(t, names[tool], "bench-full must include semantic tool %q", tool)
		}

		// Control arm sets NEITHER disable flag (D-02).
		assert.False(t, p.DisableLSPSubsystem, "bench-full must NOT disable the LSP subsystem")
		assert.False(t, p.DisableStructuredEditSubsystem, "bench-full must NOT disable structured edits")
	})

	t.Run("bench-no-lsp", func(t *testing.T) {
		p, ok := store.Profile("bench-no-lsp")
		require.True(t, ok, "bench-no-lsp profile must be loaded")

		names := resolvedToolNames(t, p)
		require.NotEmpty(t, names, "bench-no-lsp must still expose a non-empty (non-LSP) surface")

		for _, tool := range lspBackedTools {
			assert.False(t, names[tool], "bench-no-lsp must EXCLUDE LSP-backed tool %q", tool)
		}
		// Tree-sitter repomap + file-ops remain available.
		assert.True(t, names["get_repo_map"], "bench-no-lsp must keep tree-sitter get_repo_map")
		assert.True(t, names["read_file"], "bench-no-lsp must keep file-ops read_file")

		assert.True(t, p.DisableLSPSubsystem, "bench-no-lsp must declare disable_lsp_subsystem: true")
		assert.False(t, p.DisableStructuredEditSubsystem, "bench-no-lsp must NOT disable structured edits")
	})

	t.Run("bench-no-semantic", func(t *testing.T) {
		p, ok := store.Profile("bench-no-semantic")
		require.True(t, ok, "bench-no-semantic profile must be loaded")

		names := resolvedToolNames(t, p)
		require.NotEmpty(t, names, "bench-no-semantic must expose a non-empty surface")

		for _, tool := range semanticStoreTools {
			assert.False(t, names[tool], "bench-no-semantic must EXCLUDE semantic-store tool %q", tool)
		}
		// get_repo_map/get_context REMAIN exposed; the Phase 81 kernel gate
		// forces their SemanticLookup reads to NoopLookup (tree-sitter fallback).
		assert.True(t, names["get_repo_map"], "bench-no-semantic must keep get_repo_map (D-11)")
		assert.True(t, names["get_context"], "bench-no-semantic must keep get_context (D-11)")
		// LSP + structured-edit tools remain (this arm only ablates semantic).
		assert.True(t, names["go_to_definition"], "bench-no-semantic keeps LSP tools")
		assert.True(t, names["replace_symbol_body"], "bench-no-semantic keeps structured edits")

		// Phase 81 ABLATE-06: the kernel gate has now LANDED — this arm declares
		// the semantic kernel flag (deliberate flip of the Phase 76 D-11/D-12
		// deferral). It still ablates ONLY semantic, so LSP/structured-edit stay false.
		assert.True(t, p.DisableSemanticSubsystem, "bench-no-semantic must declare disable_semantic_subsystem: true")
		assert.False(t, p.DisableLSPSubsystem, "bench-no-semantic must NOT disable the LSP subsystem")
		assert.False(t, p.DisableStructuredEditSubsystem, "bench-no-semantic must NOT disable structured edits")
	})

	t.Run("bench-no-structured-edit", func(t *testing.T) {
		p, ok := store.Profile("bench-no-structured-edit")
		require.True(t, ok, "bench-no-structured-edit profile must be loaded")

		names := resolvedToolNames(t, p)
		require.NotEmpty(t, names, "bench-no-structured-edit must expose a non-empty surface")

		for _, tool := range structuredEditTools {
			assert.False(t, names[tool], "bench-no-structured-edit must EXCLUDE structured-edit tool %q", tool)
		}
		// Plain replace_in_file REMAINS (the agent's only edit primitive, ABLATE-07).
		assert.True(t, names["replace_in_file"], "bench-no-structured-edit must keep replace_in_file")

		assert.True(t, p.DisableStructuredEditSubsystem, "bench-no-structured-edit must declare disable_structured_edit_subsystem: true")
		assert.False(t, p.DisableLSPSubsystem, "bench-no-structured-edit must NOT disable the LSP subsystem")
	})
}
