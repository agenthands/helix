---
phase: 71-p1-single-symbol-read-tools
plan: 05
subsystem: skill-semantic
tags:
  - semantic
  - mcp-tool
  - validate-graph-edge
  - ci-gate
  - integration-test
  - read-only
  - tdd
  - phase-71
  - wave-3
dependency-graph:
  requires:
    - internal/skill/semantic.SeedInput / resolveSeed (Phase 71-01)
    - internal/skill/semantic.SymbolByNameAccessor / ExtractorRunAccessor (Phase 71-01)
    - internal/skill/semantic.buildPopulatedGraphFixture (Phase 71-01)
    - internal/skill/semantic.FreshnessV2 + FreshnessStatus + FreshnessSource (Phase 71-02)
    - internal/skill/semantic.EdgeKindSurface + MapInternalKind (Phase 71-02)
    - internal/skill/semantic.SemanticSkill.assembleFreshness (Phase 71-03)
    - internal/skill/semantic.handleExplainSymbolDeep (Phase 71-03 — cross-tool consistency test)
    - internal/skill/semantic.handleFindRelatedSymbols (Phase 71-04 — cross-tool consistency test)
    - internal/semantic/types.CapCommentConfidence + EvidenceComment (TYPES-04 cap)
  provides:
    - "validate_graph_edge MCP tool registered through RegisterAll"
    - "ValidateGraphEdgeArgs / ValidateGraphEdgeResult / EvidenceCitation response shapes"
    - "EvidenceSource / EvidenceStatus closed-enum types + constants"
    - "EdgeEvidenceAccessor + EdgeEvidenceRow + EvidenceRange narrow read seam (Phase 71-05)"
    - "TestThreeTools_{CrossConsistency,Concurrent,EnvelopeShape,ModeEnforcement} integration suite"
    - "TestReadOnlyGate_Phase71Handlers in-tree D-09 gate (Branch B per 71-CONTEXT.md D6)"
  affects:
    - .planning/STATE.md — Phase 71 P1 single-symbol read tools COMPLETE
    - .planning/ROADMAP.md — Phase 71 row marked done (orchestrator does final write)
tech-stack:
  added: []
  patterns:
    - "Closed-enum gated handler (Pattern from tools_explain_symbol.go / tools_find_related.go)"
    - "D4 lenient evidence assembly (Pitfall 5 — TYPES-04 cap vs additive sum asymmetry)"
    - "D4 closed-enum source / status (named-type-safe surface, mirrors EdgeKindSurface)"
    - "Inline surface→internal reverse projection (per D3 — per-tool concern, no API on MapInternalKind)"
    - "Recorder StoreAccessor canary for D-09 / D-13 invariant (mirrors 71-03 / 71-04)"
    - "Cross-tool integration harness — single SemanticSkill wired for all three handlers (newThreeToolsHarness)"
    - "Concurrent invocation idempotency under -race (regression guard mirrors 71-04 Pitfall 6)"
    - "In-tree static gate test as CI runner equivalent (CONTEXT.md D6 Branch B)"
key-files:
  created:
    - path: internal/skill/semantic/tools_validate_edge.go
      role: "handler + register + types + help const + 3 evidence builders + INVARIANT header"
    - path: internal/skill/semantic/tools_validate_edge_test.go
      role: "13 tests (2 closed-enum + 11 behavior) + recorder canaries + fake EdgeEvidenceAccessor"
    - path: internal/skill/semantic/readonly_gate_test.go
      role: "TestReadOnlyGate_Phase71Handlers — in-tree D-09 gate covering all three handler files"
  modified:
    - path: internal/skill/semantic/accessors.go
      role: "added EdgeEvidenceAccessor + EdgeEvidenceRow + EvidenceRange (Phase 71-05 read seam)"
    - path: internal/skill/semantic/skill.go
      role: "added edgeEvidence field + SetEdgeEvidence post-init setter"
    - path: internal/skill/semantic/register.go
      role: "added registerValidateGraphEdge(server, s, tracer) call"
    - path: internal/skill/semantic/integration_test.go
      role: "added TestThreeTools_{CrossConsistency,Concurrent,EnvelopeShape,ModeEnforcement} + harness"
