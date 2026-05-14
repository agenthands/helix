// Phase 63 P63-02 Task 2: compactor unit tests via fakes.
//
// These tests exercise the compaction-control flow without spinning up
// a real DuckDB store. The runCompaction body is exercised in the
// integration smoke test; here we focus on the gate re-check, the
// pre-flight size guard, and the metric outcome labels.

package compact_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/compact"
	"github.com/agenthands/helix/internal/semantic/retrieval"
	"github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeBleveMeta is a in-memory BleveMeta implementation. Used by the
// Phase 69-03 last_compact_at write tests.
type fakeBleveMeta struct {
	mu       sync.Mutex
	data     map[string][]byte
	failKeys map[string]error
	calls    int
}

func newFakeBleveMeta() *fakeBleveMeta {
	return &fakeBleveMeta{data: map[string][]byte{}, failKeys: map[string]error{}}
}

func (f *fakeBleveMeta) SetMeta(key string, val []byte) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if err, ok := f.failKeys[key]; ok && err != nil {
		return err
	}
	cp := make([]byte, len(val))
	copy(cp, val)
	f.data[key] = cp
	return nil
}

func (f *fakeBleveMeta) get(key string) ([]byte, int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data[key], f.calls
}

// fakeMetrics records every observation so tests can assert outcomes.
type fakeMetrics struct {
	mu               sync.Mutex
	compactionCalls  []compactionObs
	compactionBlocks []string
	vacuumCalls      []vacuumObs
}
type compactionObs struct {
	outcome string
	seconds float64
}
type vacuumObs struct {
	outcome string
	seconds float64
}

func (m *fakeMetrics) SemanticCompactionObserve(outcome string, seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compactionCalls = append(m.compactionCalls, compactionObs{outcome, seconds})
}
func (m *fakeMetrics) SemanticCompactionBlocked(reason string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.compactionBlocks = append(m.compactionBlocks, reason)
}
func (m *fakeMetrics) SemanticVacuumObserve(outcome string, seconds float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.vacuumCalls = append(m.vacuumCalls, vacuumObs{outcome, seconds})
}
func (m *fakeMetrics) lastCompaction() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.compactionCalls) == 0 {
		return ""
	}
	return m.compactionCalls[len(m.compactionCalls)-1].outcome
}

// fakeOverlay returns a configurable rowcount + records calls.
type fakeOverlay struct {
	rows                 int
	rowsErr              error
	rowCountCalls        int
	updateLastVacuumCall int
}

func (f *fakeOverlay) OverlayRowCount(_ context.Context, _ string, _ uint64) (int, error) {
	f.rowCountCalls++
	return f.rows, f.rowsErr
}
func (f *fakeOverlay) UpdateLastVacuumAt(_ context.Context, _ string, _ interface{}) error {
	f.updateLastVacuumCall++
	return nil
}

// CurrentOverlayEpoch (Phase 63 review CR-01): returns 0 by default —
// the unit-fake compactor tests do not exercise concurrent overlay
// writers, so the captured-epoch value is irrelevant for these
// scenarios. Tests that need a non-zero epoch can set fakeOverlay.epoch
// and have the method return that value.
func (f *fakeOverlay) CurrentOverlayEpoch(_ context.Context, _ string) (uint64, error) {
	return 0, nil
}

// fakeStore stubs the snapshot-tx surface. Each call increments a
// counter so tests can assert call orderings.
type fakeStore struct {
	mu                 sync.Mutex
	beginCalls         int
	beginErr           error
	writeCalls         int
	writeErr           error
	commitCalls        int
	commitErr          error
	abortCalls         int
	abortReasons       []string
	checkpointCalls    int
	vacuumCalls        int
	vacuumErr          error
	deleteRetentionErr error
	clearOverlayErr    error
}

func (f *fakeStore) BeginSnapshot(_ context.Context, meta store.SnapshotMeta) (*store.Snapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.beginCalls++
	if f.beginErr != nil {
		return nil, f.beginErr
	}
	// We can't construct a real *store.Snapshot from outside the
	// store package; tests that need this construct a real store via
	// the integration smoke test. Unit tests assert outcomes via the
	// metric counts and the BeginSnapshot fail path.
	return nil, errors.New("fakeStore: real *store.Snapshot construction not supported in unit fake")
}
func (f *fakeStore) WriteSnapshotFacts(_ context.Context, _ *store.Snapshot, _ store.Facts) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.writeCalls++
	return f.writeErr
}
func (f *fakeStore) CommitSnapshot(_ context.Context, _ *store.Snapshot, _ store.SnapshotSummary) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.commitCalls++
	return f.commitErr
}
func (f *fakeStore) AbortSnapshot(_ context.Context, _ *store.Snapshot, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.abortCalls++
	f.abortReasons = append(f.abortReasons, reason)
	return nil
}
func (f *fakeStore) Vacuum(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.vacuumCalls++
	return f.vacuumErr
}
func (f *fakeStore) Checkpoint(_ context.Context) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.checkpointCalls++
	return nil
}

