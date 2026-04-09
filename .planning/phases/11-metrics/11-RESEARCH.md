# Phase 11: Metrics - Research

**Researched:** 2026-04-08
**Domain:** Prometheus instrumentation for Go MCP daemon (RED metrics + lspool gauges + `/metrics` on admin listener)
**Confidence:** HIGH

## Summary

Phase 11 adds `prometheus/client_golang` as the only runtime dep, extends the Phase 10 `obs.Provider` with a `Metrics()` accessor over a pre-registered metric vector set, replaces `loggingMiddleware` with a `TelemetryMiddleware` that emits RED metrics for every MCP tool call, wires lspool-owned gauges directly via `obs.Provider`, and mounts `promhttp.Handler()` on the Phase 10 admin listener mux at `/metrics`. A CI-time Go test (`metrics_labels_test.go`) enforces the bounded-label allowlist at compile/test time; no runtime panic wrapper.

Every decision in CONTEXT.md is already locked. This research does not explore alternatives — it verifies API surfaces, pins versions, lists exact hot-path patterns to avoid allocations, and identifies the concrete integration seams in `internal/obs/`, `internal/mcp/middleware.go`, `internal/daemon/telemetry.go`, and `internal/kernel/lspool/`.

**Primary recommendation:** Use pre-registered `*prometheus.CounterVec` / `*prometheus.HistogramVec` stored on `obs.Provider`, resolved once per request via `.WithLabelValues(...)` in the middleware; register `collectors.NewGoCollector()` in `daemon/telemetry.go` alongside the existing `/healthz` / `/readyz` handlers; give lspool a `MetricsSink` interface satisfied by `obs.Provider` so the pool does not import `obs` directly.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions

**Histogram Buckets**
- **D-01:** Use `prometheus.DefBuckets` (`.005 / .01 / .025 / .05 / .1 / .25 / .5 / 1 / 2.5 / 5 / 10` seconds) for all tool latency histograms.
- **D-02:** Single bucket set for all tools (no per-category tuning). Covers 95% of observed distribution from Phase 9 baseline; revisit in v1.3 if p99 resolution is insufficient.

**Cardinality Lint**
- **D-03:** CI-time enforcement via Go test `metrics_labels_test.go` that enumerates all registered vectors and asserts each label name is in the allowlist.
- **D-04:** Allowlist (immutable constant): `tool_name`, `profile`, `mode`, `language`, `outcome`. Any other label fails the test.
- **D-05:** NO runtime panic wrapper — the list is known at compile time; CI catches drift before merge. Simpler and faster than defense-in-depth.

**Middleware Placement**
- **D-06:** Central `TelemetryMiddleware` in `internal/mcp/middleware.go` wraps ALL tool calls. Runs BEFORE `ProfileFilterMiddleware` so denied calls still emit spans/metrics with `outcome=denied`.
- **D-07:** Middleware resolves all 5 labels: `tool_name` from request, `profile`+`mode` from session, `language` from active workspace, `outcome` from result status.
- **D-08:** lspool emits its OWN gauges directly via `obs.Provider` — `internal/kernel/lspool/metrics.go` registers workers/evictions/restarts/circuit-state gauges. Colocated with pool internals. Middleware doesn't peek through accessors.

**RED Metrics**
- **D-09:** Rate: implicit from histogram count
- **D-10:** Errors: `serena_tool_calls_total{tool_name, profile, mode, language, outcome}` counter where `outcome` ∈ `{success, denied, invalid_args, not_found, circuit_open, ls_crash, timeout, internal}`
- **D-11:** Duration: `serena_tool_duration_seconds{tool_name, profile, mode, language}` histogram

**lspool Gauges**
- **D-12:** `serena_lspool_workers{language}` — active worker count per language
- **D-13:** `serena_lspool_evictions_total{language, reason}` counter — `reason` ∈ `{idle, pressure, crash, shutdown}`
- **D-14:** `serena_lspool_circuit_state{language}` — 0=closed, 1=half-open, 2=open
- **D-15:** `serena_lspool_restarts_total{language}` counter

**Go Runtime Collectors**
- **D-16:** Use `collectors.NewGoCollector()` (goroutines, GC, memory) — standard prom pattern, zero config

**Endpoint**
- **D-17:** `/metrics` registered on Phase 10 admin listener (loopback-only). No auth in v1.2.

### Claude's Discretion
- Exact initialization of metric vectors (`init()` vs explicit constructor called in `obs.Provider.Init`)
- Whether to expose `obs.Provider.Metrics()` accessor or package-level helpers
- Hot-path optimizations (pre-bound label values vs per-call lookups) — target: ≤ +3 allocs/op vs Phase 10 baseline

