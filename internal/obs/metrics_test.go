package obs

import (
	"io"
	"log/slog"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
)

// TestMetrics_NewMetricsReturnsIsolatedRegistry asserts that newMetrics()
// constructs a fresh, non-nil *Metrics with an owned registry that is NOT
// the global DefaultRegisterer (PITFALLS.md meta-rule, T-11-05 mitigation).
func TestMetrics_NewMetricsReturnsIsolatedRegistry(t *testing.T) {
	m := newMetrics()
	if m == nil {
		t.Fatal("newMetrics() returned nil")
	}
	if m.registry == nil {
		t.Fatal("newMetrics().registry is nil")
	}
	if m.Registry() == nil {
		t.Fatal("Metrics.Registry() returned nil")
	}
	// The owned registry must not BE the default registerer. There's no
	// direct identity comparison, but prometheus.DefaultRegisterer is a
	// *Registry; compare pointers via interface equality.
	if any(m.Registry()) == any(prometheus.DefaultRegisterer) {
		t.Fatal("newMetrics() used prometheus.DefaultRegisterer — globals banned")
	}
}

// TestMetrics_RegisteredFamilies asserts the registry gathers the expected
// metric families: 6 helix_* vectors plus Go + Process runtime collectors.
func TestMetrics_RegisteredFamilies(t *testing.T) {
	m := newMetrics()
	// Touch each vector so the family shows up in Gather() even without
	// any scrapes (CounterVec with no labelled series yields an empty family,
	// which Gather() will omit — prime them with WithLabelValues).
	m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
	m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
	m.LSPoolWorkers.WithLabelValues("go").Set(1)
	m.LSPoolEvictions.WithLabelValues("go", "idle").Inc()
	m.LSPoolCircuitState.WithLabelValues("go").Set(0)
	m.LSPoolRestarts.WithLabelValues("go").Inc()
	m.RenameStrategy.WithLabelValues("lsp-native").Inc()
	// Phase 53 D-13: prime the 5 new vectors so each family appears in Gather().
	m.LSPoolLookups.WithLabelValues("go", "hit").Inc()
	m.RepoMapLookups.WithLabelValues("go", "hit").Inc()
	m.RepoMapExtract.WithLabelValues("go", "treesitter").Observe(0.005)
	m.SessionLifecycle.WithLabelValues("started", "stdio").Inc()
	m.EditOutcome.WithLabelValues("replace_symbol_body", "success", "exact").Inc()

	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	found := map[string]bool{}
	for _, mf := range mfs {
		found[mf.GetName()] = true
	}
	want := []string{
		"helix_tool_calls_total",
		"helix_tool_duration_seconds",
		"helix_lspool_workers",
		"helix_lspool_evictions_total",
		"helix_lspool_circuit_state",
		"helix_lspool_restarts_total",
		// Phase 53 D-13: 5 new helix_* vectors closing OBS-03 metric gaps.
		"helix_lspool_lookups_total",
		"helix_repomap_lookups_total",
		"helix_repomap_extract_duration_seconds",
		"helix_session_lifecycle_total",
		"helix_edit_outcome_total",
		// Latent gap surfaced by PATTERNS.md: helix_rename_strategy_total
		// (Phase 47 D-07) was missing from this list. Fixed in Phase 53.
		"helix_rename_strategy_total",
		"go_goroutines",
		"process_resident_memory_bytes",
	}
	for _, name := range want {
		if !found[name] {
			t.Errorf("registry missing metric family %q", name)
		}
	}
}

