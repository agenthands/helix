package semantic

import (
	"context"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/types"
)

// INVARIANT (D-09 / D-13): explain_symbol_deep MUST NOT touch the snapshot-
// write surface of *Store. Specifically: no Begin/Commit/Abort/Write methods
// on snapshots, and no compactor flush trigger. The grep gate in CI enforces
// the absence of those identifier tokens in this file; the recorder mocks in
// tools_explain_symbol_test.go enforce it under unit test. Read+ stays
// read-only with respect to committed state — explain only consumes the
// existing committed snapshot + overlay state through narrow accessors.

// D2 surface caps (CONTEXT.md):
//   - callers truncate to 50
//   - incoming / outgoing edges truncate to 100 per direction
const (
	explainCallersCap = 50
	explainEdgesCap   = 100
)

// ExplainSymbolDeepArgs is the typed-args input schema for explain_symbol_deep.
type ExplainSymbolDeepArgs struct {
	Seed SeedInput `json:"seed" jsonschema:"seed symbol — symbol_id OR (file_path AND symbol_name)"`
}

// SeedEnvelope echoes the resolved seed back to the caller, including the
// closed-enum resolution disposition and (for ambiguous resolution) the
// candidate list.
type SeedEnvelope struct {
	SymbolID            string           `json:"symbol_id,omitempty"`
	Resolution          Resolution       `json:"resolution"`
	AmbiguousCandidates []integ.SymbolID `json:"ambiguous_candidates,omitempty"`
}

// TypeChainEntry is one entry in the explain_symbol_deep type_chain field.
// Tier is the SPEC §38.2 ladder tier string ("tier1_lsp" .. "tier7_unknown").
// EvidenceKind is the types.EvidenceKind serialization.
type TypeChainEntry struct {
	Tier           string  `json:"tier"`
	EvidenceKind   string  `json:"evidence_kind"`
	Confidence     float64 `json:"confidence"`
	TargetSymbolID string  `json:"target_symbol_id,omitempty"`
}

// CallerRef is one caller entry in the response.
type CallerRef struct {
	From         integ.SymbolID  `json:"from"`
	EdgeKind     EdgeKindSurface `json:"edge_kind"`
	InternalKind string          `json:"internal_kind"`
}

// EdgeRef is one edge entry in the response (incoming or outgoing).
type EdgeRef struct {
	From         integ.SymbolID  `json:"from,omitempty"`
	To           integ.SymbolID  `json:"to,omitempty"`
	EdgeKind     EdgeKindSurface `json:"edge_kind"`
	InternalKind string          `json:"internal_kind"`
}

// ClusterRef is the cluster membership reference. ClusterID==0 + Size==0
// signals "no cluster" (isolated symbol OR cluster engine disabled); both
// are non-error states.
type ClusterRef struct {
	ClusterID uint64 `json:"cluster_id"`
	Size      int    `json:"size"`
}

// ExplainSymbolDeepResult is the explain_symbol_deep response shape.
type ExplainSymbolDeepResult struct {
	Seed                       SeedEnvelope     `json:"seed"`
	TypeChain                  []TypeChainEntry `json:"type_chain"`
	Callers                    []CallerRef      `json:"callers"`
	CallersCountTotal          int              `json:"callers_count_total"`
	CallersCountReturned       int              `json:"callers_count_returned"`
	EdgesIncoming              []EdgeRef        `json:"edges_incoming"`
	EdgesIncomingCountTotal    int              `json:"edges_incoming_count_total"`
	EdgesIncomingCountReturned int              `json:"edges_incoming_count_returned"`
	EdgesOutgoing              []EdgeRef        `json:"edges_outgoing"`
	EdgesOutgoingCountTotal    int              `json:"edges_outgoing_count_total"`
	EdgesOutgoingCountReturned int              `json:"edges_outgoing_count_returned"`
	Cluster                    ClusterRef       `json:"cluster"`
	Freshness                  FreshnessV2      `json:"freshness"`
	Confidence                 float64          `json:"confidence"`
	FallbackReason             string           `json:"fallback_reason,omitempty"`
}

