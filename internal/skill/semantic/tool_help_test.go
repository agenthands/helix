// Phase 73 Plan 73-03 Task 3: get_tool_help param-doc coverage for the 6 P1 tools.
//
// Asserts that each P1 tool's typed-args struct has jsonschema field tags
// that produce >= 1 ParamDoc with a non-empty Description when passed
// through help.ExtractParamDocs (SC#2 regression guard for D-03).
//
// Uses jsonschema.For[T] — the same generator the register* funcs use at
// daemon registration time — so the extraction path is byte-for-byte
// identical to what the running daemon feeds into ExtractParamDocs.
//
// One t.Run per tool (Go generics are not table-friendly with type
// parameters driven by a value table).

package semantic

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"

	"github.com/agenthands/helix/internal/kernel/help"
)

// schemaForP1 generates the JSON schema for a typed-args struct T using the
// same generator the MCP server uses at tool registration time.
func schemaForP1[T any](t *testing.T) any {
	t.Helper()
	s, err := jsonschema.For[T](nil)
	if err != nil {
		t.Fatalf("jsonschema.For[%T]: %v", *new(T), err)
	}
	return s
}

// assertParamDocs verifies that docs is non-empty and at least one entry has
// a non-empty Description.
func assertParamDocs(t *testing.T, toolName string, docs []help.ParamDoc) {
	t.Helper()
	if len(docs) == 0 {
		t.Errorf("%s: ExtractParamDocs returned empty docs — jsonschema tags may be missing", toolName)
		return
	}
	for _, d := range docs {
		if d.Description != "" {
			return
		}
	}
	t.Errorf("%s: all %d ParamDocs have empty Description — jsonschema tag values may be blank", toolName, len(docs))
}

// TestToolHelp_P1_ParamDocCoverage asserts that all 6 P1 tools produce
// non-empty parameter documentation via help.ExtractParamDocs.
func TestToolHelp_P1_ParamDocCoverage(t *testing.T) {
	t.Run("explain_symbol_deep", func(t *testing.T) {
		docs := help.ExtractParamDocs(schemaForP1[ExplainSymbolDeepArgs](t))
		assertParamDocs(t, "explain_symbol_deep", docs)
	})

	t.Run("find_related_symbols", func(t *testing.T) {
		docs := help.ExtractParamDocs(schemaForP1[FindRelatedSymbolsArgs](t))
		assertParamDocs(t, "find_related_symbols", docs)
	})

	t.Run("validate_graph_edge", func(t *testing.T) {
		docs := help.ExtractParamDocs(schemaForP1[ValidateGraphEdgeArgs](t))
		assertParamDocs(t, "validate_graph_edge", docs)
	})

	t.Run("get_cluster_map", func(t *testing.T) {
		docs := help.ExtractParamDocs(schemaForP1[GetClusterMapArgs](t))
		assertParamDocs(t, "get_cluster_map", docs)
	})

	t.Run("explain_cluster", func(t *testing.T) {
		docs := help.ExtractParamDocs(schemaForP1[ExplainClusterArgs](t))
		assertParamDocs(t, "explain_cluster", docs)
	})

	t.Run("get_change_impact_graph", func(t *testing.T) {
		docs := help.ExtractParamDocs(schemaForP1[GetChangeImpactGraphArgs](t))
		assertParamDocs(t, "get_change_impact_graph", docs)
	})
}
