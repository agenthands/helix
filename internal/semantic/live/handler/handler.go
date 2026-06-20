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
	"sync"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
	graphpkg "github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
	"github.com/agenthands/helix/internal/semantic/lspenrich"
	"github.com/agenthands/helix/internal/semantic/scheduler"
	semstore "github.com/agenthands/helix/internal/semantic/store"
)

// FileFactStore is the narrow store-side surface the Phase 68 Tier-1
// populator consumes. Satisfied by *semstore.Store; daemon wires post-init.
type FileFactStore interface {
	GetLatestFileFact(ctx context.Context, repoID, path string) (semstore.PriorFileFact, bool, error)
}

// ExtractRegistry is the narrow extract-registry surface the Phase 68
// Tier-1/Tier-2 populators consume. Satisfied by *extract.Registry.
type ExtractRegistry interface {
	Provider(lang string) (extract.Provider, bool)
}

// FileFactDiffMetricsSink is the narrow metrics surface the Phase 68
// populator emits to. Satisfied by *obs.Metrics. Kept narrow so the
// handler package does not pull in the full *obs.Metrics surface (D-02
// kernel↔semantic boundary intact).
type FileFactDiffMetricsSink interface {
	LiveFileFactDiffInc(tier, repo string)
	LiveFileFactDiffSyntheticReasonInc(reason string)
}

// FileFactDiffRecorder is the tx-scoped recorder that future populators
// (Phase 60 P04 full FileFact upsert; future type-resolver live-edge
// retrofit) write through as they mutate the overlay tx. The handler
// constructs one recorder per tx span, hands it to populators, then
// snapshots its state immediately after tx.Commit() to feed the post-
// commit ComputeGraphRepair → ApplyRepair pipeline.
//
// The recorder is intentionally narrow: five SymbolDiff variants (matching
// graphpkg.SymbolDiff fields verbatim) plus AddedEdges/RemovedEdges record
// methods. It does NOT validate (the consumer of the snapshot validates);
// it does NOT deduplicate (graphpkg.ComputeGraphRepair handles dedup via
// nodeSet); it does NOT lock (each instance is owned by exactly one tx
// goroutine).
//
// CONCURRENCY CONTRACT: a FileFactDiffRecorder MUST be owned by a single
// goroutine for the lifetime of one overlay tx. Cross-goroutine sharing is
// a data race on the underlying slices (matching the graphpkg.nodeSet
// contract documented at internal/semantic/graph/repair.go:138-145).
//
// 62-09 closure (truth #22): the recorder seam is the single point where
// Phase 60 P04 (full FileFact upsert) and any future Phase 62 type-resolver
// live-edge retrofit MUST populate SymbolDiff entries / edge add/remove
// sets. Until those populators land, the recorder stays empty in
// production and the post-commit hook short-circuits via IsEmpty(); a
// once-INFO log per workspace per process surfaces the gap to operators.
type FileFactDiffRecorder struct {
	removedSymbols []graphpkg.SymbolDiff
	changedSymbols []graphpkg.SymbolDiff
	addedSymbols   []graphpkg.SymbolDiff
	addedEdges     []graphpkg.GraphEdge
	removedEdges   []graphpkg.GraphEdge
}

// RecordSymbolRemoved appends a removed-symbol diff entry. Method names
// match the graphpkg.SymbolDiff bit-flags so populators read clearly at
// the call site. Nil-receiver-safe (no-op).
func (r *FileFactDiffRecorder) RecordSymbolRemoved(d graphpkg.SymbolDiff) {
	if r == nil {
		return
	}
	r.removedSymbols = append(r.removedSymbols, d)
}

// RecordSymbolChanged appends a changed-symbol diff entry. Nil-receiver-safe.
func (r *FileFactDiffRecorder) RecordSymbolChanged(d graphpkg.SymbolDiff) {
	if r == nil {
		return
	}
	r.changedSymbols = append(r.changedSymbols, d)
}

// RecordSymbolAdded appends an added-symbol diff entry. Nil-receiver-safe.
func (r *FileFactDiffRecorder) RecordSymbolAdded(d graphpkg.SymbolDiff) {
	if r == nil {
		return
	}
	r.addedSymbols = append(r.addedSymbols, d)
}

// RecordEdgeAdded appends an added-edge entry. Nil-receiver-safe.
func (r *FileFactDiffRecorder) RecordEdgeAdded(e graphpkg.GraphEdge) {
	if r == nil {
		return
	}
	r.addedEdges = append(r.addedEdges, e)
}

