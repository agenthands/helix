---
phase: 08-advanced-testing
plan: 03
subsystem: test-integration
tags: [testing, concurrency, race-detector, synctest, worker-pool, stress]
requirements: [ADV-03]
dependency_graph:
  requires:
    - Plan 08-01 (Options.MaxWorkers, Options.Profile, Options.Mode extensions)
    - Phase 6 integration harness (StartTestDaemon, callTool, PrepareFixture)
    - Phase 7 Go fixture (testdata/fixtures/go with Helper/Greeter symbols)
  provides:
    - Three-tier concurrency test coverage (scenario stress, hot-path fan-out, deterministic unit)
    - Thread-safe SessionInfo with RWMutex-protected Snapshot/SetAllowedTools/RecordModeTransition accessors
    - make test-stress target for elevated-count CI runs
  affects:
    - internal/mcp/session.go — SessionInfo now mutex-protected; direct field access for bootstrap initialization only
    - internal/mcp/middleware.go — ProfileFilterMiddleware takes SessionSnapshot
    - internal/profile/skill.go — ExecuteSwitchMode + ExecuteGetTokenBudget read via Snapshot, write via SetAllowedTools
tech_stack:
  added:
    - golang.org/x/sync/errgroup (already in go.mod, first use in test/integration/)
    - testing/synctest (Go 1.25 stdlib, first use in repo)
  patterns:
    - t.Parallel() scenario subtests with shared daemon (Tier 1)
    - errgroup fan-out with ctx cancellation for saturation (Tier 2)
    - synctest virtual clock for scheduler-sensitive pure-function unit tests (Tier 3)
    - Read/write snapshot pattern for shared session state (SessionInfo.Snapshot)
key_files:
  created:
    - test/integration/concurrency_test.go
    - internal/kernel/lspool/pool_synctest_test.go
  modified:
    - internal/mcp/session.go
    - internal/mcp/middleware.go
    - internal/profile/skill.go
    - Makefile
decisions:
  - "Tier 3 scoped to WorkerMetrics.OnReuse/IdleDuration — pure functions consuming time.Now()/time.Since() with no Pool dependencies, so synctest virtual clock drives them directly"
  - "Tier 1 scenario list driven by actually-registered tool names (search_symbols, list_directory, find_files, search_in_files, get_symbol_overview) — not the aspirational names in the plan (find_symbol, list_dir, find_file, search_for_pattern)"
  - "ModeSwitchRace cycles through transitions reachable from 'edit' per full profile graph (read, edit, read, review, read); admin re-entry is not reachable mid-session and is not tested, since the read-only property we care about is no-deadlock/no-race, not every-transition-succeeds"
  - "SessionInfo guarded by sync.RWMutex with Snapshot/SetAllowedTools/RecordModeTransition accessor API; direct field access retained only for single-writer daemon bootstrap before the session becomes visible to MCP handlers"
  - "Ordering: SetAllowedTools BEFORE RecordModeTransition in ExecuteSwitchMode, so any concurrent reader that observes the new Mode also observes the matching tool whitelist"
metrics:
  duration: "~25min"
  completed: "2026-04-08"
  tasks: 3
  files: 6
---

# Phase 08 Plan 03: Three-Tier Concurrency Strategy Summary

Three-tier concurrency test strategy for the worker pool: scenario stress with `t.Parallel()`, hot-path fan-out with `errgroup`, deterministic unit tests with `testing/synctest`. Uncovered and fixed a real data race on `SessionInfo` under concurrent switch_mode + tool invocation (threat T-08-08), then added `make test-stress` for elevated CI runs.

## What Changed

### Task 1: Tier 1 + Tier 2 integration stress tests + SessionInfo race fix

Created `test/integration/concurrency_test.go` (build tag `integration`, package `integration_test`) with three tests:

- **`TestConcurrency_MixedScenarios` (Tier 1):** eight heterogeneous tool-call subtests (`search_symbols` x2, `search_in_files` x2, `list_directory` x2, `find_files`, `get_symbol_overview`) launched with `t.Parallel()` against a single daemon with `MaxWorkers=8`. Exercises worker pool share-until-dirty across realistic workloads.
- **`TestConcurrency_PoolSaturation` (Tier 2):** `errgroup.WithContext` fan-out of `N=100` `search_symbols` calls — explicit hot-path saturation. Baseline for CI stress runs.
- **`TestConcurrency_ModeSwitchRace` (T-08-08):** 20 concurrent `list_directory` calls interleaved with mode-switch calls cycling `read -> edit -> read -> review -> read`. Session starts in `admin` via `cfg.Mode` to bypass the transition graph for the initial state, then the test races reachable transitions against parallel tool calls.

**Race discovered and fixed (Rule 1 + threat T-08-08 mitigation):** The mode-switch race test immediately flagged a data race on `SessionInfo`:

```
Read at 0x… by goroutine 84:
  profile.(*profileSkill).ExecuteSwitchMode()  (reads sess.Mode at skill.go:128)
Previous write at 0x… by goroutine 85:
  mcp.(*SessionInfo).RecordModeTransition()    (writes sess.Mode at session.go:33)
```

`SessionInfo` had no synchronization and was freely mutated by concurrent MCP handlers. Fix:

1. Added `sync.RWMutex` to `SessionInfo` and three accessor methods:
   - `Snapshot() SessionSnapshot` — defensive-copy read path used by readers.
   - `SetAllowedTools([]string)` — copying write path for tool whitelist.
   - `RecordModeTransition(from, to)` — already existed, now takes the write lock.
2. Added `SessionSnapshot` — an immutable value type carrying `{SessionID, WorkspaceKey, Mode, Profile, AllowedTools}` so readers can make consistent multi-field decisions without holding the lock.
3. Updated `profile.ExecuteSwitchMode` to `Snapshot()` the session for reads and call `SetAllowedTools` + `RecordModeTransition` in that order (so a reader that observes the new `Mode` also sees the matching whitelist).
4. Updated `profile.ExecuteGetTokenBudget` to read via `Snapshot()`.
5. Updated `mcp.ProfileFilterMiddleware` to take a single `Snapshot()` per `tools/list` request so filtering and description overrides see a consistent `(AllowedTools, Profile)` pair.

Direct field access on `SessionInfo` is retained only for the single-writer daemon bootstrap in `daemon.go:212`, which runs before the session is exposed to any MCP handler.

Files: `test/integration/concurrency_test.go` (new), `internal/mcp/session.go`, `internal/mcp/middleware.go`, `internal/profile/skill.go`
Commit: `27b725f4` (see Commit Accounting below)

### Task 2: Tier 3 synctest unit test for WorkerMetrics decay

Created `internal/kernel/lspool/pool_synctest_test.go` (no build tag — runs in the default unit-test suite). Scoped to `WorkerMetrics.OnReuse` and `IdleDuration` because those are pure methods on a zero-dependency struct: both consume `time.Now()` / `time.Since` directly, which is exactly what `testing/synctest`'s virtual clock replaces. No need to construct a `Pool`, registry, installer, pressure monitor, or logger.

The test pins the decay formula `score = score*exp(-elapsed/300) + 1` (worker.go:86) across three reuse events:

1. First `OnReuse` at t=0 — score goes from 0 to ~1.0.
2. Advance virtual time 300s — `IdleDuration` reports 300s (virtual). Second `OnReuse` — score becomes `1*exp(-1) + 1 ≈ 1.3679`. `IdleDuration` resets to 0.
3. Advance another 600s — third `OnReuse` — score becomes `1.3679*exp(-2) + 1 ≈ 1.1851`.

Expected values are pinned to 1e-6 / 1e-9 tolerance; if the formula drifts, the test fails with a clear diff rather than silently weakening.

