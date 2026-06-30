package semantic

// INVARIANT (D-09 / D-13): get_change_impact_graph MUST NOT touch the snapshot-
// write surface of *Store. Specifically: no Begin/Commit/Abort/Write methods on
// snapshots, and no compactor flush trigger. The grep gate in CI enforces the
// absence of those identifier tokens in this file; the recorder mocks in
// tools_change_impact_test.go enforce it under unit test. Review+ tools MUST
// still be read-only on graph state — get_change_impact_graph only consumes the
// existing committed snapshot through the narrow ImpactLookupAccessor seam.

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
)

// D3 surface caps and defaults for get_change_impact_graph (CONTEXT.md D3 + D4).
const (
	impactDefaultDepth = 2
	impactMaxDepth     = 5
	impactNodeCap      = 200
	impactEdgeCap      = 500
)

// GetChangeImpactGraphArgs is the typed-args input schema for get_change_impact_graph.
type GetChangeImpactGraphArgs struct {
	// Seed is the starting symbol: either a direct SymbolID, or a
	// (file_path, symbol_name) tuple (Phase 71 D1 shape).
	Seed SeedInput `json:"seed"`
	// MaxDepth controls BFS expansion depth. Default 2, max 5.
	MaxDepth int `json:"max_depth,omitempty" jsonschema:"BFS expansion depth (default 2, max 5)"`
	// EdgeKinds is an optional filter using the Phase 71 D3 surface enum.
	// When empty all edge kinds are returned.
	EdgeKinds []string `json:"edge_kinds,omitempty" jsonschema:"optional edge kind filter (MCP surface enum values)"`
}

// ImpactNode is one node in the get_change_impact_graph response.
type ImpactNode struct {
	// SymbolID is the stable graph identity string.
	SymbolID string `json:"symbol_id"`
	// QualifiedName is the human-readable qualified name (empty when unavailable).
	QualifiedName string `json:"qualified_name,omitempty"`
	// Package is the package/module path (empty when unavailable).
	Package string `json:"package,omitempty"`
	// PageRank is the per-node PageRank score; 0.0 when not available from ExpandFrom.
	PageRank float64 `json:"pagerank,omitempty"`
}

// ImpactEdge is one directed edge in the get_change_impact_graph response.
type ImpactEdge struct {
	// From is the source symbol_id.
	From string `json:"from"`
	// To is the target symbol_id.
	To string `json:"to"`
	// EdgeKind is the closed MCP-surface enum value.
	EdgeKind EdgeKindSurface `json:"edge_kind"`
	// InternalKind is the raw extractor/resolver kind string (e.g. "CALLS").
	InternalKind string `json:"internal_kind"`
	// Confidence is the per-edge confidence score (may be capped by capEdgeConfidences).
	Confidence float64 `json:"confidence"`
}

// ConfidenceCap carries the envelope-level confidence degradation signal emitted
// when the OQ-1 predicate fires (any impact.Confidence < 0.8 → type_resolver_tier_3).
type ConfidenceCap struct {
	// Value is the cap ceiling applied to all per-edge Confidence values.
	Value float64 `json:"value"`
	// Reason is the closed-enum degradation reason.
	// "type_resolver_tier_3" is the Phase 72 addition to the reason set.
	Reason string `json:"reason"`
}