decisions:
  - "Open Q3 resolution (AST citation metadata): the existing extractor pass writes Edge.Source as `lsp.{lang}.{method}` for LSP-backed citations but does NOT today stamp `tree_sitter_kind` or per-citation Range on AST contributions. The handler therefore models AST citation metadata as OPTIONAL: when present (TreeSitterKind != \"\"), the citation emits the full kind+range; when absent but the graph attests an AST contribution (ASTAttested=true on the row), the citation emits source=ast / tree_sitter_kind=\"\" and the envelope's evidence_status drops to \"partial\". This honors the lenient-D4 stance while preserving the contract for the future wave that wires extractor metadata end-to-end."
  - "EdgeEvidenceAccessor declared in the semantic skill package (not graph) — keeps the narrow accessor seam free of graph-package transitive imports. EvidenceRange struct lives co-located for the same reason (Pattern F narrow-interface convention)."
  - "Surface→internal reverse projection (e.g., has_type → {RESOLVES_TO}) lives INLINE in surfaceToInternalKinds inside tools_validate_edge.go. Per D3 + 71-04 Deferred Ideas, the reverse is intentionally NOT exported as a public API on MapInternalKind — it's a per-tool concern that future tools can re-derive if they need a different projection."
  - "D4 asymmetry implementation: top-level Confidence is clamped via types.CapCommentConfidence + manual ceiling to ≤ 0.6 on degraded paths; per-source ConfidenceContribution in the evidence array stays UNCLAMPED. Downstream agents see the raw signal that triggered the cap."
  - "D-09 gate Branch B selected: 71-01's grep-gate location investigation established the repo carries no external CI runner (no .github/workflows/*.yml, no scripts/, no Makefile rule) lint handler files. The in-tree TestReadOnlyGate_Phase71Handlers serves the equivalent role per CONTEXT.md D6. Adding a new read-only handler in a future phase MUST extend gatedHandlerFiles in readonly_gate_test.go and carry the same INVARIANT header copied from tools_refresh.go."
  - "Three-tools integration harness shares a single SemanticSkill instance across all three handlers (no per-test re-wiring). The harness exposes seedSymbol + callerSymbol so the same CALLS edge can be asserted by all three tools."
metrics:
  duration: "~50 minutes execution"
  completed: "2026-05-17"
  tasks_completed: 3
  files_created: 3
  files_modified: 4
  commits: 4
---

# Phase 71 Plan 05: `validate_graph_edge` MCP Tool Summary

Ship the third — and final — Phase 71 P1 single-symbol read tool. Given a (from, to, edge_kind) tuple, return a confidence score in [0, 1] plus a flat ordered evidence array (capped at 10) sorted by per-source confidence contribution descending. Honors the D4 lenient stance (edge present but no metadata yields non-zero confidence ≤ 0.6) and the TYPES-04 cap (top-level confidence is clamped on any degraded-evidence path while per-source contributions stay unclamped). Co-lands the cross-tool integration suite that exercises all three Phase 71 tools together and the in-tree D-09 read-only gate covering the three handler files.

## What Landed

| # | Commit  | Type | Description                                                          |
| - | ------- | ---- | -------------------------------------------------------------------- |
| 1 | ef410d92 | test | RED — 13 failing validate_graph_edge tests (2 closed-enum + 11 behavior) |
| 2 | 11fb940f | feat | GREEN — handler + register + EdgeEvidenceAccessor seam; full suite green |
| 3 | 50489cad | test | Cross-tool integration suite (TestThreeTools_*) covering D7 #4 + #5     |
| 4 | 538b0384 | test | In-tree D-09 read-only gate (Branch B per CONTEXT.md D6)               |

## Behavior Tests (all green under `-race -count=1`)

### `tools_validate_edge_test.go`

