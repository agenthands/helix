---
phase: 23-tool-migration
plan: 08
subsystem: errors
tags: [serr, typed-errors, mcp, middleware, sentinel-cleanup]

requires:
  - phase: 22-error-taxonomy
    provides: serr package with Kind type, Error struct, sentinel errors
  - phase: 23-tool-migration (plans 01-07)
    provides: all tool packages migrated to serr, making deprecated re-exports dead code
provides:
  - All deprecated sentinel re-exports removed (single source of truth in internal/errors)
  - MCP middleware uses serr.ErrCircuitOpen directly
  - MCP core tests use typed serr constructors
affects: []

tech-stack:
  added: []
  patterns: [direct serr import everywhere, no re-export bridges]

key-files:
  created: []
  modified:
    - internal/mcp/middleware.go
    - internal/mcp/telemetry_middleware_test.go
    - internal/mcp/telemetry_span_test.go
    - internal/mcp/server.go
    - internal/mcp/server_test.go
    - internal/kernel/lspool/circuit_test.go

key-decisions:
  - "Deleted errors.go entirely rather than keeping empty file -- no non-deprecated content remained"
  - "Replaced ErrorDetail struct usage in server.go with serr.Wrap for workspace activation error"
  - "Removed TestErrorDetail_JSON and TestDomainErrors tests since they tested deleted types"

patterns-established:
  - "No re-export bridges: all packages import serr directly from internal/errors"

requirements-completed: [MIG-08]

duration: 4min
completed: 2026-04-15
---

# Phase 23 Plan 08: MCP Core Cleanup Summary

**Removed all deprecated sentinel re-exports and bridge aliases, making internal/errors the single source of truth for error types**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-15T12:27:48Z
- **Completed:** 2026-04-15T12:31:37Z
- **Tasks:** 2
- **Files modified:** 6 modified, 2 deleted

## Accomplishments
- Middleware classifyOutcome uses serr.ErrCircuitOpen directly (no lspool indirection)
- All 3 MCP core test error sites migrated to serr.New(serr.Internal, ...) constructors
- Deleted internal/mcp/errors.go (5 deprecated sentinels + ErrorDetail struct)
- Deleted internal/kernel/lspool/circuit_err.go (ErrCircuitOpen re-export)
- Zero references to deprecated symbols remain in codebase

## Task Commits

Each task was committed atomically:

1. **Task 1: Update middleware and test references to use serr directly** - `cb28b9b8` (feat)
2. **Task 2: Remove deprecated re-exports and delete circuit_err.go** - `fee70ff4` (feat)

## Files Created/Modified
- `internal/mcp/middleware.go` - Replaced lspool.ErrCircuitOpen with serr.ErrCircuitOpen
- `internal/mcp/telemetry_middleware_test.go` - Replaced lspool.ErrCircuitOpen and errors.New with serr equivalents
- `internal/mcp/telemetry_span_test.go` - Replaced errors.New with serr.New(serr.Internal, ...)
- `internal/mcp/server.go` - Replaced ErrorDetail usage with serr.Wrap for workspace activation
- `internal/mcp/server_test.go` - Removed tests for deleted ErrorDetail and deprecated sentinels
- `internal/kernel/lspool/circuit_test.go` - Changed ErrCircuitOpen to serr.ErrCircuitOpen
- `internal/mcp/errors.go` - DELETED (all deprecated re-exports removed)
- `internal/kernel/lspool/circuit_err.go` - DELETED (ErrCircuitOpen re-export removed)

## Decisions Made
- Deleted errors.go entirely rather than emptying it -- no non-deprecated content remained
- Replaced ErrorDetail struct usage in server.go activate_project handler with serr.Wrap(serr.NoWorkspace, ...) -- proper typed error instead of struct-based approach
- Removed TestErrorDetail_JSON, TestErrorDetail_JSON_NoSuggestion, and TestDomainErrors tests since they tested deleted types/sentinels

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Fixed additional references to deleted types in server.go, server_test.go, circuit_test.go**
- **Found during:** Task 2 (removing deprecated re-exports)
- **Issue:** Plan only identified middleware.go and telemetry test files as references. server.go used ErrorDetail struct, server_test.go had ErrorDetail and sentinel tests, circuit_test.go used package-level ErrCircuitOpen
- **Fix:** Updated server.go to use serr.Wrap, removed 3 test functions from server_test.go, updated circuit_test.go to use serr.ErrCircuitOpen
- **Files modified:** internal/mcp/server.go, internal/mcp/server_test.go, internal/kernel/lspool/circuit_test.go
- **Verification:** go vet ./... && go test ./... && go build ./cmd/serena all pass
- **Committed in:** fee70ff4 (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Auto-fix was necessary to complete deletion of deprecated types. No scope creep.

## Issues Encountered
None beyond the additional references documented above.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Phase 23 migration is complete across all 8 plans
- All 38+ tools now use typed serr errors
- Zero deprecated re-exports remain
- Full test suite passes with 0 references to legacy error patterns

---
*Phase: 23-tool-migration*
*Completed: 2026-04-15*