// TestMetrics_NoopProviderReturnsUsableSink asserts that obs.Noop produces
// a Provider whose Metrics() returns a non-nil sink with functional vectors
// (call sites never branch on nil).
func TestMetrics_NoopProviderReturnsUsableSink(t *testing.T) {
	p := Noop(slog.NewTextHandler(io.Discard, nil))
	if p == nil {
		t.Fatal("Noop returned nil")
	}
	m := p.Metrics()
	if m == nil {
		t.Fatal("Noop().Metrics() returned nil — contract violated")
	}
	// Exercise all vectors — must not panic / nil deref.
	m.ToolCalls.WithLabelValues("t", "p", "m", "go", "success").Inc()
	m.ToolDuration.WithLabelValues("t", "p", "m", "go").Observe(0.001)
	m.LSPoolWorkersSet("go", 1)
	m.LSPoolEviction("go", "idle")
	m.LSPoolCircuitStateSet("go", 2)
	m.LSPoolRestart("go")
	m.RenameStrategyInc("lsp-native")
	// Phase 53 D-13: exercise the 5 new helpers on the noop sink.
	m.LSPoolLookup("go", "hit")
	m.LSPoolLookup("go", "miss")
	m.RepoMapLookup("go", "hit")
	m.RepoMapLookup("go", "miss")
	m.RepoMapExtractObserve("go", "treesitter", 0.005)
	m.RepoMapExtractObserve("go", "lsp", 0.05)
	m.RepoMapExtractObserve("go", "fallback", 0.1)
	m.SessionLifecycleInc("started", "stdio")
	m.SessionLifecycleInc("ended", "stdio")
	m.SessionLifecycleInc("error", "http")
	m.EditOutcomeInc("replace_symbol_body", "success", "exact")
	m.EditOutcomeInc("rename_symbol", "success", "none")
	m.EditOutcomeInc("fuzzy_edit", "no_match", "none")
}

// TestMetrics_MultipleConstructionNoPanic proves that each call to newMetrics()
// gets a fresh registry — double-registration panics would surface here.
func TestMetrics_MultipleConstructionNoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("newMetrics() panicked on repeat call: %v", r)
		}
	}()
	_ = newMetrics()
	_ = newMetrics()
	_ = newMetrics()
}

// TestMetrics_AllowedLabelsShape locks the D-04 allowlist. Phase 61 P03
// added "lane" for the helix_semantic_lsp_enrichment_lane_depth gauge.
// Any future edit to the allowlist must explicitly update this test.
func TestMetrics_AllowedLabelsShape(t *testing.T) {
	want := [6]string{"tool_name", "profile", "mode", "language", "outcome", "lane"}
	if AllowedLabels != want {
		t.Fatalf("AllowedLabels = %v, want %v", AllowedLabels, want)
	}
}

// --- Phase 60 D-07 (60-05B) tests for SemanticLiveUpdatesInc ---
//
// Pattern mirrors the EditOutcomeInc tests above: the helper drops on
// unknown kind OR unknown outcome and only emits when both labels are
// inside the closed enum.

// TestSemanticLiveUpdatesInc_DropsUnknownKind confirms an unknown kind
// label DROPS the emission. Cardinality bound (T-60-05b-04 mitigation
// echo).
func TestSemanticLiveUpdatesInc_DropsUnknownKind(t *testing.T) {
	m := newMetrics()
	m.SemanticLiveUpdatesInc("garbage", "applied")
	mfs, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "helix_semantic_live_updates_total" {
			if len(mf.GetMetric()) != 0 {
				t.Fatalf("unknown kind leaked into metric family: %v", mf.GetMetric())
			}
		}
	}
}

// TestSemanticLiveUpdatesInc_DropsUnknownOutcome mirrors the kind drop
// test on the second label.
func TestSemanticLiveUpdatesInc_DropsUnknownOutcome(t *testing.T) {
	m := newMetrics()
	m.SemanticLiveUpdatesInc("file_modified", "exploded")
	mfs, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "helix_semantic_live_updates_total" {
			if len(mf.GetMetric()) != 0 {
				t.Fatalf("unknown outcome leaked into metric family: %v", mf.GetMetric())
			}
		}
	}
}

