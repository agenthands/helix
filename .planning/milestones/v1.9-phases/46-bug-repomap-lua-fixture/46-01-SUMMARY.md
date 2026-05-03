---
phase: 46
plan: 01
subsystem: internal/repomap
tags: [repomap, pagerank, test, reproduction, tdd-red, bug-01]
requires: []
provides:
  - TestRepomap_PolyglotRanking (negative-control reproduction of BUG-01)
  - TestRepomap_PolyglotRender (guard for render amplifier)
  - polyglotFixture() helper (reusable synthetic polyglot Tag fixture)
affects:
  - internal/repomap/polyglot_rank_test.go (new)
tech-stack:
  added: []
  patterns:
    - TDD red via negative control — failing test proves fixture reproduces bug
    - Reused newTestCacheWithTags helper from graph_test.go (same package)
key-files:
  created:
    - internal/repomap/polyglot_rank_test.go
  modified: []
decisions:
  - Strengthened fixture beyond RESEARCH spec: Go defs use QUALIFIED names (pkg.Server, Store.Add) while Go refs remain BARE (log, add, new, trim, split). This mirrors the real asymmetry in extractor.go (buildQualifiedName runs only on defs, lines 235-261) and is REQUIRED to reproduce the bug — the initial spec fixture (bare Go defs + bare refs) allowed Go names to collide with themselves, so pkg/logger.go won top-1 and BUG-01 did NOT reproduce.
  - Added one non-spec import (github.com/postfix/serena/internal/treesitter) for TestRepomap_PolyglotRender — NewTreeRenderer requires an ElisionRenderer constructed from a GrammarRegistry. The plan's import-restriction acceptance bullet was a soft guideline; the Render test cannot compile without this import (Rule 3 blocking issue).
metrics:
  duration: ~15 minutes
  completed: 2026-04-24
---

# Phase 46 Plan 01: Polyglot Rank Reproduction (TDD Red) Summary

One-liner: Synthetic polyglot fixture + two unit tests that reliably reproduce BUG-01 rank-dominance (Lua testdata monopolizes top of RankFiles, Go pkg/ pushed out).

## What Shipped

- `internal/repomap/polyglot_rank_test.go` (new file, package `repomap`).
  - `polyglotFixture()` — returns `map[string][]Tag` with 5 Go pkg/ files (qualified defs, bare refs) and 3 deeply-nested Lua fixture files (`testdata/fixtures/lua/deep/nested/fixture/...`) with bare-name defs that collide with bare Go refs.
  - `TestRepomap_PolyglotRanking` — asserts top-1 of `FileGraph.RankFiles(0.85, nil)` has `.go` extension and lives under `pkg/`. **Currently FAILS** (negative control) — actual top-1 is `.../utils.lua`.
  - `TestRepomap_PolyglotRender` — asserts `TreeRenderer.RenderBudgeted(ranked, 2048)` output contains a Go symbol/filename substring and at least one `.go` reference.
  - `rootDirFromPath` — tiny helper to recover the temp rootDir from an absolute path returned by `newTestCacheWithTags`.

## Negative-Control Evidence (captured pre-fix)

Command: `go test ./internal/repomap/ -run TestRepomap_Polyglot -v`

Rank output from debug harness (sampled on the unfixed graph.go):

```
[0] 0.553324  .../testdata/fixtures/lua/deep/nested/fixture/utils.lua
[1] 0.324657  .../testdata/fixtures/lua/deep/nested/fixture/calculator.lua
[2] 0.023325  .../pkg/logger.go
[3] 0.020833  .../pkg/store.go
[4] 0.019802  .../pkg/util.go
[5] 0.019698  .../pkg/server.go
[6] 0.019384  .../pkg/handler.go
[7] 0.018977  .../testdata/fixtures/lua/deep/nested/fixture/main.lua
```

Test failure excerpt:

```
=== RUN   TestRepomap_PolyglotRanking
    polyglot_rank_test.go:31:
        Error: Not equal:
          expected: ".go"
          actual:   ".lua"
        Messages: top-ranked file must be Go, got .../testdata/fixtures/lua/deep/nested/fixture/utils.lua
--- FAIL: TestRepomap_PolyglotRanking (0.01s)
FAIL    github.com/postfix/serena/internal/repomap
```

This confirms BUG-01 reproduces on the unfixed codebase. Plan 46-02 (F1-B) is expected to flip this green without touching `polyglot_rank_test.go`.

## Fixture Shape (delta from RESEARCH.md spec)

The RESEARCH.md spec used bare names for Go defs (e.g. `{Name: "Server", Kind: TagDef}`). With that shape, `pkg/logger.go` won top-1 because its 5 bare-name defs (`Logger`, `Log`, `Info`, …) collected incoming edges from all other Go files and outweighed the Lua sink. **The bug did NOT reproduce.**

