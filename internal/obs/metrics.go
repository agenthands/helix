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

	// Phase 53 D-01/D-02: lspool cache decisions (result hit|miss; scope clean|dirty|crashed).
	LSPoolCache *prometheus.CounterVec

	// Phase 53 D-01/D-02: repomap tag-cache decisions (result hit|miss).
	RepoMapCache *prometheus.CounterVec

	// Phase 53 D-10/D-11: repomap tag-extraction latency by language (DefBuckets).
	RepoMapExtractDuration *prometheus.HistogramVec

	// Phase 53 D-04/D-05: workspace session lifecycle (phase activate|deactivate|timeout|shutdown).
	SessionLifecycle *prometheus.CounterVec

	// Phase 53 D-07/D-09: edit-tool outcomes (closed tool allowlist; outcome success|fuzzy_applied|refused_ambiguous|failed).
	EditOutcome *prometheus.CounterVec
}

// newMetrics constructs a fresh *Metrics with an owned prometheus.Registry.
// All serena_* vectors plus the Go runtime + Process collectors (D-16) are
// registered. Calling newMetrics() multiple times is safe: each call gets
// its own registry, so double-registration panics cannot occur.
func newMetrics() *Metrics {
	reg := prometheus.NewRegistry() // owned; NOT the prometheus global registerer

	m := &Metrics{
		registry: reg,
		ToolCalls: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_tool_calls_total",
				Help: "MCP tool calls by outcome (RED: errors).",
			},
			[]string{"tool_name", "profile", "mode", "language", "outcome"},
		),
		ToolDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "serena_tool_duration_seconds",
				Help:    "MCP tool call latency in seconds (RED: duration).",
				Buckets: prometheus.DefBuckets, // D-01
			},
			[]string{"tool_name", "profile", "mode", "language"},
		),
		LSPoolWorkers: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "serena_lspool_workers",
				Help: "Active LS workers per language.",
			},
			[]string{"language"},
		),
		LSPoolEvictions: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_lspool_evictions_total",
				Help: "LS worker evictions by reason (idle/pressure/crash/shutdown).",
			},
			// CONTEXT.md D-13: "reason" is a closed 4-value enum enforced at
			// emission sites. It is NOT in the D-04 RED allowlist; the CI label
			// lint carves it out explicitly for this family.
			[]string{"language", "reason"},
		),
		LSPoolCircuitState: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "serena_lspool_circuit_state",
				Help: "LS pool circuit breaker state per language (0=closed, 1=half-open, 2=open).",
			},
			[]string{"language"},
		),
		LSPoolRestarts: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_lspool_restarts_total",
				Help: "LS worker restarts per language.",
			},
			[]string{"language"},
		),
		RenameStrategy: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_rename_strategy_total",
				Help: "rename_symbol successes by strategy (lsp-native | rust-client-side).",
			},
			// "strategy" is a closed-enum dimension carved out of AllowedLabels
			// for this family only (Phase 47 D-07). Enforced at emission sites.
			[]string{"strategy"},
		),
		LSPoolCache: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_lspool_cache_total",
				Help: "LS pool cache decisions by language, result (hit|miss), and scope (clean|dirty|crashed).",
			},
			[]string{"language", "result", "scope"},
		),
		RepoMapCache: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_repomap_cache_total",
				Help: "RepoMap tag cache decisions by language and result (hit|miss).",
			},
			[]string{"language", "result"},
		),
		RepoMapExtractDuration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "serena_repomap_extract_duration_seconds",
				Help:    "RepoMap tag extraction latency by language.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"language"},
		),
		SessionLifecycle: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_session_lifecycle_total",
				Help: "Workspace session lifecycle transitions (activate|deactivate|timeout|shutdown).",
			},
			[]string{"language", "phase"},
		),
		EditOutcome: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "serena_edit_outcome_total",
				Help: "Edit-tool outcomes by tool (closed allowlist) and outcome (success|fuzzy_applied|refused_ambiguous|failed).",
			},
			[]string{"tool", "outcome"},
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
		m.LSPoolCache,
		m.RepoMapCache,
		m.RepoMapExtractDuration,
		m.SessionLifecycle,
		m.EditOutcome,
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

// --- Phase 53 helper methods (FROZEN signatures — plans 53-02/53-03) ---

// LSPoolCacheInc increments the lspool cache decision counter.
// result MUST be {hit, miss}; scope MUST be {clean, dirty, crashed}.
// Unknown values are dropped to preserve bounded cardinality (D-02).
func (m *Metrics) LSPoolCacheInc(language, result, scope string) {
	if result != "hit" && result != "miss" {
		return
	}
	if scope != "clean" && scope != "dirty" && scope != "crashed" {
		return
	}
	m.LSPoolCache.WithLabelValues(language, result, scope).Inc()
}

// RepoMapCacheInc increments the repomap tag-cache decision counter.
// result MUST be {hit, miss}; unknown values are dropped (D-02).
func (m *Metrics) RepoMapCacheInc(language, result string) {
	if result != "hit" && result != "miss" {
		return
	}
	m.RepoMapCache.WithLabelValues(language, result).Inc()
}

// RepoMapExtractObserve records a repomap tag-extraction latency sample for
// a language. language is open (bounded only by the LS catalog) so no enum
// guard is applied (D-10).
func (m *Metrics) RepoMapExtractObserve(language string, seconds float64) {
	m.RepoMapExtractDuration.WithLabelValues(language).Observe(seconds)
}

// SessionLifecycleInc increments the workspace session lifecycle counter.
// phase MUST be {activate, deactivate, timeout, shutdown}; unknown values
// are dropped (D-04).
func (m *Metrics) SessionLifecycleInc(language, phase string) {
	if phase != "activate" && phase != "deactivate" && phase != "timeout" && phase != "shutdown" {
		return
	}
	m.SessionLifecycle.WithLabelValues(language, phase).Inc()
}

// EditOutcomeInc increments the edit-tool outcome counter.
// tool MUST be in the inline closed allowlist below (mirrors edit.AllowedTools
// to be defined in plan 53-02; duplicating the string set here avoids an
// import cycle — internal/obs imports nothing back). outcome MUST be
// {success, fuzzy_applied, refused_ambiguous, failed}. Unknown values for
// either dimension drop silently (D-07/D-08).
//
// Read-only diagnostics tools are INTENTIONALLY excluded from the allowlist
// — they are not edits (RESEARCH.md A3, Open Question 3).
func (m *Metrics) EditOutcomeInc(tool, outcome string) {
	switch tool {
	case "replace_symbol_body", "insert_before_symbol", "insert_after_symbol",
		"rename_symbol", "safe_delete_symbol",
		"replace_in_file", "fuzzy_edit", "create_file":
		// allowed
	default:
		return
	}
	if outcome != "success" && outcome != "fuzzy_applied" &&
		outcome != "refused_ambiguous" && outcome != "failed" {
		return
	}
	m.EditOutcome.WithLabelValues(tool, outcome).Inc()
}
