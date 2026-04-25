# Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-25
**Phase:** 56-bug-ls-notification-dispatch-and-jdtls-readiness
**Areas discussed:** Dispatcher wiring point, jdtls readiness contract, Unknown-method + concurrency, Test strategy + Java scope

---

## Dispatcher Wiring Point

| Option | Description | Selected |
|--------|-------------|----------|
| Worker.Start sets via Conn() | Worker.Start assigns p.Conn().OnNotification after ProcessHandle.Start; risk of race against existing inline Listen launch. | ✓ |
| ProcessHandle.Start accepts a handler arg | Pass dispatcher into ProcessHandle.Start; set before Listen launches. | |
| jsonrpc.Conn gains SetOnNotification + start gate | Mutex-guarded setter + locked dispatch read. | |

**User's choice:** Worker.Start sets via Conn() (Recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Move Listen launch out of ProcessHandle.Start | Worker.Start drives Listen after wiring OnNotification. Eliminates race entirely. | ✓ |
| Conn buffers / sync.Once setter | Conn-side machinery to hide ordering. | |
| Worker passes dispatcher into ProcessHandle.Start | Bigger signature change. | |

**User's choice:** Move Listen launch out of ProcessHandle.Start (Recommended)

---

## JDTLS Readiness Contract

| Option | Description | Selected |
|--------|-------------|----------|
| Ship handler + WaitUntilReady, mirror RustAnalyzer | JdtlsAdapter handles language/status; exposes WaitUntilJavaReady mirroring Phase 47 prior art. | ✓ |
| Dispatcher only — readiness deferred to phase 48 | Handlers stay nil; phase 48 designs the contract. | |
| Handler ships, no WaitUntilReady API | Internal flag only; callers poll. | |

**User's choice:** Ship handler + WaitUntilReady, mirror RustAnalyzer (Recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| ServiceReady AND ProjectStatus=OK | Both required, mirrors legacy eclipse_jdtls.py. | ✓ |
| ServiceReady only | Faster gate; index may still be running. | |
| ProjectStatus=OK only | Skips explicit "LS up" signal. | |

**User's choice:** ServiceReady AND ProjectStatus=OK (Recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Phase 56 fixes Java tests too | Wire WaitUntilJavaReady into Java integration tests; mark them green in default go test ./... | ✓ |
| Phase 56 ships infra only; phase 48 owns Java green-up | Cleaner phase boundaries; Java red across two phases. | |

**User's choice:** Phase 56 fixes Java tests too (Recommended)

---

## Unknown-Method + Concurrency

| Option | Description | Selected |
|--------|-------------|----------|
| Debug-log and drop | logger.Debug with method + worker id; zero info+ noise. | ✓ |
| Silently drop | No log at all. | |
| Warn-once-per-method | First occurrence per (worker, method) at Warn. | |

**User's choice:** Debug-log and drop (Recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Synchronous on Listen goroutine | Inline dispatch; handlers must be non-blocking; panic-recover. | ✓ |
| Async via per-conn handler goroutine | Decouples Listen from handler latency; adds buffering. | |
| Sync, with documented contract + runtime warning | Same as sync but with hard contract + slow-handler warning. | |

**User's choice:** Synchronous on Listen goroutine (Recommended)

---

## Test Strategy + Java Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Unit + integration + Java end-to-end | Unit conn dispatch + jdtls payloads; integration regression for rust-analyzer experimental/serverStatus; Java tests pass via WaitUntilJavaReady. | ✓ |
| Unit-heavy, integration only on Java | Skip rust regression; rely on Phase 47 tests. | |
| Unit + Java integration only, no rust regression | Lightest investment. | |

**User's choice:** Unit + integration + Java end-to-end (Recommended)

| Option | Description | Selected |
|--------|-------------|----------|
| Yes — Worker.Start asserts dispatcher non-nil when quirks have handlers | Log Error + return error from Start if wiring is missing. | ✓ |
| No — unit tests are the safety net | Rely on test discipline. | |

**User's choice:** Yes — regression assertion in Worker.Start (Recommended)

---

## Claude's Discretion

- Exact lifecycle method shape (StartListen vs returning the conn).
- Dispatcher closure shape (precomputed map at Worker.Start vs per-call lookup).
- Internal channel/event naming on JdtlsAdapter.
- Java fixture layout for cross-file refs.

## Deferred Ideas

- Telemetry counters on dispatched notifications.
- Async/queued handler dispatch model.
- Warn-once-per-method logging for unknown notifications.
- Intellicode enablement parity with legacy (only fold in if research proves it's needed).
- jdtls restart / cross-worker readiness state.
- Notification handlers for adapters currently returning nil (Vue, TypeScript, Pyright, etc.).