// RecordEdgeRemoved appends a removed-edge entry. Nil-receiver-safe.
func (r *FileFactDiffRecorder) RecordEdgeRemoved(e graphpkg.GraphEdge) {
	if r == nil {
		return
	}
	r.removedEdges = append(r.removedEdges, e)
}

// Snapshot returns the FileFactDiff value the handler hands to
// graphpkg.ComputeGraphRepair. After Snapshot() returns, the recorder MUST
// NOT be mutated further (single-snapshot per recorder). Nil-receiver-safe
// (returns the zero FileFactDiff).
func (r *FileFactDiffRecorder) Snapshot() graphpkg.FileFactDiff {
	if r == nil {
		return graphpkg.FileFactDiff{}
	}
	return graphpkg.FileFactDiff{
		RemovedSymbols: r.removedSymbols,
		ChangedSymbols: r.changedSymbols,
		AddedSymbols:   r.addedSymbols,
		AddedEdges:     r.addedEdges,
		RemovedEdges:   r.removedEdges,
	}
}

// IsEmpty reports whether any record method was invoked. When true, the
// downstream graphpkg.ComputeGraphRepair would itself produce an empty
// GraphRepair (D-06 body-only / no-flag short-circuit), so the handler
// avoids the snapshot allocation and the ComputeGraphRepair call entirely.
// Nil-receiver-safe (returns true).
func (r *FileFactDiffRecorder) IsEmpty() bool {
	if r == nil {
		return true
	}
	return len(r.removedSymbols) == 0 &&
		len(r.changedSymbols) == 0 &&
		len(r.addedSymbols) == 0 &&
		len(r.addedEdges) == 0 &&
		len(r.removedEdges) == 0
}

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

	// factStore is the pre-edit FileFact accessor consumed by the Phase 68
	// Tier-1 populator. Nil-safe; daemon wires via SetFileFactStore.
	factStore FileFactStore
	// extractRegistry resolves the per-language extract.Provider for
	// Tier-1/Tier-2 ExtractFile calls. Nil-safe; daemon wires via
	// SetExtractRegistry.
	extractRegistry ExtractRegistry
	// FileFactDiffMetrics is the Phase 68 D-07 / D-08 outcome + synthetic-
	// reason metric sink. Public field for test injection; production wires
	// via direct assignment in the daemon bootstrap. Nil-safe.
	FileFactDiffMetrics FileFactDiffMetricsSink

	// lastRecorderSnapshot captures the recorder.Snapshot() value taken
	// before ComputeGraphRepair, for Plan 68-04 E2E test inspection (Q4).
	// Single-goroutine tx ownership invariant from 62-09 guarantees no
	// concurrent population per Handler.
	lastRecorderSnapshot graphpkg.FileFactDiff

	// emptyDiffOnces gates the empty-diff INFO log per workspace per
	// process. 62-09 closure (truth #22): production callers see exactly
	// one log per (workspace, Handler instance) until Phase 60 P04 /
	// future type-resolver retrofit populate the recorder. Key: repoID
	// string. Value: *sync.Once.
	emptyDiffOnces sync.Map

	// populateRecorderForTest is an unexported test-only seam that lets
	// handler_test.go simulate Phase 60 P04 / future type-resolver
	// retrofit populators without pre-implementing those phases. It is
	// invoked AFTER UpsertOverlayFile and BEFORE Commit. It MUST remain
	// nil in production; the only writer is SetPopulateRecorderForTest
	// in export_test.go.
	populateRecorderForTest func(*FileFactDiffRecorder)
}

// emptyDiffOnce fires fn exactly once per repoID per Handler instance.
// 62-09 closure: surfaces the empty-FileFactDiff path so operators see
// ApplyRepair short-circuits in production until populators land.
func (h *Handler) emptyDiffOnce(repoID string, fn func()) {
	v, _ := h.emptyDiffOnces.LoadOrStore(repoID, &sync.Once{})
	v.(*sync.Once).Do(fn)
}

// SetFileFactStore installs (or replaces) the Phase 68 FileFactStore
// surface consumed by the Tier-1 populator. Nil-safe (Phase 60 pattern).
// Production wires *semstore.Store via the daemon post-init step.
func (h *Handler) SetFileFactStore(s FileFactStore) {
	if h == nil {
		return
	}
	h.factStore = s
}