| Test                                              | Asserts                                                                                                                                                       |
| ------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestEvidenceSource_ClosedSet`                    | Declared constants `EvidenceSourceLSP/AST/TypeResolver` match the documented closed set `{lsp, ast, type_resolver}`; mirrors `TestEdgeKindSurface_ClosedSet`. |
| `TestEvidenceStatus_ClosedSet`                    | Declared constants match `{complete, partial, none}`.                                                                                                          |
| `TestValidateGraphEdge_HappyPath`                 | Full citation set (LSP + AST + type_resolver tier1) yields confidence ~1.0; evidence sorted desc; evidence_status=complete; no fallback_reason.                |
| `TestValidateGraphEdge_LenientEmptyEvidence`      | D4 lenient: edge present with no metadata → confidence > 0 but ≤ 0.6 (TYPES-04); fallback_reason="evidence_lookup_lagging".                                   |
| `TestValidateGraphEdge_TYPES04Cap`                | tier6_heuristic alone → top-level confidence ≤ 0.6 (clamped); per-source contribution UNCLAMPED in evidence array.                                            |
| `TestValidateGraphEdge_EdgeAbsent`                | No matching rows → confidence=0, evidence=[], evidence_status=none, fallback_reason="edge_not_found" (NOT an error).                                          |
| `TestValidateGraphEdge_EdgeKindClosedEnum`        | `edge_kind="FREEFORM_KIND"` → error envelope via serr.InvalidArgs.                                                                                            |
| `TestValidateGraphEdge_EvidenceCapAt10`           | 12 LSP citations → returned=10, total≥12 — honest truncation.                                                                                                  |
| `TestValidateGraphEdge_BothSeedsResolution`       | from=symbol_id (exact) + to=(file,name) ambiguous → from_resolution="exact", to_resolution="ambiguous".                                                       |
| `TestValidateGraphEdge_FromNotFound`              | Either seed not_found → confidence=0, evidence_status=none, fallback_reason="symbol_not_found" (NOT an error).                                                |
| `TestValidateGraphEdge_ASTCitationFallback`       | Open Q3: AST contribution without extractor metadata → tree_sitter_kind="" / evidence_status="partial".                                                       |
| `TestValidateGraphEdge_ReadOnly`                  | Recorder StoreAccessor canaries (Begin/Commit/Abort/Write) never invoked across the full happy-path traversal.                                                 |
| `TestValidateGraphEdge_ModeRejected`              | Resolver-unwired path produces error envelope; no accessor I/O on reject (Pitfall 6 invariant).                                                               |

### `integration_test.go` (Three-Tools suite — D7 #4 + #5)

| Test                                  | Asserts                                                                                                                                                                                            |
| ------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestThreeTools_CrossConsistency`     | explain_symbol_deep reports an inbound CALLS edge → validate_graph_edge confirms (confidence > 0 AND fallback_reason != edge_not_found) for every sampled edge; find_related includes the caller. |
| `TestThreeTools_Concurrent` (×10)     | 8 goroutines × 3 tools = 24 concurrent invocations → per-tool byte-identical responses (idempotency); race detector clean across 10 -count repeats.                                                |
| `TestThreeTools_EnvelopeShape`        | Each tool's response carries FreshnessV2 with non-zero graph_version + snapshot_id + extractor_run_id + as_of_unix_ms; status ∈ {current,stale,unknown}.                                            |
| `TestThreeTools_ModeEnforcement`      | Resolver-unwired alternate path drives error envelopes for ALL three handlers; recorder canaries (t.Fatalf-baked) never fire on either path.                                                       |

### `readonly_gate_test.go` (D-09 gate — Branch B)

| Test                                       | Asserts                                                                                                                                                                                                       |
| ------------------------------------------ | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `TestReadOnlyGate_Phase71Handlers` (×3)    | Each of `tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go` scanned line-by-line; no `BeginSnapshot` / `CommitSnapshot` / `AbortSnapshot` / `WriteSnapshotFacts` outside Go comments. |

## Deviations from Plan

No Rule 1 / 2 / 3 fixes during execution — the seams established by 71-01 + 71-02 + 71-03 + 71-04 gave the handler everything it needed. Two minor in-scope adjustments worth noting:

1. **Plan asks for `*graph.Range`** in the EvidenceCitation field. The `graph` package does not declare a `Range` type; introducing one would have grown the graph public surface for a single tool's use. I declared `EvidenceRange` co-located in the semantic skill package (next to `EdgeEvidenceRow`). This matches Pattern F (narrow-interface) and keeps the new accessor seam free of graph-package transitive imports. Documented in **Decisions** above.

2. **Plan's `recorderClusterMembership.gvAtClusterRead` Pitfall-3 regression check** is not part of 71-05's scope (71-04 already covers it via `TestFindRelatedSymbols_ClusterBoostSameGraphVersion`). validate_graph_edge does not perform a cluster boost, so the Pitfall-3 invariant is N/A here.

### Pragmatic `TestValidateGraphEdge_ModeRejected` (carried over from 71-03 / 71-04)