// gateAlwaysReady is a CompactionGate stand-in for tests that want to
// exercise the runCompaction body without a real gate.
//
// We construct the gate via NewCompactionGate with happy fakes since
// the gate type is concrete.
func gateAlwaysReady() *compact.CompactionGate {
	now := time.Now()
	deps := compact.GateDeps{
		Coalescer: &fixedFlush{lastFlushAt: now.Add(-time.Hour)},
		OverlayTx: &fixedOpenTx{count: 0},
		OverlayRow: &fixedPending{
			pending: true,
		},
		LSPQueue:   &fixedLSP{depth: 0},
		Scheduler:  &fixedQuiescent{q: true},
		KernelEdit: &fixedEditTx{count: 0},
	}
	return compact.NewCompactionGate("repo", deps, compact.GateConfig{
		CompactAfterIdle:     5 * time.Second,
		LSPCompactionMaxWait: 30 * time.Second,
	}, func() time.Time { return now })
}

type fixedFlush struct{ lastFlushAt time.Time }

func (f *fixedFlush) LastFlushAt() time.Time { return f.lastFlushAt }

type fixedOpenTx struct{ count int }

func (f *fixedOpenTx) OverlayTxOpenCount(workspace.WorkspaceKey) int { return f.count }

type fixedPending struct{ pending bool }

func (f *fixedPending) OverlayHasPendingRows(string) bool { return f.pending }
func (f *fixedPending) OverlayRowCount(_ context.Context, _ string, _ uint64) (int, error) {
	return 0, nil
}

type fixedLSP struct {
	depth   int
	lastEnq time.Time
}

func (f *fixedLSP) Depth() int               { return f.depth }
func (f *fixedLSP) LastEnqueueAt() time.Time { return f.lastEnq }

type fixedQuiescent struct{ q bool }

func (f *fixedQuiescent) IsQuiescent(workspace.WorkspaceKey) bool { return f.q }

type fixedEditTx struct{ count int }

func (f *fixedEditTx) ActiveEditTxCount(workspace.WorkspaceKey) int { return f.count }

func TestCompactor_RunCompaction_PartialOnSizeGuard(t *testing.T) {
	m := &fakeMetrics{}
	o := &fakeOverlay{rows: 999_999} // > MaxOverlayRows default 4000
	s := &fakeStore{}
	c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{
		MaxOverlayRows: 4000,
	}, compact.Deps{
		Gate:       gateAlwaysReady(),
		Store:      s,
		OverlayOps: o,
		Metrics:    m,
	})
	// Drive runCompaction directly via the public OnFlush path, which
	// won't fire a real timer because c.cfg.CompactAfterIdle is now
	// default. Instead, invoke the (unexported) fire() body via a
	// public seam: trigger OnFlush, then sleep less than the timer.
	// Simpler: call NewCompactor's exposed RunOnce-style helper. Since
	// runCompaction is unexported, we use a tiny public seam.
	//
	// Public seam: TestRunCompaction is exposed only through the
	// fact that the timer-driven path is identical in behavior. The
	// alternative is testing through OnFlush + a real timer; for unit
	// scope we compute the outcome by invoking via the
	// PublicTriggerForTest hook in the package (added below).
	c.PublicTriggerForTest()
	if got := m.lastCompaction(); got != "partial" {
		t.Errorf("outcome: got %q, want partial", got)
	}
	if s.beginCalls != 0 {
		t.Errorf("BeginSnapshot calls on partial: got %d, want 0", s.beginCalls)
	}
}

func TestCompactor_RunCompaction_SkippedBlockedWhenGateNotReady(t *testing.T) {
	m := &fakeMetrics{}
	o := &fakeOverlay{rows: 0}
	s := &fakeStore{}

	// Build a gate that blocks on overlay_empty.
	now := time.Now()
	gate := compact.NewCompactionGate("repo", compact.GateDeps{
		OverlayRow: &fixedPending{pending: false},
	}, compact.GateConfig{CompactAfterIdle: 5 * time.Second}, func() time.Time { return now })

	c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{}, compact.Deps{
		Gate:       gate,
		Store:      s,
		OverlayOps: o,
		Metrics:    m,
	})
	c.PublicTriggerForTest()

	if got := m.lastCompaction(); got != "skipped_blocked" {
		t.Errorf("outcome: got %q, want skipped_blocked", got)
	}
	if len(m.compactionBlocks) == 0 || m.compactionBlocks[0] != string(compact.BlockedOverlayEmpty) {
		t.Errorf("blocked reason: got %v, want overlay_empty", m.compactionBlocks)
	}
	if o.rowCountCalls != 0 {
		t.Errorf("OverlayRowCount on skipped: got %d calls, want 0", o.rowCountCalls)
	}
}

