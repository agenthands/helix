package cli

import "testing"

// TestRenderClassFor_KnownTools pins the render class of the load-bearing tools
// that the Phase 92 terse renderer treats specially. The classification rules
// are anchored in the daemon-side formatters:
//   - locus-list: handlers that emit flat "file://…:L:C" lines via
//     formatLocations (internal/kernel/symbols/tools.go) or the fileops
//     "relpath:L: text" form.
//   - tree: handlers that emit an indented shape via formatOutline /
//     formatHierarchy (shape-only passthrough per OUT-03).
//   - opaque: markdown / JSON-envelope / no-locus passthrough (Pitfall 2/3).
func TestRenderClassFor_KnownTools(t *testing.T) {
	cases := []struct {
		tool string
		want renderClass
	}{
		// locus-list: formatLocations emitters + fileops search.
		{"go_to_definition", classLocusList},
		{"find_references", classLocusList},
		{"find_implementations", classLocusList},
		{"search_symbols", classLocusList},
		{"search_in_files", classLocusList},
		// tree: outline + hierarchy emit an indented "Kind Name (…)" shape,
		// NOT flat loci (formatHierarchy at tools.go:216-227), so they are
		// shape-only passthrough per OUT-03.
		{"get_symbol_overview", classTree},
		{"get_call_hierarchy", classTree},
		{"get_type_hierarchy", classTree},
		// opaque: markdown / JSON-envelope / no-locus passthrough.
		{"get_hover_info", classOpaque},
		{"get_repo_map", classOpaque},
		{"get_context", classOpaque},
		{"analyze_blast_radius", classOpaque},
		{"write_memory", classOpaque},
	}
	for _, c := range cases {
		if got := renderClassFor(c.tool); got != c.want {
			t.Errorf("renderClassFor(%q) = %v, want %v", c.tool, got, c.want)
		}
	}
}

// TestRenderClassFor_UnknownDefaultsOpaque proves the defensive default:
// an unknown tool name never panics and never drops output — it falls back
// to classOpaque (Security V5: never drop output).
func TestRenderClassFor_UnknownDefaultsOpaque(t *testing.T) {
	if got := renderClassFor("this_tool_does_not_exist"); got != classOpaque {
		t.Errorf("renderClassFor(unknown) = %v, want classOpaque", got)
	}
	if got := renderClassFor(""); got != classOpaque {
		t.Errorf("renderClassFor(empty) = %v, want classOpaque", got)
	}
}

// TestRenderClass_CoverageOfAllVerbs iterates the generated verbSpecs catalog
// directly (not a hardcoded count) so a future-added verb fails this test until
// it is explicitly classified in renderClassByTool. Every one of the generated
// verbs MUST have an entry — the renderer must never encounter an unclassified
// tool name.
func TestRenderClass_CoverageOfAllVerbs(t *testing.T) {
	for verb, spec := range verbSpecs {
		if _, ok := renderClassByTool[spec.toolName]; !ok {
			t.Errorf("verb %q (toolName %q) is unclassified in renderClassByTool — add it to the render-class map", verb, spec.toolName)
		}
	}
}
