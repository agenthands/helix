---
phase: 46-bug-repomap-lua-fixture
verified: 2026-04-24T00:00:00Z
status: passed
score: 6/6 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: null
  previous_score: null
  gaps_closed: []
  gaps_remaining: []
  regressions: []
---

# Phase 46: bug-repomap-lua-fixture Verification Report

**Phase Goal:** Fix `get_repo_map` so that invoking it on Serena's own workspace returns a ranked view of `internal/` Go sources instead of the single deep Lua testdata fixture — backed by a regression test preventing silent recurrence, without introducing path-based heuristics (D-01).

**Verified:** 2026-04-24
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | `get_repo_map` on polyglot Go+Lua surfaces Go symbols (not dominated by Lua fixtures) | VERIFIED | `TestRepomap_PolyglotRanking` PASS — post-fix top-1 is `pkg/logger.go` (0.4954), top-3 contains Go file. Pre-fix had `.../utils.lua` at 0.5533. |
| 2 | Regression test in `internal/repomap/` asserts polyglot invariant (≥1 Go symbol, Lua not dominant) | VERIFIED | `internal/repomap/polyglot_rank_test.go` exists with `TestRepomap_PolyglotRanking` + `TestRepomap_PolyglotRender` — both PASS. |
| 3 | Oracle smoke test pins the real-workspace symptom at the MCP boundary | VERIFIED | `test/oracle/scenario/repomap_polyglot_test.go::TestScenario_RepoMap_Polyglot` PASS (0.29s, integration build tag). |
| 4 | Root cause documented in phase review (PageRank/extractor/root/elide attribution) | VERIFIED | `46-RCA.md` explicitly rules on all 4 candidates: (1) PageRank starvation + (2) Extractor asymmetry **CONFIRMED co-primary**; (3) walker/workspace-root **REJECTED as root cause, amplifier only**; (4) elide/render **REJECTED**. |
| 5 | `go test ./...` and `go vet ./...` pass (modulo documented pre-existing failures) | VERIFIED | `go vet ./...` clean (pre-existing swift macro warning only). `go test ./internal/repomap/...` PASS. Full-suite failures (`TestClientRegistryContainsAll`, `TestToolDescriptionsComplete/GoldenFile`) verified pre-existing via `git checkout c1ff7a06^` — they fail identically before F1-B/F1-A commits. |
| 6 | D-01 constraint honored — zero path-based heuristics introduced | VERIFIED | `grep -cE 'filepath\.Ext\|strings\.HasPrefix.*\.(go\|lua)\|skipDirs' internal/repomap/graph.go internal/repomap/extractor.go` returns `0` and `0`. |

