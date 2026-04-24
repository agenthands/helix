---
phase: 46
plan: 02
subsystem: internal/repomap
tags: [repomap, pagerank, graph, fix, f1-b, bug-01, partial]
requires: [46-01]
provides:
  - BuildGraph with F1-B ambiguity-weighted edges (1/sqrt(1+defDegree))
  - graph_test.go weight constants updated symmetrically
affects:
  - internal/repomap/graph.go (modified — F1-B applied)
  - internal/repomap/graph_test.go (modified — weight constants)
tech-stack:
  added: []
  patterns:
    - Ambiguity-weighted PageRank edges (information-theoretic, language-agnostic)
key-files:
  created: []
  modified:
    - internal/repomap/graph.go
    - internal/repomap/graph_test.go
decisions:
  - Applied F1-B LITERALLY per plan specification (`weight = sqrt(refCount) * 1/sqrt(1+defDegree[name])`). All acceptance grep criteria pass; all non-polyglot tests pass.
  - **Partial outcome surfaced as Rule 4 architectural finding:** TestRepomap_PolyglotRanking does NOT flip GREEN with F1-B alone on the Plan 01 fixture. See "Remaining Gap" below.
  - Did NOT unilaterally escalate beyond F1-B. Explored alternatives (totalDegree = defDegree+refDegree, linear 1/(1+totalDegree), reverse edges, F1-A-lite bare-suffix matching, case-insensitive suffix matching) during execution; none produced a clean fix aligned with the plan's literal F1-B code block + grep acceptance criteria. Surfacing to orchestrator for a follow-up plan per Rule 4.
metrics:
  duration: ~45 minutes (bulk spent on deviation analysis)
  completed: 2026-04-24
---

# Phase 46 Plan 02: F1-B Ambiguity-Weighted Graph Edges Summary

**One-liner:** `BuildGraph` now scales cross-file ref→def edges by `1/sqrt(1+defDegree[name])` — literal F1-B per RESEARCH.md — closing common-name ambiguity at the weight-math level, but the Plan 01 polyglot fixture does NOT flip green because every bare name in that fixture has `defDegree=1`.

## Before / After Diff (BuildGraph edge-creation block)

```diff
+    // F1-B (Phase 46 / BUG-01): weight edges by inverse sqrt of name
+    // ambiguity. A name defined in many files is a weaker signal per-edge
+    // than a name defined in one file. This dampens common-name collisions
+    // (add, log, new, Logger) across languages without any path- or
+    // language-specific heuristic. See
+    // .planning/phases/46-bug-repomap-lua-fixture/46-RESEARCH.md.
+    defDegree := make(map[string]int, len(defs))
+    for name, files := range defs {
+        defDegree[name] = len(files)
+    }

     g := NewFileGraph()

-    // Create edges: referencer -> definer, weight = sqrt(refCount).
+    // Create edges: referencer -> definer, weight = sqrt(refCount) * ambiguityScale.
     for ident, definers := range defs {
+        ambiguityScale := 1.0 / math.Sqrt(1.0+float64(defDegree[ident]))
         refFiles, hasRefs := refs[ident]
         if !hasRefs {
             for _, defFile := range definers {
                 g.addEdge(defFile, defFile, 0.1)
             }
             continue
         }
         for refFile, count := range refFiles {
             for _, defFile := range definers {
                 if refFile == defFile {
                     continue // skip same-file refs
                 }
-                g.addEdge(refFile, defFile, math.Sqrt(float64(count)))
+                g.addEdge(refFile, defFile, math.Sqrt(float64(count))*ambiguityScale)
             }
         }
     }
```

Test constant updates (`graph_test.go`):
- Line 63: `math.Sqrt(1)` → `math.Sqrt(1)*1.0/math.Sqrt(2)` (≈ 0.7071)
- Line 84: `2.0` → `2.0*1.0/math.Sqrt(2)` (≈ 1.4142)

## Grep Acceptance Audit (all pass)

| Criterion | Expected | Actual |
|-----------|----------|--------|
| `defDegree` count | ≥ 3 | 3 ✓ |
| `ambiguityScale` count | ≥ 2 | 3 ✓ |
| `1.0 / math.Sqrt(1.0+float64(defDegree` | 1 | 1 ✓ |
| `g.addEdge(defFile, defFile, 0.1)` | 1 | 1 ✓ |
| `if refFile == defFile` | 1 | 1 ✓ |
| Path heuristics `filepath.Ext \| HasPrefix.*\.(go\|lua) \| skipDirs` | 0 | 0 ✓ (D-01 honored) |
| `math.Sqrt(1)*1.0/math.Sqrt(2)` in test | 1 | 1 ✓ |
| `2.0*1.0/math.Sqrt(2)` in test | 1 | 1 ✓ |

## Test Results

### `go test ./internal/repomap/ -v -run 'TestBuildGraph|TestFileGraph_Rank|TestTagCache|TestEnrichFromLSP'`

