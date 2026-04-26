package repomap

import "testing"

// TestNoopSink_ZeroAlloc confirms the NoopSink path is allocation-free
// (Phase 53 D-15 invariant: noop default must not allocate). This pins
// the assertion at the repomap-package boundary, complementing the
// lspool.NoopSink and edit.NoopSink alloc tests already in place.
//
// Phase 53 IN-02 follow-up: previously alloc-tested only at
// internal/obs/metrics_alloc_test.go for lspool; this co-locates the
// repomap.NoopSink assertion.
func TestNoopSink_ZeroAlloc(t *testing.T) {
	var sink MetricsSink = NoopSink{}
	allocs := testing.AllocsPerRun(100, func() {
		sink.RepoMapCacheInc("go", ResultHit)
		sink.RepoMapCacheInc("go", ResultMiss)
		sink.RepoMapExtractObserve("go", 0.001)
	})
	if allocs != 0 {
		t.Fatalf("NoopSink methods allocated %v times, want 0", allocs)
	}
}
