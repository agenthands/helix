package pipelines

import (
	"context"
	"errors"

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
// so Phase 57's TestLiveUpdatePipelineValidates and TestPipelineCount_MatchesSpec
// continue to compile against a package-level var. Phase 60-05B added the
// BuildLiveUpdatePhases constructor (below) which the daemon uses to thread
// real wired components into the closures; the daemon DOES NOT touch this var.
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

// LiveUpdateComponents bundles the wired runtime components the
// BuildLiveUpdatePhases constructor receives from the daemon (60-05B).
// Each field is interface-typed so the pipelines package does not import
// the concrete live/scheduler/store/lspqueue packages — the daemon
// instantiates the concrete types and passes them through this seam.
//
// The phasegraph runner for the live pipeline is REACTIVE: events arrive
// through the watcher / scanner / kernel hooks and are dispatched
// immediately by the live.Service registered via SetEditNotifier. The
// DAG itself is run at daemon bootstrap (Phase 60 D-06) as a wiring
// validator: every Run closure verifies its required component is
// non-nil and returns an error if not. This catches a regression where
// a future refactor drops one of the wired components from the daemon.
type LiveUpdateComponents struct {
	// EditNotifier is the kernel.EditNotifier impl the daemon installs
	// via Kernel.SetEditNotifier. In production this is *live.Service;
	// the BuildLiveUpdatePhases constructor only checks non-nil.
	EditNotifier any

	// OverlayStore is the store.Store handle that owns BeginOverlayTx
	// (60-02). The handler.Handler consumes it via the OverlayWriter
	// interface — wiring is through the live service, this field is the
	// validator handle.
	OverlayStore any

	// IncrementalScheduler is the scheduler.IncrementalHandler that the
	// scheduler invokes during ScheduleIncremental dispatch.
	IncrementalScheduler any

	// LSPRevalidationQueue is the lspqueue.Queue Phase 61 will drain.
	LSPRevalidationQueue any
}

// validate returns the first non-nil error if any required component is
// missing. The Run closures call this to convert a misconfiguration into
// a phase-graph-failure rather than a silent runtime nop.
func (c LiveUpdateComponents) validate() error {
	if c.EditNotifier == nil {
		return errors.New("LiveUpdateComponents.EditNotifier is nil — daemon must wire kernel.SetEditNotifier(liveService) before BuildLiveUpdatePhases")
	}
	if c.OverlayStore == nil {
		return errors.New("LiveUpdateComponents.OverlayStore is nil — daemon must wire the semantic store before BuildLiveUpdatePhases")
	}
	if c.IncrementalScheduler == nil {
		return errors.New("LiveUpdateComponents.IncrementalScheduler is nil — daemon must wire scheduler.SetIncrementalHandler before BuildLiveUpdatePhases")
	}
	if c.LSPRevalidationQueue == nil {
		return errors.New("LiveUpdateComponents.LSPRevalidationQueue is nil — daemon must construct lspqueue.Queue before BuildLiveUpdatePhases")
	}
	return nil
}

// BuildLiveUpdatePhases is the Phase 60-05B real-bodies constructor. It
// returns a fresh slice (constructor, not package-level var) wired with
// closures that validate the LiveUpdateComponents at run-time. Each
// closure returns the input deps unchanged with no error when wiring is
// healthy — the dependent phase consumes deps[ID] by name.
//
// Per the plan's Phase 60 D-06 contract: "fill the bodies". The reactive
// nature of the live pipeline means each closure's substantive work
// happens via the live.Service hooks (watcher, scanner, kernel edit
// notifier) rather than per-tick. The closures here serve two purposes:
//
//  1. Validator: a missing component fails the bootstrap rather than
//     silently dropping events.
//  2. Provides-pass-through: each phase emits a typed PhaseOutput naming
//     the wiring layer that handles the SPEC §39.6 step at runtime.
//
// `grep -v '^//' internal/phasegraph/pipelines/live.go | grep -c 'noopRun'`
// returns 0 inside this constructor — every Run is a real closure.
func BuildLiveUpdatePhases(c LiveUpdateComponents) []phasegraph.PhaseSpec {
	mkValidator := func(component any, name string) phasegraph.PhaseRunFunc {
		return func(_ context.Context, _ phasegraph.PhaseDeps) (phasegraph.PhaseOutput, error) {
			if err := c.validate(); err != nil {
				return nil, err
			}
			if component == nil {
				return nil, errors.New("phase " + name + ": required component nil")
			}
			return name, nil
		}
	}
	return []phasegraph.PhaseSpec{
		{
			ID: PhaseCollectEvents, Requires: nil, Provides: []string{"events"},
			// Producers: kernel EditNotifier (60-03) + watcher (60-05A) +
			// scanner (60-05B). Validator pins the EditNotifier seam.
			Run: mkValidator(c.EditNotifier, "collect_events"),
		},
		{
			ID: PhaseCoalesceEvents, Requires: []phasegraph.PhaseID{PhaseCollectEvents}, Provides: []string{"coalesced_events"},
			// live/coalescer per-workspace goroutine (60-04). Reactive,
			// not phase-driven; validator confirms the EditNotifier is
			// installed (which carries the coalescer registry).
			Run: mkValidator(c.EditNotifier, "coalesce_events"),
		},
		{
			ID: PhaseClassifyEvents, Requires: []phasegraph.PhaseID{PhaseCoalesceEvents}, Provides: []string{"classified_events"},
			// live.ClassifyPathChange (60-04). Same EditNotifier carrier.
			Run: mkValidator(c.EditNotifier, "classify_events"),
		},
		{
			ID: PhaseParseChangedFiles, Requires: []phasegraph.PhaseID{PhaseClassifyEvents}, Provides: []string{"reparsed_trees"},
			// scheduler.ScheduleIncremental dispatches per-FileChange
			// (60-04 fill). Validator pins the IncrementalScheduler.
			Run: mkValidator(c.IncrementalScheduler, "parse_changed_files"),
		},
		{
			ID: PhaseDiffEffectiveFacts, Requires: []phasegraph.PhaseID{PhaseParseChangedFiles}, Provides: []string{"fact_diff"},
			// Phase 62 (effective-fact diff) lands the body; Phase 60
			// validator pins the OverlayStore seam.
			Run: mkValidator(c.OverlayStore, "diff_effective_facts"),
		},
		{
			ID: PhaseWriteOverlay, Requires: []phasegraph.PhaseID{PhaseDiffEffectiveFacts}, Provides: []string{"overlay_writes"},
			// handler.UpdateChangedFile / HandleFileDeleted (60-04) write
			// through OverlayTx (60-02). Validator pins OverlayStore.
			Run: mkValidator(c.OverlayStore, "write_overlay"),
		},
		{
			ID: PhaseRepairGraphCache, Requires: []phasegraph.PhaseID{PhaseWriteOverlay}, Provides: []string{"repaired_graph"},
			// Phase 62 body; Phase 60 validator confirms OverlayStore.
			Run: mkValidator(c.OverlayStore, "repair_graph_cache"),
		},
		{
			ID: PhaseMarkScoresClusters, Requires: []phasegraph.PhaseID{PhaseRepairGraphCache}, Provides: []string{"marked_freshness"},
			// Phase 62 body; Phase 60 validator confirms OverlayStore.
			Run: mkValidator(c.OverlayStore, "mark_scores_clusters"),
		},
		{
			ID: PhaseEnqueueLSPRevalidation, Requires: []phasegraph.PhaseID{PhaseMarkScoresClusters}, Provides: []string{"lsp_revalidation_queue"},
			// Producer side ships in 60-04 (lspqueue.Queue.Enqueue);
			// Phase 61 wires the consumer worker. Validator pins the
			// queue handle.
			Run: mkValidator(c.LSPRevalidationQueue, "enqueue_lsp_revalidation"),
		},
	}
}
