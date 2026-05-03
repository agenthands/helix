# Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness — Research

**Researched:** 2026-04-25
**Domain:** LSP notification dispatch wiring + jdtls readiness signal (Go, jsonrpc, lspool)
**Confidence:** HIGH (all claims verified against the live codebase or legacy reference)

---

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Dispatcher Wiring**
- **D-01:** `Worker.Start` builds the dispatcher closure from `w.quirks.NotificationHandlers()` and assigns it to `process.Conn().OnNotification` BEFORE Listen is launched.
- **D-02:** `ProcessHandle.Start` no longer launches the Listen goroutine. It sets up pipes + creates the `jsonrpc.Conn` only. A new method (e.g. `ProcessHandle.StartListen(ctx)` or returning the conn for the worker to drive) is called by `Worker.Start` after `OnNotification` is set. Closes the race window completely.
- **D-03:** All existing `ProcessHandle` callers/tests must be updated for the changed lifecycle.

**Unknown-Method Handling**
- **D-04:** Dispatcher behavior for incoming methods with no registered handler: `logger.Debug("unhandled LS notification", "method", method, "worker_id", id)` and drop.

**Handler Concurrency**
- **D-05:** Handlers run synchronously on the Listen goroutine. Dispatcher MUST recover from panics in user-supplied handlers — log Error and continue Listen, never crash the worker.
- **D-06:** Doc comment on `QuirkAdapter.NotificationHandlers` documents: handlers must be non-blocking (atomic ops, channel signal, mutex-guarded flag flip). No I/O, no LSP calls back to the same conn.

**JDTLS Readiness Contract**
- **D-07:** `JdtlsAdapter.NotificationHandlers()` registers a handler for `language/status`. Payload schema: `{type: string, message: string}`.
- **D-08:** Readiness gate = ServiceReady AND ProjectStatus=OK (both required, mirrors legacy). Two internal events on the adapter (`serviceReadyCh`, `projectReadyCh`); a public `WaitUntilJavaReady(ctx)` blocks until both are closed or the context cancels.
- **D-09:** Adapter API mirrors `RustAnalyzerAdapter`: `WaitUntilJavaReady(ctx) error`, internal `signalServiceReady()` / `signalProjectReady()` are idempotent. On adapter restart the adapter is fresh — no cross-instance state.
- **D-10:** Java integration test setup calls `adapter.WaitUntilJavaReady(ctx)` with a generous timeout (e.g. 60s) before issuing symbol queries. Any timeout fails the test loudly.

**Regression Assertion**
- **D-11:** After `Worker.Start` wires `OnNotification`, if `w.quirks.NotificationHandlers()` returned a non-empty map AND `process.Conn().OnNotification` is still nil, log Error and return an error from `Start`.

**Test Coverage**
- **D-12:** Unit — `jsonrpc.Conn` dispatch lookup: registered method calls handler, unregistered method debug-logs and drops, panicking handler is recovered and Listen continues.
- **D-13:** Unit — `JdtlsAdapter.NotificationHandlers()['language/status']` flips `serviceReadyCh` on `{type:"ServiceReady", message:"ServiceReady"}` and `projectReadyCh` on `{type:"ProjectStatus", message:"OK"}`. Malformed payloads are no-ops.
- **D-14:** Integration regression test — spin up a real `rust-analyzer` worker, assert `experimental/serverStatus` actually flips `RustAnalyzerAdapter.quiescent`.
- **D-15:** Java integration tests — hover, cross-file references, and `replace_symbol_body` against the Java fixture must pass in default `go test ./...`, gated by `WaitUntilJavaReady`.

### Claude's Discretion

- Exact method signatures (`StartListen` vs returning the conn) — pick whichever yields the cleanest diff.
- Dispatcher closure shape (precomputed map at Worker.Start vs per-call lookup) — prefer precomputed map for clarity.
- Internal channel naming on the JdtlsAdapter — match Phase 47 conventions.
- Java fixture layout: reuse what exists in `test/integration/java*/`; add a minimal extra file only if needed for cross-file refs.

### Deferred Ideas (OUT OF SCOPE — DO NOT RESEARCH)

