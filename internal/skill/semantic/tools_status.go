package semantic

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
)

// statusHelp is the verbose help text for the get_semantic_graph_status tool.
// Surfaced via get_tool_help and tools/list HelpText.
const statusHelp = `## Usage Examples

Read the current semantic graph status:
  get_semantic_graph_status()

## Parameters
None — this tool takes no arguments.

## Return Shape
- latest_snapshot_id (uint64): The most-recent committed snapshot id, or 0
  when no committed snapshot exists yet.
- graph_version (uint64): Workspace graph_version (bumped by Phase 62
  ApplyRepair on overlay updates).
- overlay_active (bool): True when the live overlay has pending rows.
- pending_lsp_files (int): Total LSP enrichment queue depth across lanes.
- freshness (string): Closed enum (SPEC §26.2):
    * "fresh"                                       — clean state.
    * "stale"                                       — retrieval engine is
                                                       rebuilding (e.g.,
                                                       bleve recovery after
                                                       daemon restart).
    * "structurally_fresh_semantically_pending"     — overlay applied AND
                                                       LSP enrichment still
                                                       pending.
    * "overlay_active"                              — overlay has pending
                                                       rows but no LSP work.
- score_status (map[projection]string): Per-projection score status (closed
  enum from Phase 62 graph.ScoreStatus):
    * "exact"        — row exists at current graph_version.
    * "approximate"  — row exists, flagged approximate by full-recompute.
    * "stale"        — row exists at older graph_version.
    * "missing"      — no row exists.
  Projections surfaced: CALL_GRAPH_PAGERANK, REFERENCE_PAGERANK,
  FILE_DEPENDENCY_PAGERANK.
- cluster_status (object): Structured cluster status. Closed-enum state
  values: "unknown" | "current" | "stale" | "building". Until Phase 65/67
  wires a live cluster source, the production adapter returns
  state="unknown" with reason="phase-62-clustering-no-status-accessor".
- last_live_update_ms (int64): Unix-millis timestamp of the most-recent
  overlay flush, or 0 if no flush has occurred.
- retrieval_pending (bool): True when the retrieval engine (bleve) is still
  rebuilding (e.g., recovery after daemon restart).

## Mode Tier
read+ — every session passes; this is the read-tier inspector for
deciding whether to call refresh_semantic_graph or index_semantic_graph.`

// GetSemanticGraphStatusArgs is the typed-args input schema for
// get_semantic_graph_status. The tool takes no parameters; the empty struct
// is declared for jsonschema-generation consistency with the other Phase 64
// tools and to give get_tool_help a non-nil schema to introspect.
type GetSemanticGraphStatusArgs struct{}

// registerGetSemanticGraphStatus wires get_semantic_graph_status into the MCP
// server with kernel-style typed-args registration + tracing. Daemon (P64-08)
// calls this from semantic_wiring.go after wiring the StoreAccessor /
// SchedulerAccessor / QueueAccessor / LiveAccessor / RetrievalAccessor
// adapters.
func registerGetSemanticGraphStatus(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_semantic_graph_status",
		Description: "Return semantic graph status (read+).",
	}, kernel.WrapToolSpan(tracer, "get_semantic_graph_status",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetSemanticGraphStatusArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleGetSemanticGraphStatus(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "get_semantic_graph_status",
		Description:      "Return semantic graph status (read+).",
		BriefDescription: "Semantic index status",
		HelpText:         statusHelp,
	})
}

// statusProjections is the closed list of projections surfaced in the
// score_status map. Keep in sync with what Phase 62 RankScheduler ships.
var statusProjections = []string{
	"CALL_GRAPH_PAGERANK",
	"REFERENCE_PAGERANK",
	"FILE_DEPENDENCY_PAGERANK",
}

