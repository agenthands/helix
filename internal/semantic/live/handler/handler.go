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
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
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
// 60-02 ships the full Mark*Deleted surface; the handler consumes
// MarkFileDeleted to write a real tombstone row, and surfaces the
// symbol/reference/edge variants when the classifier (Phase 60+ revision)
// resolves file_id / node_id sets at classify time.
type OverlayTx interface {
	Epoch() uint64
	UpsertOverlayFile(ctx context.Context, path, contentHash string) error
	MarkFileDeleted(ctx context.Context, path string) error
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

// LSPRevalidationEnqueuer is the producer side of the lspqueue.Queue
// (Phase 60 P04). The handler enqueues a RevalidateFileJob after every
// successful overlay write so Phase 61's worker has a queue to drain.
// Nil is a no-op (test paths and unwired daemons skip the enqueue).
type LSPRevalidationEnqueuer interface {
	Enqueue(job lspqueue.RevalidateFileJob) bool
}

// Handler is the dispatcher.  Construct via New (or zero-value with
// fields set directly in tests).
type Handler struct {
	Store    OverlayWriter
	Hasher   Hasher
	Sched    IncrementalScheduler
	Logger   Logger
	LSPQueue LSPRevalidationEnqueuer // nil-safe (Phase 60 producer side; Phase 61 consumes)
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
		return h.UpdateChangedFile(ctx, ev.RepoID, ev.Path)
	case live.ChangeFileDeleted:
		return h.HandleFileDeleted(ctx, ev.RepoID, ev.Path)
	case live.ChangeFileRenamed:
		return h.HandleFileRenamed(ctx, ev.RepoID, ev.OldPath, ev.Path)
	case live.ChangeBulkUpdate:
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
func (h *Handler) UpdateChangedFile(ctx context.Context, repoID semantic.RepoID, path string) error {
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
	if err := tx.Commit(); err != nil {
		return err
	}
	if h.LSPQueue != nil {
		_ = h.LSPQueue.Enqueue(lspqueue.RevalidateFileJob{RepoID: repoID, Path: path})
	}
	return nil
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
func (h *Handler) HandleFileRenamed(ctx context.Context, repoID semantic.RepoID, oldPath, newPath string) error {
	if err := h.HandleFileDeleted(ctx, repoID, oldPath); err != nil {
		return err
	}
	return h.UpdateChangedFile(ctx, repoID, newPath)
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