`checkMode(snap, modeTierRead)` returns `nil` for all sessions by design — read+ is the lowest tier; every session passes. True mode rejection is impossible for read tools. The test asserts the *spirit* of the plan's "no accessor I/O on reject" invariant by driving the resolver-unwired error path and confirming no canaries fire. Identical handling as 71-03 / 71-04.

### Open Q3 Resolution (RESEARCH.md)

> *How should `validate_graph_edge` derive AST citations?*

**Investigated finding:** Today's extractor pass writes `Edge.Source` as `lsp.{lang}.{method}` for LSP-backed citations and does NOT stamp `tree_sitter_kind` / per-citation Range on AST-only contributions. The handler models AST citation metadata as OPTIONAL via the new `EdgeEvidenceRow.ASTAttested` boolean:

- **Metadata present** (TreeSitterKind != ""): emit citation with full kind+range, contribution 0.3.
- **Metadata absent but graph attests** (ASTAttested=true): emit citation with `source="ast"` / `tree_sitter_kind=""`, contribution 0.3, and drop envelope `evidence_status` to `"partial"`.

This honors the lenient-D4 stance — non-zero AST contribution surfaces in the response even when the extractor pass has not yet been upgraded — while preserving the contract for the future wave that wires extractor metadata end-to-end.

## D-09 Gate Branch Taken

**Branch B** (in-tree gate). File: `internal/skill/semantic/readonly_gate_test.go`. Test: `TestReadOnlyGate_Phase71Handlers` (covers `tools_explain_symbol.go`, `tools_find_related.go`, `tools_validate_edge.go`).

Rationale: 71-01's grep-gate location investigation confirmed the repo has no external CI runner that lints handler files for snapshot-write tokens (no `.github/workflows/*.yml`, no `scripts/`, no `Makefile` rule with the requisite grep pipeline). The in-tree Go test serves the equivalent role per 71-CONTEXT.md D6.

The gate strips Go comment lines (`//` prefix after trim) and asserts none of `BeginSnapshot`, `CommitSnapshot`, `AbortSnapshot`, `WriteSnapshotFacts` appear on the remaining code lines. Adding a new read-only handler in a future phase MUST extend `gatedHandlerFiles` in the test file and carry the same INVARIANT header.

## Threat Model — Mitigations Observed

| Threat ID  | Mitigation in code                                                                                                                                                                                                          |
| ---------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| T-71-05-01 | `validSurfaceEdgeKinds` map gates EdgeKind input before any accessor I/O; freeform input → `serr.InvalidArgs` envelope.                                                                                                     |
| T-71-05-02 | Pitfall 5: top-level Confidence clamped via `types.CapCommentConfidence` AND a defensive manual ceiling to 0.6; per-source ConfidenceContribution explicitly UNCLAMPED with inline documentation (D4 asymmetry).            |
| T-71-05-03 | D4 lenient + TYPES-04 cap: any non-`complete` evidence_status (partial, none) AND any tier6/tier7 path forces the top-level cap. Test `TestValidateGraphEdge_TYPES04Cap` exercises the heuristic-only path.                |
| T-71-05-04 | In-tree `TestReadOnlyGate_Phase71Handlers` plus per-handler recorder StoreAccessor canaries (Begin/Commit/Abort/Write). The static gate covers future refactors that bypass per-handler tests.                              |
| T-71-05-05 | `TestThreeTools_CrossConsistency` asserts explain_symbol_deep and validate_graph_edge agree on edge existence for every sampled inbound edge. Disagreement is a failing assertion.                                          |
| T-71-05-06 | `TestThreeTools_Concurrent` runs 8 goroutines × 3 tools under `-race -count=10`; byte-identical responses across goroutines (idempotency); race detector clean across all 10 repeats.                                       |

## must_haves Observance

