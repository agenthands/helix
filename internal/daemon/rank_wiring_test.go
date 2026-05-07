// Phase 64 P64-08 stub-collapse: the previous Phase 62 P08 stub-observability
// tests asserted that the four rankStoreAdapter read methods incremented
// SemanticGraphRepairInc("stub_no_data") on every call and emitted a
// once-WARN log per (workspace, method) pair. Phase 64-02 shipped the real
// *Store implementations of QueryEffectiveAdjacency / CountStaleScoreRows /
// MarkAllScoreRowsStale, and Phase 64-08 collapsed the four stubs to
// delegate to *Store directly. The "stub_no_data" canary is therefore
// retired — the methods now flow real data, and the once-WARN log no
// longer fires. The stubObserve harness on rankStoreAdapter is kept on
// the type for forward-compatibility (a future closed-enum signal can
// re-arm it without a constructor change), but the four read paths no
// longer reach it.
//
// The compile-time interface guards in rank_wiring.go (lines 420-424) plus
// the Phase 64-02 *Store unit tests (internal/semantic/store/effective_graph_test.go)
// cover what the deleted tests previously asserted: that the adapter
// satisfies graph.SchedulerStore and that the underlying *Store methods
// behave correctly. No test deletion gap.

package daemon
