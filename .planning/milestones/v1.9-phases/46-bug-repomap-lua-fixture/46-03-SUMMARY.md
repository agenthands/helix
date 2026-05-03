---
phase: 46
plan: 03
subsystem: test/oracle/scenario + .planning
tags: [repomap, oracle, integration-test, rca, cleanup, bug-01]
requires: [46-01, 46-02]
provides:
  - TestScenario_RepoMap_Polyglot (MCP-tool-boundary regression guard for BUG-01)
  - testdata/fixtures/polyglot_lua/ (Go pkg/ + nested Lua fixture)
  - 46-RCA.md (phase-review root-cause write-up, closes success criterion 3)
affects:
  - test/oracle/scenario/repomap_polyglot_test.go (new)
  - testdata/fixtures/polyglot_lua/** (new)
  - .planning/phases/46-bug-repomap-lua-fixture/46-RCA.md (new)
  - .planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/ (deleted)
tech-stack:
  added: []
  patterns:
    - Oracle smoke test harnessing get_repo_map via in-process MCP client
      (mirrors test/oracle/scenario/repomap_test.go)
    - t.Setenv("HOME", t.TempDir()) to isolate daemon.New()'s skill.InitAll
      which binds RepoMapSkill to $HOME/.serena/default-project
key-files:
  created:
    - test/oracle/scenario/repomap_polyglot_test.go
    - testdata/fixtures/polyglot_lua/go.mod
    - testdata/fixtures/polyglot_lua/pkg/{server,store,logger,handler,util}.go
    - testdata/fixtures/polyglot_lua/testdata/fixtures/lua/deep/nested/fixture/{main,calculator,utils}.lua
    - .planning/phases/46-bug-repomap-lua-fixture/46-RCA.md
  deleted:
    - .planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/.gitkeep
  modified: []
decisions:
  - Overrode $HOME with t.Setenv("HOME", t.TempDir()) because daemon.New()
    re-runs skill.InitAll with $HOME/.serena/default-project as ProjectDir —
    without the override the test's prior skill.InitAll(tempDeps) in
    StartRunner is clobbered and RepoMapSkill binds to the developer's
    populated ~/.serena/default-project/index/tags.db. Diagnosed via a
    TagCache.AllFiles() dump during initial test run (962 files of
    developer-repo tags instead of the expected ~8 fixture files).
  - Did NOT downgrade oracle assertions per D-05. The test passes
    cleanly once the $HOME isolation is in place; assertions remain
    as specified in the plan.
metrics:
  duration: ~40 minutes (including diagnosis of $HOME leakage)
  completed: 2026-04-24
---

# Phase 46 Plan 03: Oracle Smoke Test + RCA + Orphan Cleanup Summary

**One-liner:** Oracle-layer integration smoke test pins the BUG-01 fix
at the `get_repo_map` MCP-tool boundary, phase RCA closes success
criterion 3, and the orphan backlog directory is removed — completing
Phase 46 end-to-end.

## What Shipped

### Oracle smoke test — `test/oracle/scenario/repomap_polyglot_test.go`

- Build tag: `//go:build integration || llm || llmjudge` (does not run
  in default `go test ./...`).
- Uses `harness.PrepareFixture(t, "polyglot_lua")` to copy the fixture
  into `t.TempDir()/polyglot_lua/`.
- Calls `activate_project` (via `StartRunner`), then `get_repo_map`
  with `token_budget=4096`.
- Assertions:
  1. Output non-empty + no "No files found".
  2. Contains `.go` and `pkg` substrings.
  3. Contains at least one of `Server` / `Store` / `Handler` /
     `Logger+logger.go` — at least one Go symbol from the fixture.
  4. Go reference count > 0; Go appears whenever Lua appears (guard
     against Lua-domination).
- `t.Setenv("HOME", t.TempDir())` isolates the daemon from the
  developer's populated `~/.serena/default-project` tag cache (see
  Deviations).

### Polyglot fixture — `testdata/fixtures/polyglot_lua/`

Shape (confirming acceptance criteria):

```
testdata/fixtures/polyglot_lua/
├── go.mod                                            (module github.com/postfix/serena-fixture-polyglot)
├── pkg/
│   ├── server.go                                     (type Server; Start/Stop; calls Logger.Log, Store.Add)
│   ├── store.go                                      (type Store; Add/Get/Delete; calls Logger.Log)
│   ├── logger.go                                     (type Logger; Log/Info/Debug/Warn/Error)
│   ├── handler.go                                    (type Handler; Handle; calls Server.Start/Stop, Store.Add, Logger.Log/Error)
│   └── util.go                                       (Trim/Split/LogAll; calls Logger.Log)
└── testdata/
    └── fixtures/
        └── lua/
            └── deep/
                └── nested/
                    └── fixture/
                        ├── main.lua                  (requires calculator+utils; bare calls to add/subtract/multiply/log/trim/split)
                        ├── calculator.lua            (add/subtract/multiply/divide/power — bare field defs)
                        └── utils.lua                 (log/trim/split/Logger.new/Logger:log — bare field defs)
```

File counts:

```
$ ls testdata/fixtures/polyglot_lua/pkg/*.go | wc -l
       5
$ find testdata/fixtures/polyglot_lua -name '*.lua' | wc -l
       3
$ find testdata/fixtures/polyglot_lua -path '*deep/nested/fixture*.lua' | wc -l
       3
```

All acceptance criteria for file-count and path-depth met.

### RCA document — `.planning/phases/46-bug-repomap-lua-fixture/46-RCA.md`

A ~200-line markdown write-up with sections: Symptom, Reproduction
(unit + oracle), Four Candidates Evaluated (D-02, all four on equal
footing), Confirmed Cause, Fix Applied (F1-B + F1-A), Why This Is Not
A Path Filter (D-01 compliance), Evidence, Links. Cites commits
`c1ff7a06` (F1-B) and `a8f9be80` (F1-A).

### Orphan directory removal

`.planning/phases/999.1-repomap-returns-lua-fixture-instead-of-go-sources/`
removed via `git rm -rf` after pre-delete safety check confirmed it
contained only `.gitkeep` (no investigation notes to carry over, per
CONTEXT.md line 67 and ROADMAP.md directive).

## Verification Output

### Oracle test (green)

```
$ go test -tags=integration ./test/oracle/scenario/ \
      -run TestScenario_RepoMap_Polyglot -v
=== RUN   TestScenario_RepoMap_Polyglot
--- PASS: TestScenario_RepoMap_Polyglot (0.30s)
PASS
ok  	github.com/postfix/serena/test/oracle/scenario	1.350s
```

### `go test ./...` (default)

Two pre-existing failures remain, both unrelated to Phase 46 and
documented in `46-02-SUMMARY.md` (verified independent of this plan
via `git stash` bisection):

- `TestClientRegistryContainsAll` in `internal/cli/` — client registry
  assertion drift (registry map has 7 items, test expects 6).
- `TestToolDescriptionsComplete` / `TestToolDescriptionsGoldenFile`
  in `test/bench/` — tool description golden-file drift.

Per SCOPE BOUNDARY these are logged but not fixed in this plan.

### `go vet ./...`

Clean modulo the pre-existing Swift C-macro redefinition warning
in `internal/treesitter/bindings/swift/src/scanner.c` — unrelated, not
touched by this plan.

### D-01 grep audit

```
$ grep -cE 'filepath\.Ext|skipDirs' \
      internal/repomap/graph.go internal/repomap/extractor.go
internal/repomap/graph.go:0
internal/repomap/extractor.go:0
```

Zero path/language/extension heuristics. D-01 honored.

### git status + git log

```
$ git status --short
?? .serena/session-stats.json

$ git log --oneline -n 5
05c9b924 docs(46-03): phase RCA + remove orphan backlog dir (closes BUG-01)
d5c06aee test(46-03): add oracle polyglot smoke test + fixture for BUG-01
9eb538ce docs(phase-46): update tracking after wave 2 .planning/ROADMAP.md .planning/STATE.md
a8f9be80 fix(46-02): qualify Go call-site refs by operand identifier (F1-A)
c1ff7a06 fix(46-02): weight repomap graph edges by name ambiguity (F1-B)
```

## Deviations from Plan

### Rule 3 — Blocking Issue: $HOME leakage breaks oracle-test isolation

- **Found during:** first run of `TestScenario_RepoMap_Polyglot`.
- **Issue:** Test failed because `get_repo_map` returned a ranking
  dominated by `/Users/Janis_Vizulis/go/src/github.com/postfix/serena/legacy/test/resources/repos/lua/test_repo/src/utils.lua`
  — a file NOT in our fixture. Instrumentation via
  `TagCache.AllFiles()` showed 962 cached files, including Serena's
  own source tree.
- **Root cause:** `daemon.New()` (at `internal/daemon/daemon.go:215`)
  re-runs `skill.InitAll` with
  `ProjectDir = filepath.Join(homeDir, ".serena", "default-project")`.
  This overrode the test harness's prior `skill.InitAll(tempDeps)` in
  `StartRunner` and bound the singleton RepoMapSkill to the
  developer's populated
  `~/.serena/default-project/index/tags.db`. `activate_project`
  correctly reset `cachePopulated=false` and re-walked the fixture,
  but the daemon-scoped `AllFiles()` query returned every previously
  extracted developer-repo file as well.
- **Fix:** Added `t.Setenv("HOME", t.TempDir())` at the top of the
  test body. Go's `testing` package restores the original `HOME` on
  test cleanup. The daemon now computes `homeDir` = temp dir and the
  RepoMapSkill binds to a fresh empty cache DB.
- **Files modified:** `test/oracle/scenario/repomap_polyglot_test.go`
  (only — no daemon code changes).
- **Rationale (scope):** This is a test-environment isolation issue,
  not a repomap bug. The daemon's behavior (re-running `skill.InitAll`
  with a default project dir) is load-bearing for normal daemon
  startup and out-of-scope for a BUG-01 regression guard. The
  single-line `t.Setenv` is the minimal correct fix. Documented
  inline in the test file so future maintainers understand why.
- **Commit:** `d5c06aee`.

### No D-05 downgrade

The oracle test assertions remain as specified in the plan — no
downgrade applied. The test passes cleanly on the first fixture
iteration once `$HOME` isolation is in place. No brittleness
observed.

### No adjacent fixes

Pre-existing failures in `internal/cli` (`TestClientRegistryContainsAll`)
and `test/bench` (`TestToolDescriptionsComplete` / `Golden`) are NOT
in the Phase 46 scope (Plan 02 SUMMARY logged them identically).
Verified via `git stash` bisection that they exist on base commit
`9eb538ce` without any Phase 46 code.

## Success Criteria Status

| # | Criterion | Status |
|---|-----------|--------|
| 1 | Unit test exists + guards BUG-01 (D-04 unit half) | ✓ Plan 01 — `internal/repomap/polyglot_rank_test.go` |
| 2 | Oracle smoke test exists + guards BUG-01 (D-04 oracle half) | ✓ Plan 03 — this plan |
| 3 | RCA documents root cause + fix + D-01 compliance | ✓ `46-RCA.md` |
| 4 | `go test ./...` + `go vet ./...` green | ✓ modulo 2 pre-existing unrelated failures |
| — | Orphan backlog dir removed | ✓ `git rm -rf` of `999.1-…` after safety check |
| — | D-01 honored (zero path heuristics) | ✓ grep audit returns 0 |

Phase 46 ready for `/gsd-verify-work`.

## Commits Landed

- `d5c06aee` — `test(46-03): add oracle polyglot smoke test + fixture for BUG-01`
- `05c9b924` — `docs(46-03): phase RCA + remove orphan backlog dir (closes BUG-01)`

## Self-Check

- [x] `test/oracle/scenario/repomap_polyglot_test.go` exists (FOUND).
- [x] `head -1 test/oracle/scenario/repomap_polyglot_test.go` equals `//go:build integration || llm || llmjudge`.
- [x] `grep -c 'func TestScenario_RepoMap_Polyglot' test/oracle/scenario/repomap_polyglot_test.go` = 1.
- [x] `grep -c 'harness.PrepareFixture' test/oracle/scenario/repomap_polyglot_test.go` = 1.
- [x] `grep -c 'SkipLS:\s*true' test/oracle/scenario/repomap_polyglot_test.go` = 1.
- [x] Fixture: 5 `pkg/*.go`, 3 `*.lua` files under `deep/nested/fixture/`.
- [x] Oracle test passes: `--- PASS: TestScenario_RepoMap_Polyglot (0.30s)`.
- [x] `go test ./...` passes modulo 2 pre-existing unrelated failures (verified via git stash).
- [x] `go vet ./...` clean (pre-existing swift macro warning only).
- [x] `gofmt -l` empty on all touched Go files.
- [x] `.planning/phases/999.1-...` directory deleted; `git status` shows staged deletion committed.
- [x] `46-RCA.md` exists, wc -l ≥ 40, contains all four candidates, F1-B ≥ 2 occurrences, `bare-name` ≥ 2 occurrences, `Plan 01`/`Plan 02`/`negative-control` ≥ 2 occurrences.
- [x] D-01 grep audit returns 0 on graph.go + extractor.go.
- [x] Commit `d5c06aee` exists (Task 1).
- [x] Commit `05c9b924` exists (Task 2).

## Self-Check: PASSED
