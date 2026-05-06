---
phase: 62-graph-engine-ranking-type-resolution
plan: 01
subsystem: graph-engine
tags: [pagerank, determinism, generics, repomap-migration, hex-digest]
requires:
  - .planning/phases/62-graph-engine-ranking-type-resolution/62-CONTEXT.md
  - .planning/phases/62-graph-engine-ranking-type-resolution/62-PATTERNS.md
  - internal/repomap/pagerank.go (pre-migration body, lifted into engine)
provides:
  - internal/graph.PageRank[T cmp.Ordered]
  - internal/graph.RankNodes[T cmp.Ordered]
  - internal/graph.Options
  - internal/graph.Ranked[T cmp.Ordered]
affects:
  - internal/repomap/pagerank.go (rewritten as adapter)
tech_stack_added:
  - none (stdlib only — cmp, math, sort)
patterns_used:
  - sort-before-iterate (D-03 determinism patch)
  - thin-adapter delegation (FileGraph.PageRank → graph.PageRank)
  - golden hex-digest oracle (sha256 over %.17g serialization)
  - generics over cmp.Ordered for type-parametric engine
key_files:
  created:
    - internal/graph/pagerank.go
    - internal/graph/options.go
    - internal/graph/personalize.go
    - internal/graph/pagerank_test.go
    - internal/graph/testdata/pagerank/golden_uniform.txt
    - internal/graph/testdata/pagerank/golden_personalized.txt
    - internal/graph/testdata/pagerank/golden_tiebreak.txt
  modified:
    - internal/repomap/pagerank.go (rewrite — 150 LOC → 86 LOC)
decisions:
  - Single-node graph short-circuits to score=1.0 (matches existing repomap test fixture)
  - Personalize stored as map[any]float64 on Options (keeps Options non-generic; assert to T at call time)
  - Tiebreak via sort.SliceStable with secondary key node-asc (GRAPH-03 stable-key NodeID)
  - Adapter snapshot helper extracted to keep file < 100 LOC
metrics:
  duration: ~50 min
  completed: "2026-05-06T14:07:26Z"
  tasks_total: 4
  tasks_completed: 4
  files_created: 7
  files_modified: 1
  loc_engine: 265   # pagerank.go (130) + options.go (43) + personalize.go (92)
  loc_tests:  241   # pagerank_test.go
  loc_adapter: 86   # internal/repomap/pagerank.go (was 150)
---

# Phase 62 Plan 01: Generic Deterministic PageRank Engine + Repomap Adapter Summary

**One-liner:** Stdlib-only generic PageRank engine at `internal/graph` over `cmp.Ordered`, with sort-before-iterate determinism, sha256 golden-digest oracle, and a thin `internal/repomap` adapter that preserves `FileGraph` public API.

## Files Added

| Path | LOC | Provides |
| ---- | --- | -------- |
| `internal/graph/pagerank.go` | 130 | `PageRank[T cmp.Ordered]` — sort-once-at-entry power iteration with dangling-mass redistribution |
| `internal/graph/options.go` | 43 | `Options{Damping, Epsilon, MaxIter, Personalize}` + `withDefaults()` |
| `internal/graph/personalize.go` | 92 | `buildTeleport`, `RankNodes[T cmp.Ordered]`, `Ranked[T]` |
| `internal/graph/pagerank_test.go` | 241 | 6 tests — Deterministic / HexDigest / Personalized / TieBreak / Empty / Dangling + `digestScores` helper |
| `internal/graph/testdata/pagerank/golden_uniform.txt` | 1 | sha256 = `0f861905fdcae2c919222664173e4d299527ff6e9a7e187c015608ae849ab9db` |
| `internal/graph/testdata/pagerank/golden_personalized.txt` | 1 | sha256 = `b696723dffaa5f43e34ad90860cd42a7749aee1ddcd3c4e63a3b647ca36630bd` |
| `internal/graph/testdata/pagerank/golden_tiebreak.txt` | 1 | sha256 = `3c5453969f323a0989058287505fb78301655a5263ca1123309b5d7aa6511db9` |

