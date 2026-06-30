package semantic

import (
	"context"
	"fmt"
	"sort"
	"strings"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"go.opentelemetry.io/otel/trace"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/guardrails"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/types"
)

// INVARIANT (D-09 / D-13): validate_graph_edge MUST NOT touch the snapshot-
// write surface of *Store. Specifically: no Begin/Commit/Abort/Write methods
// on snapshots, and no compactor flush trigger. The grep gate in CI enforces
// the absence of those identifier tokens in this file; the recorder mocks in
// tools_validate_edge_test.go enforce it under unit test. Read+ stays
// read-only with respect to committed state — validate only consumes per-edge
// evidence rows via narrow accessors.

// D4 caps + scoring constants (71-CONTEXT.md):
//   - evidence array cap = 10 (cap; total reported honestly)
//   - source contribution weights: LSP=0.4, AST=0.3, type_resolver=scaled-by-tier
//   - TYPES-04 cap: top-level confidence ≤ 0.6 when evidence_status != complete
//     OR any type_resolver tier ≥ tier6_heuristic.
const (
	validateEdgeEvidenceCap   = 10
	contributionLSP           = 0.4
	contributionASTComplete   = 0.3
	contributionASTPartial    = 0.3 // contribution value identical; envelope status differs
	contributionTierTier1     = 0.30
	contributionTierTier2     = 0.30
	contributionTierTier3     = 0.25
	contributionTierTier4     = 0.20
	contributionTierTier5     = 0.15
	contributionTierTier6     = 0.10
	contributionTierTier7     = 0.05
	degradedConfidenceCeiling = types.ConfidenceComment // 0.60
)

// EvidenceSource is the closed enum identifying which subsystem produced an
// evidence citation. D4: source ∈ {lsp, ast, type_resolver}.
type EvidenceSource string

const (
	// EvidenceSourceLSP — citation came from an LSP method (definition,
	// references, implementation). Confidence contribution = 0.4.
	EvidenceSourceLSP EvidenceSource = "lsp"
	// EvidenceSourceAST — citation came from tree-sitter extractor metadata.
	// Confidence contribution = 0.3. Missing tree_sitter_kind/range metadata
	// drops envelope evidence_status to "partial" (Open Q3 resolution).
	EvidenceSourceAST EvidenceSource = "ast"
	// EvidenceSourceTypeResolver — citation came from the 7-tier type
	// resolver ladder. Confidence contribution is scaled by tier (tier1=0.30
	// down to tier7=0.05). Tier6/tier7 trigger the TYPES-04 top-level cap.
	EvidenceSourceTypeResolver EvidenceSource = "type_resolver"
)

// EvidenceStatus is the closed enum classifying the assembled evidence array's
// completeness. D4: evidence_status ∈ {complete, partial, none}.
type EvidenceStatus string

const (
	// EvidenceStatusComplete — at least one citation per source class is
	// present AND no AST partial fallback AND no type_resolver tier ≥ tier6.
	EvidenceStatusComplete EvidenceStatus = "complete"
	// EvidenceStatusPartial — at least one citation present but some
	// degradation signal exists (missing AST metadata, tier ≥ tier6, or
	// fewer than all three source classes).
	EvidenceStatusPartial EvidenceStatus = "partial"
	// EvidenceStatusNone — no citations could be assembled (graph attests
	// the edge exists but no metadata available, or edge absent entirely).
	EvidenceStatusNone EvidenceStatus = "none"
)

// EvidenceCitation is one row in the validate_graph_edge response's evidence
// array. Each citation surfaces (a) the source class, (b) optional metadata
// (lsp method / tree-sitter kind / tier), (c) optional source location, and
// (d) the unclamped per-source confidence contribution (D4 asymmetry — see
// handler step (k)).
type EvidenceCitation struct {
	Source                 EvidenceSource `json:"source"`
	LSPMethod              string         `json:"lsp_method,omitempty"`
	TreeSitterKind         string         `json:"tree_sitter_kind,omitempty"`
	Tier                   string         `json:"tier,omitempty"`
	EvidenceKind           string         `json:"evidence_kind,omitempty"`
	File                   string         `json:"file,omitempty"`
	Range                  *EvidenceRange `json:"range,omitempty"`
	ConfidenceContribution float64        `json:"confidence_contribution"`
}

