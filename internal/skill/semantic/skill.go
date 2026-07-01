// Package semantic implements the semantic skill, contributing 10 MCP tools
// for indexing, refreshing, inspecting, and querying the live semantic graph
// (Phase 64).
//
// This file (skill.go) is FINAL at end-of-W0 (Phase 64-03). Wave-1 (64-04)
// and Wave-2 (64-05/06/07) plans NEVER re-edit this file except to delete a
// single help-text stub line each from the stub-declaration block at the
// bottom of the file. Each stub deletion has zero conflict surface across
// plans because each plan removes a distinct constant.
package semantic

import (
	"context"
	"log/slog"
	"sync"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// SemanticSkill implements skill.ToolProvider, exposing 10 MCP tools backed by
// the Phase 60-63 semantic engine: the 4 P0 tools (index_semantic_graph,
// refresh_semantic_graph, get_semantic_graph_status, get_semantic_context) and
// the 6 P1 tools (explain_symbol_deep, find_related_symbols, validate_graph_edge,
// get_cluster_map, explain_cluster, get_change_impact_graph).
type SemanticSkill struct {
	mu        sync.Mutex
	logger    *slog.Logger
	store     StoreAccessor
	scheduler SchedulerAccessor
	queue     QueueAccessor
	live      LiveAccessor
	runner    RunnerAccessor
	retrieval RetrievalAccessor
	compactor CompactorAccessor
	session   SessionAccessor // injected: ctx -> *mcp.SessionInfo (closes checker W2)

	// Phase 71-01 additions: read-only seams for the P1 single-symbol tools
	// (71-03 explain_symbol_deep, 71-04 find_related_symbols,
	// 71-05 validate_graph_edge). All three are post-init wired by the
	// daemon adapter; nil means the seam is unwired (handler MUST guard).
	symbolByName      SymbolByNameAccessor
	extractorRun      ExtractorRunAccessor
	clusterMembership ClusterMembershipAccessor

	// Phase 71-03 additions: per-symbol type-chain + edges seams driving
	// explain_symbol_deep. Both are read-only; nil means the seam is
	// unwired and the handler degrades gracefully (empty type_chain / no
	// edges) rather than erroring.
	typeChain   TypeChainAccessor
	symbolEdges          SymbolEdgesAccessor
	dataFlowReachability DataFlowReachabilityAccessor // v2.10: trace_data_flow seed->sink reachability over DATA_FLOWS

	// Phase 71-05 addition: per-edge evidence seam driving
	// validate_graph_edge. Read-only; nil means the seam is unwired and
	// the handler degrades the response to evidence_status=none with
	// fallback_reason="evidence_lookup_unavailable" while still answering
	// edge-presence (D4 lenient stance).
	edgeEvidence EdgeEvidenceAccessor

	// Phase 72 additions: read-only seams for the P1 cluster & impact tools
	// (get_cluster_map, explain_cluster, get_change_impact_graph). All four
	// are post-init wired by the daemon adapter; nil means the seam is
	// unwired and the handler MUST guard (degrade gracefully).
	clusterMap      ClusterMapAccessor
	clusterMember   ClusterMemberAccessor
	clusterPageRank ClusterPageRankAccessor
	impactLookup    ImpactLookupAccessor
}

func init() { skill.Register(&SemanticSkill{}) }

// Name returns the skill identifier.
func (s *SemanticSkill) Name() string { return "semantic" }

// Description returns a human-readable description.
func (s *SemanticSkill) Description() string {
	return "Semantic graph indexing and retrieval (10 tools)"
}

// Init initializes the skill with shared dependencies.
func (s *SemanticSkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}
	return nil
}

// ----- Post-init setters (mirror RepoMapSkill.SetEnrichFn). FINAL set; wave-1/wave-2 plans never add more. -----

// SetStore wires the StoreAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetStore(a StoreAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = a
}

// SetScheduler wires the SchedulerAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetScheduler(a SchedulerAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduler = a
}

// SetQueue wires the QueueAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetQueue(a QueueAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = a
}

// SetLive wires the LiveAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetLive(a LiveAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live = a
}