## Files Modified

| Path | Change |
| ---- | ------ |
| `internal/repomap/pagerank.go` | Rewritten as thin adapter (150 → 86 LOC). `FileGraph.PageRank` and `FileGraph.RankFiles` snapshot under `g.mu.RLock()` then delegate to `graph.PageRank` / `graph.RankNodes`. `math.Abs` convergence loop removed (D-01 single-locus). |

## Test Results

| Suite | Result | Notes |
| ----- | ------ | ----- |
| `go test ./internal/graph/... -count=10 -run TestPageRank_Deterministic` | PASS | 10× determinism multiplier proves byte-equality across runs |
| `go test ./internal/graph/... -count=1` | PASS | All 6 tests green; goldens are pinned 64-char sha256 hex |
| `go test ./internal/repomap/... -count=1` | PASS | All 6 existing tests pass against migrated engine without vector edits — score VALUES preserved |
| `go vet ./...` | PASS | Only the preexisting Swift binding `TOKEN_COUNT` warning surfaces (out-of-scope) |
| `go test ./... -count=1` | PASS | Whole-repo green |
| D-01 invariant: `go list -deps ./internal/graph/... \| grep github.com/agenthands/helix \| grep -v 'internal/graph$'` | EMPTY | Zero project imports — stdlib only |

## Acceptance Criteria

### GRAPH-01 byte-equality
- `TestPageRank_HexDigest` passes against pinned `golden_uniform.txt`
- All goldens are 64-char sha256 hex (`awk '{print length($0)}'` returns `64`)
- `-count=10` runs of `TestPageRank_Deterministic` pass

### GRAPH-02 personalized PR
- `TestPageRank_Personalized` passes
- `Options.Epsilon = 1e-6` honored
- `RankNodes` returns desc-sorted slice with stable secondary key

### GRAPH-03 tiebreak
- `TestPageRank_TieBreak` proves equal-score 4-node ring resolves to `[a, b, c, d]` (sorted-key NodeID)

### D-01 import invariant
- `internal/graph/{pagerank,options,personalize}.go` import only `cmp`, `math`, `sort`
- `go list -deps ./internal/graph/...` shows zero project imports

### D-04 repomap migration
- `internal/repomap/graph.go` unchanged (`git diff --stat` shows zero modifications)
- `internal/repomap/pagerank.go` is 86 LOC (< 100 target)
- `grep -c "math.Abs" internal/repomap/pagerank.go` = 0 (engine body lives only in `internal/graph`)
- All existing repomap tests pass without test-vector edits

### Map-iteration leak invariant (D-03)
- `grep -nE "for [a-zA-Z_]+ := range (edges|teleport|rank|next|outWeight)" internal/graph/pagerank.go` returns ZERO matches
- All node-keyed iterations run over the pre-sorted slice; inner edge-sum loops are commutative addition (annotated)

## Commits

| Task | Hash | Message |
| ---- | ---- | ------- |
| 1 | `c1eccc93` | `test(62-01): add failing PR determinism + hex-digest + personalized + tiebreak tests` |
| 2a | `310c2bc3` | `feat(62-01): implement deterministic generic PageRank engine at internal/graph` |
| 2b | `c03466d6` | `test(62-01): pin golden PR hex digests after engine GREEN` |
| 3 | `7acf2a03` | `refactor(62-01): migrate internal/repomap PR to internal/graph adapter (D-04 vector re-pin)` |
| 4 | _no patches needed_ | Whole-repo gate clean on first run; no `chore(62-01)` commit |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Trim adapter file under 100-LOC target**
- **Found during:** Task 3
- **Issue:** First-pass adapter measured 101 LOC, just above the plan's `< 100` acceptance criterion. The two methods duplicated the same RLock-snapshot-then-build-pAny boilerplate.
- **Fix:** Extracted `(g *FileGraph) snapshot()` helper and `toAnyMap(map[string]float64) map[any]float64` helper; both methods became 6-line bodies. Final size: 86 LOC.
- **Files modified:** `internal/repomap/pagerank.go`
- **Commit:** `7acf2a03` (rolled into the same commit; no separate fix patch)