// handleGetSemanticGraphStatus is the testable handler body. Extracted out of
// the registration closure so tests can drive it directly without spinning up
// an MCP server.
//
// Order of operations:
//  1. checkMode(modeTierRead) — every session passes; retained for code-review
//     visibility and Phase 66 GuardrailMiddleware precedent.
//  2. Resolve workspace + repoID (ws.Hash()).
//  3. Fan-out read-only accessor calls. Individual accessor errors are
//     tolerated — the field is zero-valued and the response continues. Status
//     is a pure inspector; surfacing a partial envelope is more useful to the
//     agent than a hard failure.
//  4. Compute closed-enum freshness (SPEC §26.2) from the gathered state.
//  5. Marshal the StatusResult envelope (SPEC §23.3).
//
// HARD INVARIANT: pure read path. The handler MUST NOT call any write surface
// (no Begin/Commit/Abort/Write methods on snapshots, no compactor flush
// trigger, no Drain). The StoreAccessor + SchedulerAccessor + QueueAccessor +
// LiveAccessor interfaces (accessors.go) deliberately omit those methods, so
// this handler is read-only by Go compile-time guarantee.
func (s *SemanticSkill) handleGetSemanticGraphStatus(ctx context.Context, _ GetSemanticGraphStatusArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes; retained for
	//    code-review visibility and the Phase 66 GuardrailMiddleware precedent).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + repoID. ws.Hash() is the canonical repoID seam
	//    consumed by *Store and the scheduler adapter (matches the
	//    refresh/index handlers).
	ws := s.workspaceKey(ctx)
	repoID := ws.Hash()

	// 3. Fan-out accessor reads. Each accessor is independently nil-guarded so
	//    the handler still returns a structured envelope when the daemon has
	//    not wired one of the seams (test path / pre-activation).
	var (
		latestSnapshotID uint64
		graphVersion     uint64
		overlayActive    bool
		pendingLSPFiles  int
		lastLiveUpdateMs int64
		retrievalPending bool
	)

	if s.store != nil {
		// LatestCommittedSnapshot: tolerate error → 0. A missing latest
		// snapshot is a perfectly normal pre-index state.
		if id, err := s.store.LatestCommittedSnapshot(ctx, repoID); err == nil {
			latestSnapshotID = id
		} else if s.logger != nil {
			s.logger.Warn("get_semantic_graph_status: LatestCommittedSnapshot read failed",
				"repo_id", repoID, "err", err)
		}
		// CurrentGraphVersion: tolerate error → 0.
		if gv, err := s.store.CurrentGraphVersion(ctx, repoID); err == nil {
			graphVersion = gv
		} else if s.logger != nil {
			s.logger.Warn("get_semantic_graph_status: CurrentGraphVersion read failed",
				"repo_id", repoID, "err", err)
		}
		overlayActive = s.store.OverlayHasPendingRows(repoID)
	}
	if s.queue != nil {
		pendingLSPFiles = s.queue.DepthAll(ws)
	}
	if s.live != nil {
		lastLiveUpdateMs = s.live.LastFlushAt(ws)
	}

	// score_status fan-out per projection. Hard-coded list of three
	// projections (T-64-06-03: caller cannot inject a projection name).
	scoreStatus := make(map[string]string, len(statusProjections))
	for _, projection := range statusProjections {
		if s.scheduler != nil {
			scoreStatus[projection] = string(s.scheduler.ScoreStatus(repoID, projection))
		} else {
			// No scheduler wired (test path) — surface "missing" as the
			// closed-enum default so the response still type-checks.
			scoreStatus[projection] = string(graphScoreStatusMissing)
		}
	}

	// cluster_status: production adapter returns
	// {State:"unknown", Reason:"phase-62-clustering-no-status-accessor"}
	// until Phase 65/67 wires a live source (closes checker W1). The
	// SchedulerAccessor.ClusterStatus method declared in accessors.go
	// guarantees the response shape is correct from day one.
	var clusterStatus ClusterStatus
	if s.scheduler != nil {
		clusterStatus = s.scheduler.ClusterStatus(repoID)
	} else {
		// No scheduler wired (test path) — mirror the production-adapter
		// default so callers see a stable shape.
		clusterStatus = ClusterStatus{
			State:  "unknown",
			Reason: "phase-62-clustering-no-status-accessor",
		}
	}

	// retrieval_pending: bleve recovery in progress.
	if s.retrieval != nil {
		retrievalPending = s.retrieval.RetrievalPending(ws)
	}

	// 4. Closed-enum freshness selection (SPEC §26.2).
	//
	//    Priority order:
	//      retrievalPending          -> stale (engine rebuilding)
	//      overlayActive && pendingLSPFiles>0 -> structurally_fresh_semantically_pending
	//      overlayActive             -> overlay_active
	//      otherwise                 -> fresh
	//
	//    The retrieval-rebuilding case maps to "stale" rather than a new
	//    enum value because bleve recovery means the retrieval index doesn't
	//    yet reflect the latest committed snapshot — agent-visible behavior
	//    is the same as a stale cache.
	var freshness Freshness
	switch {
	case retrievalPending:
		freshness = FreshnessStale
	case overlayActive && pendingLSPFiles > 0:
		freshness = FreshnessStructurallyFreshSemanticallyPending
	case overlayActive:
		freshness = FreshnessOverlayActive
	default:
		freshness = FreshnessFresh
	}

	// 5. Marshal envelope (SPEC §23.3).
	return jsonResult(StatusResult{
		CommonEnvelope: CommonEnvelope{
			Freshness:     freshness,
			GraphVersion:  graphVersion,
			OverlayActive: overlayActive,
		},
		LatestSnapshotID: latestSnapshotID,
		ScoreStatus:      scoreStatus,
		ClusterStatus:    clusterStatus,
		LastLiveUpdateMs: lastLiveUpdateMs,
		PendingLSPFiles:  pendingLSPFiles,
		RetrievalPending: retrievalPending,
	})
}

// graphScoreStatusMissing is a local copy of graph.ScoreStatusMissing's value
// to avoid an extra import (graph is already imported by accessors.go for the
// SchedulerAccessor.ScoreStatus signature; importing here just for one
// constant would obscure the seam). Source: internal/semantic/graph/status.go.
const graphScoreStatusMissing = "missing"
