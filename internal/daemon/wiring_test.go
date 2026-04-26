package daemon

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/postfix/serena/internal/kernel"
	"github.com/postfix/serena/internal/kernel/edit"
	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/obs"
	"github.com/postfix/serena/internal/repomap"
)

// Compile-time proofs that *obs.Metrics satisfies every per-package sink
// interface used by the daemon. If any of these stops compiling, the
// upstream helper signatures in internal/obs/metrics.go and the consumer
// interface have drifted apart — the FIX is to align the consumer
// interface, NOT to mutate internal/obs/metrics.go (the helpers are the
// frozen contract per the design rules in metrics.go).
//
// Plan 11-03 owns the lspool sink, plans 53-02/53-03 own the repomap,
// edit, and kernel session sinks.
var _ lspool.MetricsSink = (*obs.Metrics)(nil)
var _ repomap.MetricsSink = (*obs.Metrics)(nil)
var _ edit.MetricsSink = (*obs.Metrics)(nil)
var _ kernel.SessionMetricsSink = (*obs.Metrics)(nil)

// TestObsMetricsIsLSPoolSink is the runtime companion to the var _
// assertion above. It also exercises obs.Noop so we don't regress on
// obs.Provider.Metrics() returning nil.
func TestObsMetricsIsLSPoolSink(t *testing.T) {
	var sink lspool.MetricsSink = obs.Noop(nil).Metrics()
	if sink == nil {
		t.Fatal("obs.Noop(...).Metrics() returned nil sink")
	}
	// Smoke: each method must be callable without panic.
	sink.LSPoolWorkersSet("go", +1)
	sink.LSPoolWorkersSet("go", -1)
	sink.LSPoolEviction("go", lspool.EvictIdle)
	sink.LSPoolCircuitStateSet("go", lspool.CircuitClosed)
	sink.LSPoolRestart("go")
	sink.LSPoolCacheInc("go", lspool.ResultHit, lspool.ScopeClean)
}

// TestObsMetricsIsRepoMapSink — runtime companion to the repomap var _
// assertion. Exercises hit, miss, and extract-observe paths against a
// noop provider.
func TestObsMetricsIsRepoMapSink(t *testing.T) {
	var sink repomap.MetricsSink = obs.Noop(nil).Metrics()
	if sink == nil {
		t.Fatal("obs.Noop(...).Metrics() returned nil sink")
	}
	sink.RepoMapCacheInc("go", repomap.ResultHit)
	sink.RepoMapCacheInc("go", repomap.ResultMiss)
	sink.RepoMapExtractObserve("go", 0.001)
}

// TestObsMetricsIsEditSink — runtime companion to the edit var _
// assertion. Exercises every Outcome* enum value against a noop
// provider.
func TestObsMetricsIsEditSink(t *testing.T) {
	var sink edit.MetricsSink = obs.Noop(nil).Metrics()
	if sink == nil {
		t.Fatal("obs.Noop(...).Metrics() returned nil sink")
	}
	sink.EditOutcomeInc("replace_symbol_body", edit.OutcomeSuccess)
	sink.EditOutcomeInc("replace_in_file", edit.OutcomeFuzzyApplied)
	sink.EditOutcomeInc("fuzzy_edit", edit.OutcomeRefusedAmbiguous)
	sink.EditOutcomeInc("create_file", edit.OutcomeFailed)
}

// TestObsMetricsIsSessionSink — runtime companion to the kernel session
// var _ assertion. Exercises every Phase* enum value against a noop
// provider.
func TestObsMetricsIsSessionSink(t *testing.T) {
	var sink kernel.SessionMetricsSink = obs.Noop(nil).Metrics()
	if sink == nil {
		t.Fatal("obs.Noop(...).Metrics() returned nil sink")
	}
	sink.SessionLifecycleInc("go", kernel.PhaseActivate)
	sink.SessionLifecycleInc("go", kernel.PhaseDeactivate)
	sink.SessionLifecycleInc("go", kernel.PhaseTimeout)
	sink.SessionLifecycleInc("go", kernel.PhaseShutdown)
}

// TestEditOutcomeAllowlist_DriftGuard locks the WR-03/IN-03 invariant:
// every tool in edit.AllowedTools MUST also appear in the inline switch
// in (*obs.Metrics).EditOutcomeInc — drift is otherwise silent (the
// helper-side guard drops unknown tools without logging).
//
// The test feeds each entry of edit.AllowedTools through a real
// (registered) *obs.Metrics and asserts the labelled series increments.
// If a tool is added to edit.AllowedTools but not to the obs switch, the
// counter stays at 0 and this test fails loudly.
func TestEditOutcomeAllowlist_DriftGuard(t *testing.T) {
	provider := obs.Noop(nil)
	metrics := provider.Metrics()
	if metrics == nil {
		t.Fatal("obs.Noop(...).Metrics() returned nil")
	}

	// Sanity: allowlist is non-empty.
	if len(edit.AllowedTools) == 0 {
		t.Fatal("edit.AllowedTools is empty — allowlist must enumerate at least one edit tool")
	}

	for tool := range edit.AllowedTools {
		// Use a fresh outcome label per call so each (tool, outcome)
		// series is unambiguous.
		metrics.EditOutcomeInc(tool, edit.OutcomeSuccess)
		got := testutil.ToFloat64(metrics.EditOutcome.WithLabelValues(tool, edit.OutcomeSuccess))
		if got != 1 {
			t.Errorf("edit.AllowedTools entry %q is NOT in the obs.Metrics.EditOutcomeInc inline switch — series stayed at %v after one increment. The two allowlists have drifted; align internal/obs/metrics.go EditOutcomeInc with internal/kernel/edit/metrics.go AllowedTools.", tool, got)
		}
	}
}
