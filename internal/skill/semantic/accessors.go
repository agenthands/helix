package semantic

import (
	"context"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// This file declares ALL narrow accessor interfaces for the semantic skill.
// FINAL at end-of-W0 (Phase 64-03): every interface every wave-2 plan needs
// is declared here and frozen — wave-2 plans (64-04/05/06/07) only consume.
// Closes checkers B1, B3, B4, W1, W2 (per 64-03-PLAN.md).

// StoreAccessor is the narrow seam between SemanticSkill and *internal/semantic/store.Store.
// Daemon wires a concrete adapter in internal/daemon/semantic_wiring.go (P64-08).
//
// Phase 70-04 extensions (CurrentOverlayEpoch, OverlayChangedPathsSince,
// LatestCommittedSnapshotBaseEpoch) are pure read accessors over the semantic
// store; they do NOT mutate state and may be called concurrently with the
// existing accessors.
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
	// CurrentOverlayEpoch returns the current overlay write_epoch for repoID,
	// or 0 if the repo has never had overlay activity. Phase 70-04 seam.
	CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error)
	// OverlayChangedPathsSince returns the set of distinct overlay paths whose
	// write_epoch is strictly greater than baseEpoch, together with the
	// current overlay write_epoch. Phase 70-04 seam — feeds the incremental
	// drain path in collectCandidatePaths.
	OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) (paths []string, currentEpoch uint64, err error)
	// LatestCommittedSnapshotBaseEpoch returns the base_overlay_epoch stamped
	// on the most-recent committed snapshot for repoID. (0, false, nil)
	// signals cold-start (no committed snapshot exists yet). Phase 70-04
	// seam — supplies the baseline for OverlayChangedPathsSince.
	LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (epoch uint64, ok bool, err error)
}

