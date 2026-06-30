---
author: architect
responsible: architect
phase: 123
phase_type: implementation
hard_bar: true
security_relevant: false
design_fork: false
status: in_progress
parent_artifacts:
  - .planning/milestones/v2.8-ROADMAP.md
  - .planning/REQUIREMENTS.md
  - agent://RedTeamV28
---

# Phase 123 CONTEXT — Emission + Distinctness + Read Surface

## Contract (Hoare frame)

**{P}** Every fingerprintable symbol carries an RI `ContextVec` (Phase 122),
collected onto `fingerprintedNode` in the batch.

**{S}** Emit `SEMANTICALLY_RELATED` edges between vocabulary-cosine-similar
function bodies; prove the edge is DISTINCT from SIMILAR_TO and SELECTIVE on real
bodies; prove it is READABLE from a `helix` verb.

**{Q}** The kind goes 0 producers → ≥1; on real Go, vocabulary-cosine and
structural-Jaccard give orthogonal verdicts and unrelated bodies draw no edge;
the edge surfaces through `explain-symbol-deep`.

## Red-team-hardened design (folded from agent://RedTeamV28)

The independent pre-mortem returned **PROCEED-WITH-FIXES** with 3 blockers +
3 majors. All folded:

- **B1 (read surface) — the catch I was blind to.** My original acceptance named
  `validate-graph-edge` + `get-change-impact-graph`. Both *structurally cannot*
  surface a new edge kind: change-impact runs the hardcoded `call_graph`
  projection and `bfsExpand` rewrites every edge `Kind` to `"calls"`;
  validate-edge's `SetEdgeEvidence` is deferred to Phase 75. **Fix:** the real
  reader is `explain-symbol-deep` (no kind filter; `shapeEdges` maps via
  `MapInternalKind`). Proven by `TestShapeEdges_SurfacesSemanticallyRelated`.
- **B2 (determinism).** Already correct in the build (int32 accumulation, not
  float sum) — but the requirement text said "seeded" and the test was a same-
  input re-run (passes by luck). **Fix:** RELATE-01b now mandates integer
  accumulation + no map-iteration; `TestDeterminism_NShuffles` proves it over 50
  permutations.
- **M1 (boilerplate saturation) — the deepest catch.** Without down-weighting,
  RI cosine is dominated by `err/ctx/nil/req/...` and the edge degenerates to a
  near-complete graph while a curated divergence guard stays green = theater.
  **Fix:** `relatedidx/stopwords.go` drops keywords + ubiquitous plumbing
  identifiers + 1-char/numeric fragments; the guard now asserts SELECTIVITY
  (unrelated pair → NO edge), not just divergence.
- **M2 (risk retired too late).** Real-body discrimination now lives in the
  guard, exercised at Phase 123 against measured thresholds (engine math, no LSH
  flake).
- **M3 (uncalibrated threshold).** Measured on real bodies: unrelated ≈ 0.00,
  shared-domain ≈ 0.80. `semanticRelatedThreshold = 0.55` pinned in the gap with
  a rationale comment citing the measurement test.
- **m1/m2 (compute cliff + rune hashing).** O(n²) avoided via SimHash LSH
  (`relatedidx/lsh.go`, 16×4 bands for ~0.9997 recall at cos 0.8); hash-seed
  derivation hardened to `binary.LittleEndian`. ContextVec is build-transient
  (never persisted) — the no-migration claim holds.

## Distinctness — the measured 2×2 (the milestone's whole point)

| Pair | Jaccard (structure) | Cosine (vocab) | Verdict |
|---|---|---|---|
| same-struct / disjoint-vocab | 1.000 → SIMILAR_TO | 0.000 | not related ✅ |
| diff-struct / shared-vocab | 0.000 | 0.804 → RELATED | not similar ✅ |
| unrelated | — | −0.019 | not related ✅ |

Orthogonal by construction (MinHash erases vocabulary; RI erases structure) and
selective in aggregate (stop-list defeats boilerplate). Both are required: the
first proves the edge is a *new* signal, the second proves it isn't *noise*.

## Acceptance (verifiable)

1. `semanticallyRelatedEdges` emits `SEMANTICALLY_RELATED`/`Source: random_index`, wired into `factsFromExtracted` after similar/dataflow. ✓
2. Emission e2e on real Go: shared-vocabulary functions link with valid distinct NodeIDs. ✓ (`TestFactsFromExtracted_E2E_SemanticallyRelated`)
3. Distinctness + selectivity guard green on real bodies. ✓ (`TestRelatedness_DistinctAndSelective`)
4. Read surface: SEMANTICALLY_RELATED surfaces through `explain-symbol-deep`'s shapeEdges unfiltered. ✓ (`TestShapeEdges_SurfacesSemanticallyRelated`)
5. Determinism: 50-shuffle byte-identical. ✓ (`TestDeterminism_NShuffles`)
6. Full sweep green; `make vet` clean; `go.mod` unchanged.

## Out of scope
Workstream B (true DATA_FLOWS); a dedicated SEMANTICALLY_RELATED reranker verb;
the get-change-impact-graph multi-projection rework (milestone-sized).