// ValidateGraphEdgeArgs is the typed-args input schema for validate_graph_edge.
// EdgeKind accepts the closed surface enum values only; freeform input is
// rejected via serr.InvalidArgs.
type ValidateGraphEdgeArgs struct {
	From     SeedInput `json:"from"      jsonschema:"source seed — symbol_id OR (file_path AND symbol_name)"`
	To       SeedInput `json:"to"        jsonschema:"target seed — symbol_id OR (file_path AND symbol_name)"`
	EdgeKind string    `json:"edge_kind" jsonschema:"closed surface enum: calls / references / implements / extends / has_type / uses_type / contains / defines / imports / data_flows / http_calls / async_calls / emits / listens_on / similar_to / semantically_related / handles / configures / writes / member_of / tests / file_changes_with / cross_imports / cross_calls / other"`
}

// ValidateGraphEdgeResult is the validate_graph_edge response shape. Both
// per-seed resolutions surface independently so the caller can detect partial
// ambiguity (one seed exact, the other ambiguous) without re-issuing the call.
type ValidateGraphEdgeResult struct {
	FromResolution        Resolution         `json:"from_resolution"`
	ToResolution          Resolution         `json:"to_resolution"`
	FromSymbolID          string             `json:"from_symbol_id,omitempty"`
	ToSymbolID            string             `json:"to_symbol_id,omitempty"`
	Confidence            float64            `json:"confidence"`
	Evidence              []EvidenceCitation `json:"evidence"`
	EvidenceCountTotal    int                `json:"evidence_count_total"`
	EvidenceCountReturned int                `json:"evidence_count_returned"`
	EvidenceStatus        EvidenceStatus     `json:"evidence_status"`
	Freshness             FreshnessV2        `json:"freshness"`
	FallbackReason        string             `json:"fallback_reason,omitempty"`
}

// validateGraphEdgeHelp is the verbose help text for validate_graph_edge.
const validateGraphEdgeHelp = `
## Usage Examples

Validate an edge by stable ids:
  validate_graph_edge(
    from={symbol_id: "repo/src/svc.go::ServeHTTP"},
    to={symbol_id: "repo/src/svc.go::handle"},
    edge_kind="calls",
  )

Validate using (file, name) tuples on either side:
  validate_graph_edge(
    from={file_path: "src/svc.go", symbol_name: "ServeHTTP"},
    to={file_path: "src/svc.go", symbol_name: "handle"},
    edge_kind="calls",
  )

## Parameters
- from / to (object, required): each one of two forms —
    * { symbol_id: <stable graph id> } — direct lookup, no name resolution.
    * { file_path, symbol_name } — name-resolved lookup; ambiguous matches
      surface in {from,to}_resolution = "ambiguous".
- edge_kind (string, required): CLOSED surface enum. One of:
    calls / references / implements / extends / has_type / uses_type /
    contains / other. Freeform values are rejected via InvalidArgs.

## Return Shape
- from_resolution / to_resolution (string ∈ {exact|ambiguous|not_found})
- from_symbol_id / to_symbol_id (string, omitted when not_found)
- confidence (float64): top-level confidence ∈ [0, 1]. Capped at ≤ 0.6 when
  evidence_status != "complete" OR any type_resolver tier is tier6/tier7.
- evidence ([]EvidenceCitation): up to 10 citations sorted by
  confidence_contribution descending. Per-source contribution values are
  UNCLAMPED (D4 asymmetry; the cap applies only to the top-level confidence).
- evidence_count_total / evidence_count_returned (int): honest truncation.
- evidence_status (string ∈ {complete|partial|none})
- freshness (FreshnessV2): { graph_version, snapshot_id, extractor_run_id,
  as_of_unix_ms, status ∈ {current|stale|unknown}, source }.
- fallback_reason (string, optional): closed-enum reason —
    * "symbol_not_found"            — either seed resolved to not_found.
    * "edge_not_found"               — no row matched the (from, to,
                                       projected_internal_kinds) tuple.
    * "evidence_lookup_lagging"      — edge exists per the graph but the
                                       evidence accessor returned no metadata
                                       rows (D4 lenient path).
    * "evidence_lookup_unavailable"  — EdgeEvidenceAccessor not wired.

## Mode Tier
read+ — every session passes; this is the third single-symbol read tool
alongside explain_symbol_deep and find_related_symbols.

## TYPES-04 Cap
When the response carries a tier6_heuristic / tier7_unknown citation OR the
envelope's evidence_status is not "complete", the top-level confidence is
clamped to ≤ 0.6 via types.CapCommentConfidence. Per-source contributions in
the evidence array are NOT clamped (D4 asymmetry — preserves the raw signal
so downstream agents can debug why the top-level was capped).`

