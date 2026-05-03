# Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness - Context

**Gathered:** 2026-04-25
**Status:** Ready for planning

<domain>
## Phase Boundary

Wire `jsonrpc.Conn.OnNotification` in the worker bootstrap so `QuirkAdapter.NotificationHandlers()` actually receive incoming LS notifications, and use that pipeline to ship a deterministic jdtls readiness signal so Java integration tests pass in default `go test ./...`.

**In scope:**
- Wire OnNotification dispatcher in `internal/kernel/lspool/worker.go` before LS Listen begins.
- Restructure `internal/kernel/lspool/process.go` so Listen is started by the caller (Worker), not inside `ProcessHandle.Start`.
- Implement `JdtlsAdapter.NotificationHandlers()` for `language/status` and a public `WaitUntilJavaReady(ctx)` API on the adapter.
- Make Java integration tests (hover / cross-file refs / `replace_symbol_body`) pass deterministically in default `go test ./...` via the readiness gate.
- Unit + integration test coverage including a real-LS regression test for rust-analyzer's `experimental/serverStatus`.

**Out of scope:**
- jdtls warm-cache reuse across test runs (that's Phase 48).
- Telemetry/metrics for dispatched notifications (defer; raise as a backlog item if needed).
- Async handler dispatch model — handlers stay synchronous.
- Intellicode enablement parity with legacy (defer unless Java tests prove it's needed for the readiness gate; surface during research if so).
- New quirk handlers for adapters that currently return nil (Vue, TypeScript, Pyright, Zls, SourceKit, Intelephense, Markdown, Clangd, jdtls beyond `language/status`).

</domain>

<decisions>
## Implementation Decisions

### Dispatcher Wiring
- **D-01:** `Worker.Start` builds the dispatcher closure from `w.quirks.NotificationHandlers()` and assigns it to `process.Conn().OnNotification` BEFORE Listen is launched.
- **D-02:** `ProcessHandle.Start` no longer launches the Listen goroutine. It sets up pipes + creates the `jsonrpc.Conn` only. A new method (e.g. `ProcessHandle.StartListen(ctx)` or returning the conn for the worker to drive) is called by `Worker.Start` after `OnNotification` is set. This closes the race window completely.
- **D-03:** All existing `ProcessHandle` callers/tests must be updated for the changed lifecycle. Audit `internal/kernel/lspool/process.go`, `worker.go`, and any tests that currently expect Listen to be running after `Start`.

### Unknown-Method Handling
- **D-04:** Dispatcher behavior for incoming methods with no registered handler: `logger.Debug("unhandled LS notification", "method", method, "worker_id", id)` and drop. Zero info+ noise; visible during debugging.

### Handler Concurrency
- **D-05:** Handlers run **synchronously** on the Listen goroutine. RustAnalyzer (atomic store) and jdtls (event flag set) both fit this model. The dispatcher MUST recover from panics in user-supplied handlers — log Error and continue Listen, never crash the worker.
- **D-06:** A short comment on `QuirkAdapter.NotificationHandlers` documents the contract: handlers must be non-blocking (atomic ops, channel signal, mutex-guarded flag flip). No I/O, no LSP calls back to the same conn.

### JDTLS Readiness Contract
- **D-07:** `JdtlsAdapter.NotificationHandlers()` registers a handler for `language/status`. Payload schema (matches legacy `eclipse_jdtls.py:861-867`): `{type: string, message: string}`.
- **D-08:** Readiness gate = **ServiceReady AND ProjectStatus=OK** (both required, mirrors legacy). Two internal events on the adapter (`serviceReadyCh`, `projectReadyCh`); a public `WaitUntilJavaReady(ctx)` blocks until both are closed or the context cancels.
- **D-09:** Adapter API mirrors `RustAnalyzerAdapter`: `WaitUntilJavaReady(ctx) error`, internal `signalServiceReady()` / `signalProjectReady()` are idempotent. On adapter restart (worker recreated) the adapter is fresh — no cross-instance state.
- **D-10:** Java integration test setup (test/integration/java*/) calls `adapter.WaitUntilJavaReady(ctx)` with a generous timeout (e.g. 60s) before issuing symbol queries. Any timeout fails the test loudly; do not silently proceed.

### Regression Assertion
- **D-11:** After `Worker.Start` wires `OnNotification`, if `w.quirks.NotificationHandlers()` returned a non-empty map AND `process.Conn().OnNotification` is still nil, log Error and return an error from `Start`. Makes future refactors that drop the wiring fail loudly in tests.

### Test Coverage
- **D-12:** **Unit** — `jsonrpc.Conn` dispatch lookup: registered method calls handler, unregistered method debug-logs and drops, panicking handler is recovered and Listen continues.
- **D-13:** **Unit** — `JdtlsAdapter.NotificationHandlers()['language/status']` flips `serviceReadyCh` on `{type:"ServiceReady", message:"ServiceReady"}` and `projectReadyCh` on `{type:"ProjectStatus", message:"OK"}`. Malformed payloads are no-ops.
- **D-14:** **Integration regression test** — spin up a real `rust-analyzer` worker, assert `experimental/serverStatus` actually flips `RustAnalyzerAdapter.quiescent`. This regression-proofs the silent-drop bug (without it, Phase 47's WaitUntilRenameReady has been silently never receiving notifications in production).
- **D-15:** **Java integration tests** — hover, cross-file references, and `replace_symbol_body` against the Java fixture must pass in default `go test ./...` (no `-short=false`, no skip), gated by `WaitUntilJavaReady`.

### Claude's Discretion
- Exact method signatures (`StartListen` vs returning the conn) — pick whichever yields the cleanest diff.
- Dispatcher closure shape (precomputed map at Worker.Start vs per-call lookup) — both fine; prefer the precomputed map for clarity.
- Internal channel naming on the JdtlsAdapter — match Phase 47 conventions.
- Java fixture layout: reuse whatever exists in `test/integration/java*/`; add a minimal extra file only if needed for cross-file refs.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Dispatcher + Conn lifecycle
- `internal/kernel/jsonrpc/conn.go` §`Listen`/`OnNotification` (lines 33-34, 157-205) — current dispatch surface; the field that is silently never set in production.
- `internal/kernel/lspool/process.go` §`Start` (around line 60-100) — currently launches `go p.conn.Listen(ctx)` itself. Must be restructured.
- `internal/kernel/lspool/worker.go` §`Start` (around lines 158-238) — where the dispatcher must be wired and Listen must be triggered after.

### Quirk adapter pattern + RustAnalyzer prior art
- `internal/kernel/lspool/quirks.go` §`QuirkAdapter.NotificationHandlers` (line 45-46) — the interface that's been silently never invoked.
- `internal/kernel/lspool/quirks.go` §`RustAnalyzerAdapter.NotificationHandlers` (lines 157-183) — canonical prior art for the readiness pattern (`experimental/serverStatus` → `quiescent` + `readyCh`).
- `internal/kernel/lspool/quirks.go` §`JdtlsAdapter` (lines 304-345) — current shell with `NotificationHandlers() → nil`. To be filled in.
- `internal/kernel/lspool/quirks_test.go` — extend with new dispatch + jdtls readiness tests.

### Phase 47 (rust readiness — the model we mirror)
- `.planning/phases/47-bug-rust-analyzer-rename/` — full prior art including PLAN, RCA, and verification.

### Legacy reference for the jdtls readiness contract
- `legacy/src/solidlsp/language_servers/eclipse_jdtls.py` lines 861-867, 893-901 — canonical `language/status` payload shape, ServiceReady + ProjectStatus=OK gate semantics, intellicode enablement timing.

### Phase neighbors
- Phase 48 (`bug-jdtls-warm-cache`) — consumer of the readiness API. Phase 56 ships the Wait API; Phase 48 layers warm-cache reuse on top. Coordinate via the `WaitUntilJavaReady` contract.
- Phase 49 verification (commit `01811153`) — surfaced this bug.

### Phase 56 directory
- `.planning/phases/56-bug-ls-notification-dispatch-and-jdtls-readiness/` — phase artifacts root.

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `RustAnalyzerAdapter` readiness machinery (`ensureReadyCh`, `signalReady`, `resetReadyCh`, `WaitUntilRenameReady`) is the template for `JdtlsAdapter`. Same idiom: lazy-init channel, idempotent close, mutex-guarded.
- `jsonrpc.Conn.OnNotification` field already exists; the Listen loop already does the lookup and invocation (`conn.go:198-205`). The fix is purely about *setting* it.
- `LSAdapter` (post-init helper) gives quirks a way to call back into the LS for didOpen etc., but is not needed for notification dispatch.

### Established Patterns
- Quirks are optional interfaces (e.g. `ArgsModifier`, `ExperimentalCapabilities`) implemented selectively via type assertions in `Worker.Start`. Notification dispatch is *not* an optional interface — `NotificationHandlers()` is on the base `QuirkAdapter` interface and just returns nil for adapters that don't care.
- Worker.Start has a clear linear bootstrap (apply quirks → start process → initialize → initialized → PostInitialize → replay didOpen). The new dispatcher wiring slots in immediately after `process.Start` and before any LS Listen activity.
- Tests for adapter behavior live in `internal/kernel/lspool/quirks_test.go`; tests for conn-level behavior in `internal/kernel/jsonrpc/`.

### Integration Points
- `Worker.Start` (the single bootstrap path) is the only place that needs the wiring change.
- `ProcessHandle.Start` callers need to follow the lifecycle change (no auto-Listen).
- Java integration tests (`test/integration/java*/`) consume `WaitUntilJavaReady`.
- Phase 48's warm-cache work will layer on top of `WaitUntilJavaReady` — keep the API stable.

</code_context>

<specifics>
## Specific Ideas

- Mirror `RustAnalyzerAdapter` API surface exactly: `WaitUntilJavaReady(ctx) error` parallels `WaitUntilRenameReady(ctx) error`. This consistency is intentional — future quirks gain a recognizable readiness pattern.
- Dispatcher should panic-recover and log Error, never crash Listen. A single misbehaving quirk handler must not take down the worker.
- Regression assertion in `Worker.Start` (D-11) is the most important safety net: this exact bug went undetected because nothing failed when the field was nil.

</specifics>

<deferred>
## Deferred Ideas

- Telemetry counters on dispatched notifications (per worker, per method) — useful for observability but not required to fix the bug. Backlog candidate alongside Phase 53 (obs-metrics-gaps).
- Async/queued handler dispatch — only worth revisiting if a real quirk needs to do non-trivial work in a handler.
- Warn-once-per-method logging for unknown notifications — phase 56 ships debug-log; can be upgraded later if signal-to-noise becomes an issue.
- jdtls intellicode enablement parity with legacy (`java.intellicode.enable` execute_command after `_intellicode_enable_command_available`) — only fold in if research/Java tests prove it's needed for the readiness gate to deliver a working hover/refs surface.
- jdtls restart semantics across worker recycle — current scope assumes a fresh adapter per worker, which matches the rest of the pool. Revisit only if warm-cache work (Phase 48) requires cross-worker readiness state.
- New handlers for adapters returning nil today (Vue, TypeScript, Pyright, etc.). Once dispatch is wired, these become trivial to add per-language as needs arise.

</deferred>

---

*Phase: 56-bug-ls-notification-dispatch-and-jdtls-readiness*
*Context gathered: 2026-04-25*