// SetRunner wires the RunnerAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetRunner(a RunnerAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runner = a
}

// SetRetrieval wires the RetrievalAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetRetrieval(a RetrievalAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retrieval = a
}

// SetCompactor wires the CompactorAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetCompactor(a CompactorAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactor = a
}

// SetSessionAccessor wires the per-request session lookup. Daemon (P64-08)
// passes the same closure used by InstallMiddleware (since getSession is an
// injected closure, NOT an exported package symbol). Closes checker W2.
func (s *SemanticSkill) SetSessionAccessor(a SessionAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = a
}

// SetSymbolByName wires the SymbolByNameAccessor adapter (Phase 71-01 seam).
// Used by 71-03/04/05 handler plans via the shared resolveSeed helper.
func (s *SemanticSkill) SetSymbolByName(a SymbolByNameAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.symbolByName = a
}

// SetExtractorRun wires the ExtractorRunAccessor adapter (Phase 71-01 seam).
// Drives the v1.10 FreshnessV2 envelope's extractor_run_id field.
func (s *SemanticSkill) SetExtractorRun(a ExtractorRunAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.extractorRun = a
}

// SetClusterMembership wires the ClusterMembershipAccessor adapter
// (Phase 71-01 seam). Drives the optional cluster co-membership boost in
// 71-04 find_related_symbols. Production binding may return (0, 0, nil)
// to disable the boost, in which case 71-04 emits fallback_reason:
// "cluster_boost_unavailable".
func (s *SemanticSkill) SetClusterMembership(a ClusterMembershipAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterMembership = a
}

// SetTypeChain wires the TypeChainAccessor adapter (Phase 71-03 seam).
// Drives the explain_symbol_deep type_chain response field. Production
// binding wraps the *Store type-chain row reader at the latest committed
// snapshot.
func (s *SemanticSkill) SetTypeChain(a TypeChainAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.typeChain = a
}

// SetSymbolEdges wires the SymbolEdgesAccessor adapter (Phase 71-03 seam).
// Drives the explain_symbol_deep callers / edges_incoming / edges_outgoing
// response fields. Production binding wraps the *Store per-symbol edge
// reader at the latest committed snapshot.
func (s *SemanticSkill) SetSymbolEdges(a SymbolEdgesAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.symbolEdges = a
}

// SetDataFlowReachability wires the DataFlowReachabilityAccessor adapter
// (v2.10 seam). Drives trace_data_flow's source->sink reachability walk over
// DATA_FLOWS edges. Production binding wraps the *Store bulk-edge reader at the
// latest committed snapshot.
func (s *SemanticSkill) SetDataFlowReachability(a DataFlowReachabilityAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dataFlowReachability = a
}

// getDataFlowReachability returns the wired DataFlowReachabilityAccessor under
// the skill mutex, or nil (the handler MUST nil-guard and degrade gracefully).
func (s *SemanticSkill) getDataFlowReachability() DataFlowReachabilityAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dataFlowReachability
}

// SetEdgeEvidence wires the EdgeEvidenceAccessor adapter (Phase 71-05 seam).
// Drives validate_graph_edge's evidence array + confidence computation.
// Production binding wraps the *Store per-edge evidence reader at the latest
// committed snapshot.
func (s *SemanticSkill) SetEdgeEvidence(a EdgeEvidenceAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.edgeEvidence = a
}

// SetClusterMap wires the ClusterMapAccessor adapter (Phase 72 seam).
// Drives get_cluster_map's top-N cluster list. Production binding wraps
// *Store.QueryClusterSummaries.
func (s *SemanticSkill) SetClusterMap(a ClusterMapAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterMap = a
}

// SetClusterMember wires the ClusterMemberAccessor adapter (Phase 72 seam).
// Drives explain_cluster's member list. Production binding wraps
// *Store.QueryClusterMembers.
func (s *SemanticSkill) SetClusterMember(a ClusterMemberAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterMember = a
}

