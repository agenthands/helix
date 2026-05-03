---
phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness
plan: 02
subsystem: lspool
tags: [lspool, dispatcher, notifications, regression-assertion, d-01, d-04, d-05, d-11]

requires:
  - phase: 56-01
    provides: ProcessHandle.StartListen so Worker.Start can wire OnNotification before Listen
provides:
  - lspool.buildDispatcher (precomputed-map closure, drops unknown methods, recovers panics)
  - lspool.assertDispatcherWired (D-11 regression check)
  - Worker.Start wiring: Conn().OnNotification = buildDispatcher(...) BEFORE process.StartListen(ctx) BEFORE first 'initialize' call
affects:
  - 56-03 (jdtls language/status handler can now actually receive notifications)
  - 56-04 (full-suite + integration gate)

tech-stack:
  added: []
  patterns:
    - "Defensive-copy + closure-capture dispatcher pattern (handlers map cloned at wire-time, T-56-06)"
    - "deferred recover() inside per-method dispatch closure (panic isolation per-call, D-05)"
    - "Loud regression assertion for invariants that were silent in the bug (D-11)"

key-files:
  created:
    - internal/kernel/lspool/worker_test.go
  modified:
    - internal/kernel/lspool/worker.go
    - internal/kernel/jsonrpc/codec_test.go

key-decisions:
  - "TestWorker_DispatcherWired uses Shape B (direct buildDispatcher unit test) per the plan's preferred fallback. Shape A (cat-backed Worker.Start end-to-end) is unfit for unit form because Worker.Start performs a full LSP `initialize` handshake against `cat`, which would never respond — tests would have to fake an entire LSP server. Shape B proves the wiring contract at the unit layer (the closure dispatches correctly when handlers are non-empty), and Plan 01's TestProcessHandle_StartListenSeparate already proves the lifecycle gap is honored at the process layer."
  - "Defensive map copy in buildDispatcher: `routes := make(map[...], len(handlers))` then range-copy. Prevents post-construction mutations of the QuirkAdapter's map from racing with dispatch (T-56-06 mitigation)."
  - "Insertion site in Worker.Start: dispatcher wiring + regression assertion + StartListen sit between `process.Start` and the existing `Initializing` state transition (currently lines 184-208 of worker.go). This places them BEFORE any LSP traffic, satisfying D-01 and D-02 simultaneously."

patterns-established:
  - "Per-method dispatcher with logged-and-dropped unknown methods + recovered panics, owned by the Worker layer. jsonrpc.Conn intentionally stays panic-naive — the breadcrumb comment in codec_test.go documents this for future maintainers."

requirements-completed: [LSDISP-01, LSDISP-03, LSDISP-04, LSDISP-04b]

duration: ~10min
completed: 2026-04-25
---

# Phase 56 Plan 02: Dispatcher wiring in Worker.Start Summary

**Worker.Start now assigns Conn().OnNotification = buildDispatcher(quirks.NotificationHandlers(), ...) BEFORE invoking process.StartListen(ctx); a D-11 regression assertion fails the worker loud if a future refactor drops the wiring while quirks still declare handlers.**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-04-25 (worktree session)
- **Completed:** 2026-04-25
- **Tasks:** 3 (Task 1 RED, Task 2 GREEN, Task 3 codec coverage + gate)
- **Files modified:** 3 (1 created, 2 modified)

## Accomplishments

- `buildDispatcher` (worker.go) — precomputed-map closure: defensive-copies the handlers map, debug-logs and drops unknown methods (D-04), and recovers handler panics with Error-level log (D-05).
- `assertDispatcherWired` (worker.go) — D-11 regression check: returns error and Error-logs if non-empty handlers paired with nil OnNotification; passes silently for empty handlers.
- `Worker.Start` (worker.go:185-205) — wires dispatcher → runs assertion → calls `StartListen(ctx)` BEFORE the `Initializing` state transition and BEFORE the `initialize` LSP request. Closes the silent-drop window opened by Plan 01.
- `TestBuildDispatcher_KnownMethod`, `_UnknownMethodLogsAndDrops`, `_PanicRecovers` (worker_test.go) — pure-function unit coverage of dispatcher contract; uses bytes.Buffer-backed slog handlers to capture log lines.
- `TestWorker_DispatcherWired` (worker_test.go) — LSDISP-01 unit-level proof; explicitly NOT deferred to integration.
- `TestWorker_DispatcherWiringRegression` (worker_test.go) — D-11 sub-tests: wired-ok, broken (errors + logs at ERROR), empty-handlers (no-op nil return).
- `TestConn_NotificationUnknownMethod` (codec_test.go) — LSDISP-04b: regression bait against any future Conn-layer method allow-listing.
- One-line breadcrumb in codec_test.go pointing future maintainers at `TestBuildDispatcher_PanicRecovers` and documenting the deliberate Conn-layer no-recover stance.

