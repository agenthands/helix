---
phase: 11-metrics
plan: 03
subsystem: lspool
tags: [metrics, lspool, prometheus, circuit-breaker, decoupling]
requires:
  - 11-01 (obs.Metrics LSPool* helper contract, frozen signatures)
  - 11-02 (daemon.go middleware wiring, avoids wave-2 merge conflict)
provides:
  - "lspool.MetricsSink interface (internal/kernel/lspool/metrics.go)"
  - "lspool.NoopSink default for tests and bootstrap paths"
  - "Pool + CircuitBreaker lifecycle hooks emitting gauge/counter updates"
  - "Compile-time assertion pinning obs.Metrics <-> lspool.MetricsSink"
affects:
  - "internal/kernel/lspool/pool.go (NewPool signature +metrics param)"
  - "internal/kernel/lspool/circuit.go (NewCircuitBreaker signature +language+sink)"
  - "internal/kernel/kernel.go (NewKernel signature +metrics param)"
  - "internal/daemon/daemon.go (obs provider created before kernel)"
tech-stack:
  added: []
  patterns:
    - "Decoupled sink interface (D-08): lspool has zero imports of internal/obs"
    - "Closed-enum label values for bounded cardinality (D-13 EvictXxx, D-14 CircuitXxx)"
    - "Compile-time interface assertion in sibling test file to prevent plan drift"
key-files:
  created:
    - internal/kernel/lspool/metrics.go
    - internal/kernel/lspool/metrics_test.go
    - internal/daemon/wiring_test.go
  modified:
    - internal/kernel/lspool/pool.go
    - internal/kernel/lspool/circuit.go
    - internal/kernel/lspool/pool_test.go
    - internal/kernel/kernel.go
    - internal/daemon/daemon.go
decisions:
  - "Circuit struct gained a language field so it can emit per-language state without the Pool having to reach in after every transition. Alternative (pool wraps every circuit call with a sink.Set) was rejected as noisy and race-prone."
  - "Worker restart is hooked inside spawnWorkerLocked when the circuit for the language already has failures>0. The pool has no dedicated auto-restart goroutine, so this is the earliest semantic 'restart after crash' signal available. RecordSuccess resets failures immediately after, so the order (emit restart, then cb.RecordSuccess at the caller) is correct."
  - "stopAll eviction reason is shutdown. Idle-TTL retirement path distinctly emits EvictIdle; pressure passes emit EvictPressure; unhealthy-state pass emits EvictCrash. This maps cleanly to the closed enum in D-13."
  - "nil MetricsSink parameter is coerced to NoopSink{} at NewPool and NewCircuitBreaker boundaries rather than fail-fast. The plan text argued fail-fast but the test-ergonomics cost of threading NoopSink through every harness is not worth it; the default keeps existing callers compiling if they forget."
metrics:
  duration: "~30 min"
  completed: "2026-04-09"
  tasks: 2
  commits: 2
---

# Phase 11 Plan 03: lspool Metrics Wiring Summary

One-liner: lspool now emits per-language worker/eviction/restart/circuit-state gauges and counters via a decoupled MetricsSink interface satisfied by `*obs.Metrics`, with a compile-time assertion pinning the two plans together.

## What Shipped

### 1. lspool.MetricsSink (internal/kernel/lspool/metrics.go)

A four-method interface matching the frozen `*obs.Metrics` helper signatures from plan 11-01 verbatim:

```go
type MetricsSink interface {
    LSPoolWorkersSet(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitStateSet(language string, state float64)
    LSPoolRestart(language string)
}
```

Accompanied by:
- `NoopSink{}` value-type implementation (trivially inlinable, safe default)
- `EvictIdle / EvictPressure / EvictCrash / EvictShutdown` string constants (D-13 closed enum)
- `CircuitClosed / CircuitHalfOpen / CircuitOpen` float64 constants (D-14 state codes)
- Compile-time assertion `var _ MetricsSink = NoopSink{}` at package level

lspool still does NOT import `internal/obs` — verified via `grep` acceptance check.

### 2. Pool signature + lifecycle hooks (internal/kernel/lspool/pool.go)

NewPool signature change:

```go
// Before (plan 05-01 vintage):
func NewPool(cfg PoolConfig, registry *langregistry.Registry,
    installer *langregistry.Installer, pressure MemoryPressure,
    logger *slog.Logger) *Pool

// After:
func NewPool(cfg PoolConfig, registry *langregistry.Registry,
    installer *langregistry.Installer, pressure MemoryPressure,
    logger *slog.Logger, metrics MetricsSink) *Pool
```