// registerValidateGraphEdge wires validate_graph_edge into the MCP server
// with kernel-style typed-args registration + tracing.
func registerValidateGraphEdge(server *mcp.SerenaMCPServer, s *SemanticSkill, tracer trace.Tracer) {
	mcpsdk.AddTool(server.SDK(), &mcpsdk.Tool{
		Name:        "validate_graph_edge",
		Description: "Validate a (from, to, edge_kind) graph claim with confidence + evidence (read+).",
	}, kernel.WrapToolSpan(tracer, "validate_graph_edge",
		func(ctx context.Context, req *mcpsdk.CallToolRequest, args ValidateGraphEdgeArgs) (*mcpsdk.CallToolResult, any, error) {
			return s.handleValidateGraphEdge(ctx, args), nil, nil
		}))
	server.Registry().Register(&mcp.ToolDef{
		Name:             "validate_graph_edge",
		Description:      "Validate a (from, to, edge_kind) graph claim with confidence + evidence (read+).",
		BriefDescription: "Validate a graph edge claim",
		HelpText:         validateGraphEdgeHelp,
	})
}

// surfaceToInternalKinds projects a closed-enum surface EdgeKind to the set
// of internal extractor/resolver kinds that could project up to it. Inverse
// of MapInternalKind (which is one-way per D3). This reverse mapping is
// inline-documented in this handler ONLY (per D3 + 71-04 Deferred Ideas) so
// the surface→internal contract stays a per-tool concern rather than a public
// API on MapInternalKind.
func surfaceToInternalKinds(surface EdgeKindSurface) []string {
	switch surface {
	case EdgeKindCalls:
		return []string{"CALLS"}
	case EdgeKindReferences:
		// References surface absorbs IMPORTS per 71-RESEARCH.md line 347.
		return []string{"REFERENCES", "IMPORTS"}
	case EdgeKindImplements:
		return []string{"IMPLEMENTS"}
	case EdgeKindExtends:
		return []string{"EXTENDS"}
	case EdgeKindHasType:
		// Pitfall 2: RESOLVES_TO projects to has_type.
		return []string{"RESOLVES_TO"}
	case EdgeKindUsesType:
		return []string{"USES_TYPE"}
	case EdgeKindContains:
		// Contains surface absorbs DEFINED_IN per edge_kind_surface.go.
		return []string{"CONTAINS", "DEFINED_IN"}
	case EdgeKindOther:
		return []string{"OTHER"}
	case EdgeKindDefines:
		return []string{"DEFINES"}
	case EdgeKindImports:
		return []string{"IMPORTS"}
	case EdgeKindDataFlows:
		return []string{"DATA_FLOWS"}
	case EdgeKindHTTPCalls:
		return []string{"HTTP_CALLS"}
	case EdgeKindAsyncCalls:
		return []string{"ASYNC_CALLS"}
	case EdgeKindEmits:
		return []string{"EMITS"}
	case EdgeKindListensOn:
		return []string{"LISTENS_ON"}
	case EdgeKindSimilarTo:
		return []string{"SIMILAR_TO"}
	case EdgeKindSemanticallyRelated:
		return []string{"SEMANTICALLY_RELATED"}
	case EdgeKindHandles:
		return []string{"HANDLES"}
	case EdgeKindConfigures:
		return []string{"CONFIGURES"}
	case EdgeKindWrites:
		return []string{"WRITES"}
	case EdgeKindMemberOf:
		return []string{"MEMBER_OF"}
	case EdgeKindTests:
		return []string{"TESTS"}
	case EdgeKindFileChangesWith:
		return []string{"FILE_CHANGES_WITH"}
	case EdgeKindCrossImports:
		return []string{"CROSS_IMPORTS"}
	case EdgeKindCrossCalls:
		return []string{"CROSS_CALLS"}
	default:
		return nil
	}
}

