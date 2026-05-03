package obs

import (
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
)

// carveOuts holds per-metric-family label carve-outs that are NOT in the
// D-04 AllowedLabels set but are accepted by the CI lint. Each entry is a
// closed enum enforced at emission sites (not at scrape-time), and exists
// only because the dimension is orthogonal to the RED tool-call labels.
//
// CONTEXT.md D-13: helix_lspool_evictions_total carries a "reason" label
// whose value is one of {idle, pressure, crash, shutdown}. "reason" is an
// internal dimensional slice on lspool worker lifecycle, not a request
// label on tool calls, so it lives outside the AllowedLabels set. The
// emission code in plan 11-03 is the single source of truth for the value
// enum — the CI lint only carves the label NAME, not its values.
var carveOuts = map[string]map[string]bool{
	"helix_lspool_evictions_total": {"reason": true},
	// Phase 47 D-07: closed-enum "strategy" label on the rename dispatcher
	// counter. Values enforced at emission (see *Metrics.RenameStrategyInc);
	// the CI lint only carves the label NAME.
	"helix_rename_strategy_total": {"strategy": true},
	// Phase 53 D-04: closed-enum "result" ∈ {hit,miss} on lspool/repomap
	// cache lookup counters. Values enforced at emission; lint carves the NAME only.
	"helix_lspool_lookups_total":  {"result": true},
	"helix_repomap_lookups_total": {"result": true},
	// Phase 53 D-07: closed-enum "extractor" ∈ {treesitter,lsp,fallback}
	// on repomap extraction histogram.
	"helix_repomap_extract_duration_seconds": {"extractor": true},
	// Phase 53 D-08/D-09: closed-enum "phase" ∈ {started,ended,error} and
	// "transport" ∈ {stdio,http} on session lifecycle counter.
	"helix_session_lifecycle_total": {"phase": true, "transport": true},
	// Phase 53 D-11: closed-enum "strategy" ∈ {exact, whitespace_normalized,
	// indentation_flexible, none} on edit-tool outcome counter. Note:
	// "tool_name" and "outcome" are already in AllowedLabels — only "strategy"
	// is carved out here. Per AMENDED D-11 + Q-4 resolution, "ellipsis" is
	// NOT a valid value (no fuzzy.Strategy declares it) and "failed" is not
	// emitted (it maps to outcome=no_match,strategy=none at call sites).
	"helix_edit_outcome_total": {"strategy": true},
	// Phase 57 D-06/D-07: closed-enum "reason" ∈ {corrupt_file,
	// schema_forward_incompat, schema_unreadable, unknown} on semantic store
	// quarantine counter. workspace_label is a bounded hashed identifier
	// (T-57-02-06 mitigation) — also carved out.
	"helix_semantic_store_quarantine_total": {"workspace_label": true, "reason": true},
	// Phase 57: closed-enum "outcome" ∈ {opened, quarantined, created} on
	// semantic store open counter. workspace_label is bounded.
	"helix_semantic_store_open_total": {"workspace_label": true, "outcome": true},
}

// runtimeFamilyPrefixes names metric families contributed by
// collectors.NewGoCollector() and collectors.NewProcessCollector(). These
// are not Helix-owned, carry whatever labels upstream Prometheus chooses,
// and are exempt from the D-04 allowlist.
var runtimeFamilyPrefixes = []string{"go_", "process_"}

