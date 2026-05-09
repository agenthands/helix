package semantic

import (
	"context"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
)

// INVARIANT (D-09 / D-13): refresh_semantic_graph MUST NOT touch the
// snapshot-write surface of *Store nor trigger the per-workspace compactor.
// Specifically: no Begin/Commit/Abort/Write methods on snapshots, and no
// compactor flush trigger. The grep gate in CI enforces the absence of those
// identifier tokens in this file; the recorder mocks in
// tools_refresh_test.go enforce it under unit test. Read+ stays read-only
// with respect to committed state — refresh only drains the live pipeline
// into the overlay; producing a new committed snapshot is the review+/admin
// tier's exclusive responsibility (index_semantic_graph).

// RefreshSemanticGraphArgs is the typed-args input schema for
// refresh_semantic_graph. Field tags (json + jsonschema) feed the MCP SDK's
// schema generator; get_tool_help also reads the jsonschema tag for parameter
// docs (per internal/kernel/help/help.go:21-67).
type RefreshSemanticGraphArgs struct {
	Paths      []string `json:"paths,omitempty"           jsonschema:"strict-subset path filter (D-11): only requested paths are processed; remainder remain queued"`
	WaitForLSP bool     `json:"wait_for_lsp,omitempty"    jsonschema:"block on LSP enrichment queue draining (cap = max_wait_ms)"`
	MaxWaitMs  int      `json:"max_wait_ms,omitempty"     jsonschema:"foreground cap for wait_for_lsp polling in ms (default 3000)"`
}

// refreshHelp is the verbose help text for the refresh_semantic_graph tool.
// Surfaced via get_tool_help and tools/list HelpText.
const refreshHelp = `## Usage Examples

Drain pending live changes (no LSP wait):
  refresh_semantic_graph()

Drain only specific paths (strict subset — others remain queued):
  refresh_semantic_graph(paths=["src/auth", "src/db/conn.go"])

Drain and wait for LSP enrichment (default 3s cap):
  refresh_semantic_graph(wait_for_lsp=true)

Drain with custom LSP wait cap (1.5s):
  refresh_semantic_graph(wait_for_lsp=true, max_wait_ms=1500)

## Parameters
- paths ([]string, optional): STRICT-SUBSET path filter (D-11). When
  provided, refresh processes ONLY those paths from the live queue;
  other queued changes remain pending until the next refresh call (or
  the natural Phase 60 coalescer flush). Empty/absent = drain all
  queued paths. Absolute paths must live inside the workspace root;
  ".." is rejected.
- wait_for_lsp (bool, optional): When true, block up to max_wait_ms
  for the LSP enrichment queue to drain after the structural overlay
  flush. On timeout, response carries pending_lsp=true with the
  remaining queue depth. Default false.
- max_wait_ms (int, optional): Foreground cap for wait_for_lsp polling
  in milliseconds. Default 3000 (3s); ignored when wait_for_lsp=false.

## Return Shape
- files_updated (int): Count of paths drained in this call.
- pending_lsp (bool): True when wait_for_lsp timed out before drain.
- pending_lsp_files (int): Remaining LSP queue depth at return time.
- graph_version (uint64): Workspace graph_version at return time.
- overlay_active (bool): True when the live overlay has pending rows.
- freshness (string): "fresh" if all LSP enrichment drained,
  "structurally_fresh_semantically_pending" if structural overlay
  drained but LSP timed out, "overlay_active" otherwise.

## Mode Tier
read+ — every session passes; this is the read-tier counterpart to
index_semantic_graph (review+/admin).

## Boundary (D-09 / D-13)
NEVER produces a new committed snapshot and NEVER triggers Phase 63
compaction. Use index_semantic_graph(mode="incremental") to commit a
new snapshot.`

// registerRefreshSemanticGraph wires refresh_semantic_graph into the MCP
// server with kernel-style typed-args registration + tracing. Daemon (P64-08)
// calls this from semantic_wiring.go after wiring the LiveAccessor /
// QueueAccessor / StoreAccessor adapters.
func registerRefreshSemanticGraph(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "refresh_semantic_graph",
		Description: "Apply pending live source changes (read+).",
	}, kernel.WrapToolSpan(tracer, "refresh_semantic_graph",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args RefreshSemanticGraphArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleRefreshSemanticGraph(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "refresh_semantic_graph",
		Description:      "Apply pending live source changes (read+).",
		BriefDescription: "Refresh live overlay",
		HelpText:         refreshHelp,
	})
}