// SetClusterPageRank wires the ClusterPageRankAccessor adapter (Phase 72 seam).
// Drives per-member PageRank ranking in explain_cluster. Production binding
// wraps *Store.QueryNodePageRanks.
func (s *SemanticSkill) SetClusterPageRank(a ClusterPageRankAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clusterPageRank = a
}

// SetImpactLookup wires the ImpactLookupAccessor adapter (Phase 72 seam).
// Drives get_change_impact_graph's graph traversal. Production binding wraps
// *integSemanticLookup.
func (s *SemanticSkill) SetImpactLookup(a ImpactLookupAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.impactLookup = a
}

// getClusterMap returns the wired ClusterMapAccessor under the skill mutex.
// Returns nil when the accessor has not been wired (handler MUST guard).
func (s *SemanticSkill) getClusterMap() ClusterMapAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clusterMap
}

// getClusterMember returns the wired ClusterMemberAccessor under the skill mutex.
// Returns nil when the accessor has not been wired (handler MUST guard).
func (s *SemanticSkill) getClusterMember() ClusterMemberAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clusterMember
}

// getClusterPageRank returns the wired ClusterPageRankAccessor under the skill mutex.
// Returns nil when the accessor has not been wired (handler MUST guard).
func (s *SemanticSkill) getClusterPageRank() ClusterPageRankAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.clusterPageRank
}

// getImpactLookup returns the wired ImpactLookupAccessor under the skill mutex.
// Returns nil when the accessor has not been wired (handler MUST guard).
func (s *SemanticSkill) getImpactLookup() ImpactLookupAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.impactLookup
}

// sessionSnapshot returns the SessionSnapshot for the current request, or a
// zero-value snapshot when the daemon has not wired SessionAccessor (test
// path). This is the seam through which checkMode reads session.Mode.
func (s *SemanticSkill) sessionSnapshot(ctx context.Context) mcp.SessionSnapshot {
	s.mu.Lock()
	a := s.session
	s.mu.Unlock()
	if a == nil {
		return mcp.SessionSnapshot{}
	}
	sess := a.Session(ctx)
	if sess == nil {
		return mcp.SessionSnapshot{}
	}
	return sess.Snapshot()
}

// workspaceKey returns the WorkspaceKey for the current request. Returns the
// zero-value workspace.WorkspaceKey{} when SessionAccessor has not been wired
// (test path) or the session has no workspace bound.
//
// Implementation reads via SessionAccessor.Workspace(ctx) (not via the
// SessionSnapshot, which only carries scalar fields like Mode/Profile/Language
// plus a hashed WorkspaceKey string — not the canonical workspace.WorkspaceKey
// struct). The daemon's SessionAccessor adapter (P64-08) owns the translation
// from *mcp.SessionInfo to workspace.WorkspaceKey.
//
// Authored in this plan (W1 closure) so wave-1/wave-2 handler plans
// (64-04/05/06/07) consume `s.workspaceKey(ctx)` without re-editing skill.go.
func (s *SemanticSkill) workspaceKey(ctx context.Context) workspace.WorkspaceKey {
	s.mu.Lock()
	a := s.session
	s.mu.Unlock()
	if a == nil {
		return workspace.WorkspaceKey{}
	}
	return a.Workspace(ctx)
}