func isRuntimeFamily(name string) bool {
	for _, p := range runtimeFamilyPrefixes {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

// lintLabels walks every metric family in the given registry's Gather()
// output and verifies each label name is either in AllowedLabels or in the
// carve-out map for that family. Returns a slice of human-readable errors
// (empty on success) so both the positive and negative tests can share
// this implementation.
func lintLabels(t *testing.T, gatherer prometheus.Gatherer) []string {
	t.Helper()
	mfs, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	allowed := map[string]bool{}
	for _, l := range AllowedLabels {
		allowed[l] = true
	}
	var problems []string
	for _, mf := range mfs {
		name := mf.GetName()
		if isRuntimeFamily(name) {
			continue
		}
		carve := carveOuts[name]
		for _, metric := range mf.GetMetric() {
			for _, lp := range metric.GetLabel() {
				ln := lp.GetName()
				if allowed[ln] {
					continue
				}
				if carve[ln] {
					continue
				}
				problems = append(problems,
					"metric "+name+" uses forbidden label "+ln+
						" — add to AllowedLabels or remove")
			}
		}
	}
	return problems
}

// TestMetricsLabelsAllowlist enforces METRIC-05: every label on every
// Helix-owned metric vector is a member of AllowedLabels (or an explicitly
// carved-out dimension like D-13's lspool eviction reason). Adding a new
// vector with label "user_id" in a future plan must break this test.
func TestMetricsLabelsAllowlist(t *testing.T) {
	m := newMetrics()
	// Prime every vector so Gather() returns a non-empty family for each
	// (CounterVec/HistogramVec/GaugeVec with no observations yields an
	// empty family that Gather() will drop).
	m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
	m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
	m.LSPoolWorkers.WithLabelValues("go").Set(1)
	m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
	m.LSPoolCircuitState.WithLabelValues("go").Set(0)
	m.LSPoolRestarts.WithLabelValues("go").Inc()
	m.RenameStrategy.WithLabelValues("lsp-native").Inc()
	// Phase 53 D-13: prime new vectors so TestMetricsLabelsAllowlist actually
	// scans their labels. RESEARCH Pitfall #3: an unprimed vector silently
	// bypasses lintLabels because Gather() omits empty families.
	m.LSPoolLookups.WithLabelValues("go", "hit").Inc()
	m.RepoMapLookups.WithLabelValues("go", "hit").Inc()
	m.RepoMapExtract.WithLabelValues("go", "treesitter").Observe(0.001)
	m.SessionLifecycle.WithLabelValues("started", "stdio").Inc()
	m.EditOutcome.WithLabelValues("replace_symbol_body", "success", "exact").Inc()
	// Phase 57: prime the semantic-store vectors so TestMetricsLabelsAllowlist
	// scans their labels (Pitfall #3 from Phase 53 D-13 — empty families are
	// dropped by Gather()).
	m.SemanticStoreQuarantine.WithLabelValues("ws-aaa", "corrupt_file").Inc()
	m.SemanticStoreOpen.WithLabelValues("ws-aaa", "opened").Inc()

	problems := lintLabels(t, m.Registry())
	if len(problems) > 0 {
		for _, p := range problems {
			t.Error(p)
		}
	}
}

// TestMetricsLabelsAllowlist_catchesDrift is the negative proof: we
// register a fake vector with a forbidden label ("user_id") on a temporary
// registry and confirm lintLabels() flags it. If this test ever passes
// silently, the lint is broken.
func TestMetricsLabelsAllowlist_catchesDrift(t *testing.T) {
	reg := prometheus.NewRegistry()
	bad := prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "helix_drift_test_total",
			Help: "Deliberately broken vector for drift-detection.",
		},
		[]string{"user_id"},
	)
	reg.MustRegister(bad)
	bad.WithLabelValues("alice").Inc()

	problems := lintLabels(t, reg)
	if len(problems) == 0 {
		t.Fatal("lintLabels() failed to catch forbidden label \"user_id\" — drift detection is broken")
	}
	// The problem message must identify both the metric name and the label
	// so operators can act on a CI failure without a debugger.
	joined := strings.Join(problems, "|")
	if !strings.Contains(joined, "helix_drift_test_total") {
		t.Errorf("problem missing metric name: %v", problems)
	}
	if !strings.Contains(joined, "user_id") {
		t.Errorf("problem missing label name: %v", problems)
	}
}

// Compile-time assertion that the prometheus client_model dto package is
// linked — lintLabels uses the Gatherer interface above, but dto is the
// canonical home for LabelPair etc. and we want the import tracked so
// future refactors don't silently drop it.
var _ = (*dto.MetricFamily)(nil)

// --- Phase 53 cardinality bound tests (D-01..D-12) ---
//
// Each test primes a closed-enum sample of label combinations, calls
// Gather(), and asserts the per-family `len(mf.GetMetric())` (label-combo
// count, NOT scraped-line count) does not exceed the documented ceiling.
// RESEARCH.md Pitfall #1 + C-4: histograms emit (N_buckets+3) lines per
// label-combo, but `len(mf.GetMetric())` returns label-combo count only.