// handleRefreshSemanticGraph is the testable handler body. Extracted out of
// the registration closure so tests can drive it directly without spinning up
// an MCP server.
//
// Order of operations is load-bearing:
//  1. checkMode(modeTierRead) FIRST — every session passes, but the call is
//     retained as a precedent for Phase 66 GuardrailMiddleware and to make
//     the gating discipline visible for code review.
//  2. validatePaths — reject path traversal + absolute paths outside the
//     workspace root (T-64-05-01).
//  3. Drain pending changes via LiveAccessor.OnWorkspaceChanged (D-11
//     strict-subset semantics: when args.Paths is non-empty, only those
//     paths flow through).
//  4. Read graph_version + overlay_active for the response envelope.
//  5. If args.WaitForLSP, poll QueueAccessor.DepthAll until either depth
//     hits 0 OR max_wait_ms (default 3000) elapses. On timeout, the
//     response carries pending_lsp=true (D-12).
//  6. Pick the freshness label (D-09): fresh if no overlay + no pending
//     LSP, structurally_fresh_semantically_pending if LSP timed out,
//     overlay_active if overlay still has pending rows.
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the
// snapshot-write surface of *Store (the four Begin/Commit/Abort/Write
// methods on snapshots) NOR the per-workspace compactor's flush trigger.
// The StoreAccessor interface (accessors.go) deliberately omits the
// snapshot-write surface; the recorder mocks in tools_refresh_test.go
// fail loudly if a future change ever reaches them via type assertion.
func (s *SemanticSkill) handleRefreshSemanticGraph(ctx context.Context, args RefreshSemanticGraphArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes; retained for the
	//    Phase 66 GuardrailMiddleware precedent and code-review visibility).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + path-traversal validation (T-64-05-01).
	ws := s.workspaceKey(ctx)
	if err := validatePaths(args.Paths, ws.RepoRoot); err != nil {
		return errorResult(err.Error())
	}

	// 3. Default the LSP-wait cap.
	maxWaitMs := args.MaxWaitMs
	if maxWaitMs <= 0 {
		maxWaitMs = 3000
	}

	// 4. Drain pending live changes via LiveAccessor (D-11).
	//
	//    The accessor signature accepts the request paths verbatim. Daemon
	//    adapter translates to live.Service.OnWorkspaceChanged with a
	//    WorkspaceChangeSignal; from the handler's perspective the strict-
	//    subset filter is just the args.Paths slice.
	if s.live != nil {
		if err := s.live.OnWorkspaceChanged(ws, args.Paths); err != nil {
			return errorResult(err.Error())
		}
	}

	// 5. Read graph_version (post-drain so ApplyRepair's bump is observed).
	var graphVersion uint64
	if s.store != nil {
		if gv, err := s.store.CurrentGraphVersion(ctx, ws.Hash()); err == nil {
			graphVersion = gv
		}
	}

	// files_updated semantics: when args.Paths is supplied, the strict-
	// subset count is exactly len(args.Paths). When empty, the live drain
	// processes whatever the coalescer flushes — without a return-value
	// signal from the accessor we report 0 (the SPEC §23.2 envelope is
	// best-effort here; consumers asking about specific files supply the
	// paths argument).
	filesUpdated := len(args.Paths)

	// 6. Optional LSP-wait poll (D-12).
	pendingLSPFiles := 0
	if s.queue != nil {
		pendingLSPFiles = s.queue.DepthAll(ws)
	}
	pendingLSP := false

	if args.WaitForLSP && s.queue != nil {
		deadline := time.Now().Add(time.Duration(maxWaitMs) * time.Millisecond)
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		for {
			pendingLSPFiles = s.queue.DepthAll(ws)
			if pendingLSPFiles == 0 {
				break
			}
			if !time.Now().Before(deadline) {
				break
			}
			select {
			case <-ticker.C:
				// Continue polling.
			case <-ctx.Done():
				// Request canceled; stop polling and report current depth.
				break
			}
			if ctx.Err() != nil {
				break
			}
		}
		pendingLSPFiles = s.queue.DepthAll(ws)
		pendingLSP = pendingLSPFiles > 0
	}

	// 7. Freshness selection (closed enum; D-09 boundaries).
	overlayActive := false
	if s.store != nil {
		overlayActive = s.store.OverlayHasPendingRows(ws.Hash())
	}

	freshness := FreshnessFresh
	switch {
	case overlayActive:
		freshness = FreshnessOverlayActive
	case pendingLSP:
		freshness = FreshnessStructurallyFreshSemanticallyPending
	}

	return jsonResult(RefreshResult{
		CommonEnvelope: CommonEnvelope{
			Freshness:     freshness,
			GraphVersion:  graphVersion,
			OverlayActive: overlayActive,
		},
		FilesUpdated:    filesUpdated,
		PendingLSP:      pendingLSP,
		PendingLSPFiles: pendingLSPFiles,
	})
}