// validSurfaceEdgeKinds is the set of acceptable input edge_kind strings.
// Used by handleValidateGraphEdge to validate the EdgeKind arg before any
// accessor I/O.
var validSurfaceEdgeKinds = map[string]EdgeKindSurface{
	string(EdgeKindCalls):               EdgeKindCalls,
	string(EdgeKindReferences):          EdgeKindReferences,
	string(EdgeKindImplements):          EdgeKindImplements,
	string(EdgeKindExtends):             EdgeKindExtends,
	string(EdgeKindHasType):             EdgeKindHasType,
	string(EdgeKindUsesType):            EdgeKindUsesType,
	string(EdgeKindContains):            EdgeKindContains,
	string(EdgeKindImports):             EdgeKindImports,
	string(EdgeKindDefines):             EdgeKindDefines,
	string(EdgeKindDataFlows):           EdgeKindDataFlows,
	string(EdgeKindHTTPCalls):           EdgeKindHTTPCalls,
	string(EdgeKindAsyncCalls):          EdgeKindAsyncCalls,
	string(EdgeKindEmits):               EdgeKindEmits,
	string(EdgeKindListensOn):           EdgeKindListensOn,
	string(EdgeKindSimilarTo):           EdgeKindSimilarTo,
	string(EdgeKindSemanticallyRelated): EdgeKindSemanticallyRelated,
	string(EdgeKindHandles):             EdgeKindHandles,
	string(EdgeKindConfigures):          EdgeKindConfigures,
	string(EdgeKindWrites):              EdgeKindWrites,
	string(EdgeKindMemberOf):            EdgeKindMemberOf,
	string(EdgeKindTests):               EdgeKindTests,
	string(EdgeKindFileChangesWith):     EdgeKindFileChangesWith,
	string(EdgeKindCrossImports):        EdgeKindCrossImports,
	string(EdgeKindCrossCalls):          EdgeKindCrossCalls,
	string(EdgeKindOther):               EdgeKindOther,
}

// tierContribution scales the per-tier confidence weight used by
// type_resolver citations. Higher tiers (more authoritative) contribute more.
func tierContribution(tier string) float64 {
	switch tier {
	case "tier1_lsp":
		return contributionTierTier1
	case "tier2_annotation":
		return contributionTierTier2
	case "tier3_constructor":
		return contributionTierTier3
	case "tier4_assignment":
		return contributionTierTier4
	case "tier5_godoc":
		return contributionTierTier5
	case "tier6_heuristic":
		return contributionTierTier6
	case "tier7_unknown":
		return contributionTierTier7
	default:
		return contributionTierTier7
	}
}

// isDegradedTier reports whether the given tier is at or below tier6_heuristic
// (triggers the TYPES-04 top-level cap).
func isDegradedTier(tier string) bool {
	return tier == "tier6_heuristic" || tier == "tier7_unknown"
}

// parseLSPMethod parses an edge.Source string of the form
// "lsp.{lang}.{lsp_method}" into the {lsp_method} suffix. Returns the empty
// string when the input does not match the documented LSP prefix shape.
func parseLSPMethod(source string) string {
	if !strings.HasPrefix(source, "lsp.") {
		return ""
	}
	rest := source[len("lsp."):]
	// Drop the `{lang}` segment if present.
	if dot := strings.IndexByte(rest, '.'); dot >= 0 {
		return rest[dot+1:]
	}
	return rest
}

