// export_p1_test.go — Phase 73-04 Task 2: test-only exports of the 6 P1
// handler unexported handler functions so the black-box `package semantic_test`
// E2E suite (p1_e2e_external_test.go) can invoke them without adding a new
// public method onto SemanticSkill.
//
// Pattern: standard Go `export_test.go` — file lives in `package semantic` and
// is compiled only for tests (the `_test.go` suffix), so the exported names are
// visible to the black-box `semantic_test` package without leaking into the
// production API. Mirrors export_status_test.go (HandleGetSemanticGraphStatusForTest
// / HandleIndexSemanticGraphForTest added in Phase 69-06).

package semantic

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// HandleExplainSymbolDeepForTest invokes the unexported handleExplainSymbolDeep
// handler. Phase 73-04 E2E test calls this against a SemanticSkill wired with
// a real *Store + bleve + populated graph via buildP1E2EFixture.
func HandleExplainSymbolDeepForTest(s *SemanticSkill, ctx context.Context, args ExplainSymbolDeepArgs) *mcpsdk.CallToolResult {
	return s.handleExplainSymbolDeep(ctx, args)
}

// HandleFindRelatedSymbolsForTest invokes the unexported handleFindRelatedSymbols
// handler. Phase 73-04 E2E test calls this against a SemanticSkill wired with
// a real *Store + bleve + populated graph via buildP1E2EFixture.
func HandleFindRelatedSymbolsForTest(s *SemanticSkill, ctx context.Context, args FindRelatedSymbolsArgs) *mcpsdk.CallToolResult {
	return s.handleFindRelatedSymbols(ctx, args)
}

// HandleValidateGraphEdgeForTest invokes the unexported handleValidateGraphEdge
// handler. Phase 73-04 E2E test calls this against a SemanticSkill wired with
// a real *Store + bleve + populated graph via buildP1E2EFixture.
func HandleValidateGraphEdgeForTest(s *SemanticSkill, ctx context.Context, args ValidateGraphEdgeArgs) *mcpsdk.CallToolResult {
	return s.handleValidateGraphEdge(ctx, args)
}

// HandleGetClusterMapForTest invokes the unexported handleGetClusterMap handler.
// Phase 73-04 E2E test calls this against a SemanticSkill wired with a real
// *Store + bleve + populated graph via buildP1E2EFixture.
func HandleGetClusterMapForTest(s *SemanticSkill, ctx context.Context, args GetClusterMapArgs) *mcpsdk.CallToolResult {
	return s.handleGetClusterMap(ctx, args)
}

// HandleExplainClusterForTest invokes the unexported handleExplainCluster handler.
// Phase 73-04 E2E test calls this against a SemanticSkill wired with a real
// *Store + bleve + populated graph via buildP1E2EFixture.
func HandleExplainClusterForTest(s *SemanticSkill, ctx context.Context, args ExplainClusterArgs) *mcpsdk.CallToolResult {
	return s.handleExplainCluster(ctx, args)
}

// HandleGetChangeImpactGraphForTest invokes the unexported handleGetChangeImpactGraph
// handler. Phase 73-04 E2E test calls this against a SemanticSkill wired with
// a real *Store + bleve + populated graph via buildP1E2EFixture.
func HandleGetChangeImpactGraphForTest(s *SemanticSkill, ctx context.Context, args GetChangeImpactGraphArgs) *mcpsdk.CallToolResult {
	return s.handleGetChangeImpactGraph(ctx, args)
}

// WiredAccessorsBoolMap is a snapshot of which P1 accessor fields are non-nil
// on a SemanticSkill. Used by the D-03 runtime bootstrap test to assert
// production wiring without reflection.
type WiredAccessorsBoolMap struct {
	SymbolByName      bool
	ExtractorRun      bool
	ClusterMembership bool
	TypeChain         bool
	SymbolEdges       bool
	EdgeEvidence      bool
	ClusterMap        bool
	ClusterMember     bool
	ClusterPageRank   bool
	ImpactLookup      bool
}

// WiredAccessorsForTest returns a WiredAccessorsBoolMap snapshot of which P1
// accessor fields are non-nil on s. Called by daemon-package bootstrap tests
// (Plan 74-05) across the package boundary as semantic.WiredAccessorsForTest.
func WiredAccessorsForTest(s *SemanticSkill) WiredAccessorsBoolMap {
	s.mu.Lock()
	defer s.mu.Unlock()
	return WiredAccessorsBoolMap{
		SymbolByName:      s.symbolByName != nil,
		ExtractorRun:      s.extractorRun != nil,
		ClusterMembership: s.clusterMembership != nil,
		TypeChain:         s.typeChain != nil,
		SymbolEdges:       s.symbolEdges != nil,
		EdgeEvidence:      s.edgeEvidence != nil,
		ClusterMap:        s.clusterMap != nil,
		ClusterMember:     s.clusterMember != nil,
		ClusterPageRank:   s.clusterPageRank != nil,
		ImpactLookup:      s.impactLookup != nil,
	}
}
