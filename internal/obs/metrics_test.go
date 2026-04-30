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

// TestMetrics_AllowedLabelsShape locks the D-04 allowlist to exactly five
// entries. Any future edit to the allowlist must explicitly update this test.
func TestMetrics_AllowedLabelsShape(t *testing.T) {
	want := [5]string{"tool_name", "profile", "mode", "language", "outcome"}
	if AllowedLabels != want {
		t.Fatalf("AllowedLabels = %v, want %v", AllowedLabels, want)
	}
}
