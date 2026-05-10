package semantic

import (
	"context"
	"crypto/sha256"
	"fmt"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/retrieval"
)

// tools_context.go is OWNED by P64-07. It declares:
//   - const contextHelp (the verbose multi-line help text Tools() points to).
//   - GetSemanticContextArgs typed-args struct.
//   - registerGetSemanticContext (kernel-style typed-args + WrapToolSpan).
//   - (s *SemanticSkill).handleGetSemanticContext testable handler body.
//   - greedyPack token-budget packer (inline; not in handler_helpers.go which
//     is owned by P64-04).
//
// Note: the W0 stub `contextHelp = "...stub..."` in skill.go's stub-var block
// was deleted by this plan (Task 3 RED commit); Tools() now resolves
// contextHelp to the const declared here.

// contextHelp is the verbose help text for the get_semantic_context tool.
// Surfaced via get_tool_help and tools/list HelpText.
const contextHelp = `## Usage Examples

Retrieve ranked context for a natural-language task:
  get_semantic_context(task="implement OAuth refresh-token rotation")

Anchor the retrieval to specific files / symbols:
  get_semantic_context(
    task="add idempotency to billing job",
    files=["src/billing/processor.go", "src/billing/job.go"],
    symbols=["BillingProcessor.Run", "Job.Idempotent"]
  )

Tighter budget for a quick exploratory query:
  get_semantic_context(task="locate JWT verification", max_tokens=512)

Require fresh state (block on overlay/LSP drain):
  get_semantic_context(task="validate auth flow", freshness_mode="require_current")

## Parameters
- task (string, optional): Natural-language task description. Drives the
  bleve full-text query against the indexed corpus (D-06 corpus = name +
  docstring + path + ~5-line comment window).
- files ([]string, optional): Anchor file paths (relative to workspace
  root). Seed personalized PageRank surface and bias bleve scoring so
  task-relevant code in the anchored neighborhood ranks higher. Absolute
  paths must live inside the workspace root; ".." is rejected.
- symbols ([]string, optional): Anchor symbol IDs. Same role as files —
  PageRank seed + bleve doc-id boost.
- max_tokens (int, optional): Token budget. Default 2048; clamped to
  [64, 32768]. Greedy packing under budget order = score desc.
- freshness_mode (string, optional): Closed enum
  "allow_stale" | "require_current" | "validate_live". Default
  "allow_stale".

## Return Shape
- candidates ([]ContextCandidate): Ranked candidates under the token
  budget. Each candidate carries:
    * symbol_id (string)
    * confidence (float, [0,1])
    * evidence:
        * text_rank  (int, 1-based; 0 if absent)
        * graph_rank (int, 1-based; 0 if absent)
        * matched_terms ([]string, bleve hit terms)
        * top_edges ([]string, capped at 5)
- graph_version (uint64)
- overlay_active (bool)
- freshness (closed enum, SPEC §26.2)
- freshness_mode (closed enum)
- pending_lsp_files (int)
- retrieval_pending (bool): True when bleve recovery is rebuilding the
  retrieval index. When true, candidates is empty + freshness=stale.

## Mode Tier
read+ — every session passes; this is the read-tier semantic retrieval
front door for agents.

## Determinism
Same (task, files, symbols, max_tokens) inputs produce byte-identical
candidate ordering across runs (Phase 62 sort-before-iterate doctrine,
asserted by TestContextHandler_Determinism_10Runs).`

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
// with kernel-style typed-args registration + tracing. Daemon (P64-08) calls
// this from semantic_wiring.go after constructing the RetrievalAccessor
// adapter (which wraps a *retrieval.Engine).
func registerGetSemanticContext(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "get_semantic_context",
		Description: "Ranked, evidence-backed semantic context (read+).",
	}, kernel.WrapToolSpan(tracer, "get_semantic_context",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args GetSemanticContextArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleGetSemanticContext(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "get_semantic_context",
		Description:      "Ranked, evidence-backed semantic context (read+).",
		BriefDescription: "Semantic context retrieval",
		HelpText:         contextHelp,
	})
}

