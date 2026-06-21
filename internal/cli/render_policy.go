package cli

// render_policy.go defines the per-tool render-class taxonomy the Phase 92
// terse renderer dispatches on. The map is keyed by the underlying TOOL NAME
// (the `toolName` field in verbSpecs, NOT the kebab verb name). The coverage
// test in render_policy_test.go iterates verbSpecs and enforces that EVERY
// generated verb has an entry here, so a newly-generated verb cannot ship
// unclassified.
//
// Classification is anchored in the daemon-side formatters
// (internal/kernel/symbols/tools.go, internal/kernel/fileops/*):
//
//   - classLocusList: handlers that emit flat "file://<abs>:L:C [— payload]"
//     lines via formatLocations (go_to_definition, find_references,
//     find_implementations, search_symbols) or the fileops "relpath:L: text"
//     form (search_in_files). These get the relpath:line:col<TAB>payload
//     terse treatment (sort + dedup, OUT-01/02).
//
//   - classTree: handlers whose output is an indented shape, NOT flat loci —
//     formatOutline (get_symbol_overview) and formatHierarchy
//     (get_call_hierarchy, get_type_hierarchy; tools.go:216-227 emits
//     "Kind Name (URI)" trees, not loci) — plus directory/file listings
//     (list_directory, find_files). Per OUT-03 these print shape only and are
//     passed through verbatim.
//
//   - classOpaque: markdown (get_hover_info), JSON envelopes
//     (analyze_blast_radius, tools.go:681), repo/context maps, and every
//     memory / workflow / diagnostics / repomap / edit tool — passthrough.
//     classOpaque is also the SAFE DEFAULT for any unknown tool name so the
//     renderer never panics and never drops output (Security V5).

// renderClass enumerates how a tool's daemon text is rendered CLI-side.
// classOpaque is the zero value so an absent / unknown tool defaults to
// passthrough.
type renderClass int

const (
	// classOpaque passes daemon text through verbatim. It is the zero value
	// and the safe default for unknown tool names.
	classOpaque renderClass = iota
	// classLocusList re-renders "file://…:L:C — payload" lines as terse
	// relpath:line:col<TAB>payload, sorted and deduped.
	classLocusList
	// classTree passes shape-only output (outline / hierarchy / listing)
	// through verbatim (OUT-03).
	classTree
)

// renderClassByTool maps every callable tool name to its render class. Keyed by
// the toolName field in verbSpecs. The coverage test enforces completeness.
var renderClassByTool = map[string]renderClass{
	// --- locus-list: formatLocations emitters + fileops search ---
	"go_to_definition":     classLocusList,
	"find_references":      classLocusList,
	"find_implementations": classLocusList,
	"search_symbols":       classLocusList,
	"search_in_files":      classLocusList,

	// --- tree: outline / hierarchy / listing (shape-only, OUT-03) ---
	"get_symbol_overview": classTree,
	"get_call_hierarchy":  classTree,
	"get_type_hierarchy":  classTree,
	"list_directory":      classTree,
	"find_files":          classTree,

	// --- opaque: navigation passthrough (markdown / JSON envelope) ---
	"get_hover_info":       classOpaque,
	"analyze_blast_radius": classOpaque,

	// --- opaque: repomap (maps, graphs, clusters — not loci) ---
	"get_context":               classOpaque,
	"get_repo_map":              classOpaque,
	"get_semantic_context":      classOpaque,
	"get_cluster_map":           classOpaque,
	"explain_cluster":           classOpaque,
	"explain_symbol_deep":       classOpaque,
	"find_related_symbols":      classOpaque,
	"get_change_impact_graph":   classOpaque,
	"get_semantic_graph_status": classOpaque,
	"index_semantic_graph":      classOpaque,
	"refresh_semantic_graph":    classOpaque,
	"validate_graph_edge":       classOpaque,

	// --- opaque: fileops (content / mutation — not loci) ---
	"read_file":       classOpaque,
	"create_file":     classOpaque,
	"replace_in_file": classOpaque,
	"fuzzy_edit":      classOpaque,

	// --- opaque: edit (mutation receipts / status) ---
	"insert_after_symbol":  classOpaque,
	"insert_before_symbol": classOpaque,
	"rename_symbol":        classOpaque,
	"replace_symbol_body":  classOpaque,
	"safe_delete_symbol":   classOpaque,
	"verify_edit":          classOpaque,

	// --- opaque: diagnostics ---
	"get_diagnostics":  classOpaque,
	"get_code_actions": classOpaque,
	"format_code":      classOpaque,

	// --- opaque: memory / workflow / introspection ---
	"write_memory":                 classOpaque,
	"read_memory":                  classOpaque,
	"edit_memory":                  classOpaque,
	"delete_memory":                classOpaque,
	"rename_memory":                classOpaque,
	"list_memories":                classOpaque,
	"search_memories":              classOpaque,
	"onboard_project":              classOpaque,
	"prepare_for_new_conversation": classOpaque,
	"switch_mode":                  classOpaque,
	"get_health":                   classOpaque,
	"get_tool_help":                classOpaque,
	"get_token_budget":             classOpaque,
}

// renderClassFor returns the render class for a tool name, defaulting to
// classOpaque for any name absent from the map (defensive — never drop output,
// never panic; Security V5).
func renderClassFor(toolName string) renderClass {
	if c, ok := renderClassByTool[toolName]; ok {
		return c
	}
	return classOpaque
}
