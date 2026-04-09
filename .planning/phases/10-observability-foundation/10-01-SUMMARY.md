---
phase: 10-observability-foundation
plan: 01
subsystem: observability
tags: [slog, observability, obs, tracing-stub, koanf, cli]

# Dependency graph
requires:
  - phase: 09-benchmark-harness
    provides: allocation budget measurement infrastructure (≤ +1 alloc/op gate)
provides:
  - internal/obs package with Provider struct + Noop constructor
  - ContextHandler slog wrapper with zero-alloc fast path
  - SpanContext stub + WithSpanContext helper (reserved for Phase 12)
  - ObservabilityConfig schema (AdminAddr, EnablePprof) with koanf bindings
  - --admin-addr CLI flag with guarded precedence override
  - ContextHandler installed in runDaemon and runForwarder before slog.New
affects: [10-02-admin-listener, 10-03-phase-gate, 11-metrics, 12-tracing]

# Tech tracking
tech-stack:
  added: []  # stdlib only — zero new module dependencies
  patterns:
    - "Noop-default provider pattern (never return nil)"
    - "slog.Handler wrapper with early-return fast path"
    - "Guarded koanf override (empty CLI flag does not wipe config)"
    - "Unexported extractor stub pinning Phase 12 migration point"

key-files:
  created:
    - internal/obs/obs.go
    - internal/obs/spancontext.go
    - internal/obs/handler.go
    - internal/obs/obs_test.go
    - internal/obs/handler_test.go
  modified:
    - internal/config/config.go
    - internal/config/defaults.go
    - internal/config/loader_test.go
    - internal/cli/root.go

key-decisions:
  - "Provider as struct not interface — additive evolution across Phases 11/12"
  - "spanContextFromContext unexported — signature reserved for Phase 12 change"
  - "ContextHandler installed in cli/root.go not daemon.New — zero signature churn"
  - "Wrap base handler in BOTH runDaemon and runForwarder for Phase 12 trace propagation parity"
  - "Empty --admin-addr flag is a no-op override (Pitfall #5 guard)"

patterns-established:
  - "Noop-default provider: Noop(inner) returns non-nil Provider with non-nil SlogHandler"
  - "slog hot-path fast path: early-return before any attr construction when extractor returns false"
  - "Typed slog attrs only (slog.String, never slog.Any) to avoid boxing allocations"

requirements-completed: [OBS-01, OBS-02]

# Metrics
duration: ~8min
completed: 2026-04-08
---

# Phase 10 Plan 01: Observability Scaffolding Summary

**Stdlib-only obs package with noop Provider, zero-alloc trace-aware slog ContextHandler, ObservabilityConfig schema, and --admin-addr CLI flag wired through runDaemon and runForwarder.**

## Performance