// handleGetSemanticContext is the testable handler body. Order is
// load-bearing:
//  1. checkMode(modeTierRead) — every session passes; retained for code-
//     review visibility and the Phase 66 GuardrailMiddleware precedent.
//  2. validatePaths(args.Files, ws.RepoRoot) — reject path traversal +
//     absolute paths outside root (T-64-07-01).
//  3. Clamp args.MaxTokens to [retrieval.MinTokenBudget,
//     retrieval.MaxTokenBudget]; default retrieval.DefaultContextBudget
//     (T-64-07-02).
//  4. Resolve closed-enum FreshnessMode (default allow_stale).
//  5. Read retrieval.RetrievalPending(ws); if true, return stale envelope
//     with Candidates=nil immediately (do NOT touch QueryBleve / PageRank).
//  6. Read graph_version, overlay_active, pending_lsp_files (per-accessor
//     tolerated errors, mirrors tools_status fan-out doctrine).
//  7. Fan out QueryBleve + PersonalizedPageRank against (task, anchors).
//  8. rrf.Fuse text + graph rankings using DefaultRRFConfig.
//  9. greedyPack candidates under the token budget.
//  10. Build per-candidate ContextEvidence — TopEdges from
//     retrieval.TopEdgesFor capped at retrieval.TopEdgesPerCandidate (5).
//  11. Compute closed-enum Freshness from
//     computeContextFreshness(overlayActive, pendingLSP>0, retrievalPending).
//  12. Marshal envelope (SPEC §23.4).
//
// Pure read path. No write surfaces invoked (T-64-07-05/-07/-08 all flow
// through the read-only RetrievalAccessor + StoreAccessor seams).
func (s *SemanticSkill) handleGetSemanticContext(ctx context.Context, args GetSemanticContextArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes; retained for
	//    code-review visibility and Phase 66 GuardrailMiddleware precedent).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + path-traversal validation (T-64-07-01).
	ws := s.workspaceKey(ctx)
	if err := validatePaths(args.Files, ws.RepoRoot); err != nil {
		return errorResult(err.Error())
	}

	// 3. Clamp token budget (T-64-07-02). Empty / non-positive → default.
	budget := args.MaxTokens
	if budget <= 0 {
		budget = retrieval.DefaultContextBudget
	}
	if budget < retrieval.MinTokenBudget {
		budget = retrieval.MinTokenBudget
	}
	if budget > retrieval.MaxTokenBudget {
		budget = retrieval.MaxTokenBudget
	}

	// 4. Resolve closed-enum FreshnessMode (default allow_stale).
	fmod := resolveFreshnessMode(args.FreshnessMode)

	// 5. Read RetrievalPending. When true, return immediately with the
	//    stale envelope — DO NOT call QueryBleve / PageRank because the
	//    bleve segment is mid-rebuild and would produce inconsistent
	//    rankings (T-64-07-05/-06).
	retrievalPending := false
	if s.retrieval != nil {
		retrievalPending = s.retrieval.RetrievalPending(ws)
	}

	repoID := ws.Hash()

	// 6. Read graph_version + overlay_active + pending_lsp_files. Per-
	//    accessor errors are tolerated (status-style fan-out): a degraded
	//    read still produces a structured envelope.
	var (
		graphVersion    uint64
		overlayActive   bool
		pendingLSPFiles int
	)
	if s.store != nil {
		if gv, err := s.store.CurrentGraphVersion(ctx, repoID); err == nil {
			graphVersion = gv
		} else if s.logger != nil {
			s.logger.Warn("get_semantic_context: CurrentGraphVersion read failed",
				"repo_id", repoID, "err", err)
		}
		overlayActive = s.store.OverlayHasPendingRows(repoID)
	}
	if s.queue != nil {
		pendingLSPFiles = s.queue.DepthAll(ws)
	}

	if retrievalPending {
		return jsonResult(ContextResult{
			CommonEnvelope: CommonEnvelope{
				Freshness:     FreshnessStale,
				GraphVersion:  graphVersion,
				OverlayActive: overlayActive,
			},
			FreshnessMode:    fmod,
			PendingLSPFiles:  pendingLSPFiles,
			RetrievalPending: true,
			Candidates:       nil,
		})
	}

	// 7. Fan out QueryBleve + PersonalizedPageRank.
	anchors := append(append([]string{}, args.Files...), args.Symbols...)

	var textRanksRaw []TextRank
	var graphRanksRaw []GraphRank
	if s.retrieval != nil {
		if tr, err := s.retrieval.QueryBleve(args.Task, anchors); err == nil {
			textRanksRaw = tr
		} else if s.logger != nil {
			s.logger.Warn("get_semantic_context: QueryBleve failed",
				"repo_id", repoID, "err", err)
		}
		if gr, err := s.retrieval.PersonalizedPageRank(ctx, repoID, anchors); err == nil {
			graphRanksRaw = gr
		} else if s.logger != nil {
			s.logger.Warn("get_semantic_context: PersonalizedPageRank failed",
				"repo_id", repoID, "err", err)
		}
	}

	// Translate skill-package TextRank/GraphRank → retrieval-package
	// TextRank/GraphRank for rrf.Fuse. The two types differ only by
	// package — keeping retrieval/ self-contained avoids an import cycle.
	textForFuse := make([]retrieval.TextRank, 0, len(textRanksRaw))
	for _, t := range textRanksRaw {
		textForFuse = append(textForFuse, retrieval.TextRank{SymbolID: t.SymbolID, Score: t.Score})
	}
	graphForFuse := make([]retrieval.GraphRank, 0, len(graphRanksRaw))
	for _, g := range graphRanksRaw {
		graphForFuse = append(graphForFuse, retrieval.GraphRank{SymbolID: g.SymbolID, Score: g.Score})
	}

	// 8. Fuse via weighted RRF; gv lookup is per-snapshot (no per-symbol
	//    graph_version surface this phase) — using the global graph_version
	//    for tiebreak satisfies sort-before-iterate determinism because
	//    every symbol gets the same gv, and the third tiebreak (symbol_id
	//    asc) decides pure score ties.
	gvLookup := func(symbolID string) uint64 { return graphVersion }
	fused := retrieval.Fuse(textForFuse, graphForFuse, retrieval.DefaultRRFConfig(), gvLookup)

	// 9. Greedy-pack under the token budget.
	packed := greedyPack(fused, budget, tokensPerCandidate)

	// 10. Build per-candidate Evidence. TopEdges from RetrievalAccessor,
	//     capped at TopEdgesPerCandidate.
	candidates := make([]ContextCandidate, 0, len(packed))
	for _, c := range packed {
		var topEdges []string
		if s.retrieval != nil {
			if e, err := s.retrieval.TopEdgesFor(ctx, repoID, c.SymbolID); err == nil {
				topEdges = e
			} else if s.logger != nil {
				s.logger.Warn("get_semantic_context: TopEdgesFor failed",
					"repo_id", repoID, "symbol_id", c.SymbolID, "err", err)
			}
		}
		if len(topEdges) > retrieval.TopEdgesPerCandidate {
			topEdges = topEdges[:retrieval.TopEdgesPerCandidate]
		}
		candidates = append(candidates, ContextCandidate{
			SymbolID:   c.SymbolID,
			Confidence: clampToUnit(c.Score),
			Evidence: ContextEvidence{
				TextRank:     c.TextRank,
				GraphRank:    c.GraphRank,
				MatchedTerms: c.MatchedTerms,
				TopEdges:     topEdges,
			},
		})
	}

	// 11. Closed-enum freshness selection.
	freshness := computeContextFreshness(overlayActive, pendingLSPFiles > 0, retrievalPending)

	// 12. Marshal envelope (SPEC §23.4).
	result := jsonResult(ContextResult{
		CommonEnvelope: CommonEnvelope{
			Freshness:     freshness,
			GraphVersion:  graphVersion,
			OverlayActive: overlayActive,
		},
		FreshnessMode:    fmod,
		PendingLSPFiles:  pendingLSPFiles,
		RetrievalPending: retrievalPending,
		Candidates:       candidates,
	})

	// Phase 66 D-01 GUARD-03: issue receipt on success path ONLY (Pitfall 4: never defer).
	targetSymbols := make([]integ.SymbolID, 0, len(args.Symbols))
	for _, s := range args.Symbols {
		targetSymbols = append(targetSymbols, integ.SymbolID(s))
	}
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			FileSet:         args.Files,
			TargetSymbols:   targetSymbols,
			TaskHash:        computeSemanticTaskHash(args),
			TokenBudgetUsed: len(candidates),
			MaxTokens:       budget,
		}, "get_semantic_context")
	return result
}

