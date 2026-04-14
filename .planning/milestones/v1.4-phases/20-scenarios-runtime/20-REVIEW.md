---
phase: 20-scenarios-runtime
reviewed: 2026-04-11T12:00:00Z
depth: standard
files_reviewed: 24
files_reviewed_list:
  - test/oracle/runtime/degraded_start_test.go
  - test/oracle/runtime/doc_test.go
  - test/oracle/runtime/doc.go
  - test/oracle/runtime/errors_deferred_test.go
  - test/oracle/runtime/pool_stress_test.go
  - test/oracle/runtime/shutdown_test.go
  - test/oracle/scenario/collision_test.go
  - test/oracle/scenario/degraded_test.go
  - test/oracle/scenario/go_test.go
  - test/oracle/scenario/polyglot_test.go
  - test/oracle/scenario/profile_test.go
  - test/oracle/scenario/python_test.go
  - test/oracle/scenario/typescript_test.go
  - test/oracle/scenario/unsupported_test.go
  - test/oracle/contract/testdata/golden/errors/unsupported.golden
  - testdata/fixtures/collision/backend/config/main.go
  - testdata/fixtures/collision/web/config/main.ts
  - testdata/fixtures/collision/worker/config/main.py
  - testdata/fixtures/polyglot/main.go
  - testdata/fixtures/polyglot/pkg/greeter.go
  - testdata/fixtures/polyglot/main.py
  - testdata/fixtures/polyglot/utils.py
  - testdata/fixtures/polyglot/main.ts
  - testdata/fixtures/polyglot/greeter.ts
findings:
  critical: 0
  warning: 3
  info: 3
  total: 6
status: issues_found
---

# Phase 20: Code Review Report

**Reviewed:** 2026-04-11T12:00:00Z
**Depth:** standard
**Files Reviewed:** 24
**Status:** issues_found

## Summary

Reviewed 14 test files (6 runtime, 8 scenario), 1 golden file, and 9 test fixture files for Phase 20 (scenarios and runtime testing). The test suite is well-structured with good separation of concerns: runtime tests cover pool stress, shutdown, and degraded startup; scenario tests cover per-language full-cycle workflows, polyglot, collision, profile/mode, and error categories.

Build tags are consistently applied (`integration || llm || llmjudge`). The harness provides proper resource cleanup via `t.Cleanup(r.Stop)` and `tb.TempDir()` for temp directories. Fixture isolation via copy-on-test prevents cross-test contamination.

Key concerns: a potentially flaky error assertion in the shutdown drain test, a comment/code mismatch in the goroutine leak threshold, and global skill state mutation without parallel guards that could cause issues if test isolation changes.

## Warnings

### WR-01: ErrorIs assertion too strict for transport-level errors in shutdown drain test

**File:** `test/oracle/runtime/shutdown_test.go:103`
**Issue:** The test uses `require.ErrorIs(t, err, context.Canceled, ...)` to assert that in-flight calls return `context.Canceled` on shutdown. However, when the daemon is stopped mid-call, the MCP in-memory transport or gRPC layer may wrap or produce different errors (e.g., `io.EOF`, `net.ErrClosed`, transport-specific close errors). The `errors.Is` unwrapping may not find `context.Canceled` in the chain, causing a false test failure. The test comment says "Either nil (completed) or context canceled -- both are acceptable" but the code rejects other error types that could legitimately occur during transport teardown.
**Fix:**
```go
if err != nil {
    // Accept context.Canceled, context.DeadlineExceeded, or transport close errors.
    // The invariant is: no panic, no hang -- any error type is acceptable during shutdown.
    t.Logf("inflight call returned error (acceptable during shutdown): %v", err)
}
```

### WR-02: Comment/code mismatch on goroutine leak threshold

**File:** `test/oracle/runtime/shutdown_test.go:26`
**Issue:** The doc comment says "delta <= 5" but the assertion on line 50 allows `before+10` (delta <= 10). The same `before+10` threshold is used again on line 114. While the assertion works, the discrepancy between documented and actual threshold may mislead future maintainers into thinking a delta of 8 is a regression when it is within the coded threshold.
**Fix:** Update the doc comment to match the code:
```go
// TestRuntime_Shutdown_NoGoroutineLeaks verifies that starting a daemon,
// doing work, and stopping it does not leak goroutines (delta <= 10).
```

### WR-03: Global skill state mutation in degraded_start_test.go not protected against accidental parallel execution

**File:** `test/oracle/runtime/degraded_start_test.go:10`
**Issue:** The file header comment warns "They MUST NOT use t.Parallel()" and the tests correctly do not call `t.Parallel()`. However, there is no enforcement mechanism. If a future contributor adds `t.Parallel()` to any of these tests, `skill.Reset()` modifies process-global state and would cause data races with other tests in the same package. Go's `-race` detector would catch this, but the failure mode is subtle. Consider adding an explicit guard comment at each test function or using a package-level mutex.
**Fix:** Add a brief comment at each test function:
```go
func TestRuntime_Degraded_SkillInitFailure(t *testing.T) {
    // NOT parallel -- skill.Reset() mutates global state.
```
This is already partially addressed in the file header, but per-function comments are more visible during editing. Alternatively, a `sync.Mutex` guard in the test helper would make it fail-fast.

## Info

### IN-01: Unused import potential in polyglot fixture

**File:** `testdata/fixtures/polyglot/main.py:1`
**Issue:** `from utils import greet` is imported but `greet` is never called in `main.py`. The `helper()` function and `DemoClass` do not use `greet`. While this is a test fixture (not production code), if any test later validates diagnostics or unused import warnings from pyright, this could produce unexpected results.
**Fix:** Either add a usage of `greet` in `main.py` or remove the import if it is not needed for test scenarios. If it is intentionally present for import-graph tests, add a comment explaining why.

### IN-02: Hardcoded line/column numbers in collision and polyglot tests are fragile

**File:** `test/oracle/scenario/collision_test.go:99-100`
**Issue:** Tests like `TestScenario_Collision_NoCrossContamination` and `TestScenario_Polyglot_NoCrossLanguageContamination` use hardcoded line and column numbers (e.g., `"line": 8, "column": 17` and `"line": 3, "column": 5`) for `go_to_definition` and `find_references` calls. If the fixture files are modified, these positions silently become wrong, producing misleading test results (wrong symbol resolved, or no results). This is a common pattern in LSP tests but worth noting.
**Fix:** Add comments next to each hardcoded position referencing the exact source text it targets:
```go
// line 8, col 17 = "Config" parameter in: func Handler(c Config) string
"line":   8,
"column": 17,
```
The collision_test.go already does this partially; polyglot_test.go line 80 could benefit from the same treatment.

### IN-03: Profile concurrent sessions test is sequential, not truly concurrent

**File:** `test/oracle/scenario/profile_test.go:128-165`
**Issue:** `TestScenario_Profile_ConcurrentSessionsDifferentModes` states it verifies "concurrent sessions in different modes have independent tool sets" but actually runs them sequentially (stop first runner, start second). The test comment explains this is due to a process-level singleton limitation. The test name and the test matrix entry (`concurrent sessions`) are somewhat misleading about what is actually being tested. This is documented in the test body but could confuse someone reading just the test name.
**Fix:** Consider renaming to `TestScenario_Profile_SequentialModesProduceDifferentToolSets` or adding a `t.Log` at the start explaining the sequential approach:
```go
t.Log("NOTE: runs modes sequentially due to process-level singleton; true concurrent isolation is a v1.3 concern")
```

---

_Reviewed: 2026-04-11T12:00:00Z_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