// Tools returns the 10 MCP tool definitions for semantic graph operations.
//
// HelpText constants (indexHelp, refreshHelp, statusHelp, contextHelp) are
// declared in tool-specific files added by Wave 1/2 plans:
//   - tools_index.go    (W1 / 64-04) owns the real `const indexHelp`.
//   - tools_refresh.go  (W2 / 64-05) owns the real `const refreshHelp`.
//   - tools_status.go   (W2 / 64-06) owns the real `const statusHelp`.
//   - tools_context.go  (W2 / 64-07) owns the real `const contextHelp`.
//
// Until those tool files land, the four var declarations at the bottom of
// this file (the "help-text stub block") provide compile-time placeholders.
// Each W1/W2 tool plan deletes its stub line AND adds the real `const`
// declaration in its tool file.
//
// Phase 73-01: 6 P1 tools appended so skill.ResolveTools sees the full
// surface (explain_symbol_deep, find_related_symbols, validate_graph_edge,
// get_cluster_map, explain_cluster, get_change_impact_graph).
func (s *SemanticSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{
			Name:             "index_semantic_graph",
			Description:      "Build or refresh a committed semantic snapshot.",
			BriefDescription: "Index the semantic graph",
			HelpText:         indexHelp,
		},
		{
			Name:             "refresh_semantic_graph",
			Description:      "Apply pending live source changes (read+).",
			BriefDescription: "Refresh live overlay",
			HelpText:         refreshHelp,
		},
		{
			Name:             "get_semantic_graph_status",
			Description:      "Return semantic graph status (read+).",
			BriefDescription: "Semantic index status",
			HelpText:         statusHelp,
		},
		{
			Name:             "get_semantic_context",
			Description:      "Ranked, evidence-backed semantic context (read+).",
			BriefDescription: "Semantic context retrieval",
			HelpText:         contextHelp,
		},
		// Phase 73-01: P1 tools — ToolDef values copied verbatim from each
		// tool's server.Registry().Register block (per Phase 73 PATTERNS.md).
		{
			Name:             "explain_symbol_deep",
			Description:      "Deep symbol explanation: type chain, callers, edges, cluster (read+).",
			BriefDescription: "Deep symbol explanation",
			HelpText:         explainSymbolDeepHelp,
		},
		{
			Name:             "find_related_symbols",
			Description:      "Top-k semantically related symbols around a seed (read+).",
			BriefDescription: "Related symbols around a seed",
			HelpText:         findRelatedSymbolsHelp,
		},
		{
			Name:             "validate_graph_edge",
			Description:      "Validate a (from, to, edge_kind) graph claim with confidence + evidence (read+).",
			BriefDescription: "Validate a graph edge claim",
			HelpText:         validateGraphEdgeHelp,
		},
		{
			Name:             "get_cluster_map",
			Description:      "Workspace-level cluster overview: count, top-N clusters, members, representative symbols, dominant edge kinds (read+).",
			BriefDescription: "Workspace cluster overview",
			HelpText:         getClusterMapHelp,
		},
		{
			Name:             "explain_cluster",
			Description:      "Full cluster member list with per-member PageRank, cohesion/conductance metrics, and dominant entry points (read+).",
			BriefDescription: "Cluster member detail",
			HelpText:         explainClusterHelp,
		},
		{
			Name:             "get_change_impact_graph",
			Description:      "Pre-edit blast-radius subgraph (nodes + edges + edge kinds) for a seed symbol (review+).",
			BriefDescription: "Change impact subgraph",
			HelpText:         getChangeImpactGraphHelp,
		},
		{
			Name:             "trace_data_flow",
			Description:      "Source->sink reachability over DATA_FLOWS edges from a seed parameter (read+).",
			BriefDescription: "Data-flow reachability",
			HelpText:         traceDataFlowHelp,
		},
	}
}

// GetSemanticSkill returns the registered SemanticSkill instance for daemon
// post-init wiring (P64-08). Returns nil if the skill has not been registered.
func GetSemanticSkill() *SemanticSkill {
	s, ok := skill.Get("semantic")
	if !ok {
		return nil
	}
	ss, ok := s.(*SemanticSkill)
	if !ok {
		return nil
	}
	return ss
}

// ----- Help-text stubs (W0 placeholder block). -----
//
// Each W1/W2 tool plan REPLACES its stub:
//   - 64-04 (W1) deletes `indexHelp` line below + introduces
//     `const indexHelp = "..."` in tools_index.go.
//   - 64-05 (W2) deletes `refreshHelp` line below + introduces
//     `const refreshHelp = "..."` in tools_refresh.go.
//   - 64-06 (W2) deletes `statusHelp` line below + introduces
//     `const statusHelp = "..."` in tools_status.go.
//   - 64-07 (W2) deletes `contextHelp` line below + introduces
//     `const contextHelp = "..."` in tools_context.go.
//
// Each deletion is a SINGLE LINE — zero overlap across plans, so wave-2
// plans can land in parallel. The vars (rather than consts) accommodate
// the const declaration that W1/W2 plans bring in their tool files.
var ()