**2. [Rule 3 - Blocking] Test file used unused `sort` import after first scaffold**
- **Found during:** Task 3 (pre-Task 3 commit)
- **Issue:** Initial `internal/repomap/pagerank.go` rewrite kept a `sort` import that was no longer needed after delegation.
- **Fix:** Removed import; the `sort.SliceStable` ordering work now lives inside `graph.RankNodes`.
- **Files modified:** `internal/repomap/pagerank.go`
- **Commit:** `7acf2a03`

### Vector re-pin (D-04 expected) — not actually required

The plan anticipated that existing `internal/repomap/pagerank_test.go` vectors might need re-pinning because tiebreak ordering across equal scores becomes stable under the new engine. In practice **no test in the existing suite asserts a specific ordering across equal-score nodes**, so all 6 existing tests passed against the migrated engine without any fixture edits. Score values were unchanged (D-04 hard invariant), and the only behavioral change — stable tiebreak — was not under assertion. The plan's "one-time accept" allowance was not exercised.

## Auth Gates

None. Pure in-process Go work.

## Threat Flags

None. Surface is fully described by the plan's `<threat_model>`:
- T-62-01-T1 (determinism tampering) — **mitigated** by sort-before-iterate (D-03) on every node-keyed loop and the `TestPageRank_HexDigest` golden gate
- T-62-01-I1 (personalization disclosure) — **accepted** (no PII / secret material; engine has no logging)
- T-62-01-D1 (DoS via pathological graph) — **accepted** (pure CPU; `MaxIter` cap; no goroutines; no I/O)

No new attack surface introduced beyond the plan's register.

## Downstream Readiness

P02 (and later Phase 62 plans) may now consume `internal/graph.PageRank` directly:

```go
import "github.com/agenthands/helix/internal/graph"

scores := graph.PageRank(nodes, edges, graph.Options{
    Damping: 0.85, Epsilon: 1e-6, MaxIter: 100,
    Personalize: seedMap, // map[any]float64; keys must be assertable to T
})
ranked := graph.RankNodes(nodes, edges, opts) // []graph.Ranked[T]
```

Public surface is locked behind the GRAPH-01 byte-equality test — any future regression on determinism trips the golden digest immediately.

## Self-Check: PASSED

Verified files exist:
- `internal/graph/pagerank.go` ✓
- `internal/graph/options.go` ✓
- `internal/graph/personalize.go` ✓
- `internal/graph/pagerank_test.go` ✓
- `internal/graph/testdata/pagerank/golden_uniform.txt` ✓
- `internal/graph/testdata/pagerank/golden_personalized.txt` ✓
- `internal/graph/testdata/pagerank/golden_tiebreak.txt` ✓
- `internal/repomap/pagerank.go` ✓

Verified commits exist (`git log --oneline -5`):
- `c1eccc93` ✓
- `310c2bc3` ✓
- `c03466d6` ✓
- `7acf2a03` ✓

## TDD Gate Compliance

- RED gate: `c1eccc93` `test(62-01): ...` — tests fail with `undefined: PageRank` ✓
- GREEN gate: `310c2bc3` `feat(62-01): ...` — engine compiles; logic tests pass; goldens still PLACEHOLDER ✓
- Pin gate: `c03466d6` `test(62-01): pin golden PR hex digests after engine GREEN` ✓
- REFACTOR gate: `7acf2a03` `refactor(62-01): ...` (repomap migration) ✓

All gates present in correct order.