// ----- helpers (P64-07-owned; do NOT add to handler_helpers.go which is
// owned by P64-04) -----

// resolveFreshnessMode maps the request string to the closed-enum
// FreshnessMode, defaulting to allow_stale on empty / unrecognized input.
func resolveFreshnessMode(raw string) FreshnessMode {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(FreshnessModeRequireCurrent):
		return FreshnessModeRequireCurrent
	case string(FreshnessModeValidateLive):
		return FreshnessModeValidateLive
	default:
		return FreshnessModeAllowStale
	}
}

// computeContextFreshness selects the closed-enum Freshness value from the
// observed state. Priority order (matches tools_status.go's fan-out doctrine):
//
//	retrievalPending          → stale (engine rebuilding overrides everything)
//	overlayActive && lspPending → structurally_fresh_semantically_pending
//	overlayActive             → overlay_active
//	default                   → fresh
func computeContextFreshness(overlayActive, lspPending, retrievalPending bool) Freshness {
	switch {
	case retrievalPending:
		return FreshnessStale
	case overlayActive && lspPending:
		return FreshnessStructurallyFreshSemanticallyPending
	case overlayActive:
		return FreshnessOverlayActive
	default:
		return FreshnessFresh
	}
}

// greedyPack selects FusedCandidates greedily under the token budget. Skips
// (does NOT break on) candidates that exceed remaining budget so smaller
// followers can still pack — the "underfill" plan-design choice from
// CONTEXT.md "Claude's Discretion" -> "Token-budget packing".
func greedyPack(candidates []retrieval.FusedCandidate, budget int, costFn func(retrieval.FusedCandidate) int) []retrieval.FusedCandidate {
	out := make([]retrieval.FusedCandidate, 0, len(candidates))
	used := 0
	for _, c := range candidates {
		cost := costFn(c)
		if used+cost > budget {
			continue
		}
		out = append(out, c)
		used += cost
	}
	return out
}

