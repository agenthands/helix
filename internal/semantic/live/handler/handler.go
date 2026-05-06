// Package handler implements the dispatcher that consumes coalesced
// SourceChangeEvents and applies them to the semantic overlay store via
// OverlayTx.
//
// The Handler is intentionally interface-driven on both sides:
//
//   - OverlayWriter abstracts internal/semantic/store.Store.BeginOverlayTx,
//     so unit tests can drive the dispatcher without standing up a DuckDB
//     instance.
//   - IncrementalScheduler abstracts scheduler.ExtractionScheduler so a
//     ChangeBulkUpdate event can defer to the scheduler without circular
//     imports.
//
// Per 60-CONTEXT.md D-02 invariant: per-event errors are logged but do not
// abort the batch. The caller (coalescer.flush) iterates merged events and
// invokes Dispatch sequentially; an error from one event MUST NOT prevent
// the next event in the batch from being attempted.
package handler

import (
	"context"
	"fmt"

	"github.com/agenthands/helix/internal/semantic"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/scheduler"
)

// OverlayWriter is the subset of *store.Store the handler needs.
//
// The interface returns OverlayTx (declared in this package) rather than
// *store.OverlayTx so handler tests can inject a mock without taking on
// the DuckDB dependency at the test boundary.  An adapter in the daemon
// (60-05B) wraps the real store with a thin shim implementing this
// interface.
type OverlayWriter interface {
	BeginOverlayTx(ctx context.Context, repoID string) (OverlayTx, error)
}

// OverlayTx is the subset of store.OverlayTx the handler invokes today.
// 60-02 ships the Mark*Deleted surface; 61-01 extends with
// MarkFileSemanticPending for the producer-side bulk-update suppression
// path (D-05).
type OverlayTx interface {
	Epoch() uint64
	UpsertOverlayFile(ctx context.Context, path, contentHash string) error
	MarkFileDeleted(ctx context.Context, path string) error
	// MarkFileSemanticPending stamps the existing partial_reason column on
	// semantic_files for (repoID, path) with reason. Reason MUST be one of
	// the closed-enum values validated by the underlying store. Phase 61
	// D-05; consumed by handler.markBulkPending under ChangeBulkUpdate.
	MarkFileSemanticPending(ctx context.Context, path, reason string) error
	Commit() error
	Rollback() error
}

// IncrementalScheduler is the subset of scheduler.ExtractionScheduler the
// bulk handler invokes when collapsing a high-fanout coalesced batch.
type IncrementalScheduler interface {
	ScheduleIncremental(workspaceID semantic.WorkspaceID, changes []scheduler.FileChange) scheduler.JobID
}

// Hasher computes the content hash for an absolute path.  Production
// wiring uses xxhash64 to match Phase 59 stable-ID hash (D-05 invariant).
type Hasher func(absPath string) (string, error)

// Logger is the minimal slog-shaped surface the handler depends on.
type Logger interface {
	Warn(msg string, args ...any)
	Info(msg string, args ...any)
}

// LSPLaneEnqueuer is the producer side of the Phase 61 2-lane queue. It
// supersedes the Phase 60 single-method enqueue interface that this
// package previously declared (B5 resolution: rename + extend in-place,
// single source of truth, no parallel interfaces in the tree).
// Implemented by *lspenrich.LaneQueue. The legacy Enqueue alias is
// preserved as a method on the interface so any non-handler caller still
// holding the old shape continues to compile (the alias maps every job
// to LaneHigh; new callers MUST use EnqueueLane).
//
// Nil is a no-op (test paths and unwired daemons skip the enqueue).
type LSPLaneEnqueuer interface {
	EnqueueLane(lane lspenrich.Lane, job lspqueue.RevalidateFileJob) bool
	Enqueue(job lspqueue.RevalidateFileJob) bool // legacy; defaults to LaneHigh
}

// RankApplier is the Phase 62 P02 post-commit hook surface. The handler
// fires this after a successful overlay tx commit when the diff carries
// graph-changing edits. Production = *graph.Engine; tests can stub it.
//
// Nil is a no-op (Phase 60 nil-safe pattern, CR-04 invariant): when the
// rank applier is not wired, the handler's update path runs unchanged.
//
// W2 LOCKED: the handler accumulates the FileFactDiff LOCALLY during the
// tx span; it is NOT a method on OverlayTx. The handler is the single
// owner of the diff state, so the OverlayTx surface stays unchanged.
type RankApplier interface {
	ApplyRepair(ctx context.Context, repoID string, repair graphpkg.GraphRepair) (uint64, bool, error)
}