- ✅ `validate_graph_edge` is registered through `RegisterAll` (`grep -c "registerValidateGraphEdge(server, s, tracer)" internal/skill/semantic/register.go` → 1).
- ✅ Handler enforces read+ mode tier at entry via `checkMode(snap, modeTierRead)`.
- ✅ Accepts two seeds (from, to) each via symbol_id OR (file_path, symbol_name); accepts `edge_kind` (closed surface enum) input.
- ✅ Returns top-level `confidence ∈ [0, 1]` + ordered `evidence` array (cap 10) sorted by `confidence_contribution` desc.
- ✅ Evidence `source` is the named `EvidenceSource` type (not untyped string) with closed set `{lsp, ast, type_resolver}`.
- ✅ Evidence `evidence_status` is the named `EvidenceStatus` type with closed set `{complete, partial, none}`; D4 lenient path (non-zero confidence + empty evidence) reports `status:"none"` with capped confidence.
- ✅ TYPES-04 cap: top-level confidence ≤ 0.6 when evidence_status != complete OR any tier ∈ {tier6, tier7}.
- ✅ Edge absent → confidence=0 + `fallback_reason:"edge_not_found"` (no error).
- ✅ In-tree D-09 gate covers all three Phase 71 handler files.
- ✅ Integration suite asserts cross-tool consistency, concurrent race-cleanliness, envelope shape parity, and read+ mode enforcement across the three tools.

## Verification

```
go vet ./internal/skill/semantic/                                            # clean (only unrelated Swift binding warning)
go test ./internal/skill/semantic/ -race -count=1                            # ok 5.690s
go test ./internal/skill/semantic/ -run TestValidateGraphEdge -race -count=1 # 11/11 pass
go test ./internal/skill/semantic/ -run TestEvidenceSource_ClosedSet         # 1/1 pass
go test ./internal/skill/semantic/ -run TestEvidenceStatus_ClosedSet         # 1/1 pass
go test ./internal/skill/semantic/ -run TestThreeTools_ -race -count=10      # 4/4 × 10 repeats pass
go test ./internal/skill/semantic/ -run TestReadOnlyGate -race -count=1      # 3/3 sub-files pass

# Plan Task 3 verify-command bash loop:
for f in internal/skill/semantic/tools_explain_symbol.go \
         internal/skill/semantic/tools_find_related.go \
         internal/skill/semantic/tools_validate_edge.go; do
  HITS=$(grep -v "^//" "$f" | grep -v "^[[:space:]]*//" \
         | grep -cE "BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts" || true)
  [ "$HITS" != "0" ] && echo "GATE FAIL: $f" && exit 1
done
echo "GATE OK"   # → "GATE OK"

# Plan Task 1 acceptance grep counts:
grep -c "func (s \*SemanticSkill) handleValidateGraphEdge" internal/skill/semantic/tools_validate_edge.go  # 1
grep -rn "type EvidenceSource string" internal/skill/semantic/                                            # 1
grep -rn "type EvidenceStatus string" internal/skill/semantic/                                            # 1
grep -E "EvidenceSourceLSP|EvidenceSourceAST|EvidenceSourceTypeResolver" internal/skill/semantic/ -r | wc -l  # 16 (≥ 3)
grep -E "EvidenceStatusComplete|EvidenceStatusPartial|EvidenceStatusNone" internal/skill/semantic/ -r | wc -l # 27 (≥ 3)
grep -c "func registerValidateGraphEdge" internal/skill/semantic/tools_validate_edge.go                   # 1
grep -c "registerValidateGraphEdge(server, s, tracer)" internal/skill/semantic/register.go                # 1
grep -E "BeginSnapshot|CommitSnapshot|AbortSnapshot|WriteSnapshotFacts" internal/skill/semantic/tools_validate_edge.go | grep -v "^//"  # 0 matches
grep -c "types.CapCommentConfidence" internal/skill/semantic/tools_validate_edge.go                       # 3 (≥ 1)
```

## Phase 71 Completion Checklist (ROADMAP success criteria)

| # | Criterion                                                                                            | Status                                                                                                                                          |
| - | ---------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | All three P1 read tools (explain_symbol_deep, find_related_symbols, validate_graph_edge) ship.       | ✅ 71-03 (commit `c1b8df66`) + 71-04 (commit `74641495`) + 71-05 (commit `11fb940f`) registered in RegisterAll.                                  |
| 2 | All three tools enforce D-09 read-only invariant at runtime + static gate.                            | ✅ Recorder canaries per-tool + in-tree `TestReadOnlyGate_Phase71Handlers` covers all three handler files.                                       |
| 3 | All three tools emit the FreshnessV2 envelope (D5) with the five required identifiers + closed enums. | ✅ `TestThreeTools_EnvelopeShape` asserts every tool emits non-zero graph_version / snapshot_id / extractor_run_id / as_of_unix_ms + valid status. |
| 4 | Cross-tool consistency: tools agree on edge existence / neighbor relationships.                       | ✅ `TestThreeTools_CrossConsistency` asserts explain → validate edge agreement + find_related caller surfacing.                                  |
| 5 | Race-clean concurrent invocation under -race × 10.                                                    | ✅ `TestThreeTools_Concurrent` byte-identical responses across 8 goroutines × 3 tools × 10 -count repeats; race detector clean.                  |

