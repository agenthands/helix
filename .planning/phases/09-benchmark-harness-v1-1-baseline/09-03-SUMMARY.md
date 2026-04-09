---
phase: 09
plan: 03
subsystem: test/bench
tags: [benchmark, tools, mcp, bench-02]
requires:
  - 09-02 (bench scaffold: manifest + TB helpers + goroutine leak guard)
provides:
  - BenchmarkTools — table-driven 38-tool response-time benchmark
  - memoryReseedFn dispatch — per-iteration reseeding for mutating memory tools
  - switch_mode reseed — pre-switch to "read" so the measured call transitions read->edit
affects:
  - test/bench/bench_helpers_test.go (worker pool caps bumped to accommodate edit-tool re-activation)
  - test/bench/tools_manifest_test.go (args fixed for switch_mode, ping, echo)
tech_stack_added:
  - testing.B.Loop (stdlib, Go 1.25)
tech_stack_patterns:
  - Pattern 2 shared daemon per bench function (09-RESEARCH.md)
  - Per-iteration fresh-copy via StopTimer/StartTimer fences for mutating tools
  - Reseed-map dispatch for state-consuming tools
key_files_created:
  - test/bench/tools_bench_test.go
key_files_modified:
  - test/bench/bench_helpers_test.go
  - test/bench/tools_manifest_test.go
decisions:
  - "Unified single b.Loop() body with conditional StopTimer/StartTimer dispatch on edit/reseed flags, to satisfy the 'grep -c for b.Loop() == 1' acceptance criterion"
  - "Dispatch on tc.needsCopy (manifest flag) OR isEditTool map, so non-symbol mutating tools (create_file, replace_in_file, format_code) also get fresh-copy-per-iteration"
  - "Reseed for switch_mode pre-switches to 'read' so the measured call transitions read->edit — the 'full' profile refuses self-transitions"
  - "MaxWorkers bumped to 256, BaseTTL trimmed to 5 — per-iteration workspace re-activation from the edit path consumes pool slots faster than adaptive TTL can evict them at the default cap of 2"
metrics:
  duration_minutes: 25
  tasks_completed: 1
  completed_date: 2026-04-08
requirements_satisfied: [BENCH-02]
---

# Phase 9 Plan 3: BenchmarkTools — 38-Tool Response-Time Benchmark Summary

Table-driven `BenchmarkTools` runs every tool in the 38-tool manifest through `testing.B.Loop` with a 3-call warmup and `ReportAllocs`, using the shared-daemon pattern locked by 09-RESEARCH.md Pattern 2.

## What Shipped

`test/bench/tools_bench_test.go` (created, ~180 lines):

- `BenchmarkTools(b *testing.B)` — the single entry point. Starts one daemon, activates the Go fixture, seeds memories, then iterates `benchTools` with a `b.Run(tc.name, ...)` per tool.
- Inside each sub-benchmark: re-anchor to the shared fixture, perform exactly 3 warmup calls (per phase Q6), call `b.ReportAllocs()`, then run the measured `for b.Loop()` body.
- A single `for b.Loop()` construct dispatches on two flags:
  - `edit` (set for any tool with `needsCopy=true` OR any of the 6 symbol-edit tools): stages a fresh `prepareGoFixtureCopyB(b)` + `activateWorkspaceB` inside `StopTimer/StartTimer` fences so the measured window covers only the tool call itself.
  - `reseed` (set for `rename_memory`, `edit_memory`, `delete_memory`, `switch_mode`): runs a state-restoration closure inside `StopTimer/StartTimer` fences so the tool operates on a known-good pre-state every iteration.
  - Neither flag → tight `for b.Loop() { callToolB(...) }` with no per-iteration setup.
- `seedBenchMemories` — called once from `BenchmarkTools` after daemon startup to create the `bench-manifest`, `bench-manifest-rename-src`, and `bench-manifest-delete` memories that the memory tools operate on. TestMain cannot seed these because the daemon doesn't exist yet when TestMain runs.

## Verification Evidence

```
$ go test -bench=BenchmarkTools -benchtime=1x -run=^$ -count=1 ./test/bench/...
... (38 BenchmarkTools/* lines)
PASS
ok  	github.com/postfix/serena/test/bench	129.559s
exit 0
```

Acceptance criteria greps against `test/bench/tools_bench_test.go`:

| Pattern                   | Expected       | Actual |
|---------------------------|----------------|--------|
| `for b\.Loop()`           | exactly 1      | 1      |
| `for i := 0; i < b\.N`    | 0              | 0      |
| `b\.ReportAllocs()`       | exactly 1      | 1      |
| `b\.ResetTimer`           | 0              | 0      |
| `prepareGoFixtureCopyB`   | matches        | 5      |
| `isEditTool`              | matches        | 6      |
| `for i := 0; i < 3`       | matches        | 1      |
| `startBenchDaemon(b)`     | exactly 1      | 1      |

`go vet ./test/bench/...` → exit 0.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Bumped worker pool cap in `bench_helpers_test.go`**

- **Found during:** Task 1 smoke run
- **Issue:** `defaultBenchConfig` set `MaxWorkers=2, BaseTTL=30`. The edit-tool path re-activates a fresh tb.TempDir() workspace for every warmup call + every `b.Loop()` iteration, and each activation holds an LS worker slot until adaptive TTL evicts it. At cap=2, the pool exhausted almost immediately, surfacing as `acquire session: maximum number of workers reached`.
- **Fix:** `MaxWorkers=256, BaseTTL=5` — headroom for `-count=10` baseline runs, faster eviction, still bounded.
- **Files modified:** `test/bench/bench_helpers_test.go`
- **Commit:** 220ce341

