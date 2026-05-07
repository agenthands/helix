// Package compact: Phase 63 P63-02 Task 2.
//
// compactor.go owns the per-workspace Compactor goroutine: an
// AfterFunc-driven timer that fires CompactAfterIdle after the most
// recent OnFlush; the runCompaction body that wraps Begin/Write/Clear/
// DeleteSnapshotsBeyond/Commit in a single tx (D-01 hard invariant);
// CHECKPOINT outside the tx (COMPACT-03); the VACUUM piggyback (D-05).
//
// Lifecycle: Run blocks on ctx.Done(); the timer is stopped on shutdown.
// OnFlush is the daemon-driven entry point that resets the timer.

package compact

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// MetricsSink is the bounded-label metrics surface the compactor emits
// to. SemanticCompactionObserve is gated by a closed enum
// {success, partial, skipped_blocked, error}; SemanticVacuumObserve by
// {success, skipped, error}; SemanticCompactionBlocked is the
// fine-grained skipped_blocked sub-counter keyed on BlockedReason.
//
// Production wiring is *obs.Metrics; tests pass a recording stub.
type MetricsSink interface {
	SemanticCompactionObserve(outcome string, seconds float64)
	SemanticCompactionBlocked(reason string)
	SemanticVacuumObserve(outcome string, seconds float64)
}

// SnapshotStore is the narrow snapshot-side surface the compactor
// consumes. *store.Store satisfies it directly via the P63-01 API.
type SnapshotStore interface {
	BeginSnapshot(ctx context.Context, meta store.SnapshotMeta) (*store.Snapshot, error)
	WriteSnapshotFacts(ctx context.Context, snap *store.Snapshot, facts store.Facts) error
	CommitSnapshot(ctx context.Context, snap *store.Snapshot, summary store.SnapshotSummary) error
	AbortSnapshot(ctx context.Context, snap *store.Snapshot, reason string) error
	Vacuum(ctx context.Context) error
	Checkpoint(ctx context.Context) error
}

// OverlayOps is the narrow overlay-side surface the compactor consumes
// for the pre-flight size guard + the VACUUM-cadence persistence + the
// captured-epoch read (Phase 63 review CR-03). *store.Store satisfies
// via OverlayRowCount + UpdateLastVacuumAt + CurrentOverlayEpoch.
type OverlayOps interface {
	OverlayRowCount(ctx context.Context, repoID string, capturedEpoch uint64) (int, error)
	UpdateLastVacuumAt(ctx context.Context, repoID string, t interface{}) error
	// CurrentOverlayEpoch returns the current overlay write_epoch for
	// repoID (mirrors store.Store.CurrentGraphVersion). Phase 63 review
	// CR-01: the compactor reads this BEFORE BeginSnapshot so
	// ClearOverlayLE only deletes rows committed before the snapshot tx
	// opened. Returns 0 if no overlay writes have happened yet for repoID.
	CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error)
}

// Deps bundles the dependencies passed to NewCompactor.
type Deps struct {
	Gate       *CompactionGate
	Store      SnapshotStore
	OverlayOps OverlayOps
	Metrics    MetricsSink
	Logger     *slog.Logger
}

// Compactor is the per-workspace compaction worker. One goroutine per
// repoID; spawned by compactBundle.ensureCompactor on activation; joined
// on workspace deactivation / daemon shutdown.
type Compactor struct {
	workspaceID workspace.WorkspaceKey
	repoID      string
	cfg         Config
	deps        Deps
	now         func() time.Time

	mu           sync.Mutex
	timer        *time.Timer
	lastBaseID   uint64
	lastVacuumAt time.Time
}

// NewCompactor constructs a per-workspace compactor with the given
// config + deps. Zero-value config fields are filled from
// DefaultConfig. now == nil → time.Now (test injection seam).
func NewCompactor(ws workspace.WorkspaceKey, repoID string, cfg Config, deps Deps) *Compactor {
	if deps.Logger == nil {
		deps.Logger = slog.Default()
	}
	c := &Compactor{
		workspaceID: ws,
		repoID:      repoID,
		cfg:         mergeDefaults(cfg),
		deps:        deps,
		now:         time.Now,
	}
	return c
}

