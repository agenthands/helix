package semantic

// INVARIANT (D-09 / D-13): explain_cluster MUST NOT touch the snapshot-write
// surface of *Store. Specifically: no Begin/Commit/Abort/Write methods on
// snapshots, and no compactor flush trigger. The grep gate in CI enforces
// the absence of those identifier tokens in this file; the recorder mocks in
// tools_explain_cluster_test.go enforce it under unit test. Read+ stays
// read-only with respect to committed state — explain_cluster only consumes
// the existing committed snapshot through narrow ClusterMemberAccessor /
// ClusterPageRankAccessor / StoreAccessor seams.

import (
	"context"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
)

// D4 surface caps for explain_cluster (CONTEXT.md).
const (
	explainClusterDefaultMaxMembers = 200
	explainClusterMaxMembers        = 1000
)

// ExplainClusterArgs is the typed-args input schema for explain_cluster.
type ExplainClusterArgs struct {
	// ClusterID is the opaque composite token returned by get_cluster_map.
	// Format: "{projection}:{graph_version}:{cluster_int_id}".
	ClusterID string `json:"cluster_id" jsonschema:"opaque cluster token from get_cluster_map"`
	// MaxMembers limits the number of member entries returned. Default 200, max 1000.
	MaxMembers int `json:"max_members,omitempty" jsonschema:"maximum members to return (default 200, max 1000)"`
}

// ClusterMemberEntry is one member entry in the explain_cluster response.
type ClusterMemberEntry struct {
	// SymbolID is the stable graph identifier for this member symbol.
	SymbolID string `json:"symbol_id"`
	// PageRank is the member's PageRank score (0.0 when not available).
	PageRank float64 `json:"page_rank"`
	// IsEntryPoint indicates the symbol is an exported entry point
	// (capital-letter heuristic from stable_key's last ':'-separated segment).
	IsEntryPoint bool `json:"is_entry_point"`
}

// ExplainClusterResult is the explain_cluster response shape.
type ExplainClusterResult struct {
	// ClusterID echoes back the input cluster_id token.
	ClusterID string `json:"cluster_id"`
	// MemberCount is the total number of members in the cluster (before cap).
	MemberCount int `json:"member_count"`
	// MembersReturned is the count of members included in this response (after cap).
	MembersReturned int `json:"members_returned"`
	// Members is the list of member entries sorted by PageRank desc.
	Members []ClusterMemberEntry `json:"members"`
	// Cohesion is the intra-cluster edge density:
	//   intra_edges / (n * (n-1)) for directed graph.
	// 0.0 when n < 2 or no adjacency data.
	Cohesion float64 `json:"cohesion"`
	// Separation is the conductance metric:
	//   leaving_edges / (2*intra_edges + leaving_edges).
	// 0.0 when no edges.
	Separation float64 `json:"separation"`
	// DominantEdgeKinds is the top-3 edge kinds by intra-cluster count.
	DominantEdgeKinds []EdgeKindSurface `json:"dominant_edge_kinds"`
	// EntryPoints is the subset of Members that are exported (is_entry_point=true),
	// sorted by PageRank desc.
	EntryPoints []ClusterMemberEntry `json:"entry_points"`
	// Freshness is the FreshnessV2 envelope.
	Freshness FreshnessV2 `json:"freshness"`
	// FallbackReason is non-empty when the response is degraded.
	// Closed enum: "stale_cluster_id".
	FallbackReason string `json:"fallback_reason,omitempty"`
}