### Deferred Ideas (OUT OF SCOPE)
- SLO-tuned per-category histogram buckets (v1.3+)
- Native histograms (Prometheus 2.40+) — requires scrape-side support
- Metric exemplars linking histograms to trace IDs (requires Phase 12 tracing first)
- Runtime cardinality budget self-monitoring
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| METRIC-01 | `/metrics` endpoint on admin listener (Prometheus format) | `promhttp.Handler()` mounted on `daemon/telemetry.go` mux alongside `/healthz`/`/readyz`; listener already exists and is loopback-gated |
| METRIC-02 | RED histograms per tool (rate, errors, duration) with tuned buckets | `serena_tool_calls_total` counter + `serena_tool_duration_seconds` histogram (`prometheus.DefBuckets`) resolved in `TelemetryMiddleware` |
| METRIC-03 | lspool gauges (workers, evictions, restarts, circuit state) | `internal/kernel/lspool/metrics.go` + `MetricsSink` interface on Pool, implemented by `obs.Provider` |
| METRIC-04 | Go runtime collectors (goroutines, GC, memory) | `collectors.NewGoCollector()` + (recommended) `collectors.NewProcessCollector()` registered in `obs.Provider.Init` |
| METRIC-05 | Bounded-label contract enforced by CI lint | `internal/obs/metrics_labels_test.go` enumerates `*Vec` instances on `Provider` and asserts label names ⊆ allowlist |
</phase_requirements>

## Project Constraints (from CLAUDE.md)

Extracted directives that bound this phase's implementation:

- **Go only.** No CGO. `prometheus/client_golang` is CGO-free [VERIFIED: pkg.go.dev/github.com/prometheus/client_golang — module has no `cgo` build constraints].
- **Always run `go vet ./...` and `go test ./...` before completing any Go task.** Applies to every wave.
- **`gofmt -w .` formatting enforced.**
- **Caddy-style `init()` registration** is the established skill pattern — but metrics are NOT a skill. Phase 11 metrics live in `internal/obs/` and are wired by `daemon.New()`, not registered via `skill.Register`. [VERIFIED: CLAUDE.md + internal/daemon/daemon.go]
- **Additive only.** No kernel/skill signature changes per Phase 10 precedent. `Pool.NewPool` signature can grow a `MetricsSink` parameter (already a constructor, not an interface method).
- **Noop-default providers.** `obs.Noop(...)` must continue to satisfy the `Metrics()` contract without a real registry — return a noop metrics sink so call sites never branch on `nil`.

## Standard Stack

### Core

| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `github.com/prometheus/client_golang/prometheus` | v1.23.2 | Counter, Histogram, Gauge, Vec variants, Registry | De facto Go Prom client; required by D-01..D-15 [VERIFIED: pkg.go.dev/github.com/prometheus/client_golang, release notes] |
| `github.com/prometheus/client_golang/prometheus/promhttp` | (same module) | `/metrics` HTTP handler (`promhttp.HandlerFor`) | Native Prom integration; single-line wire to admin mux [CITED: prometheus.io/docs/guides/go-application] |
| `github.com/prometheus/client_golang/prometheus/collectors` | (same module) | `NewGoCollector`, `NewProcessCollector` | Free runtime + process metrics per D-16 [CITED: pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors] |

**Version verification:**
- `prometheus/client_golang` latest stable at v1.23.x (v1.23.2 as of late 2025) [VERIFIED: github.com/prometheus/client_golang/releases]. Training-data note: versions drift; run `go get github.com/prometheus/client_golang@latest` at wave start and record the resolved version in the commit message.

### Supporting

| Library | Version | Purpose | When to Use |
|---------|---------|---------|-------------|
| `prometheus/common` | (transitive) | Exposition format constants | Transitive — do not import directly |
| `prometheus/procfs` | (transitive) | `NewProcessCollector` backing | Transitive — do not import directly |

### Alternatives Considered (ALL REJECTED by CONTEXT.md)

| Instead of | Could Use | Why rejected |
|------------|-----------|--------------|
| `prometheus/client_golang` | `go.opentelemetry.io/otel/metric` + prom exporter bridge | Two APIs, same output. Rejected in STACK.md. Phase 11 metrics must be Prometheus-native. |
| `collectors.NewGoCollector` | `collectors.NewBuildInfoCollector` only | D-16 explicitly says Go collector for goroutines/GC/memory. |
| Runtime label-allowlist panic wrapper | Defence-in-depth interception of `.WithLabelValues` | D-05 explicitly rejects this — CI test is sufficient, simpler, faster. |

**Installation:**

```bash
go get github.com/prometheus/client_golang@latest
```

Transitive adds: `prometheus/common`, `prometheus/procfs`, `cespare/xxhash/v2`, `beorn7/perks`, `golang/protobuf` (or `google.golang.org/protobuf` — already present), `munnerz/goautoneg`. All CGO-free, all small, all well-maintained. [VERIFIED: pkg.go.dev dependency tree]

## Architecture Patterns

### Project Structure