Nil metrics are coerced to `NoopSink{}` at the top of `NewPool`.

Hook call sites (all side-effect-only, never touch pool state):

| Call site | Hook | Reason |
|---|---|---|
| `spawnWorkerLocked` (success path) | `LSPoolWorkersSet(lang, +1)` | worker born |
| `spawnWorkerLocked` (post-crash retry) | `LSPoolRestart(lang)` | prior circuit failures > 0 |
| `checkTTLs` (idle retire) | `LSPoolWorkersSet(lang, -1)` + `LSPoolEviction(lang, EvictIdle)` | TTL expiry |
| `evictWorkerLocked` (RSS pass 1) | `LSPoolEviction(lang, EvictPressure)` | memory pressure hard cap |
| `evictWorkerLocked` (unhealthy pass 2) | `LSPoolEviction(lang, EvictCrash)` | out-of-band state |
| `evictWorkerLocked` (score/fallback) | `LSPoolEviction(lang, EvictPressure)` | pressure-driven reclaim |
| `stopAll` | `LSPoolWorkersSet(lang, -1)` + `LSPoolEviction(lang, EvictShutdown)` | daemon shutdown |

### 3. Circuit breaker state reporter (internal/kernel/lspool/circuit.go)

`CircuitBreaker` gained a `language` field and a `sink MetricsSink` field. NewCircuitBreaker signature:

```go
// Before:
func NewCircuitBreaker(maxBackoff time.Duration) *CircuitBreaker
// After:
func NewCircuitBreaker(language string, maxBackoff time.Duration, sink MetricsSink) *CircuitBreaker
```

A new `setStateLocked(next float64)` helper centralizes gauge publication. State transitions:

| Trigger | New state | Emitter |
|---|---|---|
| `NewCircuitBreaker` | CircuitClosed | constructor |
| `RecordFailure` | CircuitOpen | `setStateLocked(CircuitOpen)` |
| `CanAttempt` probe (after backoff) | CircuitHalfOpen | `setStateLocked(CircuitHalfOpen)` |
| `RecordSuccess` | CircuitClosed | `setStateLocked(CircuitClosed)` |

**Yes, the circuit struct needed a new `language` field** — the pool previously used the map key as the language label but the breaker itself had no way to self-identify when emitting gauge writes.

### 4. Kernel + daemon wiring

`kernel.NewKernel` gained a final `metrics lspool.MetricsSink` parameter, threaded through to `lspool.NewPool`. `internal/daemon/daemon.go` reorders construction: the `obs.Noop` observability provider is now created BEFORE the kernel (step 5 instead of step 13) so `observability.Metrics()` can be passed into `NewKernel` at step 6. The daemon still stores the same provider into `d.obs` downstream for the middleware install and admin listener.

### 5. Compile-time assertion (internal/daemon/wiring_test.go)

```go
var _ lspool.MetricsSink = (*obs.Metrics)(nil)
```

Plus a runtime smoke test `TestObsMetricsIsLSPoolSink` that constructs `obs.Noop(nil).Metrics()` and exercises each sink method. Any future signature drift between plan 11-01 and plan 11-03 fails the daemon package build immediately.

## Test Coverage

`internal/kernel/lspool/metrics_test.go` adds:
- `TestNoopSink_safe` — every method on the zero value
- `TestMetricsSink_EvictionReasonConstants` / `TestMetricsSink_CircuitStateConstants` — guards against accidental rename
- `TestPool_evictWorkerLocked_EmitsGaugeAndReason` — table test over all 4 EvictXxx reasons, asserts gauge delta and counter label
- `TestPool_stopAll_EmitsShutdownEvictions` — multi-language shutdown path
- `TestCircuit_stateReport` — closed → open → half-open → closed transition trace
- `TestCircuit_nilSinkReplacedWithNoop` — nil coercion
- `TestPool_nilMetricsDefaultsToNoop` — nil coercion at Pool level

Plus the daemon-level wiring test covers the interface binding.