// SetNow replaces the wall-clock function. Test seam — production
// callers should not invoke this.
func (c *Compactor) SetNow(now func() time.Time) {
	if now == nil {
		return
	}
	c.now = now
}

// Run blocks until ctx is cancelled, then stops the timer cleanly.
// Errors return ctx.Err() so the daemon errgroup propagates the
// cancellation.
func (c *Compactor) Run(ctx context.Context) error {
	if c == nil {
		<-ctx.Done()
		return ctx.Err()
	}
	<-ctx.Done()
	c.mu.Lock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.mu.Unlock()
	return ctx.Err()
}

// OnFlush resets the AfterFunc timer to fire CompactAfterIdle from now.
// Wired from compactBundle.OnCoalescerFlush via the daemon's
// Coalescer.SetOnFlush hook.
func (c *Compactor) OnFlush() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.timer != nil {
		c.timer.Stop()
	}
	c.timer = time.AfterFunc(c.cfg.CompactAfterIdle, c.fire)
}

// fire is the AfterFunc callback. Runs in its own goroutine (AfterFunc
// already spawns one); MUST NOT spawn another to keep the structured
// concurrency contract clear.
func (c *Compactor) fire() {
	c.runCompaction(context.Background())
}

// PublicTriggerForTest synchronously invokes runCompaction. Test seam
// only — production callers should rely on the OnFlush/timer path.
func (c *Compactor) PublicTriggerForTest() {
	c.runCompaction(context.Background())
}