// tokensPerCandidate estimates the token cost of a single candidate's
// envelope contribution. Simple per-field byte length / 4 heuristic; the
// daemon-wiring layer (P64-08) is the natural place to plumb a real
// tokenizer (e.g., tiktoken) if measurement reveals the heuristic
// misestimates pathologically.
func tokensPerCandidate(c retrieval.FusedCandidate) int {
	// Floor at 8 tokens per candidate so even an empty SymbolID candidate
	// occupies a slot — prevents pathological "infinite candidates fit"
	// when MatchedTerms is empty.
	const floor = 8
	const bytesPerToken = 4

	cost := floor + (len(c.SymbolID) / bytesPerToken)
	for _, term := range c.MatchedTerms {
		cost += len(term) / bytesPerToken
	}
	return cost
}

// clampToUnit constrains a raw RRF score to [0, 1]. RRF scores in this
// package are bounded above by ~ 1/(K+1) per source (≈ 1/61 with K=60),
// so the typical clamp is a no-op; the bound is defensive against
// pathological negative weights that could enter via weighted RRF.
func clampToUnit(score float64) float64 {
	if score <= 0 {
		return 0
	}
	if score >= 1 {
		return 1
	}
	return score
}

// computeSemanticTaskHash returns a short hex hash of the args that identify a
// get_semantic_context call for use as the TaskHash in ContextGatheredScope.
func computeSemanticTaskHash(args GetSemanticContextArgs) string {
	h := sha256.New()
	h.Write([]byte(args.Task))
	h.Write([]byte{0})
	for _, f := range args.Files {
		h.Write([]byte(f))
		h.Write([]byte{0})
	}
	for _, s := range args.Symbols {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return fmt.Sprintf("%x", h.Sum(nil))[:16]
}
