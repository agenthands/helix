package semantic

import (
	"context"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/mcp"
)

// tools_context.go — STUB (RED gate). Real implementation lands in Task 3 GREEN.
//
// This file is OWNED by P64-07. It declares:
//   - const contextHelp (the verbose multi-line help text Tools() points to).
//   - GetSemanticContextArgs typed-args struct.
//   - registerGetSemanticContext (kernel-style typed-args + WrapToolSpan).
//   - (s *SemanticSkill).handleGetSemanticContext testable handler body.
//
// Note: the single-line stub `contextHelp = "...stub..."` in skill.go's W0
// stub-var block is deleted by this plan; Tools() now resolves contextHelp to
// the const declared here.

// contextHelp is the (RED-gate) stub text that satisfies the const-decl
// requirement. Task 3 GREEN replaces this with the verbose multi-line help
// text covering Usage Examples / Parameters / Return Shape / Mode Tier.
const contextHelp = "get_semantic_context: stub help (RED gate; full multi-line text lands in GREEN)"

// GetSemanticContextArgs is the typed-args input schema for
// get_semantic_context. Field tags (json + jsonschema) feed the MCP SDK's
// schema generator; get_tool_help also reads the jsonschema tag for parameter
// docs (per internal/kernel/help/help.go:21-67).
type GetSemanticContextArgs struct {
	Task          string   `json:"task,omitempty"           jsonschema:"natural-language task description"`
	Files         []string `json:"files,omitempty"          jsonschema:"anchor files for personalized PageRank"`
	Symbols       []string `json:"symbols,omitempty"        jsonschema:"anchor symbol IDs"`
	MaxTokens     int      `json:"max_tokens,omitempty"     jsonschema:"token budget (default 2048; clamped to [64, 32768])"`
	FreshnessMode string   `json:"freshness_mode,omitempty" jsonschema:"allow_stale | require_current | validate_live"`
}

// registerGetSemanticContext wires get_semantic_context into the MCP server
// with kernel-style typed-args registration + tracing.
//
// STUB body: panics until Task 3 GREEN supplies the real implementation.
func registerGetSemanticContext(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	panic("registerGetSemanticContext: not implemented (RED gate — Task 3 GREEN fills this in)")
}

// handleGetSemanticContext is the testable handler body. STUB — panics.
//
// Task 3 GREEN replaces the panic with the real handler that:
//   1. checkMode(snap, modeTierRead) — every session passes.
//   2. validatePaths(args.Files, ws.RepoRoot).
//   3. clamp args.MaxTokens to [64, 32768], default 2048.
//   4. resolve FreshnessMode (default allow_stale).
//   5. read RetrievalPending; if true, return stale envelope with
//      RetrievalPending=true / Candidates=nil immediately.
//   6. fan out QueryBleve + PersonalizedPageRank; fuse via rrf.Fuse.
//   7. greedyPack candidates under the token budget.
//   8. populate ContextEvidence per candidate (TextRank, GraphRank,
//      MatchedTerms, TopEdges capped at retrieval.TopEdgesPerCandidate=5).
//   9. emit ContextResult with closed-enum Freshness from
//      computeFreshness(overlayActive, pendingLSP>0, retrievalPending).
func (s *SemanticSkill) handleGetSemanticContext(ctx context.Context, args GetSemanticContextArgs) *mcpsdk.CallToolResult {
	panic("handleGetSemanticContext: not implemented (RED gate — Task 3 GREEN fills this in)")
}
