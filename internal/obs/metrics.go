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

// AllowedLabels is the bounded-label allowlist enforced at CI time by
// TestMetricsLabelsAllowlist (D-03..D-05). Changing this list requires a
// matching change to metrics_labels_test.go and a plan-level decision.
var AllowedLabels = [5]string{"tool_name", "profile", "mode", "language", "outcome"}

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
		m.SemanticExtraction,
		m.SemanticLiveUpdates,
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
// {"success","no_match","ambiguous_match","validation_failed","ls_error","internal"};
// strategy ∈ {"exact","whitespace_normalized","indentation_flexible","none"};
// any other value is dropped (Phase 53 D-10/D-11/Q-4 closed enums,
// T-53-01 mitigation). tool_name is unbounded by helper but bounded in
// practice by the 7-tool surface (D-12).
func (m *Metrics) EditOutcomeInc(toolName, outcome, strategy string) {
	switch outcome {
	case "success", "no_match", "ambiguous_match", "validation_failed", "ls_error", "internal":
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
