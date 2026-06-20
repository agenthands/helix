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
	// Phase 81 ABLATE-06: labelless read counter — no label dimensions, so
	// nothing to carve. The entry exists for documentation parity with the
	// rest of the helix_semantic_store_* family and to mark the metric as
	// intentionally labelless (a faithful "any read" signal).
	"helix_semantic_store_reads_total": {},
	// Phase 59 P02: bounded-label allowlist for tree-sitter extraction
	// outcome counter. "language" ∈ {go, typescript, python, other};
	// "outcome" ∈ {ready, partial, unsupported, failed}. Both labels are
	// already in AllowedLabels (D-04) so this entry exists only for
	// documentation parity with the helper-method drop-on-unknown discipline.
	"helix_semantic_extraction_total": {},
	// Phase 60 D-07 (60-05B): closed-enum "kind" (6 values from
	// live.SourceChangeKind) + closed-enum "outcome" ∈ {applied, no_op,
	// error, dropped} on live-update outcome counter. "outcome" is
	// already in AllowedLabels (D-04); only "kind" is carved out here.
	// Helper SemanticLiveUpdatesInc is the single emission site and
	// drops unknowns.
	"helix_semantic_live_updates_total": {"kind": true},
	// Phase 61 P03: bounded-label families for the LSP enrichment-worker
	// outcome counter, error counter, and lane-depth gauge. "language"
	// and "outcome" are already in AllowedLabels (D-04). "lane" is the
	// only new label name and is also in AllowedLabels per Phase 61 P03
	// (added alongside the gauge), so the entries below exist for
	// documentation parity with the helper-method drop-on-unknown
	// discipline (mirrors helix_semantic_extraction_total above).
	"helix_semantic_lsp_enrichment_total":                 {},
	"helix_semantic_lsp_enrichment_errors_total":          {},
	"helix_semantic_lsp_enrichment_lane_depth":            {},
	"helix_semantic_lsp_enrichment_duration_seconds":      {},
	"helix_semantic_lsp_enrichment_bulk_suppressed_total": {},
	// Phase 62 P02: graph + types metric carve-outs.
	// "scope", "projection", "status", "confidence_tier" are NEW closed-
	// enum label NAMES outside AllowedLabels. "outcome" + "language"
	// already in AllowedLabels (no carve-out needed for those).
	// "workspace_label" mirrors the helix_semantic_store_* family.
	"helix_semantic_graph_pagerank_duration_seconds": {"scope": true, "projection": true},
	"helix_semantic_graph_score_status_total":        {"projection": true, "status": true},
	"helix_semantic_graph_repair_total":              {},
	"helix_semantic_graph_version":                   {"workspace_label": true},
	"helix_semantic_types_resolution_total":          {"confidence_tier": true},
	// Phase 63 P63-02 D-04: compaction + vacuum metrics. The
	// `outcome` label is already in AllowedLabels (D-04 RED tag), so
	// it does not need a carve-out — but `reason` on
	// helix_semantic_compaction_blocked_total is a new closed-enum
	// label name (BlockedReason: overlay_empty, idle_too_short,
	// edit_tx_active, overlay_tx_active, lsp_pending, rank_repairing).
	// Pre-fix this entry was missing AND the family was not primed in
	// TestMetricsLabelsAllowlist; Gather() drops empty families so
	// the lint silently passed at CI time but would have failed at
	// runtime on the first emission. Phase 63 review IN-04 closes
	// both halves: adds the carve-out here and primes the family in
	// the test below.
	"helix_semantic_compaction_blocked_total":     {"reason": true},
	"helix_semantic_compaction_duration_seconds":  {},
	"helix_semantic_vacuum_duration_seconds":      {},
	// Phase 68 D-07: closed-enum "tier" ∈ {full, added-only, synthetic}
	// + bounded "repo" identifier on the precise FileFactDiff outcome
	// counter. Neither label is in AllowedLabels; both are carved out
	// here. Helper LiveFileFactDiffInc is the single emission site and
	// drops unknown tier values.
	"helix_live_filefactdiff_total": {"tier": true, "repo": true},
	// Phase 68 D-08: closed-enum "reason" ∈ {cold_start, extract_failed,
	// extract_unsupported} on the Tier-3 synthetic-reason counter. Helper
	// LiveFileFactDiffSyntheticReasonInc drops unknown reason values.
	"helix_live_filefactdiff_synthetic_reason_total": {"reason": true},
	// Phase 70 D-04: closed-enum "reason" ∈ {cold_start, overlay_rotated,
	// empty_overlay, error} + bounded "repo" identifier on the
	// incremental-refresh fallback counter. Neither label is in
	// AllowedLabels; both are carved out here. Helper
	// IncrementalRefreshFallbackInc is the single emission site and drops
	// unknown reason values.
	"helix_incremental_refresh_fallback_total": {"reason": true, "repo": true},
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
	// Phase 81 ABLATE-06: prime the labelless read counter so Gather()
	// returns its family (empty families are dropped by Gather()).
	m.SemanticStoreReadsInc()
	// Phase 59 P02: prime the tree-sitter extraction vector.
	m.SemanticExtraction.WithLabelValues("go", "ready").Inc()
	// Phase 60 D-07 (60-05B): prime the live-update outcome vector so
	// TestMetricsLabelsAllowlist scans its labels (Pitfall #3 from
	// Phase 53 D-13 — empty families are dropped by Gather()).
	m.SemanticLiveUpdates.WithLabelValues("file_modified", "applied").Inc()
	// Phase 61 P03: prime the LSP enrichment vectors so the lint scans
	// their labels (Pitfall #3 — empty families are dropped by Gather()).
	m.LSPEnrichmentTotal("go", "applied")
	m.LSPEnrichmentDuration("go", 0.05)
	m.LSPEnrichmentErrors("go", "timeout")
	m.LSPEnrichmentLaneDepth("high", 1)
	m.LSPEnrichmentBulkSuppressed(1)
	// Phase 62 P02: prime the graph + types vectors.
	m.SemanticGraphPagerankObserve("incremental", "call_graph", 0.005)
	m.SemanticGraphScoreStatusInc("call_graph", "exact")
	m.SemanticGraphRepairInc("applied")
	m.SemanticGraphRepairInc("stub_no_data") // 62-08 closure: keep family scanned for the new outcome (WR-05).
	m.SemanticGraphVersionSet("ws-aaa", 1)
	m.SemanticTypesResolutionInc("go", "1.00")
	// Phase 63 review IN-04: prime the compaction + vacuum vectors so
	// TestMetricsLabelsAllowlist scans their labels (Pitfall #3 from
	// Phase 53 D-13 — empty families are dropped by Gather()). Without
	// these primings the `reason` label on
	// helix_semantic_compaction_blocked_total escapes the lint until
	// it is emitted at runtime on a live registry.
	m.SemanticCompactionObserve("success", 0.1)
	m.SemanticCompactionBlocked("overlay_empty")
	m.SemanticVacuumObserve("success", 0.1)
	// Phase 68: prime the LiveFileFactDiff vectors so TestMetricsLabelsAllowlist
	// scans their labels (Pitfall #3 — empty families are dropped by Gather()).
	m.LiveFileFactDiffInc("full", "repo-a")
	m.LiveFileFactDiffSyntheticReasonInc("cold_start")
	// Phase 70 D-04: prime the IncrementalRefreshFallback vector so the lint
	// scans its labels (Pitfall #3 — empty families are dropped by Gather()).
	m.IncrementalRefreshFallbackInc(IncrementalRefreshFallbackReasonColdStart, "repo-a")

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