// handleValidateGraphEdge is the testable handler body. See the help text for
// the full contract; the order of operations is load-bearing:
//  1. checkMode(modeTierRead) — every session passes; retained for code-review
//     visibility and Phase 66 GuardrailMiddleware precedent.
//  2. Validate args.EdgeKind against the closed surface enum.
//  3. Resolve from + to seeds via s.resolveSeed.
//  4. If either resolution is not_found → short-circuit with
//     fallback_reason="symbol_not_found".
//  5. Project surface EdgeKind → candidate internal kinds.
//  6. Query EdgeEvidenceAccessor.EvidenceForEdge → []EdgeEvidenceRow.
//  7. If accessor returns 0 rows AND the accessor is wired → fallback_reason
//     ="edge_not_found", evidence_status=none.
//  8. Assemble citations: one LSP citation per row with lsp.* source; one
//     AST citation per row carrying tree_sitter_kind metadata OR ASTAttested;
//     one type_resolver citation per row carrying a Tier value.
//  9. Sort evidence by ConfidenceContribution desc; cap at 10; record honest
//     totals.
//  10. Compute raw sum + evidence_status. Top-level confidence:
//     degraded → types.CapCommentConfidence (clamps to ≤ 0.6).
//  11. Assemble FreshnessV2 envelope (shared helper from 71-03).
//  12. Issue receipt; return jsonResult.
//
// HARD INVARIANT (D-09 / D-13): this function MUST NOT reach the snapshot-
// write surface of *Store nor the per-workspace compactor's flush trigger.
// The recorder mocks in tools_validate_edge_test.go fail loudly on any
// future regression that reaches them.
func (s *SemanticSkill) handleValidateGraphEdge(ctx context.Context, args ValidateGraphEdgeArgs) *mcpsdk.CallToolResult {
	// 1. Mode-tier check.
	snap := s.sessionSnapshot(ctx)
	if err := checkMode(snap, modeTierRead); err != nil {
		return errorResult(err.Error())
	}

	// 2. Validate edge_kind against the closed surface enum.
	surface, ok := validSurfaceEdgeKinds[args.EdgeKind]
	if !ok {
		return errorResult(
			serr.New(serr.InvalidArgs,
				fmt.Sprintf("edge_kind %q is not in the closed surface enum (calls/references/implements/extends/has_type/uses_type/contains/defines/imports/data_flows/http_calls/async_calls/emits/listens_on/similar_to/semantically_related/handles/configures/writes/member_of/tests/file_changes_with/cross_imports/cross_calls/other)",
					args.EdgeKind)).Error(),
		)
	}

	// 3. Resolve workspace + both seeds.
	ws := s.workspaceKey(ctx)
	repoID := ws.Hash()

	fromResolved, err := s.resolveSeed(ctx, ws, args.From)
	if err != nil {
		return errorResult(err.Error())
	}
	toResolved, err := s.resolveSeed(ctx, ws, args.To)
	if err != nil {
		return errorResult(err.Error())
	}

	// 4. Short-circuit when either seed resolved to not_found.
	if fromResolved.Resolution == ResolutionNotFound || toResolved.Resolution == ResolutionNotFound {
		return jsonResult(ValidateGraphEdgeResult{
			FromResolution: fromResolved.Resolution,
			ToResolution:   toResolved.Resolution,
			FromSymbolID:   string(fromResolved.SymbolID),
			ToSymbolID:     string(toResolved.SymbolID),
			Confidence:     0,
			Evidence:       []EvidenceCitation{},
			EvidenceStatus: EvidenceStatusNone,
			Freshness:      s.assembleFreshness(ctx, repoID),
			FallbackReason: "symbol_not_found",
		})
	}

	// 5. Project surface EdgeKind → candidate internal kinds.
	internalKinds := surfaceToInternalKinds(surface)

	// 6. Read per-edge evidence rows.
	s.mu.Lock()
	ev := s.edgeEvidence
	s.mu.Unlock()

	if ev == nil {
		// Seam unwired: degrade gracefully (D4 lenient) — evidence absent
		// but the handler still returns a structured response.
		return jsonResult(ValidateGraphEdgeResult{
			FromResolution: fromResolved.Resolution,
			ToResolution:   toResolved.Resolution,
			FromSymbolID:   string(fromResolved.SymbolID),
			ToSymbolID:     string(toResolved.SymbolID),
			Confidence:     0,
			Evidence:       []EvidenceCitation{},
			EvidenceStatus: EvidenceStatusNone,
			Freshness:      s.assembleFreshness(ctx, repoID),
			FallbackReason: "evidence_lookup_unavailable",
		})
	}

	rows, err := ev.EvidenceForEdge(ctx, repoID, fromResolved.SymbolID, toResolved.SymbolID, internalKinds)
	if err != nil {
		// Read-error: degrade gracefully with structured envelope; never
		// reach the snapshot-write surface (D-09).
		return jsonResult(ValidateGraphEdgeResult{
			FromResolution: fromResolved.Resolution,
			ToResolution:   toResolved.Resolution,
			FromSymbolID:   string(fromResolved.SymbolID),
			ToSymbolID:     string(toResolved.SymbolID),
			Confidence:     0,
			Evidence:       []EvidenceCitation{},
			EvidenceStatus: EvidenceStatusNone,
			Freshness:      s.assembleFreshness(ctx, repoID),
			FallbackReason: "evidence_lookup_unavailable",
		})
	}

	// 7. No rows at all → edge_not_found (closed enum).
	if len(rows) == 0 {
		return jsonResult(ValidateGraphEdgeResult{
			FromResolution: fromResolved.Resolution,
			ToResolution:   toResolved.Resolution,
			FromSymbolID:   string(fromResolved.SymbolID),
			ToSymbolID:     string(toResolved.SymbolID),
			Confidence:     0,
			Evidence:       []EvidenceCitation{},
			EvidenceStatus: EvidenceStatusNone,
			Freshness:      s.assembleFreshness(ctx, repoID),
			FallbackReason: "edge_not_found",
		})
	}

	// 8. Assemble citations per source class.
	citations := make([]EvidenceCitation, 0, len(rows)*2)
	astMetadataPartial := false
	hasDegradedTier := false
	edgeAttestedWithoutMetadata := false // D4 lenient: row exists but yields no citation

	for _, r := range rows {
		emitted := false

		if strings.HasPrefix(r.Source, "lsp.") {
			citations = append(citations, buildLSPCitation(r))
			emitted = true
		}

		if r.TreeSitterKind != "" {
			citations = append(citations, buildASTCitation(r, false))
			emitted = true
		} else if r.ASTAttested {
			citations = append(citations, buildASTCitation(r, true))
			astMetadataPartial = true
			emitted = true
		}

		if r.Tier != "" {
			citations = append(citations, buildTypeResolverCitation(r))
			if isDegradedTier(r.Tier) {
				hasDegradedTier = true
			}
			emitted = true
		}

		if !emitted {
			// Row attests edge presence but carries no citation metadata —
			// D4 lenient: yields a non-zero attestation contribution without
			// adding to the citation array (kept off the evidence list to
			// preserve the "no citation" invariant).
			edgeAttestedWithoutMetadata = true
		}
	}

	// 9. Sort citations by ConfidenceContribution desc; cap at 10.
	sort.SliceStable(citations, func(i, j int) bool {
		return citations[i].ConfidenceContribution > citations[j].ConfidenceContribution
	})
	totalCitations := len(citations)
	if len(citations) > validateEdgeEvidenceCap {
		citations = citations[:validateEdgeEvidenceCap]
	}

	// 10. Compute raw additive confidence sum + evidence_status.
	rawConfidence := 0.0
	hasLSP, hasAST, hasTR := false, false, false
	for _, c := range citations {
		rawConfidence += c.ConfidenceContribution
		switch c.Source {
		case EvidenceSourceLSP:
			hasLSP = true
		case EvidenceSourceAST:
			hasAST = true
		case EvidenceSourceTypeResolver:
			hasTR = true
		}
	}

	// D4 lenient: edge attested but no citations → assign a low attestation
	// confidence so the response surfaces edge existence; capped by TYPES-04
	// downstream.
	fallbackReason := ""
	if edgeAttestedWithoutMetadata && len(citations) == 0 {
		rawConfidence = 0.3 // attestation-only signal; capped to ≤ 0.6 below
		fallbackReason = "evidence_lookup_lagging"
	}

	evidenceStatus := EvidenceStatusComplete
	switch {
	case len(citations) == 0 && !edgeAttestedWithoutMetadata:
		evidenceStatus = EvidenceStatusNone
	case len(citations) == 0 && edgeAttestedWithoutMetadata:
		evidenceStatus = EvidenceStatusNone
	case astMetadataPartial || hasDegradedTier || !(hasLSP && hasAST && hasTR):
		evidenceStatus = EvidenceStatusPartial
	default:
		evidenceStatus = EvidenceStatusComplete
	}

	// 11. Compute final top-level confidence with TYPES-04 cap.
	//
	// D4 asymmetry: per-source contributions stay UNCLAMPED (preserved in the
	// citation array) so downstream agents can see the raw signal that
	// triggered the cap. Only the top-level confidence value is clamped.
	finalConfidence := rawConfidence
	if finalConfidence > 1.0 {
		finalConfidence = 1.0
	}
	if evidenceStatus != EvidenceStatusComplete || hasDegradedTier {
		// Use CapCommentConfidence with EvidenceComment to apply the canonical
		// 0.60 ceiling. The cap is symmetric across degraded paths (per
		// 71-03's stance): manual second clamp ensures the ceiling holds
		// regardless of whether the cap helper's EvidenceComment early-return
		// triggered.
		finalConfidence = types.CapCommentConfidence(finalConfidence, types.EvidenceComment)
		if finalConfidence > degradedConfidenceCeiling {
			finalConfidence = degradedConfidenceCeiling
		}
	}

	// 12. Assemble freshness + receipt; return.
	result := ValidateGraphEdgeResult{
		FromResolution:        fromResolved.Resolution,
		ToResolution:          toResolved.Resolution,
		FromSymbolID:          string(fromResolved.SymbolID),
		ToSymbolID:            string(toResolved.SymbolID),
		Confidence:            finalConfidence,
		Evidence:              citations,
		EvidenceCountTotal:    totalCitations,
		EvidenceCountReturned: len(citations),
		EvidenceStatus:        evidenceStatus,
		Freshness:             s.assembleFreshness(ctx, repoID),
		FallbackReason:        fallbackReason,
	}

	guardrails.IssueReceiptOnSuccess(ctx, guardrails.ClassContextGathered,
		guardrails.ContextGatheredScope{
			TargetSymbols:   []integ.SymbolID{fromResolved.SymbolID, toResolved.SymbolID},
			TaskHash:        string(fromResolved.SymbolID) + "->" + string(toResolved.SymbolID),
			TokenBudgetUsed: len(citations),
			MaxTokens:       validateEdgeEvidenceCap,
		}, "validate_graph_edge")
	return jsonResult(result)
}