```
internal/
├── obs/
│   ├── obs.go                  # MODIFY: add metrics fields + Metrics() accessor + Init/Shutdown
│   ├── handler.go              # unchanged (Phase 10)
│   ├── metrics.go              # NEW: vector constructors, Registry, MetricsSink adapter
│   └── metrics_labels_test.go  # NEW: CI label allowlist enforcement (METRIC-05)
├── daemon/
│   ├── telemetry.go            # MODIFY: mount /metrics via promhttp.HandlerFor
│   └── daemon.go               # MODIFY: construct Provider with metrics enabled, pass to MCP + lspool
├── mcp/
│   ├── middleware.go           # MODIFY: TelemetryMiddleware replaces loggingMiddleware; runs BEFORE ProfileFilterMiddleware
│   └── server.go               # MODIFY: NewSerenaMCPServer accepts *obs.Provider
├── kernel/lspool/
│   ├── metrics.go              # NEW: MetricsSink interface + noop + pool gauge hooks
│   ├── pool.go                 # MODIFY: NewPool accepts MetricsSink; increment on lifecycle events
│   └── circuit.go              # MODIFY: SetState() wired through MetricsSink
test/bench/
└── metrics_bench_test.go       # NEW: middleware alloc delta vs Phase 10 baseline
```

### Pattern 1: Pre-Registered Vectors on Provider

**What:** Metric `*Vec` instances are constructed ONCE during `obs.Provider.Init()` and stored as struct fields. Middleware resolves per-call via `.WithLabelValues(tool, profile, mode, language, outcome)`.

**When to use:** Every RED metric. Never call `prometheus.NewCounterVec` outside `Init` — that path creates a new vector on every call, causes double-registration panics, and leaks memory.

**Example:**

```go
// Source: prometheus/client_golang docs — https://pkg.go.dev/github.com/prometheus/client_golang/prometheus#CounterVec
// [CITED: pkg.go.dev/github.com/prometheus/client_golang/prometheus]

package obs

import "github.com/prometheus/client_golang/prometheus"

// AllowedLabels is the bounded-label allowlist enforced by CI test.
// Changing this list requires a matching change in metrics_labels_test.go.
var AllowedLabels = [...]string{"tool_name", "profile", "mode", "language", "outcome"}

type Metrics struct {
    registry       *prometheus.Registry
    toolCalls      *prometheus.CounterVec
    toolDuration   *prometheus.HistogramVec

    lspoolWorkers        *prometheus.GaugeVec
    lspoolEvictions      *prometheus.CounterVec
    lspoolCircuitState   *prometheus.GaugeVec
    lspoolRestarts       *prometheus.CounterVec
}

func newMetrics() *Metrics {
    reg := prometheus.NewRegistry() // NOT prometheus.DefaultRegisterer — avoids global state (PITFALLS.md meta-rule)
    m := &Metrics{
        registry: reg,
        toolCalls: prometheus.NewCounterVec(
            prometheus.CounterOpts{
                Name: "serena_tool_calls_total",
                Help: "MCP tool calls by outcome (RED: errors).",
            },
            []string{"tool_name", "profile", "mode", "language", "outcome"},
        ),
        toolDuration: prometheus.NewHistogramVec(
            prometheus.HistogramOpts{
                Name:    "serena_tool_duration_seconds",
                Help:    "MCP tool call latency (RED: duration).",
                Buckets: prometheus.DefBuckets, // D-01
            },
            []string{"tool_name", "profile", "mode", "language"},
        ),
        lspoolWorkers: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{Name: "serena_lspool_workers", Help: "Active LS workers per language."},
            []string{"language"},
        ),
        lspoolEvictions: prometheus.NewCounterVec(
            prometheus.CounterOpts{Name: "serena_lspool_evictions_total", Help: "LS worker evictions by reason."},
            []string{"language", "reason"},
        ),
        lspoolCircuitState: prometheus.NewGaugeVec(
            prometheus.GaugeOpts{Name: "serena_lspool_circuit_state", Help: "Circuit breaker state (0=closed,1=half-open,2=open)."},
            []string{"language"},
        ),
        lspoolRestarts: prometheus.NewCounterVec(
            prometheus.CounterOpts{Name: "serena_lspool_restarts_total", Help: "LS worker restarts per language."},
            []string{"language"},
        ),
    }
    reg.MustRegister(
        m.toolCalls, m.toolDuration,
        m.lspoolWorkers, m.lspoolEvictions, m.lspoolCircuitState, m.lspoolRestarts,
        collectors.NewGoCollector(),          // D-16
        collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}), // bonus: RSS/FDs, mandatory per PROJECT memory requirement
    )
    return m
}
```

### Pattern 2: Middleware Label Resolution — hot path

**What:** The middleware resolves all five labels once per call, calls `.WithLabelValues(...).Inc()` and `.Observe(...)`, then returns the inner result unchanged.

**When to use:** `TelemetryMiddleware` in `internal/mcp/middleware.go`. Runs BEFORE `ProfileFilterMiddleware` (D-06) so denied listings still increment with `outcome=denied`.

**Example:**

