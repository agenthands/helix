# Phase 56: bug-ls-notification-dispatch-and-jdtls-readiness — Pattern Map

**Mapped:** 2026-04-25
**Files analyzed:** 10
**Analogs found:** 10 / 10 (all have strong in-tree analogs; A1 pool-accessor gap noted as a "no analog" sub-item)

## File Classification

| New/Modified File | Role | Data Flow | Closest Analog | Match Quality |
|-------------------|------|-----------|----------------|---------------|
| `internal/kernel/lspool/process.go` | LS-process lifecycle (modify) | event-driven (pipes + reaper goroutines) | self (current `ProcessHandle.Start` at lines 56-97) — split, do not invent new shape | exact (refactor in place) |
| `internal/kernel/lspool/worker.go` | worker bootstrap (modify) | request-response + event-driven setup | self (`Worker.Start` lines 159-268) plus `quirks.go` `ExperimentalCapabilities` type-assertion idiom (lines 204-212) | exact (extend existing flow) |
| `internal/kernel/lspool/quirks.go` (`JdtlsAdapter`) | LS quirk adapter (modify/extend) | event-driven (notification handler) + sync primitive (readiness gate) | `RustAnalyzerAdapter` lines 119-251 — verbatim mirror | exact |
| `internal/kernel/jsonrpc/conn.go` | JSON-RPC dispatch (verify only) | event-driven dispatch loop | self (`Listen` lines 159-230, `OnNotification` field line 33-34) — no behavior change | exact (verify-only) |
| `internal/kernel/lspool/process_test.go` (NEW) | unit test (lifecycle) | request-response | use `quirks_test.go` style (testify/require + assert; table-light) | role-match — no existing process_test.go |
| `internal/kernel/lspool/worker_test.go` (NEW) | unit test (dispatcher wiring + regression assertion) | request-response | `quirks_test.go` lines 16-74 (Phase 47 unit test shape) | role-match — no existing worker_test.go |
| `internal/kernel/lspool/quirks_test.go` (extend) | unit test (jdtls handler + Wait API) | request-response | self lines 16-74 (`TestRustAnalyzerAdapter_ServerStatusReadiness`, `_ServerStatusMalformed`, `_ImplementsQuirkAdapter`) | exact |
| `internal/kernel/jsonrpc/codec_test.go` (extend) | unit test (panic recovery + unknown method) | event-driven | self lines 228-250 (`TestConn_Notification_Handler`) | exact |
| `test/integration/java_test.go` (modify) | integration test (gate) | request-response | self lines 18-32 (existing flow) — insert `waitJavaReady` after `StartTestDaemon` | exact (in-place gate insertion) |
| `test/integration/rust_test.go` (extend, NEW test func) | integration test (real-LS regression, build-tag) | event-driven | self lines 1-25 (`//go:build integration` + `requireLS` + `StartTestDaemon` shape) | exact |
| `test/integration/harness.go` (POSSIBLY extend) | test-only helper | accessor | A1 GAP — no `Daemon.LSPool()` public method (verified: `internal/daemon/daemon.go` has no `LSPool()`; `kernel.Pool()` is internal). Need a test-only accessor. | NO ANALOG — scaffold |

---

## Pattern Assignments

### `internal/kernel/lspool/process.go` (lifecycle modify)

**Analog:** itself — `ProcessHandle.Start` at `internal/kernel/lspool/process.go:56-97`.

**Current shape to preserve** (lines 56-97):
```go
func (p *ProcessHandle) Start(ctx context.Context, sessionPrefix string) error {
    var err error
    // pipes
    p.stdin, err = p.cmd.StdinPipe()
    // ...
    if err := p.cmd.Start(); err != nil { ... }
    rwc := &stdinStdoutRWC{stdin: p.stdin, stdout: p.stdout}
    p.conn = jsonrpc.NewConn(rwc, sessionPrefix)
    go p.drainStderr()
    go p.conn.Listen(ctx)   // <-- REMOVE this line (D-02)
    go p.reap()
    go p.watchContext(ctx)
    return nil
}
```

