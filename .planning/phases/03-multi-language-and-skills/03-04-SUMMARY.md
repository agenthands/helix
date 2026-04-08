---
phase: 03-multi-language-and-skills
plan: 04
subsystem: kernel
tags: [lsp, quirks, adapter-pattern, language-registry, pool]

requires:
  - phase: 03-multi-language-and-skills
    provides: "Language registry with 52 embedded entries (Plan 01)"
provides:
  - "QuirkAdapter interface with per-language behavioral hooks"
  - "Pool uses language registry for LS resolution instead of hardcoded map"
  - "Dedicated adapters for Go, Rust, C/C++, Java, Vue"
  - "DefaultQuirkAdapter for 30+ low-quirk languages"
affects: [03-multi-language-and-skills, kernel, lspool]

tech-stack:
  added: []
  patterns: ["QuirkAdapter interface pattern for per-language LS behavioral hooks", "Factory function GetQuirkAdapter for adapter resolution from registry entries"]

key-files:
  created:
    - "internal/kernel/lspool/quirks_test.go"
  modified:
    - "internal/kernel/lspool/quirks.go"
    - "internal/kernel/lspool/pool.go"
    - "internal/kernel/lspool/worker.go"
    - "internal/kernel/lspool/pool_test.go"
    - "internal/kernel/kernel.go"

key-decisions:
  - "adapterFactory map for language-to-adapter resolution instead of switch statement"
  - "Notification handler registration deferred -- jsonrpc.Conn lacks RegisterNotificationHandler; all current adapters return nil handlers"

patterns-established:
  - "QuirkAdapter interface: InitOptions, NotificationHandlers, NormalizeSymbolName, PostInitialize"
  - "GetQuirkAdapter factory: returns specific adapter or DefaultQuirkAdapter"

requirements-completed: [LNG-03]

duration: 4min
completed: 2026-04-08
---

# Phase 03 Plan 04: QuirkAdapter Interface and Registry Wiring Summary

**QuirkAdapter interface replacing hardcoded LanguageQuirks with per-language behavioral hooks, wired to language registry for LS resolution**

## Performance

- **Duration:** 4 min
- **Started:** 2026-04-08T09:06:54Z
- **Completed:** 2026-04-08T09:11:40Z
- **Tasks:** 2
- **Files modified:** 6

## Accomplishments
- Evolved LanguageQuirks data struct to QuirkAdapter interface with 4 behavioral hooks (InitOptions, NotificationHandlers, NormalizeSymbolName, PostInitialize)
- Created 6 adapter implementations: Default, Gopls, RustAnalyzer, Clangd, Jdtls, Vue
- Wired Pool to use langregistry.Registry instead of hardcoded DefaultQuirks map
- Worker.Start now uses QuirkAdapter hooks for init options and post-initialization

## Task Commits

Each task was committed atomically:

1. **Task 1: Evolve LanguageQuirks to QuirkAdapter interface** - `63e7ba1e` (feat)
2. **Task 2: Wire pool to use language registry and QuirkAdapter** - `a430b4cd` (feat)

## Files Created/Modified
- `internal/kernel/lspool/quirks.go` - QuirkAdapter interface, 6 adapter implementations, GetQuirkAdapter factory
- `internal/kernel/lspool/quirks_test.go` - 16 tests covering all adapters, factory resolution, behavioral hooks
- `internal/kernel/lspool/pool.go` - Registry field, spawnWorkerLocked uses registry.Get + GetQuirkAdapter
- `internal/kernel/lspool/worker.go` - quirks field, SetQuirks, NormalizeSymbolName, PostInitialize call in Start
- `internal/kernel/lspool/pool_test.go` - Updated to use registry-based pool creation
- `internal/kernel/kernel.go` - NewKernel accepts langregistry.Registry, passes to pool

## Decisions Made
- Used adapterFactory map (language -> constructor func) for clean adapter resolution instead of a switch statement
- Deferred notification handler registration since jsonrpc.Conn does not yet have a RegisterNotificationHandler method; all current adapters return nil for NotificationHandlers anyway
- Clangd adapter searches 4 common compile_commands.json locations (root, build/, cmake-build-debug/, cmake-build-release/)
- JdtlsAdapter.EnsureDataDir is exposed as a public method for explicit use rather than auto-creating in PostInitialize

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Notification handler registration deferred**
- **Found during:** Task 2 (Worker.Start integration)
- **Issue:** Plan specified registering quirks.NotificationHandlers() with JSON-RPC connection, but jsonrpc.Conn has no RegisterNotificationHandler method
- **Fix:** Removed notification handler registration from Worker.Start; all current adapters return nil handlers, so no functional impact
- **Files modified:** internal/kernel/lspool/worker.go
- **Verification:** go build ./... passes, go test ./internal/kernel/... passes
- **Committed in:** a430b4cd (Task 2 commit)

---

**Total deviations:** 1 auto-fixed (1 blocking)
**Impact on plan:** Minor -- notification handlers are a future capability. No current adapter uses them.

## Issues Encountered
None

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- QuirkAdapter interface ready for future language-specific behavioral extensions
- Pool fully wired to language registry for dynamic LS resolution
- Ready for Plan 05 or higher-level integration work

---
*Phase: 03-multi-language-and-skills*
*Completed: 2026-04-08*