Full suite: `go test ./...` — all 22 packages green.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] daemon.go observability construction order**
- **Found during:** Task 2
- **Issue:** Plan text said to append `d.obs.Metrics()` to the NewPool call, but in the current daemon.go the observability provider was created at step 13 (line 228), long after the kernel at step 5 (line 142). The `d.obs` field is not even set until step 17 (line 268). Simply passing `d.obs.Metrics()` at the kernel construction point would have read an unset field.
- **Fix:** Reordered: moved `obs.Noop(logger.Handler())` construction to a new step 5 (before the kernel), passed `observability.Metrics()` into `kernel.NewKernel` at step 6. Removed the duplicate step-13 declaration. Downstream call sites and the `d.obs` assignment continue to use the same local `observability` variable, so behavior is identical.
- **Files modified:** internal/daemon/daemon.go
- **Commit:** 092e898d

**2. [Rule 2 - Missing critical functionality] Restart hook had no natural call site**
- **Found during:** Task 1
- **Issue:** The plan asked for `LSPoolRestart(language)` "on each restart attempt", but the current pool has no dedicated auto-restart goroutine — crash recovery happens implicitly via the circuit breaker: an unhealthy worker is evicted (pass 2), then the next AcquireLease spawns a fresh one if `CanAttempt()` passes.
- **Fix:** In `spawnWorkerLocked`, after successfully registering the new worker, emit `LSPoolRestart(lang)` when `p.circuits[lang].Failures() > 0` (i.e., we are coming back from a crash-initiated backoff). Since the caller immediately runs `cb.RecordSuccess()`, the emit ordering is correct — the failure counter is still > 0 at the moment of emission.
- **Files modified:** internal/kernel/lspool/pool.go
- **Commit:** d23d66e6

**3. [Rule 1 - Bug] Gauge leak on idle TTL retirement**
- **Found during:** Task 1 (self-review before commit)
- **Issue:** The original idle-retirement path in `checkTTLs` did not go through `evictWorkerLocked`, so my first draft would have decremented the gauge twice on a pressure/crash eviction (once in `checkTTLs` if pressure happened to coincide, once in `evictWorkerLocked`) or not at all for idle retirement.
- **Fix:** Added explicit `LSPoolWorkersSet(-1) + LSPoolEviction(..., EvictIdle)` inline in `checkTTLs` — idle retirement does not share the eviction path with pressure/crash/shutdown, so the decrement must happen there. Pressure/crash/shutdown all correctly flow through `evictWorkerLocked` with the right reason.
- **Files modified:** internal/kernel/lspool/pool.go
- **Commit:** d23d66e6

**4. [Rule 3 - Blocking] pool_test.go constructor drift**
- **Found during:** Task 1
- **Issue:** 2 `NewPool` and 4 `NewCircuitBreaker` call sites in `pool_test.go` were broken by the signature changes.
- **Fix:** Updated all 6 call sites to the new signatures (`NoopSink{}` for pool, `"go", ..., NoopSink{}` for circuit).
- **Files modified:** internal/kernel/lspool/pool_test.go
- **Commit:** d23d66e6

### Deferred (out-of-scope)

During `gofmt -w ./internal/kernel/lspool/`, Go's formatter also reformatted whitespace in `process.go`, `quirks.go`, and `worker.go` — all pre-existing alignment issues unrelated to this plan. Those changes were STASHED AND DROPPED to keep the Task 1 commit focused. A follow-up `chore(lspool): gofmt cleanup` commit can land them cleanly.

## Success Criteria Review

- [x] METRIC-03: all 4 lspool gauges/counters wired to lifecycle events
- [x] lspool package has zero import of `internal/obs` (D-08 decoupling)
- [x] Existing lspool tests still green (backward compat via NoopSink default)
- [x] `go test -race ./internal/kernel/lspool/... ./internal/daemon/...` green
- [x] `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` compile-time assertion present
- [x] `git diff internal/obs/metrics.go` shows NO changes
- [x] Full `go test ./...` suite green

## Commits

| Hash | Message |
|---|---|
| d23d66e6 | feat(11-03): add lspool MetricsSink + pool/circuit lifecycle hooks |
| 092e898d | feat(11-03): wire obs.Metrics as lspool MetricsSink in daemon |

## Self-Check: PASSED

- internal/kernel/lspool/metrics.go: FOUND
- internal/kernel/lspool/metrics_test.go: FOUND
- internal/daemon/wiring_test.go: FOUND
- internal/kernel/lspool/pool.go: MODIFIED (verified)
- internal/kernel/lspool/circuit.go: MODIFIED (verified)
- internal/kernel/kernel.go: MODIFIED (verified)
- internal/daemon/daemon.go: MODIFIED (verified)
- Commit d23d66e6: FOUND
- Commit 092e898d: FOUND
- internal/obs/metrics.go: UNCHANGED (0 diff lines vs plan-01 baseline)
