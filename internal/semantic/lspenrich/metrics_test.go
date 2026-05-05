package lspenrich_test

import (
	"testing"

	dto "github.com/prometheus/client_model/go"

	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
)

// gatherFamily returns the named MetricFamily from a Provider's Gatherer
// output, or nil if absent.
func gatherFamily(t *testing.T, m *obs.Metrics, name string) *dto.MetricFamily {
	t.Helper()
	mfs, err := m.Registry().Gather()
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

// findCombo returns the metric whose label set matches all (name,value)
// pairs in want, or nil if no such combo exists.
func findCombo(mf *dto.MetricFamily, want map[string]string) *dto.Metric {
	if mf == nil {
		return nil
	}
	for _, m := range mf.GetMetric() {
		match := true
		got := map[string]string{}
		for _, lp := range m.GetLabel() {
			got[lp.GetName()] = lp.GetValue()
		}
		for k, v := range want {
			if got[k] != v {
				match = false
				break
			}
		}
		if match {
			return m
		}
	}
	return nil
}

func newMetrics(t *testing.T) *obs.Metrics {
	t.Helper()
	// obs.Noop returns a Provider with a real *Metrics (and an owned
	// prometheus.Registry) so tests can Gather() actual series.
	prov := obs.Noop(nil)
	if prov == nil {
		t.Fatal("obs.Noop returned nil")
	}
	return prov.Metrics()
}

// M1: LSPEnrichmentTotal("go","applied") increments the counter by 1 on
// the matching label combo.
func TestLSPEnrichment_M1_TotalIncrements(t *testing.T) {
	m := newMetrics(t)
	sink := lspenrich.NewProdMetricsSink(m)
	sink.LSPEnrichmentTotal("go", "applied")

	mf := gatherFamily(t, m, "helix_semantic_lsp_enrichment_total")
	if mf == nil {
		t.Fatal("helix_semantic_lsp_enrichment_total not registered")
	}
	c := findCombo(mf, map[string]string{"language": "go", "outcome": "applied"})
	if c == nil {
		t.Fatalf("no combo {go,applied} after one increment")
	}
	if got := c.GetCounter().GetValue(); got != 1 {
		t.Errorf("counter = %v, want 1", got)
	}
}

// M2: garbage outcome drops the emission entirely (no series added).
func TestLSPEnrichment_M2_TotalDropsUnknown(t *testing.T) {
	m := newMetrics(t)
	sink := lspenrich.NewProdMetricsSink(m)
	sink.LSPEnrichmentTotal("go", "garbage")

	mf := gatherFamily(t, m, "helix_semantic_lsp_enrichment_total")
	if mf != nil && len(mf.GetMetric()) > 0 {
		t.Errorf("garbage outcome added series: %d combos", len(mf.GetMetric()))
	}
}

// M3: all 5 closed-enum outcome values are accepted.
func TestLSPEnrichment_M3_AllOutcomesAccepted(t *testing.T) {
	m := newMetrics(t)
	sink := lspenrich.NewProdMetricsSink(m)
	for _, oc := range []string{
		"applied", "partial_budget", "partial_preempted",
		"partial_lsp_unavailable", "dropped",
	} {
		sink.LSPEnrichmentTotal("go", oc)
	}
	mf := gatherFamily(t, m, "helix_semantic_lsp_enrichment_total")
	if mf == nil {
		t.Fatal("not registered")
	}
	if got := len(mf.GetMetric()); got != 5 {
		t.Errorf("combo count = %d, want 5", got)
	}
}

// M4: error counter accepts the 5 closed-enum values; unknowns drop.
func TestLSPEnrichment_M4_ErrorsClosedEnum(t *testing.T) {
	m := newMetrics(t)
	sink := lspenrich.NewProdMetricsSink(m)
	for _, oc := range []string{"timeout", "ls_crash", "circuit_open", "readiness_timeout", "other"} {
		sink.LSPEnrichmentErrors("go", oc)
	}
	sink.LSPEnrichmentErrors("go", "garbage") // dropped
	mf := gatherFamily(t, m, "helix_semantic_lsp_enrichment_errors_total")
	if mf == nil {
		t.Fatal("not registered")
	}
	if got := len(mf.GetMetric()); got != 5 {
		t.Errorf("error combo count = %d, want 5 (garbage dropped)", got)
	}
}

// M5: lane-depth gauge accepts {high,background}; unknown lanes drop.
func TestLSPEnrichment_M5_LaneDepthClosedEnum(t *testing.T) {
	m := newMetrics(t)
	sink := lspenrich.NewProdMetricsSink(m)
	sink.LSPEnrichmentLaneDepth("high", 5)
	sink.LSPEnrichmentLaneDepth("background", 10)
	sink.LSPEnrichmentLaneDepth("garbage", 99) // dropped

	mf := gatherFamily(t, m, "helix_semantic_lsp_enrichment_lane_depth")
	if mf == nil {
		t.Fatal("not registered")
	}
	if got := len(mf.GetMetric()); got != 2 {
		t.Errorf("lane combo count = %d, want 2 (garbage dropped)", got)
	}
	high := findCombo(mf, map[string]string{"lane": "high"})
	if high == nil {
		t.Fatal("missing high combo")
	}
	if got := high.GetGauge().GetValue(); got != 5 {
		t.Errorf("high depth = %v, want 5", got)
	}
}

// M6: bulk-suppressed counter increments by n.
func TestLSPEnrichment_M6_BulkSuppressed(t *testing.T) {
	m := newMetrics(t)
	sink := lspenrich.NewProdMetricsSink(m)
	sink.LSPEnrichmentBulkSuppressed(250)
	sink.LSPEnrichmentBulkSuppressed(50)
	sink.LSPEnrichmentBulkSuppressed(0) // no-op
	sink.LSPEnrichmentBulkSuppressed(-1) // no-op

	mf := gatherFamily(t, m, "helix_semantic_lsp_enrichment_bulk_suppressed_total")
	if mf == nil {
		t.Fatal("not registered")
	}
	combos := mf.GetMetric()
	if len(combos) != 1 {
		t.Fatalf("combo count = %d, want 1", len(combos))
	}
	if got := combos[0].GetCounter().GetValue(); got != 300 {
		t.Errorf("counter = %v, want 300", got)
	}
}

// M7: NewProdMetricsSink(nil) returns a noop MetricsSink that does not panic.
// All methods must be callable; the assertion is the absence of panic.
func TestLSPEnrichment_M7_NilSafe(t *testing.T) {
	sink := lspenrich.NewProdMetricsSink(nil)
	if sink == nil {
		t.Fatal("NewProdMetricsSink(nil) returned nil interface")
	}
	// Smoke calls — must not panic.
	sink.LSPEnrichmentTotal("go", "applied")
	sink.LSPEnrichmentDuration("go", 0.5)
	sink.LSPEnrichmentErrors("go", "timeout")
	sink.LSPEnrichmentLaneDepth("high", 1)
	sink.LSPEnrichmentBulkSuppressed(1)

	// Compile-time assertion: the returned interface satisfies MetricsSink.
	var _ lspenrich.MetricsSink = sink
}
