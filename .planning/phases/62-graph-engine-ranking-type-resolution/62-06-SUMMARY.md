---
phase: 62-graph-engine-ranking-type-resolution
plan: 06
subsystem: type-resolution
tags: [type-resolution, determinism, sort-before-iterate, gap-closure, cr-03]
gap_closure: true

requires: []
provides:
  - "Sorted suffixRule slice + deterministic iteration in go/python/typescript guessFromName"
  - "Test-seam guessFromNameWithRules in all three resolvers"
  - "Meta-guard test (TestGuessFromName_NoMapIteration) preventing regression"
affects:
  - "GRAPH-01 byte-equal determinism contract (locked structurally against future suffix-overlap drift)"
  - "TYPES-04 emit-determinism invariant (locked structurally against future suffix-overlap drift)"

tech_stack_added: []
patterns_added:
  - "Sort-before-iterate doctrine: suffix-rule lookups now use []suffixRule sorted ascending by short, mirroring frontier.go / pagerank.go / weak.go / util.go.sortedNodeIDs"
  - "Test-seam pattern: production helper delegates to xxxWithRules(name, rules) for fixture-injectable determinism tests"
  - "Meta-guard test pattern: regex over source bytes detects forbidden patterns, with the regex constructed from string fragments to avoid self-matching"

key_files:
  created:
    - "internal/semantic/types/golang/guess_from_name_test.go"
    - "internal/semantic/types/python/guess_from_name_test.go"
    - "internal/semantic/types/typescript/guess_from_name_test.go"
  modified:
    - "internal/semantic/types/golang/resolver.go"
    - "internal/semantic/types/python/resolver.go"
    - "internal/semantic/types/typescript/resolver.go"

decisions:
  - "Use []suffixRule (struct slice) rather than parallel []string slices: keeps short/long pairing visually local and prevents arity drift on future additions"
  - "Sort by `short` ascending: lex order is the simplest structural invariant (verifiable by eye in code review) and reviewer-checkable in PRs"
  - "Add test-seam helper guessFromNameWithRules rather than exposing or mutating the production rules slice: avoids any production behaviour change while enabling fixture-driven overlap testing"
  - "Construct the meta-guard regex from string fragments via forbiddenSuffixMapPattern(): the test source itself does not match the rule it polices, keeping the repo-wide grep gate genuinely zero"

metrics:
  duration: "~6 minutes"
  tasks_completed: 4
  commits_created: 6
  files_changed: 6
  loc_changed: "+201 / -53"
  completed_date: "2026-05-07"
---

# Phase 62 Plan 06: Suffix-rule sort-before-iterate (CR-03 closure) Summary

**One-liner:** Replaced unsorted `for short, long := range suffixMap` map iteration in the per-language `guessFromName` helpers (go / python / typescript) with sorted `[]suffixRule` iteration plus test-seam helpers and meta-guard tests, locking GRAPH-01 / TYPES-04 byte-equal determinism against future suffix-overlap drift.

## Goal Achievement

The plan closed verification finding **CR-03** ("Per-language guessFromName helpers in golang/typescript/python iterate map[string]string directly — silent determinism break") cleanly. All four tasks completed; the whole-tree determinism gate is now zero across `internal/semantic/types/`.

### Behaviour preserved
The production suffix sets (Go: 8 rules, Python: 6 rules, TypeScript: 6 rules) are overlap-free at the tail level today, so behaviour is unchanged for every existing input. The change is structural — a contract upgrade locking down future additions.

### Test coverage added
Each language gained three new sub-tests:
- `TestGuessFromName_Deterministic` — 100× stability assertion on a baseline production input.
- `TestGuessFromName_DeterministicUnderOverlap` — exercises `guessFromNameWithRules` with deliberately overlapping rule pairs to assert that the lex-smaller short wins consistently. Under the prior map iteration this would flip-flop run-to-run.
- `TestGuessFromName_NoMapIteration` — meta-guard regex over `resolver.go` bytes guarding against future regressions to bare unsorted-map range iteration.

## Tasks Completed

| Task | Name | Commit | Files |
|---|---|---|---|
| 1 | RED — failing determinism tests for go guessFromName | `d513bb95` | `internal/semantic/types/golang/guess_from_name_test.go` (new) |
| 2 | GREEN — refactor go resolver to sorted slice | `4dd7ff31` | `internal/semantic/types/golang/resolver.go` |
| 3a | RED — failing determinism tests for python guessFromName | `d7ddcd37` | `internal/semantic/types/python/guess_from_name_test.go` (new) |
| 3b | GREEN — refactor python resolver to sorted slice (+ overlap-fixture order fix) | `a729163f` | `internal/semantic/types/python/resolver.go`, `internal/semantic/types/python/guess_from_name_test.go` |
| 4a | RED — failing determinism tests for ts guessFromName | `62584f0e` | `internal/semantic/types/typescript/guess_from_name_test.go` (new) |
| 4b | GREEN — refactor ts resolver to sorted slice (+ meta-guard test self-match fix) | `ded04d52` | `internal/semantic/types/typescript/resolver.go`, all three `guess_from_name_test.go` |

Plan slices the work as 4 logical tasks (Task 3 and Task 4 each bundle RED+GREEN per the plan); each RED/GREEN gate is a separate commit, totalling 6 commits.

## Verification Run

```bash
go test ./internal/semantic/types/{golang,python,typescript}/... -count=10 -run TestGuessFromName
```
PASS — three packages, ten iterations each, all sub-tests green.

