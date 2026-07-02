package semantic

// INVARIANT (D-09 / D-13): trace_data_flow MUST NOT touch the snapshot-write
// surface of *Store. Specifically: no Begin/Commit/Abort/Write methods on
// snapshots, and no compactor flush trigger. The grep gate in CI (gatedHandlerFiles
// in readonly_gate_test.go) enforces the absence of those identifier tokens in this
// file. read+ tools MUST still be read-only on graph state — trace_data_flow only
// consumes the existing committed snapshot through the narrow
// DataFlowReachabilityAccessor seam (v2.10).

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
)

// v2.10 surface caps and defaults for trace_data_flow.
const (
	traceDefaultHops = 5
	traceMaxHops     = 10
	traceNodeCap     = 200
)

// TraceDataFlowArgs is the typed-args input schema for trace_data_flow (v2.10).
type TraceDataFlowArgs struct {
	// Seed is the starting PARAMETER symbol — the taint entry point. Either a
	// direct SymbolID or a (file_path, symbol_name) tuple. Post-v2.13, DATA_FLOWS
	// also carries function->param (def_use_inbody) edges, so a FUNCTION seed now
	// mechanically reaches in-body targets; full function-seed support in the verb
	// contract is a fast-follow (Phase 140) — the documented seed stays a parameter.
	Seed SeedInput `json:"seed"`
	// MaxHops bounds the reachability BFS depth. Default 5, max 10.
	MaxHops int `json:"max_hops,omitempty" jsonschema:"reachability BFS depth (default 5, max 10)"`
}

// ReachableSymbol is one symbol reachable from the seed via DATA_FLOWS edges.
type ReachableSymbol struct {
	// SymbolID is the stable graph identity string.
	SymbolID string `json:"symbol_id"`
	// Hops is 0 for the seed itself, 1+ for symbols reached in N data-flow hops.
	Hops int `json:"hops"`
}

// TraceDataFlowResult is the trace_data_flow response shape.
type TraceDataFlowResult struct {
	// Reachable is the capped set of symbols reachable from the seed.
	Reachable []ReachableSymbol `json:"reachable"`
	// NodesCount is the pre-cap total reachable count.
	NodesCount int `json:"nodes_count"`
	// ReachedHops is the effective BFS depth used (the requested, clamped, maxHops).
	ReachedHops int `json:"reached_hops"`
	// FallbackReason is non-empty when the tool degrades (symbol not found,
	// accessor unwired, etc.). Empty on a successful reachability walk —
	// INCLUDING a genuinely-empty reachable set (an honest "nothing flows from
	// this param"), which is NOT a fallback.
	FallbackReason string `json:"fallback_reason,omitempty"`
	Freshness      FreshnessV2 `json:"freshness"`
}

// registerTraceDataFlow wires trace_data_flow into the MCP server. Defined in
// FLOW-02; CALLED from register.go in the FLOW-03 atomic frozen-51 surface bump
// (so the verb does not enter the live registry until the whole surface — verbs_
// gen, reference, anchor, render-class, SKILL matrix, goldens — lands together).
func registerTraceDataFlow(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "trace_data_flow",
		Description: "Source->sink reachability over DATA_FLOWS edges from a seed parameter (read+).",
	}, kernel.WrapToolSpan(tracer, "trace_data_flow",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args TraceDataFlowArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleTraceDataFlow(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "trace_data_flow",
		Description:      "Source->sink reachability over DATA_FLOWS edges from a seed parameter (read+).",
		BriefDescription: "Data-flow reachability",
		HelpText:         traceDataFlowHelp,
	})
}

// handleTraceDataFlow is the testable handler body for trace_data_flow (v2.10).
// Order is load-bearing: (1) read+ mode check; (2) workspace + repoID;
// (3) resolve seed — short-circuit on not_found; (4) clamp maxHops;
// (5) nil-guard DataFlowReachabilityAccessor; (6) ReachableFrom BFS;
// (7) shape + cap; (8) freshness envelope.
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the snapshot-write
// surface of *Store nor the per-workspace compactor's flush trigger.
func (s *SemanticSkill) handleTraceDataFlow(ctx context.Context, args TraceDataFlowArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes; mirrors explain_symbol_deep).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
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
		return jsonResult(TraceDataFlowResult{
			FallbackReason: "symbol_not_found",
			Freshness:      s.assembleFreshness(ctx, repoID),
		})
	}

	// 4. Clamp maxHops to [1, traceMaxHops]; default when zero.
	clampedHops := args.MaxHops
	if clampedHops <= 0 {
		clampedHops = traceDefaultHops
	}
	if clampedHops > traceMaxHops {
		clampedHops = traceMaxHops
	}

	// 5. Nil-guard DataFlowReachabilityAccessor.
	lookup := s.getDataFlowReachability()
	if lookup == nil {
		return jsonResult(TraceDataFlowResult{
			FallbackReason: "data_flow_lookup_unavailable",
			ReachedHops:    clampedHops,
			Freshness:      s.assembleFreshness(ctx, repoID),
		})
	}

	// 6. Reachability BFS over DATA_FLOWS edges from the seed.
	reached, reachErr := lookup.ReachableFrom(ctx, repoID, resolved.SymbolID, clampedHops)
	if reachErr != nil {
		if s.logger != nil {
			s.logger.Warn("trace_data_flow: ReachableFrom failed",
				"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", reachErr)
		}
		return jsonResult(TraceDataFlowResult{
			FallbackReason: "data_flow_lookup_unavailable",
			ReachedHops:    clampedHops,
			Freshness:      s.assembleFreshness(ctx, repoID),
		})
	}

	// 7. Shape + cap (deterministic — ReachableFrom returns sorted by Hops then SymbolID).
	out := make([]ReachableSymbol, 0, len(reached))
	for _, n := range reached {
		if len(out) >= traceNodeCap {
			break
		}
		out = append(out, ReachableSymbol{SymbolID: string(n.SymbolID), Hops: n.Hops})
	}

	// 8. Freshness envelope + result. NOTE: a genuinely-empty reachable set is
	// NOT a fallback — it honestly means "no DATA_FLOWS out of this param."
	return jsonResult(TraceDataFlowResult{
		Reachable:    out,
		NodesCount:   len(reached),
		ReachedHops:  clampedHops,
		Freshness:    s.assembleFreshness(ctx, repoID),
	})
}

// traceDataFlowHelp is the verbose help text registered in the tool registry.
const traceDataFlowHelp = `
## Usage Examples

Trace data-flow reachability from a seed parameter by stable id (default max_hops=5):
  trace_data_flow(seed={symbol_id: "repo/src/svc.go::processRequest::req"})

Trace from a (file_path, symbol_name) tuple to depth 8:
  trace_data_flow(
    seed={file_path: "src/svc.go", symbol_name: "req"},
    max_hops=8
  )

## Output Shape

JSON envelope: { reachable: [{symbol_id, hops}], nodes_count, reached_hops, freshness }.
Each reachable entry carries its hop distance from the seed (0 = the seed itself).
fallback_reason is non-empty ONLY on degradation (symbol_not_found /
data_flow_lookup_unavailable); an empty reachable set is a HONEST "nothing flows
from this param," not a fallback.

## Notes

- Seed semantics: the seed MUST be a PARAMETER (the taint entry point). A function
  seed returns only itself — DATA_FLOWS edges are param-anchored.
- Backed by v2.9's DATA_FLOWS substrate (case-1 caller.param -> callee.param flow).
- read+ tier (every session passes).
`
