---
author: architect
responsible: architect
phase: 123
milestone: v2.8
status: passed
verified: 2026-06-30
parent_artifacts:
  - .planning/phases/123-emission-distinctness-read-surface/SUMMARY.md
---

# VERIFICATION — v2.8 Workstream A

**Status:** passed (verified by re-running the suite + measuring code reality,
not trusting the summary). Substrate note: standing-team/SMTC-Decision tier
absent this session → claims below are Structural/grep-tier + direct test
execution, honestly graded.

## Success criteria (from REQUIREMENTS.md)

| # | Criterion | Result | Evidence |
|---|---|---|---|
| 1 | SEMANTICALLY_RELATED emitted on real Go (0 → ≥1 producers) | ✓ | `TestFactsFromExtracted_E2E_SemanticallyRelated` PASS — edge with valid distinct NodeIDs, `Source=random_index` |
| 2 | Distinctness guard green + mutation-confirmed RED | ✓ | `TestRelatedness_DistinctAndSelective` PASS; stop-list-removed mutation → PAIR3 cosine 0.686 ≥ 0.55 → FAIL (RED), reverted → GREEN |
| 3 | RI vectors deterministic | ✓ | `TestDeterminism_NShuffles` (50 permutations byte-identical); int32 accumulation |
| 4 | Zero new Go deps | ✓ | `git diff --stat go.mod go.sum` empty |
| 5 | No schema migration | ✓ | ContextVec is transient `SymbolFact` field; SEMANTICALLY_RELATED already in surface enum |
| 6 | build/test/vet/gofmt clean | ✓ | `go build ./...` clean; 44 pkg ok; `make vet` (8 vettools) clean; gofmt -l empty |
| 7 | Reachable from a `helix` verb | ✓ | `TestShapeEdges_SurfacesSemanticallyRelated` PASS — `explain-symbol-deep` surfaces it unfiltered (B1 correction) |

## Measured distinctness 2×2 (real parsed Go bodies)

```
PAIR1 same-struct/disjoint-vocab: Jaccard=1.000 Cosine=0.000  → SIMILAR_TO, not RELATED
PAIR2 diff-struct/shared-vocab  : Jaccard=0.000 Cosine=0.804  → RELATED, not SIMILAR_TO
PAIR3 unrelated (boilerplate)   : Cosine=-0.019               → neither (selective)
```

Orthogonal by construction + selective in aggregate. Threshold 0.55 sits in the
gap between unrelated (~0.0) and related (~0.8).

## Mutation confirmation (anti-vacuity, repo discipline)

The selectivity guard is non-vacuous — proven by toggling the discriminator:

```
stop-list ON  : PAIR3 cosine 0.137 (boilerplate-saturated fixture) → guard GREEN
stop-list OFF : PAIR3 cosine 0.686 ≥ 0.55 → guard RED ("boilerplate saturation")
```

The fixture is deliberately boilerplate-saturated (ctx/err/nil/return/result) so
removing the stop-list actually flips it — a domain-rich fixture would NOT catch
the regression (the trap the red-team's M1 flagged).

## Gaps

None blocking Workstream A. Workstream B (true DATA_FLOWS) deferred to the B0
design-fork gate — explicit, not silent.
