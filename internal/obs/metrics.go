// Phase 11 metrics scaffolding: pre-registered Prometheus vectors on an owned
// registry. Middleware and lspool (plans 11-02 / 11-03) consume via *obs.Metrics
// only — prometheus/client_golang is intentionally kept confined to this package
// so the blast radius of the dependency stays tight.
//
// Design rules (frozen for downstream plans):
//
//   - Exactly ONE *prometheus.Registry owned by each *Metrics instance.
//     The prometheus global registerer is NEVER touched (PITFALLS.md
//     meta-rule, T-11-05 mitigation).
//   - Metric vectors are constructed ONCE in newMetrics(); middleware resolves
//     labels per call via .WithLabelValues(...) (Pattern 1, RESEARCH.md).
//   - AllowedLabels is an ARRAY, not a slice — immutable at the type level.
//   - Noop providers still get a real Metrics sink so call sites never branch
//     on nil (see obs.Noop). The vectors are unscraped in the noop path; the
//     cost of constructing them at startup is negligible.
//   - lspool sink helper signatures below are the source of truth for the
//     lspool.MetricsSink interface in plan 11-03. Changing them here is a
//     breaking change for that plan.
package obs

import "github.com/prometheus/client_golang/prometheus"
import "github.com/prometheus/client_golang/prometheus/collectors"
import dto "github.com/prometheus/client_model/go"

// Phase 70 D-04: closed-enum "reason" values for the incremental-refresh
// fallback counter. Declared as package-level constants so call sites in
// the refresh tool import a single source of truth (mirrors the Phase 68
// D-08 placement pattern). Any change here MUST be matched in
// IncrementalRefreshFallbackInc and the metrics_labels_test.go carve-out.
const (
	IncrementalRefreshFallbackReasonColdStart      = "cold_start"
	IncrementalRefreshFallbackReasonOverlayRotated = "overlay_rotated"
	IncrementalRefreshFallbackReasonEmptyOverlay   = "empty_overlay"
	IncrementalRefreshFallbackReasonError          = "error"
)

// AllowedLabels is the bounded-label allowlist enforced at CI time by
// TestMetricsLabelsAllowlist (D-03..D-05). Changing this list requires a
// matching change to metrics_labels_test.go and a plan-level decision.
var AllowedLabels = [6]string{"tool_name", "profile", "mode", "language", "outcome", "lane"}

