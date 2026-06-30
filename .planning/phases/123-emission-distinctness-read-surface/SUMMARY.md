---
author: architect
responsible: architect
phase: 123
milestone: v2.8
status: complete
parent_artifacts:
  - .planning/phases/123-emission-distinctness-read-surface/123-CONTEXT.md
  - agent://RedTeamV28
---

# SUMMARY — v2.8 Workstream A (Phases 121-123): SEMANTICALLY_RELATED

## Outcome

The one declared-but-**unwired** cbm-mcp edge kind is now **emitted, distinct,
selective, deterministic, and readable**. The kind went from **0 producers → ≥1**
via Random Indexing — cbm-mcp's own mechanism, pure-Go, zero new deps, no schema
migration.

## What shipped

**Phase 121 — RI engine (`internal/semantic/relatedidx/`, leaf pkg):**
- `relatedidx.go`: `ComputeContextVector` (sparse random projection over
  identifier/comment vocabulary), `Vector [256]int32` (integer accumulation =
  exact, order-independent determinism), `Cosine`, `ComputeVector` (AST path),
  `seededHash` (binary-encoded, not `string(rune())`).
- `stopwords.go`: curated cross-language keyword + plumbing-identifier stop-list
  (the M1 anti-saturation fix) + length/numeric filtering.
- `lsh.go`: SimHash random-hyperplane LSH (16×4 bands) for bounded candidate
  retrieval (the O(n²) fix).
- 12 unit tests incl. N-shuffle determinism, keepToken, LSH retrieval, cosine
  discrimination.

**Phase 122 — plumbing (`SymbolFact.ContextVec`):**
- Field added to `extract/fact.go`; computed in the shared `FingerprintBody`
  seam → all 11 providers inherit it with no per-provider edit; carried on the
  daemon `fingerprintedNode`.
- `TestContextVec_PopulatedForFunction_NilForContainer` (real Go AST).

**Phase 123 — emission + guards:**
- `semanticallyRelatedEdges` (`semantic_similarity_edges.go`): SimHash-LSH
  candidates + exact-cosine recheck ≥ `semanticRelatedThreshold` (0.55,
  calibrated), fan-out capped, deterministic; wired into `factsFromExtracted`.
- `TestRelatedness_DistinctAndSelective` — the anti-vacuity guard (real bodies).
- `TestFactsFromExtracted_E2E_SemanticallyRelated` — emission e2e.
- `TestShapeEdges_SurfacesSemanticallyRelated` — read-surface proof.

## Red-team (agent://RedTeamV28) — PROCEED-WITH-FIXES, all 6 folded

- **B1 (blocker, the catch I was blind to):** original read-surface acceptance
  named two tools that structurally cannot surface the edge
  (`get-change-impact-graph` hardcodes `Kind:"calls"` + `call_graph` projection;
  `validate-graph-edge` evidence wiring deferred to Phase 75). Retargeted to
  `explain-symbol-deep` (no kind filter) + corrected REQUIREMENTS/scope/RESUME.
- **B2 (determinism):** code was already correct (int32, not float sum); fixed
  the requirement text + strengthened to a 50-shuffle test.
- **M1 (boilerplate saturation, deepest):** added the stop-list; guard now asserts
  SELECTIVITY, **mutation-confirmed** — without the stop-list, boilerplate-heavy
  unrelated bodies link (cosine 0.69 > 0.55) → guard RED.
- **M2:** real-body discrimination pulled into the guard.
- **M3:** threshold calibrated on measured bodies (unrelated ≈0.0, related ≈0.8),
  pinned 0.55 with rationale.
- **m1/m2:** O(n²) → SimHash LSH; rune-cast hashing → binary encoding. ContextVec
  is build-transient (no persistence, no migration).

## Verification

- `go build ./...` clean; `go test ./internal/semantic/... ./internal/daemon/...
  ./internal/skill/semantic/...` → **44 pkg ok**; `make vet` (8 vettools) clean;
  gofmt clean; **`go.mod`/`go.sum` byte-unchanged**.
- Measured distinctness 2×2 + selectivity (see VERIFICATION.md), mutation-confirmed.

## Deferred (explicit, not silent)

- **Workstream B — true interprocedural DATA_FLOWS** — gated on the B0 design
  fork (rename structural edge vs Source-discriminate). Co-driver decision
  required before roadmapping. NOT in v2.8's committed scope.
- A dedicated SEMANTICALLY_RELATED reranker verb / `get-change-impact-graph`
  multi-projection rework — follow-ups; the edge is already readable via
  `explain-symbol-deep`.
- IDF (vs the curated stop-list) — possible future refinement.
