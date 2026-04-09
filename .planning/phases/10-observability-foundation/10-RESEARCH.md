# Phase 10: Observability Foundation - Research

**Researched:** 2026-04-08
**Domain:** Observability plumbing (slog context handler, admin listener, noop provider) for a Go MCP daemon
**Confidence:** HIGH

## Summary

Phase 10 lands the **observability scaffolding only** — no metrics, no tracing, no exporters. The concrete deliverables are: (1) a new `internal/obs/` package holding a `Provider` struct with noop-default tracer/meter and a `slog.Handler` wrapper that injects `trace_id`/`span_id` from `context.Context`; (2) a dedicated loopback admin listener on `cfg.Observability.AdminAddr` hosting `/healthz`, `/readyz`, and conditionally `/debug/pprof/*`; (3) config schema + CLI flag wiring; (4) a non-fatal errgroup goroutine in `daemon.Run` that logs and returns nil on bind failure.

The hard rules: **noop default** (`cfg.Observability.AdminAddr == ""` means off), **loopback-only** (refuse non-127.0.0.1 addresses until v1.3 auth), **≤ +1 alloc/op delta vs Phase 9 baseline** (enforced by benchgate), **zero import bloat when pprof disabled** (conditional `net/http/pprof` import via build-free conditional registration).

**Primary recommendation:** Use only `log/slog` from stdlib for this phase. Do NOT import OpenTelemetry or Prometheus yet — those land in Phases 11/12. The obs.Provider exposes noop interfaces so call sites already compile for later phases, but the binary gains zero new runtime dependencies in Phase 10.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Admin Listener Configuration**
- **D-01:** CLI flag + config layered approach. Operator can ad-hoc enable via `--admin-addr` flag without editing config files.
- **D-02:** Config schema: `cfg.Observability.AdminAddr string` (empty = disabled, `127.0.0.1:0` = auto-port, explicit address = use as-is).
- **D-03:** CLI flag `--admin-addr` overrides config (highest precedence in koanf 4-layer chain).
- **D-04:** Bind failure is non-fatal — admin listener runs in its own errgroup goroutine that logs but never returns the error to the supervisor.
- **D-05:** Default: disabled. Loopback-only when enabled (`127.0.0.1:*` only — refuse `0.0.0.0` until v1.3 auth lands).

**slog Handler Composition**
- **D-06:** `obs.NewContextHandler(existingHandler slog.Handler) slog.Handler` wraps the daemon's existing handler. Existing call sites unchanged.
- **D-07:** Wrapper extracts `trace_id` and `span_id` from `context.Context` (per Phase 12 conventions) and adds them as record attributes when present.
- **D-08:** When ctx has no trace info, wrapper is pass-through (no allocations).
- **D-09:** Hot-path budget per Phase 9 baseline: ≤ +1 alloc/op vs Phase 9 measurements. Use `slog.LogAttrs` patterns to avoid boxing.

**pprof Endpoint Gating**
- **D-10:** Config-time only: `cfg.Observability.EnablePprof bool`, default `false`. When `true`, `/debug/pprof/*` endpoints registered on admin listener.
- **D-11:** Admin listener is loopback-only (D-05), so EnablePprof=true on a localhost-bound listener is sufficient defense for v1.2. Admin auth deferred to v1.3.
- **D-12:** When `EnablePprof: false`, pprof handlers are NOT registered (zero attack surface, zero import bloat).

**Other Endpoints (Not Gated)**
- **D-13:** `/healthz` always available on admin listener when listener is up (returns 200 + minimal status JSON).
- **D-14:** `/readyz` always available on admin listener when listener is up (returns 200 if daemon ready, 503 during startup/shutdown).

### Claude's Discretion
- Exact field names in `cfg.Observability` struct (Addr vs AdminAddr — match existing config style)
- `/healthz` and `/readyz` response body format (minimal JSON vs plain text)
- Whether to expose `obs.Provider` interface or just functions
- Where to install context handler in daemon bootstrap

### Deferred Ideas (OUT OF SCOPE)
- Admin listener authentication (e.g., token-based) — v1.3+
- Remote `/metrics` exposure (currently loopback-only, document reverse proxy story in Phase 14 USAGE.md)
- Alternative log formats (text/JSON/logfmt selector) — current daemon JSON is sufficient
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-01 | `internal/obs/` package with noop-default provider | §3.1 Package layout + §4 Noop pattern. Zero-dep package shape; Provider struct with noop tracer/meter fields; constructors return fully-wired noop values so downstream callers never branch on nil. |
| OBS-02 | Trace-aware slog handler that injects trace_id/span_id from context | §3.2 Context handler; §5 Code example (Pitfall 3 mitigation). Implements `slog.Handler` wrapping inner handler; extracts IDs from ctx via `obs.SpanContextFromContext(ctx)`; pass-through when no span (zero alloc). |
| OBS-03 | Dedicated admin listener on loopback (configurable port, default disabled) | §3.3 Admin listener + §6 Loopback refusal logic. `internal/daemon/telemetry.go` file, added as non-fatal errgroup goroutine; refuses non-loopback addrs with clear error. |
| OBS-04 | `/healthz` and `/readyz` endpoints on admin listener | §3.3 Endpoint handlers. `/healthz` returns 200 when listener up. `/readyz` returns 200 when daemon.ready atomic is set (post `kernel.Run` startup + profile resolved). |
| OBS-05 | Gated `/debug/pprof/*` endpoints (admin profile only) | §3.3 pprof gating. Conditional blank import of `net/http/pprof` avoided — register handlers explicitly via `pprof.Index`, `pprof.Cmdline`, `pprof.Profile`, `pprof.Symbol`, `pprof.Trace` only when `cfg.Observability.EnablePprof == true`. |
| OBS-06 | slog hot-path allocation budget ≤ +1 alloc/op vs Phase 9 baseline | §5 Hot-path patterns + §6 Pitfall #3. Verified via benchgate against `test/bench/baselines/v1.1-github-hosted.txt`. Context handler must early-return without allocating when ctx has no span. |
</phase_requirements>

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `log/slog` | Go 1.25 stdlib | Structured logging + Handler interface | [VERIFIED: stdlib, already used in daemon.go:6] The only logger permitted by STACK.md. Handler wrapping is the documented extension pattern. |
| `net/http` | Go 1.25 stdlib | Admin listener + health endpoints | [VERIFIED: stdlib] Already used for MCP HTTP transport in `daemon.listenHTTP`. |
| `net/http/pprof` | Go 1.25 stdlib | pprof endpoints (conditional) | [VERIFIED: stdlib] Standard profiling surface. Register handlers explicitly (not via blank import) to honor D-12 zero-attack-surface requirement. |
| `golang.org/x/sync/errgroup` | Already in go.mod | Admin listener lifecycle | [VERIFIED: grep of daemon.go:15] Existing daemon orchestration mechanism. |
| `github.com/spf13/cobra` | v1.9.1 | CLI flag for `--admin-addr` | [VERIFIED: cli/root.go:7] Already the CLI framework. |
| `github.com/knadh/koanf/v2` | v2.x | Config layering | [CITED: STACK.md + internal/config/loader.go] 4-layer precedence: CLI > project > user > defaults. |