```bash
go test ./internal/semantic/types/... -count=1
```
PASS — 7 packages including java/php/ruby stubs and the `types/` shared core; no regression in ladder/chain/fixpoint/emit suite.

```bash
go test ./internal/graph/... ./internal/semantic/graph/... ./internal/semantic/cluster/... -count=1
```
PASS — Phase 62's other 18 verified must-haves remain green (PageRank determinism goldens, WeakComponents goldens, ApplyRepair contract tests, frontier overflow signal, etc.).

```bash
go vet ./...
```
Clean — only the pre-existing Swift TOKEN_COUNT cosmetic warning that 62-VERIFICATION.md already classifies as PASS.

```bash
grep -rnE "for [_a-zA-Z]+, [_a-zA-Z]+ := range suffixMap" internal/semantic/types/
```
0 matches — whole-tree determinism gate is zero.

## TDD Gate Compliance

The plan declared `type: tdd`. All three languages followed RED → GREEN cleanly:

| Language | RED commit | GREEN commit | Gate sequence |
|---|---|---|---|
| go | `d513bb95` (`test(62-06)…`) | `4dd7ff31` (`feat(62-06)…`) | OK |
| python | `d7ddcd37` (`test(62-06)…`) | `a729163f` (`feat(62-06)…`) | OK |
| typescript | `62584f0e` (`test(62-06)…`) | `ded04d52` (`feat(62-06)…`) | OK |

REFACTOR commits were not necessary — each GREEN landed at a clean shape that did not warrant a separate cleanup pass.

The fail-fast invariant ("a test passing unexpectedly during RED is a stop signal") was honoured: each RED commit was verified as a build failure (undefined `suffixRule` / `guessFromNameWithRules`) before the GREEN refactor was applied.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Python overlap-fixture rule slice was not lex-sorted**
- **Found during:** Task 3 (Python GREEN run)
- **Issue:** The RED commit declared the overlap rules as `[{short:"repo"...},{short:"bar_repo"...}]`. The helper's documented invariant is "callers MUST pass rules sorted ascending by short" — `bar_repo` < `repo` lexicographically, so the slice was unsorted, causing the Python GREEN run to fail with `"FooBarRepository"` (matched `repo` first because it appeared first in the slice) instead of the expected `"FooBarRepoFull"`.
- **Fix:** Reordered the literal in `guess_from_name_test.go` so `bar_repo` precedes `repo`, matching the invariant.
- **Files modified:** `internal/semantic/types/python/guess_from_name_test.go`
- **Commit:** Folded into GREEN-Python `a729163f`

**2. [Rule 1 - Bug] Test files self-matched the meta-guard regex**
- **Found during:** Task 4 (whole-tree gate check after TS GREEN)
- **Issue:** The plan's acceptance criterion required the repo-wide grep `grep -rnE "for [_a-zA-Z]+, [_a-zA-Z]+ := range suffixMap" internal/semantic/types/` to return zero matches. The first three test files each contained the literal regex `for [_a-zA-Z]+, [_a-zA-Z]+ := range suffixMap` as a Go raw string AND in human-readable comments / error messages. Those literal copies tripped the very pattern they were guarding (six matches across three test files) — the gate was structurally not zero.
- **Fix:** Rewrote each test to construct the regex from string fragments at runtime via `forbiddenSuffixMapPattern()` (returns `"for [_a-zA-Z]+, [_a-zA-Z]+ := range " + "suffix" + "Map"`). Reworded comments and error messages to avoid the literal "for k, v := range suffixMap" form.
- **Files modified:** `internal/semantic/types/golang/guess_from_name_test.go`, `internal/semantic/types/python/guess_from_name_test.go`, `internal/semantic/types/typescript/guess_from_name_test.go`
- **Commit:** Folded into GREEN-TypeScript `ded04d52`

Both deviations were Rule 1 bugs in the test artefacts (not the production resolvers); the production-side refactor matched the plan's specification verbatim. After both fixes, all four `verify` commands listed in the plan pass cleanly.

## Threat Flags

(None. The change is a structural refactor of an internal heuristic; it adds no new network surface, auth path, file access pattern, or trust boundary.)

## Self-Check: PASSED

**Files created — verified present:**
- FOUND: `internal/semantic/types/golang/guess_from_name_test.go`
- FOUND: `internal/semantic/types/python/guess_from_name_test.go`
- FOUND: `internal/semantic/types/typescript/guess_from_name_test.go`

**Files modified — verified containing target strings:**
- FOUND: `type suffixRule struct` + `var goSuffixRules = []suffixRule{` in `internal/semantic/types/golang/resolver.go`
- FOUND: `type suffixRule struct` + `var pySuffixRules = []suffixRule{` in `internal/semantic/types/python/resolver.go`
- FOUND: `type suffixRule struct` + `var tsSuffixRules = []suffixRule{` in `internal/semantic/types/typescript/resolver.go`

**Commits — verified present in git log:**
- FOUND: `d513bb95` (test go RED)
- FOUND: `4dd7ff31` (feat go GREEN)
- FOUND: `d7ddcd37` (test python RED)
- FOUND: `a729163f` (feat python GREEN)
- FOUND: `62584f0e` (test ts RED)
- FOUND: `ded04d52` (feat ts GREEN)

**Acceptance gates:**
- 0 matches for `for [_a-zA-Z]+, [_a-zA-Z]+ := range suffixMap` across `internal/semantic/types/` (whole-tree gate honoured)
- 8 + 6 + 6 = 20 `long: ` rules preserved across the three resolvers (matches the historical map cardinality)
- All 18 prior verified must-haves still green (Phase 62 cross-package smoke clean)