// GetChangeImpactGraphResult is the get_change_impact_graph response shape.
type GetChangeImpactGraphResult struct {
	// Nodes is the list of impact nodes (capped at impactNodeCap).
	Nodes []ImpactNode `json:"nodes"`
	// Edges is the list of impact edges (capped at impactEdgeCap).
	Edges []ImpactEdge `json:"edges"`
	// Truncated is true when nodes or edges were capped.
	Truncated bool `json:"truncated"`
	// ReachedDepth is the effective BFS depth used.
	// Pitfall 4: ExpandFrom does not expose per-impact depth; ReachedDepth = requested depth.
	ReachedDepth int `json:"reached_depth"`
	// NodesCount is the pre-cap total impact count.
	NodesCount int `json:"nodes_count"`
	// EdgesCount is the pre-cap total edge count (sum of all evidence edges).
	EdgesCount int `json:"edges_count"`
	// ConfidenceCap is non-nil when the OQ-1 predicate fired (any impact.Confidence < 0.8).
	ConfidenceCap *ConfidenceCap `json:"confidence_cap,omitempty"`
	// Freshness is the FreshnessV2 envelope.
	Freshness FreshnessV2 `json:"freshness"`
	// FallbackReason is non-empty when the response is degraded.
	// Closed enum: "symbol_not_found" | "impact_lookup_unavailable".
	FallbackReason string `json:"fallback_reason,omitempty"`
}

// getChangeImpactGraphHelp is the verbose help text registered in the tool registry.
const getChangeImpactGraphHelp = `
## Usage Examples

Expand impact from a seed by stable id (default depth=2):
  get_change_impact_graph(seed={symbol_id: "repo/src/svc.go::ServeHTTP"})

Expand from a (file_path, symbol_name) tuple:
  get_change_impact_graph(seed={file_path: "src/svc.go", symbol_name: "ServeHTTP"})

Expand to depth 4 and filter to call edges only:
  get_change_impact_graph(
    seed={symbol_id: "repo/src/svc.go::ServeHTTP"},
    max_depth=4,
    edge_kinds=["calls"],
  )

## Parameters
- seed (object, required): Starting symbol. One of two forms —
    * { symbol_id: <stable graph id> } — direct lookup, no name resolution.
    * { file_path, symbol_name } — name-resolved lookup.
- max_depth (int, optional): BFS expansion depth. Default 2, max 5.
  Larger depths surface more transitive impact but increase response size.
- edge_kinds ([]string, optional): Optional filter restricting traversal to the
  specified closed-enum edge kinds (calls / references / implements / extends /
  has_type / uses_type / contains / other). When empty all edge kinds are
  returned.

## Return Shape
- nodes ([]ImpactNode): List of impacted symbol nodes (capped at 200). Each
  entry carries: symbol_id (string), qualified_name (string, optional),
  package (string, optional), pagerank (float64, 0.0 when unavailable).
- edges ([]ImpactEdge): Directed edges in the subgraph (capped at 500). Each
  entry carries: from (string), to (string), edge_kind (EdgeKindSurface),
  internal_kind (string), confidence (float64).
- truncated (bool): true when nodes or edges were capped at their respective
  limits (200 nodes / 500 edges).
- reached_depth (int): Effective BFS depth used (equals the clamped max_depth).
- nodes_count (int): Pre-cap total impact node count.
- edges_count (int): Pre-cap total edge count (sum across all evidence edges).
- confidence_cap (object, optional): Non-nil when the OQ-1 predicate fired
  (any impact.Confidence < 0.8). Carries: value (float64 = 0.6),
  reason (string = "type_resolver_tier_3").
- freshness (FreshnessV2): { graph_version, snapshot_id, extractor_run_id,
  as_of_unix_ms, status ∈ {current|stale|unknown}, source }.
- fallback_reason (string, optional): Closed enum — "symbol_not_found" |
  "impact_lookup_unavailable".

## Mode Tier
review+ — call switch_mode(target_mode="review") or "admin" to elevate.`

// registerGetChangeImpactGraph wires get_change_impact_graph into the MCP server.
func registerGetChangeImpactGraph(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_change_impact_graph",
		Description: "Pre-edit blast-radius subgraph (nodes + edges + edge kinds) for a seed symbol (review+).",
	}, kernel.WrapToolSpan(tracer, "get_change_impact_graph",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetChangeImpactGraphArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleGetChangeImpactGraph(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "get_change_impact_graph",
		Description:      "Pre-edit blast-radius subgraph (nodes + edges + edge kinds) for a seed symbol (review+).",
		BriefDescription: "Change impact subgraph",
		HelpText:         getChangeImpactGraphHelp,
	})
}