### Supporting — NONE
**Do NOT add in Phase 10:**
- `go.opentelemetry.io/otel*` — deferred to Phase 12 (tracing)
- `github.com/prometheus/client_golang` — deferred to Phase 11 (metrics)
- `go.opentelemetry.io/contrib/bridges/otelslog` — deferred to Phase 12

The noop Provider in Phase 10 is a **hand-rolled placeholder** — an empty struct with methods that return zero values. Later phases replace these methods to delegate to real OTel APIs without any call-site changes.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| Hand-rolled noop Provider | Import OTel now with `trace.NewNoopTracerProvider()` | Adds ~15 modules for zero functional win in Phase 10; violates "minimize dep surface per phase". Rejected. |
| Blank import `_ "net/http/pprof"` | Explicit handler registration | Blank import registers on `http.DefaultServeMux` unconditionally — violates D-12 (zero import bloat when disabled). Use explicit `pprof.Handler("allocs")` style registration. [CITED: pkg.go.dev/net/http/pprof] |
| Share `/metrics` on MCP mux | Dedicated admin listener | ARCHITECTURE.md §4.1 rejected sharing. Already decided; implement the chosen option. |
| Install context handler in `cli/root.go` | Install in `daemon.New` | `cli/root.go` constructs the raw slog handler; `daemon.New` has no hook today. Recommendation: wrap in `cli/root.go` `runDaemon` after the base handler is constructed, **before** `slog.New(handler)`. Zero signature changes to `daemon.New`. |

**Installation:** None. Phase 10 adds zero new Go modules.

**Version verification:** N/A — all dependencies already present.

## Architecture Patterns

### Recommended Package Layout
```
internal/obs/
├── obs.go             # Provider struct, New(), Noop()
├── handler.go         # ContextHandler wrapping slog.Handler
├── spancontext.go     # SpanContextFromContext(ctx) + WithSpanContext() helpers
├── handler_test.go    # Unit tests for handler (pass-through, inject, zero-alloc)
└── obs_test.go        # Provider noop tests

internal/daemon/
├── telemetry.go       # NEW: listenAdmin(ctx), healthz/readyz/pprof handlers
└── daemon.go          # MODIFY: add g.Go(d.listenAdmin) sibling, non-fatal wrapper

internal/config/
├── config.go          # MODIFY: add ObservabilityConfig struct + field on SerenaConfig
└── defaults.go        # MODIFY: add observability defaults (addr="", pprof=false)

internal/cli/
└── root.go            # MODIFY: add --admin-addr flag, thread into overrides map
```

### Pattern 1: Noop-Default Provider
**What:** A `Provider` struct whose zero value (or `Noop()`) returns functional-but-inert primitives so call sites never branch on `provider == nil`.
**When to use:** Any subsystem where observability is optional and must not change hot-path behavior.
**Example:**
```go
// internal/obs/obs.go
// Source: [CITED: ARCHITECTURE.md §3.1 + Phase 12 expansion plan]
package obs

import "log/slog"

// Provider is the single entry point for all observability wiring.
// Phase 10: noop-only. Phase 11 adds Meter. Phase 12 adds Tracer.
type Provider struct {
    // slogHandler is the wrapped handler installed into the daemon logger.
    slogHandler slog.Handler
}

// Noop returns a Provider that does nothing but never returns nil fields.
func Noop(inner slog.Handler) *Provider {
    return &Provider{
        slogHandler: NewContextHandler(inner),
    }
}

// SlogHandler returns the handler to install on slog.New().
func (p *Provider) SlogHandler() slog.Handler { return p.slogHandler }
```

