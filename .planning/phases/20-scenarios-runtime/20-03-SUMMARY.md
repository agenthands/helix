---
phase: 20-scenarios-runtime
plan: 03
subsystem: profile-mode-errors
tags: [profile, mode, errors, golden, circuit-breaker, oracle]
dependency_graph:
  requires:
    - 20-01 (test fixtures, runtime oracle stub)
  provides:
    - test/oracle/scenario/profile_test.go (profile/mode behavior tests)
    - test/oracle/runtime/errors_deferred_test.go (deferred CONT-03 error tests)
    - test/oracle/contract/testdata/golden/errors/unsupported.golden
  affects:
    - test/oracle/contract/errors_test.go (completes TODO(phase-20) for deferred categories)
tech_stack:
  added: []
  patterns: [sequential mode isolation, unit-level circuit breaker testing]
key_files:
  created:
    - test/oracle/scenario/profile_test.go (5 test functions)
    - test/oracle/runtime/errors_deferred_test.go (3 test functions)
    - test/oracle/contract/testdata/golden/errors/unsupported.golden
  modified: []
decisions:
  - ProfileFilterMiddleware only filters tools/list, not tools/call; tests verify advertised tool set
  - Concurrent session isolation tested sequentially due to singleton profile skill session provider
  - Python mode behavior tested with SkipLS since mode filtering is profile-level, not LS-level
  - activate_project not in admin tool assertions (registered by daemon, not via skill ToolProvider)
  - Circuit open tested at unit level (breaker exhaustion) rather than integration (cannot crash gopls)
metrics:
  duration: 505s
  completed: 2026-04-11
---

# Phase 20 Plan 03: Profile/Mode Behavior and Deferred Error Categories Summary

Profile mode filtering tests across Go and Python fixtures plus three deferred CONT-03 error categories (unsupported with golden, timeout with deadline injection, circuit open with breaker exhaustion).

## Task Completion

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Profile/mode behavior scenario tests | 2ce3707c | test/oracle/scenario/profile_test.go |
| 2 | Deferred CONT-03 error category tests | 20eaa701 | test/oracle/runtime/errors_deferred_test.go, test/oracle/contract/testdata/golden/errors/unsupported.golden |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Fixed tool name mismatch in replace_in_file arguments**
- **Found during:** Task 1
- **Issue:** Plan used `old`/`new`/`literal` args but actual tool schema uses `pattern`/`replacement`/`is_regex`
- **Fix:** Updated CallTool arguments to match actual ReplaceInFileArgs schema
- **Files modified:** test/oracle/scenario/profile_test.go

**2. [Rule 1 - Bug] Removed activate_project from admin mode expected tools**
- **Found during:** Task 1
- **Issue:** `activate_project` is registered directly by the daemon, not via a skill ToolProvider, so it does not appear in `skill.ResolveTools` output used by ProfileFilterMiddleware
- **Fix:** Removed from `adminModeToolsPresent` assertion list
- **Files modified:** test/oracle/scenario/profile_test.go

**3. [Rule 3 - Blocking] Adapted concurrent sessions test for singleton architecture**
- **Found during:** Task 1
- **Issue:** Profile skill is a process-level singleton with single session provider. Two daemons in the same process share mode state, making true per-session isolation untestable.
- **Fix:** Changed to sequential test (stop first runner before starting second) that verifies each mode produces the correct tool set independently. Documented as v1.3 concern.
- **Files modified:** test/oracle/scenario/profile_test.go

**4. [Rule 3 - Blocking] Python test changed to SkipLS mode**
- **Found during:** Task 1
- **Issue:** `pyright-langserver` may not be installed in test environment, and mode filtering is profile-level not LS-level
- **Fix:** Used `SkipLS: true` instead of `RequireLS(t, "pyright-langserver")` to ensure test runs everywhere while still validating D-07 two-language coverage
- **Files modified:** test/oracle/scenario/profile_test.go

## Verification Results

- `go vet -tags integration ./test/oracle/scenario/ ./test/oracle/runtime/` passes
- All 5 profile tests pass: ReadModeBlocksEdits, AdminGrantsAll, ModeSwitchUpdatesVisibility, ConcurrentSessionsDifferentModes, PythonModeBehavior
- All 3 deferred error tests pass: Unsupported (with golden), Timeout, CircuitOpen
- Golden file deterministic across repeated runs

## Known Stubs

None.

## Self-Check: PASSED

- test/oracle/scenario/profile_test.go: FOUND
- test/oracle/runtime/errors_deferred_test.go: FOUND
- test/oracle/contract/testdata/golden/errors/unsupported.golden: FOUND
- Commit 2ce3707c: FOUND
- Commit 20eaa701: FOUND