// Handler is the dispatcher.  Construct via New (or zero-value with
// fields set directly in tests).
type Handler struct {
	Store    OverlayWriter
	Hasher   Hasher
	Sched    IncrementalScheduler
	Logger   Logger
	LSPQueue LSPLaneEnqueuer // nil-safe (Phase 60 producer side; Phase 61 lane-aware)
	// Metrics is the lspenrich-side metrics surface for bulk-suppressed
	// counter emissions (Phase 61 D-05). Nil-safe: nil metrics short-
	// circuits the bump. Production wiring (P03) supplies a
	// ProdMetricsSink wrapping *obs.Metrics.
	Metrics lspenrich.MetricsSink
	// rankApplier is the Phase 62 P02 post-commit hook target. Nil-safe;
	// wired via SetRankApplier from the daemon bootstrap when the rank
	// engine is constructed (CR-04 nil-safety invariant).
	rankApplier RankApplier
}

// SetRankApplier installs (or replaces) the Phase 62 RankApplier hook.
// Safe to call before or after the handler is in service. Passing nil is
// the documented "unwire" path — subsequent dispatches do not fire
// ApplyRepair.
func (h *Handler) SetRankApplier(r RankApplier) {
	if h == nil {
		return
	}
	h.rankApplier = r
}

// New constructs a Handler with the given dependencies.  Logger is
// required; the others may be nil if the tests pin specific dispatch
// branches that don't reach them.
func New(store OverlayWriter, hasher Hasher, sched IncrementalScheduler, logger Logger) *Handler {
	if logger == nil {
		logger = noopLogger{}
	}
	return &Handler{Store: store, Hasher: hasher, Sched: sched, Logger: logger}
}

// Dispatch dispatches a single coalesced event to the appropriate
// handler.  Per-event errors propagate to the caller (coalescer.flush)
// which logs them and continues with the next event in the batch (D-02
// invariant: per-event errors do not abort the batch).
//
// Empty batches MUST be filtered by the caller BEFORE Dispatch is called;
// see coalescer.Coalescer.flush which short-circuits len(coalesced)==0
// before invoking the handler.  This pre-Dispatch filter is the
// load-bearing guarantee that no-op flushes do NOT advance overlay_epoch
// (60-04 acceptance #8).
func (h *Handler) Dispatch(ctx context.Context, ev live.SourceChangeEvent) error {
	switch ev.Kind {
	case live.ChangeFileCreated, live.ChangeFileModified, live.ChangeHelixEdit:
		return h.updateChangedFileWithKind(ctx, ev.RepoID, ev.Path, ev.Kind)
	case live.ChangeFileDeleted:
		return h.HandleFileDeleted(ctx, ev.RepoID, ev.Path)
	case live.ChangeFileRenamed:
		return h.handleFileRenamedWithKind(ctx, ev.RepoID, ev.OldPath, ev.Path, ev.Kind)
	case live.ChangeBulkUpdate:
		// Phase 61 D-05: producer-side suppression. Do NOT enqueue any
		// per-file LSP revalidation — at the bulk threshold the queue
		// would be flooded (PITFALLS C8 prescription). Instead, mark
		// every affected file partial_reason="bulk_update_pending" so
		// it is observable, and bump the bulk-suppressed counter for
		// ops dashboards.
		if err := h.markBulkPending(ctx, ev.RepoID, ev.Paths); err != nil {
			h.Logger.Warn("handler: markBulkPending failed",
				"repo", ev.RepoID, "n", len(ev.Paths), "err", err)
			// Fall through — we still want HandleBulkUpdate to run
			// (scheduler rebuild is the structural-recovery path).
		}
		if h.Metrics != nil {
			h.Metrics.LSPEnrichmentBulkSuppressed(len(ev.Paths))
		}
		return h.HandleBulkUpdate(ctx, ev.RepoID)
	default:
		return fmt.Errorf("handler.Dispatch: unknown kind %q", ev.Kind)
	}
}

