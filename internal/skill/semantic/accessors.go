package semantic

import (
	"context"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/workspace"
)

// This file declares ALL narrow accessor interfaces for the semantic skill.
// FINAL at end-of-W0 (Phase 64-03): every interface every wave-2 plan needs
// is declared here and frozen — wave-2 plans (64-04/05/06/07) only consume.
// Closes checkers B1, B3, B4, W1, W2 (per 64-03-PLAN.md).

// StoreAccessor is the narrow seam between SemanticSkill and *internal/semantic/store.Store.
// Daemon wires a concrete adapter in internal/daemon/semantic_wiring.go (P64-08).
type StoreAccessor interface {
	// LatestCommittedSnapshot returns the most-recent committed snapshot id
	// for the given repo, or 0 if no committed snapshot exists yet.
	LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error)
	// CurrentGraphVersion returns the workspace's current graph_version (the
	// version bumped by Phase 62 ApplyRepair on overlay updates).
	CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error)
	// OverlayHasPendingRows reports whether the live overlay has any pending
	// rows for the given repo (i.e., overlay_active = true).
	OverlayHasPendingRows(repoID string) bool
	// QueryEffectiveAdjacency returns (out, in) adjacency for the (repo,
	// projection) effective graph (snapshot ⊕ overlay − tombstones).
	QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
		out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
	)
}

// SchedulerAccessor surfaces RankScheduler state. ClusterStatus is consumed by
// tools_status.go (P64-06); it returns a structured value so the response shape
// is correct even when Phase 62 has no live cluster-status accessor (in which
// case the production adapter returns
// ClusterStatus{State: "unknown", Reason: "phase-62-clustering-no-status-accessor"}
// until Phase 65/67 wires a real source). Closes checker W1.
type SchedulerAccessor interface {
	// IsQuiescent reports whether the rank scheduler has no in-flight work
	// for the given repo (Phase 62 RankScheduler.IsQuiescent).
	IsQuiescent(repoID string) bool
	// ScoreStatus returns the closed-enum score status for the given (repo,
	// projection) pair (Phase 62 graph.ScoreStatus).
	ScoreStatus(repoID, projection string) graph.ScoreStatus
	// ClusterStatus returns the structured cluster status. Production
	// adapter returns
	// {State: "unknown", Reason: "phase-62-clustering-no-status-accessor"}
	// until Phase 65/67 wires a real source.
	ClusterStatus(repoID string) ClusterStatus
}

// QueueAccessor is the narrow seam to Phase 61's LSPQueue.
type QueueAccessor interface {
	// DepthAll returns the total number of pending LSP enrichment items
	// across all lanes for the given workspace.
	DepthAll(ws workspace.WorkspaceKey) int
	// LastEnqueueAt returns the unix-millis timestamp of the most-recent
	// enqueue for the given workspace, or 0 if the queue has been idle.
	LastEnqueueAt(ws workspace.WorkspaceKey) int64
}

// LiveAccessor is the narrow seam to Phase 60's live update service.
type LiveAccessor interface {
	// OnWorkspaceChanged drains the live queue for the given workspace,
	// applying overlay updates for the requested paths (or all queued paths
	// if paths is nil/empty).
	OnWorkspaceChanged(ws workspace.WorkspaceKey, paths []string) error
	// LastFlushAt returns the unix-millis timestamp of the most-recent
	// overlay flush for the given workspace, or 0 if no flush has occurred.
	LastFlushAt(ws workspace.WorkspaceKey) int64
}

// RunnerAccessor is implemented by *IndexRunner (P64-04 owns the type).
//
// ResolveAuto resolves mode=auto -> "full" | "incremental" per CONTEXT.md
// D-03. The handler MUST call ResolveAuto BEFORE Run so the singleflight key
// is keyed on the RESOLVED mode (closes checker B5).
type RunnerAccessor interface {
	// Run executes (or joins an in-flight) index build for the workspace
	// under the resolved mode, blocking up to maxMs milliseconds. On
	// timeout, returns an IndexResult with Partial=true and Status=building.
	Run(ctx context.Context, ws workspace.WorkspaceKey, mode string, maxMs int) (IndexResult, error)
	// ResolveAuto resolves mode="auto" to either "full" (no committed
	// snapshot exists) or "incremental" (committed snapshot exists). Per
	// CONTEXT.md D-03.
	ResolveAuto(ctx context.Context, ws workspace.WorkspaceKey) string
}

// RetrievalAccessor is implemented by *retrieval.Engine wrapper (P64-07).
//
// TopEdgesFor returns up to 5 top-weighted graph edges incident to symbolID,
// formatted as descriptive strings for ContextEvidence.TopEdges.
type RetrievalAccessor interface {
	// QueryBleve runs a bleve full-text query against the indexed corpus
	// of symbol facts; results are biased toward the anchored neighborhood
	// (CONTEXT.md D-05/D-06).
	QueryBleve(task string, anchors []string) ([]TextRank, error)
	// PersonalizedPageRank returns the per-symbol PageRank scores under
	// personalization seeded by anchors (CONTEXT.md D-05).
	PersonalizedPageRank(ctx context.Context, repoID string, anchors []string) ([]GraphRank, error)
	// RetrievalPending reports whether the retrieval engine is still
	// rebuilding (e.g., bleve recovery after daemon restart). The
	// get_semantic_context handler surfaces this via response envelope.
	RetrievalPending(ws workspace.WorkspaceKey) bool
	// TopEdgesFor returns up to 5 top-weighted graph edges incident to
	// symbolID, formatted as descriptive strings for ContextEvidence.TopEdges.
	TopEdgesFor(ctx context.Context, repoID, symbolID string) ([]string, error)
}

// CompactorAccessor is the narrow seam to Phase 63's per-workspace compactor.
//
// refresh_semantic_graph (P64-05) MUST NOT call OnFlush (D-13); the
// recorder-style mock in 64-05's RED tests asserts non-invocation.
// Production adapter delegates to internal/semantic/compact/compactor.go
// OnFlush. Closes checker B3.
type CompactorAccessor interface {
	// OnFlush is the explicit compaction trigger; only
	// index_semantic_graph --mode=incremental should call it (D-10/D-13).
	OnFlush(ws workspace.WorkspaceKey) error
}

// SessionAccessor is the bridge to the per-request *mcp.SessionInfo and the
// per-request workspace.WorkspaceKey. Daemon wires this via SetSessionAccessor
// in P64-08, passing the same closure used for InstallMiddleware (since
// getSession is an injected closure, NOT an exported package symbol — verified
// during planning). Closes checker W2.
//
// Why two methods (Session + Workspace): SessionInfo carries a hashed
// WorkspaceKey *string*, not the canonical workspace.WorkspaceKey *struct*.
// The daemon's adapter owns the translation between MCP session state and
// the workspace registry, so SemanticSkill consumes the WorkspaceKey via a
// dedicated method rather than reaching into SessionInfo internals.
type SessionAccessor interface {
	// Session returns the SessionInfo bound to the given request context, or
	// nil if no session is active.
	Session(ctx context.Context) *mcp.SessionInfo
	// Workspace returns the WorkspaceKey for the request's active workspace,
	// or the zero value if no workspace is bound.
	Workspace(ctx context.Context) workspace.WorkspaceKey
}
