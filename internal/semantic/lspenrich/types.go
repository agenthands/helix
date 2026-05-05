package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/semantic/store"
)

// Outcome is the per-file enrichment outcome — closed enum (Phase 61 D-07,
// 61-CONTEXT.md "Metrics" lines 495-510).
//
// B3 resolution: Outcome lives in P01 (this package), NOT P02. Both P02
// cascade.go (the worker that produces the outcome) AND P03 manager.go +
// status.go (the metrics emission and status accessor) reference Outcome.
// Hosting the enum here eliminates the hidden P03→P02 dependency that
// would otherwise serialize Wave-2 (`P03 must wait for P02`); with this
// file in P01, P02 and P03 become truly parallel — both depends_on:
// [61-01].
//
// The enum is intentionally additive — Phase 62 may grow new outcomes
// (e.g. "applied_with_quirks" for jdtls cold-start partials) by appending
// constants here without breaking existing producers.
type Outcome string

const (
	// OutcomeApplied — the cascade ran to completion; symbols/edges/
	// diagnostics all committed; no preemption, no budget exhaustion.
	OutcomeApplied Outcome = "applied"

	// OutcomePartialBudget — per-file budget (timeout / max-symbols /
	// max-references) exhausted mid-cascade; partial facts committed,
	// remaining steps skipped. partial_reason="budget exhausted".
	OutcomePartialBudget Outcome = "partial_budget"

	// OutcomePartialPreempted — ForegroundBusy returned true between
	// cascade steps; worker yielded; partial facts committed.
	// partial_reason="preempted".
	OutcomePartialPreempted Outcome = "partial_preempted"

	// OutcomePartialLSPUnavail — readiness gate timeout, circuit open,
	// or LS crash; partial facts committed (whatever was gathered).
	// partial_reason="lsp_unavailable".
	OutcomePartialLSPUnavail Outcome = "partial_lsp_unavailable"

	// OutcomeDropped — file dropped before cascade started (language
	// has no LS installed; file outside enrichment surface). No facts
	// written, no partial_reason stamped.
	OutcomeDropped Outcome = "dropped"
)

// MetricsSink is the lspenrich-side metrics surface emitted by Worker /
// Cascade (P02) and Manager (P03). Implemented by ProdMetricsSink (P03)
// wrapping *obs.Metrics with bounded-label closed-enum semantics.
//
// B3 resolution: this interface lives in P01 so handler.go (P01,
// produces LSPEnrichmentBulkSuppressed) AND cascade.go (P02, produces
// LSPEnrichmentTotal/Duration/Errors) AND manager.go (P03, gauges
// LaneDepth) can ALL reference the SAME interface — no parallel
// declarations across plans.
//
// Method shape mirrors the closed-enum bounded-label discipline (only
// strings, never user-controlled values, are passed as labels).
//
//   - LSPEnrichmentTotal(language, outcome) — counter on cascade
//     completion; outcome ∈ {applied, partial_budget, partial_preempted,
//     partial_lsp_unavailable, dropped} (the Outcome enum above).
//   - LSPEnrichmentDuration(language, secs) — histogram per file.
//   - LSPEnrichmentErrors(language, outcome) — counter on cascade
//     failure; outcome ∈ {timeout, ls_crash, circuit_open,
//     readiness_timeout, other}.
//   - LSPEnrichmentLaneDepth(lane, depth) — gauge; lane ∈ {high,
//     background} (the Lane enum from queue.go).
//   - LSPEnrichmentBulkSuppressed(n) — counter bumped per
//     ChangeBulkUpdate dispatch (NOT per affected file); n is the path
//     count for ops dashboards.
type MetricsSink interface {
	LSPEnrichmentTotal(language, outcome string)
	LSPEnrichmentDuration(language string, secs float64)
	LSPEnrichmentErrors(language, outcome string)
	LSPEnrichmentLaneDepth(lane string, depth int)
	LSPEnrichmentBulkSuppressed(n int)
}

// OverlayStore is the narrow store seam used by Cascade.Run (P02) and
// Worker.markPending (P02), as well as the producer-side markBulkPending
// helper in handler.go (P01). One method, one purpose: open a per-tx
// epoch-bumped overlay write transaction.
//
// P03 daemon wiring supplies a small adapter wrapping *store.Store; the
// adapter has no logic, only forwards. Hosted in P01 (this package, B3
// resolution) so all consumers (P01 handler, P02 cascade, P03 daemon)
// reference the same interface.
type OverlayStore interface {
	BeginOverlayTx(ctx context.Context, repoID string) (*store.OverlayTx, error)
}