// Metrics holds all Prometheus vectors owned by this package plus the
// private registry they are registered against. Access via *obs.Provider.Metrics().
type Metrics struct {
	registry *prometheus.Registry

	// RED metric vectors for MCP tool calls (plan 11-02).
	ToolCalls    *prometheus.CounterVec
	ToolDuration *prometheus.HistogramVec

	// lspool gauges and counters (plan 11-03).
	LSPoolWorkers      *prometheus.GaugeVec
	LSPoolEvictions    *prometheus.CounterVec
	LSPoolCircuitState *prometheus.GaugeVec
	LSPoolRestarts     *prometheus.CounterVec

	// Phase 47 D-07: rename_symbol dispatcher strategy counter.
	// Closed-enum label "strategy" ∈ {"lsp-native", "rust-client-side"};
	// enforced at emission sites (see *Metrics.RenameStrategyInc and
	// internal/mcp.RecordRenameStrategy). The "strategy" label is carved
	// out of AllowedLabels in metrics_labels_test.go for this family only.
	RenameStrategy *prometheus.CounterVec

	// Phase 53 D-01/D-04: lspool worker-pool cache lookup counter.
	// Closed-enum label "result" ∈ {"hit","miss"} — hit = shared warm worker,
	// miss = spawn (or refusal); enforced at emission via LSPoolLookup helper.
	LSPoolLookups *prometheus.CounterVec

	// Phase 53 D-01/D-03/D-04: repomap TagCache mtime-match lookup counter.
	// Closed-enum label "result" ∈ {"hit","miss"}.
	RepoMapLookups *prometheus.CounterVec

	// Phase 53 D-05/D-06/D-07: repomap extractor latency histogram (cache-miss path only).
	// Closed-enum label "extractor" ∈ {"treesitter","lsp","fallback"}.
	// Custom buckets target 1ms→2.5s to give fast-path resolution for tree-sitter
	// and tail visibility for LSP fallback.
	RepoMapExtract *prometheus.HistogramVec

	// Phase 53 D-08/D-09: session lifecycle counter.
	// Closed-enum "phase" ∈ {"started","ended","error"}; "transport" ∈ {"stdio","http"}.
	SessionLifecycle *prometheus.CounterVec

	// Phase 53 D-10/D-11/D-12: edit-tool outcome counter (7 tools).
	// Closed-enum "outcome" ∈ {"success","no_match","ambiguous_match","validation_failed","ls_error","internal"};
	// "strategy" ∈ {"exact","whitespace_normalized","indentation_flexible","none"} (per Q-4 resolution).
	// "tool_name" reuses the existing AllowedLabels entry — NOT a carve-out.
	EditOutcome *prometheus.CounterVec

	// Phase 57 D-06/D-07: semantic store DuckDB-file quarantine counter.
	// Closed-enum "reason" ∈ {"corrupt_file","schema_forward_incompat",
	// "schema_unreadable","unknown"}; "workspace_label" is the bounded
	// hashed/truncated identifier used by the existing helix_lspool_*
	// metrics (T-57-02-06 mitigation — no raw paths).
	SemanticStoreQuarantine *prometheus.CounterVec

	// Phase 57: semantic store open-attempt counter.
	// Closed-enum "outcome" ∈ {"opened","quarantined","created"}.
	SemanticStoreOpen *prometheus.CounterVec

	// Phase 81 ABLATE-06: semantic store DuckDB read/query counter. NET-NEW —
	// the rest of the helix_semantic_* family (open/quarantine/extraction/
	// live-updates/enrichment/pagerank/types/compaction/vacuum) counts
	// writes/maintenance only; none counted a read before this. Labelless:
	// a faithful "any semantic-store read happened" signal. The no_semantic
	// ablation arm asserts this counter == 0 after a run (D-05); every read
	// that funnels through the s.db read chokepoint increments it.
	SemanticStoreReads prometheus.Counter

	// Phase 59 P02: tree-sitter extraction outcome counter.
	// Closed-enum "language" ∈ {"go","typescript","python","other"};
	// "outcome" ∈ {"ready","partial","unsupported","failed"}. Both labels
	// are members of AllowedLabels; bounded-cardinality is enforced at
	// emission via SemanticExtractionTotal (T-59-02-02 mitigation).
	SemanticExtraction *prometheus.CounterVec

	// Phase 60 D-07: live-update pipeline outcome counter (60-05B).
	// Closed-enum "kind" ∈ {"file_created","file_modified","file_deleted",
	// "file_renamed","helix_edit","bulk_update"} — the SourceChangeKind
	// enum from internal/semantic/live/signal.go verbatim.
	// Closed-enum "outcome" ∈ {"applied","no_op","error","dropped"}.
	// Cardinality bound: 6 × 4 = 24 combos per workspace. Both labels are
	// carved out in metrics_labels_test.go and enforced at emission via
	// SemanticLiveUpdatesInc (drop-on-unknown).
	SemanticLiveUpdates *prometheus.CounterVec

	// Phase 68 D-07: precise FileFactDiff populator outcome counter.
	// Closed-enum "tier" ∈ {"full","added-only","synthetic"}; "repo" is
	// a bounded per-workspace identifier. Cardinality bound: 3 × N repos.
	// Carved out in metrics_labels_test.go and enforced at emission via
	// LiveFileFactDiffInc (drop-on-unknown).
	LiveFileFactDiff *prometheus.CounterVec

	// Phase 68 D-08: Tier-3 synthetic-marker fall-through reason breakdown.
	// Closed-enum "reason" ∈ {"cold_start","extract_failed",
	// "extract_unsupported"}. Carved out in metrics_labels_test.go and
	// enforced at emission via LiveFileFactDiffSyntheticReasonInc
	// (drop-on-unknown).
	LiveFileFactDiffSynRsn *prometheus.CounterVec

	// Phase 70 D-04: incremental refresh fallback counter. Bumped when
	// the refresh tool falls back to a full overlay walk instead of the
	// incremental drain path. Closed-enum "reason" ∈ {"cold_start",
	// "overlay_rotated","empty_overlay","error"} + bounded "repo"
	// identifier. Neither label is in AllowedLabels; both are carved out
	// in metrics_labels_test.go. Helper IncrementalRefreshFallbackInc is
	// the single emission site and drops unknown reason values.
	IncrementalRefreshFallback *prometheus.CounterVec

	// Phase 61 P03: LSP enrichment-worker outcome counter (vector).
	// Closed-enum "language" ∈ AllowedLabels (already a member);
	// "outcome" ∈ {"applied","partial_budget","partial_preempted",
	// "partial_lsp_unavailable","dropped"} — the lspenrich.Outcome enum
	// from internal/semantic/lspenrich/types.go verbatim.
	// Helper method LSPEnrichmentTotal drops unknowns (T-61-03-01).
	// Field is named *Vec to avoid Go name collision with the helper.
	LSPEnrichmentTotalVec *prometheus.CounterVec

	// Phase 61 P03: LSP enrichment-worker per-file duration histogram.
	// Single label "language"; buckets target 50ms→30s to give fast-path
	// resolution for cached jdtls/gopls hover and tail visibility for cold
	// cascades.
	LSPEnrichmentDurationVec *prometheus.HistogramVec

	// Phase 61 P03: LSP enrichment-worker error counter (vector).
	// Closed-enum "outcome" ∈ {"timeout","ls_crash","circuit_open",
	// "readiness_timeout","other"}. Helper LSPEnrichmentErrors drops
	// unknowns (T-61-03-01).
	LSPEnrichmentErrorsVec *prometheus.CounterVec

	// Phase 61 P03: LSP enrichment-worker per-lane queue depth gauge (vec).
	// Closed-enum "lane" ∈ {"high","background"} — the lspenrich.Lane enum
	// from internal/semantic/lspenrich/queue.go verbatim. Helper
	// LSPEnrichmentLaneDepth drops unknowns (T-61-03-01).
	LSPEnrichmentLaneDepthVec *prometheus.GaugeVec

	// Phase 61 P03: LSP enrichment-worker bulk-update suppression counter.
	// No labels (event count is what matters; per-file counts inflate the
	// counter without adding signal). Bumped per ChangeBulkUpdate dispatch
	// from handler.markBulkPending — see Phase 61 P01 D-05.
	LSPEnrichmentBulkSuppressedCtr prometheus.Counter

	// Phase 62 P02: PageRank engine duration histogram.
	// Closed-enum "scope" ∈ {"incremental","full"};
	// "projection" ∈ {"call_graph"} (reserved for future projections).
	// Helper SemanticGraphPagerankObserve drops unknown labels (T5).
	SemanticGraphPagerankDurationVec *prometheus.HistogramVec

	// Phase 62 P02: score-status counter (read-time emission).
	// Closed-enum "projection" ∈ {"call_graph"};
	// "status" ∈ {"exact","approximate","stale","missing"} (D-07).
	SemanticGraphScoreStatusVec *prometheus.CounterVec

	// Phase 62 P02: repair outcome counter.
	// Closed-enum "outcome" ∈ {"applied","frontier_overflow","preempted","error","stub_no_data"}.
	// stub_no_data was added by Phase 62-08 to surface deferred Phase 64
	// read paths on rankStoreAdapter (62-VERIFICATION.md gap truth #21,
	// WR-05) so dashboards distinguish "no data yet" from clean repair.
	SemanticGraphRepairVec *prometheus.CounterVec

	// Phase 62 P02: graph_version gauge per workspace.
	// "workspace_label" is the same bounded hashed/truncated identifier
	// used by helix_semantic_store_*.
	SemanticGraphVersionGauge *prometheus.GaugeVec

	// Phase 62 P02: type-resolution outcome counter.
	// "language" ∈ AllowedLabels;
	// "confidence_tier" ∈ {"1.00","0.90","0.80","0.70","0.60","0.45","0.20"} (D-12).
	SemanticTypesResolutionVec *prometheus.CounterVec

	// Phase 63 P63-02 Task 3: compaction + vacuum duration histograms.
	// Closed-enum outcome labels keep cardinality bounded (T-63-02-06
	// mitigation). Drop-on-unknown via SemanticCompactionObserve /
	// SemanticVacuumObserve helpers.
	SemanticCompactionDurationVec *prometheus.HistogramVec
	SemanticCompactionBlockedVec  *prometheus.CounterVec
	SemanticVacuumDurationVec     *prometheus.HistogramVec

	// Phase 66 P01: guardrail receipt counters (GUARD-05 telemetry).
	// Closed-enum label discipline; drop-unknown at emission sites.
	//
	// helix_receipt_issued_total{class} — class ∈ {references_checked,
	//   impact_checked, context_gathered, structural_overview, diagnostics_clean}.
	//   Incremented on successful receipt issuance from a read-side tool.
	//
	// helix_receipt_expired_total{reason} — reason ∈ {ttl, graph_drift,
	//   lru_evicted}. Incremented whenever a receipt is removed from the store.
	//
	// helix_receipt_lookup_total{outcome} — outcome ∈ {hit, miss, expired,
	//   scope_mismatch, graph_drift, workspace_mismatch, freshness_rejected,
	//   wrong_class}. Incremented on every Get() call in the store.
	//
	// helix_guardrail_eval_timeout_total — UNLABELED (T-66-21 fail-open audit
	//   trail; no label to avoid cardinality from timeout sources). Incremented
	//   from the GuardrailMiddleware eval timeout branch (Plan 04).
	ReceiptIssuedVec     *prometheus.CounterVec
	ReceiptExpiredVec    *prometheus.CounterVec
	ReceiptLookupVec     *prometheus.CounterVec
	GuardrailEvalTimeout prometheus.Counter
}

