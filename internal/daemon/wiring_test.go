package daemon

import (
	"testing"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/obs"
	"github.com/agenthands/helix/internal/repomap"
)

// Compile-time proof that *obs.Metrics satisfies lspool.MetricsSink.
//
// The lspool.MetricsSink interface in plan 11-03 was frozen against the
// *obs.Metrics helper method signatures declared by plan 11-01. If this
// line stops compiling, the two plans have drifted apart and the fix is
// to ALIGN lspool.MetricsSink (plan 11-03 owns the interface) — NOT to
// mutate internal/obs/metrics.go (plan 11-01 is the upstream contract).
var _ lspool.MetricsSink = (*obs.Metrics)(nil)

// Compile-time proof that *obs.Metrics satisfies repomap.MetricsSink (Phase 53 D-15).
//
// The repomap.MetricsSink interface in plan 53-03 was frozen against the
// *obs.Metrics helper method signatures declared by plan 53-01. If this
// line stops compiling, the two plans have drifted apart and the fix is
// to ALIGN repomap.MetricsSink (plan 53-03 owns the interface) — NOT to
// mutate internal/obs/metrics.go (plan 53-01 is the upstream contract).
var _ repomap.MetricsSink = (*obs.Metrics)(nil)

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
}

// TestObsMetricsIsRepoMapSink is the runtime companion to the
// var _ repomap.MetricsSink = (*obs.Metrics)(nil) assertion above.
// Phase 53 D-15.
func TestObsMetricsIsRepoMapSink(t *testing.T) {
	var sink repomap.MetricsSink = obs.Noop(nil).Metrics()
	if sink == nil {
		t.Fatal("obs.Noop(...).Metrics() returned nil sink")
	}
	// Smoke: each method must be callable without panic, including for
	// every closed-enum constant.
	sink.RepoMapLookup("go", repomap.LookupHit)
	sink.RepoMapLookup("go", repomap.LookupMiss)
	sink.RepoMapExtractObserve("go", repomap.ExtractorTreesitter, 0.001)
	sink.RepoMapExtractObserve("go", repomap.ExtractorLSP, 0.05)
	sink.RepoMapExtractObserve("go", repomap.ExtractorFallback, 0.1)
}