// SchedulerAccessor surfaces RankScheduler state. ClusterStatus is consumed by
// tools_status.go (P64-06); it returns a structured value so the response
// shape is correct from day one. Phase 69-05 wires the production derivation
// via NewSchedulerAccessorForStore (internal/daemon/semantic_accessor_factories.go).
type SchedulerAccessor interface {
	// IsQuiescent reports whether the rank scheduler has no in-flight work
	// for the given repo (Phase 62 RankScheduler.IsQuiescent).
	IsQuiescent(repoID string) bool
	// ScoreStatus returns the closed-enum score status for the given (repo,
	// projection) pair (Phase 62 graph.ScoreStatus).
	ScoreStatus(repoID, projection string) graph.ScoreStatus
	// ClusterStatus returns the structured cluster status. Phase 69-05
	// wires the production derivation; states are "current" | "stale" |
	// "unknown" with closed-enum reasons (see RetrievalStatus for the
	// reason-priority pattern).
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
//
// Phase 70-04 extension: FlushNow is a SYNCHRONOUS flush of the coalescer's
// pending batch for the workspace. It is NOT a snapshot write — it just
// drains queued OnWorkspaceChanged events into the overlay so a subsequent
// OverlayChangedPathsSince query observes them. Refresh-tool callers (Plan
// 70-05) invoke this before OverlayChangedPathsSince to close the
// fire-and-forget race documented in RESEARCH.md Pitfall 1.
type LiveAccessor interface {
	// OnWorkspaceChanged drains the live queue for the given workspace,
	// applying overlay updates for the requested paths (or all queued paths
	// if paths is nil/empty).
	OnWorkspaceChanged(ws workspace.WorkspaceKey, paths []string) error
	// LastFlushAt returns the unix-millis timestamp of the most-recent
	// overlay flush for the given workspace, or 0 if no flush has occurred.
	LastFlushAt(ws workspace.WorkspaceKey) int64
	// FlushNow synchronously drains the coalescer's pending batch for ws.
	// Returns nil for unregistered workspaces (no-op). Phase 70-04 seam.
	FlushNow(ctx context.Context, ws workspace.WorkspaceKey) error
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
	// RetrievalStatus returns the bleve-corpus state for the given workspace
	// (Phase 69-04 STATUS-01). The Reason field follows the closed-enum
	// priority order documented on the RetrievalStatus type:
	//
	//   bleve-unavailable > corpus_version-uninitialized > corpus_version-lag
	//     > compactor-never-ran > "".
	//
	// Phase 69-05 wires the production adapter (semRetrievalAdapter); this
	// interface method is the seam they share.
	RetrievalStatus(ws workspace.WorkspaceKey) RetrievalStatus
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

// ----- Phase 71-01 additions: read-only seams for the P1 single-symbol tools. -----
//
// These three narrow interfaces are declared in this plan (71-01) so wave-2
// handler plans (71-03/04/05) can compile against the seam before the
// production daemon wiring lands. Each interface is independent and exposes
// exactly one method (Pattern F: narrow-interface convention).

// SymbolByNameAccessor resolves (file_path, symbol_name) tuples to one or
// more graph SymbolIDs at the latest committed snapshot. Production binding
// wraps *Store.QuerySymbolByName and casts []string → []integ.SymbolID.
//
// Returned slice is ordered deterministically (stable_key ASC) and capped
// at 6 entries so the seed resolver can detect "> 5 candidates" and apply
// the D1 ambiguous-truncation policy (cap 5).
type SymbolByNameAccessor interface {
	QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error)
}

// ExtractorRunAccessor surfaces a monotonic opaque identifier for the most
// recent extractor pass against repoID. Drives the v1.10 FreshnessV2
// envelope's extractor_run_id field (D5).
//
// Returns ("", nil) when no committed snapshot exists for the repo
// (cold-start case).
type ExtractorRunAccessor interface {
	LatestExtractorRunID(ctx context.Context, repoID string) (string, error)
}

// ClusterMembershipAccessor returns the cluster_id and member-count for the
// cluster that contains symbolID at the latest committed graph_version.
// Drives the optional cluster co-membership boost in 71-04
// find_related_symbols.
//
// If the production binding chooses to disable the boost (e.g., cluster
// engine not wired or accessor cost is non-trivial), the binding returns
// (0, 0, nil) and the handler emits fallback_reason:
// "cluster_boost_unavailable".
type ClusterMembershipAccessor interface {
	ClusterIDOf(ctx context.Context, repoID string, symbolID integ.SymbolID) (clusterID uint64, size int, err error)
}

// ----- Phase 71-03 additions: per-symbol type-chain + edges seams -----
//
// Phase 71-01 declared name-keyed symbol lookup + extractor-run + cluster
// membership. The 71-03 explain_symbol_deep handler additionally needs (a)
// per-symbol type-chain rows carrying evidence kind / tier and (b) per-symbol
// edge rows (callers + incoming + outgoing) carrying the internal_kind label
// the closed-enum surface mapper consumes. The existing StoreAccessor.
// QueryEffectiveAdjacency returns whole-graph adjacency keyed on uint64
// graph.NodeID with edge-weight values only — it cannot surface the
// internal_kind label the MCP edge surface needs (Pitfall 2: RESOLVES_TO →
// has_type) and cannot be filtered to one symbol cheaply. These two narrow
// accessors close that gap.
//
// Both interfaces are READ-ONLY (D-09 invariant; no Begin/Commit/Abort/Write
// methods). Production bindings (future plan) will wrap *Store SQL reads at
// the latest committed snapshot.

// TypeChainRow is one resolved type-chain entry surfaced to the
// explain_symbol_deep handler. Tier is the closed-enum SPEC §38.2 tier
// string ("tier1_lsp" .. "tier7_unknown"); EvidenceKind is the
// types.EvidenceKind serialization (`lsp` | `annotation` | `constructor` |
// `assignment` | `comment` | `heuristic` | `unknown`).
type TypeChainRow struct {
	Tier           string
	EvidenceKind   string
	TargetSymbolID string
}

// TypeChainAccessor returns the per-symbol type-chain rows used to populate
// explain_symbol_deep's type_chain response field. Empty slice signals "no
// chain rows materialized for this symbol" — not an error.
type TypeChainAccessor interface {
	TypeChainForSymbol(ctx context.Context, repoID string, sym integ.SymbolID) ([]TypeChainRow, error)
}

// SymbolEdgeRow is one per-symbol edge surfaced to the explain_symbol_deep
// handler. InternalKind is the raw extractor / resolver kind string
// (CALLS / REFERENCES / RESOLVES_TO / USES_TYPE / CONTAINS / IMPLEMENTS /
// EXTENDS / DEFINED_IN / IMPORTS / ...) — the handler runs MapInternalKind
// to derive the closed-enum surface EdgeKind in the response. From / To are
// the endpoint stable IDs; depending on edge direction one of the two may be
// the seed itself (callers: From=caller, To=seed; outgoing: From=seed,
// To=target; incoming: From=src, To=seed).
type SymbolEdgeRow struct {
	From         integ.SymbolID
	To           integ.SymbolID
	InternalKind string
}

// SymbolEdgesAccessor returns per-symbol edge rows partitioned by direction.
// All three methods are independent reads — implementations MAY share
// indexes but MUST NOT cache state between calls (D-09 read-only invariant).
//
// CallersOf is a specialization of IncomingEdgesOf filtered to CALLS edges;
// surfaced separately so the handler can apply the D2 callers≤50 cap before
// the edges≤100 cap.
type SymbolEdgesAccessor interface {
	CallersOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]SymbolEdgeRow, error)
	IncomingEdgesOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]SymbolEdgeRow, error)
	OutgoingEdgesOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]SymbolEdgeRow, error)
}