// handleGetChangeImpactGraph is the testable handler body for get_change_impact_graph.
// Step order is load-bearing:
//  1. checkMode(snap, modeTierReview) — MUST be first; returns PermissionDenied when mode < review.
//  2. Workspace key + repoID.
//  3. resolveSeed(ctx, ws, args.Seed) — short-circuit on not_found.
//  4. Clamp maxDepth to [1, impactMaxDepth]; apply impactDefaultDepth when 0.
//  5. Nil-guard ImpactLookupAccessor; degrade gracefully when nil.
//  6. s.getImpactLookup().ExpandFrom(ctx, ws, sym, clampedDepth) → []integ.Impact.
//  7. Filter by args.EdgeKinds using surfaceToInternalKinds (same package, Pitfall 5).
//  8. NodesCount = len(impacts) before cap; EdgesCount = total edges before cap.
//  9. Cap nodes at impactNodeCap, edges at impactEdgeCap; set Truncated.
//
// 10. OQ-1 predicate: if any impact.Confidence < 0.8 → capEdgeConfidences(edges, 0.6);
//
//	set ConfidenceCap{Value: 0.6, Reason: "type_resolver_tier_3"}.
//
// 11. Assemble ImpactNode + ImpactEdge slices using MapInternalKind for EdgeKind field.
// 12. assembleFreshness for FreshnessV2 envelope.
// 13. guardrails.IssueReceiptOnSuccess.
// 14. return jsonResult(GetChangeImpactGraphResult{...}).
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the snapshot-write
// surface of *Store nor the per-workspace compactor's flush trigger.
func (s *SemanticSkill) handleGetChangeImpactGraph(ctx context.Context, args GetChangeImpactGraphArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (review+ — rejects sessions with mode < "review").
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierReview); err != nil {
		return errorResult(err.Error())
	}

	// 2. Workspace key + repoID.
	ws := s.workspaceKey(ctx)
	repoID := ws.Hash()

	// 3. Resolve seed.
	resolved, err := s.resolveSeed(ctx, ws, args.Seed)
	if err != nil {
		return errorResult(err.Error())
	}
	if resolved.Resolution == ResolutionNotFound {
		return jsonResult(GetChangeImpactGraphResult{
			FallbackReason: "symbol_not_found",
			Freshness:      s.assembleFreshness(ctx, repoID),
		})
	}

	// 4. Clamp maxDepth to [1, impactMaxDepth]; default when zero.
	clampedDepth := args.MaxDepth
	if clampedDepth <= 0 {
		clampedDepth = impactDefaultDepth
	}
	if clampedDepth > impactMaxDepth {
		clampedDepth = impactMaxDepth
	}

	// 5. Nil-guard ImpactLookupAccessor.
	lookup := s.getImpactLookup()
	if lookup == nil {
		return jsonResult(GetChangeImpactGraphResult{
			FallbackReason: "impact_lookup_unavailable",
			Freshness:      s.assembleFreshness(ctx, repoID),
		})
	}

	// 6. Expand from seed.
	impacts, expandErr := lookup.ExpandFrom(ctx, ws, resolved.SymbolID, clampedDepth)
	if expandErr != nil {
		if s.logger != nil {
			s.logger.Warn("get_change_impact_graph: ExpandFrom failed",
				"repo_id", repoID, "err", expandErr)
		}
		return jsonResult(GetChangeImpactGraphResult{
			FallbackReason: "impact_lookup_unavailable",
			Freshness:      s.assembleFreshness(ctx, repoID),
		})
	}

	// 7. Build edge kind allow-set from args.EdgeKinds filter.
	// Uses surfaceToInternalKinds (Pitfall 5: same package, no copy needed).
	var allowedInternalKinds map[string]struct{}
	if len(args.EdgeKinds) > 0 {
		allowedInternalKinds = make(map[string]struct{})
		for _, k := range args.EdgeKinds {
			for _, internalKind := range surfaceToInternalKinds(EdgeKindSurface(k)) {
				allowedInternalKinds[internalKind] = struct{}{}
			}
		}
	}

	// 8. Count pre-cap totals.
	nodesCount := len(impacts)
	edgesCount := 0
	for _, imp := range impacts {
		for _, e := range imp.Evidence.Edges {
			// Count only edges that pass the kind filter.
			if allowedInternalKinds == nil {
				edgesCount++
			} else if _, ok := allowedInternalKinds[e.Kind]; ok {
				edgesCount++
			}
		}
	}

	// 9. Apply node cap; collect edges respecting both node-cap and edge-cap.
	truncated := nodesCount > impactNodeCap || edgesCount > impactEdgeCap
	cappedImpacts := impacts
	if len(cappedImpacts) > impactNodeCap {
		cappedImpacts = cappedImpacts[:impactNodeCap]
	}

	// Collect raw edges from capped nodes, applying the kind filter and edge cap.
	rawEdges := make([]ImpactEdge, 0, edgesCount)
	for _, imp := range cappedImpacts {
		for _, e := range imp.Evidence.Edges {
			if allowedInternalKinds != nil {
				if _, ok := allowedInternalKinds[e.Kind]; !ok {
					continue
				}
			}
			rawEdges = append(rawEdges, ImpactEdge{
				From:         string(e.From),
				To:           string(e.To),
				EdgeKind:     MapInternalKind(e.Kind),
				InternalKind: e.Kind,
				Confidence:   e.Confidence,
			})
			if len(rawEdges) >= impactEdgeCap {
				truncated = true
				break
			}
		}
		if len(rawEdges) >= impactEdgeCap {
			break
		}
	}

	// 10. OQ-1 confidence cap predicate: any impact.Confidence < 0.8 → cap edges to 0.6.
	var confCap *ConfidenceCap
	for _, imp := range impacts {
		if imp.Confidence < 0.8 {
			confCap = &ConfidenceCap{Value: 0.6, Reason: "type_resolver_tier_3"}
			capEdgeConfidences(rawEdges, 0.6)
			break
		}
	}

	// 11. Assemble ImpactNode slice from capped impacts.
	nodes := make([]ImpactNode, 0, len(cappedImpacts))
	for _, imp := range cappedImpacts {
		nodes = append(nodes, ImpactNode{
			SymbolID: string(imp.SymbolID),
			// PageRank: 0.0 — ExpandFrom does not expose per-impact PageRank.
			// Pitfall 4: ExpandFrom does not expose per-impact depth or PageRank;
			// future improvement: wire ClusterPageRankAccessor here.
		})
	}

	// 12. assembleFreshness for FreshnessV2 envelope.
	freshness := s.assembleFreshness(ctx, repoID)

	// 13. Receipt issuance on success path.
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			TargetSymbols:   []integ.SymbolID{resolved.SymbolID},
			TaskHash:        repoID,
			TokenBudgetUsed: nodesCount,
			MaxTokens:       0,
		}, "get_change_impact_graph")

	// 14. Return the subgraph result.
	return jsonResult(GetChangeImpactGraphResult{
		Nodes:         nodes,
		Edges:         rawEdges,
		Truncated:     truncated,
		ReachedDepth:  clampedDepth,
		NodesCount:    nodesCount,
		EdgesCount:    edgesCount,
		ConfidenceCap: confCap,
		Freshness:     freshness,
	})
}

// capEdgeConfidences clamps all edge Confidence values to ≤ cap.
// Mutates the slice in place (locally owned, no copy needed).
// Local helper — NOT imported from internal/kernel/symbols/blast_radius_strangler.go
// (vet-nokernel2semantic boundary enforced per plan; see RESEARCH.md Pitfall 1).
func capEdgeConfidences(edges []ImpactEdge, cap float64) {
	for i := range edges {
		if edges[i].Confidence > cap {
			edges[i].Confidence = cap
		}
	}
}
