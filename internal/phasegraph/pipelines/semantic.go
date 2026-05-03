// Package pipelines ships the SHAPE of Helix's order-sensitive workflows
// (semantic indexing, live updates, evaluation runs) as []phasegraph.PhaseSpec
// literals.
//
// Run bodies are noopRun placeholders. Phase 59-62 (semantic indexing),
// Phase 60 (live updates), and Phase 67 (eval) replace these with real
// implementations. The shapes (Requires/Provides chains) are the v1.10
// contract per DAG-02 — downstream phases must not change phase IDs or the
// Requires graph.
package pipelines

import (
	"context"

	"github.com/agenthands/helix/internal/phasegraph"
)

// Semantic-index phase IDs (SPEC §39.5, verbatim).
const (
	PhaseDiscoverFiles        phasegraph.PhaseID = "discover_files"
	PhaseParseTreeSitter      phasegraph.PhaseID = "parse_tree_sitter"
	PhaseExtractSymbols       phasegraph.PhaseID = "extract_symbols"
	PhaseResolveImports       phasegraph.PhaseID = "resolve_imports"
	PhaseResolveTypes         phasegraph.PhaseID = "resolve_types"
	PhaseLSPEnrich            phasegraph.PhaseID = "lsp_enrich"
	PhaseMergeFacts           phasegraph.PhaseID = "merge_facts"
	PhaseBuildEdges           phasegraph.PhaseID = "build_edges"
	PhaseWriteSnapshot        phasegraph.PhaseID = "write_snapshot"
	PhaseComputeScores        phasegraph.PhaseID = "compute_scores"
	PhaseComputeClusters      phasegraph.PhaseID = "compute_clusters"
	PhaseInitializeGraphCache phasegraph.PhaseID = "initialize_graph_cache"
)

// SemanticIndexPhases ships the SHAPE only — Run bodies are noopRun placeholders
// that Phase 59-62 will replace with real implementations (DAG-02 contract).
//
// Chain (linear, per SPEC §39.5):
//
//	discover_files → parse_tree_sitter → extract_symbols → resolve_imports
//	→ resolve_types → lsp_enrich → merge_facts → build_edges → write_snapshot
//	→ compute_scores → compute_clusters → initialize_graph_cache
var SemanticIndexPhases = []phasegraph.PhaseSpec{
	{ID: PhaseDiscoverFiles, Requires: nil, Provides: []string{"files"}, Run: noopRun},
	{ID: PhaseParseTreeSitter, Requires: []phasegraph.PhaseID{PhaseDiscoverFiles}, Provides: []string{"trees"}, Run: noopRun},
	{ID: PhaseExtractSymbols, Requires: []phasegraph.PhaseID{PhaseParseTreeSitter}, Provides: []string{"symbols"}, Run: noopRun},
	{ID: PhaseResolveImports, Requires: []phasegraph.PhaseID{PhaseExtractSymbols}, Provides: []string{"import_edges"}, Run: noopRun},
	{ID: PhaseResolveTypes, Requires: []phasegraph.PhaseID{PhaseResolveImports}, Provides: []string{"type_facts"}, Run: noopRun},
	{ID: PhaseLSPEnrich, Requires: []phasegraph.PhaseID{PhaseResolveTypes}, Provides: []string{"lsp_facts"}, Run: noopRun},
	{ID: PhaseMergeFacts, Requires: []phasegraph.PhaseID{PhaseLSPEnrich}, Provides: []string{"merged_facts"}, Run: noopRun},
	{ID: PhaseBuildEdges, Requires: []phasegraph.PhaseID{PhaseMergeFacts}, Provides: []string{"edges"}, Run: noopRun},
	{ID: PhaseWriteSnapshot, Requires: []phasegraph.PhaseID{PhaseBuildEdges}, Provides: []string{"snapshot"}, Run: noopRun},
	{ID: PhaseComputeScores, Requires: []phasegraph.PhaseID{PhaseWriteSnapshot}, Provides: []string{"scores"}, Run: noopRun},
	{ID: PhaseComputeClusters, Requires: []phasegraph.PhaseID{PhaseComputeScores}, Provides: []string{"clusters"}, Run: noopRun},
	{ID: PhaseInitializeGraphCache, Requires: []phasegraph.PhaseID{PhaseComputeClusters}, Provides: []string{"graph_cache"}, Run: noopRun},
}

// noopRun is the placeholder Run body shared by every pipeline shape. It
// returns (nil, nil) and never inspects deps. Downstream phases (P59-62, P60,
// P67) replace these with real implementations.
func noopRun(_ context.Context, _ phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
	return nil, nil
}
