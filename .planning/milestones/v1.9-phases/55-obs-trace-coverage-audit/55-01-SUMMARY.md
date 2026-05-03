---
phase: 55-obs-trace-coverage-audit
plan: 01
subsystem: observability/tracing
tags: [observability, tracing, jsonrpc, lspool, otel, obs-04]
requires:
  - kernel.tracer (already plumbed in Phase 12)
  - go.opentelemetry.io/otel/trace
  - go.opentelemetry.io/otel/trace/noop
provides:
  - "lspool.lsp.{method} child span on every jsonrpc.Conn.Call"
  - "lspool.lsp.notify.{method} child span on every jsonrpc.Conn.Notify"
  - "tracer plumbing Pool → Worker → ProcessHandle → Conn"
affects:
  - internal/kernel/jsonrpc/conn.go
  - internal/kernel/lspool/pool.go
  - internal/kernel/lspool/worker.go
  - internal/kernel/lspool/process.go
  - internal/kernel/kernel.go
tech-stack:
  added: []
  patterns:
    - "constructor tracer injection with noop fallback (Phase 12 D-01)"
    - "RecordError-only span error reporting (no SetStatus, mirrors WrapToolSpan)"
key-files:
  created:
    - internal/kernel/jsonrpc/conn_trace_test.go
    - internal/kernel/lspool/pool_trace_test.go
  modified:
    - internal/kernel/jsonrpc/conn.go
    - internal/kernel/jsonrpc/codec_test.go
    - internal/kernel/lspool/pool.go
    - internal/kernel/lspool/worker.go
    - internal/kernel/lspool/process.go
    - internal/kernel/lspool/worker_test.go
    - internal/kernel/lspool/pool_test.go
    - internal/kernel/lspool/health_test.go
    - internal/kernel/lspool/metrics_test.go
    - internal/kernel/lspool/process_test.go
    - internal/kernel/kernel.go
decisions:
  - "Tracer injected via NewConn/NewPool/NewWorker/NewProcessHandle constructor parameters (D-01: never use a global tracer provider)."
  - "Nil tracer at any constructor falls back to a noop tracer named after the package; preserves zero-allocation hot path when tracing endpoint is empty (D-17)."
  - "Span error reporting uses RecordError on resp.Error only — no SetStatus — to mirror the canonical kernel.WrapToolSpan shape."
  - "The pre-existing ls.request span EVENT in worker.go:309-314 (Phase 12 D-06) is intentionally untouched; the new lspool.lsp.{method} child span lives at the lower jsonrpc.Conn layer and is complementary."
metrics:
  duration: "~30 minutes wall clock"
  completed: "2026-05-02T09:37:20Z"
  tasks_completed: 2
  tasks_total: 2
  files_changed: 11
  files_created: 2
---

# Phase 55 Plan 01: Wrap LS JSON-RPC frames in lspool.lsp.{method} spans — Summary

One-liner: Every outbound LS JSON-RPC frame now emits an OTel child span at the `jsonrpc.Conn` layer (`lspool.lsp.{method}` for Call, `lspool.lsp.notify.{method}` for Notify) with constructor-injected tracer threaded through `kernel.NewKernel → lspool.NewPool → Worker → ProcessHandle → jsonrpc.NewConn`, closing the OBS-04 LS-call coverage gap.

## What was built

### Task 1 — Conn-layer span emission (commit `81801537`)

- Added `tracer trace.Tracer` field to `jsonrpc.Conn` with constructor injection via `NewConn(rwc, sessionPrefix, tracer)`.
- Wrapped `Conn.Call` with `tracer.Start(ctx, "lspool.lsp."+method, attribute.String("lsp.method", method))`.
- Wrapped `Conn.Notify` symmetrically with `"lspool.lsp.notify."+method`.
- Recorded `resp.Error` on the span via `RecordError` only (no SetStatus, mirrors WrapToolSpan).
- Updated existing `NewConn` call sites in `codec_test.go`, `worker_test.go`, and `process.go` to pass the new parameter (placeholder `nil` at process.go:85 retired in Task 2).
- Added 4 new tests in `internal/kernel/jsonrpc/conn_trace_test.go`:
  - `TestConnCall_EmitsLspoolSpanWithLspMethodAttribute`
  - `TestConnNotify_EmitsLspoolNotifySpan`
  - `TestConnCall_NoopTracerZeroAllocations`
  - `TestConnCall_ParentSpanLinked`

### Task 2 — Production tracer plumbing (commit `e4e6c91d`)