// TestSemanticExtractionTotal_BoundedLabels asserts the helper drops emissions
// with language ∉ {go,typescript,python,other} and outcome ∉ {ready,partial,
// unsupported,failed}. Phase 59 P02 introduces helix_semantic_extraction_total
// with bounded-label allowlist enforced at emission (T-59-02-02 mitigation).
func TestSemanticExtractionTotal_BoundedLabels(t *testing.T) {
	m := newMetrics()

	// Valid emissions land.
	m.SemanticExtractionTotal("go", "ready")
	m.SemanticExtractionTotal("typescript", "partial")
	m.SemanticExtractionTotal("python", "unsupported")
	m.SemanticExtractionTotal("other", "failed")

	// Unknown outcome is dropped (no row added).
	m.SemanticExtractionTotal("go", "exploded")

	// Unknown language is coerced to "other" — still records.
	m.SemanticExtractionTotal("elixir", "ready")

	mf := gatherFamily(t, m.Registry(), "helix_semantic_extraction_total")
	if mf == nil {
		t.Fatal("helix_semantic_extraction_total not registered")
	}
	// 4 valid + 1 coerced ("elixir" → "other"/"ready" is duplicate of "other"/"failed"
	// only by language; with outcome=ready the combo is (other,ready) which is new).
	// Distinct combos so far:
	//   (go,ready), (typescript,partial), (python,unsupported), (other,failed),
	//   (other,ready) ← coerced "elixir"
	got := len(mf.GetMetric())
	if got != 5 {
		t.Errorf("helix_semantic_extraction_total combo count = %d, want 5", got)
	}

	// Bound: 4 langs × 4 outcomes = 16 max combos.
	if got > 16 {
		t.Errorf("helix_semantic_extraction_total cardinality = %d, want ≤ 16 (4 langs × 4 outcomes)", got)
	}
}