// TestSemanticLiveUpdatesInc_AcceptsAllValid exercises the full 6 × 4 =
// 24 cardinality matrix and confirms each combination produces exactly
// one metric line.
func TestSemanticLiveUpdatesInc_AcceptsAllValid(t *testing.T) {
	m := newMetrics()
	kinds := []string{
		"file_created", "file_modified", "file_deleted",
		"file_renamed", "helix_edit", "bulk_update",
	}
	outcomes := []string{"applied", "no_op", "error", "dropped"}
	for _, k := range kinds {
		for _, o := range outcomes {
			m.SemanticLiveUpdatesInc(k, o)
		}
	}
	mfs, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == "helix_semantic_live_updates_total" {
			if got := len(mf.GetMetric()); got != len(kinds)*len(outcomes) {
				t.Fatalf("expected %d metric lines, got %d", len(kinds)*len(outcomes), got)
			}
			return
		}
	}
	t.Fatal("helix_semantic_live_updates_total family not found in Gather() output")
}

// --- Phase 68 D-07/D-08 tests for LiveFileFactDiff helpers ---
//
// Pattern mirrors the SemanticLiveUpdatesInc tests above: closed-enum
// drop-on-unknown for "tier" and "reason"; cardinality bound enforced at
// the emission boundary (T-68-09 mitigation).

// countSamples returns the number of metric lines for the given family,
// or -1 if the family is absent from Gather().
func countSamples(t *testing.T, m *Metrics, family string) int {
	t.Helper()
	mfs, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() == family {
			return len(mf.GetMetric())
		}
	}
	return -1
}

// sampleValue returns the counter value for the given family + label set,
// or -1 if no matching sample exists.
func sampleValue(t *testing.T, m *Metrics, family string, want map[string]string) float64 {
	t.Helper()
	mfs, err := m.registry.Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	for _, mf := range mfs {
		if mf.GetName() != family {
			continue
		}
		for _, sample := range mf.GetMetric() {
			labels := map[string]string{}
			for _, lp := range sample.GetLabel() {
				labels[lp.GetName()] = lp.GetValue()
			}
			match := true
			for k, v := range want {
				if labels[k] != v {
					match = false
					break
				}
			}
			if match {
				return sample.GetCounter().GetValue()
			}
		}
	}
	return -1
}

// TestLiveFileFactDiffInc_KnownTiers exercises every value of the closed
// "tier" enum and asserts each produces exactly one metric sample with
// value 1.
func TestLiveFileFactDiffInc_KnownTiers(t *testing.T) {
	for _, tier := range []string{"full", "added-only", "synthetic"} {
		t.Run(tier, func(t *testing.T) {
			m := newMetrics()
			m.LiveFileFactDiffInc(tier, "repo-a")
			got := sampleValue(t, m, "helix_live_filefactdiff_total",
				map[string]string{"tier": tier, "repo": "repo-a"})
			if got != 1 {
				t.Fatalf("expected counter == 1 for tier=%q, got %v", tier, got)
			}
		})
	}
}

// TestLiveFileFactDiffInc_UnknownTierDropped confirms unknown tier values
// DROP the emission (closed-enum discipline, T-68-09 mitigation).
func TestLiveFileFactDiffInc_UnknownTierDropped(t *testing.T) {
	m := newMetrics()
	m.LiveFileFactDiffInc("nonsense", "repo-a")
	// Family should be absent (no emissions yet).
	if got := countSamples(t, m, "helix_live_filefactdiff_total"); got > 0 {
		t.Fatalf("unknown tier leaked into metric family: %d samples", got)
	}
}

// TestLiveFileFactDiffInc_NilSafe proves the helper is a no-op on a nil
// receiver (matches the SemanticLiveUpdatesInc safety contract).
func TestLiveFileFactDiffInc_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LiveFileFactDiffInc panicked on nil receiver: %v", r)
		}
	}()
	var m *Metrics
	m.LiveFileFactDiffInc("full", "repo-a")
}

