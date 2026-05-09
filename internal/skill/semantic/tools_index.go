package semantic

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
)

// indexHelp is the verbose help text for the index_semantic_graph tool.
// Surfaced via get_tool_help and tools/list HelpText.
const indexHelp = `## Usage Examples

Build a fresh full snapshot:
  index_semantic_graph(mode="full")

Auto-resolve (default — full if no committed snapshot, else incremental):
  index_semantic_graph()
  index_semantic_graph(mode="auto")

Refresh just the live overlay before committing:
  index_semantic_graph(mode="refresh")

Cap the foreground wait at 5 seconds (background build keeps running):
  index_semantic_graph(mode="full", max_duration_ms=5000)

Limit indexing to a subset of paths:
  index_semantic_graph(paths=["src/auth", "src/db"])

## Parameters
- mode (string, optional): "auto" | "full" | "incremental" | "refresh".
  "auto" resolves to "full" when no committed snapshot exists, else
  "incremental". Default is "auto".
- max_duration_ms (int, optional): Per-call foreground timeout in
  milliseconds. Default 120000 (120s). When the timeout fires before the
  build commits, the call returns partial=true / status=building and the
  background build keeps running — observe its eventual commit via
  get_semantic_graph_status.
- paths ([]string, optional): Restrict indexing to a subset of workspace
  paths (relative to workspace root). Absolute paths must live inside the
  workspace root; ".." is rejected.

## Return Shape
- snapshot_id (uint64): The committed (or in-flight) snapshot identifier.
- status (string): "committed" | "building" | "failed".
- partial (bool): true when the foreground timeout fired before commit.
- files_indexed (int64): Files processed in this build.
- files_reused (int64): Files reused from prior snapshots (incremental).
- duration_ms (int64): Wall-clock duration of the foreground call.
- graph_version (uint64): Workspace graph_version at completion.
- overlay_active (bool): True when the live overlay has pending rows.
- freshness (string): "fresh" | "stale" | "structurally_fresh_semantically_pending" |
  "overlay_active".

## Mode Tier
review+ — call switch_mode(target_mode="review") or "admin" to elevate.

## Concurrency
Concurrent callers asking for the same resolved mode JOIN the same
in-flight build (singleflight); they all observe the same snapshot_id.`

// IndexSemanticGraphArgs is the typed-args input schema for
// index_semantic_graph. Field tags (json + jsonschema) feed the MCP SDK's
// schema generator; get_tool_help also reads the jsonschema tag for parameter
// docs (per internal/kernel/help/help.go:21-67).
type IndexSemanticGraphArgs struct {
	Mode          string   `json:"mode,omitempty"            jsonschema:"auto, full, incremental, or refresh"`
	MaxDurationMs int      `json:"max_duration_ms,omitempty" jsonschema:"per-call timeout in ms (default 120000)"`
	Paths         []string `json:"paths,omitempty"           jsonschema:"optional path filter (relative to workspace root)"`
}

// registerIndexSemanticGraph wires index_semantic_graph into the MCP server
// with kernel-style typed-args registration + tracing. Daemon (P64-08) calls
// this from semantic_wiring.go after constructing the IndexRunner adapter.
func registerIndexSemanticGraph(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "index_semantic_graph",
		Description: "Build or refresh a committed semantic snapshot.",
	}, kernel.WrapToolSpan(tracer, "index_semantic_graph",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args IndexSemanticGraphArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleIndexSemanticGraph(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "index_semantic_graph",
		Description:      "Build or refresh a committed semantic snapshot.",
		BriefDescription: "Index the semantic graph",
		HelpText:         indexHelp,
	})
}

// handleIndexSemanticGraph is the testable handler body. Extracted out of the
// registration closure so tests can drive it directly without spinning up an
// MCP server.
//
// Order of operations is load-bearing:
//  1. checkMode(modeTierReview) FIRST — denies read/edit sessions with a
//     structured PermissionDenied envelope (T-64-04-01).
//  2. validatePaths — reject path traversal + absolute paths outside the
//     workspace root (T-64-04-02).
//  3. Resolve mode="auto" / "" via runner.ResolveAuto BEFORE invoking Run
//     (closes checker B5 + T-64-04-07: the singleflight key in Run is keyed
//     on the RESOLVED mode; auto callers must convert here so concurrent
//     auto callers join the same build).
//  4. runner.Run dispatches the (possibly singleflight-joined) build with the
//     resolved mode + per-call timeout.
func (s *SemanticSkill) handleIndexSemanticGraph(ctx context.Context, args IndexSemanticGraphArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check FIRST (T-64-04-01: prevent read/edit-mode bypass).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierReview); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + path-traversal validation (T-64-04-02).
	ws := s.workspaceKey(ctx)
	if err := validatePaths(args.Paths, ws.RepoRoot); err != nil {
		return errorResult(err.Error())
	}

	// 3. Resolve mode=auto BEFORE Run (closes checker B5 + T-64-04-07).
	mode := args.Mode
	if mode == "" || mode == "auto" {
		mode = s.runner.ResolveAuto(ctx, ws)
	}

	// 4. Singleflight-joined dispatch (D-02 + D-04).
	result, err := s.runner.Run(ctx, ws, mode, args.MaxDurationMs)
	if err != nil {
		return errorResult(err.Error())
	}
	return jsonResult(result)
}