```go
// Source: extrapolated from existing loggingMiddleware at internal/mcp/middleware.go:88-110
// [VERIFIED: read of middleware.go]

func TelemetryMiddleware(provider *obs.Provider, getSession func(ctx context.Context) *SessionInfo, logger *slog.Logger) mcpsdk.Middleware {
    m := provider.Metrics() // never nil — noop returns a sink with no-op counters
    return func(next mcpsdk.MethodHandler) mcpsdk.MethodHandler {
        return func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
            // Only instrument tools/call — tools/list and other methods stay cheap.
            if method != "tools/call" {
                return next(ctx, method, req)
            }

            start := time.Now()
            toolName := extractToolName(req) // cheap — field read
            snap := snapshotSession(ctx, getSession) // pre-computed struct, cheap
            profile, mode, language := snap.Profile, snap.Mode, snap.Language

            result, err := next(ctx, method, req)
            elapsed := time.Since(start).Seconds()

            outcome := classifyOutcome(result, err) // returns one of the 8 enum values (D-10)

            // Hot-path: WithLabelValues returns a cached observer, then one method call each.
            m.ToolCalls.WithLabelValues(toolName, profile, mode, language, outcome).Inc()
            m.ToolDuration.WithLabelValues(toolName, profile, mode, language).Observe(elapsed)

            // Log line (existing behavior preserved)
            if err != nil {
                logger.Warn("request failed", "method", method, "tool", toolName, "outcome", outcome, "duration", elapsed, "error", err)
            } else {
                logger.Info("request handled", "method", method, "tool", toolName, "outcome", outcome, "duration", elapsed)
            }
            return result, err
        }
    }
}
```

**Hot-path alloc budget (≤ +3 allocs/op vs Phase 10):**

1. `WithLabelValues` internally builds a `[]string` label slice — if the args are passed as positional string variadics, the slice escapes to the heap. This is 1 alloc per call per Vec.
2. `time.Since` + `Seconds()` is alloc-free.
3. `classifyOutcome` must not format strings — use constants only.
4. `extractToolName` must read a pre-parsed field, not `req.Params` unmarshal.

Expected baseline vs Phase 10: +2 allocs/op (one per `WithLabelValues` × 2 vectors). The +3 budget gives one alloc of headroom for log-line field construction. Verify with `test/bench/metrics_bench_test.go` running against `test/bench/baselines/v1.2-phase10-github-hosted.txt`. [VERIFIED: prom client source — `MetricVec.getOrCreateMetricWithLabelValues` allocates a `[]string` on miss, caches on hit; curryWith is not used here]

### Pattern 3: Outcome Classification — closed enum

**What:** Map MCP result/error to exactly one of the 8 outcome constants.

**When to use:** `classifyOutcome` in middleware — called once per tool call. Keeps label cardinality bounded per D-10.

**Example:**

```go
// Outcome enum is closed (PITFALLS.md #2: bounded labels).
const (
    outcomeSuccess     = "success"
    outcomeDenied      = "denied"      // ProfileFilterMiddleware rejected
    outcomeInvalidArgs = "invalid_args"
    outcomeNotFound    = "not_found"
    outcomeCircuitOpen = "circuit_open" // lspool.ErrCircuitOpen (Phase 13 wires this)
    outcomeLSCrash     = "ls_crash"
    outcomeTimeout     = "timeout"     // context.DeadlineExceeded
    outcomeInternal    = "internal"    // fallback
)

func classifyOutcome(result mcpsdk.Result, err error) string {
    switch {
    case errors.Is(err, context.DeadlineExceeded):
        return outcomeTimeout
    case errors.Is(err, lspool.ErrCircuitOpen): // Phase 13 hookup; Phase 11 uses errors.Is which returns false safely
        return outcomeCircuitOpen
    case errors.Is(err, errPermissionDenied): // defined in mcp package when ProfileFilter rejects
        return outcomeDenied
    case err != nil:
        return outcomeInternal
    }
    // Tool-level error via IsError=true on CallToolResult
    if ctr, ok := result.(*mcpsdk.CallToolResult); ok && ctr.IsError {
        return outcomeInternal // or map specific error codes; keep closed
    }
    return outcomeSuccess
}
```

### Pattern 4: lspool MetricsSink interface

**What:** `lspool` does not import `internal/obs` directly. Instead it defines a small `MetricsSink` interface that `obs.Provider` satisfies. Keeps the pool's dependency footprint small and testable.

**When to use:** Phase 11 wiring in `internal/kernel/lspool/metrics.go`.

**Example:**

```go
// internal/kernel/lspool/metrics.go

package lspool

// MetricsSink is the minimal surface lspool needs from the observability layer.
// Implemented by obs.Provider; noopSink is used when the pool runs without metrics
// (tests, bootstrap before Provider is ready).
type MetricsSink interface {
    LSPoolWorkers(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitState(language string, state float64) // 0=closed, 1=half-open, 2=open
    LSPoolRestart(language string)
}

type noopSink struct{}
func (noopSink) LSPoolWorkers(string, float64)       {}
func (noopSink) LSPoolEviction(string, string)       {}
func (noopSink) LSPoolCircuitState(string, float64)  {}
func (noopSink) LSPoolRestart(string)                {}
```

