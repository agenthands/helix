package obs

import (
	"testing"

	"github.com/postfix/serena/internal/kernel/lspool"
)

// TestMetrics_NoopZeroAllocOnSinkPath asserts the noop-default invariant
// (CONTEXT.md D-15): consumers that wire lspool.NoopSink{} pay zero
// allocations on the emission path. Other package NoopSinks (repomap.NoopSink,
// edit.NoopSink, kernel.NoopSessionSink) land in plan 53-02 and get their
// own alloc-tests in those packages — this plan scopes the assertion to
// lspool.NoopSink only, since that is the sole NoopSink reachable from
// internal/obs without an import cycle.
func TestMetrics_NoopZeroAllocOnSinkPath(t *testing.T) {
	var sink lspool.MetricsSink = lspool.NoopSink{}

	allocs := testing.AllocsPerRun(100, func() {
		sink.LSPoolEviction("go", "idle")
		sink.LSPoolWorkersSet("go", 1)
	})
	if allocs > 0 {
		t.Fatalf("lspool.NoopSink hot path allocated %v allocs/op; want 0", allocs)
	}
}