// explainSymbolDeepHelp is the verbose help text for explain_symbol_deep.
const explainSymbolDeepHelp = `
## Usage Examples

Explain a symbol by stable id:
  explain_symbol_deep(seed={symbol_id: "repo/src/svc.go::ServeHTTP"})

Explain by (file_path, symbol_name) tuple:
  explain_symbol_deep(seed={file_path: "src/svc.go", symbol_name: "ServeHTTP"})

## Parameters
- seed (object, required): One of two forms —
    * { symbol_id: <stable graph id> } — direct lookup, no name resolution.
    * { file_path, symbol_name } — name-resolved lookup; ambiguous matches
      surface the candidates without selecting one (resolution=ambiguous).

## Return Shape
- seed (object): { symbol_id, resolution ∈ {exact|ambiguous|not_found},
  ambiguous_candidates? }.
- type_chain ([]TypeChainEntry): tier + evidence_kind + confidence per
  resolution step (SPEC §38.2 7-tier ladder).
- callers ([]CallerRef): up to 50 entries; full count in
  callers_count_total / callers_count_returned.
- edges_incoming / edges_outgoing ([]EdgeRef): up to 100 per direction;
  full counts in *_count_total / *_count_returned.
- cluster (ClusterRef): { cluster_id, size }. size=0 is non-error
  (isolated symbol or cluster engine disabled).
- freshness (FreshnessV2): { graph_version, snapshot_id, extractor_run_id,
  as_of_unix_ms, status ∈ {current|stale|unknown}, source }.
- confidence (float64): top-level confidence; capped at 0.6 when the
  type-resolver returned tier6_heuristic or tier7_unknown (TYPES-04 cap
  via types.CapCommentConfidence).
- fallback_reason (string, optional): closed-enum reason —
  "symbol_not_found" | "type_resolver_degraded".

## Mode Tier
read+ — every session passes; this is the deep-inspection counterpart to
goto_definition + find_references + get_context.

## Caps (D2)
callers ≤ 50, edges ≤ 100 per direction. Truncation is honest: the count
fields tell the caller exactly how many edges existed pre-truncation.

## Determinism
Same seed inputs produce byte-identical response ordering across runs
(Phase 62 sort-before-iterate doctrine).`

// registerExplainSymbolDeep wires explain_symbol_deep into the MCP server
// with kernel-style typed-args registration + tracing.
func registerExplainSymbolDeep(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "explain_symbol_deep",
		Description: "Deep symbol explanation: type chain, callers, edges, cluster (read+).",
	}, kernel.WrapToolSpan(tracer, "explain_symbol_deep",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args ExplainSymbolDeepArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleExplainSymbolDeep(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "explain_symbol_deep",
		Description:      "Deep symbol explanation: type chain, callers, edges, cluster (read+).",
		BriefDescription: "Deep symbol explanation",
		HelpText:         explainSymbolDeepHelp,
	})
}