All pass:
- `TestBuildGraph_CrossFileEdges` — PASS (updated constant)
- `TestBuildGraph_WeightByRefCount` — PASS (updated constant)
- `TestBuildGraph_IsolatedDefinition` — PASS (self-loop 0.1 invariant preserved)
- `TestBuildGraph_SkipsSameFileRefs` — PASS (F1-B only scales down; threshold `w < 0.5` still satisfied)
- `TestBuildGraph_EmptyCache` — PASS
- `TestFileGraph_RankFiles_SortedDescending` — PASS
- `TestTagCache_*` (5) — PASS
- `TestEnrichFromLSP_*` (2) — PASS

### `go test ./internal/repomap/ -run 'TestRepomap_Polyglot'`

- `TestRepomap_PolyglotRanking` — **FAIL** (top-1 still `utils.lua`, not a Go file)
- `TestRepomap_PolyglotRender` — PASS (render output includes `.go` references via ranked list)

### `go vet ./...`

Clean (modulo the pre-existing unrelated `TOKEN_COUNT` warning in `internal/treesitter/bindings/swift` which is not touched by this plan).

## Remaining Gap — Rule 4 Architectural Finding

### The Mathematical Reason F1-B Alone Cannot Flip Plan 01's Polyglot Fixture

Plan 01's synthetic polyglot fixture was deliberately strengthened (per its SUMMARY's `decisions[0]`) so that Go definitions are **qualified** (`pkg.Server`, `Store.Add`, `Logger.Log`) while Go references remain **bare** (`log`, `add`, `new`, `trim`, `split`, `subtract`, `multiply`). The bare Go refs collide only with Lua bare defs in `utils.lua` and `calculator.lua`.

**Critically, every bare name in the collision set is defined in exactly ONE Lua file.** For example:
- `log` is defined only in `utils.lua` → `defDegree["log"] = 1`
- `add` is defined only in `calculator.lua` → `defDegree["add"] = 1`
- …and so on for every bare ref name.

With `defDegree[name] = 1` for every such name, F1-B's `1/sqrt(1+1) = 1/sqrt(2) ≈ 0.7071` scales **every cross-file edge uniformly**. A uniform scale factor cannot change a ranking — it only rescales all scores by a constant.

Post-F1-B rank numbers (captured in-worktree via a throwaway debug test):

```
RANK 0.550228  utils.lua          <— still #1
RANK 0.323960  calculator.lua     <— still #2
RANK 0.025107  pkg/logger.go      <— Go files start here
RANK 0.021673  pkg/store.go
RANK 0.020232  pkg/util.go
RANK 0.020085  pkg/server.go
RANK 0.019644  pkg/handler.go
RANK 0.019071  main.lua
```

Compare to Plan 01's pre-fix capture (utils.lua 0.553, calculator.lua 0.325, Go ≤ 0.024): **the shape is unchanged**; only the absolute numbers drifted slightly.

### Why This Is Structural, Not a Formula Bug

Go files in the fixture have **zero incoming cross-file edges** because:

1. Go defs are qualified (`Server.Start`) — no ref in any file matches them (refs are bare `start`, and `start` is never referenced in the fixture anyway; bare refs like `log` don't match qualified `Logger.Log` under the plain-name equality test in BuildGraph).
2. Go bare refs all point to Lua defs.
3. Lua files (`utils.lua`, `calculator.lua`) have no cross-file refs of their own (only `main.lua` refs into them).

Thus the graph has ONE flow direction: Go files → Lua sinks. F1-B dampens this flow but cannot reverse it. Under PageRank, mass still accumulates in the Lua sinks. The only ways to change the ranking order without a path/language heuristic are:

- **F1-A (ref qualification):** Qualify bare Go refs symmetrically with defs, so `s.add(...)` in a Go file produces ref `Store.Add` (or at minimum `?.add` → indexed by suffix), matching a qualified Go def. This creates incoming edges on Go files and restores the Go subgraph. **Cannot be added by a pure `graph.go` edit — needs `extractor.go:buildQualifiedName` changes.**
- **Bare-suffix indexing:** When a ref name equals the unqualified suffix of a qualified def (case-insensitive), also create an edge. Pure `graph.go` change, but the fixture is case-mismatched (`Store.Add` vs ref `add`) so only case-insensitive matching would work — a new heuristic the plan does not authorize.
- **Reciprocal edges:** Add a small def→ref back-edge. Experimented with 0.5× and 1.0× ratios — neither flipped the ranking because bidirectional mass flow through utils.lua (which has the highest degree in both directions) still keeps it at #1.

### Recommendation for Follow-up Plan

Per RESEARCH.md §"Recommended sequence": *"Try F1-B first (smallest diff). If Lua still dominates, combine with F1-A."* **The gap is exactly the F1-A path.**

A Plan 46-02b (or 46-03 if that slot is reserved for RCA write-up) should:
1. Extend `internal/repomap/extractor.go:buildQualifiedName` to also run on `reference.*` capture names, producing `ReceiverType.method` or (where type is not inferable from the local tree-sitter parse) `?.method` ref names.
2. Update BuildGraph to match refs like `?.add` against qualified defs `*.add` (suffix match).
3. Confirm the Plan 01 polyglot tests finally flip green and no existing `graph_test.go` assertions break (bare-ref tests like `{Name: "Foo", Kind: TagRef}` should continue to work since `buildQualifiedName` falls through to the bare name when no receiver is present).

## RCA Evidence for Plan 03 (Phase Review)

**Primary Candidate (per RESEARCH.md §Top Candidates):** Candidate 1 — PageRank with bare-name cross-language edges. **Confirmed.**

Mechanism in two parts:
1. **Extractor asymmetry** (`extractor.go:buildQualifiedName` runs only on defs, lines 235-261): Go defs become qualified (`Store.Add`); Go refs stay bare (`add`).
2. **Graph builder bare-name match** (`graph.go:BuildGraph` pre-F1-B): bare refs match only bare defs. Every bare Go ref fails to find a matching Go def and pumps rank into whatever Lua file holds the bare def.

F1-B dampens the edge weights but does not address the matching asymmetry — hence the partial outcome. Contribution of other candidates based on this wave's evidence:

| Candidate | Contribution | Evidence |
|-----------|--------------|----------|
| 1 (PageRank via bare-name edges) | **Primary** | Confirmed above; fixture mechanism reproduces mathematically. |
| 2 (Extractor bias) | Contributes (the def/ref qualification asymmetry IS an extractor choice, even if Go gets healthy tag counts otherwise) | `buildQualifiedName` asymmetry is the root enabler of Candidate 1. |
| 3 (Walker / worktree inflation) | Amplifier in real repo only | Not relevant to the synthetic fixture, but in the real Serena repo `.claude/worktrees/` multiplies Lua sink strength ~13×. |
| 4 (Render amplifier) | Not observed in this fixture | `RenderBudgeted` output included `.go` references post-F1-B (TestRepomap_PolyglotRender passes). |

## Deviations from Plan

### Rule 4 (architectural) — Partial outcome on TDD-green test

- **Found during:** Task 1 verification.
- **Issue:** Plan `must_haves.truths[4]` states `Polyglot ranking unit test from Plan 01 now PASSES (TDD green): top-1 is a .go file under pkg/`. After applying the literal F1-B formula (which satisfies ALL grep acceptance criteria), `TestRepomap_PolyglotRanking` still fails.
- **Root cause:** Every bare name in Plan 01's adversarial fixture has `defDegree=1`. F1-B's uniform `1/sqrt(2)` scaling cannot re-order a graph where Go files have no incoming cross-file edges (all flow is Go→Lua). RESEARCH.md explicitly anticipated this: *"If Lua still dominates, combine with F1-A."*
- **Action:** Committed the literal F1-B per plan spec (all grep criteria satisfied, all existing unit tests green). Did NOT unilaterally add ref-qualification (F1-A) or bare-suffix matching because:
  1. Extractor changes (F1-A) are out of scope for a graph-only plan.
  2. Bare-suffix matching requires case-insensitive handling (new heuristic) to work on Plan 01's fixture.
  3. Reciprocal edges and ref-degree-weighted ambiguity were tried and also failed to flip the ranking.
- **Surfacing:** Recommending orchestrator dispatch a follow-up plan (46-02b or later) to add F1-A ref-qualification.
- **Files modified:** None beyond the plan's specified files.
- **Commit:** c1ff7a06.

### Rule 3 — In-flight exploration (not committed)

During execution I briefly tried three alternative formulations (totalDegree = defDegree+refDegree, reverse edges at 0.5 and 1.0, inline bare-suffix indexing). None are reflected in the final commit — each was reverted once it failed to flip the ranking. The final committed state is exactly the plan's literal F1-B formula.

## Self-Check

- [x] File `internal/repomap/graph.go` modified and staged (MODIFIED).
- [x] File `internal/repomap/graph_test.go` modified and staged (MODIFIED).
- [x] Commit `c1ff7a06` exists (`git log --oneline` confirms).
- [x] `gofmt -l internal/repomap/graph.go internal/repomap/graph_test.go` prints nothing.
- [x] `go vet ./internal/repomap/...` clean.
- [x] All non-polyglot repomap tests pass.
- [x] All grep acceptance criteria pass (see audit table).
- [x] D-01 honored — zero path/language/extension heuristics in graph.go.
- [ ] **`TestRepomap_PolyglotRanking` PASSES** — does NOT pass; see Remaining Gap.
- [x] `TestRepomap_PolyglotRender` passes.
- [x] Rest of the suite (`go test ./...` excluding polyglot) green.

## Self-Check: PARTIAL

One stated must_have truth (Plan 01 polyglot ranking test flips GREEN) is **not met** due to the structural finding documented in "Remaining Gap". All other acceptance criteria met. Orchestrator decision requested per Rule 4.