// newMetrics constructs a fresh *Metrics with an owned prometheus.Registry.
// All helix_* vectors plus the Go runtime + Process collectors (D-16) are
// registered. Calling newMetrics() multiple times is safe: each call gets
// its own registry, so double-registration panics cannot occur.
func newMetrics() *Metrics {
	reg := prometheus.NewRegistry() // owned; NOT the prometheus global registerer

	m := &Metrics{
		registry: reg,
		ToolCalls: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_tool_calls_total",
				Help: "MCP tool calls by outcome (RED: errors).",
			},
			[]string{"tool_name", "profile", "mode", "language", "outcome"},
		),
		ToolDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helix_tool_duration_seconds",
				Help:    "MCP tool call latency in seconds (RED: duration).",
				Buckets: prometheus.DefBuckets, // D-01
			},
			[]string{"tool_name", "profile", "mode", "language"},
		),
		LSPoolWorkers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helix_lspool_workers",
				Help: "Active LS workers per language.",
			},
			[]string{"language"},
		),
		LSPoolEvictions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_lspool_evictions_total",
				Help: "LS worker evictions by reason (idle/pressure/crash/shutdown).",
			},
			// CONTEXT.md D-13: "reason" is a closed 4-value enum enforced at
			// emission sites. It is NOT in the D-04 RED allowlist; the CI label
			// lint carves it out explicitly for this family.
			[]string{"language", "reason"},
		),
		LSPoolCircuitState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helix_lspool_circuit_state",
				Help: "LS pool circuit breaker state per language (0=closed, 1=half-open, 2=open).",
			},
			[]string{"language"},
		),
		LSPoolRestarts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_lspool_restarts_total",
				Help: "LS worker restarts per language.",
			},
			[]string{"language"},
		),
		RenameStrategy: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_rename_strategy_total",
				Help: "rename_symbol successes by strategy (lsp-native | rust-client-side).",
			},
			// "strategy" is a closed-enum dimension carved out of AllowedLabels
			// for this family only (Phase 47 D-07). Enforced at emission sites.
			[]string{"strategy"},
		),
		LSPoolLookups: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_lspool_lookups_total",
				Help: "lspool AcquireLease lookups by result (hit=shared warm worker, miss=spawn or refusal). Phase 53 D-01.",
			},
			[]string{"language", "result"},
		),
		RepoMapLookups: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_repomap_lookups_total",
				Help: "repomap TagCache GetOrExtract lookups by result (hit=mtime match, miss=extractFn invoked). Phase 53 D-01/D-03.",
			},
			[]string{"language", "result"},
		),
		RepoMapExtract: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helix_repomap_extract_duration_seconds",
				Help:    "repomap extractor latency in seconds, cache-miss path only. Phase 53 D-05/D-06.",
				Buckets: []float64{0.001, 0.0025, 0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1.0, 2.5},
			},
			[]string{"language", "extractor"},
		),
		SessionLifecycle: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_session_lifecycle_total",
				Help: "MCP session lifecycle phase transitions by transport. Phase 53 D-08/D-09. Note: transport=http phase=ended is best-effort (no SDK hook in v1.5.0).",
			},
			[]string{"phase", "transport"},
		),
		EditOutcome: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_edit_outcome_total",
				Help: "edit-tool handler outcomes by tool, outcome bucket, and fuzzy strategy. Phase 53 D-10/D-11/D-12.",
			},
			[]string{"tool_name", "outcome", "strategy"},
		),
		SemanticStoreQuarantine: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_store_quarantine_total",
				Help: "Semantic store DuckDB file quarantines by reason (corrupt_file/schema_forward_incompat/schema_unreadable/unknown). Phase 57 D-06/D-07.",
			},
			// Phase 57 D-06: closed-enum "reason" + per-workspace hashed label.
			[]string{"workspace_label", "reason"},
		),
		SemanticStoreOpen: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_store_open_total",
				Help: "Semantic store open attempts by outcome (opened/quarantined/created). Phase 57.",
			},
			// Phase 57: closed-enum "outcome" + per-workspace hashed label.
			[]string{"workspace_label", "outcome"},
		),
		// Phase 81 ABLATE-06: labelless read counter. Emitted at the single
		// DuckDB read chokepoint (internal/semantic/store s.queryContext /
		// s.queryRowContext). Must read 0 on the no_semantic arm (D-05).
		SemanticStoreReads: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "helix_semantic_store_reads_total",
				Help: "Semantic store DuckDB read/query operations (Phase 81 ABLATE-06: must be 0 on the no_semantic arm).",
			},
		),
		SemanticExtraction: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_extraction_total",
				Help: "Tree-sitter extraction outcomes by language and result (ready/partial/unsupported/failed). Phase 59 P02.",
			},
			// Phase 59 P02: closed-enum "language" + closed-enum "outcome".
			// Both already in AllowedLabels; helper drops unknowns.
			[]string{"language", "outcome"},
		),
		SemanticLiveUpdates: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_live_updates_total",
				Help: "Live-update pipeline outcomes by SourceChangeKind and result (applied/no_op/error/dropped). Phase 60 D-07.",
			},
			// Phase 60 D-07: closed-enum "kind" (6 values) + closed-enum
			// "outcome" (4 values). Carved out in metrics_labels_test.go;
			// helper SemanticLiveUpdatesInc drops unknowns.
			[]string{"kind", "outcome"},
		),
		LiveFileFactDiff: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_live_filefactdiff_total",
				Help: "Live FileFactDiff populator outcomes by tier (full/added-only/synthetic) and repo. Phase 68.",
			},
			// Phase 68 D-07: closed-enum "tier" + per-repo bounded label.
			// Carved out in metrics_labels_test.go; helper
			// LiveFileFactDiffInc drops unknowns.
			[]string{"tier", "repo"},
		),
		LiveFileFactDiffSynRsn: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_live_filefactdiff_synthetic_reason_total",
				Help: "Tier-3 synthetic-marker reason breakdown. Phase 68 D-08.",
			},
			// Phase 68 D-08: closed-enum "reason" (cold_start /
			// extract_failed / extract_unsupported). Carved out in
			// metrics_labels_test.go; helper
			// LiveFileFactDiffSyntheticReasonInc drops unknowns.
			[]string{"reason"},
		),
		IncrementalRefreshFallback: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_incremental_refresh_fallback_total",
				Help: "Incremental refresh fell back to full-walk; reason is one of cold_start|overlay_rotated|empty_overlay|error. Phase 70.",
			},
			// Phase 70 D-04: closed-enum "reason" + bounded "repo".
			// Carved out in metrics_labels_test.go; helper
			// IncrementalRefreshFallbackInc drops unknown reason values.
			[]string{"reason", "repo"},
		),
		LSPEnrichmentTotalVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_lsp_enrichment_total",
				Help: "LSP enrichment-worker outcomes by language and result (applied/partial_budget/partial_preempted/partial_lsp_unavailable/dropped). Phase 61 P03.",
			},
			// Phase 61 P03: both labels closed-enum; helper LSPEnrichmentTotal
			// drops unknowns. "language" is in AllowedLabels; "outcome" is in
			// AllowedLabels. Carve-out entry in metrics_labels_test.go is
			// documentation parity (no NEW label name to carve).
			[]string{"language", "outcome"},
		),
		LSPEnrichmentDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "helix_semantic_lsp_enrichment_duration_seconds",
				Help: "LSP enrichment-worker per-file cascade duration in seconds. Phase 61 P03.",
				// Buckets target 50ms→30s — fast path is cached jdtls hover
				// (~50-200ms) and tail is cold-jdtls projectStatus wait.
				Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1.0, 2.5, 5.0, 10.0, 20.0, 30.0},
			},
			[]string{"language"},
		),
		LSPEnrichmentErrorsVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_lsp_enrichment_errors_total",
				Help: "LSP enrichment-worker errors by language and category (timeout/ls_crash/circuit_open/readiness_timeout/other). Phase 61 P03.",
			},
			// Phase 61 P03: closed-enum "outcome" (5 values, distinct from
			// the success-side outcomes). Helper LSPEnrichmentErrors drops
			// unknowns.
			[]string{"language", "outcome"},
		),
		LSPEnrichmentLaneDepthVec: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helix_semantic_lsp_enrichment_lane_depth",
				Help: "LSP enrichment-worker queue depth per lane (high|background). Phase 61 P03.",
			},
			// Phase 61 P03: closed-enum "lane" — new label name, carved out
			// in metrics_labels_test.go.
			[]string{"lane"},
		),
		LSPEnrichmentBulkSuppressedCtr: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "helix_semantic_lsp_enrichment_bulk_suppressed_total",
				Help: "LSP enrichment-worker bulk-update suppression counter (per ChangeBulkUpdate dispatch). Phase 61 P03 D-05.",
			},
		),
		// Phase 62 P02: graph engine + type-resolution metrics. All
		// emission helpers (SemanticGraphPagerankObserve, *Inc, *Set)
		// drop unknown closed-enum values to keep cardinality bounded
		// (T5 mitigation).
		SemanticGraphPagerankDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name: "helix_semantic_graph_pagerank_duration_seconds",
				Help: "PageRank engine pass duration in seconds. Phase 62 P02.",
				// 1ms → 60s — incremental local repair lives in the low end,
				// full recompute on a large workspace lives in the tail.
				Buckets: []float64{0.001, 0.005, 0.025, 0.1, 0.5, 1.0, 5.0, 15.0, 30.0, 60.0},
			},
			[]string{"scope", "projection"},
		),
		SemanticGraphScoreStatusVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_graph_score_status_total",
				Help: "Score-row status emissions by projection and computed status (exact/approximate/stale/missing). Phase 62 P02 D-07.",
			},
			[]string{"projection", "status"},
		),
		SemanticGraphRepairVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_graph_repair_total",
				Help: "ApplyRepair outcomes by category (applied/frontier_overflow/preempted/error/stub_no_data — the last surfaces Phase 64-deferred read paths on rankStoreAdapter). Phase 62 P02 D-06/D-09; stub_no_data added by 62-08.",
			},
			[]string{"outcome"},
		),
		SemanticGraphVersionGauge: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "helix_semantic_graph_version",
				Help: "Current graph_version per workspace (monotone non-decreasing). Phase 62 P02 D-05.",
			},
			[]string{"workspace_label"},
		),
		SemanticTypesResolutionVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_types_resolution_total",
				Help: "Type-resolver edge-emission outcomes by language and confidence tier (1.00/0.90/.../0.20). Phase 62 P02 (D-12).",
			},
			[]string{"language", "confidence_tier"},
		),
		// Phase 63 P63-02 Task 3: compaction + vacuum metrics.
		SemanticCompactionDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helix_semantic_compaction_duration_seconds",
				Help:    "Compaction cycle duration by outcome (success/partial/skipped_blocked/error). Phase 63 D-01.",
				Buckets: []float64{0.001, 0.005, 0.025, 0.1, 0.5, 1.0, 5.0, 15.0, 30.0, 60.0},
			},
			[]string{"outcome"},
		),
		SemanticCompactionBlockedVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_semantic_compaction_blocked_total",
				Help: "Compaction skipped_blocked outcomes by closed-enum BlockedReason. Phase 63 D-04.",
			},
			[]string{"reason"},
		),
		SemanticVacuumDurationVec: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "helix_semantic_vacuum_duration_seconds",
				Help:    "VACUUM cycle duration by outcome (success/skipped/error). Phase 63 D-05.",
				Buckets: []float64{0.01, 0.05, 0.1, 0.5, 1.0, 5.0, 15.0, 60.0, 300.0},
			},
			[]string{"outcome"},
		),
		// Phase 66 P01: guardrail receipt counters (GUARD-05 / T-66-04/T-66-05).
		ReceiptIssuedVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_receipt_issued_total",
				Help: "Capability receipts issued by class. Phase 66 P01. class ∈ {references_checked, impact_checked, context_gathered, structural_overview, diagnostics_clean}.",
			},
			[]string{"class"},
		),
		ReceiptExpiredVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_receipt_expired_total",
				Help: "Capability receipts expired by reason. Phase 66 P01. reason ∈ {ttl, graph_drift, lru_evicted}.",
			},
			[]string{"reason"},
		),
		ReceiptLookupVec: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "helix_receipt_lookup_total",
				Help: "Capability receipt lookups by outcome. Phase 66 P01. outcome ∈ {hit, miss, expired, scope_mismatch, graph_drift, workspace_mismatch, freshness_rejected, wrong_class}.",
			},
			[]string{"outcome"},
		),
		GuardrailEvalTimeout: prometheus.NewCounter(
			prometheus.CounterOpts{
				Name: "helix_guardrail_eval_timeout_total",
				Help: "Guardrail evaluation timeouts (fail-open; T-66-21 mitigation). Unlabeled to avoid cardinality from timeout sources.",
			},
		),
	}

	reg.MustRegister(
		m.ToolCalls,
		m.ToolDuration,
		m.LSPoolWorkers,
		m.LSPoolEvictions,
		m.LSPoolCircuitState,
		m.LSPoolRestarts,
		m.RenameStrategy,
		m.LSPoolLookups,
		m.RepoMapLookups,
		m.RepoMapExtract,
		m.SessionLifecycle,
		m.EditOutcome,
		m.SemanticStoreQuarantine,
		m.SemanticStoreOpen,
		// Phase 81 ABLATE-06: semantic store read counter.
		m.SemanticStoreReads,
		m.SemanticExtraction,
		m.SemanticLiveUpdates,
		m.LiveFileFactDiff,
		m.LiveFileFactDiffSynRsn,
		// Phase 70 D-04: incremental refresh fallback counter.
		m.IncrementalRefreshFallback,
		m.LSPEnrichmentTotalVec,
		m.LSPEnrichmentDurationVec,
		m.LSPEnrichmentErrorsVec,
		m.LSPEnrichmentLaneDepthVec,
		m.LSPEnrichmentBulkSuppressedCtr,
		// Phase 62 P02: graph + types metrics.
		m.SemanticGraphPagerankDurationVec,
		m.SemanticGraphScoreStatusVec,
		m.SemanticGraphRepairVec,
		m.SemanticGraphVersionGauge,
		m.SemanticTypesResolutionVec,
		// Phase 63 P63-02 Task 3: compaction + vacuum metrics.
		m.SemanticCompactionDurationVec,
		m.SemanticCompactionBlockedVec,
		m.SemanticVacuumDurationVec,
		// Phase 66 P01: guardrail receipt counters.
		m.ReceiptIssuedVec,
		m.ReceiptExpiredVec,
		m.ReceiptLookupVec,
		m.GuardrailEvalTimeout,
		collectors.NewGoCollector(), // D-16: goroutines, GC, memory
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)
	return m
}