- **Duration:** ~8 min
- **Tasks:** 2
- **Files created:** 5 (internal/obs/*)
- **Files modified:** 4 (config + cli + loader_test)
- **New go.mod entries:** 0

## Accomplishments

- Delivered OBS-01: `internal/obs` package with `Provider` struct and `Noop(inner slog.Handler) *Provider` constructor.
- Delivered OBS-02: `ContextHandler` slog wrapper with benchmark-verified zero-allocation fast path (`0 B/op, 0 allocs/op` measured).
- Added `ObservabilityConfig{AdminAddr, EnablePprof}` to `SerenaConfig` with koanf defaults (empty/false) — noop by default.
- Registered `--admin-addr` CLI flag with Pitfall #5 guard (empty flag does not overwrite project config value).
- Installed `obs.NewContextHandler` wrapper in both `runDaemon` and `runForwarder` before `slog.New(handler)` for Phase 12 trace propagation parity.
- `spanContextFromContext` Phase 10 stub always returns `false`, pinning the OBS-06 alloc budget and marking the Phase 12 migration point via an unexported symbol.
- Zero new module dependencies: stdlib (`log/slog`, `context`) only. No `go.opentelemetry.io/*`, no `github.com/prometheus/*`.

## Task Commits

1. **Task 1 RED:** `a0b5aa3c` test(10-01): add failing tests for obs package scaffolding
2. **Task 1 GREEN:** `11a073a9` feat(10-01): implement obs package with noop provider and trace-aware handler
3. **Task 2 RED:** `a62fc465` test(10-01): add failing tests for ObservabilityConfig schema
4. **Task 2 GREEN:** `2a499959` feat(10-01): wire ObservabilityConfig, --admin-addr flag, and ContextHandler

## Files Created/Modified

### Created
- `internal/obs/obs.go` — `Provider` struct + `Noop` constructor + `SlogHandler()` accessor.
- `internal/obs/spancontext.go` — `SpanContext` type, `WithSpanContext` exported helper, unexported `spanContextFromContext` Phase 10 stub.
- `internal/obs/handler.go` — `ContextHandler` implementing `slog.Handler` with fast-path early return.
- `internal/obs/obs_test.go` — Noop constructor returns non-nil Provider wrapping inner.
- `internal/obs/handler_test.go` — delegation tests, fast-path pass-through test, benchmark with `b.ReportAllocs()`, Phase 10 stub contract test.

### Modified
- `internal/config/config.go` — added `Observability ObservabilityConfig` field on `SerenaConfig` + new `ObservabilityConfig` type.
- `internal/config/defaults.go` — added `observability.admin_addr=""` and `observability.enable_pprof=false` defaults.
- `internal/config/loader_test.go` — three new tests covering defaults, koanf override, and guarded-override behavior.
- `internal/cli/root.go` — registered `--admin-addr` flag, added guarded override in `runDaemon`, installed `obs.NewContextHandler` wrap in both `runDaemon` and `runForwarder`.

## Decisions Made

- **Provider as concrete struct, not interface:** Phases 11/12 will add Meter/Tracer accessors additively without breaking call sites.
- **`spanContextFromContext` unexported:** Phase 12 may change the signature; pin as package-private so no external code grows a dependency on the current shape.
- **Wrap handler in `cli/root.go`, not inside `daemon.New`:** keeps `daemon.New` signature unchanged across the observability rollout (avoids cascading call-site churn in tests and forwarder).
- **Apply ContextHandler in forwarder too:** even though the forwarder has no span source in Phase 10, mirroring the wrap keeps Phase 12 cross-process trace propagation trivial to enable.
- **Guarded empty-flag override (Pitfall #5):** the single line `if adminAddr != ""` prevents a blank CLI invocation from silently wiping project config — verified by `TestLoad_EmptyAdminAddrOverrideIsNotApplied`.

## Deviations from Plan

None — plan executed exactly as written. TDD cycle followed for both tasks.

## Verification

### Automated

- `go test ./internal/obs/... -count=1` — PASS
- `go test ./internal/obs/... -bench=BenchmarkContextHandler -benchmem -run=^$ -count=1`:
  ```
  BenchmarkContextHandler_Handle-14    7317541    153.2 ns/op    0 B/op    0 allocs/op
  ```
  Zero extra allocations on the hot path — OBS-06 budget trivially satisfied.
- `go test ./internal/config/... ./internal/cli/... ./internal/daemon/... -count=1` — PASS
- `go build ./cmd/serena` — succeeds
- `go vet ./internal/... ./cmd/...` — clean
- `./serena --help | grep admin-addr` — new flag visible:
  ```
  --admin-addr string   Loopback admin listener address (e.g. 127.0.0.1:9090); empty = disabled
  ```

### Acceptance Criteria (from plan)

- [x] `func Noop` in internal/obs/obs.go
- [x] `func NewContextHandler` in internal/obs/handler.go
- [x] `return SpanContext{}, false` in internal/obs/spancontext.go
- [x] `type ContextHandler struct` in internal/obs/handler.go
- [x] `return h.inner.Handle(ctx, r)` fast-path in internal/obs/handler.go
- [x] `b.ReportAllocs` in internal/obs/handler_test.go
- [x] No `opentelemetry` imports in internal/obs/* (only a doc comment noting their absence)
- [x] No `prometheus` imports in internal/obs/*
- [x] `Observability ObservabilityConfig` field in internal/config/config.go
- [x] `type ObservabilityConfig struct` in internal/config/config.go
- [x] `admin_addr` + `enable_pprof` defaults in internal/config/defaults.go
- [x] `admin-addr` flag in internal/cli/root.go
- [x] `obs.NewContextHandler` appears 2x in internal/cli/root.go (runDaemon + runForwarder)
- [x] `if adminAddr != ""` guard in internal/cli/root.go
- [x] `TestLoad_Observability*` tests in internal/config/loader_test.go
- [x] `go build ./cmd/serena` succeeds
- [x] `go test ./internal/config/... ./internal/cli/...` passes

### Success Criteria (from plan)

- [x] OBS-01 delivered: obs package with Noop provider.
- [x] OBS-02 delivered: trace-aware slog handler, fast-path pass-through with benchmark proof of 0 extra allocs.
- [x] ObservabilityConfig schema + defaults merged.
- [x] `--admin-addr` flag registered with guarded precedence.
- [x] ContextHandler installed in both runDaemon and runForwarder before slog.New(handler).
- [x] Zero new go.mod entries.

## Issues Encountered

- Pre-existing `go vet ./...` warning in `borrow/aider/tests/fixtures/languages/go/test.go` (unused `strings` import) — **out of scope** per SCOPE BOUNDARY rule. Not in any file this plan touches; logged but not fixed. Scoped vet on `./internal/... ./cmd/...` is clean.

## Next Phase Readiness

- Plan 10-02 (telemetry.go + admin listener wiring) can now consume `cfg.Observability.AdminAddr`/`EnablePprof` directly and construct a non-fatal errgroup goroutine in `daemon.Run`.
- Plan 10-03 (phase gate + benchstat delta) has a zero-alloc baseline to compare against.
- Phase 12 tracing has a single, well-marked flip point: replace the body of `internal/obs/spancontext.go:spanContextFromContext` with real OTel extraction. The ContextHandler slow path is already wired and awaiting (`r.AddAttrs(slog.String("trace_id", …), slog.String("span_id", …))`).

## Self-Check: PASSED

- `internal/obs/obs.go` — FOUND
- `internal/obs/spancontext.go` — FOUND
- `internal/obs/handler.go` — FOUND
- `internal/obs/obs_test.go` — FOUND
- `internal/obs/handler_test.go` — FOUND
- Commit `a0b5aa3c` — FOUND
- Commit `11a073a9` — FOUND
- Commit `a62fc465` — FOUND
- Commit `2a499959` — FOUND

---
*Phase: 10-observability-foundation*
*Completed: 2026-04-08*