// explainClusterHelp is the verbose help text for explain_cluster.
var explainClusterHelp = `
## Usage Examples

Get full member list for a cluster:
  explain_cluster(cluster_id="weak_components:42:7")

Limit response to top members:
  explain_cluster(cluster_id="weak_components:42:7", max_members=50)

## Parameters
- cluster_id (string, required): opaque token from get_cluster_map cluster entries.
- max_members (int, optional): maximum members to return. Default 200, max 1000.

## Return Shape
- cluster_id (string): echoed input token.
- member_count (int): total cluster members before cap.
- members_returned (int): count included in this response.
- members ([]ClusterMemberEntry): list sorted by PageRank desc. Each entry:
    * symbol_id (string): stable graph identifier.
    * page_rank (float64): PageRank score (0.0 when unavailable).
    * is_entry_point (bool): true when the symbol is exported (capital-letter heuristic).
- cohesion (float64): intra_edges / (n*(n-1)) — 0.0 for n<2.
- separation (float64): conductance — leaving/(2*intra+leaving).
- dominant_edge_kinds ([]EdgeKindSurface): top-3 intra-cluster edge kinds.
- entry_points ([]ClusterMemberEntry): exported members sorted by PageRank desc.
- freshness (FreshnessV2): {graph_version, snapshot_id, extractor_run_id,
  as_of_unix_ms, status ∈ {current|stale|unknown}, source}.
- fallback_reason (string, optional): "stale_cluster_id" when the embedded
  graph_version is no longer current. Call get_cluster_map to refresh tokens.

## Mode Tier
read+ — every session passes.

## Stale-cluster-id Recovery
When fallback_reason=="stale_cluster_id", the cluster token's embedded
graph_version does not match the current workspace graph_version. The full
response still includes freshness with the current graph_version so the caller
can determine whether to re-run get_cluster_map.

## Cohesion / Separation (OQ-3)
Metrics are computed on demand from QueryEffectiveAdjacency.
TODO(perf): O(E) per explain_cluster call; future: persist intra_edge_count +
leaving_edge_count on semantic_clusters.`

// registerExplainCluster wires explain_cluster into the MCP server.
func registerExplainCluster(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "explain_cluster",
		Description: "Full cluster member list with per-member PageRank, cohesion/conductance metrics, and dominant entry points (read+).",
	}, kernel.WrapToolSpan(tracer, "explain_cluster",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args ExplainClusterArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleExplainCluster(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "explain_cluster",
		Description:      "Full cluster member list with per-member PageRank, cohesion/conductance metrics, and dominant entry points (read+).",
		BriefDescription: "Cluster member detail",
		HelpText:         explainClusterHelp,
	})
}

// isExportedSymbol checks if the last component of a stable_key starts with an
// uppercase letter. stable_key format: "pkg:TypeName:MethodName" — the last
// ':'-delimited segment's first byte determines exportedness.
//
// Mirrors internal/kernel/symbols/blast_radius_strangler.go:isExported but
// declared here as a package-private function to respect the vet-nokernel2semantic
// boundary (no import of kernel packages from skill/semantic).
func isExportedSymbol(stableKey string) bool {
	if i := strings.LastIndexByte(stableKey, ':'); i >= 0 && i+1 < len(stableKey) {
		return stableKey[i+1] >= 'A' && stableKey[i+1] <= 'Z'
	}
	return stableKey != "" && stableKey[0] >= 'A' && stableKey[0] <= 'Z'
}