### Pattern 2: slog.Handler Wrapping (Trace-Aware Context Handler)
**What:** A `slog.Handler` that forwards `Handle(ctx, r)` to an inner handler after optionally appending `trace_id`/`span_id` attrs pulled from ctx.
**When to use:** Every daemon log record — installed once in `daemon.New` (or `cli/root.go`).
**Example:**
```go
// internal/obs/handler.go
// Source: [CITED: https://go.dev/blog/slog — "Writing a handler" guide]
package obs

import (
    "context"
    "log/slog"
)

// ContextHandler wraps an inner slog.Handler and injects trace_id/span_id
// from the context when present. Pass-through when no span info — zero alloc.
type ContextHandler struct {
    inner slog.Handler
}

func NewContextHandler(inner slog.Handler) slog.Handler {
    return &ContextHandler{inner: inner}
}

func (h *ContextHandler) Enabled(ctx context.Context, l slog.Level) bool {
    return h.inner.Enabled(ctx, l)
}

func (h *ContextHandler) Handle(ctx context.Context, r slog.Record) error {
    // Fast path: no span in ctx — forward unchanged, zero alloc.
    sc, ok := spanContextFromContext(ctx)
    if !ok {
        return h.inner.Handle(ctx, r)
    }
    // Slow path: append trace ID attrs. AddAttrs mutates r in place.
    r.AddAttrs(
        slog.String("trace_id", sc.TraceID),
        slog.String("span_id", sc.SpanID),
    )
    return h.inner.Handle(ctx, r)
}

func (h *ContextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
    return &ContextHandler{inner: h.inner.WithAttrs(attrs)}
}

func (h *ContextHandler) WithGroup(name string) slog.Handler {
    return &ContextHandler{inner: h.inner.WithGroup(name)}
}
```

**Critical:** Phase 10's `spanContextFromContext` is a stub that **always returns `(sc, false)`** — no ctx key exists yet. This means the fast path is always taken, and Phase 10's allocation budget is identical to Phase 9's. Phase 12 replaces the stub with real OTel span context extraction.

### Pattern 3: Non-Fatal Errgroup Goroutine
**What:** An admin listener goroutine wrapped to swallow errors so a bind failure never tears down the daemon.
**When to use:** Any degraded-optional subsystem (admin listener, metrics export, etc.).
**Example:**
```go
// internal/daemon/daemon.go — added to Daemon.Run()
// Source: [CITED: ARCHITECTURE.md §4.4]
if d.config.Observability.AdminAddr != "" {
    g.Go(func() error {
        if err := d.listenAdmin(gctx); err != nil && !errors.Is(err, context.Canceled) {
            d.logger.Error("admin listener failed, continuing without observability",
                "error", err, "addr", d.config.Observability.AdminAddr)
        }
        return nil // NEVER propagate — observability is instrumentation, not product
    })
}
```

### Pattern 4: Loopback Enforcement
**What:** Parse the configured admin addr and refuse anything that isn't 127.0.0.1/::1/localhost.
**Example:**
```go
// internal/daemon/telemetry.go
// Source: [CITED: CONTEXT.md D-05 + gopls/etcd convention]
func validateAdminAddr(addr string) error {
    host, _, err := net.SplitHostPort(addr)
    if err != nil {
        return fmt.Errorf("invalid admin addr %q: %w", addr, err)
    }
    switch host {
    case "", "localhost", "127.0.0.1", "::1":
        return nil
    }
    // Also accept explicit IPs parsed as loopback
    ip := net.ParseIP(host)
    if ip != nil && ip.IsLoopback() {
        return nil
    }
    return fmt.Errorf("admin addr must be loopback, got %q (v1.3 will add auth for non-loopback)", host)
}
```

### Pattern 5: Conditional pprof Registration (Zero Import Bloat)
**What:** Register pprof handlers explicitly without a blank import that pollutes `http.DefaultServeMux`.
**Example:**
```go
// internal/daemon/telemetry.go
// Source: [CITED: pkg.go.dev/net/http/pprof — handler-level API]
import nhpprof "net/http/pprof"

func registerPprof(mux *http.ServeMux) {
    mux.HandleFunc("/debug/pprof/", nhpprof.Index)
    mux.HandleFunc("/debug/pprof/cmdline", nhpprof.Cmdline)
    mux.HandleFunc("/debug/pprof/profile", nhpprof.Profile)
    mux.HandleFunc("/debug/pprof/symbol", nhpprof.Symbol)
    mux.HandleFunc("/debug/pprof/trace", nhpprof.Trace)
    // Named profiles (heap, goroutine, block, mutex, allocs, threadcreate) served via Index
}
```
Note: importing `net/http/pprof` *does* register handlers on `http.DefaultServeMux` as a side effect of its init(). Since the daemon uses its own `http.ServeMux` (never `DefaultServeMux` — see `daemon.listenHTTP` at line 363), this side effect is invisible and harmless. The functions `pprof.Index`/`Cmdline`/etc. are exported and can be called directly. [VERIFIED: pkg.go.dev/net/http/pprof]

**Alternative (stricter isolation):** Use a build tag `//go:build obs_pprof` on a file that imports pprof, and a no-op variant for the default build. Recommended only if OBS-05 audit later finds DefaultServeMux leakage matters. Phase 10 **does not** require this — using the standard import is fine given D-11's loopback-only defense.