// Registry returns the owned *prometheus.Registry for use with
// promhttp.HandlerFor on the admin listener (plan 11-01, task 3).
func (m *Metrics) Registry() *prometheus.Registry { return m.registry }

// --- lspool MetricsSink helper methods (FROZEN signatures — plan 11-03) ---
//
// These four methods are the source of truth for the lspool.MetricsSink
// interface. If plan 11-03 needs a different signature, THAT plan must adapt
// — obs.Metrics is the upstream authority here so the CI label lint has a
// single bounded surface to enforce.

// LSPoolWorkersSet adjusts the active worker gauge for a language by delta.
// Positive delta on acquire, negative delta on release/evict.
func (m *Metrics) LSPoolWorkersSet(language string, delta float64) {
	m.LSPoolWorkers.WithLabelValues(language).Add(delta)
}

// LSPoolEviction increments the eviction counter for a language + reason.
// reason must be one of {idle, pressure, crash, shutdown} (D-13).
func (m *Metrics) LSPoolEviction(language, reason string) {
	m.LSPoolEvictions.WithLabelValues(language, reason).Inc()
}

// LSPoolCircuitStateSet publishes the circuit breaker state for a language:
// 0=closed, 1=half-open, 2=open (D-14).
func (m *Metrics) LSPoolCircuitStateSet(language string, state float64) {
	m.LSPoolCircuitState.WithLabelValues(language).Set(state)
}