// handleExplainSymbolDeep is the testable handler body. Order is
// load-bearing:
//  1. checkMode(modeTierRead) — every session passes; retained for code-
//     review visibility and Phase 66 GuardrailMiddleware precedent.
//  2. resolveSeed — translate input to ResolvedSeed; short-circuit on
//     not_found with fallback_reason="symbol_not_found".
//  3. Read type_chain rows + classify as TypeChainEntry; track whether any
//     entry is degraded (tier6_heuristic / tier7_unknown).
//  4. Read callers + incoming + outgoing edges; map InternalKind via
//     MapInternalKind; cap to D2 limits; record honest counts.
//  5. Read cluster_id + size (cluster engine may be disabled; size=0 is OK).
//  6. Assemble FreshnessV2 envelope.
//  7. Compute top-level confidence, clamping via types.CapCommentConfidence
//     when degraded.
//  8. Issue context-gathered receipt; return jsonResult.
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the snapshot-
// write surface of *Store nor the per-workspace compactor's flush trigger.
// The recorder mocks in tools_explain_symbol_test.go fail loudly on any
// future regression that reaches them.
func (s *SemanticSkill) handleExplainSymbolDeep(ctx context.Context, args ExplainSymbolDeepArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check (read+ — every session passes; retained for
	//    code-review visibility and Phase 66 GuardrailMiddleware precedent).
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Resolve workspace + seed.
	ws := s.workspaceKey(ctx)
	repoID := ws.Hash()

	resolved, err := s.resolveSeed(ctx, ws, args.Seed)
	if err != nil {
		return errorResult(err.Error())
	}

	seedEnv := SeedEnvelope{
		SymbolID:            string(resolved.SymbolID),
		Resolution:          resolved.Resolution,
		AmbiguousCandidates: resolved.AmbiguousCandidates,
	}

	// Short-circuit on not_found — return structured envelope with the
	// freshness envelope still populated so the caller can distinguish a
	// real miss from a stale read.
	if resolved.Resolution == ResolutionNotFound {
		return jsonResult(ExplainSymbolDeepResult{
			Seed:           seedEnv,
			Freshness:      s.assembleFreshness(ctx, repoID),
			FallbackReason: "symbol_not_found",
		})
	}

	// 3. Type chain.
	var typeChainRows []TypeChainRow
	if tc := s.getTypeChain(); tc != nil {
		if rows, err := tc.TypeChainForSymbol(ctx, repoID, resolved.SymbolID); err == nil {
			typeChainRows = rows
		} else if s.logger != nil {
			s.logger.Warn("explain_symbol_deep: TypeChainForSymbol failed",
				"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", err)
		}
	}

	degraded := false
	typeChainEntries := make([]TypeChainEntry, 0, len(typeChainRows))
	var rawConfidence float64
	for _, row := range typeChainRows {
		ek := types.EvidenceKind(row.EvidenceKind)
		conf := types.ConfidenceForEvidence(ek)
		typeChainEntries = append(typeChainEntries, TypeChainEntry{
			Tier:           row.Tier,
			EvidenceKind:   row.EvidenceKind,
			Confidence:     conf,
			TargetSymbolID: row.TargetSymbolID,
		})
		if row.Tier == "tier6_heuristic" || row.Tier == "tier7_unknown" ||
			ek == types.EvidenceHeuristic || ek == types.EvidenceUnknown {
			degraded = true
		}
		if conf > rawConfidence {
			rawConfidence = conf
		}
	}

	// 4. Edges + callers.
	var callers []CallerRef
	callersTotal := 0
	var incoming []EdgeRef
	incomingTotal := 0
	var outgoing []EdgeRef
	outgoingTotal := 0

	if se := s.getSymbolEdges(); se != nil {
		if rows, err := se.CallersOf(ctx, repoID, resolved.SymbolID); err == nil {
			callersTotal = len(rows)
			capped := rows
			if len(capped) > explainCallersCap {
				capped = capped[:explainCallersCap]
			}
			callers = make([]CallerRef, 0, len(capped))
			for _, r := range capped {
				callers = append(callers, CallerRef{
					From:         r.From,
					EdgeKind:     MapInternalKind(r.InternalKind),
					InternalKind: r.InternalKind,
				})
			}
		} else if s.logger != nil {
			s.logger.Warn("explain_symbol_deep: CallersOf failed",
				"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", err)
		}
		if rows, err := se.IncomingEdgesOf(ctx, repoID, resolved.SymbolID); err == nil {
			incoming, incomingTotal = shapeEdges(rows, explainEdgesCap)
		} else if s.logger != nil {
			s.logger.Warn("explain_symbol_deep: IncomingEdgesOf failed",
				"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", err)
		}
		if rows, err := se.OutgoingEdgesOf(ctx, repoID, resolved.SymbolID); err == nil {
			outgoing, outgoingTotal = shapeEdges(rows, explainEdgesCap)
		} else if s.logger != nil {
			s.logger.Warn("explain_symbol_deep: OutgoingEdgesOf failed",
				"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", err)
		}
	}

	// 5. Cluster reference.
	var cluster ClusterRef
	s.mu.Lock()
	cm := s.clusterMembership
	s.mu.Unlock()
	if cm != nil {
		if id, size, err := cm.ClusterIDOf(ctx, repoID, resolved.SymbolID); err == nil {
			cluster.ClusterID = id
			cluster.Size = size
		} else if s.logger != nil {
			s.logger.Warn("explain_symbol_deep: ClusterIDOf failed",
				"repo_id", repoID, "symbol_id", resolved.SymbolID, "err", err)
		}
	}

	// 6. Freshness envelope.
	freshness := s.assembleFreshness(ctx, repoID)

	// 7. Top-level confidence: cap on degraded paths per TYPES-04.
	confidence := rawConfidence
	fallbackReason := ""
	if degraded {
		confidence = types.CapCommentConfidence(confidence, types.EvidenceComment)
		// Ensure the cap holds even when no EvidenceComment branch
		// triggered the CapCommentConfidence early-return; clamp manually
		// to the same ceiling because the degraded signal extends to all
		// tier6/tier7 origins, not just comment edges.
		if confidence > types.ConfidenceComment {
			confidence = types.ConfidenceComment
		}
		fallbackReason = "type_resolver_degraded"
	}

	result := ExplainSymbolDeepResult{
		Seed:                       seedEnv,
		TypeChain:                  typeChainEntries,
		Callers:                    callers,
		CallersCountTotal:          callersTotal,
		CallersCountReturned:       len(callers),
		EdgesIncoming:              incoming,
		EdgesIncomingCountTotal:    incomingTotal,
		EdgesIncomingCountReturned: len(incoming),
		EdgesOutgoing:              outgoing,
		EdgesOutgoingCountTotal:    outgoingTotal,
		EdgesOutgoingCountReturned: len(outgoing),
		Cluster:                    cluster,
		Freshness:                  freshness,
		Confidence:                 confidence,
		FallbackReason:             fallbackReason,
	}

	// 8. Receipt issuance on success path.
	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			TargetSymbols:   []integ.SymbolID{resolved.SymbolID},
			TaskHash:        string(resolved.SymbolID),
			TokenBudgetUsed: len(typeChainEntries) + len(callers) + len(incoming) + len(outgoing),
			MaxTokens:       0,
		}, "explain_symbol_deep")
	return jsonResult(result)
}

