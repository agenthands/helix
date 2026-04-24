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
// CONTEXT.md D-13: serena_lspool_evictions_total carries a "reason" label
// whose value is one of {idle, pressure, crash, shutdown}. "reason" is an
// internal dimensional slice on lspool worker lifecycle, not a request
// label on tool calls, so it lives outside the AllowedLabels set. The
// emission code in plan 11-03 is the single source of truth for the value
// enum — the CI lint only carves the label NAME, not its values.
var carveOuts = map[string]map[string]bool{
	"serena_lspool_evictions_total": {"reason": true},
	// Phase 47 D-07: closed-enum "strategy" label on the rename dispatcher
	// counter. Values enforced at emission (see *Metrics.RenameStrategyInc);
	// the CI lint only carves the label NAME.
	"serena_rename_strategy_total": {"strategy": true},
}

// runtimeFamilyPrefixes names metric families contributed by
// collectors.NewGoCollector() and collectors.NewProcessCollector(). These
// are not Serena-owned, carry whatever labels upstream Prometheus chooses,
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
// Serena-owned metric vector is a member of AllowedLabels (or an explicitly
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
			Name: "serena_drift_test_total",
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
	if !strings.Contains(joined, "serena_drift_test_total") {
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