// buildLSPCitation shapes a row's LSP component into an EvidenceCitation.
func buildLSPCitation(r EdgeEvidenceRow) EvidenceCitation {
	return EvidenceCitation{
		Source:                 EvidenceSourceLSP,
		LSPMethod:              parseLSPMethod(r.Source),
		File:                   r.File,
		Range:                  r.Range,
		ConfidenceContribution: contributionLSP,
	}
}

// buildASTCitation shapes a row's AST component into an EvidenceCitation.
// When partialMetadata is true the citation emits empty TreeSitterKind and the
// caller drops envelope evidence_status to "partial".
func buildASTCitation(r EdgeEvidenceRow, partialMetadata bool) EvidenceCitation {
	c := EvidenceCitation{
		Source:                 EvidenceSourceAST,
		File:                   r.File,
		Range:                  r.Range,
		ConfidenceContribution: contributionASTComplete,
	}
	if !partialMetadata {
		c.TreeSitterKind = r.TreeSitterKind
	}
	if partialMetadata {
		c.ConfidenceContribution = contributionASTPartial
	}
	return c
}

// buildTypeResolverCitation shapes a row's type-resolver component into an
// EvidenceCitation, scaling the contribution by ladder tier.
func buildTypeResolverCitation(r EdgeEvidenceRow) EvidenceCitation {
	return EvidenceCitation{
		Source:                 EvidenceSourceTypeResolver,
		Tier:                   r.Tier,
		EvidenceKind:           r.EvidenceKind,
		File:                   r.File,
		Range:                  r.Range,
		ConfidenceContribution: tierContribution(r.Tier),
	}
}