// gatherFamily walks the registry's Gather() output and returns the named
// MetricFamily, or nil if absent. Helper for cardinality assertions.
func gatherFamily(t *testing.T, gatherer prometheus.Gatherer, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := gatherer.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

// TestMetrics_CardinalityBounds_LSPoolLookups asserts the helix_lspool_lookups_total
// label-combo count stays within the per-language × {hit,miss} ceiling (2*N).
// We prime 5 languages × 2 results and assert ≤ 2*52 (registry-wide cap).
func TestMetrics_CardinalityBounds_LSPoolLookups(t *testing.T) {
	m := newMetrics()
	for _, lang := range []string{"go", "rust", "java", "typescript", "python"} {
		m.LSPoolLookups.WithLabelValues(lang, "hit").Inc()
		m.LSPoolLookups.WithLabelValues(lang, "miss").Inc()
	}
	mf := gatherFamily(t, m.Registry(), "helix_lspool_lookups_total")
	if mf == nil {
		t.Fatal("helix_lspool_lookups_total not registered")
	}
	if got, max := len(mf.GetMetric()), 2*52; got > max {
		t.Errorf("helix_lspool_lookups_total cardinality = %d, want ≤ %d (2 × N_languages)", got, max)
	}
}

// TestMetrics_CardinalityBounds_RepoMapLookups mirrors LSPoolLookups: per-language ×
// {hit,miss} ≤ 2*N_languages.
func TestMetrics_CardinalityBounds_RepoMapLookups(t *testing.T) {
	m := newMetrics()
	for _, lang := range []string{"go", "rust", "java", "typescript", "python"} {
		m.RepoMapLookups.WithLabelValues(lang, "hit").Inc()
		m.RepoMapLookups.WithLabelValues(lang, "miss").Inc()
	}
	mf := gatherFamily(t, m.Registry(), "helix_repomap_lookups_total")
	if mf == nil {
		t.Fatal("helix_repomap_lookups_total not registered")
	}
	if got, max := len(mf.GetMetric()), 2*52; got > max {
		t.Errorf("helix_repomap_lookups_total cardinality = %d, want ≤ %d (2 × N_languages)", got, max)
	}
}

// TestMetrics_CardinalityBounds_RepoMapExtract asserts the histogram's label-combo
// count stays within per-language × {treesitter,lsp,fallback} ≤ 3*N_languages.
//
// RESEARCH Pitfall #1 + C-4: scraped-line count is 3*N*(N_buckets+3) but
// `len(mf.GetMetric())` returns label-combo count = 3*N. This test asserts
// the latter — the former is naturally bounded by N_buckets being a constant.
func TestMetrics_CardinalityBounds_RepoMapExtract(t *testing.T) {
	m := newMetrics()
	for _, lang := range []string{"go", "rust", "java"} {
		for _, extractor := range []string{"treesitter", "lsp", "fallback"} {
			m.RepoMapExtract.WithLabelValues(lang, extractor).Observe(0.005)
		}
	}
	mf := gatherFamily(t, m.Registry(), "helix_repomap_extract_duration_seconds")
	if mf == nil {
		t.Fatal("helix_repomap_extract_duration_seconds not registered")
	}
	if got, max := len(mf.GetMetric()), 3*52; got > max {
		t.Errorf("helix_repomap_extract_duration_seconds cardinality = %d, want ≤ %d (3 × N_languages)", got, max)
	}
}

// TestMetrics_CardinalityBounds_SessionLifecycle asserts the closed-enum bound
// 3 phases × 2 transports = 6 combos.
func TestMetrics_CardinalityBounds_SessionLifecycle(t *testing.T) {
	m := newMetrics()
	for _, phase := range []string{"started", "ended", "error"} {
		for _, transport := range []string{"stdio", "http"} {
			m.SessionLifecycle.WithLabelValues(phase, transport).Inc()
		}
	}
	mf := gatherFamily(t, m.Registry(), "helix_session_lifecycle_total")
	if mf == nil {
		t.Fatal("helix_session_lifecycle_total not registered")
	}
	if got, max := len(mf.GetMetric()), 6; got > max {
		t.Errorf("helix_session_lifecycle_total cardinality = %d, want ≤ %d (3 phases × 2 transports)", got, max)
	}
}

// TestMetrics_CardinalityBounds_EditOutcome asserts the closed-enum bound:
// 7 tools × 6 outcomes × 4 strategies = 168 combos.
//
// Cardinality bound: 7 tools (replace_symbol_body, insert_before_symbol,
// insert_after_symbol, rename_symbol, safe_delete_symbol, replace_in_file,
// fuzzy_edit) × 6 outcomes (success, no_match, ambiguous_match,
// validation_failed, ls_error, internal) × 4 strategies (exact,
// whitespace_normalized, indentation_flexible, none) = 168. `failed` is not
// emitted as a strategy — fuzzy.StrategyFailed paths map to outcome=no_match
// with strategy=none per Q-4 (D-11 amended 2026-04-30).
func TestMetrics_CardinalityBounds_EditOutcome(t *testing.T) {
	m := newMetrics()
	tools := []string{
		"replace_symbol_body",
		"insert_before_symbol",
		"insert_after_symbol",
		"rename_symbol",
		"safe_delete_symbol",
		"replace_in_file",
		"fuzzy_edit",
	}
	outcomes := []string{"success", "no_match", "ambiguous_match", "validation_failed", "ls_error", "internal"}
	strategies := []string{"exact", "whitespace_normalized", "indentation_flexible", "none"}
	for _, tool := range tools {
		for _, outcome := range outcomes {
			for _, strategy := range strategies {
				m.EditOutcome.WithLabelValues(tool, outcome, strategy).Inc()
			}
		}
	}
	mf := gatherFamily(t, m.Registry(), "helix_edit_outcome_total")
	if mf == nil {
		t.Fatal("helix_edit_outcome_total not registered")
	}
	if got, max := len(mf.GetMetric()), 168; got > max {
		t.Errorf("helix_edit_outcome_total cardinality = %d, want ≤ %d (7 tools × 6 outcomes × 4 strategies)", got, max)
	}
}