**New method to add (after `Conn()` accessor at line 100-102):**
```go
// StartListen begins the JSON-RPC dispatch loop. Caller MUST set
// Conn().OnNotification (if needed) before invoking StartListen — otherwise
// notifications received before assignment are silently dropped (this is the
// exact bug Phase 56 fixes; see jsonrpc.Conn.Listen comment).
func (p *ProcessHandle) StartListen(ctx context.Context) {
    go p.conn.Listen(ctx)
}
```

**Why this exact shape:** Encapsulation already established by `ProcessHandle.Conn()` accessor (line 100). `StartListen` is the smallest diff (research §Pattern 2, §"Don't Hand-Roll" — return-conn alternative explicitly rejected).

---

### `internal/kernel/lspool/worker.go` (bootstrap modify)

**Analog:** itself — `Worker.Start` at `internal/kernel/lspool/worker.go:159-268`.

**Insertion point:** immediately after `process.Start` succeeds (line 179-182), and BEFORE `Conn().Call(ctx, "initialize", ...)` (line 220).

**Imports already present** (lines 1-18) — `encoding/json`, `log/slog`, `sync/atomic` available. Will need to add `encoding/json` if not already imported, and reference `jsonrpc.NotificationFunc` type.

**Shape of inserted block (after line 182, before line 184 "Initializing" state set):**
```go
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

**`buildDispatcher` helper to add at file scope** (mirrors research Code Example 1; place near top of file or in its own helper file):
```go
func buildDispatcher(
    handlers map[string]func(json.RawMessage),
    logger *slog.Logger,
    workerID string,
) jsonrpc.NotificationFunc {
    routes := make(map[string]func(json.RawMessage), len(handlers))
    for k, v := range handlers {
        routes[k] = v
    }
    return func(method string, params json.RawMessage) {
        h, ok := routes[method]
        if !ok {
            logger.Debug("unhandled LS notification", "method", method, "worker_id", workerID)
            return
        }
        defer func() {
            if r := recover(); r != nil {
                logger.Error("LS notification handler panicked",
                    "method", method, "worker_id", workerID, "panic", fmt.Sprint(r))
            }
        }()
        h(params)
    }
}
```

**Optional-interface idiom to mirror** (existing in `worker.go:204-212`):
```go
if ec, ok := w.quirks.(ExperimentalCapabilities); ok {
    exp := map[string]any{}
    for k, v := range ec.ExperimentalCapabilities() {
        exp[k] = v
    }
    initParams.Capabilities.Experimental = exp
}
```
This is the established type-assertion + nil-guard pattern — apply the same conservatism to the quirks-handler check (`if w.quirks != nil`).

---

### `internal/kernel/lspool/quirks.go` — `JdtlsAdapter` extension

**Analog:** `RustAnalyzerAdapter` at `internal/kernel/lspool/quirks.go:119-251`. Mirror **verbatim**, twice (one channel per signal).

**Struct extension** (replace current shell at lines 305-307):
```go
type JdtlsAdapter struct {
    Entry langregistry.LSEntry

    readyMu      sync.Mutex
    serviceReady chan struct{} // closed when ServiceReady seen
    projectReady chan struct{} // closed when ProjectStatus=OK seen
}
```

**`NotificationHandlers` (replace nil-returning stub at lines 317-319):**
```go
// NotificationHandlers handles jdtls language/status notifications. Payload
// schema (per legacy/src/solidlsp/language_servers/eclipse_jdtls.py:861-867):
// {type: string, message: string}. Handlers MUST be non-blocking — atomic ops,
// channel close, or mutex-guarded flag flip only. No I/O, no LSP calls back to
// the same conn (Phase 56 D-06).
func (j *JdtlsAdapter) NotificationHandlers() map[string]func(params json.RawMessage) {
    return map[string]func(params json.RawMessage){
        "language/status": func(raw json.RawMessage) {
            var s struct {
                Type    string `json:"type"`
                Message string `json:"message"`
            }
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
```

**Lazy-channel + signal helpers** — mirror lines 184-216 of `RustAnalyzerAdapter`. One pair per channel:
```go
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
    default:
        close(j.serviceReady)
    }
}

// Mirror identically for projectReady (ensureProjectReadyCh / signalProjectReady).
```

**Public Wait API** (mirror `WaitUntilRenameReady` at lines 226-251 — note it returns `bool` while ours returns `error` per D-09):
```go
// javaReadinessTimeout bounds WaitUntilJavaReady when the caller's context has
// no deadline. Mirrors renameReadinessTimeout (quirks.go:37). 90s is generous
// vs. legacy's 20s hotfix (eclipse_jdtls.py:921-927) which sometimes proceeds
// without ProjectStatus=OK; we keep the strict gate but raise the ceiling.
const javaReadinessTimeout = 90 * time.Second

func (j *JdtlsAdapter) WaitUntilJavaReady(ctx context.Context) error {
    svc := j.ensureServiceReadyCh()
    proj := j.ensureProjectReadyCh()
    timer := time.NewTimer(javaReadinessTimeout)
    defer timer.Stop()
    select {
    case <-svc:
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        return fmt.Errorf("jdtls ServiceReady timeout after %s", javaReadinessTimeout)
    }
    select {
    case <-proj:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    case <-timer.C:
        return fmt.Errorf("jdtls ProjectStatus=OK timeout after %s", javaReadinessTimeout)
    }
}
```

**Imports to verify present in `quirks.go`** (lines 3-15): all needed (`context`, `encoding/json`, `sync`, `time`, `fmt` may need to be added for the `fmt.Errorf` calls — `fmt` is NOT currently imported; add it).

---

### `internal/kernel/jsonrpc/conn.go` (verify only — no edits)

**Analog:** itself.

**Verify the dispatch loop is correct** (lines 196-205 — already correct, no change):
```go
if peek.Method != "" && peek.ID == nil {
    // Notification (has method, no ID).
    if c.OnNotification != nil {
        var notif Notification
        if err := json.Unmarshal(msg, &notif); err == nil {
            c.OnNotification(notif.Method, notif.Params)
        }
    }
    continue
}
```

**Note:** D-12 panic recovery is intentionally placed at the **dispatcher closure** (Worker layer), NOT inside `Conn.Listen`. This keeps `jsonrpc.Conn` agnostic of higher-layer handler semantics. Do not add panic recovery here.

---

### `internal/kernel/lspool/process_test.go` (NEW)

**Analog:** `internal/kernel/lspool/quirks_test.go:1-14` (package + imports header) and `internal/kernel/jsonrpc/codec_test.go:228-250` (mock RWC + Listen idiom).

**Test imports header (copy):**
```go
package lspool

import (
    "context"
    "testing"
    "time"

    "github.com/stretchr/testify/assert"
    "github.com/stretchr/testify/require"
)
```

**Core pattern — test that Listen does NOT run after `Start` until `StartListen` is called:** spawn a benign no-op process (e.g., `sleep 1` via `exec.Command`), assert that `process.Conn().OnNotification` can be set after `Start` returns and BEFORE `StartListen` is invoked, and that no notification is dispatched until `StartListen` fires. Use a buffered fake stdout via custom `stdinStdoutRWC` only if needed — simpler to lean on a real short-lived process.

**Skip-on-platform pattern (if needed for CI):** mirror `requireLS(t, "...")` but for spawning `sleep` — should be available on darwin/linux without skip.

---

### `internal/kernel/lspool/worker_test.go` (NEW)

**Analog:** `quirks_test.go:16-74` for shape, plus the optional-interface assertion at lines 61-63.

**Test pattern for `TestWorker_DispatcherWired`:** construct a `Worker` with a stub `QuirkAdapter` whose `NotificationHandlers()` returns one entry. Drive `Worker.Start` against a fake LS (or use a mocked `ProcessHandle` — verify whether such a seam exists; if not, the test scaffolds a lightweight LS-like subprocess that produces a valid `initialize` response then a `language/status`-style notification).

**Test pattern for `TestWorker_DispatcherWiringRegression` (D-11):** inject a `QuirkAdapter` that returns a non-empty `NotificationHandlers()` map. Replace the wiring step with a fault-injection (or rely on a `t.Helper` that nils out `OnNotification` post-assignment) to force the regression assertion to fire. Assert `Worker.Start` returns a non-nil error containing "dispatcher wiring failed".

**Quick fallback if a full subprocess harness is heavy:** factor `buildDispatcher` to be testable in isolation (already pure — see worker.go pattern above). Test it directly:
```go
func TestBuildDispatcher_KnownMethod(t *testing.T) { ... }
func TestBuildDispatcher_UnknownMethodLogsAndDrops(t *testing.T) { ... }
func TestBuildDispatcher_PanicRecovers(t *testing.T) {
    handlers := map[string]func(json.RawMessage){
        "boom": func(json.RawMessage) { panic("kaboom") },
    }
    d := buildDispatcher(handlers, slog.New(slog.NewTextHandler(io.Discard, nil)), "w-1")
    assert.NotPanics(t, func() { d("boom", json.RawMessage(`{}`)) })
}
```

---

### `internal/kernel/lspool/quirks_test.go` (extend)

**Analog:** lines 16-74 (Phase 47 rust-analyzer test triplet — readiness, malformed, structural).

**Tests to add (verbatim shape mirror):**

`TestJdtlsAdapter_LanguageStatusReadiness` (mirror of lines 19-43):
```go
func TestJdtlsAdapter_LanguageStatusReadiness(t *testing.T) {
    j := &JdtlsAdapter{}
    handlers := j.NotificationHandlers()
    handler, ok := handlers["language/status"]
    require.True(t, ok, "jdtls adapter must register language/status handler")

    // Not ready before any notification.
    ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel()
    assert.Error(t, j.WaitUntilJavaReady(ctx))

    // ServiceReady alone insufficient.
    handler(json.RawMessage(`{"type":"ServiceReady","message":"ServiceReady"}`))
    ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
    defer cancel2()
    assert.Error(t, j.WaitUntilJavaReady(ctx2))

    // After ProjectStatus=OK, both signals closed → success.
    handler(json.RawMessage(`{"type":"ProjectStatus","message":"OK"}`))
    assert.NoError(t, j.WaitUntilJavaReady(context.Background()))
}
```

`TestJdtlsAdapter_LanguageStatusMalformed` (mirror of lines 47-57): malformed JSON → no panic, gate stays closed.

`TestJdtlsAdapter_ImplementsQuirkAdapter` (mirror of lines 61-63): `var _ QuirkAdapter = (*JdtlsAdapter)(nil)` — already structurally satisfied; this is a compile-time guard.

---

### `internal/kernel/jsonrpc/codec_test.go` (extend)

**Analog:** `TestConn_Notification_Handler` at lines 228-250 — the exact pattern (mock RWC + writeNotification + Listen + assert callback fired).

**Tests to add:**

`TestConn_NotificationUnknownMethod`: pre-load a notification, set `OnNotification` to a callback that logs/records the call; assert it is invoked with the unhandled method name. (NOTE: per D-04, "unhandled" means no registered handler in the **Worker dispatcher** — at the `jsonrpc.Conn` layer, ALL notifications are routed through `OnNotification`. Verify no early-return logic exists for unknown methods in `Conn.Listen` — there isn't, per lines 196-205.)

`TestConn_NotificationHandlerPanic`: this test belongs in `worker_test.go` for the dispatcher (since panic recovery lives at the Worker layer per the architectural note above). If kept here, it would test that `Conn.Listen` does NOT recover (current behavior — handler panic crashes Listen goroutine). **Recommend: route this test to `worker_test.go`** as `TestBuildDispatcher_PanicRecovers`.

---

### `test/integration/java_test.go` (modify)

**Analog:** itself, lines 18-32 (current `TestSymbols_JavaFixture` setup block).

**Insertion point:** immediately after `StartTestDaemon` (line 32) and BEFORE the first `t.Run` (line 34). Same insertion in `TestEdit_JavaFixture`.

**Pattern (research §Code Example 4):**
```go
func waitJavaReady(t *testing.T, td *TestDaemon, fixture string) {
    t.Helper()
    worker := td.Daemon.LSPool().GetWorker("java", fixture) // A1: needs harness accessor
    require.NotNil(t, worker)
    j, ok := worker.Quirks().(*lspool.JdtlsAdapter)
    require.True(t, ok, "expected jdtls quirks adapter")
    ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
    defer cancel()
    require.NoError(t, j.WaitUntilJavaReady(ctx),
        "WaitUntilJavaReady must succeed before issuing symbol queries; D-10 forbids silent proceed")
}
```

**Existing setup pattern to preserve** (lines 18-32):
- `requireLS(t, "jdtls")` skip guard
- `jdtlscache.ResolveDataDir(...)` for warm cache (Phase 48 helper, already shipped)
- `Options{WorkspaceDir, JdtlsDataDir, LSTimeout: 120s}` shape

Insert `waitJavaReady(t, td, fixture)` between `td := StartTestDaemon(...)` and the first subtest. Apply to BOTH `TestSymbols_JavaFixture` and `TestEdit_JavaFixture` (locate both via grep before editing).

---

### `test/integration/rust_test.go` (extend with new func, build-tagged)

**Analog:** itself — header at line 1 (`//go:build integration`) + `TestSymbols_RustFixture` shape at lines 14-25.

**New test (research §Code Example 3):**
```go
// TestRustAnalyzer_NotificationDispatchEndToEnd is the regression proof for
// Phase 56: assert that experimental/serverStatus actually flips quiescent via
// the wired dispatch path (not direct handler invocation as in unit tests).
// Without Phase 56's wiring this test fails — that's the entire point.
func TestRustAnalyzer_NotificationDispatchEndToEnd(t *testing.T) {
    requireLS(t, "rust-analyzer")
    fixture := PrepareFixture(t, "rust")
    td := StartTestDaemon(t, Options{
        WorkspaceDir: fixture,
        LSTimeout:    45 * time.Second,
        LSQuery:      "helper",
    })
    worker := td.Daemon.LSPool().GetWorker("rust", fixture) // A1: harness accessor
    require.NotNil(t, worker)
    ra, ok := worker.Quirks().(*lspool.RustAnalyzerAdapter)
    require.True(t, ok)
    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()
    require.True(t, ra.WaitUntilRenameReady(ctx),
        "rust-analyzer must reach quiescence via wired dispatch — without dispatcher this returns false")
}
```

**File header to preserve** (line 1): `//go:build integration` — keeps default `go test ./...` fast.

---

### `test/integration/harness.go` — A1 GAP (test-only accessor)

**No analog — scaffold a test-only helper.** Verified: `internal/daemon/daemon.go` exposes no `LSPool()`; `kernel.Pool()` is package-internal accessed via `k.Pool().AcquireLease(...)` (daemon.go:239, 304, 308, 671). The integration harness's `TestDaemon` (harness.go:56-58) wraps `*daemon.Daemon` privately.

**Required addition (test-only, in `harness.go` or a new `harness_lspool.go`):**
```go
// LSPoolWorker returns the warm worker for (language, workspaceDir) from the
// test daemon's LS pool. Test-only — do NOT add this to the public daemon API
// surface. Used by Phase 56 integration tests to reach into adapter readiness
// state (TestRustAnalyzer_NotificationDispatchEndToEnd, waitJavaReady).
func (td *TestDaemon) LSPoolWorker(language, workspaceDir string) *lspool.Worker {
    // Implementation: reach through td.daemon → kernel → pool. Exact accessor
    // path TBD by executor — likely needs a small test-only export on either
    // daemon or kernel (e.g., daemon.LSPoolForTests()).
}
```

**Worker accessors already present** (from `worker.go`):
- `Worker.Quirks() QuirkAdapter` (line 391)
- `Worker.Language() string` (line 340)
- `Worker.WorkDir() string` (line 345)

These are the consumer surface for the gate functions; no extension needed on the Worker side.

---

## Shared Patterns

### Optional-interface type-assertion (across all quirk-related code)

**Source:** `internal/kernel/lspool/worker.go:204-212` (`ExperimentalCapabilities` check) and `internal/kernel/lspool/worker.go:172-174` (`ArgsModifier` check).

**Apply to:** `Worker.Start` dispatcher wiring (NEW), all integration tests reaching adapter (`waitJavaReady`, regression test).

```go
if w.quirks != nil {
    if x, ok := w.quirks.(SomeOptionalInterface); ok {
        // use x
    }
}
```

### Lazy-channel + idempotent close + mutex (readiness signaling)

**Source:** `internal/kernel/lspool/quirks.go:184-216` (`ensureReadyCh`, `signalReady`).

**Apply to:** `JdtlsAdapter.ensureServiceReadyCh`/`signalServiceReady`/`ensureProjectReadyCh`/`signalProjectReady`. Two parallel pairs — do NOT try to share state across them (research Anti-Patterns §3).

### Wait-with-timeout (public Wait API)

**Source:** `internal/kernel/lspool/quirks.go:226-251` (`WaitUntilRenameReady`).

**Apply to:** `JdtlsAdapter.WaitUntilJavaReady`. Differences:
- Returns `error` (not `bool`) per D-09 → richer test assertions and ctx-cancel distinction.
- Two sequential `select` blocks (one per channel) instead of one — both gates must close.
- Constant `javaReadinessTimeout = 90 * time.Second` mirrors `renameReadinessTimeout = 10 * time.Second` (line 37).

### Adapter handler unit-test triplet (Phase 47 idiom)

**Source:** `internal/kernel/lspool/quirks_test.go:16-74` — 4 tests per readiness-bearing adapter:
1. `*_*Readiness` — exercises the happy path through Wait API
2. `*_*Malformed` — malformed payload → no panic, no state change
3. `*_ImplementsQuirkAdapter` — compile-time interface guard (`var _ QuirkAdapter = (*X)(nil)`)
4. (Rust-only) `*_ExperimentalCapabilities` — opt-in check; jdtls does NOT need this

**Apply to:** `quirks_test.go` extension for `JdtlsAdapter` (3 of 4 tests; skip the experimental-caps one).

### Mock RWC for Conn-level tests

**Source:** `internal/kernel/jsonrpc/codec_test.go:228-250` (`TestConn_Notification_Handler`) plus `mockRWC` helpers (`writeNotification`, `writeResponse`).

**Apply to:** any Conn-layer test extension. NOT applicable to `quirks_test.go` (which works on adapter directly without going through Conn).

### Integration test setup (build-tagged + skip-on-missing-LS)

**Source:** `test/integration/rust_test.go:1-19` (header + `requireLS` + `StartTestDaemon`). For non-tagged: `test/integration/java_test.go:1-32` (no build tag — Java tests run in default `go test ./...`).

**Apply to:**
- `TestRustAnalyzer_NotificationDispatchEndToEnd` → file `rust_test.go` under existing `//go:build integration` tag (D-14).
- `waitJavaReady` insertion in `java_test.go` → no tag change (Java already runs by default per D-15).

---

## No Analog Found

| File / Component | Reason | Mitigation |
|------------------|--------|------------|
| `TestDaemon.LSPoolWorker(language, workspaceDir)` accessor | A1 — no public `Daemon.LSPool()`; `kernel.Pool()` is internal. Verified via grep: only `k.Pool().AcquireLease(...)` callsites (daemon.go:239, 304, 308, 671). | Scaffold test-only helper in `test/integration/harness.go`. Likely requires a thin export from `daemon` or `kernel` (e.g., `daemon.LSPoolForTests()`). Planner reads `internal/daemon/daemon.go` and `internal/kernel/kernel.go` to pick the cleanest seam. |
| CI `jdtls` install verification (A4) | Not a code file — `.github/workflows/*.yml` not yet inspected for jdtls install step. | Planner reads `.github/workflows/go-test.yml` (or equivalent) and adds a Wave 0 task to install jdtls if missing, OR documents the gap and defers to Phase 48. |

---

## Metadata

**Analog search scope:**
- `internal/kernel/lspool/` (process.go, worker.go, quirks.go, quirks_test.go) — full read of quirks.go (622 lines), targeted reads of process.go (full), worker.go lines 130-289.
- `internal/kernel/jsonrpc/` (conn.go full, codec_test.go lines 180-260).
- `test/integration/` (java_test.go header, rust_test.go full, harness.go lines 170-250).
- Pool accessor surface: `internal/kernel/lspool/pool.go` (grep).
- Daemon LS-pool exposure: `internal/daemon/daemon.go` (grep).

**Files scanned:** 8 source + 2 test + 1 daemon for accessor verification = 11.

**Pattern extraction date:** 2026-04-25

---

## PATTERN MAPPING COMPLETE

**Phase:** 56 - bug-ls-notification-dispatch-and-jdtls-readiness
**Files classified:** 11 (10 with strong analogs + 1 A1 gap requiring scaffolding)
**Analogs found:** 10 / 11 (A1 harness accessor needs scaffolding — flagged in §"No Analog Found")

### Coverage
- Files with exact analog: 8 (process.go, worker.go, quirks.go JdtlsAdapter mirror, conn.go verify-only, quirks_test.go extension, codec_test.go extension, java_test.go gate insertion, rust_test.go new test)
- Files with role-match analog: 2 (process_test.go NEW, worker_test.go NEW — quirks_test.go shape is the model)
- Files with no analog: 1 (harness `LSPoolWorker` accessor — scaffold required)

### Key Patterns Identified
- All readiness-bearing quirk adapters use the **lazy-channel + idempotent-close + sync.Mutex** idiom (`RustAnalyzerAdapter` lines 184-216) — `JdtlsAdapter` mirrors twice (once per signal).
- Optional-interface type-assertion is the established way Worker layers per-LS behavior on top of the base `QuirkAdapter` interface (`worker.go:172-174` for `ArgsModifier`, `:204-212` for `ExperimentalCapabilities`).
- The **dispatcher-wiring fix** is a Worker-layer concern; `jsonrpc.Conn.OnNotification` field already exists and the Listen loop already invokes it correctly (`conn.go:196-205`). Phase 56 is purely about *setting* the field at the right moment, plus splitting `ProcessHandle.Start` to make that moment exist.
- Adapter unit-test pattern is a 3-test (or 4-test for experimental-caps adapters) triplet established in Phase 47 (`quirks_test.go:16-74`); jdtls extension follows the same shape verbatim.
- Java integration tests have **no build tag** (run by default); Rust integration tests are tagged `//go:build integration` — keep this split for the regression test (D-14 → rust_test.go) vs. the gate (D-15 → java_test.go).

### File Created
`/Users/Janis_Vizulis/go/src/github.com/postfix/serena/.planning/phases/56-bug-ls-notification-dispatch-and-jdtls-readiness/56-PATTERNS.md`

### Ready for Planning
Pattern mapping complete. Planner can now reference analog patterns in PLAN.md files. The single open structural question (A1 — pool/worker accessor for integration tests) is documented with a recommended scaffolding seam; planner should resolve by reading `internal/daemon/daemon.go` and `internal/kernel/kernel.go` to pick between `daemon.LSPoolForTests()` and `kernel.LSPoolForTests()`.