// runCompaction is the single-tx compaction body. Returns the outcome
// label observed on SemanticCompactionDurationVec.
//
// D-01 hard invariant: BeginSnapshot → WriteSnapshotFacts →
// snap.ClearOverlayLE → snap.DeleteSnapshotsBeyond → CommitSnapshot all
// run on the SAME tx. CHECKPOINT runs outside the tx (COMPACT-03).
//
// Phase 63 review WR-01: this order matches the canonical
// fakeCompactor witness in
// internal/semantic/store/snapshot_fake_compactor_test.go (Begin →
// Write → ClearOverlayLE → DeleteSnapshotsBeyond → Commit). Keeping
// production and the snapshot-side reference fake in agreement
// minimizes the window during which the new snapshot exists alongside
// obsolete overlay rows, and prevents future maintainers from "fixing"
// production back to a different order based on a stale doc comment.
func (c *Compactor) runCompaction(ctx context.Context) (outcome string) {
	outcome = "success"
	start := c.now()
	defer func() {
		if c.deps.Metrics != nil {
			c.deps.Metrics.SemanticCompactionObserve(outcome, c.now().Sub(start).Seconds())
		}
	}()

	// Gate re-check (state may have shifted in the AfterFunc delay).
	if c.deps.Gate != nil {
		ready, reason := c.deps.Gate.IsReady(c.workspaceID)
		if !ready {
			outcome = "skipped_blocked"
			if c.deps.Metrics != nil {
				c.deps.Metrics.SemanticCompactionBlocked(string(reason))
			}
			return
		}
	}

	if c.deps.Store == nil || c.deps.OverlayOps == nil {
		outcome = "error"
		c.deps.Logger.Warn("compact.runCompaction: missing deps", "repo_id", c.repoID)
		return
	}

	// Capture the current overlay write_epoch BEFORE opening the snapshot
	// tx (Phase 60 D-04 CAS contract / Phase 63 review CR-01). The read is
	// issued on s.db (not in any tx) so it observes the most-recently-
	// committed bump from BeginOverlayTx's monotone allocator. Rows
	// committed AFTER this read receive write_epoch > capturedEpoch and
	// SURVIVE the subsequent snap.ClearOverlayLE — preventing the
	// silent-data-loss scenario the design exists to close.
	capturedEpoch, err := c.deps.OverlayOps.CurrentOverlayEpoch(ctx, c.repoID)
	if err != nil {
		outcome = "error"
		c.deps.Logger.Warn("compact.CurrentOverlayEpoch", "repo_id", c.repoID, "err", err)
		return
	}

	// Pre-flight size guard.
	rows, err := c.deps.OverlayOps.OverlayRowCount(ctx, c.repoID, capturedEpoch)
	if err != nil {
		outcome = "error"
		c.deps.Logger.Warn("compact.OverlayRowCount", "repo_id", c.repoID, "err", err)
		return
	}
	if rows > c.cfg.MaxOverlayRows {
		outcome = "partial"
		c.deps.Logger.Info("compact: pre-flight size guard tripped",
			"repo_id", c.repoID, "rows", rows, "limit", c.cfg.MaxOverlayRows)
		return
	}

	// Open the snapshot tx.
	snap, err := c.deps.Store.BeginSnapshot(ctx, store.SnapshotMeta{
		RepoID:         c.repoID,
		BaseSnapshotID: c.lastBaseID,
		CapturedEpoch:  capturedEpoch,
	})
	if err != nil {
		outcome = "error"
		c.deps.Logger.Warn("compact.BeginSnapshot", "repo_id", c.repoID, "err", err)
		return
	}

	// WriteSnapshotFacts: empty facts. Phase 63 P63-02 ships the
	// compaction-cycle PLUMBING; the merge-input loader (Loader pulling
	// base + overlay into a Facts payload) is left for a follow-up
	// closure plan because Phase 60 lazy-overlay readers and Phase 64
	// effective-graph queries land that path. The compaction tx still
	// runs Begin → ClearOverlayLE → DeleteSnapshotsBeyond → Commit so
	// the ATOMIC-RETENTION + CAS-OVERLAY-DRAIN invariants hold; the
	// overlay rows simply drain into a thin pending snapshot until
	// follow-up Loader work merges fact data.
	if err := c.deps.Store.WriteSnapshotFacts(ctx, snap, store.Facts{}); err != nil {
		_ = c.deps.Store.AbortSnapshot(ctx, snap, "write_facts: "+err.Error())
		outcome = "error"
		return
	}

	// Phase 63 review WR-01: ClearOverlayLE BEFORE DeleteSnapshotsBeyond
	// (matches the canonical fakeCompactor cycle in
	// snapshot_fake_compactor_test.go). DuckDB serializes statements
	// inside the tx so the final committed state is identical regardless
	// of order, but the fakeCompactor ordering is the documented
	// reference and minimizes the in-tx window where the new snapshot
	// exists alongside obsolete overlay rows.
	if err := snap.ClearOverlayLE(ctx, c.repoID, capturedEpoch); err != nil {
		_ = c.deps.Store.AbortSnapshot(ctx, snap, "clear_overlay: "+err.Error())
		outcome = "error"
		return
	}

	if err := snap.DeleteSnapshotsBeyond(ctx, c.cfg.SnapshotRetention); err != nil {
		_ = c.deps.Store.AbortSnapshot(ctx, snap, "retention: "+err.Error())
		outcome = "error"
		return
	}

	if err := c.deps.Store.CommitSnapshot(ctx, snap, store.SnapshotSummary{}); err != nil {
		outcome = "error"
		c.deps.Logger.Warn("compact.CommitSnapshot", "repo_id", c.repoID, "err", err)
		return
	}

	// CHECKPOINT outside the tx (COMPACT-03).
	if err := c.deps.Store.Checkpoint(ctx); err != nil {
		c.deps.Logger.Warn("compact: checkpoint failed (non-fatal)",
			"repo_id", c.repoID, "err", err)
	}

	c.lastBaseID = snap.ID

	// VACUUM piggyback. Outcome on the parent compaction stays "success"
	// regardless — VACUUM has its own metric outcome label.
	c.maybeVacuum(ctx)

	return
}

// Run is callable from a goroutine. Compile-time guard ensures Compactor
// satisfies the daemon-side interface (the daemon spawns it as
// `g.Go(func() error { return c.Run(gctx) })`).
var _ interface {
	Run(ctx context.Context) error
} = (*Compactor)(nil)

// errClosed is the sentinel for context cancellation paths that don't
// flow through ctx.Err() directly.
var errClosed = errors.New("compactor closed")

var _ = errClosed // reserved for future shutdown-error surfaces