// LSPoolRestart increments the worker restart counter for a language (D-15).
func (m *Metrics) LSPoolRestart(language string) {
	m.LSPoolRestarts.WithLabelValues(language).Inc()
}

// RenameStrategyInc increments the rename dispatcher strategy counter.
// strategy MUST be one of the closed-enum values {"lsp-native","rust-client-side"};
// any other value is dropped to preserve bounded cardinality (Phase 47 D-07,
// threat T-47-08 mitigation).
func (m *Metrics) RenameStrategyInc(strategy string) {
	if strategy != "lsp-native" && strategy != "rust-client-side" {
		return
	}
	m.RenameStrategy.WithLabelValues(strategy).Inc()
}

// --- Phase 53 helper methods (D-01..D-12, drop-unknown closed-enum discipline) ---

// LSPoolLookup increments helix_lspool_lookups_total. result ∈ {"hit","miss"};
// any other value is dropped (Phase 53 D-04 closed enum, T-53-01 mitigation).
func (m *Metrics) LSPoolLookup(language, result string) {
	if result != "hit" && result != "miss" {
		return
	}
	m.LSPoolLookups.WithLabelValues(language, result).Inc()
}

// RepoMapLookup increments helix_repomap_lookups_total. Mirrors LSPoolLookup
// (Phase 53 D-04 closed enum, T-53-01 mitigation).
func (m *Metrics) RepoMapLookup(language, result string) {
	if result != "hit" && result != "miss" {
		return
	}
	m.RepoMapLookups.WithLabelValues(language, result).Inc()
}

// RepoMapExtractObserve records seconds for the cache-miss extractor path.
// extractor ∈ {"treesitter","lsp","fallback"}; any other value is dropped
// (Phase 53 D-07 closed enum, T-53-01 mitigation).
func (m *Metrics) RepoMapExtractObserve(language, extractor string, seconds float64) {
	if extractor != "treesitter" && extractor != "lsp" && extractor != "fallback" {
		return
	}
	m.RepoMapExtract.WithLabelValues(language, extractor).Observe(seconds)
}

// SessionLifecycleInc increments helix_session_lifecycle_total. phase ∈
// {"started","ended","error"}; transport ∈ {"stdio","http"}; any other
// value is dropped (Phase 53 D-08/D-09 closed enums, T-53-01 mitigation).
func (m *Metrics) SessionLifecycleInc(phase, transport string) {
	if phase != "started" && phase != "ended" && phase != "error" {
		return
	}
	if transport != "stdio" && transport != "http" {
		return
	}
	m.SessionLifecycle.WithLabelValues(phase, transport).Inc()
}

// EditOutcomeInc increments helix_edit_outcome_total. outcome ∈
// {"success","no_match","ambiguous_match","validation_failed","ls_error","internal","unsupported"};
// strategy ∈ {"exact","whitespace_normalized","indentation_flexible","none"};
// any other value is dropped (Phase 53 D-10/D-11/Q-4 closed enums,
// T-53-01 mitigation). tool_name is unbounded by helper but bounded in
// practice by the 7-tool surface (D-12).
//
// "unsupported" (Phase 76): emitted by the structured-edit ablation guards
// when DisableStructuredEditSubsystem returns serr.Unsupported before any
// edit work — lets operators measure how often agents attempt disabled tools.
func (m *Metrics) EditOutcomeInc(toolName, outcome, strategy string) {
	switch outcome {
	case "success", "no_match", "ambiguous_match", "validation_failed", "ls_error", "internal", "unsupported":
	default:
		return
	}
	switch strategy {
	case "exact", "whitespace_normalized", "indentation_flexible", "none":
	default:
		return
	}
	m.EditOutcome.WithLabelValues(toolName, outcome, strategy).Inc()
}

