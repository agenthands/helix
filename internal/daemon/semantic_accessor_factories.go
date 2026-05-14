// Package daemon: Phase 69-05 — exported factory constructors for the
// production semantic SchedulerAccessor / RetrievalAccessor surfaces.
//
// These constructors are the SINGLE source-of-truth for the
// ClusterStatus + RetrievalStatus derivation logic. Both the daemon's
// own adapters (semSchedulerAdapter / semRetrievalAdapter, declared in
// semantic_wiring.go) AND the Plan 69-06 E2E test consume them — so the
// E2E test exercises the real adapter code path rather than a parallel
// reimplementation (revision Warning 3, preferred approach).
//
// Carve-out — retrievalAccessorImpl is STATUS-ONLY. Non-status methods
// (QueryBleve / PersonalizedPageRank / RetrievalPending / TopEdgesFor)
// return zero values and MUST NOT be used for production query paths;
// the daemon's semRetrievalAdapter retains the full production
// implementations for those. This impl exists so daemon AND the Plan
// 69-06 E2E test share a single ClusterStatus / RetrievalStatus code
// path.

package daemon

import (
	"context"
	"strconv"

	"github.com/agenthands/helix/internal/semantic/compact"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/retrieval"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// ----- Scheduler accessor (cluster status) -----

// schedulerAccessorImpl is the production semantic.SchedulerAccessor
// backed by *semanticstore.Store. It only implements ClusterStatus
// non-trivially; IsQuiescent and ScoreStatus return zero-equivalent
// values pending future phases (RESEARCH Q9 — ScoreStatus per-projection
// accessor pending Phase 65/67).
type schedulerAccessorImpl struct {
	store *semanticstore.Store
}

// NewSchedulerAccessorForStore returns a production SchedulerAccessor
// backed by *Store. Used by the daemon's semSchedulerAdapter AND the
// Plan 69-06 E2E test — single implementation, two call sites.
//
// The returned accessor is safe to construct per-call: the underlying
// struct is a single pointer, escape-analysis-friendly. The daemon
// instantiates per-call; Plan 69-06 instantiates once. Both exercise
// the identical underlying logic.
func NewSchedulerAccessorForStore(store *semanticstore.Store) semantic.SchedulerAccessor {
	return &schedulerAccessorImpl{store: store}
}

// IsQuiescent: this factory-produced accessor is the STATUS surface;
// rank-bundle-backed IsQuiescent is owned by semSchedulerAdapter
// directly. Safe default of true (the consumer treats "true" as the
// pre-data state).
func (a *schedulerAccessorImpl) IsQuiescent(_ string) bool { return true }

// ScoreStatus: RESEARCH Q9 — Phase 62 RankScheduler does not expose a
// per-(workspace, projection) accessor today; closed-enum default
// "missing" maps to the pre-data state in downstream consumers.
func (a *schedulerAccessorImpl) ScoreStatus(_, _ string) graph.ScoreStatus {
	return graph.ScoreStatusMissing
}

// ClusterStatus derives the closed-enum cluster state per CONTEXT D1:
//
//   - a == nil || a.store == nil                            → unknown / no-store
//   - CurrentGraphVersion err OR gv == 0                    → unknown / no-graph-version
//   - ClusterStatusForGraphVersion err                      → unknown / accessor-error
//   - row.ClusterCount == 0                                 → unknown / no-cluster-rows
//   - row.ClusterCount > 0 && row.IsCurrent                 → current
//   - row.ClusterCount > 0 && !row.IsCurrent                → stale / graph_version-lag
//
// The "building" state is reserved for a future phase (no signal source
// today; see Plan 69-01 deferred block).
func (a *schedulerAccessorImpl) ClusterStatus(repoID string) semantic.ClusterStatus {
	if a == nil || a.store == nil {
		return semantic.ClusterStatus{State: "unknown", Reason: "no-store"}
	}
	ctx := context.Background()
	gv, err := a.store.CurrentGraphVersion(ctx, repoID)
	if err != nil || gv == 0 {
		return semantic.ClusterStatus{State: "unknown", Reason: "no-graph-version"}
	}
	row, err := a.store.ClusterStatusForGraphVersion(ctx, repoID, gv)
	if err != nil {
		return semantic.ClusterStatus{State: "unknown", Reason: "accessor-error"}
	}
	if row.ClusterCount == 0 {
		return semantic.ClusterStatus{State: "unknown", Reason: "no-cluster-rows"}
	}
	if row.IsCurrent {
		return semantic.ClusterStatus{
			State:       "current",
			ComputedAt:  row.ComputedAt,
			MemberCount: int(row.MemberCount),
		}
	}
	return semantic.ClusterStatus{
		State:       "stale",
		Reason:      "graph_version-lag",
		ComputedAt:  row.ComputedAt,
		MemberCount: int(row.MemberCount),
	}
}

// ----- Retrieval accessor (corpus / retrieval status) -----

// retrievalAccessorImpl is the STATUS-FOCUSED RetrievalAccessor backed
// by *Store + *Engine. See the file-level carve-out: non-status methods
// return zero values; the daemon's full production query paths live on
// semRetrievalAdapter.
type retrievalAccessorImpl struct {
	store  *semanticstore.Store
	engine *retrieval.Engine
}

// NewRetrievalAccessorForStore returns a status-focused RetrievalAccessor
// backed by *Store + *Engine. Used by the daemon's semRetrievalAdapter
// AND the Plan 69-06 E2E test — single implementation, two call sites.
//
// QueryBleve / PersonalizedPageRank / RetrievalPending / TopEdgesFor are
// intentionally minimal zero-value stubs — this constructor is for the
// STATUS surface only; daemon production query paths continue to use the
// full semRetrievalAdapter for those.
func NewRetrievalAccessorForStore(store *semanticstore.Store, eng *retrieval.Engine) semantic.RetrievalAccessor {
	return &retrievalAccessorImpl{store: store, engine: eng}
}

// QueryBleve — non-status method, zero-value stub. See file-level
// carve-out. Production callers use semRetrievalAdapter.QueryBleve.
func (a *retrievalAccessorImpl) QueryBleve(_ string, _ []string) ([]semantic.TextRank, error) {
	return nil, nil
}

// PersonalizedPageRank — non-status method, zero-value stub.
func (a *retrievalAccessorImpl) PersonalizedPageRank(_ context.Context, _ string, _ []string) ([]semantic.GraphRank, error) {
	return nil, nil
}

// RetrievalPending — non-status method, zero-value stub. The daemon's
// semRetrievalAdapter implements this against the per-workspace
// Recoverer.
func (a *retrievalAccessorImpl) RetrievalPending(_ workspace.WorkspaceKey) bool {
	return false
}

// TopEdgesFor — non-status method, zero-value stub.
func (a *retrievalAccessorImpl) TopEdgesFor(_ context.Context, _, _ string) ([]string, error) {
	return nil, nil
}

// RetrievalStatus derives the bleve-corpus state for ws, applying the
// closed-enum priority order:
//
//  1. engine == nil                                            → bleve-unavailable
//  2. GetMeta(MetaKeyCorpusVersion) empty/missing              → corpus_version-uninitialized
//  3. parsed corpus_version < store.CurrentGraphVersion(ws)    → corpus_version-lag (fields populated)
//  4. GetMeta(MetaKeyLastCompactAt) empty/missing              → compactor-never-ran
//  5. else                                                     → "" (healthy)
//
// All meta reads use the exported constants from package retrieval —
// no string literals.
func (a *retrievalAccessorImpl) RetrievalStatus(ws workspace.WorkspaceKey) semantic.RetrievalStatus {
	if a == nil || a.engine == nil {
		return semantic.RetrievalStatus{Reason: "bleve-unavailable"}
	}

	// 2. corpus_version meta presence.
	cvBytes, err := a.engine.GetMeta(retrieval.MetaKeyCorpusVersion)
	if err != nil || len(cvBytes) == 0 {
		return semantic.RetrievalStatus{Reason: "corpus_version-uninitialized"}
	}
	corpusVersion, parseErr := strconv.ParseUint(string(cvBytes), 10, 64)
	if parseErr != nil {
		// Unparseable corpus_version bytes is morally "uninitialized";
		// the meta key exists but its value is not a valid uint64.
		return semantic.RetrievalStatus{Reason: "corpus_version-uninitialized"}
	}

	// Read indexed_files (best-effort; absence here does NOT escalate
	// to a degradation reason — only corpus_version drives that).
	var indexedFiles int64
	if ifBytes, _ := a.engine.GetMeta(retrieval.MetaKeyIndexedFiles); len(ifBytes) > 0 {
		if v, err := strconv.ParseInt(string(ifBytes), 10, 64); err == nil {
			indexedFiles = v
		}
	}

	// Read last_compact_at (used both for the populated field AND for
	// the priority-4 "compactor-never-ran" reason).
	var lastCompactAt int64
	lastCompactBytes, _ := a.engine.GetMeta(retrieval.MetaKeyLastCompactAt)
	if len(lastCompactBytes) > 0 {
		if v, err := strconv.ParseInt(string(lastCompactBytes), 10, 64); err == nil {
			lastCompactAt = v
		}
	}

	// IndexedSymbols sourced from bleve DocCount (Plan 69-02).
	var indexedSymbols int64
	if n, err := a.engine.DocCount(); err == nil {
		indexedSymbols = int64(n)
	}

	rs := semantic.RetrievalStatus{
		CorpusVersion:  corpusVersion,
		IndexedFiles:   indexedFiles,
		IndexedSymbols: indexedSymbols,
		LastCompactAt:  lastCompactAt,
	}

	// 3. corpus_version-lag — fields populated, reason set.
	if a.store != nil {
		if gv, err := a.store.CurrentGraphVersion(context.Background(), ws.Hash()); err == nil && gv > 0 && corpusVersion < gv {
			rs.Reason = "corpus_version-lag"
			return rs
		}
	}

	// 4. compactor-never-ran.
	if len(lastCompactBytes) == 0 || lastCompactAt == 0 {
		rs.Reason = "compactor-never-ran"
		return rs
	}

	// 5. healthy.
	return rs
}

// ----- Compile-time guards -----

var (
	_ semantic.SchedulerAccessor = (*schedulerAccessorImpl)(nil)
	_ semantic.RetrievalAccessor = (*retrievalAccessorImpl)(nil)
	// *retrieval.Engine satisfies compact.BleveMeta via SetMeta(string, []byte) error.
	// This guard makes the Plan 69-05 bleveMetaFn closure binding (in
	// daemon.go) a single-line, compile-checked operation.
	_ compact.BleveMeta = (*retrieval.Engine)(nil)
)