// HasFactStore reports whether a FileFactStore is currently wired. Used by the
// Phase 81 Plan 07 (CR-01) daemon gate test to assert the background read-driver
// is INERT under effSemanticDisabled (the FileFactStore is the thing that drives
// GetLatestFileFact / LatestCommittedSnapshot reads against the DuckDB store).
// Nil-safe.
func (h *Handler) HasFactStore() bool {
	if h == nil {
		return false
	}
	return h.factStore != nil
}

// SetExtractRegistry installs (or replaces) the Phase 68 ExtractRegistry
// surface consumed by the Tier-1/Tier-2 populators. Nil-safe.
func (h *Handler) SetExtractRegistry(r ExtractRegistry) {
	if h == nil {
		return
	}
	h.extractRegistry = r
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
// accumulates a graphpkg.FileFactDiff LOCALLY via FileFactDiffRecorder
// during the tx span. Phase 60 P02's UpsertOverlayFile is the only fact
// write today (no symbol/edge mutation), so the recorder stays empty in
// production and the Phase 62 hook short-circuits via recorder.IsEmpty();
// the empty path emits a once-INFO log per workspace per Handler instance
// to surface the gap. Phase 60 P04 (full FileFact upsert) and any future
// Phase 62 type-resolver retrofit MUST populate the recorder.
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

	// === 62-09 closure: tx-scoped FileFactDiffRecorder ===
	// The recorder is the single populator surface future phases write
	// through, BEFORE tx.Commit(). It is owned by this goroutine for the
	// lifetime of this tx span (concurrency contract above on
	// FileFactDiffRecorder).
	//
	// TODO(phase-60-p04): UpsertOverlayFile (and the upcoming full
	// FileFact upsert variant) MUST record SymbolDiff entries via
	// recorder.RecordSymbolChanged / RecordSymbolRemoved /
	// RecordSymbolAdded.
	//
	// TODO(phase-62-future): a live type-resolver invocation feeding the
	// recorder via recorder.RecordEdgeAdded / RecordEdgeRemoved would
	// close the type-resolver half of truth #22.
	recorder := &FileFactDiffRecorder{}
	if h.populateRecorderForTest != nil {
		// Test-only seam (export_test.go SetPopulateRecorderForTest);
		// production code paths leave the field nil.
		h.populateRecorderForTest(recorder)
	} else {
		// F-01 production populator (best-effort, never errors). Tier 3
		// synthetic-marker fallback guarantees the recorder is non-empty
		// so ApplyRepair fires and graph_version advances on every live
		// edit. See difffacts.go for the tiering rationale and
		// DEF-67-F01-FULL-DIFF in .planning/deferred-items.md for the
		// full-precision follow-up.
		h.populateRecorderForFile(ctx, repoID, path, recorder)
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Capture the recorder snapshot for the Plan 68-04 E2E test seam
	// (LastRecorderSnapshotForTest). Single-goroutine tx ownership keeps
	// this safe without a mutex (62-09).
	h.lastRecorderSnapshot = recorder.Snapshot()

	// === Phase 62 P02 post-commit hook ===
	// Fires AFTER tx.Commit() succeeds and BEFORE EnqueueLane. Empty
	// recorder (today's production state) emits a once-INFO log per
	// workspace and skips ApplyRepair entirely. Populated recorder
	// (future Phase 60 P04 / type-resolver retrofit) snapshots, computes
	// the GraphRepair, and fires ApplyRepair if non-empty.
	if h.rankApplier != nil {
		if recorder.IsEmpty() {
			// 62-09 closure: surface the empty-diff path so operators
			// see that ApplyRepair short-circuits in production until
			// populators land. Per-workspace once-gated to avoid log
			// spam.
			h.emptyDiffOnce(string(repoID), func() {
				h.Logger.Info(
					"live FileFactDiff is empty; ApplyRepair short-circuited (Phase 60 P04 full FileFact upsert + future type-resolver retrofit will populate)",
					"repo_id", repoID,
					"phase_dependency", "60-P04",
					"see", "62-VERIFICATION.md truth #22; closure 62-09-PLAN.md",
				)
			})
		} else {
			diff := h.lastRecorderSnapshot
			repair := graphpkg.ComputeGraphRepair(diff)
			if !repair.IsEmpty() {
				if _, _, err := h.rankApplier.ApplyRepair(ctx, string(repoID), repair); err != nil {
					// Non-fatal: a missed advance is degraded but safe —
					// the next tx that DOES advance graph_version will
					// surface fresh score rows. Log and continue to the
					// LSP enqueue.
					h.Logger.Warn("apply_repair failed",
						"err", err, "repo_id", repoID, "path", path)
				}
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
