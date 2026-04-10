---
phase: 12
slug: tracing-end-to-end
status: complete
nyquist_compliant: true
wave_0_complete: true
created: 2026-04-10
---

# Phase 12 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (stdlib) |
| **Config file** | none — stdlib testing |
| **Quick run command** | `go test -count=1 ./internal/obs/... ./internal/mcp/... ./internal/kernel/... ./internal/forwarder/...` |
| **Full suite command** | `go test -count=1 ./internal/... ./test/integration/...` |
| **Estimated runtime** | ~15 seconds |

---

## Sampling Rate

- **After every task commit:** Run `go test -count=1 ./internal/obs/... ./internal/mcp/... ./internal/kernel/... ./internal/forwarder/...`
- **After every plan wave:** Run `go test -count=1 ./internal/... ./test/integration/...`
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** 15 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| 12-01-01 | 01 | 1 | TRACE-04 | unit | `go test -run TestTracingDegradedFallback ./internal/obs/...` | ✅ | ✅ green |
| 12-01-02 | 01 | 1 | TRACE-05 | unit | `go test -run TestDefaultSamplerOff ./internal/obs/...` | ✅ | ✅ green |
| 12-01-03 | 01 | 1 | TRACE-04 | unit | `go test -run TestNoopTracerNonNil ./internal/obs/...` | ✅ | ✅ green |
| 12-01-04 | 01 | 1 | TRACE-05 | unit | `go test -run TestSpanContextFromContext ./internal/obs/...` | ✅ | ✅ green |
| 12-02-01 | 02 | 2 | TRACE-01 | unit | `go test -run TestTelemetryMiddlewareSpan_ToolCallCreatesSpan ./internal/mcp/...` | ✅ | ✅ green |
| 12-02-02 | 02 | 2 | TRACE-02 | unit | `go test -run TestTelemetryMiddlewareSpan_NonToolMethodNoSpan ./internal/mcp/...` | ✅ | ✅ green |
| 12-02-03 | 02 | 2 | TRACE-02 | unit | `go test -run TestTelemetryMiddlewareSpan_ErrorRecorded ./internal/mcp/...` | ✅ | ✅ green |
| 12-02-04 | 02 | 2 | TRACE-02 | unit | `go test -run TestTelemetryMiddlewareSpan_NoopTracerZeroCost ./internal/mcp/...` | ✅ | ✅ green |
| 12-03-01 | 03 | 2 | TRACE-01 | unit | `go test -run TestForwarderRootSpan ./internal/forwarder/...` | ✅ | ✅ green |
| 12-03-02 | 03 | 2 | TRACE-01 | unit | `go test -run TestForwarderRootSpan_NoopTracer ./internal/forwarder/...` | ✅ | ✅ green |
| 12-03-03 | 03 | 2 | TRACE-01 | unit | `go test -run TestIsToolsCall ./internal/forwarder/...` | ✅ | ✅ green |
| 12-04-01 | 04 | 3 | TRACE-03 | unit | `go test -run TestWrapToolSpan_SpanCreated ./internal/kernel/...` | ✅ | ✅ green |
| 12-04-02 | 04 | 3 | TRACE-03 | unit | `go test -run TestWrapToolSpan_ErrorRecorded ./internal/kernel/...` | ✅ | ✅ green |
| 12-04-03 | 04 | 3 | TRACE-03 | unit | `go test -run TestWrapToolSpan_ParentSpanLinked ./internal/kernel/...` | ✅ | ✅ green |
| 12-04-04 | 04 | 3 | TRACE-03 | unit | `go test -run TestWrapToolSpan_NoopDoesNotPanic ./internal/kernel/...` | ✅ | ✅ green |
| 12-05-01 | 05 | 4 | TRACE-01 | integration | `go test -run TestTraceparentPropagation ./test/integration/...` | ✅ | ✅ green |
| 12-05-02 | 05 | 4 | TRACE-01 | integration | `go test -run TestTraceparentPropagation_TracingEnabled ./test/integration/...` | ✅ | ✅ green |
| 12-05-03 | 05 | 4 | TRACE-01 | integration | `go test -run TestForwarderSpanCreation ./test/integration/...` | ✅ | ✅ green |
| 12-05-04 | 05 | 4 | TRACE-05 | integration | `go test -run TestTraceFlushOnShutdown ./test/integration/...` | ✅ | ✅ green |
| 12-05-05 | 05 | 4 | TRACE-05 | bench | `go test -bench BenchmarkTracingOffPath -benchmem ./test/bench/...` | ✅ | ✅ green |
| 12-05-06 | 05 | 4 | TRACE-05 | bench | `go test -bench BenchmarkTracingOnPath -benchmem ./test/bench/...` | ✅ | ✅ green |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

Existing infrastructure covers all phase requirements. No Wave 0 stubs needed.

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Forwarder→daemon traceparent across real Unix gRPC socket | TRACE-01 | InMemoryTransports lack gRPC metadata; E2E socket test would require real daemon process | Start daemon, send tool call via forwarder, verify 3-span tree in OTLP collector |

---

## Validation Sign-Off

- [x] All tasks have automated verify commands
- [x] Sampling continuity: no 3 consecutive tasks without automated verify
- [x] Wave 0 covers all MISSING references (none missing)
- [x] No watch-mode flags
- [x] Feedback latency < 15s
- [x] `nyquist_compliant: true` set in frontmatter

**Approval:** approved 2026-04-10

---

## Validation Audit 2026-04-10

| Metric | Count |
|--------|-------|
| Gaps found | 0 |
| Resolved | 0 |
| Escalated | 0 |
| Manual-only | 1 |

*Reconstructed from artifacts (State B). All 5 requirements (TRACE-01..05) have automated test coverage across 21 test functions in 8 test files. All tests pass.*
