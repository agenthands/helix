# Phase 13: Graceful Degradation - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Make Serena survive slow LS workers, crashes, and memory pressure with typed errors, bounded budgets, and clean shutdown. This phase does NOT add new tool capabilities — it hardens the existing 28-tool surface against failure modes.

</domain>

<decisions>
## Implementation Decisions

### Timeout Budgets
- **D-01:** Deadlines originate in the daemon's TelemetryMiddleware. The middleware wraps `context.WithTimeout` per tool class BEFORE calling the kernel handler. Single enforcement point. Forwarder passes through unchanged.
- **D-02:** Per-class timeout budgets are configurable via project.yml with hardcoded defaults: read 5s / search 15s / edit 10s / index 120s / diagnostics 20s. Config path: `degradation.timeout_{class}`.
- **D-03:** Tool-to-class mapping is a closed map in `internal/degrade/` — each of the 28 tools maps to one of 5 classes (read, search, edit, index, diagnostics).

### Circuit Breaker Tuning
- **D-04:** Decorrelated jitter for backoff: `sleep = min(cap, random_between(base, sleep*3))`. AWS-recommended pattern prevents thundering herd when multiple workers crash simultaneously.
- **D-05:** Restart budget: 3 consecutive crashes then circuit stays open until TTL expiry or manual intervention. Prevents restart loops burning CPU.
- **D-06:** Single-probe half-open: when backoff expires, exactly one request is admitted as a probe. If it succeeds → close circuit. If it fails → reopen with increased backoff.

### Error Surface Design
- **D-07:** Typed `ErrCircuitOpen` carries: Language, BackoffRemaining, Failures, RetryAfter. Structured envelope returned in MCP error response with retry-after hint.
- **D-08:** `classifyOutcome` in middleware.go already handles `ErrCircuitOpen` (maps to `circuit_open` outcome) and `DeadlineExceeded` (maps to `timeout`). These paths are wired — Phase 13 makes them produce real typed errors instead of generic Go errors.

### Memory Pressure & Shutdown
- **D-09:** Config-driven `runtime/debug.SetMemoryLimit` wired from `degradation.memory_limit` in project.yml. Two layers: Go GC budget (GOMEMLIMIT) + existing platform pressure eviction (vm_stat/cgroups) operate independently.
- **D-10:** SIGTERM triggers errgroup context cancellation. In-flight tool calls get their context cancelled. `ShutdownTimeout` (configurable, default 10s) bounds the drain. Kernel.Shutdown drains workers. Tracing flush follows with dedicated 5s context (already implemented in Phase 12).
- **D-11:** Two-stage shutdown ordering preserved from Phase 12: (1) kernel drain → (2) tracing flush → (3) listener close → (4) socket cleanup.

### Claude's Discretion
- Tool-to-class mapping assignments (which tools go in which timeout class)
- Specific jitter algorithm parameters (base, cap values)
- Config key naming for degradation settings
- Integration test design for SIGTERM mid-request drain

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Circuit Breaker
- `internal/kernel/lspool/circuit.go` — Existing CB implementation (exponential backoff, no jitter, no restart budget)
- `internal/kernel/lspool/metrics.go` — MetricsSink interface with restart/circuit state hooks (Phase 11)

### Timeout & Error Handling
- `internal/mcp/middleware.go` — TelemetryMiddleware where deadline wrapping will be added; classifyOutcome already routes ErrCircuitOpen and DeadlineExceeded
- `internal/kernel/lspool/pool.go` — ErrCircuitOpen sentinel (if exists) or where to add it

### Shutdown
- `internal/daemon/shutdown.go` — Existing 2-phase shutdown with tracing flush
- `internal/daemon/daemon.go:Run()` — errgroup orchestration, signal handling

### Config
- `internal/config/config.go` — Where DegradationConfig struct will be added
- `internal/config/defaults.go` — Where default timeout budgets will be registered

### Prior Phase Context
- `.planning/phases/12-tracing-end-to-end/12-CONTEXT.md` — Tracing decisions (D-17 hot-path budget, D-01 no globals)
- `.planning/phases/11-metrics/11-CONTEXT.md` — Metrics label contract, TelemetryMiddleware design

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `CircuitBreaker` in `lspool/circuit.go` — needs jitter + restart budget + single-probe, but the structure is there
- `MetricsSink` interface — restart/evict/circuit hooks already wired from Phase 11
- `classifyOutcome` — already classifies `ErrCircuitOpen` and `DeadlineExceeded` outcomes
- `shutdown.go` — 2-phase shutdown with tracing flush already ordered correctly

### Established Patterns
- Degraded-optional: construction failures log warning and fall back to noop (Phase 12 `obs.WithTracing`)
- Explicit DI, no globals: tracer/metrics/sink all passed as constructor parameters
- Config via koanf with 4-layer precedence (CLI > project > user > defaults)

### Integration Points
- TelemetryMiddleware: deadline wrapping goes here (before `next(ctx, method, req)`)
- `lspool.Pool.AcquireLease`: where circuit check + typed error return happens
- `daemon.Run`: SIGTERM signal handler + errgroup context cancellation
- `config.SerenaConfig`: new `Degradation` field alongside existing `Observability`

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches. The roadmap success criteria are precise enough to guide implementation.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 13-graceful-degradation*
*Context gathered: 2026-04-10*