// TestMetrics_CardinalityBounds_EditOutcome asserts the closed-enum bound:
// 7 tools × 7 outcomes × 4 strategies = 196 combos.
//
// Cardinality bound: 7 tools (replace_symbol_body, insert_before_symbol,
// insert_after_symbol, rename_symbol, safe_delete_symbol, replace_in_file,
// fuzzy_edit) × 7 outcomes (success, no_match, ambiguous_match,
// validation_failed, ls_error, internal, unsupported) × 4 strategies (exact,
// whitespace_normalized, indentation_flexible, none) = 196. `failed` is not
// emitted as a strategy — fuzzy.StrategyFailed paths map to outcome=no_match
// with strategy=none per Q-4 (D-11 amended 2026-04-30). `unsupported` was
// added in Phase 76 for the structured-edit ablation guards.
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
	outcomes := []string{"success", "no_match", "ambiguous_match", "validation_failed", "ls_error", "internal", "unsupported"}
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
	if got, max := len(mf.GetMetric()), 196; got > max {
		t.Errorf("helix_edit_outcome_total cardinality = %d, want ≤ %d (7 tools × 7 outcomes × 4 strategies)", got, max)
	}
}

// TestMetrics_EditOutcomeInc_UnsupportedCounted asserts that the Phase 76
// "unsupported" outcome — emitted by the structured-edit ablation guards —
// is inside the closed enum and therefore COUNTED rather than silently
// dropped by the EditOutcomeInc default branch (WR-01).
func TestMetrics_EditOutcomeInc_UnsupportedCounted(t *testing.T) {
	m := newMetrics()
	m.EditOutcomeInc("replace_symbol_body", "unsupported", "none")
	mf := gatherFamily(t, m.Registry(), "helix_edit_outcome_total")
	if mf == nil {
		t.Fatal("helix_edit_outcome_total not registered")
	}
	var found bool
	for _, metric := range mf.GetMetric() {
		var outcome string
		for _, lp := range metric.GetLabel() {
			if lp.GetName() == "outcome" {
				outcome = lp.GetValue()
			}
		}
		if outcome == "unsupported" {
			found = true
			if got := metric.GetCounter().GetValue(); got != 1 {
				t.Errorf("unsupported outcome counter = %v, want 1", got)
			}
		}
	}
	if !found {
		t.Error("EditOutcomeInc(\"unsupported\") was dropped; expected it to be counted (WR-01)")
	}
}


// TestSemanticCompactionOutcomeCardinality verifies the closed-enum
// drop-on-unknown discipline for SemanticCompactionObserve. Phase 63
// P63-02 D-04 / T-63-02-06.
func TestSemanticCompactionOutcomeCardinality(t *testing.T) {
	m := newMetrics()
	for _, ok := range []string{"success", "partial", "skipped_blocked", "error"} {
		m.SemanticCompactionObserve(ok, 0.1)
	}
	// Unknown values MUST drop.
	m.SemanticCompactionObserve("made_up", 0.1)
	m.SemanticCompactionObserve("", 0.1)

	mf := gatherFamily(t, m.Registry(), "helix_semantic_compaction_duration_seconds")
	if mf == nil {
		t.Fatal("helix_semantic_compaction_duration_seconds not registered")
	}
	if got := len(mf.GetMetric()); got > 4 {
		t.Errorf("helix_semantic_compaction_duration_seconds cardinality = %d, want ≤ 4 (closed enum)", got)
	}
}

// TestSemanticVacuumOutcomeCardinality mirrors the compaction test for the
// VACUUM histogram. Closed enum {success, skipped, error}.
func TestSemanticVacuumOutcomeCardinality(t *testing.T) {
	m := newMetrics()
	for _, ok := range []string{"success", "skipped", "error"} {
		m.SemanticVacuumObserve(ok, 0.1)
	}
	m.SemanticVacuumObserve("made_up", 0.1)
	m.SemanticVacuumObserve("", 0.1)

	mf := gatherFamily(t, m.Registry(), "helix_semantic_vacuum_duration_seconds")
	if mf == nil {
		t.Fatal("helix_semantic_vacuum_duration_seconds not registered")
	}
	if got := len(mf.GetMetric()); got > 3 {
		t.Errorf("helix_semantic_vacuum_duration_seconds cardinality = %d, want ≤ 3 (closed enum)", got)
	}
}

// TestSemanticCompactionBlockedCardinality verifies BlockedReason closed
// enum on the helix_semantic_compaction_blocked_total counter.
func TestSemanticCompactionBlockedCardinality(t *testing.T) {
	m := newMetrics()
	for _, r := range []string{
		"overlay_empty", "idle_too_short", "edit_tx_active",
		"overlay_tx_active", "lsp_pending", "rank_repairing",
	} {
		m.SemanticCompactionBlocked(r)
	}
	m.SemanticCompactionBlocked("made_up")
	mf := gatherFamily(t, m.Registry(), "helix_semantic_compaction_blocked_total")
	if mf == nil {
		t.Fatal("helix_semantic_compaction_blocked_total not registered")
	}
	if got := len(mf.GetMetric()); got > 6 {
		t.Errorf("helix_semantic_compaction_blocked_total cardinality = %d, want ≤ 6 (closed enum)", got)
	}
}