// TestRunCompaction_WritesLastCompactAt — Phase 69-03 RED.
//
// Asserts:
//   - Test 1 (writes): a successful "stamp" via the exported test seam
//     PublicStampLastCompactAtForTest writes a unix-ms int64 string under
//     retrieval.MetaKeyLastCompactAt within [start, end].
//   - Test 2 (nil-safe): Deps{BleveMeta: nil} stamp is a no-op (no panic).
//   - Test 3 (non-fatal): a SetMeta error is swallowed (`_ =` at the call
//     site) — stamp returns no error and outcome is unaffected.
//   - Test 4 (failure path not stamped): when BeginSnapshot fails, the
//     compactor never reaches the success-path stamp, so SetMeta is
//     never called. Exercised via the existing fakeStore (BeginSnapshot
//     returns an error today) + a full PublicTriggerForTest invocation.
func TestRunCompaction_WritesLastCompactAt(t *testing.T) {
	t.Run("writes_unix_ms_under_MetaKeyLastCompactAt", func(t *testing.T) {
		bm := newFakeBleveMeta()
		fixedNow := time.Date(2026, 5, 14, 12, 0, 0, 0, time.UTC)
		c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{}, compact.Deps{
			BleveMeta: bm,
		})
		c.SetNow(func() time.Time { return fixedNow })

		c.PublicStampLastCompactAtForTest()

		raw, calls := bm.get(retrieval.MetaKeyLastCompactAt)
		if calls != 1 {
			t.Fatalf("SetMeta call count: got %d, want 1", calls)
		}
		got, err := strconv.ParseInt(string(raw), 10, 64)
		if err != nil {
			t.Fatalf("parse last_compact_at: %v (raw=%q)", err, string(raw))
		}
		if want := fixedNow.UnixMilli(); got != want {
			t.Errorf("last_compact_at unix-ms: got %d, want %d", got, want)
		}
	})

	t.Run("nil_bleve_meta_no_panic", func(t *testing.T) {
		c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{}, compact.Deps{
			BleveMeta: nil,
		})
		// Must not panic.
		c.PublicStampLastCompactAtForTest()
	})

	t.Run("set_meta_error_is_non_fatal", func(t *testing.T) {
		bm := newFakeBleveMeta()
		bm.failKeys[retrieval.MetaKeyLastCompactAt] = errors.New("bleve full")
		c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{}, compact.Deps{
			BleveMeta: bm,
		})
		// Must not panic and must not return an error.
		c.PublicStampLastCompactAtForTest()
		_, calls := bm.get(retrieval.MetaKeyLastCompactAt)
		if calls != 1 {
			t.Errorf("SetMeta call count on error path: got %d, want 1", calls)
		}
	})

	t.Run("failure_path_not_stamped", func(t *testing.T) {
		// BeginSnapshot in fakeStore always errors → outcome=error, never
		// reach the stamp. Assert SetMeta was never called.
		bm := newFakeBleveMeta()
		m := &fakeMetrics{}
		o := &fakeOverlay{rows: 0}
		s := &fakeStore{} // BeginSnapshot fails by construction
		c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{}, compact.Deps{
			Gate:       gateAlwaysReady(),
			Store:      s,
			OverlayOps: o,
			Metrics:    m,
			BleveMeta:  bm,
		})
		c.PublicTriggerForTest()
		if got := m.lastCompaction(); got != "error" {
			t.Errorf("outcome on BeginSnapshot fail: got %q, want error", got)
		}
		if _, calls := bm.get(retrieval.MetaKeyLastCompactAt); calls != 0 {
			t.Errorf("SetMeta should not be called on failure path: got %d calls, want 0", calls)
		}
	})
}

func TestCompactor_OnFlush_ResetsTimer(t *testing.T) {
	// Tight timer — fires within 50ms of OnFlush.
	m := &fakeMetrics{}
	o := &fakeOverlay{rows: 0}
	s := &fakeStore{}
	gate := gateAlwaysReady()
	c := compact.NewCompactor(workspace.WorkspaceKey{RepoRoot: "/r"}, "repo", compact.Config{
		CompactAfterIdle: 30 * time.Millisecond,
	}, compact.Deps{Gate: gate, Store: s, OverlayOps: o, Metrics: m})
	c.OnFlush()
	// Wait for the timer to fire.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if got := m.lastCompaction(); got != "" {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	got := m.lastCompaction()
	if got == "" {
		t.Fatal("compaction never fired after OnFlush+timer")
	}
}