- Telemetry counters on dispatched notifications.
- Async/queued handler dispatch model.
- Warn-once-per-method logging for unknown notifications.
- Intellicode enablement parity with legacy (only fold in if research proves it's needed — see §5 below).
- jdtls restart / cross-worker readiness state.
- Notification handlers for adapters currently returning nil (Vue, TypeScript, Pyright, etc.).
- jdtls warm-cache reuse (Phase 48).
</user_constraints>

<phase_requirements>
## Phase Requirements

This phase has **no formal REQ-ID** in `.planning/REQUIREMENTS.md` (verified — REQ table only covers BUG-01..BUG-04, TOOL-01..02, PKG-01..04, OBS-01..04). Phase 56 was added to ROADMAP.md (line 200-208) AFTER milestone v1.9 was opened, as a discovered prerequisite for **BUG-03 (Phase 48)**.

**Implicit requirements from CONTEXT.md domain block:**

| ID (synthetic) | Description | Research Support |
|----------------|-------------|------------------|
| LSDISP-01 | `jsonrpc.Conn.OnNotification` is wired in production for every LS worker that registers handlers (no silent drop). | §Architecture Patterns / Pattern 1 (Worker.Start dispatcher wiring), §Common Pitfalls (Pitfall 1) |
| LSDISP-02 | `ProcessHandle.Start` lifecycle is restructured so Listen is started by `Worker.Start` AFTER `OnNotification` is set. | §Architecture Patterns / Pattern 2 (ProcessHandle lifecycle restructure) |
| LSDISP-03 | Dispatcher panic-recovers and continues Listen on handler crash. | §Code Examples / Panic-recovering dispatcher closure |
| LSDISP-04 | Worker.Start emits an Error and returns an error if quirks declare handlers but `OnNotification` is unset (regression assertion). | §Architecture Patterns / Pattern 3 |
| JDTLS-RDY-01 | `JdtlsAdapter` exposes `WaitUntilJavaReady(ctx) error` mirroring `RustAnalyzerAdapter.WaitUntilRenameReady`. | §Standard Stack / RustAnalyzerAdapter prior art (`quirks.go:106-251`); §Code Examples |
| JDTLS-RDY-02 | The Java integration suite (`test/integration/java_test.go`) passes deterministically in default `go test ./...`. | §Validation Architecture; §Code Context (java_test.go has no build tag — already runs by default) |
| LSDISP-REG-01 | A real-LS regression test for rust-analyzer's `experimental/serverStatus` runs in the integration tag, asserting the adapter's `quiescent` flag flips. | §Validation Architecture; §Common Pitfalls (Pitfall 1) |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** (CLAUDE.md §Go Development Commands)
- **Single Go binary** — no Python or runtime dependencies introduced; legacy/ is read-only reference.
- **MCP-first** — no behavior changes to MCP tool surface; Phase 56 is plumbing-only.
- **No upstream LS forks** — the readiness gate must live in `internal/kernel/lspool/quirks.go`, not in jdtls itself.
- **GSD workflow** — file changes flow through `/gsd-execute-phase`.

---

## Summary

Phase 56 fixes a **silent dispatch bug** (`jsonrpc.Conn.OnNotification` is `nil` in every production worker) and ships the **deterministic Java readiness signal** that depends on it. The bug has been latent since Phase 47 — `RustAnalyzerAdapter.WaitUntilRenameReady` has technically never received `experimental/serverStatus` notifications in production; it has only been observed working in unit tests that invoke the handler closure directly.

The fix has three structural moves:

1. **Restructure `ProcessHandle.Start`** (process.go:58-97) so it does NOT launch `go p.conn.Listen(ctx)`. Add a separate `StartListen(ctx)` method (or equivalent) that the caller drives.
2. **Wire the dispatcher in `Worker.Start`** (worker.go:158-268) immediately after `process.Start` returns and BEFORE the LSP `initialize` call (the conn must be listening for `initialize` to receive its response — so Listen MUST be triggered by Worker.Start before the first `Conn().Call`).
3. **Implement `JdtlsAdapter.NotificationHandlers()` + `WaitUntilJavaReady(ctx)`** as a near-identical mirror of `RustAnalyzerAdapter`, using two events (ServiceReady channel + ProjectStatus=OK channel) closed via idempotent `signalXxx()` helpers under a shared mutex.

**Primary recommendation:** Use the **precomputed map closure** for the dispatcher (D-discretion); use a **`StartListen(ctx)` method** on `ProcessHandle` (cleanest diff: only 1 call site needs adjusting in worker.go and 1 in jsonrpc/codec_test.go which calls Listen directly). Mirror Phase 47's lazy-channel + signalReady idiom verbatim — twice (one per readiness signal) — and have `WaitUntilJavaReady` block on both via a small select-and-loop.

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| LS process lifecycle (pipes, fork) | `internal/kernel/lspool/process.go` (ProcessHandle) | — | Owns OS-level LS process; should NOT own protocol-level dispatch wiring. |
| JSON-RPC dispatch (Listen + OnNotification) | `internal/kernel/jsonrpc/conn.go` (Conn) | — | Already correctly placed — has the field and the dispatch loop. The bug is purely "field never set". |
| Notification handler registration | `internal/kernel/lspool/quirks.go` (per-language `*Adapter`) | — | Each adapter owns its own readiness state; no cross-adapter coupling. |
| Dispatcher wiring (gluing `quirks.NotificationHandlers()` → `Conn.OnNotification`) | `internal/kernel/lspool/worker.go` (Worker.Start) | — | Worker is the only place that knows BOTH the quirks adapter AND the conn. CONTEXT D-01 confirms this assignment. |
| Java readiness API | `internal/kernel/lspool/quirks.go` (`JdtlsAdapter.WaitUntilJavaReady`) | `test/integration/java_test.go` (consumer) | Mirrors `RustAnalyzerAdapter`; consumed by integration tests via type-assertion on `Worker.Quirks()`. |

---

## Standard Stack

### Core (already in tree, NO new dependencies)
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `internal/kernel/jsonrpc` | in-tree | JSON-RPC 2.0 codec, `Conn.Listen`, `Conn.OnNotification` field | Already implements the dispatch contract — only the field assignment is missing. |
| `internal/kernel/lspool` | in-tree | Worker / ProcessHandle / QuirkAdapter | Phase boundary lives entirely here. |
| `log/slog` | stdlib | Debug log for unhandled methods (D-04), Error log for panic recovery (D-05). | Project standard — `Worker.logger` is already a `*slog.Logger` (worker.go:141). |
| `sync` + `sync/atomic` | stdlib | Same primitives RustAnalyzerAdapter uses (`sync.Mutex` + `atomic.Bool`). | Project pattern — see `quirks.go:124-131`. |

### Supporting
| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `encoding/json` | stdlib | Unmarshal `language/status` payload — same as `RustAnalyzerAdapter` does for `experimental/serverStatus`. | In the JdtlsAdapter status handler. |
| `github.com/stretchr/testify/require` + `assert` | already vendored | Project test idiom (see `quirks_test.go:12-14`). | All new unit tests. |

### Alternatives Considered

| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Precomputed map closure | Per-call lookup against `quirks.NotificationHandlers()` | Per-call rebuilds the map per notification — wasteful. CONTEXT marks precomputed as preferred. |
| `ProcessHandle.StartListen(ctx)` | Return `*Conn` from `Start` and let Worker call `conn.Listen(ctx)` directly | Either works; `StartListen` preserves encapsulation (Conn lifecycle still owned by ProcessHandle) and is the smaller diff because callers already use `process.Conn()`. **Recommend StartListen.** |
| Two separate channels (D-08) | Single `readyCh` closed when both flags set | Two-channel approach matches legacy semantics 1:1, allows independent reset on adapter recycle, and makes test assertions sharper. |

**No `go get` / `npm install` needed — pure refactor + new code in existing packages.**

**Version verification:** N/A — no external dependencies introduced. `go.mod` unchanged.

---

## Architecture Patterns

### System Architecture Diagram

```
Worker.Start(ctx)                            ProcessHandle                         jsonrpc.Conn
─────────────────                            ─────────────                         ────────────

  1. Apply quirks (InitOptions, ExtraArgs)
  2. NewProcessHandle(...)
  3. process.Start(ctx, sessionPrefix) ──►   set up pipes
                                             cmd.Start()
                                             NewConn(rwc, prefix) ──►              create Conn
                                             go p.drainStderr()
                                             go p.reap()
                                             go p.watchContext(ctx)
                                             [NO Listen launch — D-02]
                                       ◄──── return nil
  4. handlers := w.quirks.NotificationHandlers()
  5. dispatcher := buildDispatcher(handlers, logger, w.id)  ── [precomputed map + recover]
  6. process.Conn().OnNotification = dispatcher  ────────────────────────────────►  field set
  7. REGRESSION ASSERT: if len(handlers) > 0 && Conn().OnNotification == nil → return error  [D-11]
  8. process.StartListen(ctx)            ──► go p.conn.Listen(ctx)  ─────────────►  dispatch loop runs:
                                                                                    incoming notif → OnNotification(method, params)
                                                                                                       │
                                                                                                       ▼
                                                                                    dispatcher: recover-wrapped handler[method] OR debug-log+drop
  9. process.Conn().Call(ctx, "initialize", ...)
 10. ... (initialized, PostInitialize, replay didOpen, Ready)


JdtlsAdapter.NotificationHandlers()
   "language/status" ──► parse {type, message}
                         if type=="ServiceReady" && message=="ServiceReady" → signalServiceReady() (idempotent close)
                         if type=="ProjectStatus" && message=="OK"        → signalProjectReady() (idempotent close)

JdtlsAdapter.WaitUntilJavaReady(ctx)
   ensureServiceReadyCh()      ┐
   ensureProjectReadyCh()      ├─► select { both closed → nil ; ctx.Done → ctx.Err ; timer → timeout error }
                               ┘
```

### Recommended Project Structure

```
internal/kernel/
├── jsonrpc/
│   └── conn.go              # NO change to dispatch logic; dispatch tests (D-12) extended in conn_test.go / codec_test.go
├── lspool/
│   ├── process.go           # MODIFY: Start drops `go p.conn.Listen(ctx)`; ADD StartListen(ctx)
│   ├── worker.go            # MODIFY: Start wires dispatcher + StartListen + regression assertion
│   ├── quirks.go            # MODIFY: JdtlsAdapter gains state + handler + WaitUntilJavaReady (mirror of RustAnalyzer §106-251)
│   └── quirks_test.go       # EXTEND: TestJdtlsAdapter_LanguageStatusReadiness, malformed payload, integration assertion
└── ...

test/integration/
├── java_test.go             # MODIFY: Wait for JdtlsAdapter.WaitUntilJavaReady before issuing tool calls (or rely on harness gate)
└── rust_test.go             # EXTEND: Add regression test — real rust-analyzer flips quiescent (under //go:build integration)
```

### Pattern 1: Worker.Start Dispatcher Wiring (canonical implementation)

**What:** Immediately after `process.Start(ctx, w.id)` returns, build the dispatcher and assign it to `process.Conn().OnNotification`. THEN call `process.StartListen(ctx)`. THEN call `process.Conn().Call(ctx, "initialize", ...)`.

**When to use:** Always — this is the only correct ordering.

**Example:**
```go
// Source: synthesized from worker.go:178-182 (current) + CONTEXT D-01..D-02
if err := w.process.Start(ctx, w.id); err != nil {
    w.state.Store(int32(WorkerStopped))
    return fmt.Errorf("starting LS process: %w", err)
}

// D-01 + D-02: wire dispatcher BEFORE Listen, BEFORE first LSP call.
var handlers map[string]func(json.RawMessage)
if w.quirks != nil {
    handlers = w.quirks.NotificationHandlers()
}
if len(handlers) > 0 {
    w.process.Conn().OnNotification = buildDispatcher(handlers, w.logger, w.id)

    // D-11: regression assertion — paranoid check that we actually wired it.
    if w.process.Conn().OnNotification == nil {
        w.logger.Error("dispatcher wiring lost between assignment and check",
            "worker_id", w.id, "handler_count", len(handlers))
        _ = w.process.Stop(ctx)
        w.state.Store(int32(WorkerStopped))
        return fmt.Errorf("dispatcher wiring failed for worker %s", w.id)
    }
}

// D-02: now safe to start the dispatch loop.
w.process.StartListen(ctx)
```

### Pattern 2: ProcessHandle Lifecycle Restructure

**What:** Split `ProcessHandle.Start` into "spawn process + create conn" and "begin reading".

**When to use:** Required by D-02 to close the race window between `OnNotification` assignment and the first incoming notification.

**Example:**
```go
// Source: synthesized from process.go:58-97 + CONTEXT D-02
func (p *ProcessHandle) Start(ctx context.Context, sessionPrefix string) error {
    // ... pipes + cmd.Start() unchanged ...
    rwc := &stdinStdoutRWC{stdin: p.stdin, stdout: p.stdout}
    p.conn = jsonrpc.NewConn(rwc, sessionPrefix)
    go p.drainStderr()
    // REMOVED: go p.conn.Listen(ctx)
    go p.reap()
    go p.watchContext(ctx)
    return nil
}

// StartListen begins the JSON-RPC dispatch loop. Caller MUST set
// Conn().OnNotification (if needed) before invoking StartListen — otherwise
// notifications received before assignment are silently dropped (this is the
// exact bug Phase 56 fixes; see jsonrpc.Conn.Listen comment).
func (p *ProcessHandle) StartListen(ctx context.Context) {
    go p.conn.Listen(ctx)
}
```

### Pattern 3: Regression Assertion (D-11)

**What:** After wiring, fail loudly if a future refactor drops the assignment.

**Why:** The exact bug went undetected because nothing ever failed when the field was nil. The assertion makes Worker.Start crash-loud the moment a handler-bearing adapter is started against an unwired conn.

**Example:** see Pattern 1 §`if w.process.Conn().OnNotification == nil`.

### Pattern 4: JdtlsAdapter Readiness Mirror

**What:** Two-channel readiness, idempotent signal helpers, public `WaitUntilJavaReady(ctx) error`.

**Example:**
```go
// Source: mirror of quirks.go:119-251 (RustAnalyzerAdapter), legacy/eclipse_jdtls.py:861-867 for payload schema
type JdtlsAdapter struct {
    Entry langregistry.LSEntry

    readyMu       sync.Mutex
    serviceReady  chan struct{}  // closed when ServiceReady seen
    projectReady  chan struct{}  // closed when ProjectStatus=OK seen
}

func (j *JdtlsAdapter) NotificationHandlers() map[string]func(json.RawMessage) {
    return map[string]func(json.RawMessage){
        "language/status": func(raw json.RawMessage) {
            var s struct{ Type, Message string }
            if err := json.Unmarshal(raw, &s); err != nil {
                return // malformed → no-op (D-13)
            }
            if s.Type == "ServiceReady" && s.Message == "ServiceReady" {
                j.signalServiceReady()
            }
            if s.Type == "ProjectStatus" && s.Message == "OK" {
                j.signalProjectReady()
            }
        },
    }
}

// WaitUntilJavaReady blocks until BOTH ServiceReady and ProjectStatus=OK have
// been observed via language/status notifications, ctx is cancelled, or a
// generous internal timeout elapses. Returns nil iff both gates closed.
func (j *JdtlsAdapter) WaitUntilJavaReady(ctx context.Context) error {
    svc := j.ensureServiceReadyCh()
    proj := j.ensureProjectReadyCh()
    select {
    case <-svc:
    case <-ctx.Done():
        return ctx.Err()
    }
    select {
    case <-proj:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

### Anti-Patterns to Avoid

- **Setting `OnNotification` AFTER `Conn.Call(initialize)`** — `initialize` returns a *response* (not notification) so this technically works for the response, but ANY mid-handshake notification (e.g., `window/showMessage`, jdtls's `language/status` early-fire of `Starting…`) sent before assignment is dropped. The wiring MUST happen before `StartListen`, which must happen before any `Call`.
- **Returning `*jsonrpc.Conn` from `ProcessHandle.Start`** — works, but breaks the encapsulation pattern already established (`ProcessHandle.Conn()` is the accessor). Prefer `StartListen(ctx)`.
- **Reusing the `RustAnalyzerAdapter.signalReady` shape verbatim with TWO calls to `ensureReadyCh`** — the lazy-channel idiom is per-channel; you need TWO `ensureXxxCh` helpers (one per signal). Don't try to share state.
- **Doing I/O in the `language/status` handler** — D-06: handlers are non-blocking. Channel close + atomic flag only.

---

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Channel-based one-shot readiness | A `sync.Once` + condition variable + waiters list | The lazy-channel + `select`-on-closed pattern from `RustAnalyzerAdapter.WaitUntilRenameReady` (`quirks.go:185-251`) | Already in tree, already tested, already mirrored in expected API surface. |
| JSON-RPC dispatch loop | A new method on Worker that reads frames | `jsonrpc.Conn.Listen(ctx)` (already implemented `conn.go:159-230`) | The dispatch loop already exists and correctly handles responses, notifications, and unknowns. |
| Per-method handler routing inside `Conn` | A new `RegisterHandler(method, fn)` API on Conn | The `OnNotification NotificationFunc` callback (`conn.go:34`) — Worker builds the routing closure | Lower-level Conn shouldn't know about LS-specific methods; routing is a Worker concern. CONTEXT D-01 confirms. |
| Panic-recover wrapper | A new `internal/safety` package | Inline `defer func(){ recover() }()` inside the dispatcher closure | Single use site; no need for a package. (Project pattern: `internal/kernel/edit/rename_override_test.go:137` uses inline recover too.) |
| Java fixture for cross-file refs | New multi-package layout | The existing `testdata/fixtures/java/{Main.java, Greeter.java}` — already has a 2-file cross-file ref scenario in `java_test.go:80-90` (`Greeter` interface referenced from `Main.java`) | Already shipped; no fixture work needed. |

**Key insight:** All the building blocks exist. Phase 56 is **wiring + mirroring**, not invention.

---

## Common Pitfalls

### Pitfall 1: Listen launched before OnNotification assigned (THE BUG)

**What goes wrong:** `ProcessHandle.Start` does `go p.conn.Listen(ctx)` inline (process.go:88). At that moment `Conn.OnNotification` is `nil`. Any notification arriving before Worker wires it (or ANY notification at all, since Worker NEVER wires it today) is silently dropped at `conn.go:198` (`if c.OnNotification != nil` — false branch is silent).

**Why it happens:** The dispatch-callback contract was added to `Conn` as an *optional* field with no failure mode if unset. Worker.Start was written before quirks adapters had `NotificationHandlers()`, and the wiring step was never added when it was. Phase 47's tests exercise `RustAnalyzerAdapter.NotificationHandlers()['experimental/serverStatus'](raw)` *directly* via the unit test (`quirks_test.go:31`), bypassing the dispatch path entirely. Result: a 100% green test suite over a 100% broken production path.

**How to avoid:** D-02 (move Listen out of ProcessHandle.Start) + D-01 (Worker.Start wires before StartListen) + D-11 (regression assertion).

**Warning signs:** A "readiness" or "wait" API that callers always invoke with `_ = ` (best-effort), and a unit test that only hits the handler closure — never the dispatch path.

### Pitfall 2: `initialize` response races with first notification

**What goes wrong:** If you assign `OnNotification` *between* `Conn.Call("initialize", ...)` and `Conn.Notify("initialized", ...)`, the handshake works but jdtls sometimes emits `language/status: Starting` (and other early notifications) DURING the initialize round-trip. Those get dropped.

**Why it happens:** `Conn.Call` requires `Listen` to be running to receive the response. So Listen MUST be started before `Call`. So `OnNotification` MUST be set before `StartListen`. So the assignment must happen between `process.Start` and `process.StartListen`.

**How to avoid:** Follow Pattern 1 ordering exactly. The regression assertion (D-11) catches this if a future refactor reorders.

**Warning signs:** Flaky tests that pass on cold-start but fail on warm-cache (or vice versa) — symptomatic of timing-dependent notification delivery.

### Pitfall 3: `WaitUntilJavaReady` deadlocks if jdtls never emits ProjectStatus=OK

**What goes wrong:** Legacy `eclipse_jdtls.py:921-927` has a hardcoded 20-second hotfix timeout and proceeds anyway with a warning, because jdtls intermittently never sends `ProjectStatus=OK` even after indexing succeeds.

**Why it happens:** jdtls bug — well-documented in legacy comment "Hotfix: Using timeout until we figure out why sometimes we don't get the project ready event".

**How to avoid:** `WaitUntilJavaReady(ctx)` MUST honor a context deadline cleanly. Tests pass `60s` timeout. Adapter could optionally add an internal `javaReadinessTimeout` constant (mirror Phase 47's `renameReadinessTimeout = 10s` at quirks.go:37) for a default-bound when ctx has no deadline. **Recommend:** internal `javaReadinessTimeout = 90s` default; ctx deadline takes precedence if shorter. Return a typed error on timeout vs. on ctx cancel so tests can distinguish.

**Warning signs:** Test fixture passes hover/refs but `WaitUntilJavaReady` itself returns ctx.DeadlineExceeded — that's the legacy hotfix scenario. May need a "ServiceReady-only" graceful degradation path, but ONLY if observed in practice (D-08 says both required; do not weaken the gate prophylactically).

### Pitfall 4: Forgetting to also fix `jsonrpc/codec_test.go`

**What goes wrong:** `codec_test.go:199` and `:246` call `conn.Listen(ctx)` directly in tests. These are unit tests of `Conn`, not `ProcessHandle`, so they don't change shape — BUT make sure no test mocks `ProcessHandle.Start` with the assumption Listen has been launched.

**How to avoid:** Search `grep -rn "p.conn.Listen\|ProcessHandle\|NewProcessHandle"` — verified, only `process.go` and `worker.go` are callers (no test directly constructs ProcessHandle). Confirmed safe.

**Warning signs:** None expected — clean.

### Pitfall 5: Panic-recover swallows real bugs in handler logic

**What goes wrong:** A buggy handler that always panics will keep panicking on every notification, but recover() silently logs and continues — masking the bug for weeks.

**How to avoid:** Log at **Error** level (D-05), include `worker_id`, `method`, and stringified panic value. Never demote to Warn or Debug.

---

## Code Examples

### Example 1: Panic-recovering dispatcher closure (D-05 + D-04)

```go
// Source: synthesized to satisfy CONTEXT D-04 + D-05; precomputed-map per CONTEXT discretion.
// File location: internal/kernel/lspool/worker.go (helper function)
func buildDispatcher(
    handlers map[string]func(json.RawMessage),
    logger *slog.Logger,
    workerID string,
) jsonrpc.NotificationFunc {
    // Defensive copy so future quirk reconfigurations don't mutate the live map.
    routes := make(map[string]func(json.RawMessage), len(handlers))
    for k, v := range handlers {
        routes[k] = v
    }
    return func(method string, params json.RawMessage) {
        h, ok := routes[method]
        if !ok {
            // D-04: debug-log and drop.
            logger.Debug("unhandled LS notification", "method", method, "worker_id", workerID)
            return
        }
        defer func() {
            // D-05: recover, log Error, never crash Listen.
            if r := recover(); r != nil {
                logger.Error("LS notification handler panicked",
                    "method", method, "worker_id", workerID, "panic", fmt.Sprint(r))
            }
        }()
        h(params)
    }
}
```

### Example 2: JdtlsAdapter readiness state plumbing (mirror of `RustAnalyzerAdapter`)

```go
// Source: derivative of quirks.go:185-251; legacy schema from eclipse_jdtls.py:861-867
func (j *JdtlsAdapter) ensureServiceReadyCh() chan struct{} {
    j.readyMu.Lock()
    defer j.readyMu.Unlock()
    if j.serviceReady == nil {
        j.serviceReady = make(chan struct{})
    }
    return j.serviceReady
}

func (j *JdtlsAdapter) signalServiceReady() {
    j.readyMu.Lock()
    defer j.readyMu.Unlock()
    if j.serviceReady == nil {
        ch := make(chan struct{})
        close(ch)
        j.serviceReady = ch
        return
    }
    select {
    case <-j.serviceReady:
        // already closed
    default:
        close(j.serviceReady)
    }
}
// Mirror identically for projectReady. (Optional refactor: factor into a helper
// taking a *chan struct{} pointer — but two call sites doesn't justify it.)
```

### Example 3: Real-LS regression test for rust-analyzer (D-14)

```go
// Source: synthesized from rust_test.go:14-25 pattern + CONTEXT D-14
// File: test/integration/rust_test.go (under //go:build integration)
func TestRustAnalyzer_NotificationDispatchEndToEnd(t *testing.T) {
    requireLS(t, "rust-analyzer")
    fixture := PrepareFixture(t, "rust")
    td := StartTestDaemon(t, Options{
        WorkspaceDir: fixture,
        LSTimeout:    45 * time.Second,
        LSQuery:      "helper",
    })
    // Activate already done in StartTestDaemon. Reach into the worker pool to
    // grab the rust worker's adapter and assert quiescent flipped via the
    // ACTUAL dispatch path (not direct handler invocation).
    worker := td.Daemon.LSPool().GetWorker("rust", fixture)  // exact accessor TBD by planner
    require.NotNil(t, worker)
    ra, ok := worker.Quirks().(*lspool.RustAnalyzerAdapter)
    require.True(t, ok)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    require.True(t, ra.WaitUntilRenameReady(ctx),
        "rust-analyzer must reach quiescence via wired dispatch — without dispatcher this returns false")
}
```

> Note: the exact pool accessor (`GetWorker`/`LeaseWorker`/etc.) and `td.Daemon` field name should be verified during planning by the executor — this example shows the shape, not the literal call. Planner: read `internal/kernel/lspool/pool.go` and `test/integration/harness.go` `Daemon` field exposure.

### Example 4: Java integration test gate (D-10 + D-15)

```go
// Source: extends test/integration/java_test.go:18-30 (shape of existing TestSymbols_JavaFixture)
// Add right after StartTestDaemon, before the t.Run subtests.
func waitJavaReady(t *testing.T, td *TestDaemon, fixture string) {
    t.Helper()
    worker := td.Daemon.LSPool().GetWorker("java", fixture)
    require.NotNil(t, worker)
    j, ok := worker.Quirks().(*lspool.JdtlsAdapter)
    require.True(t, ok, "expected jdtls quirks adapter")
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    require.NoError(t, j.WaitUntilJavaReady(ctx),
        "WaitUntilJavaReady must succeed before issuing symbol queries; D-10 forbids silent proceed")
}
```

---

## Runtime State Inventory

> Phase 56 is a code-only refactor + new code in existing packages. No data migrations, no rename, no OS-registered state, no env vars renamed.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — verified by grep for `OnNotification` and `JdtlsAdapter` across all storage layers (SQLite memory, repomap tagcache). | none |
| Live service config | None — Phase 56 doesn't touch agent profiles, LS registry YAML, or daemon config. | none |
| OS-registered state | None — no PATH binaries, no launchd, no scheduled tasks. | none |
| Secrets/env vars | `SERENA_TEST_JDTLS_DATA_DIR` is referenced read-only by `JdtlsAdapter.ExtraArgs` (quirks.go:339); Phase 56 does NOT modify this. The Phase 48 jdtlscache helper sets it from tests. | none — read-only consumer; behavior unchanged. |
| Build artifacts | None — no generated code, no compiled binaries to refresh. `go build ./cmd/serena` rebuilds cleanly from source. | none |

---

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| `go` | All build/test | (presumed ✓) | 1.25+ per Phase 50 target | — |
| `jdtls` | Java integration tests + regression assertion | dev-machine specific | — | `requireLS(t, "jdtls")` skips if missing (helpers.go:13-18) |
| `rust-analyzer` | Rust regression test (D-14, integration tag) | dev-machine specific | — | `requireLS(t, "rust-analyzer")` skips if missing |
| Phase 48 jdtlscache helper | Java tests use `jdtlscache.ResolveDataDir` for warm-cache | ✓ already shipped (`test/integration/jdtlscache/cache.go`) | — | None needed |

**Missing dependencies with no fallback:** none — the test framework's skip-on-missing pattern covers all cases.

**Missing dependencies with fallback:** if `jdtls` is missing on the planner/executor's machine, the Java integration tests skip with a clear message. CI must have jdtls installed for D-15 to be enforceable in default `go test ./...` (verify CI workflow installs it as part of Phase 56 acceptance).

> ⚠️ **CI verification gap:** Confirm during planning that `.github/workflows/go-test.yml` (or equivalent) installs jdtls before `go test ./...` runs. Otherwise D-15 ("Java integration tests pass in default `go test ./...`") is satisfied locally but silently skipped in CI.

---

## Validation Architecture

> `nyquist_validation: true` per `.planning/config.json` — section is REQUIRED.

### Test Framework
| Property | Value |
|----------|-------|
| Framework | `go test` (Go 1.25 stdlib) |
| Config file | none (`go test` discovers via package layout) |
| Quick run command | `go test ./internal/kernel/jsonrpc/... ./internal/kernel/lspool/... -run 'Notification|Jdtls|Dispatcher' -count=1` |
| Full suite command | `go test ./... -count=1` |
| Integration regression command | `go test -tags=integration ./test/integration/... -run 'TestRustAnalyzer_NotificationDispatchEndToEnd|TestSymbols_JavaFixture|TestEdit_JavaFixture' -count=1` |
| Estimated runtime | ~3s quick, ~60s full (excludes integration tag), ~3-5min with integration tag (jdtls cold start dominant) |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| LSDISP-01 | `OnNotification` is set when handlers are registered | unit | `go test ./internal/kernel/lspool/... -run 'TestWorker_DispatcherWired' -count=1` | ❌ Wave 0 |
| LSDISP-02 | `ProcessHandle.Start` does NOT auto-launch Listen | unit | `go test ./internal/kernel/lspool/... -run 'TestProcessHandle_StartListenSeparate' -count=1` | ❌ Wave 0 |
| LSDISP-03 | Dispatcher recovers from handler panic and continues | unit | `go test ./internal/kernel/jsonrpc/... -run 'TestConn_NotificationHandlerPanic' -count=1` (D-12) | ❌ Wave 0 |
| LSDISP-04 | Worker.Start returns error if dispatcher unwired with non-empty handlers | unit | `go test ./internal/kernel/lspool/... -run 'TestWorker_DispatcherWiringRegression' -count=1` (D-11) | ❌ Wave 0 |
| LSDISP-04b | Unhandled method debug-logs and drops (no panic) | unit | `go test ./internal/kernel/jsonrpc/... -run 'TestConn_NotificationUnknownMethod' -count=1` (D-12) | ❌ Wave 0 |
| JDTLS-RDY-01a | `JdtlsAdapter.NotificationHandlers()` registers `language/status` | unit | `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter_LanguageStatusReadiness' -count=1` (D-13) | ❌ Wave 0 |
| JDTLS-RDY-01b | Malformed `language/status` payload no-ops | unit | `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter_LanguageStatusMalformed' -count=1` (D-13) | ❌ Wave 0 |
| JDTLS-RDY-01c | `WaitUntilJavaReady` blocks until BOTH events; honors ctx cancel | unit | `go test ./internal/kernel/lspool/... -run 'TestJdtlsAdapter_WaitUntilJavaReady' -count=1` | ❌ Wave 0 |
| JDTLS-RDY-02 | Java fixture: hover, cross-file refs, replace_symbol_body, find_implementations all green | integration | `go test ./test/integration/... -run 'TestSymbols_JavaFixture\|TestEdit_JavaFixture' -count=1` (D-15; runs in default `go test ./...` — no build tag on java_test.go) | ✅ exists, needs Wait gate added |
| LSDISP-REG-01 | Real rust-analyzer dispatch: `quiescent` flips after worker start | integration (build-tag) | `go test -tags=integration ./test/integration/... -run 'TestRustAnalyzer_NotificationDispatchEndToEnd' -count=1` (D-14) | ❌ Wave 0 (under `//go:build integration`) |

### Sampling Rate
- **Per task commit:** quick run command (≤3s) — covers all unit tests for the bug + readiness API.
- **Per wave merge:** full suite command (≤60s without integration tag) + `go vet ./...`.
- **Phase gate (`/gsd-verify-work`):** full suite + integration tag full suite, both green.
- **Max feedback latency:** 60s during execution; ~5min at phase gate.

### Wave 0 Gaps

- [ ] `internal/kernel/jsonrpc/codec_test.go` — extend with `TestConn_NotificationUnknownMethod`, `TestConn_NotificationHandlerPanic` (D-12).
- [ ] `internal/kernel/lspool/quirks_test.go` — extend with `TestJdtlsAdapter_LanguageStatusReadiness`, `TestJdtlsAdapter_LanguageStatusMalformed`, `TestJdtlsAdapter_WaitUntilJavaReady` (D-13), `TestJdtlsAdapter_ImplementsQuirkAdapter` (mirror existing rust-analyzer structural test at `:61-63`).
- [ ] `internal/kernel/lspool/worker_test.go` (file may not exist — verify) — add `TestWorker_DispatcherWired`, `TestWorker_DispatcherWiringRegression` (D-11). If file doesn't exist, scaffold it.
- [ ] `internal/kernel/lspool/process_test.go` (file may not exist — verify) — add `TestProcessHandle_StartListenSeparate` (D-02 contract: Listen does NOT run after Start until StartListen called).
- [ ] `test/integration/rust_test.go` — extend with `TestRustAnalyzer_NotificationDispatchEndToEnd` (D-14). Reuse existing `Options` + `requireLS` infrastructure.
- [ ] `test/integration/java_test.go` — modify both `TestSymbols_JavaFixture` and `TestEdit_JavaFixture` to call `waitJavaReady(t, td, fixture)` immediately after `StartTestDaemon` (D-10).
- [ ] Verify (do NOT add — only verify) that `td.Daemon.LSPool().GetWorker(...)` or equivalent accessor exists. If not, scaffold a test-only helper in the harness.

### Regression Assertion Budget
- `TestWorker_DispatcherWiringRegression` is the safety net for D-11. It MUST run as part of every `go test ./internal/kernel/lspool/...` invocation. ≤50ms.

---

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| Handler closure invoked directly in unit tests; production dispatch unwired | Wired in `Worker.Start` between `process.Start` and `process.StartListen` | Phase 56 (this work) | Fixes silent-drop bug latent since Phase 47. |
| `ProcessHandle.Start` self-launches Listen | Caller drives Listen via `StartListen(ctx)` | Phase 56 | Eliminates race window; matches "ownership at the seam where wiring happens" idiom. |
| jdtls readiness via polling `search_symbols` (existing `WaitForLS` helper, harness.go:224-237) | Deterministic gate via `WaitUntilJavaReady` + `language/status` notification | Phase 56 | Replaces probe-and-hope with first-class signal; matches Phase 47 pattern for rust-analyzer. |

**Deprecated/outdated:**
- The implicit "rust-analyzer rename works because tests pass" assumption — false in production. Phase 56 closes the gap.

---

## Assumptions Log

> Claims tagged `[ASSUMED]` need user confirmation; verified claims need none.

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `td.Daemon.LSPool().GetWorker(lang, fixture)` is the accessor for fetching a live worker from integration tests. | Code Examples §3, §4 | Test scaffolding needs different shape — planner verifies during plan creation by reading `pool.go` + `harness.go`. Mitigation: read the actual harness file before writing tests. |
| A2 | Java integration tests will pass with Wait gate alone; intellicode enablement (legacy eclipse_jdtls.py:912-918) is NOT required because hover/refs/replace_symbol_body don't need the intellicode catalog. | Out of scope §intellicode | If wrong, hover may return signature-only (legacy already documents this — `_request_hover` retries up to 5 times to get richer format). The existing assertion `assert.Contains(t, text, "helper")` is loose enough to pass with signature-only hover. Mitigation: if test still flakes, fold in intellicode enablement as a follow-up; CONTEXT explicitly defers it. |
| A3 | jdtls reliably emits both `ServiceReady` and `ProjectStatus=OK` for the small 2-file Main.java/Greeter.java fixture within 60s. | Pitfall 3 | Legacy hotfix proves it sometimes doesn't (eclipse_jdtls.py:921-927 has a 20s timeout-and-proceed-anyway). If observed in tests, may need to relax to ServiceReady-only OR raise timeout. Hard ceiling: 90s per recommendation. Mitigation: phase verification step inspects log output of WaitUntilJavaReady on first 5 CI runs; promote to typed errors so tests can distinguish gate-timeout vs. real test failure. |
| A4 | CI installs `jdtls` before running `go test ./...`. | Environment Availability ⚠️ | If CI lacks jdtls, `requireLS(t, "jdtls")` skips silently and the Java suite is "green" but never actually exercised. **MUST be verified during phase planning** — read `.github/workflows/go-test.yml`. |

> All other claims (file line numbers, pattern code, payload shape) are `[VERIFIED: codebase grep + Read]` against the live tree at commit-time of this research.

---

## Open Questions

1. **Pool accessor for integration tests** (A1)
   - What we know: `Worker.Quirks()` exists (worker.go:391). `pool.go` and `harness.go` have not been read in detail in this research pass.
   - What's unclear: exact accessor name for grabbing a worker by (language, workspace) from inside an integration test.
   - Recommendation: planner reads `internal/kernel/lspool/pool.go` accessor surface + `test/integration/harness.go` `Daemon` field exposure during PLAN.md authoring. If no accessor exists, scaffold one in `harness.go` (test-only) — do not add to public Pool API.

2. **CI jdtls install** (A4)
   - What we know: D-15 mandates Java tests pass in default `go test ./...`.
   - What's unclear: whether CI workflow installs jdtls.
   - Recommendation: planner adds a Wave 0 task: "Read `.github/workflows/go-test.yml`; if jdtls install missing, add a step before `go test`". If GH Actions doesn't have a clean jdtls install path, document the gap and consider Phase 48 as the canonical place to fix it (since Phase 48 also depends on jdtls being present).

3. **Should `WaitUntilJavaReady` return a typed sentinel for "ServiceReady seen but ProjectStatus timed out"?**
   - What we know: legacy hotfix proves this happens.
   - What's unclear: whether tests should distinguish.
   - Recommendation: ship as plain error first (D-08 says both required, no degradation); add typed sentinel ONLY if observed flakiness in Phase 56 verification. Track as a Phase 48 follow-up.

---

## Sources

### Primary (HIGH confidence)
- `internal/kernel/jsonrpc/conn.go:33-34, 159-230` — Conn.OnNotification field + Listen dispatch loop (verified via Read).
- `internal/kernel/lspool/process.go:58-97` — ProcessHandle.Start with auto-Listen (verified via Read).
- `internal/kernel/lspool/worker.go:158-268` — Worker.Start full bootstrap path (verified via Read).
- `internal/kernel/lspool/quirks.go:106-263` — RustAnalyzerAdapter readiness machinery (verified via Read; canonical mirror target).
- `internal/kernel/lspool/quirks.go:303-345` — JdtlsAdapter current shell + ExtraArgs (verified via Read).
- `internal/kernel/lspool/quirks_test.go:16-74` — Phase 47 unit test patterns to mirror (verified via Read).
- `legacy/src/solidlsp/language_servers/eclipse_jdtls.py:861-927` — canonical jdtls `language/status` payload + ServiceReady/ProjectStatus=OK gate semantics + 20s hotfix anomaly (verified via Bash sed).
- `test/integration/java_test.go:1-167` — Java fixture test surface, no build tag, already uses jdtlscache.ResolveDataDir (verified via Read).
- `test/integration/rust_test.go:1-25` — Rust integration test pattern (verified via Bash); under `//go:build integration`.
- `test/integration/helpers.go:12-18` — `requireLS` skip pattern (verified via Read).
- `test/integration/harness.go:180-237` — Options struct + WaitForLS polling helper (verified via Read).
- `.planning/phases/47-bug-rust-analyzer-rename/47-VALIDATION.md` — prior-art validation strategy (verified via Read).

### Secondary (MEDIUM confidence)
- `.planning/phases/47-bug-rust-analyzer-rename/47-RESEARCH.md` referenced but NOT re-read in this pass (assumed consistent with VALIDATION.md frontmatter and quirks.go RustAnalyzerAdapter implementation).

### Tertiary (LOW confidence)
- None. Phase 56 is a self-contained refactor with all reference points in-tree.

---

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies in stdlib + already-vendored test libs.
- Architecture: HIGH — verified line-by-line against live code.
- Pitfalls: HIGH — Pitfalls 1, 2, 4 verified by direct grep; Pitfall 3 cited from legacy hotfix comment.
- Validation: HIGH — mirrors Phase 47 validation shape, file paths verified.
- Open questions A1/A4: MEDIUM — answerable with one Read call during planning.

**Research date:** 2026-04-25
**Valid until:** 2026-05-09 (14 days — codebase is the source of truth, low staleness risk).

---

## RESEARCH COMPLETE

**Phase:** 56 - bug-ls-notification-dispatch-and-jdtls-readiness
**Confidence:** HIGH

### Key Findings

- The bug is exactly as CONTEXT describes: `process.go:88` does `go p.conn.Listen(ctx)` while `Conn.OnNotification` is `nil`, and `Worker.Start` never sets it. The dispatch path at `conn.go:198` silently drops on the false branch of `if c.OnNotification != nil`. RustAnalyzerAdapter's readiness has therefore never functioned in production despite all unit tests passing.
- `ProcessHandle` has only TWO callers (`process.go` itself + `worker.go`) — restructuring is a small, contained diff. No test directly constructs `ProcessHandle`.
- Java integration tests (`test/integration/java_test.go`) have **NO build tag** — they run in default `go test ./...` today and use `jdtlscache.ResolveDataDir` from Phase 48's already-shipped helper. The Java fixture (`Main.java` + `Greeter.java`) already supports cross-file refs. No fixture work required.
- Rust integration tests (`test/integration/rust_test.go`) ARE behind `//go:build integration` — the D-14 regression test should live there to keep default `go test ./...` fast.
- JdtlsAdapter shell is trivial today (lines 303-345); `language/status` payload schema `{type, message}` matches `legacy/eclipse_jdtls.py:861-867` exactly. ServiceReady AND ProjectStatus=OK gate is preserved.
- Legacy has a 20s hotfix-timeout for ProjectStatus=OK — recommend default 90s ceiling in `WaitUntilJavaReady` plus ctx-deadline override.

### File Created

`/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.planning/phases/56-bug-ls-notification-dispatch-and-jdtls-readiness/56-RESEARCH.md`

### Confidence Assessment

| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Pure stdlib + already-vendored testify; no `go get`. |
| Architecture | HIGH | All wiring points verified against live `worker.go` / `process.go` / `conn.go` line numbers. |
| Pitfalls | HIGH | Pitfalls 1, 2, 4 verified by direct codebase grep; Pitfall 3 cited from legacy comment. |
| Validation | HIGH | Mirrors Phase 47 validation shape; test file locations verified to exist or correctly identified as Wave 0 gaps. |
| Pool/harness accessor (A1) | MEDIUM | Worker.Quirks() exists; pool accessor not yet read — flagged as open question for planner to resolve. |
| CI jdtls install (A4) | MEDIUM | Workflow file not read — flagged for planner verification. |

### Open Questions

- A1: exact pool accessor for integration tests (planner reads `pool.go` + `harness.go`).
- A4: CI installs jdtls? (planner reads `.github/workflows/go-test.yml`).
- A2/A3: intellicode enablement + jdtls ProjectStatus reliability — handle reactively per CONTEXT defer guidance.

### Ready for Planning

Research complete. The plan should produce 3-4 plans:
1. Restructure `ProcessHandle.Start` lifecycle + scaffold `process_test.go`.
2. Wire dispatcher + regression assertion in `Worker.Start` + scaffold `worker_test.go`.
3. Implement `JdtlsAdapter.NotificationHandlers` + `WaitUntilJavaReady` + extend `quirks_test.go`.
4. Add Java integration gate + rust-analyzer integration regression test + verify CI jdtls install.