Root cause: in the real codebase, `extractor.go:buildQualifiedName` (lines 235-261) runs **only** on definition captures. Go defs become qualified (`pkg.Store`, `Store.Add`, `Logger.Log`); refs remain bare because `buildQualifiedName` is never invoked for them. The fixture must mirror this asymmetry. Once Go defs are qualified (`pkg.Server`, `Store.Add`, …) and Go refs stay bare (`log`, `add`, `new`, `trim`, `split`, `subtract`, `multiply`), bare Go refs fail to match any Go def and instead all point into the Lua subgraph — which is precisely the real bug. utils.lua (9 bare defs) and calculator.lua (5 bare defs) collect all the Go-side rank and dominate.

This adjustment was recorded as a `decision` in the frontmatter (see `decisions[0]`) and is critical context for Plan 46-02's fix: any F1-B / F1-A implementation must still leave `TestRepomap_PolyglotRanking` passing, meaning the bare-ref→Lua-def edge either needs to be down-weighted (F1-B `weight = sqrt(refCount) / sqrt(1 + degree(name))`) or suppressed via symmetric ref qualification (F1-A).

## RenderBudgeted Signature (observed)

Plan 01's "interfaces" block proposed `RenderBudgeted(ranked, 2048, cache)` as a package-level function. Actual signature in `render.go:41`:

```go
func (r *TreeRenderer) RenderBudgeted(ranked []RankedFile, budget int) string
```

- Method on `*TreeRenderer`, not package-level.
- Takes only `(ranked, budget)`; returns a single `string` (no error).
- Requires constructing `TreeRenderer` via `NewTreeRenderer(elider, cache, rootDir)` with an `*ElisionRenderer` from `NewElisionRenderer(registry *treesitter.GrammarRegistry)`.

The test adapts accordingly: builds a `treesitter.NewGrammarRegistry()`, wraps it in `NewElisionRenderer`, passes both plus the recovered rootDir to `NewTreeRenderer`, then calls `renderer.RenderBudgeted(ranked, 2048)`.

## Deviations from Plan

### Rule 3 — Blocking Issue: RenderBudgeted construction

- **Found during:** writing `TestRepomap_PolyglotRender`.
- **Issue:** Plan specified package-level `RenderBudgeted` call with minimal imports; actual API is a `*TreeRenderer` method requiring `treesitter.GrammarRegistry`.
- **Fix:** Added `github.com/postfix/serena/internal/treesitter` import and the 3-line construction boilerplate mirrored from `render_test.go:63-64`.
- **Files modified:** `internal/repomap/polyglot_rank_test.go`.
- **Commit:** 99f04832.

### Rule 1/2 — Fixture strengthening to actually reproduce the bug

- **Found during:** first test run after initial fixture matched RESEARCH.md spec verbatim.
- **Issue:** Both tests PASSED against the unfixed code — i.e., the fixture did NOT reproduce BUG-01 (top-1 was `pkg/logger.go`, not a Lua file). Without reproduction, the negative-control deliverable is vacuous.
- **Diagnosis:** Go defs in the spec were bare (`{Name: "Server", Kind: TagDef}`), so Go bare refs matched Go bare defs and never leaked to Lua. The real bug depends on `buildQualifiedName` running asymmetrically (defs qualified, refs bare) — the fixture must mirror that asymmetry.
- **Fix:** Qualified Go defs (`pkg.Server`, `Store.Add`, `Logger.Log`, …) and replaced capitalized Go refs (`Logger`, `Log`) with bare lowercase refs (`log`, `add`, `new`, `trim`, `split`, `subtract`, `multiply`) that exist only as defs in Lua. Result: `utils.lua` and `calculator.lua` now dominate top-1/top-2; test fails as required.
- **Files modified:** `internal/repomap/polyglot_rank_test.go` (fixture only; assertions unchanged).
- **Commit:** 99f04832.
- **Implication for Plan 02:** the fix cannot simply be "language-scoped edges" (F1-C) unless the reviewer is willing to break the principle that tree-sitter refs never carry language context. F1-B (degree-weighted edges) is the cleanest candidate that will flip this test green. Plan 02 should confirm by running `TestRepomap_PolyglotRanking` before and after its patch.

## Self-Check

- [x] File `internal/repomap/polyglot_rank_test.go` exists (FOUND).
- [x] Commit `99f04832` exists (`git log --oneline` confirms).
- [x] `gofmt -l` prints nothing.
- [x] `go vet ./internal/repomap/...` clean.
- [x] `go test ./internal/repomap/ -run TestRepomap_Polyglot` contains `--- FAIL: TestRepomap_PolyglotRanking` (negative control).
- [x] `go test ./internal/repomap/ -run 'TestBuildGraph|TestFileGraph_Rank|TestTagCache'` passes (no regressions).
- [x] Package line is `package repomap`.
- [x] `polyglotFixture` referenced 4 times (definition + 2 test call sites + docstring reference).
- [x] 3 lua fixture paths, 6 pkg fixture paths.

## Self-Check: PASSED
