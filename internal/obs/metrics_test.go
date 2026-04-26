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
// metric families: 6 serena_* vectors plus Go + Process runtime collectors.
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
	// Phase 53 D-01..D-11: prime the five new vectors so they show up in Gather().
	m.LSPoolCacheInc("go", "hit", "clean")
	m.RepoMapCacheInc("go", "hit")
	m.RepoMapExtractObserve("go", 0.001)
	m.SessionLifecycleInc("go", "activate")
	m.EditOutcomeInc("replace_symbol_body", "success")

	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	found := map[string]bool{}
	for _, mf := range mfs {
		found[mf.GetName()] = true
	}
	want := []string{
		"serena_tool_calls_total",
		"serena_tool_duration_seconds",
		"serena_lspool_workers",
		"serena_lspool_evictions_total",
		"serena_lspool_circuit_state",
		"serena_lspool_restarts_total",
		// Phase 47 D-07 — closed in this plan (oversight from Phase 47).
		"serena_rename_strategy_total",
		// Phase 53 D-01..D-11 — five new families.
		"serena_lspool_cache_total",
		"serena_repomap_cache_total",
		"serena_repomap_extract_duration_seconds",
		"serena_session_lifecycle_total",
		"serena_edit_outcome_total",
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
	// Phase 53: ensure new helpers also work on the noop sink (D-15).
	m.LSPoolCacheInc("go", "hit", "clean")
	m.RepoMapCacheInc("go", "miss")
	m.RepoMapExtractObserve("go", 0.001)
	m.SessionLifecycleInc("go", "activate")
	m.EditOutcomeInc("replace_symbol_body", "success")
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