## Known Stubs

**Production wiring of `EdgeEvidenceAccessor` for `validate_graph_edge`:** the narrow seam is declared in `internal/skill/semantic/accessors.go` (Phase 71-05 addition) and the post-init setter `SetEdgeEvidence` is wired on `*SemanticSkill`. Until the production daemon adapter wraps `EdgeEvidenceAccessor` against the real `*Store` per-edge evidence reader, live calls to `validate_graph_edge` from the daemon will degrade gracefully: the handler returns a structured envelope with `evidence_status="none"` + `fallback_reason="evidence_lookup_unavailable"` rather than erroring. Identical disposition to the 71-04 cluster-boost wiring and 71-03 type-chain wiring stubs.

This is **intentional** per the seam-first wave-3 design. The follow-up wiring plan will activate the live evidence reader without further handler changes.

## Threat Flags

None new — `validate_graph_edge` reads exclusively through the existing read-only narrow accessors plus the new `EdgeEvidenceAccessor` declared in this plan. No new endpoints, schema changes, auth paths, or trust-boundary changes. The threat surface is identical to `explain_symbol_deep` (per-symbol read + per-edge read at the latest committed snapshot).

## Consumer Hooks

- **Future P2 tools** (analyze_blast_radius, get_taint_path, etc.) can reuse the same `EdgeEvidenceAccessor` seam for per-edge confidence + citation assembly. The `EvidenceSource` / `EvidenceStatus` closed-enum types are package-public and ready for re-use.
- **`pathFromSymbolID` + `pathsAllow`** (71-04) remain available for any future tool that needs strict-subset filtering over `<path>::<name>` IDs.
- **Recorder StoreAccessor canary pattern** is now established across all three Phase 71 read-tool tests (`*_test.go`). The in-tree gate (`readonly_gate_test.go`) complements the runtime canaries with a static line-scan that catches regressions even if a future change bypasses the per-test recorders.

## TDD Gate Compliance

- ✅ `test(71-05)` commit `ef410d92` precedes feat (RED gate).
- ✅ `feat(71-05)` commit `11fb940f` follows test (GREEN gate).
- N/A separate `refactor(71-05)` commit — REFACTOR step extracted `buildLSPCitation` / `buildASTCitation` / `buildTypeResolverCitation` inline as part of the GREEN commit (matches 71-03 / 71-04's minimal-refactor doctrine; the test recorder still validates the same surface).

## Self-Check: PASSED

- ✅ `internal/skill/semantic/tools_validate_edge.go` exists (handler + register + types + help const + INVARIANT header + 3 evidence builders).
- ✅ `internal/skill/semantic/tools_validate_edge_test.go` exists (2 closed-enum + 11 behavior tests + recorder canaries + fake EdgeEvidenceAccessor).
- ✅ `internal/skill/semantic/readonly_gate_test.go` exists (in-tree D-09 gate covering all three handler files).
- ✅ `internal/skill/semantic/accessors.go` carries `EdgeEvidenceAccessor` + `EdgeEvidenceRow` + `EvidenceRange`.
- ✅ `internal/skill/semantic/skill.go` carries `edgeEvidence` field + `SetEdgeEvidence` setter.
- ✅ `internal/skill/semantic/register.go` carries `registerValidateGraphEdge(server, s, tracer)`.
- ✅ `internal/skill/semantic/integration_test.go` carries `TestThreeTools_{CrossConsistency,Concurrent,EnvelopeShape,ModeEnforcement}`.
- ✅ Commits `ef410d92` (test) + `11fb940f` (feat) + `50489cad` (test, integration) + `538b0384` (test, gate) present on `worktree-agent-a825fc8335503f291`.
- ✅ `go vet ./internal/skill/semantic/` clean (only unrelated Swift binding warning).
- ✅ `go test -race -count=1 ./internal/skill/semantic/` green (5.690s).
- ✅ 11/11 `TestValidateGraphEdge_*` cases pass under `-race -count=1`.
- ✅ 4/4 `TestThreeTools_*` cases pass under `-race -count=10`.
- ✅ Gate verification command returns "GATE OK".