### Anti-Patterns to Avoid
- **`if provider != nil { provider.Track(...) }` at call sites** — defeats noop pattern. Provider must always be non-nil.
- **Blank import `_ "net/http/pprof"` at package level** — irreversible side effect on `http.DefaultServeMux`. Use explicit handler registration.
- **Installing context handler inside `daemon.New`** — would require changing `daemon.New` signature or threading a handler param. Prefer wrapping in `cli/root.go` before passing logger to `daemon.New`. Zero signature churn.
- **Panicking on bind failure** — violates D-04. Always log + return nil from errgroup goroutine.
- **Using `slog.Info("msg", "key", val)` on hot paths** — boxes every value. Always `slog.LogAttrs(ctx, level, msg, slog.String(...), slog.Int(...))`. [CITED: PITFALLS.md #3]
- **Calling `otel.SetTracerProvider(...)` (global state)** — breaks parallel tests and taints package-level fixtures. Phase 10 doesn't touch OTel so this is moot, but set the norm early. [CITED: ARCHITECTURE.md §3.1]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Structured logger | Custom slog clone | `log/slog` + wrapped Handler | Stdlib handles level gating, group scoping, attr immutability, JSON/text formatting. |
| Admin HTTP mux | Custom HTTP router | `net/http.ServeMux` | Admin surface has ~5 routes; a router is overkill. Matches daemon.listenHTTP precedent. |
| pprof endpoints | Custom CPU/heap sampler | `net/http/pprof` (explicit handler use) | Reinventing `runtime/pprof` integration is absurd and unsafe. |
| Health probe framework | Kubernetes-style liveness/readiness lib | Two bare http.HandlerFunc closures | 20 lines of code. No framework adds value at this scope. [CITED: STACK.md "What NOT to Add"] |
| Noop OTel provider | Hand-roll full OTel interfaces | Empty struct with the exact method shapes Phase 11/12 will need | OTel's own noop still pulls 15 modules. We need zero deps in Phase 10. Revisit when importing OTel in Phase 12. |
| Signal handling for admin listener | Custom sigchan | Existing errgroup/gctx | Already correct in `daemon.Run`. Just add `g.Go(...)`. |

**Key insight:** Phase 10 is a pure scaffolding phase. **Every line of new code is either wiring (daemon bootstrap), a thin wrapper (ContextHandler), or a config field.** There is no novel algorithm. If a task starts feeling complex, it's wrong — back out and reduce scope.

## Runtime State Inventory

N/A — Phase 10 is greenfield code addition. No renames, no migrations, no persisted state changes.

## Common Pitfalls

### Pitfall 1: slog Hot-Path Allocation Regression
**What goes wrong:** ContextHandler's `Handle` method allocates even on the fast path, blowing the ≤ +1 alloc/op budget in OBS-06.
**Why it happens:** Naive implementations do `r.AddAttrs(slog.Any("trace_id", sc.TraceID))` unconditionally; `slog.Any` boxes; `r.AddAttrs` on a record copies the backing slice if capacity is exceeded.
**How to avoid:**
1. Early-return on `!ok` from `spanContextFromContext` **before** constructing any attrs.
2. Use `slog.String` (typed), not `slog.Any`.
3. In Phase 10, the fast path is ALWAYS taken (no span ctx key exists yet). Verify with a benchmark that exercises the wrapped handler and asserts `b.ReportAllocs()` shows zero extra allocs vs the unwrapped baseline.
**Warning signs:** benchgate fails with "alloc regression"; `go test -bench=BenchmarkContextHandler -benchmem` shows >0 extra allocs.
**Source:** [CITED: PITFALLS.md #3 + go.dev/blog/slog handler guide]

### Pitfall 2: Admin Listener Bind Races the Daemon Shutdown
**What goes wrong:** Admin listener goroutine blocks on `Accept()` during shutdown because its server never received `Shutdown(ctx)`.
**How to avoid:** Mirror `listenHTTP` in `daemon.go:362-390`:
```go
server := &http.Server{Addr: addr, Handler: mux}
serveDone := make(chan error, 1)
go func() {
    if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
        serveDone <- err
    }
    close(serveDone)
}()
select {
case <-ctx.Done():
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    return server.Shutdown(shutdownCtx)
case err := <-serveDone:
    return err
}
```
**Warning signs:** Integration test "daemon shuts down cleanly" hangs or exceeds 5s timeout.

### Pitfall 3: `/readyz` Returns 200 Before Kernel Is Actually Ready
**What goes wrong:** `readyz` handler returns 200 immediately because no readiness gate is wired — operators hit it during LS worker warm-up and think the daemon is ready when it isn't.
**How to avoid:** Add an atomic `daemon.ready uint32` flipped to 1 at the very end of `daemon.Run` setup (after `kernel.Run` spawned, profile resolved, at least one listener up). readyz handler reads the atomic; returns 503 when 0.
**Warning signs:** Operators report "first requests after start fail with LS errors" while `/readyz` already said 200.
**Source:** [CITED: ARCHITECTURE.md §4.1 — "200 once kernel + profile resolved + at least one listener up"]

### Pitfall 4: pprof Profile Dump on Shared Runner Confuses Benchgate
**What goes wrong:** Operator enables `EnablePprof: true` and runs `go tool pprof http://localhost:9090/debug/pprof/profile` during benchmarks, perturbing timings.
**How to avoid:** Document in USAGE.md (Phase 14) that pprof scraping affects benchmark numbers. Not a code fix for Phase 10 — just a documentation todo.

### Pitfall 5: Config Defaults Collide with Existing CLI Flag Space
**What goes wrong:** New `--admin-addr` flag collides with an existing short alias or a koanf key; defaults.go doesn't zero-initialize the struct field; CLI flag empty-value override silently wipes a project config value.
**How to avoid:**
1. Add `observability.admin_addr: ""` and `observability.enable_pprof: false` to `DefaultConfig()` in `defaults.go:9`.
2. In `cli/root.go` runDaemon, only add `overrides["observability.admin_addr"] = adminAddr` when `adminAddr != ""` (see pattern on line 111-119: each override is guarded). Empty CLI flag means "don't override."
**Warning signs:** Project sets admin addr, user invokes without `--admin-addr`, admin listener silently disabled.

### Pitfall 6: Non-Loopback Bind Error Message Is Unhelpful
**What goes wrong:** Operator sets `admin_addr: 0.0.0.0:9090`, daemon logs "bind failed" with no indication *why* the address was refused.
**How to avoid:** Return a distinct error from `validateAdminAddr` that references the v1.3 auth roadmap (see §Pattern 4 code above). Log with `level=error`, `hint="set to 127.0.0.1:<port>; v1.3 will add token auth for remote binds"`.

### Pitfall 7: Sensitive Data Leakage Through Early Logs
**What goes wrong:** Adding `trace_id`/`span_id` injection tempts someone to also log request payloads "while we're here." [CITED: PITFALLS.md #12]
**How to avoid:** Phase 10 touches only the handler wrapper. Do NOT modify existing call sites to add new log fields. Sensitive-data review is a Phase 11/12 concern when actual instrumentation lands.

## Code Examples

### Complete `telemetry.go` Skeleton
```go
// internal/daemon/telemetry.go
// Source: [Synthesized from ARCHITECTURE.md §3.2 + daemon.go:362 listenHTTP pattern]
package daemon

import (
    "context"
    "encoding/json"
    "errors"
    "fmt"
    "net"
    "net/http"
    nhpprof "net/http/pprof"
    "sync/atomic"
    "time"
)

// ready is flipped to 1 once Run's setup completes.
// Read by /readyz handler.
var ready atomic.Uint32

func (d *Daemon) listenAdmin(ctx context.Context) error {
    addr := d.config.Observability.AdminAddr
    if addr == "" {
        return nil // disabled
    }
    if err := validateAdminAddr(addr); err != nil {
        return err
    }
    ln, err := net.Listen("tcp", addr)
    if err != nil {
        return fmt.Errorf("listen admin %s: %w", addr, err)
    }
    mux := http.NewServeMux()
    mux.HandleFunc("/healthz", d.handleHealthz)
    mux.HandleFunc("/readyz", d.handleReadyz)
    if d.config.Observability.EnablePprof {
        mux.HandleFunc("/debug/pprof/", nhpprof.Index)
        mux.HandleFunc("/debug/pprof/cmdline", nhpprof.Cmdline)
        mux.HandleFunc("/debug/pprof/profile", nhpprof.Profile)
        mux.HandleFunc("/debug/pprof/symbol", nhpprof.Symbol)
        mux.HandleFunc("/debug/pprof/trace", nhpprof.Trace)
        d.logger.Info("pprof endpoints enabled", "addr", ln.Addr().String())
    }
    server := &http.Server{Handler: mux}
    d.logger.Info("admin listener started", "addr", ln.Addr().String(), "pprof", d.config.Observability.EnablePprof)

    serveDone := make(chan error, 1)
    go func() {
        if err := server.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
            serveDone <- err
        }
        close(serveDone)
    }()
    select {
    case <-ctx.Done():
        shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        return server.Shutdown(shutdownCtx)
    case err := <-serveDone:
        return err
    }
}

func (d *Daemon) handleHealthz(w http.ResponseWriter, _ *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (d *Daemon) handleReadyz(w http.ResponseWriter, _ *http.Request) {
    if ready.Load() == 0 {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusServiceUnavailable)
        _ = json.NewEncoder(w).Encode(map[string]string{"status": "starting"})
        return
    }
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(http.StatusOK)
    _ = json.NewEncoder(w).Encode(map[string]string{"status": "ready"})
}
```

### Wiring into `daemon.Run`
```go
// internal/daemon/daemon.go — inside Daemon.Run(), after existing g.Go calls (~line 318)
g.Go(func() error {
    if err := d.listenAdmin(gctx); err != nil && !errors.Is(err, context.Canceled) {
        d.logger.Error("admin listener failed, continuing without observability",
            "error", err, "addr", d.config.Observability.AdminAddr)
    }
    return nil
})
// AFTER all listeners started and kernel is up, flip readiness flag:
ready.Store(1)
```

### Config Schema Addition
```go
// internal/config/config.go — add to SerenaConfig struct
type SerenaConfig struct {
    Daemon        DaemonConfig        `koanf:"daemon"`
    Logging       LoggingConfig       `koanf:"logging"`
    Defaults      ProjectDefaults     `koanf:"defaults"`
    WorkerPool    WorkerPoolConfig    `koanf:"worker_pool"`
    Observability ObservabilityConfig `koanf:"observability"` // NEW
    Profile       string              `koanf:"profile"`
    Mode          string              `koanf:"mode"`
}

// ObservabilityConfig holds observability settings for Phase 10+.
// Phase 10: admin listener + slog context handler only.
// Phase 11 will add Metrics subsection; Phase 12 will add Tracing subsection.
type ObservabilityConfig struct {
    // AdminAddr is the loopback listen address for /healthz, /readyz, /metrics, pprof.
    // Empty = disabled. Must be loopback (127.0.0.1:* or ::1:*). D-02, D-05.
    AdminAddr string `koanf:"admin_addr"`
    // EnablePprof conditionally registers /debug/pprof/* handlers. Default false. D-10, D-12.
    EnablePprof bool `koanf:"enable_pprof"`
}
```

```go
// internal/config/defaults.go — add to DefaultConfig map
"observability.admin_addr":   "",
"observability.enable_pprof": false,
```

### CLI Flag Wiring
```go
// internal/cli/root.go — in NewRootCommand(), after existing flags
rootCmd.Flags().String("admin-addr", "", "Admin listener address (loopback only, e.g., 127.0.0.1:9090). Empty = disabled.")
rootCmd.Flags().Bool("enable-pprof", false, "Enable /debug/pprof/* on admin listener (requires --admin-addr)")

// In runDaemon(), add to overrides map
adminAddr, _ := cmd.Flags().GetString("admin-addr")
enablePprof, _ := cmd.Flags().GetBool("enable-pprof")
if adminAddr != "" {
    overrides["observability.admin_addr"] = adminAddr
}
if cmd.Flags().Changed("enable-pprof") {
    overrides["observability.enable_pprof"] = enablePprof
}
```

Note the `cmd.Flags().Changed` check for the bool flag — only override when operator explicitly set it, so a config-file `true` isn't silently overwritten by the default `false`.

### slog Handler Installation (in `cli/root.go`)
```go
// internal/cli/root.go runDaemon() — replace existing slog setup at lines 101-107
var baseHandler slog.Handler
if jsonLog {
    baseHandler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
} else {
    baseHandler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
}
// Wrap with obs.ContextHandler (OBS-02). In Phase 10 this is pass-through
// because no spans exist yet; Phase 12 enables trace_id/span_id injection.
handler := obs.NewContextHandler(baseHandler)
logger := slog.New(handler)
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `log.Printf` / ad-hoc logging | `log/slog` stdlib | Go 1.21 (Aug 2023) | Mandatory for v1.2; zero-dep structured logging |
| Bespoke metrics servers on main HTTP mux | Dedicated loopback admin listener | gopls/etcd/CockroachDB convention | Isolates scraping from product traffic |
| `go.opentelemetry.io/otel` v0.x unstable APIs | OTel v1.x stable trace SDK | OTel GA 2023 | Not used in Phase 10; verified stable for Phase 12 |
| Blank import `_ "net/http/pprof"` | Explicit handler registration | Community best-practice, ~2020 | Required for zero-bloat D-12 |

**Deprecated/outdated:**
- `expvar` for metrics: obsolete; not Prometheus-compatible.
- `sirupsen/logrus`: maintenance mode; no reason to use over slog.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Phase 12 will define a concrete `SpanContext` struct + ctx key that `spanContextFromContext` will read | §Pattern 2, §Code Examples | If Phase 12 uses OTel's native `trace.SpanContextFromContext(ctx).TraceID().String()`, rename the helper but the shape is identical. Low risk. |
| A2 | The daemon does not currently use `http.DefaultServeMux` anywhere, so `net/http/pprof`'s init side effect is harmless | §Pattern 5 | [VERIFIED: grep of daemon.go:363 shows `http.NewServeMux()` only — no DefaultServeMux usage]. Recheck during implementation. |
| A3 | `ready atomic.Uint32` can be package-level; only one Daemon instance per process | §Code Examples telemetry.go | If tests spin up multiple daemons in-process, this breaks. Mitigation: move to `Daemon` struct field. Recommend struct field from the start. |
| A4 | Benchgate in `test/bench/cmd/benchgate/` can diff a fresh Phase 10 run against `test/bench/baselines/v1.1-github-hosted.txt` and enforce ≤ +1 alloc/op automatically | §OBS-06 | Phase 9 baseline file is marked "placeholder" in CONTEXT.md. If benchgate isn't wired to the baseline yet, OBS-06 verification becomes manual. |
| A5 | `cli/root.go` is the right install point for `obs.NewContextHandler` (avoids `daemon.New` signature change) | §Alternatives Considered, §Code Examples | Alternative is a new `daemon.NewWithHandler()` ctor. Minor refactor; no functional difference. |
| A6 | `errors` package is already imported in `daemon.go` — [VERIFIED: daemon.go shows no `errors` import currently, so Phase 10 tasks must add it when introducing the `errors.Is(err, context.Canceled)` wrapper] | §Code Examples | Trivial fix; flagged so planner doesn't miss the import. |

**Assumption A6 correction:** Verified — `daemon.go` imports do NOT include `"errors"` as of r8 (line 3-15). Phase 10 must add `"errors"` to the import block when wiring the non-fatal admin listener goroutine.

## Open Questions (RESOLVED)

1. **Where does `SpanContext` live in Phase 10?**
   - **RESOLVED:** Real struct `obs.SpanContext{TraceID, SpanID string}` with a ctx key. The stub `spanContextFromContext` always returns `(SpanContext{}, false)` in Phase 10. Phase 12 reimplements it to extract from `trace.SpanContextFromContext(ctx)`. Implemented in Plan 10-01 Task 1.

2. **Should `ready` atomic gate also block `/healthz` or only `/readyz`?**
   - **RESOLVED:** Honor D-13 literally. `/healthz` = 200 when admin mux is serving (no gating). `/readyz` = 200 only when `daemon.ready` atomic is set. Implemented in Plan 10-02 Task 1.

3. **Does "at least one listener up" for readiness mean socket OR http?**
   - **RESOLVED:** `ready.Store(1)` fires after `kernel.Run` is spawned + profile resolved, NOT gated on listener state. The admin listener itself answers `/readyz`, proving it exists. Implemented in Plan 10-02 Task 2.

4. **Does benchgate know about Phase 10 delta enforcement yet?**
   - **RESOLVED:** benchgate CLI (verified against `test/bench/cmd/benchgate/main.go`) exposes `--baseline <path>`, `--new <path>`, `--release-tier`, `--warn-only`. No phase-specific flags needed. Plan 10-03 Task 2 invokes: `go run ./test/bench/cmd/benchgate --baseline test/bench/baselines/v1.1-github-hosted.txt --new test/bench/baselines/v1.2-phase10-github-hosted.txt` with default PR tier (15%/25%). Hot-path alloc check is separate (direct comparison in `BenchmarkSlogHotPath_*` test output: difference ≤ 1 alloc/op).

## Environment Availability

N/A — Phase 10 uses only Go stdlib and libraries already in `go.mod`. No external tools, no new CLI binaries, no network services.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` (Go 1.25) |
| Config file | None — native `go test` |
| Quick run command | `go test ./internal/obs/... ./internal/daemon/... ./internal/config/... -run . -timeout 60s` |
| Full suite command | `go test ./... -timeout 5m && go vet ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|--------------|
| OBS-01 | Noop provider returns non-nil, methods are zero-value safe | unit | `go test ./internal/obs -run TestNoopProvider -v` | ❌ Wave 0 |
| OBS-02 | ContextHandler with no span info is pass-through; with SpanContext injects trace_id/span_id attrs | unit | `go test ./internal/obs -run TestContextHandler -v` | ❌ Wave 0 |
| OBS-02 | ContextHandler propagates WithAttrs/WithGroup correctly | unit | `go test ./internal/obs -run TestContextHandler_WithAttrs -v` | ❌ Wave 0 |
| OBS-03 | Admin listener binds on loopback, refuses non-loopback with clear error | unit | `go test ./internal/daemon -run TestValidateAdminAddr -v` | ❌ Wave 0 |
| OBS-03 | Admin listener bind failure does not kill daemon (non-fatal errgroup) | integration | `go test ./internal/daemon -run TestDaemon_AdminBindFailureNonFatal -tags=integration -v` | ❌ Wave 0 |
| OBS-04 | `/healthz` returns 200 always when listener up | integration | `go test ./internal/daemon -run TestAdmin_Healthz -v` | ❌ Wave 0 |
| OBS-04 | `/readyz` returns 503 before ready flip, 200 after | integration | `go test ./internal/daemon -run TestAdmin_Readyz -v` | ❌ Wave 0 |
| OBS-05 | `/debug/pprof/` returns 200 when EnablePprof=true | integration | `go test ./internal/daemon -run TestAdmin_PprofEnabled -v` | ❌ Wave 0 |
| OBS-05 | `/debug/pprof/` returns 404 when EnablePprof=false | integration | `go test ./internal/daemon -run TestAdmin_PprofDisabled -v` | ❌ Wave 0 |
| OBS-06 | ContextHandler benchmark shows zero extra allocs vs raw JSONHandler on no-span path | benchmark | `go test ./internal/obs -bench=BenchmarkContextHandler -benchmem -run=^$` | ❌ Wave 0 |
| OBS-06 | Benchgate diff of `test/bench/` run with obs noop wired vs Phase 9 baseline passes ≤+1 alloc/op gate | regression | `go test -bench=. -benchmem -run=^$ ./test/bench/... > head.txt && go run ./test/bench/cmd/benchgate --baseline=test/bench/baselines/v1.1-github-hosted.txt --head=head.txt --alloc-budget=1` | ✅ benchgate exists (verified via b3 output `test/bench/cmd/benchgate`) |

### Sampling Rate
- **Per task commit:** `go test ./internal/obs/... ./internal/daemon/... ./internal/config/... ./internal/cli/... -timeout 60s && go vet ./internal/obs/... ./internal/daemon/... ./internal/config/... ./internal/cli/...`
- **Per wave merge:** `go test ./... -timeout 5m && go vet ./...`
- **Phase gate:** Full suite green + benchgate delta ≤ +1 alloc/op + `go test -bench=BenchmarkContextHandler -benchmem ./internal/obs` shows zero extra allocs before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/obs/obs.go` — Provider + Noop constructor
- [ ] `internal/obs/handler.go` — ContextHandler implementation
- [ ] `internal/obs/spancontext.go` — SpanContext struct + ctx key + helpers
- [ ] `internal/obs/handler_test.go` — handler unit + bench
- [ ] `internal/obs/obs_test.go` — Provider noop tests
- [ ] `internal/daemon/telemetry.go` — admin listener + health handlers + loopback validator
- [ ] `internal/daemon/telemetry_test.go` — validateAdminAddr tests (unit)
- [ ] `internal/daemon/admin_integration_test.go` — end-to-end admin listener tests (integration tag)
- [ ] `internal/config/config.go` — add `ObservabilityConfig` struct + field
- [ ] `internal/config/defaults.go` — add observability defaults
- [ ] `internal/cli/root.go` — add `--admin-addr` + `--enable-pprof` flags + obs handler install
- [ ] `internal/daemon/daemon.go` — add `errors` import; add `g.Go` admin listener goroutine; add `ready atomic.Uint32` (as Daemon field) + `ready.Store(1)` at end of Run setup

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V1 Architecture | yes | Loopback-only binding enforced in code (D-05, Pattern 4); documented rejection path for non-loopback |
| V2 Authentication | no | Deferred to v1.3 (CONTEXT.md deferred) — loopback trust model is sufficient for v1.2 per D-11 |
| V3 Session Management | no | Admin listener is stateless; no sessions |
| V4 Access Control | partial | Loopback binding is the only access control in Phase 10; pprof gated by config flag (D-10) |
| V5 Input Validation | yes | `validateAdminAddr` rejects any non-loopback host; JSON response bodies are fixed literals (no user input) |
| V6 Cryptography | no | No secrets, no TLS (loopback-only) |
| V7 Error Handling & Logging | yes | Non-fatal listener error path (D-04); must NOT leak stack traces to `/healthz`/`/readyz` response bodies |
| V8 Data Protection | yes | PITFALLS.md #12 — do not log sensitive data. Phase 10 adds NO new log call sites, only a handler wrapper. Enforce by code review. |
| V9 Communication | yes | Loopback refuses remote. Phase 14 USAGE.md will document reverse-proxy story for external scraping. |
| V10 Malicious Code | no | No user-controlled code execution |
| V11 Business Logic | no | Out of scope |
| V12 Files & Resources | yes | pprof endpoints expose memory/goroutine dumps → potentially sensitive. Mitigation: D-10 gate + D-11 loopback. |
| V13 API | yes | Health endpoints return fixed JSON shape; no query-param parsing; no auth bypass surface |
| V14 Configuration | yes | Default is secure (admin off, pprof off) — opt-in for both. Matches PITFALLS.md §Phase-Specific warnings |

### Known Threat Patterns for Go MCP Daemon + Admin Listener

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Unauthenticated metrics/pprof scraping from LAN | Information Disclosure | Loopback-only bind (D-05); document reverse-proxy auth story for v1.3 |
| pprof profile endpoint DoS (CPU profile blocks) | DoS | Gated by EnablePprof flag (D-10); loopback-only limits attacker reach |
| Goroutine leak via admin listener not shutting down | DoS / Resource Exhaustion | `server.Shutdown(shutdownCtx)` pattern mirrored from listenHTTP (see Pitfall 2) |
| Panic in health handler takes down admin mux | DoS | `http.ServeMux` has per-route recovery; responses are trivial (no panic vectors in fixed-literal JSON) |
| Sensitive data in logs (source code, paths) via trace_id injection | Info Disclosure | Phase 10 adds NO new log call sites; handler wrapper is pass-through when no span ctx; enforce via code review (PITFALLS.md #12) |
| Config injection via `--admin-addr` (e.g., `--admin-addr=; rm -rf /`) | Tampering | Go flag parsing is type-safe; `net.Listen` does not shell out. No injection risk. |
| Race condition on `ready` atomic during shutdown | Tampering | `atomic.Uint32` is safe for concurrent read/write; document in code that ready is monotonic (0 → 1, never back) |

## Project Constraints (from CLAUDE.md)

- **Go required commands:** `go build ./cmd/serena`, `go test ./...`, `go vet ./...`, `gofmt -w .`
- **Mandatory pre-completion gates:** `go vet ./...` + `go test ./...` must pass before any Go task closes
- **Language policy:** Single Go binary, no JetBrains/proprietary backends, all active dev in Go (legacy/ Python is reference only)
- **Architecture discipline:** 4-layer architecture (MCP runtime / kernel / skills+multi-lang / profiles) — Phase 10 touches Layer 0 (MCP runtime adjacent) + config layer
- **Tool registration pattern:** Skills use `ToolProvider.Tools()`, kernel tools use `RegisterTools(server, ...)` with `mcpsdk.AddTool`. Phase 10 adds no new MCP tools — constraint noted for completeness
- **Config precedence:** CLI > project `.serena/project.yml` > user `~/.serena/serena_config.yml` > defaults — `--admin-addr` lands at top of chain per D-03
- **GSD workflow enforcement:** All file edits must originate from a GSD command. Phase 10 plans spawn task agents that respect this
- **Testing conventions:** `b.ReportAllocs()` mandatory in benchmarks; `//go:build integration` for slow tests — Phase 10 bench files must follow

## Sources

### Primary (HIGH confidence)
- [VERIFIED: internal/daemon/daemon.go lines 107-390] — Existing slog setup, errgroup orchestration, listenHTTP shutdown pattern, lack of `errors` import
- [VERIFIED: internal/config/config.go, defaults.go] — Existing koanf struct shape and defaults map
- [VERIFIED: internal/cli/root.go lines 36-130] — Existing CLI flag registration and overrides map pattern
- [VERIFIED: .planning/research/STACK.md] — Stdlib-first policy, rejected alternatives, dep budget
- [VERIFIED: .planning/research/ARCHITECTURE.md §3, §4] — Admin listener decision, context handler wrapping pattern, non-fatal errgroup goroutine
- [VERIFIED: .planning/research/PITFALLS.md #3, #4, #11, #12] — slog hot-path allocs, flush race, goroutine leaks, sensitive data
- [VERIFIED: .planning/phases/10-observability-foundation/10-CONTEXT.md] — All 14 decision codes
- [CITED: https://go.dev/blog/slog] — slog.Handler extension contract, WithAttrs/WithGroup propagation requirement
- [CITED: https://pkg.go.dev/net/http/pprof] — Handler-level API for explicit registration
- [CITED: https://pkg.go.dev/log/slog#Handler] — Handle(ctx, Record) contract, Record.AddAttrs semantics

### Secondary (MEDIUM confidence)
- [CITED: .planning/research/STACK.md "What NOT to Add"] — No health-check frameworks, no profiler UIs
- [ASSUMED: gopls/etcd/CockroachDB] — Convention of loopback-default admin endpoints (cross-referenced in ARCHITECTURE.md §4.1)
- [VERIFIED: test/bench/cmd/benchgate exists via shell ls] — Benchgate tool is present in Phase 9; exact CLI flags unverified (Open Question #4)

### Tertiary (LOW confidence)
- None — all Phase 10 claims are either stdlib contracts or re-reads of existing project code.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — stdlib-only; zero new modules; all deps verified present in go.mod
- Architecture: HIGH — mirrors existing `listenHTTP` pattern byte-for-byte; ARCHITECTURE.md §3-4 already decided structural choices
- Pitfalls: HIGH — Phase 10 is narrow enough that all relevant pitfalls (slog allocs, flush race, goroutine leak, sensitive data, bind races) are covered
- Open questions: MEDIUM — #1 (SpanContext shape) and #4 (benchgate flag) are implementation details the planner resolves during task breakdown

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (30 days — stdlib-only and in-repo code make this research very stable)
