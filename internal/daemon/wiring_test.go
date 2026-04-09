package daemon

import (
	"testing"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/obs"
)

// Compile-time proof that *obs.Metrics satisfies lspool.MetricsSink.
//
// The lspool.MetricsSink interface in plan 11-03 was frozen against the
// *obs.Metrics helper method signatures declared by plan 11-01. If this
// line stops compiling, the two plans have drifted apart and the fix is
// to ALIGN lspool.MetricsSink (plan 11-03 owns the interface) — NOT to
// mutate internal/obs/metrics.go (plan 11-01 is the upstream contract).
var _ lspool.MetricsSink = (*obs.Metrics)(nil)

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