// DataFlowReachabilityAccessor is the read seam for trace_data_flow (v2.10):
// a bounded, deterministic source->sink reachability walk over DATA_FLOWS
// edges. ReachableFrom seeds at a PARAMETER symbol (the taint entry point —
// DATA_FLOWS edges are param->param, so a function seed cannot reach them at
// HEAD: QuerySymbolEdgesOutgoing(functionNode) returns the function's own
// CALLS/IMPLEMENTS edges, not its params' DATA_FLOWS edges). It returns the
// set of symbols reachable within maxHops, each with its hop distance. Read-
// only (D-09 invariant); nil means the seam is unwired and the handler MUST
// guard (degrade gracefully).
type DataFlowReachabilityAccessor interface {
	ReachableFrom(ctx context.Context, repoID string, seed integ.SymbolID, maxHops int) ([]ReachableNode, error)
}

// ReachableNode is one symbol reachable from the seed via DATA_FLOWS edges.
// Hops is 0 for the seed itself, 1+ for symbols reached in N data-flow hops.
type ReachableNode struct {
	SymbolID integ.SymbolID
	Hops     int
}

// ----- Phase 71-05 additions: per-edge evidence seam for validate_graph_edge -----
//
// EdgeEvidenceRow is one piece of citation evidence the validate_graph_edge
// handler consumes when assembling its evidence array. A single (from, to,
// internal_kind) edge may have multiple rows (e.g., many LSP citation sites
// or both an LSP citation and a type-resolver tier annotation).
//
// Field interpretation:
//   - InternalKind: the extractor kind (CALLS / RESOLVES_TO / ...). Always set.
//   - Source: the edge.Source prefix string (e.g., "lsp.go.text_document_references"
//     for LSP-backed citations, "" for AST-only / type-resolver-only rows).
//     The handler parses the `lsp.{lang}.{method}` shape to populate the
//     EvidenceCitation.LSPMethod field.
//   - TreeSitterKind: the tree-sitter node kind ("call_expression",
//     "selector_expression", ...) when AST metadata is available. Empty
//     string when AST contribution exists but the extractor did not stamp
//     metadata (Open Q3 partial path → envelope evidence_status drops to
//     "partial").
//   - File / Range: optional source location. Range is a per-tool local type
//     (see EvidenceRange) so the accessor seam does not pull in graph
//     package internals.
//   - Tier: SPEC §38.2 ladder string ("tier1_lsp" .. "tier7_unknown") when
//     the row carries type-resolver evidence; empty otherwise.
//   - EvidenceKind: types.EvidenceKind serialization ("lsp"|"annotation"|...).
//     Empty when the row is not a type-resolver row.
//   - ASTAttested: set to true when the graph attests an AST-derived
//     contribution but the extractor did not stamp tree_sitter_kind/Range
//     metadata. The handler emits an AST citation with empty TreeSitterKind
//     and degrades the envelope's evidence_status to "partial".
type EdgeEvidenceRow struct {
	InternalKind   string
	Source         string
	TreeSitterKind string
	File           string
	Range          *EvidenceRange
	Tier           string
	EvidenceKind   string
	ASTAttested    bool
}

// EvidenceRange is the optional source-range citation co-located with an
// EdgeEvidenceRow. Declared in the semantic skill package (not graph) to keep
// the narrow accessor seam free of graph-package transitive imports.
type EvidenceRange struct {
	StartLine uint32 `json:"start_line"`
	StartCol  uint32 `json:"start_col"`
	EndLine   uint32 `json:"end_line"`
	EndCol    uint32 `json:"end_col"`
}

