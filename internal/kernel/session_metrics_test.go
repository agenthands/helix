package kernel

import "testing"

// TestNoopSessionSink_ZeroAlloc confirms the NoopSessionSink path is
// allocation-free (Phase 53 D-15 invariant: noop default must not
// allocate). This pins the assertion at the kernel-package boundary,
// complementing the lspool.NoopSink, edit.NoopSink, and repomap.NoopSink
// alloc tests.
//
// Phase 53 IN-02 follow-up: previously alloc-tested only at
// internal/obs/metrics_alloc_test.go for lspool; this co-locates the
// kernel.NoopSessionSink assertion.
func TestNoopSessionSink_ZeroAlloc(t *testing.T) {
	var sink SessionMetricsSink = NoopSessionSink{}
	allocs := testing.AllocsPerRun(100, func() {
		sink.SessionLifecycleInc("go", PhaseActivate)
		sink.SessionLifecycleInc("go", PhaseDeactivate)
		sink.SessionLifecycleInc("go", PhaseTimeout)
		sink.SessionLifecycleInc("go", PhaseShutdown)
	})
	if allocs != 0 {
		t.Fatalf("NoopSessionSink.SessionLifecycleInc allocated %v times, want 0", allocs)
	}
}