Pool constructor grows one parameter: `NewPool(cfg, registry, installer, pressure, logger, metrics MetricsSink)`. Call sites in `daemon.go` pass `provider.LSPoolSink()`; tests pass `lspool.NoopSink()`.

### Anti-Patterns to Avoid

- **`prometheus.MustRegister` on `DefaultRegisterer`.** Uses global state. Breaks parallel tests. Always use `prometheus.NewRegistry()` owned by the `Metrics` struct. [CITED: client_golang best practices; PITFALLS.md #2 global-avoidance]
- **`.WithLabelValues` inside a loop without caching.** Each call does a map lookup. For hot paths hit > 1000/sec, pre-bind via `counter.WithLabelValues("foo").Inc()` once per label combination at registration. Phase 11 tool middleware is called ≤ 100/s in typical use → no pre-binding needed.
- **Dynamic label names based on request content.** Absolute ban; this is exactly what PITFALLS.md #2 warns against. CI lint in `metrics_labels_test.go` enforces.
- **Calling `prometheus.NewCounterVec` at request time.** Double registration panics; silent memory growth. All construction happens in `newMetrics()`.
- **Wiring metrics to `http.DefaultServeMux`.** Admin listener uses its own mux (`daemon/telemetry.go:56`). Using `DefaultServeMux` would expose `/metrics` on any stray listener. [VERIFIED: read of telemetry.go]

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Prometheus exposition format | Custom text writer | `promhttp.HandlerFor(registry, promhttp.HandlerOpts{})` | OpenMetrics + content negotiation + escaping edge cases. |
| Histogram bucketing | Manual `atomic.Uint64` arrays | `prometheus.HistogramVec` | Lock-free hot path, correct bucket boundaries, integrates with Prom client exposition. |
| Goroutine/GC metrics | `runtime.NumGoroutine()` polling goroutine | `collectors.NewGoCollector()` | Prom client snapshots on scrape — no background ticker, zero steady-state cost. |
| Process RSS/FDs | `/proc` parsing or `syscall.Getrusage` | `collectors.NewProcessCollector()` | Cross-platform, correct on macOS+Linux, handles FD count. |
| Label value interning | `sync.Map[string]prometheus.Counter` | `CounterVec.WithLabelValues` | Already cached internally; reinventing this is a PITFALLS.md #2 landmine. |
| Outcome string enumeration | Dynamic `err.Error()` label | Closed 8-value const set | Error-string labels are the canonical cardinality explosion path. |

**Key insight:** Every "prometheus helper" package on GitHub either reinvents something `prometheus/client_golang` already does, or is a Prom exporter for a specific system. For in-process metrics in Go, the client library is complete.

## Common Pitfalls

### Pitfall 1: Hot-path allocation explosion from dynamic label values
**What goes wrong:** Passing `fmt.Sprintf("%d", n)` or any per-call string construction to `.WithLabelValues` burns allocations and stalls the middleware on every tool call.
**Why it happens:** `WithLabelValues` takes `...string`; each formatted arg escapes.
**How to avoid:** All label values are pre-computed constants OR directly from session snapshot (already string-typed). `classifyOutcome` returns one of 8 `const string` values. No `fmt.Sprintf` in the middleware path.
**Warning signs:** `go test -bench=BenchmarkTelemetryMiddleware -benchmem` shows > +3 allocs/op delta vs Phase 10 baseline.
**Reference:** PITFALLS.md #3 (analogous slog hot-path warning).

### Pitfall 2: Double-registration panic on test re-runs
**What goes wrong:** `prometheus.MustRegister` panics if the same vector is registered twice on the same registry — common in tests that construct multiple `Provider` instances against `prometheus.DefaultRegisterer`.
**How to avoid:** Every `Provider.Init()` creates its own `prometheus.NewRegistry()`. Never touch `DefaultRegisterer`. Tests construct fresh providers per test.
**Warning signs:** Test panic `duplicate metrics collector registration attempted`.

### Pitfall 3: `collectors.NewGoCollector` default set changed in v1.12+
**What goes wrong:** v1.12 introduced `WithGoCollectorRuntimeMetrics` options; the default collector set shifted. Older examples that pass `collectors.NewGoCollector()` with no args are still valid, but the metric names visible on `/metrics` differ slightly between versions.
**How to avoid:** Use `collectors.NewGoCollector()` (no args) — matches D-16 intent. Pin the version in `go.mod` explicitly and document the exposed metric subset in the metrics integration test. [CITED: github.com/prometheus/client_golang/blob/main/CHANGELOG.md v1.12.0]
**Warning signs:** CI integration test fails when upstream bumps the collector default.

### Pitfall 4: `/metrics` handler slower than tool calls during scrape
**What goes wrong:** `promhttp.Handler()` performs exposition under the registry's read lock; if a tool call is mid-`.Inc()` during scrape, no stall occurs (counter increments are atomic) — but `NewGoCollector` invokes `runtime.ReadMemStats` which stops the world briefly.
**How to avoid:** Accept — this is sub-millisecond on Go 1.25. Don't scrape more often than every 5s. Do NOT hand-roll a cached snapshot. Documented in v1.1 Prom best practices. [CITED: prometheus.io/docs/practices/instrumentation]
**Warning signs:** p99 latency spikes on the same cadence as Prometheus scrape interval.

### Pitfall 5: Scraping on a non-loopback admin listener exposes internal labels publicly
**What goes wrong:** If an operator sets `AdminAddr` to `0.0.0.0:9090`, a bug in `validateAdminAddr` would expose metrics (including `profile` + `mode` labels which leak security posture).
**How to avoid:** Phase 10 already enforces loopback-only via `validateAdminAddr` (`daemon/telemetry.go:92-105`). Phase 11 adds no new binding logic. [VERIFIED: read of telemetry.go]
**Warning signs:** N/A — impossible at bind time.

### Pitfall 6: Labels derived from `session` pointer race with `switch_mode`
**What goes wrong:** Reading `session.Profile` and `session.Mode` across two field accesses can see an inconsistent snapshot during a concurrent mode switch.
**How to avoid:** `SessionInfo.Snapshot()` already returns an atomic snapshot struct — use it. Same pattern as `ProfileFilterMiddleware` at `middleware.go:52-54`. [VERIFIED: read of middleware.go]
**Warning signs:** Rare mismatched `profile`/`mode` label pairs in dashboards; caught by `go test -race`.

### Pitfall 7: `prometheus.DefBuckets` p99 resolution
**What goes wrong:** PITFALLS.md #8 flags default buckets as SLO-unfit — p99 lands on 500ms edge when real value is 180ms. CONTEXT.md D-02 **accepts this tradeoff for v1.2** with explicit revisit in v1.3.
**How to avoid:** Document the limitation in the phase completion notes. Do NOT silently tune buckets away from `DefBuckets` — that contradicts D-01.
**Warning signs:** Dashboard p99 values cluster exactly on bucket edges. Expected behavior for v1.2.

## Runtime State Inventory

Phase 11 is purely additive — no rename, no migration. No runtime state exists to enumerate.

- **Stored data:** None — metrics are in-process, rebuilt on restart.
- **Live service config:** None — no external metrics backend.
- **OS-registered state:** None — `/metrics` runs inside the existing admin listener.
- **Secrets/env vars:** None.
- **Build artifacts:** `go.sum` will gain prometheus entries after `go get`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | build/test | ✓ | 1.25.1 | — |
| `prometheus/client_golang` | all metrics | ✗ (not yet in go.mod) | — | `go get github.com/prometheus/client_golang@latest` at wave start |
| `collectors.NewProcessCollector` (procfs) | process metrics | platform-dependent | — | Collector returns empty on non-Linux for some fields; acceptable |
| Phase 10 admin listener | `/metrics` host | ✓ | internal/daemon/telemetry.go | — |
| Phase 9 baseline | alloc-budget gate | ✓ | `test/bench/baselines/v1.2-phase10-github-hosted.txt` | — |

**Missing dependencies with no fallback:** None — `go get` resolves the single missing dep.

**Missing dependencies with fallback:** None.

## Validation Architecture

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go stdlib `testing` + `testing.B.Loop` (Go 1.25) |
| Config file | none |
| Quick run command | `go test ./internal/obs/... ./internal/mcp/... ./internal/kernel/lspool/...` |
| Full suite command | `go test ./... && go vet ./...` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| METRIC-01 | GET `/metrics` on admin listener returns Prom exposition with `serena_*` metrics | integration | `go test ./internal/daemon/... -run TestAdmin_Metrics -x` | Wave 0 |
| METRIC-02 | `serena_tool_calls_total` and `serena_tool_duration_seconds` incremented per tool call with correct labels | unit | `go test ./internal/mcp/... -run TestTelemetryMiddleware -x` | Wave 0 |
| METRIC-02 (perf) | Middleware ≤ +3 allocs/op vs Phase 10 baseline | benchmark | `go test ./test/bench/... -run=^$ -bench=BenchmarkTelemetryMiddleware -benchmem -count=10` | Wave 0 |
| METRIC-03 | lspool gauges update on worker spawn/evict/restart/circuit-state | unit | `go test ./internal/kernel/lspool/... -run TestMetricsSink -x` | Wave 0 |
| METRIC-04 | `/metrics` response contains `go_goroutines`, `go_gc_*`, `process_resident_memory_bytes` | integration | `go test ./internal/daemon/... -run TestAdmin_Metrics_GoCollector -x` | Wave 0 |
| METRIC-05 | Every registered vector uses labels ⊆ allowlist | CI lint | `go test ./internal/obs/... -run TestMetricsLabelsAllowlist -x` | Wave 0 |

### Sampling Rate

- **Per task commit:** `go vet ./... && go test ./internal/obs/... ./internal/mcp/... ./internal/kernel/lspool/... ./internal/daemon/...`
- **Per wave merge:** `go test ./... && go vet ./...`
- **Phase gate:** Full suite green + `benchstat test/bench/baselines/v1.2-phase10-github-hosted.txt new.txt` shows ≤ +3 allocs/op on `BenchmarkTelemetryMiddleware` before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/obs/metrics.go` — Metrics struct, vector constructors, MetricsSink adapter
- [ ] `internal/obs/metrics_labels_test.go` — CI allowlist enforcement (METRIC-05)
- [ ] `internal/obs/metrics_test.go` — Init/Shutdown, noop path, double-registration protection
- [ ] `internal/mcp/telemetry_middleware_test.go` — per-label resolution, outcome classification table
- [ ] `internal/kernel/lspool/metrics_test.go` — MetricsSink interface conformance, noop sink
- [ ] `internal/daemon/telemetry_metrics_test.go` — integration test hitting `/metrics` through real admin listener
- [ ] `test/bench/metrics_bench_test.go` — alloc delta gate (target ≤ +3 allocs/op)
- [ ] `go get github.com/prometheus/client_golang@latest` — must run before any test compiles

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | `/metrics` is loopback-only; Phase 10 enforces via `validateAdminAddr` |
| V3 Session Management | no | Stateless HTTP scrape |
| V4 Access Control | yes | Loopback bind (v1.2); auth deferred to v1.3 per CONTEXT.md D-17 |
| V5 Input Validation | no | `/metrics` is read-only, no query params consumed |
| V6 Cryptography | no | No keys; admin listener is plain HTTP on loopback |
| V7 Error Handling | yes | Outcome label classification must not leak error content (closed enum) |
| V9 Communication | yes | Plain HTTP acceptable on `127.0.0.1` only |
| V14 Configuration | yes | `AdminAddr` default is empty string; operator must opt-in |

### Known Threat Patterns for Go + Prometheus

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| Cardinality explosion via request-derived labels | Denial of Service | Bounded-label allowlist + CI test (METRIC-05) |
| Metric label leaks security posture (`profile`, `mode`) to unauthorized scrapers | Information Disclosure | Loopback-only bind (Phase 10 validateAdminAddr) |
| Outcome label leaks error internals (`outcome="sql: no rows..."`) | Information Disclosure | Closed 8-value enum (D-10) |
| `collectors.NewProcessCollector` exposes PID + executable path | Information Disclosure | Accept — loopback-only, operator-controlled binding |
| Scrape-time stop-the-world from `runtime.ReadMemStats` | Denial of Service (self) | Document 5s minimum scrape interval in USAGE.md (Phase 14) |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `MCP SDK middleware` exposes request/result objects sufficient to extract tool name cheaply without re-unmarshaling | Pattern 2 | If `extractToolName` requires JSON unmarshal, alloc budget blows; planner must add a parsed-args carrier. Verify against `mcpsdk.Request` and `mcpsdk.CallToolRequest` at wave start. |
| A2 | `SessionInfo.Snapshot()` already carries `Language` field or active workspace language is cheaply resolvable from ctx | Pattern 2 / D-07 | If `language` must be looked up through workspace registry on every call, alloc budget tightens. Planner should verify `SessionInfo` struct at wave start and add a `Language` field if missing — single-line change. |
| A3 | `errPermissionDenied` sentinel exists or can be added to `internal/mcp/` without circular imports | Pattern 3 | Minor — if not, use a string-match on a known error type defined in the middleware's own file. Either works. |
| A4 | Phase 9 baseline file `v1.2-phase10-github-hosted.txt` contains a benchmark comparable to `BenchmarkTelemetryMiddleware` (i.e., the Phase 10 middleware was benchmarked with same harness) | Validation Architecture | If no comparable Phase 10 bench exists, Phase 11 must add BOTH a Phase-10-style benchmark AND the new middleware benchmark in Wave 0 to establish the delta. Planner check at wave start. |
| A5 | `lspool.ErrCircuitOpen` sentinel will be promoted to exported symbol in Phase 13, but using `errors.Is(err, lspool.ErrCircuitOpen)` in Phase 11 middleware is safe (returns false until Phase 13 wires the error path) | Pattern 3 | Safe — `errors.Is` handles nil/missing match gracefully. `lspool.ErrCircuitOpen` is already declared in `pool.go:39` [VERIFIED: read of pool.go]. |
| A6 | `collectors.NewProcessCollector(collectors.ProcessCollectorOpts{})` is acceptable beyond D-16 (which only mentions `NewGoCollector`) | Standard Stack table | Low — PROJECT.md lists RSS/process metrics as table stakes; adding it is discretionary per Claude's Discretion clause. If user objects, drop and rely on Go collector only. |

**If these assumptions prove wrong:** Planner pauses Wave 0, resolves via a quick discuss-phase iteration, then continues. None are blocking for the overall shape of the phase — they only affect specific task wording.

## Open Questions (RESOLVED)

**Status: RESOLVED during Phase 11 planning (2026-04-08).** See `11-01-PLAN.md` through `11-04-PLAN.md` for task-level wiring.

- **Q1 → RESOLVED**: histogram excludes `outcome` per D-11; carried verbatim into Plan 01 + Plan 02. Deferred "per-outcome p99" to v1.3.
- **Q2 → RESOLVED**: Plan 01 Task 3 uses `promhttp.HandlerFor(registry, opts)` — no DefaultGatherer.
- **Q3 → RESOLVED**: Plan 01 Task 1 extends `obs.Noop(...)` to construct `newMetrics()` so `Provider.Metrics()` is never nil; real wiring happens in `daemon.New` via Plan 03 Task 2 interface assertion.
- **A1 → RESOLVED in Plan 02 Task 2**: `extractToolName` reads `*mcpsdk.CallToolRequest.Params.Name` at wave start; verified against go-sdk types at wave entry.
- **A2 → RESOLVED in Plan 02 Task 1**: `SessionInfo.Language` field + `SetLanguage` setter added, included in Snapshot.
- **A4 → RESOLVED in Plan 04 Task 1**: Case A/B branch — if Phase 10 baseline lacks a comparable bench, `BenchmarkBaselineMiddleware` is added alongside `BenchmarkTelemetryMiddleware` in the same run to compute an environment-stable delta.

### Original questions (for traceability)

1. **Should `serena_tool_duration_seconds` include `outcome` as a label?**
   - What we know: D-11 explicitly excludes `outcome` from the histogram (only counter carries it). This is correct Prometheus practice — histograms with high-cardinality labels balloon memory.
   - What's unclear: Dashboards sometimes want "p99 of successful calls only" which requires `outcome` on the histogram.
   - Recommendation: Follow D-11 verbatim. If operators need per-outcome latency later, add a v1.3 decision. Do not deviate in Phase 11.

2. **`promhttp.Handler()` vs `promhttp.HandlerFor(registry, opts)`?**
   - What we know: `promhttp.Handler()` uses `prometheus.DefaultGatherer` (global). `HandlerFor` accepts a specific registry.
   - Recommendation: **Use `HandlerFor`** — consistent with the "no globals" rule (PITFALLS.md meta-rule; Phase 10 precedent). One-line difference, zero runtime cost. [CITED: pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp#HandlerFor]

3. **Where does `obs.Provider.Init()` happen in `daemon.New`?**
   - What we know: Phase 10 constructs `obs.Noop(handler)`. Phase 11 needs real metrics.
   - Recommendation: Add `obs.NewProvider(cfg)` that returns a fully-wired provider (metrics + slog handler) when `cfg.Observability.AdminAddr != ""`, else `obs.Noop(handler)`. Keeps the noop path zero-cost.

## Sources

### Primary (HIGH confidence)

- `internal/obs/obs.go`, `internal/obs/handler.go` — Phase 10 Provider surface (read)
- `internal/daemon/telemetry.go` — admin listener mux integration point (read)
- `internal/mcp/middleware.go` — existing logging middleware + ProfileFilterMiddleware pattern (read)
- `internal/mcp/server.go` — `InstallMiddleware(server, logger)` call site (read)
- `internal/kernel/lspool/pool.go` — `ErrCircuitOpen` sentinel, `NewPool` signature (read)
- `go.mod` — confirmed no `prometheus/client_golang` dependency yet (read)
- [pkg.go.dev/github.com/prometheus/client_golang/prometheus](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus) — CounterVec/HistogramVec/Registry APIs
- [pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/promhttp) — `HandlerFor`
- [pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors](https://pkg.go.dev/github.com/prometheus/client_golang/prometheus/collectors) — `NewGoCollector`, `NewProcessCollector`
- [prometheus.io/docs/practices/naming](https://prometheus.io/docs/practices/naming/) — metric naming conventions
- [robustperception.io/cardinality-is-key](https://www.robustperception.io/cardinality-is-key/) — cardinality bound rationale
- `.planning/research/STACK.md`, `ARCHITECTURE.md`, `PITFALLS.md` — milestone-level research

### Secondary (MEDIUM confidence)

- [github.com/prometheus/client_golang/releases](https://github.com/prometheus/client_golang/releases) — v1.23.x stable (verify via `go get @latest` at wave start)
- [prometheus.io/docs/guides/go-application](https://prometheus.io/docs/guides/go-application/) — wiring tutorial

### Tertiary (LOW confidence)

- None — all claims backed by Context7/official docs or direct code reads.

## Metadata

**Confidence breakdown:**

- Standard stack: HIGH — single dep, official source, locked by CONTEXT.md
- Architecture: HIGH — all integration points verified against Phase 10 code
- Pitfalls: HIGH — cross-referenced with milestone PITFALLS.md + prom client docs
- Validation: HIGH — Phase 10 already has admin-listener integration test pattern to mirror

**Research date:** 2026-04-08
**Valid until:** 2026-05-08 (30 days; `prometheus/client_golang` is stable, low-drift)
