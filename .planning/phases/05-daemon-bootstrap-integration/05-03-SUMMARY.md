---
phase: 05-daemon-bootstrap-integration
plan: 03
subsystem: testing
tags: [integration-tests, daemon, bootstrap, e2e, smoke-tests]

requires:
  - phase: 05-daemon-bootstrap-integration/02
    provides: fully wired daemon bootstrap (daemon.go, shutdown.go, imports.go)
provides:
  - 10 integration tests proving all 6 E2E flows work end-to-end
  - Bootstrap tests for tool registration, skill init, and profile resolution
  - E2E smoke tests for memory CRUD, mode switching, token budget, shutdown, config layering, workflow onboarding
affects: []

tech-stack:
  added: []
  patterns:
    - "Integration tests use daemon.New() directly without network listeners"
    - "Skills registered via init() cannot be Reset() between tests in same process"
    - "testSessionProvider pattern for injecting session state into profile skill"

key-files:
  created:
    - internal/daemon/bootstrap_test.go
    - internal/daemon/daemon_integration_test.go
  modified: []

key-decisions:
  - "Removed skill.Reset() from test cleanup -- init()-registered skills only register once per process, so resetting loses them permanently"
  - "Used internal package access (d.mcpServer, d.activeProfile) instead of adding public accessors to Daemon struct"

patterns-established:
  - "Bootstrap test pattern: daemon.New() + registry assertions for tool count verification"
  - "E2E test pattern: skill.InitAll() with temp dirs + ExecuteTool interface for skill invocation"

requirements-completed:
  - SYM-01
  - SYM-02
  - SYM-03
  - SYM-04
  - SYM-05
  - SYM-06
  - SYM-07
  - SYM-08
  - SYM-09
  - EDT-01
  - EDT-02
  - EDT-03
  - EDT-04
  - EDT-05
  - EDT-06
  - FIL-01
  - FIL-02
  - FIL-03
  - FIL-04
  - FIL-05
  - FIL-06
  - DGN-01
  - DGN-02
  - DGN-03
  - DMN-07
  - DMN-08
  - DMN-09
  - DMN-10
  - DMN-11
  - MEM-01
  - MEM-02
  - MEM-03
  - MEM-04
  - MEM-05
  - WFL-01
  - WFL-02
  - WFL-03
  - PRF-01
  - PRF-02
  - PRF-03
  - PRF-04
  - PRF-05
  - LNG-01
  - LNG-02
  - LNG-03
  - WRK-02
  - WRK-03

duration: 3min
completed: 2026-04-08
---

# Phase 05 Plan 03: Integration Smoke Tests Summary

**10 integration tests proving daemon bootstrap registers 38 tools, initializes 7 skills, resolves profiles, and all 6 E2E flows work end-to-end**

## Performance

- **Duration:** 3 min
- **Started:** 2026-04-08T14:32:45Z
- **Completed:** 2026-04-08T14:35:57Z
- **Tasks:** 2
- **Files modified:** 2

## Accomplishments
- 4 bootstrap tests verify tool registration (38+ tools), skill initialization (7 skills), and profile resolution (claude-code, full)
- 6 E2E smoke tests cover memory CRUD, mode switching with history, token budget, clean shutdown with goroutine leak detection, profile config layering, and workflow onboarding
- All tests pass in under 1 second total, confirming the daemon bootstrap wiring from Plan 02 works at runtime

## Task Commits

Each task was committed atomically:

1. **Task 1: Bootstrap integration tests** - `3248c16d` (test)
2. **Task 2: E2E flow smoke tests** - `72932ed6` (test)

## Files Created/Modified
- `internal/daemon/bootstrap_test.go` - 4 bootstrap tests: tool registration, skill init, profile resolution, default profile
- `internal/daemon/daemon_integration_test.go` - 6 E2E tests: memory CRUD, mode switching, token budget, clean shutdown, config layering, workflow onboarding

## Decisions Made
- Removed skill.Reset() from test cleanup because init()-registered skills only register once per Go process; resetting permanently loses them
- Used internal package access (d.mcpServer, d.activeProfile) to avoid adding public accessor methods to the Daemon struct since tests are in the same package
- Used testSessionProvider to inject session state into profile skill rather than depending on daemon wiring

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed skill.Reset() calls that caused cross-test failures**
- **Found during:** Task 1 (bootstrap tests)
- **Issue:** Plan specified t.Cleanup(func() { skill.Reset() }) but skills registered via init() in imports.go only run once per process; resetting wiped the registry permanently
- **Fix:** Removed all skill.Reset() calls; skills are shared across tests as intended by the Caddy-style init() pattern
- **Files modified:** internal/daemon/bootstrap_test.go
- **Verification:** All 4 bootstrap tests pass consecutively
- **Committed in:** 3248c16d

---

**Total deviations:** 1 auto-fixed (1 bug)
**Impact on plan:** Essential fix for test correctness. No scope creep.

## Issues Encountered
None - after fixing the skill.Reset() issue, all tests passed on first run.

## User Setup Required
None - no external service configuration required.

## Known Stubs
None - all tests exercise real code paths with real assertions.

## Next Phase Readiness
- All 10 integration tests pass, covering all 6 E2E flows from the milestone audit
- Phase 05 (daemon-bootstrap-integration) is complete
- Full internal test suite passes with zero failures

---
*Phase: 05-daemon-bootstrap-integration*
*Completed: 2026-04-08*