// --- Phase 57 helper methods (D-06/D-07, drop-unknown closed-enum discipline) ---

// SemanticStoreQuarantineInc increments helix_semantic_store_quarantine_total.
// reason ∈ {"corrupt_file","schema_forward_incompat","schema_unreadable","unknown"};
// any other value is dropped (Phase 57 D-07 closed enum, T-57-02-06 mitigation).
func (m *Metrics) SemanticStoreQuarantineInc(workspaceLabel, reason string) {
	switch reason {
	case "corrupt_file", "schema_forward_incompat", "schema_unreadable", "unknown":
	default:
		return
	}
	m.SemanticStoreQuarantine.WithLabelValues(workspaceLabel, reason).Inc()
}

// SemanticStoreOpenInc increments helix_semantic_store_open_total.
// outcome ∈ {"opened","quarantined","created"}; any other value is dropped.
func (m *Metrics) SemanticStoreOpenInc(workspaceLabel, outcome string) {
	switch outcome {
	case "opened", "quarantined", "created":
	default:
		return
	}
	m.SemanticStoreOpen.WithLabelValues(workspaceLabel, outcome).Inc()
}

// SemanticStoreReadsInc increments helix_semantic_store_reads_total by one.
// Phase 81 ABLATE-06: called at the single DuckDB read chokepoint
// (internal/semantic/store s.queryContext / s.queryRowContext) so EVERY
// semantic-store read bumps the counter. Labelless (a faithful "a read
// happened" signal); the no_semantic ablation arm asserts the total == 0.
// Nil-receiver safe so store code can call it without a metrics guard.
func (m *Metrics) SemanticStoreReadsInc() {
	if m == nil || m.SemanticStoreReads == nil {
		return
	}
	m.SemanticStoreReads.Inc()
}

// SemanticStoreReadsValue returns the current value of the labelless
// helix_semantic_store_reads_total counter as an integer. Phase 81 ABLATE-06:
// the bench daemon runs HTTP-disabled over a Unix socket (D-06), so the
// Prometheus /metrics scrape is unreachable; the daemon instead emits this value
// on a single shutdown log line (msg="semantic store reads total") that the
// bench cell scrapes from daemon.log to assert == 0 on the no_semantic arm
// (D-05). Nil-receiver-safe (returns 0); a counter only ever increments, so the
// float-to-int truncation is exact for any count the process can reach.
func (m *Metrics) SemanticStoreReadsValue() int {
	if m == nil || m.SemanticStoreReads == nil {
		return 0
	}
	var metric dto.Metric
	if err := m.SemanticStoreReads.Write(&metric); err != nil {
		return 0
	}
	if metric.Counter == nil || metric.Counter.Value == nil {
		return 0
	}
	return int(*metric.Counter.Value)
}

// --- Phase 59 P02 helper (T-59-02-02 mitigation: bounded-label allowlist) ---

// allowedExtractionLanguages bounds the "language" label of
// helix_semantic_extraction_total. Unknown values are coerced to "other"
// so per-language cardinality stays at 4.
var allowedExtractionLanguages = map[string]struct{}{
	"go":         {},
	"typescript": {},
	"python":     {},
	"other":      {},
}

// allowedExtractionOutcomes bounds the "outcome" label. Unknown values
// drop the emission (closed-enum discipline).
var allowedExtractionOutcomes = map[string]struct{}{
	"ready":       {},
	"partial":     {},
	"unsupported": {},
	"failed":      {},
}

// SemanticExtractionTotal increments helix_semantic_extraction_total.
// Phase 59 P02 / T-59-02-02 mitigation:
//   - language ∉ {go,typescript,python,other} is COERCED to "other".
//   - outcome ∉ {ready,partial,unsupported,failed} DROPS the emission.
//
// Cardinality bound: 4 languages × 4 outcomes = 16 combos.
func (m *Metrics) SemanticExtractionTotal(language, outcome string) {
	if _, ok := allowedExtractionLanguages[language]; !ok {
		language = "other"
	}
	if _, ok := allowedExtractionOutcomes[outcome]; !ok {
		return
	}
	m.SemanticExtraction.WithLabelValues(language, outcome).Inc()
}

// --- Phase 60 D-07 helper (60-05B, drop-on-unknown closed-enum discipline) ---

// SemanticLiveUpdatesInc increments helix_semantic_live_updates_total.
// Phase 60 D-07 closed-enum bound:
//   - kind ∈ {file_created, file_modified, file_deleted, file_renamed,
//     helix_edit, bulk_update} — mirrors live.SourceChangeKind verbatim.
//   - outcome ∈ {applied, no_op, error, dropped}.
//
// Unknown values DROP the emission (matches the EditOutcomeInc pattern at
// lines 339-351). Cardinality bound: 6 × 4 = 24 combos.
//
// Wiring: the live handler (`internal/semantic/live/handler`) calls this
// after each Dispatch — outcome=applied for a successful Commit,
// outcome=no_op when Coalescer.flush short-circuits an empty batch,
// outcome=error when Dispatch returns a non-nil error, outcome=dropped
// when a non-blocking enqueue is refused (Coalescer.Enqueue
// select-default-drop branch).
func (m *Metrics) SemanticLiveUpdatesInc(kind, outcome string) {
	switch kind {
	case "file_created", "file_modified", "file_deleted", "file_renamed",
		"helix_edit", "bulk_update":
	default:
		return
	}
	switch outcome {
	case "applied", "no_op", "error", "dropped":
	default:
		return
	}
	m.SemanticLiveUpdates.WithLabelValues(kind, outcome).Inc()
}

// --- Phase 68 helpers (drop-on-unknown closed-enum discipline) ---
//
// Mirrors SemanticLiveUpdatesInc: closed enums enforced at the emission
// boundary, unknowns dropped to keep cardinality bounded (T-68-09 mitigation).

// LiveFileFactDiffInc increments helix_live_filefactdiff_total.
// Phase 68 D-07 closed-enum bound:
//   - tier ∈ {"full","added-only","synthetic"}
//   - repo is a bounded per-workspace identifier.
//
// Unknown tier values DROP the emission (matches the SemanticLiveUpdatesInc
// pattern). Cardinality bound: 3 × N repos.
func (m *Metrics) LiveFileFactDiffInc(tier, repo string) {
	if m == nil || m.LiveFileFactDiff == nil {
		return
	}
	switch tier {
	case "full", "added-only", "synthetic":
	default:
		return
	}
	m.LiveFileFactDiff.WithLabelValues(tier, repo).Inc()
}

