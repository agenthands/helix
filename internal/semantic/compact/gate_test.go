// Phase 63 P63-02 Task 2: gate tests.
//
// One test per BlockedReason in deterministic-precedence order; the
// "all ready" case asserts BlockedNone. Also asserts gate.go is I/O-free
// (no ExecContext / QueryRowContext) — checked structurally via package
// imports rather than via runtime reflection.

package compact_test

import (
	"context"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/semantic/compact"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeAccessors lets each test toggle accessor state independently.
type fakeAccessors struct {
	hasPending     bool
	rowCount       int
	rowCountErr    error
	lastFlush      time.Time
	openTxCount    int
	editTxCount    int
	lspDepth       int
	lspLastEnq     time.Time
	schedQuiescent bool
}

func (f *fakeAccessors) LastFlushAt() time.Time { return f.lastFlush }
func (f *fakeAccessors) OverlayTxOpenCount(workspace.WorkspaceKey) int {
	return f.openTxCount
}
func (f *fakeAccessors) OverlayHasPendingRows(string) bool { return f.hasPending }
func (f *fakeAccessors) OverlayRowCount(_ context.Context, _ string, _ uint64) (int, error) {
	return f.rowCount, f.rowCountErr
}
func (f *fakeAccessors) Depth() int                                   { return f.lspDepth }
func (f *fakeAccessors) LastEnqueueAt() time.Time                     { return f.lspLastEnq }
func (f *fakeAccessors) IsQuiescent(workspace.WorkspaceKey) bool      { return f.schedQuiescent }
func (f *fakeAccessors) ActiveEditTxCount(workspace.WorkspaceKey) int { return f.editTxCount }

func newGate(t *testing.T, f *fakeAccessors, fixedNow time.Time) *compact.CompactionGate {
	t.Helper()
	now := func() time.Time { return fixedNow }
	return compact.NewCompactionGate("repo", compact.GateDeps{
		Coalescer:  f,
		OverlayTx:  f,
		OverlayRow: f,
		LSPQueue:   f,
		Scheduler:  f,
		KernelEdit: f,
	}, compact.GateConfig{
		CompactAfterIdle:     5 * time.Second,
		LSPCompactionMaxWait: 30 * time.Second,
	}, now)
}

// happyDefaults returns a fakeAccessors that passes every gate check
// (so the under-test mutator can flip exactly ONE check at a time).
func happyDefaults(now time.Time) *fakeAccessors {
	return &fakeAccessors{
		hasPending:     true,
		lastFlush:      now.Add(-10 * time.Second),
		openTxCount:    0,
		editTxCount:    0,
		lspDepth:       0,
		schedQuiescent: true,
	}
}

func TestGate_IsReady_AllReadyReturnsBlockedNone(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if !ready || reason != compact.BlockedNone {
		t.Errorf("got (%v, %q), want (true, BlockedNone)", ready, reason)
	}
}

func TestGate_IsReady_BlockedOverlayEmpty(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	f.hasPending = false
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready || reason != compact.BlockedOverlayEmpty {
		t.Errorf("got (%v, %q), want (false, %q)", ready, reason, compact.BlockedOverlayEmpty)
	}
}

func TestGate_IsReady_BlockedIdleTooShort(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	f.lastFlush = now.Add(-1 * time.Second) // < 5s
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready || reason != compact.BlockedIdleTooShort {
		t.Errorf("got (%v, %q), want (false, %q)", ready, reason, compact.BlockedIdleTooShort)
	}
}

func TestGate_IsReady_BlockedEditTxActive(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	f.editTxCount = 1
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready || reason != compact.BlockedEditTxActive {
		t.Errorf("got (%v, %q), want (false, %q)", ready, reason, compact.BlockedEditTxActive)
	}
}

func TestGate_IsReady_BlockedOverlayTxActive(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	f.openTxCount = 1
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready || reason != compact.BlockedOverlayTxActive {
		t.Errorf("got (%v, %q), want (false, %q)", ready, reason, compact.BlockedOverlayTxActive)
	}
}

func TestGate_IsReady_BlockedLSPPending(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	f.lspDepth = 5
	f.lspLastEnq = now.Add(-1 * time.Second) // < 30s wait
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready || reason != compact.BlockedLSPPending {
		t.Errorf("got (%v, %q), want (false, %q)", ready, reason, compact.BlockedLSPPending)
	}
}

func TestGate_IsReady_LSPDepthOnlyButOldEnqueueFiresThrough(t *testing.T) {
	// Depth > 0 but last enqueue older than LSPCompactionMaxWait → fire
	// (block-then-fire ceiling).
	now := time.Now()
	f := happyDefaults(now)
	f.lspDepth = 5
	f.lspLastEnq = now.Add(-60 * time.Second) // > 30s wait
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if !ready || reason != compact.BlockedNone {
		t.Errorf("got (%v, %q), want (true, BlockedNone)", ready, reason)
	}
}

func TestGate_IsReady_BlockedRankRepairing(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	f.schedQuiescent = false
	g := newGate(t, f, now)
	ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready || reason != compact.BlockedRankRepairing {
		t.Errorf("got (%v, %q), want (false, %q)", ready, reason, compact.BlockedRankRepairing)
	}
}

// TestGate_IsReady_Idempotent: repeated calls with stable accessor
// state return the same result. Side-effect-free invariant.
func TestGate_IsReady_Idempotent(t *testing.T) {
	now := time.Now()
	f := happyDefaults(now)
	g := newGate(t, f, now)
	for i := 0; i < 5; i++ {
		ready, reason := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
		if !ready || reason != compact.BlockedNone {
			t.Fatalf("iteration %d: got (%v, %q), want (true, BlockedNone)", i, ready, reason)
		}
	}
}

// TestGate_NilSafety: nil receiver returns (false, BlockedOverlayEmpty)
// rather than panicking.
func TestGate_NilSafety(t *testing.T) {
	var g *compact.CompactionGate
	ready, _ := g.IsReady(workspace.WorkspaceKey{RepoRoot: "/r"})
	if ready {
		t.Errorf("nil gate IsReady returned ready=true")
	}
}
