package retrieval

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// Bleve dual-store reconciliation: at workspace activation, compare the
// bleve segment's last-indexed snapshot id against the *Store's latest
// committed snapshot id. On mismatch (incl. missing bleve), spawn a
// background rebuild goroutine that walks Store.IterateCommittedSymbols
// and bulk-indexes via Engine.UpsertBatch. While the rebuild runs,
// get_semantic_context returns RetrievalPending=true / Freshness=stale.

// metaKeyLastIndexed is the bleve internal-storage key used to persist the
// snapshot id whose facts the bleve segment reflects. Read via
// Engine.GetMeta; written via Engine.SetMeta on rebuild commit.
const metaKeyLastIndexed = "last_indexed_snapshot_id"

// rebuildBatchSize bounds the per-batch UpsertBatch size so a 100k-symbol
// rebuild doesn't construct a 100k-document bleve.Batch in one shot.
const rebuildBatchSize = 1000

// StoreReader is a consumer-defined interface seam (Go idiom). Production
// wiring (P64-08) passes a *internal/semantic/store.Store value, which
// implements both methods natively (the real implementations live in
// 64-02's effective_graph.go — recovery NEVER edits that file).
//
// Keeping the interface seam local to recovery.go decouples the bleve
// recovery package from the *Store type and makes recovery_test.go's
// fake-store easy to author without dragging the whole DuckDB layer in.
type StoreReader interface {
	// LatestCommittedSnapshot returns the highest committed snapshot_id for
	// repoID, or 0 if no committed snapshot exists.
	LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error)
	// IterateCommittedSymbols walks every semantic_symbols row at snapshotID
	// invoking fn(row). Returning false from fn aborts iteration cleanly.
	IterateCommittedSymbols(ctx context.Context, snapshotID uint64, fn func(store.SymbolRow) bool) error
	// CurrentGraphVersion returns the live overlay graph_version for repoID,
	// or 0 if the overlay has never been initialized. Used by Plan 69-02 to
	// stamp the bleve corpus_version meta after a successful rebuild. *Store
	// already implements this natively (overlay.go).
	CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
}

// bleveMetaWriter is a narrow seam over the subset of *Engine that the
// Recoverer needs at the rebuild commit point. The production wiring passes
// the *Engine directly (which satisfies this interface); tests inject a
// failing-SetMeta shim to assert the non-fatal Warn policy (Plan 69-02). The
// upsert/get path is included so the same value drives both the rebuild
// walk-loop write and the post-flush meta-read assertions in tests.
type bleveMetaWriter interface {
	UpsertBatch(ctx context.Context, docs []SymbolDoc) error
	GetMeta(key string) ([]byte, error)
	SetMeta(key string, val []byte) error
}

// RecoveryStatus tracks an in-flight rebuild for a single workspace. Used
// for RetrievalPending(ws) reporting and (future) progress observability.
type RecoveryStatus struct {
	InProgress bool
	StartedAt  time.Time
	Total      int64
	Indexed    atomic.Int64
}

// Recoverer reconciles a single bleve Engine against a StoreReader (the
// committed snapshot history). One Recoverer is constructed per daemon;
// Probe is invoked per workspace activation.
//
// The engine field holds the concrete *Engine so the rest of the daemon
// retains a typed handle; the writer field (which usually points at the same
// *Engine) is the narrow bleveMetaWriter seam used inside Probe/rebuildBlocking
// so Phase 69-02 tests can inject a failing-SetMeta shim. In production both
// fields reference the same *Engine value.
type Recoverer struct {
	engine *Engine
	writer bleveMetaWriter
	store  StoreReader
	logger *slog.Logger

	mu       sync.Mutex
	statuses map[string]*RecoveryStatus // key=repoID
}

// NewRecoverer constructs a Recoverer wrapping engine + store. logger may be
// nil; the Recoverer falls back to slog.Default() in that case.
func NewRecoverer(engine *Engine, reader StoreReader, logger *slog.Logger) *Recoverer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Recoverer{
		engine:   engine,
		writer:   engine, // production: writer is the *Engine itself
		store:    reader,
		logger:   logger,
		statuses: make(map[string]*RecoveryStatus),
	}
}

// newRecovererWithMetaWriter is a test-only constructor that splits the
// engine handle from the bleveMetaWriter seam: the engine value is retained
// for nil-safety / lifecycle assertions while writer drives Probe + rebuild
// meta calls. Used by Phase 69-02 tests to inject SetMeta failure shims.
func newRecovererWithMetaWriter(engine *Engine, writer bleveMetaWriter, reader StoreReader, logger *slog.Logger) *Recoverer {
	if logger == nil {
		logger = slog.Default()
	}
	return &Recoverer{
		engine:   engine,
		writer:   writer,
		store:    reader,
		logger:   logger,
		statuses: make(map[string]*RecoveryStatus),
	}
}