// TestLiveFileFactDiffSyntheticReasonInc_KnownReasons exercises every
// value of the closed "reason" enum.
func TestLiveFileFactDiffSyntheticReasonInc_KnownReasons(t *testing.T) {
	for _, reason := range []string{"cold_start", "extract_failed", "extract_unsupported"} {
		t.Run(reason, func(t *testing.T) {
			m := newMetrics()
			m.LiveFileFactDiffSyntheticReasonInc(reason)
			got := sampleValue(t, m,
				"helix_live_filefactdiff_synthetic_reason_total",
				map[string]string{"reason": reason})
			if got != 1 {
				t.Fatalf("expected counter == 1 for reason=%q, got %v", reason, got)
			}
		})
	}
}

// TestLiveFileFactDiffSyntheticReasonInc_UnknownReasonDropped confirms
// unknown reason values DROP the emission.
func TestLiveFileFactDiffSyntheticReasonInc_UnknownReasonDropped(t *testing.T) {
	m := newMetrics()
	m.LiveFileFactDiffSyntheticReasonInc("bogus")
	if got := countSamples(t, m, "helix_live_filefactdiff_synthetic_reason_total"); got > 0 {
		t.Fatalf("unknown reason leaked into metric family: %d samples", got)
	}
}

// TestLiveFileFactDiffSyntheticReasonInc_NilSafe is the nil-receiver
// safety contract for the second helper.
func TestLiveFileFactDiffSyntheticReasonInc_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("LiveFileFactDiffSyntheticReasonInc panicked on nil receiver: %v", r)
		}
	}()
	var m *Metrics
	m.LiveFileFactDiffSyntheticReasonInc("cold_start")
}

// --- Phase 70 D-04: IncrementalRefreshFallback helper tests ---

// TestMetrics_IncrementalRefreshFallbackInc_DropsUnknownReason exercises
// the closed-enum drop-on-unknown discipline: a bogus reason value MUST NOT
// emit, and a valid reason MUST emit exactly one sample.
func TestMetrics_IncrementalRefreshFallbackInc_DropsUnknownReason(t *testing.T) {
	m := newMetrics()
	// Unknown reason drops.
	m.IncrementalRefreshFallbackInc("bogus", "repo-a")
	if got := countSamples(t, m, "helix_incremental_refresh_fallback_total"); got > 0 {
		t.Fatalf("unknown reason leaked into metric family: %d samples", got)
	}
	// Valid reason emits.
	m.IncrementalRefreshFallbackInc(IncrementalRefreshFallbackReasonColdStart, "repo-a")
	got := sampleValue(t, m, "helix_incremental_refresh_fallback_total",
		map[string]string{"reason": "cold_start", "repo": "repo-a"})
	if got != 1 {
		t.Fatalf("expected counter == 1 for reason=cold_start, got %v", got)
	}
}

// TestMetrics_IncrementalRefreshFallbackInc_KnownReasons exercises every
// value of the closed-enum reason set.
func TestMetrics_IncrementalRefreshFallbackInc_KnownReasons(t *testing.T) {
	cases := []string{
		IncrementalRefreshFallbackReasonColdStart,
		IncrementalRefreshFallbackReasonOverlayRotated,
		IncrementalRefreshFallbackReasonEmptyOverlay,
		IncrementalRefreshFallbackReasonError,
	}
	for _, reason := range cases {
		t.Run(reason, func(t *testing.T) {
			m := newMetrics()
			m.IncrementalRefreshFallbackInc(reason, "repo-a")
			got := sampleValue(t, m, "helix_incremental_refresh_fallback_total",
				map[string]string{"reason": reason, "repo": "repo-a"})
			if got != 1 {
				t.Fatalf("expected counter == 1 for reason=%q, got %v", reason, got)
			}
		})
	}
}

// TestMetrics_IncrementalRefreshFallbackInc_NilSafe is the nil-receiver
// safety contract.
func TestMetrics_IncrementalRefreshFallbackInc_NilSafe(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("IncrementalRefreshFallbackInc panicked on nil receiver: %v", r)
		}
	}()
	var m *Metrics
	m.IncrementalRefreshFallbackInc("cold_start", "repo-a")
}