// UpdateChangedFile hashes path, opens an OverlayTx, upserts the file row
// stamped with the tx's overlay_epoch, commits, and enqueues a Phase 61
// LSP revalidation job (best-effort — drops are tolerated since the
// watcher / scanner provide the correctness story; LSP re-enrichment is
// a freshness optimization).
//
// This is the scheduler.IncrementalHandler entrypoint (3-arg signature).
// Callers reaching here from the scheduler's incremental path do NOT carry
// SourceChangeEvent.Kind context, so the lane defaults to LaneBackground —
// scheduler-driven revalidation is recovery work, not foreground edit
// traffic. Phase 61 D-01.
//
// Producer-side callers (the Dispatch switch) use the internal
// updateChangedFileWithKind variant which threads ev.Kind through to
// selectLane.
func (h *Handler) UpdateChangedFile(ctx context.Context, repoID semantic.RepoID, path string) error {
	return h.updateChangedFileWithKind(ctx, repoID, path, live.ChangeFileModified)
}

// updateChangedFileWithKind is the lane-aware variant of UpdateChangedFile.
// kind is consulted by selectLane to choose between LaneHigh (helix_edit)
// and LaneBackground (everything else). Phase 61 D-01.
//
// W2 LOCKED — handler-tracked diff (no OverlayTx.Diff method): the handler
// accumulates a graphpkg.FileFactDiff LOCALLY during the tx span. Phase 60
// P02's UpsertOverlayFile is the only fact write today (no symbol/edge
// mutation), so the diff is empty for now and the Phase 62 hook short-
// circuits via repair.IsEmpty(). Phase 60 P04 / Phase 62 P05 will populate
// the diff as the symbol-level upsert paths land.
func (h *Handler) updateChangedFileWithKind(ctx context.Context, repoID semantic.RepoID, path string, kind live.SourceChangeKind) error {
	hash, err := h.Hasher(path)
	if err != nil {
		return fmt.Errorf("UpdateChangedFile: hash %s: %w", path, err)
	}
	tx, err := h.Store.BeginOverlayTx(ctx, string(repoID))
	if err != nil {
		return fmt.Errorf("UpdateChangedFile: begin tx: %w", err)
	}
	if err := tx.UpsertOverlayFile(ctx, path, hash); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("UpdateChangedFile: upsert: %w", err)
	}
	// Phase 60 P02 surface writes only the file-row contentHash. No
	// symbol-level diff information flows here yet, so the local diff is
	// empty and the Phase 62 hook will short-circuit via repair.IsEmpty().
	// Phase 60 P04 (full FileFact upsert) and Phase 62 P05 (type resolver
	// edge emission) will fill this struct from their own tx-scoped
	// recorders when they land.
	var diff graphpkg.FileFactDiff
	if err := tx.Commit(); err != nil {
		return err
	}
	// === Phase 62 P02 post-commit hook ===
	// Fires AFTER tx.Commit() succeeds and BEFORE EnqueueLane. Body-only
	// edits are short-circuited via repair.IsEmpty() inside ApplyRepair,
	// so a nil-rich (no-flag) diff costs only a CurrentGraphVersion read.
	if h.rankApplier != nil {
		repair := graphpkg.ComputeGraphRepair(diff)
		if !repair.IsEmpty() {
			if _, _, err := h.rankApplier.ApplyRepair(ctx, string(repoID), repair); err != nil {
				// Non-fatal: a missed advance is degraded but safe — the
				// next tx that DOES advance graph_version will surface
				// fresh score rows. Log and continue to the LSP enqueue.
				h.Logger.Warn("apply_repair failed",
					"err", err, "repo_id", repoID, "path", path)
			}
		}
	}
	if h.LSPQueue != nil {
		_ = h.LSPQueue.EnqueueLane(selectLane(kind), lspqueue.RevalidateFileJob{
			RepoID: repoID,
			Path:   path,
		})
	}
	return nil
}

// selectLane maps a SourceChangeEvent.Kind to the queue lane per Phase 61
// D-01: ChangeHelixEdit → high; all watcher / manifest-scan kinds (Created,
// Modified, Deleted, Renamed) → background. ChangeBulkUpdate is handled
// separately (markBulkPending) and never reaches selectLane — the producer
// bypasses the queue entirely on bulk events.
//
// The lane is owned by the producer (this handler), not derived inside the
// worker (D-01 invariant).
func selectLane(kind live.SourceChangeKind) lspenrich.Lane {
	switch kind {
	case live.ChangeHelixEdit:
		return lspenrich.LaneHigh
	default:
		return lspenrich.LaneBackground
	}
}