// Probe compares the bleve segment's last-indexed snapshot id against the
// store's latest committed snapshot id; on mismatch (incl. missing bleve
// state), spawns a background rebuild goroutine and returns nil.
//
// Probe is non-blocking: while a rebuild is in flight, RetrievalPending(ws)
// returns true. Callers (get_semantic_context handler) surface that state
// to agents via RetrievalPending=true / Freshness=stale on the response.
//
// Recovery decision tree:
//  1. latest := store.LatestCommittedSnapshot(repoID).
//  2. latest == 0 → no committed snapshot exists; nothing to rebuild.
//  3. raw := engine.GetMeta(metaKeyLastIndexed); parse uint64.
//  4. raw == latest → bleve is current; no-op.
//  5. raw > latest → post-rollback case; log WARN + treat as missing.
//  6. raw < latest (or unset/parse-fail) → spawn background rebuild.
func (r *Recoverer) Probe(ctx context.Context, ws workspace.WorkspaceKey) error {
	if r == nil || r.engine == nil || r.writer == nil || r.store == nil {
		return fmt.Errorf("retrieval.Probe: recoverer not fully constructed")
	}
	repoID := ws.Hash()

	latest, err := r.store.LatestCommittedSnapshot(ctx, repoID)
	if err != nil {
		r.logger.Warn("retrieval.Probe: LatestCommittedSnapshot failed",
			"repo_id", repoID, "err", err)
		return nil // tolerate; recovery can run on the next activation
	}
	if latest == 0 {
		// No committed snapshot exists — nothing to rebuild.
		return nil
	}

	rawBytes, err := r.writer.GetMeta(metaKeyLastIndexed)
	if err != nil {
		r.logger.Warn("retrieval.Probe: GetMeta failed; treating as missing",
			"repo_id", repoID, "err", err)
		// Continue to rebuild path.
	}
	var raw uint64
	if len(rawBytes) > 0 {
		parsed, perr := strconv.ParseUint(string(rawBytes), 10, 64)
		if perr != nil {
			r.logger.Warn("retrieval.Probe: last_indexed_snapshot_id parse failed; treating as missing",
				"repo_id", repoID, "raw", string(rawBytes), "err", perr)
			raw = 0
		} else {
			raw = parsed
		}
	}

	switch {
	case raw == latest:
		// Bleve is current — no-op.
		return nil
	case raw > latest:
		// Post-rollback case: bleve thinks it's ahead of the store. Log a
		// warning and rebuild from scratch (treat as missing).
		r.logger.Warn("retrieval.Probe: bleve last_indexed > store latest (post-rollback?); rebuilding",
			"repo_id", repoID, "bleve_last", raw, "store_latest", latest)
	default:
		// raw < latest (or 0/unset) — normal stale/missing rebuild.
	}

	r.spawnRebuild(ws, latest)
	return nil
}