**2. [Rule 3 - Blocking] Repaired three args in `tools_manifest_test.go` (Plan 09-02 defects)**

- **Found during:** Task 1 smoke run
- **Issue:** Three manifest entries failed their MCP schema validation on every call:
  - `switch_mode` targeted `"read"`, but the `full` profile refuses self-transitions and the initial mode was already `read` → `transition from "read" to "read" not allowed`.
  - `ping` had empty args, but `PingArgs` in `internal/mcp/server.go` requires `Message` → `missing properties: ["message"]`.
  - `echo` passed `"message": "bench"`, but `EchoArgs` uses `Text` → `unexpected additional properties ["message"]`.
- **Fix:** Changed `switch_mode` target to `"edit"` (paired with a reseed that pre-switches to `read`, so the measured call is a legitimate `read->edit` transition). Added `message: "bench"` to `ping`. Renamed `echo`'s arg key to `text`.
- **Scope note:** These are Plan 09-02 manifest defects, not caused by my changes — but they are Rule 3 blockers for the plan's exit-0 acceptance criterion. Fixing the manifest is the minimal unblock.
- **Files modified:** `test/bench/tools_manifest_test.go`
- **Commit:** 220ce341

**3. [Rule 2 - Missing critical functionality] Added `seedBenchMemories` + `memoryReseedFn` dispatch**

- **Found during:** Task 1 smoke run
- **Issue:** The 09-02 `tools_manifest_test.go` header comment claims "TestMain seeds 'bench-manifest' memory so that read_memory, search_memories, rename_memory, edit_memory, and delete_memory all have something to operate on," but `TestMain` in `main_test.go` only runs the goroutine leak check — no seeding. `read_memory`, `rename_memory`, and `edit_memory` all failed with `memory "bench-manifest" not found`. Additionally, even with seeding, `rename_memory` consumes its source on the first call and `delete_memory` consumes its target — the second warmup call fails.
- **Fix:** Added `seedBenchMemories` (called once from `BenchmarkTools` after daemon startup — TestMain can't seed because the memory store is daemon-owned) plus `memoryReseedFn` — a map from tool name to a state-restoration closure that runs inside `StopTimer/StartTimer` fences. `switch_mode` uses the same reseed mechanism to pre-switch to `read` before each measured `read->edit` transition.
- **Files modified:** `test/bench/tools_bench_test.go` (seedBenchMemories + memoryReseedFn introduced alongside BenchmarkTools in the same commit)
- **Commit:** 220ce341

**4. [Rule 1 - Bug] Edit-tool warmup was running against the shared read-only fixture**

- **Found during:** Task 1 smoke run (second iteration)
- **Issue:** My initial BenchmarkTools implementation copied the fixture per `b.Loop()` iteration but did NOT copy for the 3 warmup calls — warmups ran against the shared read-only fixture, corrupting it for sibling sub-benches. This surfaced as `Helper` → `HelperRenamed` in the committed fixture plus stray comment lines from `insert_before_symbol`/`insert_after_symbol` warmups. The corruption persisted across runs via the checked-in fixture.
- **Fix:** Edit-tool warmup path now stages a fresh `prepareGoFixtureCopyB` + `activateWorkspaceB` for each of the 3 warmup calls. Restored the fixture from git via `git checkout testdata/fixtures/go/main.go` and removed the polluting `bench_created.txt` left behind by the `create_file` warmup.
- **Files modified:** `test/bench/tools_bench_test.go` (warmup loop gains `if edit { fresh := prepareGoFixtureCopyB(b); activateWorkspaceB(...) }`)
- **Commit:** 220ce341

### Architectural adjustments

**Dispatch on `tc.needsCopy OR isEditTool[name]`, not just the 6 symbol-edit tools.**

The plan step 3 lists 6 symbol-edit tools for the fresh-copy path. But `create_file`, `replace_in_file`, and `format_code` also mutate files (`needsCopy=true` in the 09-02 manifest), and they hit the same cross-iteration contamination problem. `isEditTool` is preserved as a literal 6-entry map to satisfy the plan's `grep -n "isEditTool"` acceptance check, but the actual dispatch keys on `tc.needsCopy || isEditTool[tc.name]` so all 9 mutating tools get the fresh-copy path.

**Single `for b.Loop()` construct with conditional in-body dispatch.**

The natural shape is three loops (edit / reseed / read-only), but the acceptance criteria require exactly one `for b.Loop()` occurrence. Collapsed into one loop body with a flag-driven StopTimer/StartTimer branch.

## Threat Flags

None. This plan does not introduce new trust boundaries, endpoints, or schema changes — it only adds benchmark scaffolding under `test/bench/`, which is not part of the production binary.

## Known Stubs

None.

## Self-Check: PASSED

- test/bench/tools_bench_test.go: FOUND
- test/bench/bench_helpers_test.go: FOUND (modified)
- test/bench/tools_manifest_test.go: FOUND (modified)
- Commit 220ce341: FOUND in git log

## Success Criteria

- [x] BENCH-02 satisfied: all 38 tools have benchmark coverage via table-driven b.Run
- [x] No compiler elision risk — `b.Loop()` mandated and the only loop form
- [x] No goroutine leaks — single daemon per bench function
- [x] Warmup policy from phase important_context Q6 honored (3 calls per sub-bench)
- [x] `go test -bench=BenchmarkTools -benchtime=1x -run=^$ -count=1 ./test/bench/...` exits 0
- [x] Output contains 38 sub-benchmark lines under `BenchmarkTools/`
- [x] `go vet ./test/bench/...` exits 0