// markBulkPending opens an OverlayTx and stamps every supplied path with
// partial_reason="bulk_update_pending" via tx.MarkFileSemanticPending. The
// path loop is best-effort — per-path errors are logged via the handler
// logger but DO NOT abort the loop and DO NOT propagate to the caller
// (PITFALLS C8: bulk-update is the death-spiral-prevention path; failing
// it loudly defeats the suppression).
//
// Empty paths is a no-op (returns nil without opening a tx).
//
// Phase 61 D-05.
func (h *Handler) markBulkPending(ctx context.Context, repoID semantic.RepoID, paths []string) error {
	if len(paths) == 0 {
		return nil
	}
	if h.Store == nil {
		return nil
	}
	tx, err := h.Store.BeginOverlayTx(ctx, string(repoID))
	if err != nil {
		return fmt.Errorf("markBulkPending: begin tx: %w", err)
	}
	for _, p := range paths {
		if err := tx.MarkFileSemanticPending(ctx, p, "bulk_update_pending"); err != nil {
			h.Logger.Warn("markBulkPending: per-file mark failed",
				"repo", repoID, "path", p, "err", err)
			// Continue — best-effort.
		}
	}
	return tx.Commit()
}

// HandleFileDeleted writes a tombstone row via OverlayTx.MarkFileDeleted
// (60-02 single-owner API).  The row's status column flips to 'deleted'
// and write_epoch advances; the classifier reads this on the next
// classify call to distinguish "deleted-then-recreated" (resurrection)
// from "still missing".
func (h *Handler) HandleFileDeleted(ctx context.Context, repoID semantic.RepoID, path string) error {
	tx, err := h.Store.BeginOverlayTx(ctx, string(repoID))
	if err != nil {
		return fmt.Errorf("HandleFileDeleted: begin tx: %w", err)
	}
	if err := tx.MarkFileDeleted(ctx, path); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("HandleFileDeleted: tombstone: %w", err)
	}
	return tx.Commit()
}

// HandleFileRenamed ships the SPEC §16.5 fallback form: tombstone the old
// path, upsert the new path.  The content-hash lineage check (rename-vs-
// modify-and-create disambiguation) is reserved for a future revision.
//
// Public 4-arg signature preserved for any external callers; the Dispatch
// switch uses handleFileRenamedWithKind which threads ev.Kind through to
// selectLane.
func (h *Handler) HandleFileRenamed(ctx context.Context, repoID semantic.RepoID, oldPath, newPath string) error {
	return h.handleFileRenamedWithKind(ctx, repoID, oldPath, newPath, live.ChangeFileRenamed)
}

// handleFileRenamedWithKind is the lane-aware variant.
func (h *Handler) handleFileRenamedWithKind(ctx context.Context, repoID semantic.RepoID, oldPath, newPath string, kind live.SourceChangeKind) error {
	if err := h.HandleFileDeleted(ctx, repoID, oldPath); err != nil {
		return err
	}
	return h.updateChangedFileWithKind(ctx, repoID, newPath, kind)
}

// HandleBulkUpdate defers to the scheduler for an incremental rewalk.
// We pass an empty changes slice; the scheduler treats this as a "rewalk
// this workspace" hint and consults a fresh manifest scan internally.
//
// HandleBulkUpdate intentionally does NOT open an OverlayTx — that would
// burn an overlay_epoch even though no rows are written here (60-04
// acceptance #8: empty coalesced batches do NOT advance overlay_epoch;
// bulk_update is the same logical class — no per-row write).
func (h *Handler) HandleBulkUpdate(ctx context.Context, repoID semantic.RepoID) error {
	if h.Sched == nil {
		// Tests / dev builds may run without a scheduler; log and return.
		h.Logger.Warn("HandleBulkUpdate: no scheduler wired", "repo_id", repoID)
		return nil
	}
	job := h.Sched.ScheduleIncremental(semantic.WorkspaceID(repoID), nil)
	h.Logger.Info("HandleBulkUpdate scheduled incremental",
		"repo_id", repoID, "job_id", job)
	_ = ctx
	return nil
}

// noopLogger is the default when New is called with nil Logger.
type noopLogger struct{}

func (noopLogger) Warn(string, ...any) {}
func (noopLogger) Info(string, ...any) {}

// Compile-time assertion: *Handler satisfies
// scheduler.IncrementalHandler.  This is the seam scheduler.Scheduler
// reaches via SetIncrementalHandler — if the method set drifts
// (e.g., a parameter rename), the build breaks here rather than at
// the daemon-wiring callsite in 60-05B.
var _ scheduler.IncrementalHandler = (*Handler)(nil)
