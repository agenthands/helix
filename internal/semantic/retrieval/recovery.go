package retrieval

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// recovery.go — STUB (RED gate). Real implementation lands in Task 2 GREEN.
//
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
type Recoverer struct {
	engine *Engine
	store  StoreReader
	logger *slog.Logger

	mu       sync.Mutex
	statuses map[string]*RecoveryStatus // key=repoID
}

// NewRecoverer constructs a Recoverer wrapping engine + store. logger may be
// nil; the Recoverer falls back to slog.Default() in that case.
func NewRecoverer(engine *Engine, store StoreReader, logger *slog.Logger) *Recoverer {
	panic("retrieval.NewRecoverer: not implemented (RED gate — Task 2 GREEN fills this in)")
}

// Probe compares the bleve segment's last-indexed snapshot id against the
// store's latest committed snapshot id; on mismatch (incl. missing bleve
// state), spawns a background rebuild goroutine and returns nil.
//
// Probe is non-blocking: while a rebuild is in flight, RetrievalPending(ws)
// returns true. Callers (get_semantic_context handler) surface that state
// to agents via RetrievalPending=true / Freshness=stale on the response.
func (r *Recoverer) Probe(ctx context.Context, ws workspace.WorkspaceKey) error {
	panic("retrieval.Recoverer.Probe: not implemented (RED gate)")
}

// RetrievalPending reports whether a rebuild is currently in flight for ws.
// Returns false if no rebuild has been started or the most-recent rebuild
// has completed.
func (r *Recoverer) RetrievalPending(ws workspace.WorkspaceKey) bool {
	panic("retrieval.Recoverer.RetrievalPending: not implemented (RED gate)")
}