// RetrievalPending reports whether a rebuild is currently in flight for ws.
// Returns false if no rebuild has been started or the most-recent rebuild
// has completed.
func (r *Recoverer) RetrievalPending(ws workspace.WorkspaceKey) bool {
	if r == nil {
		return false
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	st, ok := r.statuses[ws.Hash()]
	if !ok {
		return false
	}
	return st.InProgress
}

// spawnRebuild registers a RecoveryStatus and launches the background
// rebuild goroutine. Caller is Probe; this method is internal-only.
//
// The goroutine consumes ctx.Background()-derived context so a request-
// scoped cancellation cannot abort an in-flight rebuild (mirrors P64-04
// IndexRunner D-04 invariant).
func (r *Recoverer) spawnRebuild(ws workspace.WorkspaceKey, snapshotID uint64) {
	repoID := ws.Hash()

	r.mu.Lock()
	if existing, ok := r.statuses[repoID]; ok && existing.InProgress {
		// Rebuild already running for this workspace — coalesce.
		r.mu.Unlock()
		return
	}
	st := &RecoveryStatus{
		InProgress: true,
		StartedAt:  time.Now(),
	}
	r.statuses[repoID] = st
	r.mu.Unlock()

	go func() {
		bgCtx := context.Background()
		err := r.rebuildBlocking(bgCtx, ws, snapshotID, st)
		r.mu.Lock()
		st.InProgress = false
		r.mu.Unlock()
		if err != nil {
			r.logger.Error("retrieval.rebuild: failed",
				"repo_id", repoID, "snapshot", snapshotID, "err", err)
			return
		}
		r.logger.Info("retrieval.rebuild: complete",
			"repo_id", repoID, "snapshot", snapshotID, "indexed", st.Indexed.Load())
	}()
}

// rebuildBlocking walks the committed snapshot, batches symbols into
// SymbolDocs via corpus.MapSymbolToDoc, and upserts in bounded batches to
// the bleve engine. On successful completion, persists the snapshot id via
// Engine.SetMeta(metaKeyLastIndexed).
//
// Note on file content for comment-window extraction: IterateCommittedSymbols
// returns SymbolRow without file source bytes. We pass nil for fileSource to
// MapSymbolToDoc, accepting that the comment-window field is empty for
// symbols indexed via recovery (rather than at the original commit time).
// Bleve queries against the other text fields (name, path, docstring) still
// work. If a future phase wants comment-window on recovery, 64-02 can extend
// SymbolRow to include file bytes.
func (r *Recoverer) rebuildBlocking(ctx context.Context, ws workspace.WorkspaceKey, snapshotID uint64, st *RecoveryStatus) error {
	batch := make([]SymbolDoc, 0, rebuildBatchSize)
	// Phase 69-02: accumulate distinct FileIDs across the walk so we can
	// stamp MetaKeyIndexedFiles at the commit point. SymbolRow.FileID is
	// string (decimal-encoded uint64 today; we treat it as opaque).
	distinctFiles := make(map[string]struct{})

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		if err := r.writer.UpsertBatch(ctx, batch); err != nil {
			return fmt.Errorf("UpsertBatch (size=%d): %w", len(batch), err)
		}
		st.Indexed.Add(int64(len(batch)))
		batch = batch[:0]
		return nil
	}

	var iterErr error
	walkErr := r.store.IterateCommittedSymbols(ctx, snapshotID, func(row store.SymbolRow) bool {
		// Recovery does not load file source for the comment window — see
		// docstring above for rationale.
		batch = append(batch, MapSymbolToDoc(row, nil))
		if row.FileID != "" {
			distinctFiles[row.FileID] = struct{}{}
		}
		if len(batch) >= rebuildBatchSize {
			if err := flush(); err != nil {
				iterErr = err
				return false // abort iteration on flush error
			}
		}
		return true
	})
	if iterErr != nil {
		return iterErr
	}
	if walkErr != nil {
		return fmt.Errorf("IterateCommittedSymbols(snap=%d): %w", snapshotID, walkErr)
	}
	if err := flush(); err != nil {
		return err
	}

	repoID := ws.Hash()

	// Phase 69-02 corpus_version meta: best-effort, non-fatal. A failure
	// here must NOT undo the rebuild — the bleve segment is already in a
	// consistent state. Skip the write entirely when gv == 0 (overlay
	// uninitialized) to avoid stamping a misleading zero.
	gv, gvErr := r.store.CurrentGraphVersion(ctx, repoID)
	if gvErr != nil {
		r.logger.Warn("retrieval.rebuild: CurrentGraphVersion failed (non-fatal)",
			"repo_id", repoID, "err", gvErr)
	} else if gv > 0 {
		if err := r.writer.SetMeta(MetaKeyCorpusVersion, []byte(strconv.FormatUint(gv, 10))); err != nil {
			r.logger.Warn("retrieval.rebuild: SetMeta(corpus_version) failed (non-fatal)",
				"repo_id", repoID, "graph_version", gv, "err", err)
		}
	}

	// Phase 69-02 indexed_files meta: best-effort, non-fatal. Encoded as a
	// signed decimal int64 so future deltas (e.g. negative for shrinkage)
	// would remain compatible with the same reader.
	indexedFiles := int64(len(distinctFiles))
	if err := r.writer.SetMeta(MetaKeyIndexedFiles, []byte(strconv.FormatInt(indexedFiles, 10))); err != nil {
		r.logger.Warn("retrieval.rebuild: SetMeta(indexed_files) failed (non-fatal)",
			"repo_id", repoID, "indexed_files", indexedFiles, "err", err)
	}

	// Persist the new last_indexed marker. This is the recovery's commit
	// point: subsequent Probe calls will see raw == latest and short-
	// circuit until the next snapshot commit. FATAL on error (single-writer
	// invariant preserved).
	if err := r.writer.SetMeta(metaKeyLastIndexed, []byte(strconv.FormatUint(snapshotID, 10))); err != nil {
		return fmt.Errorf("SetMeta(last_indexed=%d): %w", snapshotID, err)
	}
	return nil
}