// EdgeEvidenceAccessor reads the per-edge evidence citations used by the
// validate_graph_edge handler to compute confidence + evidence_status. The
// handler passes the set of candidate internal_kinds (the reverse projection
// of the surface enum, e.g., "has_type" → ["RESOLVES_TO"]) and receives one
// or more rows per matched edge.
//
// Empty slice with nil error signals "no rows for any of the requested
// internal_kinds" — the handler interprets this as the edge being absent
// (fallback_reason="edge_not_found").
//
// READ-ONLY (D-09 invariant; no Begin/Commit/Abort/Write methods). Production
// binding (deferred to the daemon adapter wave) wraps *Store SQL reads at the
// latest committed snapshot.
type EdgeEvidenceAccessor interface {
	EvidenceForEdge(ctx context.Context, repoID string, from, to integ.SymbolID, internalKinds []string) ([]EdgeEvidenceRow, error)
}

// ----- Phase 72-01 additions: read-only seams for the P1 cluster & impact tools. -----
//
// Four narrow interfaces declared here so wave-2 handler plans (72-02, 72-03,
// 72-04) can compile against the seam before the production daemon wiring
// lands. Each interface is independent and exposes the minimal surface needed
// by its consumer handler. Row types are skill-layer types (not store-internal
// types) to keep the accessor seam free of store-package transitive imports.

// ClusterSummaryRow is one cluster summary entry returned by
// ClusterMapAccessor.QueryClusterSummaries. ClusterIntID is the raw uint64
// cluster identifier scoped by (projection, graph_version); MemberCount is
// the number of symbols in the cluster (from semantic_clusters.score which
// overlay.go:835 overloads with the planning-time cardinality).
type ClusterSummaryRow struct {
	ClusterIntID uint64
	MemberCount  int
}

// ClusterMemberRow is one cluster member entry returned by
// ClusterMemberAccessor.QueryClusterMembers. NodeID is the raw uint64
// graph node id; SymbolID is the stable_key from semantic_symbols (the
// opaque identifier for MCP surface consumers).
type ClusterMemberRow struct {
	NodeID   uint64
	SymbolID string
}

// ClusterMapAccessor returns workspace-level cluster summaries sorted by
// member count descending. Drives the get_cluster_map handler (Phase 72 D2).
// Production binding wraps *Store.QueryClusterSummaries.
type ClusterMapAccessor interface {
	QueryClusterSummaries(ctx context.Context, repoID, projection string, graphVersion uint64, topN int) ([]ClusterSummaryRow, error)
}

// ClusterMemberAccessor returns per-cluster member rows with stable symbol ids.
// Drives the explain_cluster handler (Phase 72). Production binding wraps
// *Store.QueryClusterMembers.
type ClusterMemberAccessor interface {
	QueryClusterMembers(ctx context.Context, repoID, projection string, graphVersion, clusterIntID uint64, limit int) ([]ClusterMemberRow, error)
}

// ClusterPageRankAccessor returns per-node PageRank scores for the supplied
// node ids from semantic_graph_scores. Drives per-member ranking in
// explain_cluster (Phase 72 D2). Production binding wraps
// *Store.QueryNodePageRanks.
type ClusterPageRankAccessor interface {
	QueryNodePageRanks(ctx context.Context, repoID, projection string, graphVersion uint64, nodeIDs []uint64) (map[uint64]float64, error)
}

// ImpactLookupAccessor is the narrow seam between get_change_impact_graph
// (Phase 72 D3) and the daemon-resident SemanticLookup. It exposes exactly
// the two SemanticLookup methods the handler needs — ExpandFrom for graph
// traversal and Status for envelope/freshness derivation — without injecting
// the full integ.SemanticLookup interface (OQ-2 resolution).
//
// Production binding wraps *integSemanticLookup (internal/daemon/semantic_wiring.go).
// READ-ONLY: no Begin/Commit/Abort/Write on this path.
type ImpactLookupAccessor interface {
	ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym integ.SymbolID, depth int) ([]integ.Impact, error)
	Status(ctx context.Context, ws workspace.WorkspaceKey) (integ.SemanticStatus, error)
}
