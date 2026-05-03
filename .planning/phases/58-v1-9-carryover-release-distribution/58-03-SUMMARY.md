---
phase: 58-v1-9-carryover-release-distribution
plan: 03
subsystem: observability/tracing
tags: [tracing, otel, otelgrpc, propagator, forwarder, daemon, REL-06]
requires: [internal/obs/tracing.go (Phase 12 WithTracing), internal/obs/obs.go (NewForTest, TracerProvider, ShutdownTracing)]
provides: ["forwarder env-var-driven TracerProvider", "obs.ClientStatsHandler / obs.ServerStatsHandler centralised propagator wiring", "TestE2ETraceContinuity contract test"]
affects: [internal/forwarder, internal/daemon, internal/obs, test/integration, CONTRIBUTING.md]
tech-stack:
  added: []
  patterns: ["per-handler propagator install (no global)", "centralised stats-handler helper to prevent client/server drift", "tdd RED→GREEN→REFACTOR with assertion-driven failure", "degraded-optional Provider construction"]
key-files:
  created:
    - internal/obs/grpc.go
    - test/integration/trace_continuity_test.go
  modified:
    - internal/forwarder/forwarder.go
    - internal/forwarder/dial.go
    - internal/daemon/daemon.go
    - CONTRIBUTING.md
decisions:
  - "Filter SERVER-kind StreamMCP span explicitly (otelgrpc emits two spans named serena.v1.ForwarderService/StreamMCP — one client-kind, one server-kind)"
  - "Reuse existing daemon.NewWithObsProvider + skill.InitAll harness shape from trace_propagation_test.go rather than extending Options{} in harness.go (no harness API change required)"
  - "Centralise propagator wiring in internal/obs/grpc.go after GREEN passes — drift prevention, not premature abstraction"
  - "ShutdownTracing on a 5s bounded context to flush the SDK batch processor; no-op on the Noop path so the bound is harmless when tracing is disabled"
metrics:
  duration: "~25min"
  completed: 2026-05-03
---

# Phase 58 Plan 03: Forwarder OTel Unification Summary

Unify the `forwarder.tools.call` client root span with the daemon-side
`serena.v1.ForwarderService/StreamMCP` server span by replacing the forwarder's
`obs.Noop` TracerProvider with an env-driven `obs.WithTracing` and wiring W3C
TraceContext propagation per-handler on both gRPC sides — closing the Phase 55
architectural caveat called out in PROJECT.md tech-debt.

## Outcome

Single TraceID now spans `stdio → forwarder → daemon → kernel` end-to-end when
`OTEL_EXPORTER_OTLP_ENDPOINT` is set, enforced by a new integration test
(`TestE2ETraceContinuity`). The no-global OTel rule is preserved: zero
`otel.SetTextMapPropagator` calls in code (only documentation comments
mention the rule).

## Tasks completed

| Task | Commit | Description |
| --- | --- | --- |
| 1 (RED)        | `395e3c98` | `test(58-03)` — add failing trace-continuity integration test |
| 2 (GREEN)      | `42254aca` | `feat(58-03)` — wire env-var-driven TracerProvider in forwarder + propagator option on both handlers |
| 3 (REFACTOR)   | `d9529b3d` | `refactor(58-03)` — factor propagator option into `obs.{Client,Server}StatsHandler` |
| 4 (DOCS)       | `eec294af` | `docs(58-03)` — document `OTEL_EXPORTER_OTLP_ENDPOINT` in CONTRIBUTING.md §Tracing |

## Verification

- `TestE2ETraceContinuity` PASS (RED→GREEN transition observed locally;
  RED log captured at `/tmp/58-03-red.log` showed `trace IDs differ — propagator not wired (Phase 58 D-06 contract)`).
- `go test ./internal/forwarder/... ./internal/daemon/... ./internal/obs/... ./test/integration/ -count=1` — all green.
- `go vet ./internal/forwarder/... ./internal/daemon/... ./internal/obs/... ./test/integration/` — clean (only an unrelated CGO `'TOKEN_COUNT' macro redefined` warning from `internal/treesitter/bindings/swift` predates this plan).
- `grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go'` returns 2 hits, both in documentation comments (`internal/obs/grpc.go:14` and `test/integration/trace_continuity_test.go:9`); zero call-site hits.
- Pitfall 5 held: `internal/forwarder/dial.go` has exactly one `WithStatsHandler(...)` call site (line 72); `internal/daemon/daemon.go` has exactly one `StatsHandler(...)` call inside the `grpc.NewServer(...)` block (line 573).

## Plan output requirements

Per the plan's `<output>` block:

- **Whether harness.go required extension to accept a shared `*obs.Provider`**: NO — the existing `daemon.NewWithObsProvider(cfg, logger, obs.NewForTest(tp))` shape from `trace_propagation_test.go:76` already accepts a Provider; the new test reuses it directly via a small private helper (`startDaemonOverSocket`) rather than threading a new field through the public `Options{}` struct in `harness.go`. The new test still lives under the existing `//go:build integration` tag, matching every other file in `test/integration/`.
- **Integration-test runtime**: `TestE2ETraceContinuity` runs in ~0.2s of test work + ~1s of go-test bootstrap (1.4s total wall on the local M-class macOS box). Well under the `< 600s` full-integration-suite latency target from `58-VALIDATION.md`. The test does not require any external language servers.
- **No-global rule held throughout**: confirmed — zero `otel.SetTextMapPropagator` call sites in the codebase. The two remaining grep hits are explicit documentation reaffirming the rule (in `internal/obs/grpc.go` package doc and the new test's file-level comment). Per-handler propagator option lives only in `obs.ClientStatsHandler` / `obs.ServerStatsHandler` (`internal/obs/grpc.go`).
- **PROJECT.md tech-debt entry "Phase 55 forwarder.tools.call span emits via Noop tracer"**: ready to be moved to "Resolved at v1.10" by the post-phase `update_project_md` step. Phase 58 Plan 03 closes the architectural caveat: forwarder's TracerProvider is now real (env-var-gated), traceparent crosses gRPC, and `TestE2ETraceContinuity` enforces the contract permanently.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] RED state required SpanKind filtering to disambiguate client vs server StreamMCP spans**

- **Found during**: Task 1 (RED).
- **Issue**: The plan's draft test filtered spans by name only (`switch s.Name { case "serena.v1.ForwarderService/StreamMCP": srvSpan = s }`). otelgrpc emits TWO spans named `serena.v1.ForwarderService/StreamMCP`: a client-kind span (created by `NewClientHandler`, child of `forwarder.tools.call` in-process) and a server-kind span (created by `NewServerHandler`, the real cross-wire span). With name-only filtering, the loop overwrote `srvSpan` with whichever appeared last and the assertion accidentally passed (the client-kind span trivially shares TraceID with `forwarder.tools.call`).
- **Fix**: Filter by `s.SpanKind == trace.SpanKindServer` for the server span case; the forwarder span case keeps name-only because there is no name collision for kind=internal.
- **Files modified**: `test/integration/trace_continuity_test.go` (added `go.opentelemetry.io/otel/trace` import for the `SpanKind` constant and switched the case to a guarded `case s.Name == "..." && s.SpanKind == trace.SpanKindServer`).
- **Commit**: `395e3c98` (RED, fixed before commit).

### Plan adaptation notes (not deviations)

- The plan's task-1 skeleton mentioned extending `harness.go`'s `Options{}` to accept a shared `*obs.Provider` "if needed". Investigation showed `daemon.NewWithObsProvider` is already the public seam used by `trace_propagation_test.go` and `trace_shutdown_test.go`; threading a Provider through `Options{}` would have widened the public test API for one caller. Used a private helper instead.
- The plan did not specify which gRPC `obs.ServerStatsHandler` import-path style to use; followed the existing pattern in `daemon.go`'s import block (alphabetised within a single grouped block) and the natural removal of the now-unused `otelgrpc` and `propagation` imports as part of the Task-3 cleanup.

## Self-Check: PASSED

- [x] `internal/obs/grpc.go` exists (`-rw-r--r--`).
- [x] `test/integration/trace_continuity_test.go` exists.
- [x] `internal/forwarder/forwarder.go` calls `obs.WithTracing` and reads `OTEL_EXPORTER_OTLP_ENDPOINT` (verified via grep).
- [x] `internal/forwarder/forwarder.go` defers `ShutdownTracing` on a 5s bounded context.
- [x] `internal/forwarder/dial.go` uses `obs.ClientStatsHandler(tp)` (single call).
- [x] `internal/daemon/daemon.go` uses `obs.ServerStatsHandler(d.obs.TracerProvider())` (single call).
- [x] `propagation` import gone from both dial.go and daemon.go (only present in `internal/obs/grpc.go`).
- [x] No `otel.SetTextMapPropagator` calls in *.go code (matches limited to doc comments).
- [x] `Phase 58 D-06` markers present in forwarder.go, dial.go, daemon.go, obs/grpc.go.
- [x] CONTRIBUTING.md §Tracing exists with all required keywords (`OTEL_EXPORTER_OTLP_ENDPOINT`, `TraceContext`, `trace_continuity_test`).
- [x] All four commit hashes verified in `git log`: `395e3c98`, `42254aca`, `d9529b3d`, `eec294af`.
- [x] `TestE2ETraceContinuity` PASS in GREEN.
- [x] `go test ./internal/forwarder/... ./internal/daemon/... ./internal/obs/... -count=1` PASS.
- [x] `go vet` clean across all four packages (modulo unrelated pre-existing CGO warning in `internal/treesitter/bindings/swift`).
