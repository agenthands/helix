// Package compact: Phase 63 P63-02 Task 2 — compaction worker, gate,
// and VACUUM no-op.
//
// accessors.go declares the read-only consumer-side interfaces the
// CompactionGate depends on. Each interface is satisfied by structural
// typing on its producer package (Task 1's accessor additions on
// coalescer / store / lspenrich / graph / kernel); compact does NOT
// import the producers directly, so this package can be tested with
// fakes by every test seam.
//
// CONTEXT.md D-04 hard invariant: every method declared here MUST
// return from in-memory state. NO I/O, no SELECT, no context.WithTimeout
// wrappers. The pre-flight OverlayRowCount SELECT lives on
// OverlayRowAccessor.OverlayRowCount and is consumed ONLY by the
// compactor's runCompaction body — never by the gate's IsReady.

package compact

import (
	"context"
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// CoalescerAccessor exposes the per-coalescer "last flush at" timestamp.
// Backed by Coalescer.lastFlushNanos (atomic.Int64); Task 1 wired this.
type CoalescerAccessor interface {
	LastFlushAt() time.Time
}

// OverlayTxAccessor exposes the per-workspace open-overlay-tx counter.
// Backed by Store.overlayTxCounts (per-repoID atomic.Int32); Task 1
// wired BeginOverlayTx → +1 / releaseLock → -1.
type OverlayTxAccessor interface {
	OverlayTxOpenCount(ws workspace.WorkspaceKey) int
}

// OverlayRowAccessor splits into two clearly-bounded methods so the
// gate's contract (no I/O) is enforced at the type-system level: the
// gate calls ONLY OverlayHasPendingRows; runCompaction calls
// OverlayRowCount.
type OverlayRowAccessor interface {
	// OverlayHasPendingRows is the in-memory atomic-counter proxy.
	// Bumped on every overlay row write; reset by Snapshot.ClearOverlayLE.
	// O(1), no I/O — D-04 hard invariant. Consumed by IsReady.
	OverlayHasPendingRows(repoID string) bool

	// OverlayRowCount is the I/O-bound `SELECT COUNT(*)` per overlay
	// table summed across (files / symbols / references / edges). Used
	// ONLY by the compactor's pre-flight size guard inside
	// runCompaction. NEVER consumed by IsReady.
	OverlayRowCount(ctx context.Context, repoID string, capturedEpoch uint64) (int, error)
}

// LSPQueueAccessor exposes the LSP enrichment queue's depth + last
// enqueue timestamp. Both are O(1) reads (atomic counters / chan len).
type LSPQueueAccessor interface {
	Depth() int
	LastEnqueueAt() time.Time
}

// SchedulerAccessor exposes the rank scheduler's quiescent predicate.
// Combines pendingChanged emptiness with the in-flight-counter Task 1
// added to runIncrementalRepair / maybeFullRecompute.
type SchedulerAccessor interface {
	IsQuiescent(ws workspace.WorkspaceKey) bool
}

// KernelEditAccessor exposes the per-workspace active-edit-tx counter.
// Backed by Kernel.editTx (process-global per-repoID atomic.Int32).
type KernelEditAccessor interface {
	ActiveEditTxCount(ws workspace.WorkspaceKey) int
}

// BleveMeta is the single-writer seam used by the compactor to stamp
// last_compact_at (and only last_compact_at) onto the bleve corpus-state
// meta surface introduced in Phase 69-02. Production wiring binds this
// to *retrieval.Engine (Plan 69-05); tests pass an in-memory fake.
//
// Nil is a valid no-op: Deps.BleveMeta == nil disables the stamp without
// affecting the compaction outcome, so existing compactor_test.go Deps
// literals continue to compile and pass unchanged (Pitfall 5 guard).
//
// The constant key for the stamp lives at
// retrieval.MetaKeyLastCompactAt — never use a string literal here.
type BleveMeta interface {
	SetMeta(key string, val []byte) error
}
