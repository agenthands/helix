// Phase 62 P08: stub-observability tests for rankStoreAdapter.
//
// These tests close 62-VERIFICATION.md gap truth #21 (WR-05): the four
// production read methods on rankStoreAdapter (QueryEffectiveGraph,
// QueryEffectiveAdjacency, CountStaleScoreRows, MarkAllScoreRowsStale)
// are deferred to Phase 64. Until Phase 64 lands, every invocation must
// surface operator-visible signal so dashboards distinguish "no data
// yet" from "successfully repaired empty graph":
//
//   - SemanticGraphRepairInc("stub_no_data") increments on EVERY call.
//   - A WARN log fires AT MOST ONCE per (workspace, method) pair per
//     process — repeated calls from the same workspace are silent so
//     log volume stays bounded.
//   - The once-gate is per-method, not just per-workspace, so the
//     four distinct stubs each log on first call from a workspace.
//
// The test seam newRankStoreAdapterForTest constructs an adapter wired
// with a recording metrics sink + recording slog handler. Production
// wiring happens through newRankStoreAdapter; the seam keeps the unit
// test scope tight (no DuckDB, no scheduler).

package daemon

import (
	"context"
	"log/slog"
	"sync"
	"testing"

	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
)

// recordingMetricsSink captures SemanticGraphRepairInc(outcome) calls
// for assertion. Implements the graphpkg.MetricsSink interface; the
// non-repair methods are no-ops because the stub-observability path
// never invokes them.
type recordingMetricsSink struct {
	mu       sync.Mutex
	outcomes map[string]int
}

func (r *recordingMetricsSink) SemanticGraphRepairInc(outcome string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.outcomes == nil {
		r.outcomes = make(map[string]int)
	}
	r.outcomes[outcome]++
}

func (r *recordingMetricsSink) SemanticGraphVersionSet(string, uint64)              {}
func (r *recordingMetricsSink) SemanticGraphPagerankObserve(string, string, float64) {}
func (r *recordingMetricsSink) SemanticGraphScoreStatusInc(string, string)           {}

// Compile-time assertion that the recording sink satisfies the
// production MetricsSink narrow seam.
var _ graphpkg.MetricsSink = (*recordingMetricsSink)(nil)

// recordingHandler captures slog records emitted by the adapter so the
// once-gate behavior can be asserted by counting WARN-level records.
type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r.Clone())
	return nil
}

func (h *recordingHandler) WithAttrs(_ []slog.Attr) slog.Handler { return h }
func (h *recordingHandler) WithGroup(_ string) slog.Handler      { return h }

func (h *recordingHandler) warnCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	n := 0
	for _, r := range h.records {
		if r.Level >= slog.LevelWarn {
			n++
		}
	}
	return n
}

// TestRankStoreAdapter_StubObservability_Metric asserts each of the four
// stubbed methods increments SemanticGraphRepairInc("stub_no_data") on
// every call, and never increments outcome="applied" (which would mask
// the gap as a clean success).
func TestRankStoreAdapter_StubObservability_Metric(t *testing.T) {
	sink := &recordingMetricsSink{}
	adapter := newRankStoreAdapterForTest(t, sink, slog.New(&recordingHandler{}))

	ctx := context.Background()
	_, _, _ = adapter.QueryEffectiveGraph(ctx, "repo-A", "call_graph")
	_, _, _ = adapter.QueryEffectiveAdjacency(ctx, "repo-A", "call_graph")
	_, _, _ = adapter.CountStaleScoreRows(ctx, "repo-A", "call_graph")
	_ = adapter.MarkAllScoreRowsStale(ctx, "repo-A", "call_graph")

	sink.mu.Lock()
	defer sink.mu.Unlock()
	if got := sink.outcomes["stub_no_data"]; got != 4 {
		t.Fatalf("stub_no_data outcome: got %d, want 4 (one per stubbed method)", got)
	}
	if got := sink.outcomes["applied"]; got != 0 {
		t.Fatalf("applied outcome must remain 0 in stub path; got %d", got)
	}
}

// TestRankStoreAdapter_StubObservability_OnceWarnPerWorkspace asserts
// the WARN log is once-gated per (workspace, method) pair. Five calls
// to QueryEffectiveGraph on repo-A produce ONE WARN; five calls on
// repo-B produce a SECOND WARN; total = 2.
func TestRankStoreAdapter_StubObservability_OnceWarnPerWorkspace(t *testing.T) {
	h := &recordingHandler{}
	adapter := newRankStoreAdapterForTest(t, &recordingMetricsSink{}, slog.New(h))

	ctx := context.Background()
	for i := 0; i < 5; i++ {
		_, _, _ = adapter.QueryEffectiveGraph(ctx, "repo-A", "call_graph")
	}
	for i := 0; i < 5; i++ {
		_, _, _ = adapter.QueryEffectiveGraph(ctx, "repo-B", "call_graph")
	}

	if got := h.warnCount(); got != 2 {
		t.Fatalf("expected exactly 2 WARN records (one per workspace, once-gated), got %d", got)
	}
}

// TestRankStoreAdapter_StubObservability_PerMethodGate asserts the gate
// is per-method, not just per-workspace. Calling four DIFFERENT methods
// on the same workspace produces four distinct WARN records.
func TestRankStoreAdapter_StubObservability_PerMethodGate(t *testing.T) {
	h := &recordingHandler{}
	adapter := newRankStoreAdapterForTest(t, &recordingMetricsSink{}, slog.New(h))

	ctx := context.Background()
	_, _, _ = adapter.QueryEffectiveGraph(ctx, "repo-A", "call_graph")
	_, _, _ = adapter.QueryEffectiveAdjacency(ctx, "repo-A", "call_graph")
	_, _, _ = adapter.CountStaleScoreRows(ctx, "repo-A", "call_graph")
	_ = adapter.MarkAllScoreRowsStale(ctx, "repo-A", "call_graph")

	if got := h.warnCount(); got != 4 {
		t.Fatalf("expected 4 WARN records (one per method, per-method gate), got %d", got)
	}
}

// newRankStoreAdapterForTest is the test seam that returns an adapter
// wired with the recording sink + logger. RED state: returns nil so each
// method call panics on nil dereference. Task 2 implements the real seam
// returning a fully-wired *rankStoreAdapter.
func newRankStoreAdapterForTest(t *testing.T, _ graphpkg.MetricsSink, _ *slog.Logger) *rankStoreAdapter {
	t.Helper()
	// RED: implementation lands in Task 2.
	return nil
}