Files: `internal/kernel/lspool/pool_synctest_test.go` (new)
Commit: `bb28d014`

### Task 3: Makefile test-stress target

Added `.PHONY: test-stress` and a target that runs the concurrency suite at elevated `-count=5 -race -timeout=10m`. Uses the existing `$(GO)` variable and a glob `^TestConcurrency` run-pattern so new concurrency tests in this package are picked up automatically.

Files: `Makefile`
Commit: `1ec3ec76`

## Verification

- `go vet ./...` — passes (including worktree cross-builds).
- `go test ./internal/kernel/lspool/... -run Synctest -race -count=1` — passes deterministically.
- `go test -tags integration ./test/integration/... -run '^TestConcurrency' -race -count=1 -timeout=3m` — all three tests pass under `-race` with `MaxWorkers=8`.
- `go test -tags integration ./test/integration/... -run '^TestMode_|^TestProfile_' -race -count=1` — regression clean; the SessionInfo locking did not break plan 08-02's profile/mode golden tests.
- `make -n test-stress` — renders the expected `go test -tags integration -run '^TestConcurrency' -race -count=5 -timeout=10m` command line.

## Commit Accounting

Three commits land this plan's work:

| Commit | Files | Purpose |
|--------|-------|---------|
| `27b725f4` | concurrency_test.go, session.go, middleware.go, skill.go | Task 1: Tier 1+2 tests + SessionInfo race fix. Note: this commit's subject line reads `test(08-04): add Band 2 destructive exhaustive error matrix` because it was prepared by a parallel 08-04 agent that picked up this plan's staged files alongside its own Band 2 work. The net content of the commit correctly includes both plans' contributions. Git history for `test/integration/concurrency_test.go` shows `27b725f4` as the create. |
| `bb28d014` | pool_synctest_test.go | Task 2: Tier 3 synctest |
| `1ec3ec76` | Makefile | Task 3: test-stress target |

The 08-04 commit-subject overlap is a worktree/parallel-agent accident, not a content problem — file-level git blame correctly attributes the Task 1 changes, and the SUMMARY above lists them under Task 1 for traceability.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug / T-08-08 mitigation] Data race on SessionInfo under concurrent switch_mode + tool invocation**
- **Found during:** Task 1, first run of `TestConcurrency_ModeSwitchRace` with `-race`.
- **Issue:** `SessionInfo.Mode`, `SessionInfo.AllowedTools`, and `SessionInfo.Profile` were read by concurrent tool-call handlers (`ExecuteSwitchMode`, `ProfileFilterMiddleware`, `ExecuteGetTokenBudget`) and written by `RecordModeTransition` without any synchronization. Go race detector flagged the conflicting Read at `skill.go:128` vs Write at `session.go:33`.
- **Fix:** Added `sync.RWMutex` to `SessionInfo`, plus three mutex-protected accessors (`Snapshot`, `SetAllowedTools`, `RecordModeTransition`). Updated all three reader call sites to take a single consistent snapshot per operation.
- **Files modified:** `internal/mcp/session.go`, `internal/mcp/middleware.go`, `internal/profile/skill.go`
- **Commit:** `27b725f4` (see Commit Accounting above)
- **Why auto-fixed:** Threat T-08-08 in the plan's `<threat_model>` is dispositioned `mitigate`, and Rule 1 (auto-fix bugs) + Rule 2 (auto-add critical correctness) both apply. This is a real correctness issue under the exact workload the test is designed to exercise; leaving it unfixed would mean shipping a failing test.