// LiveFileFactDiffSyntheticReasonInc increments
// helix_live_filefactdiff_synthetic_reason_total.
// Phase 68 D-08 closed-enum bound:
//   - reason ∈ {"cold_start","extract_failed","extract_unsupported"}.
//
// Unknown reason values DROP the emission.
func (m *Metrics) LiveFileFactDiffSyntheticReasonInc(reason string) {
	if m == nil || m.LiveFileFactDiffSynRsn == nil {
		return
	}
	switch reason {
	case "cold_start", "extract_failed", "extract_unsupported":
	default:
		return
	}
	m.LiveFileFactDiffSynRsn.WithLabelValues(reason).Inc()
}

// --- Phase 70 D-04 helper (drop-on-unknown closed-enum discipline) ---

// IncrementalRefreshFallbackInc increments helix_incremental_refresh_fallback_total.
// Phase 70 D-04 closed-enum bound:
//   - reason ∈ {cold_start, overlay_rotated, empty_overlay, error} — the
//     four scenarios under which the refresh tool falls back to a
//     full-walk instead of the incremental drain path.
//   - repo is a bounded per-workspace identifier (caller responsibility
//     to keep cardinality reasonable; mirrors LiveFileFactDiffInc).
//
// Unknown reason values DROP the emission (matches the
// LiveFileFactDiffInc / SemanticLiveUpdatesInc pattern). Nil-safe.
func (m *Metrics) IncrementalRefreshFallbackInc(reason, repo string) {
	if m == nil || m.IncrementalRefreshFallback == nil {
		return
	}
	switch reason {
	case IncrementalRefreshFallbackReasonColdStart,
		IncrementalRefreshFallbackReasonOverlayRotated,
		IncrementalRefreshFallbackReasonEmptyOverlay,
		IncrementalRefreshFallbackReasonError:
	default:
		return
	}
	m.IncrementalRefreshFallback.WithLabelValues(reason, repo).Inc()
}

// --- Phase 61 P03 helpers (drop-on-unknown closed-enum discipline) ---
//
// Mirrors the LSPoolLookup / EditOutcomeInc / SemanticLiveUpdatesInc
// pattern: closed enums enforced at the emission boundary, unknowns dropped
// to keep cardinality bounded (T-61-03-01 mitigation).

// LSPEnrichmentTotal increments helix_semantic_lsp_enrichment_total.
// outcome ∈ {"applied","partial_budget","partial_preempted",
// "partial_lsp_unavailable","dropped"} — the lspenrich.Outcome enum from
// internal/semantic/lspenrich/types.go verbatim. Unknown outcomes drop the
// emission. language is bounded in practice by the LS-installed surface.
func (m *Metrics) LSPEnrichmentTotal(language, outcome string) {
	switch outcome {
	case "applied", "partial_budget", "partial_preempted", "partial_lsp_unavailable", "dropped":
	default:
		return
	}
	m.LSPEnrichmentTotalVec.WithLabelValues(language, outcome).Inc()
}

// LSPEnrichmentDuration observes a per-file cascade duration in seconds.
// Single label "language"; no enum guard (the histogram is per-call, not
// per-result). Negative durations are dropped.
func (m *Metrics) LSPEnrichmentDuration(language string, secs float64) {
	if secs < 0 {
		return
	}
	m.LSPEnrichmentDurationVec.WithLabelValues(language).Observe(secs)
}

// LSPEnrichmentErrors increments helix_semantic_lsp_enrichment_errors_total.
// outcome ∈ {"timeout","ls_crash","circuit_open","readiness_timeout",
// "other"}. Unknown outcomes drop the emission.
func (m *Metrics) LSPEnrichmentErrors(language, outcome string) {
	switch outcome {
	case "timeout", "ls_crash", "circuit_open", "readiness_timeout", "other":
	default:
		return
	}
	m.LSPEnrichmentErrorsVec.WithLabelValues(language, outcome).Inc()
}

// LSPEnrichmentLaneDepth sets the per-lane queue depth gauge.
// lane ∈ {"high","background"}. Unknown lanes drop the emission.
func (m *Metrics) LSPEnrichmentLaneDepth(lane string, depth int) {
	switch lane {
	case "high", "background":
	default:
		return
	}
	m.LSPEnrichmentLaneDepthVec.WithLabelValues(lane).Set(float64(depth))
}

// LSPEnrichmentBulkSuppressed increments the bulk-suppressed counter by n.
// Negative or zero values are no-ops.
func (m *Metrics) LSPEnrichmentBulkSuppressed(n int) {
	if n <= 0 {
		return
	}
	m.LSPEnrichmentBulkSuppressedCtr.Add(float64(n))
}

// --- Phase 62 P02 helpers (drop-on-unknown closed-enum discipline, T5) ---
//
// All helpers validate label values against package-private allowlists
// BEFORE calling WithLabelValues — unknown values drop the emission to
// keep cardinality bounded (T-62-02-D2 mitigation).

var pagerankScopes = map[string]struct{}{
	"incremental": {},
	"full":        {},
}

var pagerankProjections = map[string]struct{}{
	"call_graph": {},
}

var graphScoreStatuses = map[string]struct{}{
	"exact":       {},
	"approximate": {},
	"stale":       {},
	"missing":     {},
}

var graphRepairOutcomes = map[string]struct{}{
	"applied":           {},
	"frontier_overflow": {},
	"preempted":         {},
	"error":             {},
	// 62-08: surfaces the four deferred Phase 64 read paths on
	// rankStoreAdapter (62-VERIFICATION.md gap truth #21 / WR-05) so
	// dashboards distinguish "no data yet" from clean repair.
	"stub_no_data": {},
}

// typesConfidenceTiers is the SPEC §38.2 ladder rendered as bucketed
// strings. Helper coerces unknowns to nothing (drop-on-unknown).
var typesConfidenceTiers = map[string]struct{}{
	"1.00": {},
	"0.90": {},
	"0.80": {},
	"0.70": {},
	"0.60": {},
	"0.45": {},
	"0.20": {},
}

// SemanticGraphPagerankObserve records a PageRank pass duration.
// scope ∈ {incremental, full}; projection ∈ {call_graph}. Unknown values
// drop the emission. Negative seconds drop.
func (m *Metrics) SemanticGraphPagerankObserve(scope, projection string, seconds float64) {
	if seconds < 0 {
		return
	}
	if _, ok := pagerankScopes[scope]; !ok {
		return
	}
	if _, ok := pagerankProjections[projection]; !ok {
		return
	}
	m.SemanticGraphPagerankDurationVec.WithLabelValues(scope, projection).Observe(seconds)
}

// SemanticGraphScoreStatusInc increments the read-time score-status
// counter. projection ∈ {call_graph};
// status ∈ {exact, approximate, stale, missing} (D-07). Unknown values
// drop the emission.
func (m *Metrics) SemanticGraphScoreStatusInc(projection, status string) {
	if _, ok := pagerankProjections[projection]; !ok {
		return
	}
	if _, ok := graphScoreStatuses[status]; !ok {
		return
	}
	m.SemanticGraphScoreStatusVec.WithLabelValues(projection, status).Inc()
}