// assembleFreshness builds the FreshnessV2 envelope for explain_symbol_deep
// from the current store state. Status selection:
//   - current: snapshot_id != 0 AND extractor_run_id != "" AND !overlay_active
//   - stale:   overlay_active
//   - unknown: snapshot_id == 0 (no committed snapshot yet)
func (s *SemanticSkill) assembleFreshness(ctx context.Context, repoID string) FreshnessV2 {
	var (
		graphVersion  uint64
		snapshotID    uint64
		overlayActive bool
	)
	s.mu.Lock()
	store := s.store
	extractor := s.extractorRun
	s.mu.Unlock()

	if store != nil {
		if gv, err := store.CurrentGraphVersion(ctx, repoID); err == nil {
			graphVersion = gv
		}
		if sid, err := store.LatestCommittedSnapshot(ctx, repoID); err == nil {
			snapshotID = sid
		}
		overlayActive = store.OverlayHasPendingRows(repoID)
	}

	var runID string
	if extractor != nil {
		if rid, err := extractor.LatestExtractorRunID(ctx, repoID); err == nil {
			runID = rid
		}
	}

	status := FreshnessStatusUnknown
	switch {
	case snapshotID == 0:
		status = FreshnessStatusUnknown
	case overlayActive:
		status = FreshnessStatusStale
	case runID != "":
		status = FreshnessStatusCurrent
	default:
		status = FreshnessStatusStale
	}

	return FreshnessV2{
		GraphVersion:   graphVersion,
		SnapshotID:     snapshotID,
		ExtractorRunID: runID,
		AsOfUnixMs:     time.Now().UnixMilli(),
		Status:         status,
		Source:         FreshnessSourceGraph,
	}
}

// getTypeChain returns the wired TypeChainAccessor under the skill mutex.
func (s *SemanticSkill) getTypeChain() TypeChainAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.typeChain
}

// getSymbolEdges returns the wired SymbolEdgesAccessor under the skill mutex.
func (s *SemanticSkill) getSymbolEdges() SymbolEdgesAccessor {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.symbolEdges
}

// shapeEdges maps a slice of SymbolEdgeRow into []EdgeRef while applying
// MapInternalKind and a fixed cap. Returns (capped slice, pre-cap total).
func shapeEdges(rows []SymbolEdgeRow, cap int) ([]EdgeRef, int) {
	total := len(rows)
	capped := rows
	if total > cap {
		capped = rows[:cap]
	}
	out := make([]EdgeRef, 0, len(capped))
	for _, r := range capped {
		out = append(out, EdgeRef{
			From:         r.From,
			To:           r.To,
			EdgeKind:     MapInternalKind(r.InternalKind),
			InternalKind: r.InternalKind,
		})
	}
	return out, total
}