// handleExplainCluster is the testable handler body for explain_cluster.
// Step order is load-bearing:
//  1. checkMode(modeTierRead) — first call; every session passes (retained for
//     code-review visibility and Phase 66 GuardrailMiddleware precedent).
//  2. decodeClusterID(args.ClusterID) — return errorResult on parse failure.
//  3. assembleFreshness to get current graph_version; stale-id check by comparing
//     decoded.GraphVersion with freshness.GraphVersion; return soft signal on mismatch.
//  4. Workspace key + repoID.
//  5. Clamp max_members to [1, 1000], default 200.
//  6. s.getClusterMember().QueryClusterMembers(...) with limit — nil-guarded.
//  7. s.getClusterPageRank().QueryNodePageRanks(...) for per-member ranking — nil-guarded.
//  8. Sort members by PageRank desc → ClusterMemberEntry list.
//  9. Compute cohesion + conductance from s.getStore().QueryEffectiveAdjacency(...)
//     filtered to intra/leaving edges for this cluster's node set.
//     TODO(perf): O(E) per explain_cluster call; future: persist intra_edge_count +
//     leaving_edge_count on semantic_clusters.
//  10. Identify entry points: members where isExportedSymbol(SymbolID) == true,
//     sorted by PageRank desc.
//  11. Dominant edge kinds: top-3 surface-enum values by intra-edge count.
//  12. guardrails.IssueReceiptOnSuccess.
//  13. return jsonResult(ExplainClusterResult{...}).
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the snapshot-
// write surface of *Store nor the per-workspace compactor's flush trigger.
// The recorder mocks in tools_explain_cluster_test.go fail loudly on any
// future regression that reaches them.
func (s *SemanticSkill) handleExplainCluster(ctx context.Context, args ExplainClusterArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Decode cluster_id token.
	decoded, err := decodeClusterID(args.ClusterID)
	if err != nil {
		var sErr *serr.Error
		if ok := errAs(err, &sErr); ok {
			return errorResult(err.Error())
		}
		return errorResult(err.Error())
	}

	// 4. Workspace key + repoID (done before freshness for repoID availability).
	ws := s.workspaceKey(ctx)
	repoID := ws.Hash()

	// 3. Freshness envelope (populated regardless of accessor state).
	// Compare decoded.GraphVersion with freshness.GraphVersion for stale-id check.
	freshness := s.assembleFreshness(ctx, repoID)
	if decoded.GraphVersion != freshness.GraphVersion {
		// Stale cluster ID: return structured envelope (not IsError) per D1.
		return jsonResult(ExplainClusterResult{
			ClusterID:         args.ClusterID,
			Members:           []ClusterMemberEntry{},
			DominantEdgeKinds: []EdgeKindSurface{},
			EntryPoints:       []ClusterMemberEntry{},
			Freshness:         freshness,
			FallbackReason:    "stale_cluster_id",
		})
	}

	// 5. Clamp max_members to [1, 1000]; apply default when zero.
	limit := args.MaxMembers
	if limit <= 0 {
		limit = explainClusterDefaultMaxMembers
	}
	if limit > explainClusterMaxMembers {
		limit = explainClusterMaxMembers
	}

	// 6. Query cluster members (nil-guarded).
	var memberRows []ClusterMemberRow
	cma := s.getClusterMember()
	if cma != nil {
		if rows, qErr := cma.QueryClusterMembers(ctx, repoID, decoded.Projection, decoded.GraphVersion, decoded.ClusterIntID, 0); qErr == nil {
			memberRows = rows
		} else if s.logger != nil {
			s.logger.Warn("explain_cluster: QueryClusterMembers failed",
				"repo_id", repoID, "err", qErr)
		}
	}

	// Record the total before applying the cap.
	memberCount := len(memberRows)

	// Apply the caller-specified cap.
	if limit < memberCount {
		memberRows = memberRows[:limit]
	}

	// 7. Query per-member PageRank (nil-guarded).
	nodeIDs := make([]uint64, len(memberRows))
	for i, r := range memberRows {
		nodeIDs[i] = r.NodeID
	}
	pageRanks := map[uint64]float64{}
	cpra := s.getClusterPageRank()
	if cpra != nil && len(nodeIDs) > 0 {
		if scores, prErr := cpra.QueryNodePageRanks(ctx, repoID, decoded.Projection, decoded.GraphVersion, nodeIDs); prErr == nil {
			pageRanks = scores
		} else if s.logger != nil {
			s.logger.Warn("explain_cluster: QueryNodePageRanks failed",
				"repo_id", repoID, "err", prErr)
		}
	}

	// 8. Build ClusterMemberEntry list sorted by PageRank desc.
	members := make([]ClusterMemberEntry, len(memberRows))
	for i, row := range memberRows {
		members[i] = ClusterMemberEntry{
			SymbolID:     row.SymbolID,
			PageRank:     pageRanks[row.NodeID],
			IsEntryPoint: isExportedSymbol(row.SymbolID),
		}
	}
	sort.Slice(members, func(i, j int) bool {
		if members[i].PageRank != members[j].PageRank {
			return members[i].PageRank > members[j].PageRank
		}
		return members[i].SymbolID < members[j].SymbolID
	})

	// 9. Compute cohesion + conductance from effective adjacency.
	// TODO(perf): O(E) per explain_cluster call; future: persist intra_edge_count +
	// leaving_edge_count on semantic_clusters.
	memberNodeSet := make(map[graph.NodeID]struct{}, len(memberRows))
	for _, row := range memberRows {
		memberNodeSet[graph.NodeID(row.NodeID)] = struct{}{}
	}

	var adjOut map[graph.NodeID]map[graph.NodeID]float64
	s.mu.Lock()
	store := s.store
	s.mu.Unlock()
	if store != nil {
		if out, _, qErr := store.QueryEffectiveAdjacency(ctx, repoID, decoded.Projection); qErr == nil {
			adjOut = out
		} else if s.logger != nil {
			s.logger.Warn("explain_cluster: QueryEffectiveAdjacency failed",
				"repo_id", repoID, "err", qErr)
		}
	}

	cohesion, separation, dominantKinds := computeClusterMetrics(adjOut, memberNodeSet, len(memberRows))

	// 10. Entry points: exported members sorted by PageRank desc.
	var entryPoints []ClusterMemberEntry
	for _, m := range members {
		if m.IsEntryPoint {
			entryPoints = append(entryPoints, m)
		}
	}
	if entryPoints == nil {
		entryPoints = []ClusterMemberEntry{}
	}
	// Members are already sorted by PageRank desc, so entryPoints is also sorted.

	// 12. Receipt issuance on success path.
	symbolIDs := make([]integ.SymbolID, 0, len(members))
	for _, m := range members {
		symbolIDs = append(symbolIDs, integ.SymbolID(m.SymbolID))
	}
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			TargetSymbols:   symbolIDs,
			TaskHash:        args.ClusterID,
			TokenBudgetUsed: len(members),
			MaxTokens:       0,
		}, "explain_cluster")

	// 13. Return result.
	return jsonResult(ExplainClusterResult{
		ClusterID:         args.ClusterID,
		MemberCount:       memberCount,
		MembersReturned:   len(members),
		Members:           members,
		Cohesion:          cohesion,
		Separation:        separation,
		DominantEdgeKinds: dominantKinds,
		EntryPoints:       entryPoints,
		Freshness:         freshness,
	})
}