// SemanticGraphRepairInc increments the ApplyRepair outcome counter.
// outcome ∈ {applied, frontier_overflow, preempted, error, stub_no_data}.
// stub_no_data is emitted by rankStoreAdapter for the four read methods
// that are deferred to Phase 64 (QueryEffectiveGraph, QueryEffectiveAdjacency,
// CountStaleScoreRows, MarkAllScoreRowsStale). See
// internal/daemon/rank_wiring.go for the call sites and 62-VERIFICATION.md
// gap truth #21 (WR-05) for the gap closure context.
func (m *Metrics) SemanticGraphRepairInc(outcome string) {
	if _, ok := graphRepairOutcomes[outcome]; !ok {
		return
	}
	m.SemanticGraphRepairVec.WithLabelValues(outcome).Inc()
}

// SemanticGraphVersionSet publishes the current graph_version for a
// workspace. workspace_label is the bounded hashed/truncated identifier
// the helix_semantic_store_* family already uses; this helper does NOT
// re-validate it (the bound is enforced at the call site, mirroring
// SemanticStoreQuarantineInc).
func (m *Metrics) SemanticGraphVersionSet(workspaceLabel string, gv uint64) {
	m.SemanticGraphVersionGauge.WithLabelValues(workspaceLabel).Set(float64(gv))
}

// SemanticTypesResolutionInc increments the type-resolver outcome
// counter. language is bounded by allowedExtractionLanguages (the
// closed-enum used by the Phase 59 extraction metric); confidenceTier
// must be one of the SPEC §38.2 ladder strings. Unknown values drop.
func (m *Metrics) SemanticTypesResolutionInc(language, confidenceTier string) {
	if _, ok := allowedExtractionLanguages[language]; !ok {
		language = "other"
	}
	if _, ok := typesConfidenceTiers[confidenceTier]; !ok {
		return
	}
	m.SemanticTypesResolutionVec.WithLabelValues(language, confidenceTier).Inc()
}

// compactionOutcomes is the closed-enum allowlist for the compaction
// duration histogram. Phase 63 D-04 / T-63-02-06 mitigation: cardinality
// is bounded at construction time by this set.
var compactionOutcomes = map[string]struct{}{
	"success":         {},
	"partial":         {},
	"skipped_blocked": {},
	"error":           {},
}

// vacuumOutcomes is the closed-enum allowlist for the VACUUM duration
// histogram.
var vacuumOutcomes = map[string]struct{}{
	"success": {},
	"skipped": {},
	"error":   {},
}

// blockedReasons is the closed-enum allowlist for the
// helix_semantic_compaction_blocked_total counter. Mirrors the gate's
// BlockedReason enum (gate.go).
var blockedReasons = map[string]struct{}{
	"overlay_empty":     {},
	"idle_too_short":    {},
	"edit_tx_active":    {},
	"overlay_tx_active": {},
	"lsp_pending":       {},
	"rank_repairing":    {},
}

// SemanticCompactionObserve records a compaction-cycle duration.
// outcome ∈ {success, partial, skipped_blocked, error}; unknown values
// drop the emission. Negative seconds drop. Phase 63 D-04.
func (m *Metrics) SemanticCompactionObserve(outcome string, seconds float64) {
	if m == nil || m.SemanticCompactionDurationVec == nil {
		return
	}
	if seconds < 0 {
		return
	}
	if _, ok := compactionOutcomes[outcome]; !ok {
		return
	}
	m.SemanticCompactionDurationVec.WithLabelValues(outcome).Observe(seconds)
}

// SemanticCompactionBlocked increments the closed-enum
// "skipped_blocked" sub-counter keyed on BlockedReason. Unknown reasons
// drop. Phase 63 D-04.
func (m *Metrics) SemanticCompactionBlocked(reason string) {
	if m == nil || m.SemanticCompactionBlockedVec == nil {
		return
	}
	if _, ok := blockedReasons[reason]; !ok {
		return
	}
	m.SemanticCompactionBlockedVec.WithLabelValues(reason).Inc()
}

// SemanticVacuumObserve records a VACUUM-cycle duration. outcome ∈
// {success, skipped, error}; unknown values drop the emission. Negative
// seconds drop. Phase 63 D-05.
func (m *Metrics) SemanticVacuumObserve(outcome string, seconds float64) {
	if m == nil || m.SemanticVacuumDurationVec == nil {
		return
	}
	if seconds < 0 {
		return
	}
	if _, ok := vacuumOutcomes[outcome]; !ok {
		return
	}
	m.SemanticVacuumDurationVec.WithLabelValues(outcome).Observe(seconds)
}

// --- Phase 66 P01 helper methods (GUARD-05 receipt counters, drop-unknown closed-enum discipline) ---

// ReceiptIssuedInc increments helix_receipt_issued_total.
// class ∈ {"references_checked","impact_checked","context_gathered",
// "structural_overview","diagnostics_clean"}; any other value is dropped
// (Phase 66 closed enum, T-66-04 mitigation).
func (m *Metrics) ReceiptIssuedInc(class string) {
	switch class {
	case "references_checked", "impact_checked", "context_gathered", "structural_overview", "diagnostics_clean":
	default:
		return
	}
	m.ReceiptIssuedVec.WithLabelValues(class).Inc()
}

// ReceiptExpiredInc increments helix_receipt_expired_total.
// reason ∈ {"ttl","graph_drift","lru_evicted"}; any other value is dropped
// (Phase 66 closed enum, T-66-05 mitigation).
func (m *Metrics) ReceiptExpiredInc(reason string) {
	switch reason {
	case "ttl", "graph_drift", "lru_evicted":
	default:
		return
	}
	m.ReceiptExpiredVec.WithLabelValues(reason).Inc()
}

// ReceiptLookupInc increments helix_receipt_lookup_total.
// outcome ∈ {"hit","miss","expired","scope_mismatch","graph_drift",
// "workspace_mismatch","freshness_rejected","wrong_class"}; any other value
// is dropped (Phase 66 closed enum, T-66-04 mitigation).
func (m *Metrics) ReceiptLookupInc(outcome string) {
	switch outcome {
	case "hit", "miss", "expired", "scope_mismatch", "graph_drift", "workspace_mismatch", "freshness_rejected", "wrong_class":
	default:
		return
	}
	m.ReceiptLookupVec.WithLabelValues(outcome).Inc()
}

// GuardrailEvalTimeoutInc increments helix_guardrail_eval_timeout_total.
// Unlabeled counter per T-66-21 fail-open mitigation (no label cardinality
// risk from timeout sources). Called from GuardrailMiddleware eval timeout branch.
func (m *Metrics) GuardrailEvalTimeoutInc() {
	m.GuardrailEvalTimeout.Inc()
}