## Task Commits

1. **Task 1: Scaffold worker_test.go (RED)** — `ba174277` (test)
2. **Task 2: Implement dispatcher wiring + regression assertion in Worker.Start (GREEN)** — `7884fdc6` (feat)
3. **Task 3: Extend codec_test.go for unknown-method coverage + breadcrumb** — `42484610` (test)

_TDD plan; no separate refactor commit needed — Task 2 GREEN is already minimal._

## Files Created/Modified

- `internal/kernel/lspool/worker_test.go` (created, 133 lines) — five required test functions plus a `nopRWC` helper for jsonrpc.Conn construction in `TestWorker_DispatcherWiringRegression`.
- `internal/kernel/lspool/worker.go` (modified, +76 lines) — added `encoding/json` import, dispatcher wiring block in `Worker.Start` (lines 185-205), and the two helper functions (`buildDispatcher`, `assertDispatcherWired`) appended at file end after `processID`.
- `internal/kernel/jsonrpc/codec_test.go` (modified, +36 lines) — added `TestConn_NotificationUnknownMethod` after `TestConn_Notification_Handler` plus the breadcrumb comment block.

### Worker field names observed (per plan's instruction to verify)

- Worker ID accessor: bare field `w.id` (also exposed via `w.ID()` method, but the file-internal call sites consistently use `w.id`).
- Logger: `w.logger` (a `*slog.Logger` set in `NewWorker` with `worker_id`/`language` attrs already attached).
- State store: `w.state.Store(int32(WorkerStopped))` for the failure path of `assertDispatcherWired`.

### Exact insertion lines in worker.go

- Dispatcher wiring block: lines 185-205 (between `process.Start` success at line 182 and the Initializing state transition).
- `OnNotification = buildDispatcher(...)` on line 193.
- `w.process.StartListen(ctx)` on line 205.
- `buildDispatcher` definition: appended at end of file.
- `assertDispatcherWired` definition: appended after `buildDispatcher`.

### Shape A vs Shape B for TestWorker_DispatcherWired

Chose **Shape B** (direct `buildDispatcher` unit test). Rationale documented in the test's doc-comment: Shape A would require driving `Worker.Start` end-to-end with a `cat`-backed `ProcessHandle`, but `Worker.Start` performs a full `initialize` handshake against the subprocess. `cat` echoes the request bytes back as garbage and never produces a valid JSON-RPC response, so `Worker.Start` would block forever on `Conn().Call(ctx, "initialize", ...)`. Faking a complete LSP `initialize` server inside a unit test would dwarf the test it supports. Shape B proves the wiring contract at the unit layer; Plan 01's `TestProcessHandle_StartListenSeparate` already proves the process-layer lifecycle gap is honored.

## Decisions Made

- **Shape B for LSDISP-01** (see above).
- **Defensive map copy** at dispatcher construction time mitigates T-56-06 (post-construction mutation racing with dispatch). The cost is one allocation per worker startup — negligible.
- **Wiring sits before the Initializing state transition.** Earlier placement isn't possible (process not yet started); later placement risks dropping notifications emitted by the LS during/after `initialize`. The chosen site is the only correct one.
- **Empty handlers short-circuit** in `assertDispatcherWired`: workers whose adapter declines to register any notification handlers (e.g., `DefaultQuirkAdapter`) intentionally leave `OnNotification == nil`. Treating that as a wiring failure would break every default-quirk language. The check only fires when `len(handlers) > 0`.

## Deviations from Plan

None — plan executed exactly as written, with two clarifications worth recording for downstream waves:

1. **VALIDATION.md row LSDISP-03 was already correct.** The planner output already pointed row LSDISP-03 at `TestBuildDispatcher_PanicRecovers` in `internal/kernel/lspool/...`. Task 3's instruction to "fix it now" was a no-op. The legacy `TestConn_NotificationHandlerPanic` token still appears once in the file — only inside the explicit "Revision note" prose explaining the migration, NOT in any active table row. The plan's `<verify>` regex `LSDISP-03.*TestConn_NotificationHandlerPanic` would technically flag the revision note line because both substrings co-occur on it; we kept the revision note for traceability rather than scrubbing legitimate documentation. The acceptance criterion's intent ("LSDISP-03 row no longer references the retired Conn-layer test name") is fully met (row at line 48 references only `TestBuildDispatcher_PanicRecovers`).
2. **`go vet ./...` emits a pre-existing CGO warning** from `internal/treesitter/bindings/swift/src/scanner.c` (TOKEN_COUNT macro redefined) that is entirely unrelated to this plan. Exit code is still 0 (warning, not error). Out of scope per Phase 56.

## Issues Encountered

- **None new.** PreToolUse hooks fired READ-BEFORE-EDIT reminders during the worker.go and codec_test.go edits; both files were already read in this session, the runtime accepted the edits, and execution continued without re-reading.

## Threat Mitigation Verification

| Threat ID | Status | Evidence |
|-----------|--------|----------|
| T-56-03 (DoS via panicking handler) | mitigated | `TestBuildDispatcher_PanicRecovers` proves recovery + Error log + continued operation |
| T-56-04 (Tampering: silent dispatcher drop) | mitigated | `assertDispatcherWired` + `TestWorker_DispatcherWiringRegression` cover the broken-wiring case |
| T-56-05 (Info disclosure via debug log) | accepted | Debug log carries method name + worker_id only, no payload bytes |
| T-56-06 (Tampering: post-wire map mutation race) | mitigated | `routes := make(...)` + range-copy in buildDispatcher; documented in doc-comment |

## Next Plan Readiness

- Plan 56-03 (jdtls `language/status` readiness) can now register a notification handler via `JdtlsAdapter.NotificationHandlers()` and trust that it will actually be invoked by `Worker.Start`'s dispatcher.
- Production paths through `Worker.Start` are now correct: notifications received between `process.Start` and `process.StartListen` are no longer dropped — Listen does not start until after the dispatcher is wired.
- Wave 2 ready to merge into Wave 3.

## Self-Check: PASSED

- `internal/kernel/lspool/worker_test.go` — FOUND
- `internal/kernel/lspool/worker.go` — modified (FOUND)
- `internal/kernel/jsonrpc/codec_test.go` — modified (FOUND)
- Commit `ba174277` — FOUND in `git log`
- Commit `7884fdc6` — FOUND in `git log`
- Commit `42484610` — FOUND in `git log`
- `grep -q 'func buildDispatcher' internal/kernel/lspool/worker.go` — VERIFIED
- `grep -q 'func assertDispatcherWired' internal/kernel/lspool/worker.go` — VERIFIED
- `grep -q 'OnNotification = buildDispatcher' internal/kernel/lspool/worker.go` — VERIFIED
- `grep -q 'w\.process\.StartListen(ctx)' internal/kernel/lspool/worker.go` — VERIFIED
- All five required test functions present in worker_test.go — VERIFIED
- `grep -q 'func TestConn_NotificationUnknownMethod' internal/kernel/jsonrpc/codec_test.go` — VERIFIED
- `grep -q 'TestBuildDispatcher_PanicRecovers' internal/kernel/jsonrpc/codec_test.go` — VERIFIED (breadcrumb)
- `grep -q 'TestBuildDispatcher_PanicRecovers' .planning/phases/56-bug-ls-notification-dispatch-and-jdtls-readiness/56-VALIDATION.md` — VERIFIED
- `go test ./internal/kernel/jsonrpc/... ./internal/kernel/lspool/... -count=1` exits 0 — VERIFIED
- `go vet ./...` exits 0 (with pre-existing unrelated CGO warning) — VERIFIED

---
*Phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness*
*Completed: 2026-04-25*
