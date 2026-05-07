// Package compact: Phase 63 P63-02 Task 2.
//
// gate.go owns CompactionGate.IsReady — the single source of truth that
// answers "is the compactor allowed to fire RIGHT NOW?".
//
// CONTEXT.md D-04 hard invariants enforced at this boundary:
//
//   - IsReady is idempotent and side-effect-free.
//   - IsReady makes ZERO I/O calls. Every accessor returns from
//     in-memory atomic state. The pre-flight OverlayRowCount SELECT
//     lives ONLY inside runCompaction's size guard.
//   - BlockedReason is a closed enum (no free-text) so the metric
//     label cardinality stays bounded.
//   - Order of checks is deterministic — RESEARCH.md Pattern 2
//     "deterministic precedence". A change in order is a behavioral
//     change reviewable in source.

package compact

import (
	"time"

	"github.com/agenthands/helix/internal/workspace"
)

// BlockedReason is the closed-enum gate-decision label.
type BlockedReason string

// The closed-enum constants. BlockedNone signals "ready to fire";
// every other value identifies the FIRST blocking check in the
// deterministic precedence order.
const (
	BlockedNone            BlockedReason = ""
	BlockedOverlayEmpty    BlockedReason = "overlay_empty"
	BlockedIdleTooShort    BlockedReason = "idle_too_short"
	BlockedEditTxActive    BlockedReason = "edit_tx_active"
	BlockedOverlayTxActive BlockedReason = "overlay_tx_active"
	BlockedLSPPending      BlockedReason = "lsp_pending"
	BlockedRankRepairing   BlockedReason = "rank_repairing"
)

// GateConfig is the subset of compact.Config the gate needs.
type GateConfig struct {
	CompactAfterIdle     time.Duration
	LSPCompactionMaxWait time.Duration
}

// GateDeps bundles the read-only accessors the gate consumes.
type GateDeps struct {
	Coalescer  CoalescerAccessor
	OverlayTx  OverlayTxAccessor
	OverlayRow OverlayRowAccessor
	LSPQueue   LSPQueueAccessor
	Scheduler  SchedulerAccessor
	KernelEdit KernelEditAccessor
}

// CompactionGate aggregates the read-only accessors and applies the
// deterministic-precedence IsReady decision.
type CompactionGate struct {
	deps GateDeps
	cfg  GateConfig
	now  func() time.Time
	// repoID is the workspace identifier the in-memory pending-rows
	// proxy keys on (the underlying counter map is per-repoID).
	repoID string
}

// NewCompactionGate constructs a gate. now == nil → defaults to
// time.Now. The deps fields are nil-tolerant individually so partially
// wired test fakes can omit accessors that aren't relevant to the
// scenario under test.
func NewCompactionGate(repoID string, deps GateDeps, cfg GateConfig, now func() time.Time) *CompactionGate {
	if now == nil {
		now = time.Now
	}
	return &CompactionGate{deps: deps, cfg: cfg, now: now, repoID: repoID}
}

// IsReady returns (true, BlockedNone) when the compactor is allowed to
// fire, or (false, BlockedReason) identifying the FIRST blocking check.
// CONTEXT.md D-04 hard invariant: side-effect-free, NO I/O, every
// accessor returns from in-memory state.
//
// Precedence order (load-bearing — DO NOT reorder without auditing the
// gate_test.go cases):
//
//  1. BlockedOverlayEmpty   — nothing to compact (in-memory proxy).
//  2. BlockedIdleTooShort   — coalescer flushed too recently.
//  3. BlockedEditTxActive   — an edit-tool Handle is in flight.
//  4. BlockedOverlayTxActive — an overlay-write tx is open.
//  5. BlockedLSPPending     — LSP enrichment work is recent + queued.
//  6. BlockedRankRepairing  — rank scheduler is mid-repair.
//  7. BlockedNone           — fire.
func (g *CompactionGate) IsReady(ws workspace.WorkspaceKey) (bool, BlockedReason) {
	if g == nil {
		return false, BlockedOverlayEmpty
	}

	// 1. Overlay must contain pending work.
	if g.deps.OverlayRow != nil {
		if !g.deps.OverlayRow.OverlayHasPendingRows(g.repoID) {
			return false, BlockedOverlayEmpty
		}
	}

	// 2. Coalescer must have been quiet for at least CompactAfterIdle.
	if g.deps.Coalescer != nil {
		last := g.deps.Coalescer.LastFlushAt()
		// Zero last means "never flushed" — a never-flushed coalescer
		// can't have pending overlay rows; if we got here, treat zero
		// as "ready" (the check above passed because pending > 0; the
		// coalescer is effectively dormant).
		if !last.IsZero() {
			if g.now().Sub(last) < g.cfg.CompactAfterIdle {
				return false, BlockedIdleTooShort
			}
		}
	}

	// 3. Active edit-tool Handle in flight.
	if g.deps.KernelEdit != nil {
		if g.deps.KernelEdit.ActiveEditTxCount(ws) > 0 {
			return false, BlockedEditTxActive
		}
	}

	// 4. Overlay-write tx open.
	if g.deps.OverlayTx != nil {
		if g.deps.OverlayTx.OverlayTxOpenCount(ws) > 0 {
			return false, BlockedOverlayTxActive
		}
	}

	// 5. LSP enrichment recent + still queued. We honor both: a recent
	// enqueue alone is not enough (the worker might already be done),
	// nor is a non-zero depth (the worker may be running and will
	// drain shortly). Block when BOTH a recent enqueue AND non-zero
	// depth are present; honor LSPCompactionMaxWait as the absolute
	// ceiling (block-then-fire). Conservative interpretation that
	// keeps the gate from blocking forever on a stuck worker.
	if g.deps.LSPQueue != nil {
		depth := g.deps.LSPQueue.Depth()
		if depth > 0 {
			lastEnq := g.deps.LSPQueue.LastEnqueueAt()
			if !lastEnq.IsZero() && g.now().Sub(lastEnq) < g.cfg.LSPCompactionMaxWait {
				return false, BlockedLSPPending
			}
		}
	}

	// 6. Rank scheduler mid-repair.
	if g.deps.Scheduler != nil {
		if !g.deps.Scheduler.IsQuiescent(ws) {
			return false, BlockedRankRepairing
		}
	}

	return true, BlockedNone
}
