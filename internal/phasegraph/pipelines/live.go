package pipelines

import (
	"github.com/agenthands/helix/internal/phasegraph"
)

// Live-update phase IDs (SPEC §39.6, verbatim).
const (
	PhaseCollectEvents          phasegraph.PhaseID = "collect_events"
	PhaseCoalesceEvents         phasegraph.PhaseID = "coalesce_events"
	PhaseClassifyEvents         phasegraph.PhaseID = "classify_events"
	PhaseParseChangedFiles      phasegraph.PhaseID = "parse_changed_files"
	PhaseDiffEffectiveFacts     phasegraph.PhaseID = "diff_effective_facts"
	PhaseWriteOverlay           phasegraph.PhaseID = "write_overlay"
	PhaseRepairGraphCache       phasegraph.PhaseID = "repair_graph_cache"
	PhaseMarkScoresClusters     phasegraph.PhaseID = "mark_scores_clusters"
	PhaseEnqueueLSPRevalidation phasegraph.PhaseID = "enqueue_lsp_revalidation"
)

// LiveUpdatePhases ships the SHAPE only — Run bodies are noopRun placeholders
// that Phase 60 (live updates) replaces with real implementations (DAG-02
// contract).
//
// Chain (linear, per SPEC §39.6):
//
//	collect_events → coalesce_events → classify_events → parse_changed_files
//	→ diff_effective_facts → write_overlay → repair_graph_cache
//	→ mark_scores_clusters → enqueue_lsp_revalidation
var LiveUpdatePhases = []phasegraph.PhaseSpec{
	{ID: PhaseCollectEvents, Requires: nil, Provides: []string{"events"}, Run: noopRun},
	{ID: PhaseCoalesceEvents, Requires: []phasegraph.PhaseID{PhaseCollectEvents}, Provides: []string{"coalesced_events"}, Run: noopRun},
	{ID: PhaseClassifyEvents, Requires: []phasegraph.PhaseID{PhaseCoalesceEvents}, Provides: []string{"classified_events"}, Run: noopRun},
	{ID: PhaseParseChangedFiles, Requires: []phasegraph.PhaseID{PhaseClassifyEvents}, Provides: []string{"reparsed_trees"}, Run: noopRun},
	{ID: PhaseDiffEffectiveFacts, Requires: []phasegraph.PhaseID{PhaseParseChangedFiles}, Provides: []string{"fact_diff"}, Run: noopRun},
	{ID: PhaseWriteOverlay, Requires: []phasegraph.PhaseID{PhaseDiffEffectiveFacts}, Provides: []string{"overlay_writes"}, Run: noopRun},
	{ID: PhaseRepairGraphCache, Requires: []phasegraph.PhaseID{PhaseWriteOverlay}, Provides: []string{"repaired_graph"}, Run: noopRun},
	{ID: PhaseMarkScoresClusters, Requires: []phasegraph.PhaseID{PhaseRepairGraphCache}, Provides: []string{"marked_freshness"}, Run: noopRun},
	{ID: PhaseEnqueueLSPRevalidation, Requires: []phasegraph.PhaseID{PhaseMarkScoresClusters}, Provides: []string{"lsp_revalidation_queue"}, Run: noopRun},
}