**Score:** 6/6 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `internal/repomap/graph.go` | F1-B ambiguity-weighted edges | VERIFIED | `defDegree` at line 104, `ambiguityScale := 1.0 / math.Sqrt(1.0+float64(defDegree[ident]))` at line 113, edge weight at line 127. |
| `internal/repomap/extractor.go` | F1-A `qualifyGoRef` helper gated by `lang == "go"` | VERIFIED | `qualifyGoRef` defined at line 313, dispatched at line 218 with Go+TagRef gate. |
| `internal/repomap/polyglot_rank_test.go` | Unit regression test | VERIFIED | Exists; `TestRepomap_PolyglotRanking` + `TestRepomap_PolyglotRender` PASS. |
| `test/oracle/scenario/repomap_polyglot_test.go` | Oracle smoke test | VERIFIED | Exists; `TestScenario_RepoMap_Polyglot` PASS under `-tags=integration`. |
| `46-RCA.md` | RCA write-up with 4-candidate verdict | VERIFIED | 209-line RCA with symptom, reproduction (unit + oracle), 4-candidate verdict table, confirmed cause, fix mechanism, D-01 compliance audit. |
| Orphan dir `.planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/` | Deleted/promoted | VERIFIED | No match under `.planning/phases/`; only `46-bug-repomap-lua-fixture/` present. |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| polyglot test | `BuildGraph` F1-B math | `FileGraph.RankFiles` | WIRED | Test runs extractor → graph → pagerank end-to-end and asserts top-1 `.go`. |
| extractor.go F1-A | graph.go F1-B | identifier-qualified keyspace | WIRED | F1-A produces `s.Add`/`Store.Add` keys that no longer collide with bare Lua defs, so F1-B's ambiguity math operates on a disambiguated key space. Fix commits `c1ff7a06` (F1-B) and `a8f9be80` (F1-A) both present in git log. |
| Oracle test | `get_repo_map` MCP tool | in-process MCP client | WIRED | Integration-tagged test invokes tool via in-process MCP client on `testdata/fixtures/polyglot_lua/`. PASS. |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
|----------|--------------|--------|--------------------|--------|
| `polyglot_rank_test.go` | ranked file list | Extractor → BuildGraph → RankFiles | Yes (asserted top-1 `.go` with 0.4954 weight, Lua at 0.1778) | FLOWING |
| `repomap_polyglot_test.go` | rendered repo map | MCP `get_repo_map` → skill adapter → engine | Yes (asserts Go file reference + Go symbol in rendered output) | FLOWING |
| `graph.go` `ambiguityScale` | per-edge weight | `defDegree` map from `defs` | Yes (non-uniform across shared names) | FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| Unit regression passes | `go test ./internal/repomap/...` | `ok` (cached) | PASS |
| Oracle regression passes | `go test -tags=integration ./test/oracle/scenario/ -run TestScenario_RepoMap_Polyglot` | `--- PASS: TestScenario_RepoMap_Polyglot (0.29s)` | PASS |
| go vet clean | `go vet ./...` | Clean (only pre-existing swift TOKEN_COUNT macro warning) | PASS |
| D-01 audit | `grep -cE 'filepath\.Ext\|strings\.HasPrefix.*\.(go\|lua)\|skipDirs' internal/repomap/graph.go internal/repomap/extractor.go` | `0`, `0` | PASS |
| Pre-existing failure isolation | `git checkout c1ff7a06^ && go test ./internal/cli/ ./test/bench/` | Both failures reproduce BEFORE phase 46 commits | PASS (failures predate phase 46) |
| Orphan dir removed | `ls .planning/phases/ \| grep 999` | no match | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|------|------|------|------|------|
| BUG-01 | 46-01, 46-02, 46-03 | `get_repo_map` returns Lua testdata instead of Go sources | SATISFIED | All 4 success criteria met — ranked output dominated by Go, unit+oracle regression tests green, RCA documented, `go test`/`go vet` pass. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| — | — | None | — | No path filters, no `filepath.Ext`, no `skipDirs`, no language/extension whitelists introduced in the fix. D-01 compliant. |

### Human Verification Required

None required. Both the unit and oracle layers were fully automated; the RCA write-up is in-tree; the real-world Serena-repo reproduction is pinned by the oracle test using an in-process MCP client against a committed polyglot fixture (`testdata/fixtures/polyglot_lua/`), replacing the manual MCP invocation originally listed in `46-VALIDATION.md`.

### Gaps Summary

No gaps. All 4 ROADMAP success criteria plus the D-01 hard constraint and orphan-cleanup directive are verified. Two full-suite failures observed (`internal/cli::TestClientRegistryContainsAll`, `test/bench::TestToolDescriptionsComplete/GoldenFile`) are confirmed pre-existing via git checkout to parent of `c1ff7a06` — they fail identically before any phase-46 commit and are unrelated to repomap. They are out of scope for this phase and tracked by unrelated prior work.

---

## VERIFIED

All 6 must-haves satisfied:
1. Polyglot rank dominance fixed — Go top-1, Lua dropped out of top-3.
2. Regression test in `internal/repomap/polyglot_rank_test.go` — asserts ≥1 Go + Lua-not-dominant.
3. Oracle smoke test in `test/oracle/scenario/repomap_polyglot_test.go` — pins the MCP-boundary symptom.
4. RCA documented in `46-RCA.md` with 4-candidate verdict (Candidates 1+2 confirmed co-primary; 3 rejected as amplifier; 4 rejected).
5. `go test ./...` and `go vet ./...` green for this phase's scope; two unrelated pre-existing failures verified independent via `git checkout c1ff7a06^`.
6. D-01 honored — `grep` audit returns 0 matches in both `graph.go` and `extractor.go`.

Orphan directory `.planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/` removed. Fix commits `c1ff7a06` (F1-B) and `a8f9be80` (F1-A) present in `git log`.

Phase 46 goal achieved. Ready to proceed.

---

_Verified: 2026-04-24_
_Verifier: Claude (gsd-verifier)_