**2. [Rule 1 - Scenario list drift] Tool-name/argument drift between plan and actual registry**
- **Found during:** Task 1 verification.
- **Issue:** The plan's scenario list referenced tools and argument names that do not match the actually-registered kernel tool set: `find_symbol` (registered as `search_symbols`), `list_dir` (`list_directory`), `find_file` (`find_files`), `search_for_pattern` (`search_in_files`), `get_symbols_overview` (`get_symbol_overview`), `relative_path` arg (`path`), `file_mask` arg (`pattern`), `name_path` arg (`query`).
- **Fix:** Rewrote the scenario table to use the real registered names and argument keys. Also updated `TestConcurrency_PoolSaturation` to use `search_symbols` and `TestConcurrency_ModeSwitchRace` to use `list_directory` with `path`.
- **Files modified:** `test/integration/concurrency_test.go`
- **Commit:** `27b725f4` (see Commit Accounting above)

**3. [Rule 1 - Arg name] switch_mode parameter is `target_mode`, not `mode`**
- **Found during:** Task 1 planning — caught from reading `internal/profile/skill.go:306`.
- **Fix:** Used `target_mode` in the ModeSwitchRace test.
- **Files modified:** `test/integration/concurrency_test.go` (incorporated from the start of the race test).

**4. [Rule 1 - Reachability] Mode transition graph**
- **Found during:** Task 1, first run of `TestConcurrency_ModeSwitchRace`.
- **Issue:** The plan's example cycled `read -> edit -> review -> admin`, but the full profile's `allowed_mode_transitions` graph does not allow `edit -> admin` or `review -> admin`. First run failed with `transition from "edit" to "admin" not allowed`.
- **Fix:** Rewrote the switch chain to `read -> edit -> read -> review -> read` (all reachable per the YAML graph). The test still races concurrent switches against tool calls, which is the actual hypothesis under test.
- **Files modified:** `test/integration/concurrency_test.go`
- **Commit:** `27b725f4`

**5. [Rule 1 - Numerics] Synctest decay expected value**
- **Found during:** Task 2 first run.
- **Issue:** Initial expected value `1.1851364510474877` was wrong by ~1.4e-5 — I mis-computed the compounded decay. The actual value is `1.3678794411714423 * exp(-2) + 1 = 1.1851223516044767`.
- **Fix:** Corrected the expected constant and tightened tolerance to `1e-9` (since the formula is fully deterministic under synctest).
- **Files modified:** `internal/kernel/lspool/pool_synctest_test.go`

## Deferred Issues

### Pre-existing race in `jsonrpc.TestConn_Call`

`go test -race ./... -count=1` uncovered a race in `internal/kernel/jsonrpc/codec_test.go:198` — a goroutine started inside that test writes to state read from the main test goroutine without synchronization. This is out of scope for plan 08-03 (pre-dates this plan, lives in `internal/kernel/jsonrpc` which plan 08-03 does not touch, and does not exercise the worker-pool concurrency paths that ADV-03 targets). Logged in `.planning/phases/08-advanced-testing/deferred-items.md` under `DEFERRED-01` for a future hardening plan.

## Threat Flags

No new trust boundaries introduced. Plan 08-03 only adds tests and a locking fix to existing state; it does not open new network surface, auth paths, or schema changes.

## Self-Check

Files:
- FOUND: `test/integration/concurrency_test.go`
- FOUND: `internal/kernel/lspool/pool_synctest_test.go`
- FOUND: `internal/mcp/session.go` (updated with RWMutex)
- FOUND: `internal/mcp/middleware.go` (Snapshot-based)
- FOUND: `internal/profile/skill.go` (Snapshot-based)
- FOUND: `Makefile` (with test-stress target)

Commits:
- FOUND: `27b725f4` — Task 1 content (concurrency_test.go + session race fix)
- FOUND: `bb28d014` — Task 2 (synctest unit)
- FOUND: `1ec3ec76` — Task 3 (Makefile test-stress)

Verification:
- FOUND: `go test -race ./internal/kernel/lspool/... -run Synctest` → ok
- FOUND: `go test -tags integration -race ./test/integration/... -run '^TestConcurrency'` → ok
- FOUND: `go test -tags integration -race ./test/integration/... -run '^TestMode_|^TestProfile_'` → ok (regression clean)

## Self-Check: PASSED