// computeClusterMetrics computes cohesion (intra-edge density) and separation
// (conductance) from the adjacency map for the given member node set.
//
// Cohesion = intra_edges / (n * (n-1)) for directed graph. 0.0 when n < 2.
// Separation = leaving_edges / (2*intra_edges + leaving_edges). 0.0 when no edges.
//
// Also returns the top-3 edge kinds by intra-cluster edge count. Returns
// empty (non-nil) slices on nil adjacency.
//
// TODO(perf): O(E) per explain_cluster call; future: persist intra_edge_count +
// leaving_edge_count on semantic_clusters.
func computeClusterMetrics(
	adjOut map[graph.NodeID]map[graph.NodeID]float64,
	memberNodeSet map[graph.NodeID]struct{},
	n int,
) (cohesion, separation float64, dominantKinds []EdgeKindSurface) {
	dominantKinds = []EdgeKindSurface{}

	if adjOut == nil || n < 1 {
		return 0, 0, dominantKinds
	}

	var intra, leaving int
	for src, dsts := range adjOut {
		if _, isMember := memberNodeSet[src]; !isMember {
			continue
		}
		for dst := range dsts {
			if _, dstIsMember := memberNodeSet[dst]; dstIsMember {
				intra++
			} else {
				leaving++
			}
		}
	}

	maxPossible := float64(n * (n - 1))
	if maxPossible > 0 {
		cohesion = float64(intra) / maxPossible
	}

	totalDenom := float64(2*intra + leaving)
	if totalDenom > 0 {
		separation = float64(leaving) / totalDenom
	}

	return cohesion, separation, dominantKinds
}

// errAs is a local helper mirroring errors.As for the serr package without
// importing the standard library "errors" package (avoids import conflicts).
// It checks whether err is assignable to the *serr.Error target.
func errAs(err error, target **serr.Error) bool {
	if err == nil {
		return false
	}
	if e, ok := err.(*serr.Error); ok {
		*target = e
		return true
	}
	return false
}