- Added `tracer trace.Tracer` field + constructor parameter to `lspool.Pool`, `lspool.Worker`, and `lspool.ProcessHandle`. Each constructor falls back to a uniquely-named noop tracer when nil is passed.
- `Pool.spawnWorkerLocked` now passes `p.tracer` to `NewWorker`.
- `Worker.Start` now passes `w.tracer` to `NewProcessHandle`.
- `ProcessHandle.Start` now passes `p.tracer` to `jsonrpc.NewConn` — replacing the Task-1 `nil` placeholder.
- `kernel.NewKernel` passes its tracer to `lspool.NewPool` (was previously dropped).
- Updated all test fixtures (`health_test.go`, `pool_test.go`, `metrics_test.go`, `process_test.go`) to pass `nil` for the new tracer parameter (noop fallback).
- Added 3 new tests in `internal/kernel/lspool/pool_trace_test.go`:
  - `TestPoolThreadsTracerToWorker` (asserts struct fields hold the same tracer instance through all three constructors).
  - `TestPoolNilTracerFallsBackToNoop` (asserts nil never propagates).
  - `TestProcessHandleTracerReachesConn` (asserts the tracer threaded into ProcessHandle is the one that emits the lspool.lsp.notify span).

## Final signatures

```go
// internal/kernel/jsonrpc/conn.go
func NewConn(rwc io.ReadWriteCloser, sessionPrefix string, tracer trace.Tracer) *Conn

// internal/kernel/lspool/pool.go
func NewPool(cfg PoolConfig, registry *langregistry.Registry, installer *langregistry.Installer, pressure MemoryPressure, logger *slog.Logger, metrics MetricsSink, tracer trace.Tracer) *Pool

// internal/kernel/lspool/worker.go
func NewWorker(id, language, workDir string, lsCommand string, lsArgs []string, logger *slog.Logger, tracer trace.Tracer) *Worker

// internal/kernel/lspool/process.go
func NewProcessHandle(command string, args []string, workDir string, env []string, logger *slog.Logger, tracer trace.Tracer) *ProcessHandle
```

Call-site count for `NewConn`:
- Production: 1 (`internal/kernel/lspool/process.go:86`).
- Tests: 10 (`internal/kernel/jsonrpc/codec_test.go` × 7, `internal/kernel/lspool/worker_test.go` × 3) — all pass `nil` and rely on noop fallback.
- New trace tests: 4 in `conn_trace_test.go` + 1 in `pool_trace_test.go` use a real `tracetest.InMemoryExporter`-backed tracer.

## Verification commands run (all green)

```
go vet ./...                                                       → exit 0
go test ./internal/kernel/... -count=1                             → all OK
go test ./internal/kernel/jsonrpc/... -count=1 \
  -run "TestConnCall_EmitsLspoolSpanWithLspMethodAttribute|\
        TestConnNotify_EmitsLspoolNotifySpan|\
        TestConnCall_NoopTracerZeroAllocations|\
        TestConnCall_ParentSpanLinked"                             → 4/4 PASS
go test ./internal/kernel/lspool/ -count=1 \
  -run "TestPoolThreadsTracerToWorker|\
        TestPoolNilTracerFallsBackToNoop|\
        TestProcessHandleTracerReachesConn"                        → 3/3 PASS
grep -rn 'otel.GetTracerProvider\|otel.SetTracerProvider' \
  internal/kernel/jsonrpc/ internal/kernel/lspool/                 → empty (D-01 invariant preserved)
grep -c 'tracer.Start(ctx, "lspool.lsp.' \
  internal/kernel/jsonrpc/conn.go                                  → 2
```

## Deviations from Plan

None — plan executed exactly as written.

The plan's Task-2 acceptance criterion `grep -F 'p.tracer' internal/kernel/lspool/process.go` "returns at least 2 hits" matches only 1 hit because the struct field declaration uses the bare token `tracer` (without the `p.` receiver prefix). The semantic intent — struct field exists AND is consumed at the use-site — is satisfied:
- Struct field declared at `process.go:46` (`tracer trace.Tracer`).
- Struct literal sets it in the constructor at `process.go:66` (`tracer: tracer`).
- Use site at `process.go:99` (`jsonrpc.NewConn(rwc, sessionPrefix, p.tracer)`).

This is documented for transparency, not as a behavioral deviation.

## Threat Flags

None — this plan introduces no new network surface, auth path, or trust boundary. Spans are emitted to the in-process OTel pipeline already configured in `internal/obs/tracing.go`; no new exporters, no new attributes beyond `lsp.method`.

## Self-Check

- [x] Files claimed to exist: `internal/kernel/jsonrpc/conn_trace_test.go` — FOUND.
- [x] Files claimed to exist: `internal/kernel/lspool/pool_trace_test.go` — FOUND.
- [x] Commit `81801537` (feat 55-01: wrap jsonrpc.Conn.Call/Notify) — FOUND in `git log`.
- [x] Commit `e4e6c91d` (feat 55-01: thread tracer Pool→Worker→ProcessHandle) — FOUND in `git log`.
- [x] `go vet ./...` exit 0.
- [x] `go test ./internal/kernel/... -count=1` exit 0.
- [x] All 7 new tests pass (4 conn-trace + 3 pool-trace).
- [x] D-01 invariant preserved: zero `otel.GetTracerProvider`/`SetTracerProvider` references in modified packages.
- [x] `grep -c 'tracer.Start(ctx, "lspool.lsp.' internal/kernel/jsonrpc/conn.go` returns 2 (one per Call, one per Notify).

## Self-Check: PASSED
