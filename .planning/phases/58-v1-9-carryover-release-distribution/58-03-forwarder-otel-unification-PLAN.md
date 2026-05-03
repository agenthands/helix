---
phase: 58-v1-9-carryover-release-distribution
plan: 03
type: tdd
wave: 1
depends_on: []
files_modified:
  - internal/forwarder/forwarder.go
  - internal/forwarder/dial.go
  - internal/daemon/daemon.go
  - test/integration/trace_continuity_test.go
  - CONTRIBUTING.md
autonomous: true
requirements: [REL-06]
must_haves:
  truths:
    - "Forwarder constructs a real env-var-driven TracerProvider via obs.WithTracing (replacing obs.Noop) when OTEL_EXPORTER_OTLP_ENDPOINT is set"
    - "Forwarder TracerProvider falls back to noop when OTEL_EXPORTER_OTLP_ENDPOINT is unset (degraded-optional, no panic)"
    - "Both client (dial.go) and server (daemon.go) gRPC handlers carry otelgrpc.WithPropagators(propagation.TraceContext{}) explicitly — NO global otel.SetTextMapPropagator call exists"
    - "An integration test asserts that forwarder.tools.call and serena.v1.ForwarderService/StreamMCP share the same TraceID via in-memory exporter"
    - "CONTRIBUTING.md §Tracing documents the OTEL_EXPORTER_OTLP_ENDPOINT env-var path"
  artifacts:
    - path: "internal/forwarder/forwarder.go"
      provides: "WithTracing-backed Provider construction; degraded-optional fallback to Noop"
      contains: "obs.WithTracing"
    - path: "internal/forwarder/dial.go"
      provides: "otelgrpc.NewClientHandler with WithPropagators(propagation.TraceContext{})"
      contains: "otelgrpc.WithPropagators"
    - path: "internal/daemon/daemon.go"
      provides: "otelgrpc.NewServerHandler with WithPropagators(propagation.TraceContext{})"
      contains: "otelgrpc.WithPropagators"
    - path: "test/integration/trace_continuity_test.go"
      provides: "TestE2ETraceContinuity asserting single TraceID end-to-end"
      contains: "TestE2ETraceContinuity"
    - path: "CONTRIBUTING.md"
      provides: "§Tracing section documenting OTEL_EXPORTER_OTLP_ENDPOINT"
      contains: "OTEL_EXPORTER_OTLP_ENDPOINT"
  key_links:
    - from: "internal/forwarder/forwarder.go"
      to: "internal/obs/tracing.go::WithTracing"
      via: "Provider construction"
      pattern: "obs\\.WithTracing"
    - from: "internal/forwarder/dial.go"
      to: "go.opentelemetry.io/otel/propagation"
      via: "TraceContext propagator option"
      pattern: "propagation\\.TraceContext\\{\\}"
    - from: "internal/daemon/daemon.go"
      to: "go.opentelemetry.io/otel/propagation"
      via: "TraceContext propagator option"
      pattern: "propagation\\.TraceContext\\{\\}"
    - from: "test/integration/trace_continuity_test.go"
      to: "tracetest.InMemoryExporter"
      via: "in-memory span collection"
      pattern: "tracetest\\.NewInMemoryExporter"
---

<objective>
Unify the forwarder.tools.call OTel client span with the daemon-side gRPC server span by:
1. Replacing the forwarder's `obs.Noop(...)` TracerProvider with an env-var-driven real provider (`obs.WithTracing`) reading `OTEL_EXPORTER_OTLP_ENDPOINT`.
2. Adding `otelgrpc.WithPropagators(propagation.TraceContext{})` to BOTH the client handler at `internal/forwarder/dial.go:70-72` AND the server handler at `internal/daemon/daemon.go:572-576` — per-handler, NOT via global `otel.SetTextMapPropagator` (no-global-OTel rule per PATTERNS).
3. Writing an integration test (`test/integration/trace_continuity_test.go`) using `tracetest.InMemoryExporter` to assert a single TraceID covers stdio → forwarder → daemon → kernel.
4. Documenting the `OTEL_EXPORTER_OTLP_ENDPOINT` env-var path in `CONTRIBUTING.md` §Tracing.

This plan is independent of Plan 01 / Plan 02 (no file overlap); runs in Wave 1.

**Note:** RESEARCH found that `otelgrpc.NewClientHandler` is ALREADY installed at `internal/forwarder/dial.go:70-72` and `otelgrpc.NewServerHandler` at `internal/daemon/daemon.go:572-576`. The actual gaps are (a) the forwarder's TracerProvider is `obs.Noop(...)` so traces are no-op, and (b) no `TextMapPropagator` is wired so `traceparent` metadata is never injected/extracted. Do NOT propose installing handlers that already exist.

Purpose: REL-06 closure (D-06 forwarder span unification). Closes the architectural caveat from PROJECT.md tech-debt section ("Phase 55 forwarder.tools.call span emits via Noop tracer").
Output: A real TracerProvider in the forwarder, propagator wiring on both gRPC handlers, an end-to-end trace-continuity test, and §Tracing documentation.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md
@internal/forwarder/forwarder.go
@internal/forwarder/dial.go
@internal/forwarder/forwarder_test.go
@internal/daemon/daemon.go
@internal/obs/obs.go
@internal/obs/tracing.go
@test/integration/harness.go
@CONTRIBUTING.md

<interfaces>
<!-- Existing OTel surface (from PATTERNS) — being extended, not replaced. -->

From internal/obs/tracing.go (existing — used as the new construction path):
```go
// WithTracing constructs a Provider with a real SDK TracerProvider backed by
// an OTLP/gRPC exporter. If cfg.Endpoint is empty, the caller should use
// Noop() instead — this function is intended for the non-empty endpoint path.
func WithTracing(inner slog.Handler, cfg TracingConfig, logger *slog.Logger) *Provider
```
TracingConfig fields used: `Endpoint string`, `ServiceName string`, `SampleRatio float64`.

From internal/obs/obs.go (existing — invariants to preserve):
```go
func (p *Provider) TracerProvider() trace.TracerProvider // never returns nil
func (p *Provider) ShutdownTracing(ctx context.Context) error
func NewForTest(tp trace.TracerProvider) *Provider // for test-time injection
```

From internal/forwarder/dial.go:70-72 (existing — extending, not replacing):
```go
grpc.WithStatsHandler(otelgrpc.NewClientHandler(
    otelgrpc.WithTracerProvider(tp),
)),
```

From internal/daemon/daemon.go:572-576 (existing — extending, not replacing):
```go
d.grpcServer = grpc.NewServer(
    grpc.StatsHandler(otelgrpc.NewServerHandler(
        otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
    )),
)
```

NEW import required in BOTH dial.go and daemon.go:
```go
"go.opentelemetry.io/otel/propagation"
```

NO global propagator install: the rule from `internal/obs/tracing.go:1-13` ("No call to otel.SetTracerProvider anywhere in this package (D-01)") extends to the propagator. Use per-handler `otelgrpc.WithPropagators(propagation.TraceContext{})` instead.

Pattern from internal/forwarder/forwarder_test.go:111-141 (existing — extends to integration test):
```go
exporter := tracetest.NewInMemoryExporter()
tp := sdktrace.NewTracerProvider(
    sdktrace.WithSampler(sdktrace.AlwaysSample()),
    sdktrace.WithSyncer(exporter),
)
defer func() { _ = tp.Shutdown(context.Background()) }()
```
</interfaces>
</context>

<tasks>

<task type="auto" tdd="true">
  <name>Task 1: RED — write failing trace-continuity integration test</name>
  <files>test/integration/trace_continuity_test.go</files>
  <read_first>
    - test/integration/harness.go (full file — daemon spin-up `Options{}` struct)
    - internal/forwarder/forwarder_test.go (lines 111-141 — TestForwarderRootSpan in-memory exporter pattern)
    - internal/obs/obs.go (NewForTest signature, TracerProvider invariants)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`test/integration/trace_continuity_test.go`" (combined harness+tracetest pattern)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md row 58-P3 (`go test ./test/integration/ -run TestE2ETraceContinuity -count=1`)
  </read_first>
  <behavior>
    - TestE2ETraceContinuity:
      - Spins up the daemon via `harness.go`'s `Options{}` builder.
      - Injects an in-memory TracerProvider into BOTH the daemon's `obs.Provider` AND the forwarder's `obs.Provider` via `obs.NewForTest(tp)`.
      - Sends a single tools/call request through the forwarder over gRPC to the daemon.
      - Calls `tp.ForceFlush(ctx)` to drain spans.
      - Reads `exporter.GetSpans()` and locates BOTH `forwarder.tools.call` (client) and `serena.v1.ForwarderService/StreamMCP` (server) by name.
      - Asserts both spans exist (`require.NotZero(...)` on each TraceID).
      - Asserts `fwdSpan.SpanContext.TraceID() == srvSpan.SpanContext.TraceID()` (the contract — propagator wired).
    - The test MUST fail at this stage because the propagator wiring (Task 2) has not landed yet. Failure mode: the two spans live in different TraceIDs (or the server span never propagates context, so the assertion `Equal(fwdSpan.TraceID, srvSpan.TraceID)` fails).
  </behavior>
  <action>
    1. Create `test/integration/trace_continuity_test.go` (build tag-gated only if existing integration tests use one — check `harness.go` and match its build-tag posture).
    2. Skeleton:
       ```go
       package integration_test

       import (
           "context"
           "testing"

           "github.com/stretchr/testify/require"
           "go.opentelemetry.io/otel/sdk/trace/tracetest"
           sdktrace "go.opentelemetry.io/otel/sdk/trace"

           "github.com/agenthands/helix/internal/daemon"
           "github.com/agenthands/helix/internal/obs"
           // Blank imports trigger skill registration via init() (Caddy-style),
           // mirroring the pattern in test/integration/harness.go.
           _ "github.com/agenthands/helix/internal/kernel/diag"
           _ "github.com/agenthands/helix/internal/kernel/edit"
           _ "github.com/agenthands/helix/internal/kernel/fileops"
           _ "github.com/agenthands/helix/internal/kernel/symbols"
           _ "github.com/agenthands/helix/internal/profile"
           _ "github.com/agenthands/helix/internal/skill/memory"
           _ "github.com/agenthands/helix/internal/skill/workflow"
       )

       func TestE2ETraceContinuity(t *testing.T) {
           ctx := context.Background()

           exporter := tracetest.NewInMemoryExporter()
           tp := sdktrace.NewTracerProvider(
               sdktrace.WithSampler(sdktrace.AlwaysSample()),
               sdktrace.WithSyncer(exporter),
           )
           defer func() { _ = tp.Shutdown(ctx) }()

           // Inject the same in-memory TracerProvider into BOTH the daemon
           // and forwarder Providers so spans from both sides land in the same
           // exporter. obs.NewForTest exists for this purpose.
           sharedProv := obs.NewForTest(tp)

           // Spin up daemon with shared Provider (use harness.Options{}; pass
           // sharedProv via the harness API — extend harness if missing).
           // ... call out to daemon.Start with Options.Obs = sharedProv ...
           // ... start forwarder with same sharedProv ...
           // ... issue a single tools/call request through the forwarder ...

           _ = tp.ForceFlush(ctx)

           spans := exporter.GetSpans()
           var fwdSpan, srvSpan tracetest.SpanStub
           for _, s := range spans {
               switch s.Name {
               case "forwarder.tools.call":
                   fwdSpan = s
               case "serena.v1.ForwarderService/StreamMCP":
                   srvSpan = s
               }
           }
           require.NotZero(t, fwdSpan.SpanContext.TraceID(), "forwarder span missing — forwarder TracerProvider may still be Noop")
           require.NotZero(t, srvSpan.SpanContext.TraceID(), "server span missing — daemon handler not wired")
           require.Equal(t, fwdSpan.SpanContext.TraceID(), srvSpan.SpanContext.TraceID(), "trace IDs differ — propagator not wired (Task 2 incomplete)")
       }
       ```
    3. **Adapt the harness API as needed.** If `harness.go`'s `Options{}` does not currently accept an `*obs.Provider`, extend it minimally. This extension is a Wave-0 prerequisite within this plan — perform it as part of Task 1 if required. Document the extension in the SUMMARY.
    4. Run the test:
       ```bash
       go test ./test/integration/ -run TestE2ETraceContinuity -count=1 -v 2>&1 | tee /tmp/58-03-red.log
       ```
    5. **Confirm RED.** Either the assertion `require.Equal(...)` fails (different trace IDs) OR `forwarder.tools.call` is missing entirely (because forwarder is using Noop). Both are acceptable RED states; record the actual failure in /tmp/58-03-red.log.
    6. Commit shape: `test(58-03): add failing trace-continuity integration test (RED)`.
  </action>
  <verify>
    <automated>! go test ./test/integration/ -run TestE2ETraceContinuity -count=1 2&gt;&amp;1 | tee /tmp/58-03-red.log; grep -qE '(FAIL|trace IDs differ|forwarder span missing)' /tmp/58-03-red.log</automated>
  </verify>
  <acceptance_criteria>
    - `test/integration/trace_continuity_test.go` exists
    - File contains `TestE2ETraceContinuity` and uses `tracetest.NewInMemoryExporter`
    - Test asserts equality of TraceIDs between `forwarder.tools.call` and `serena.v1.ForwarderService/StreamMCP`
    - Run is RED (`grep -qE 'FAIL|trace IDs differ|forwarder span missing' /tmp/58-03-red.log`)
    - No global `otel.SetTextMapPropagator` introduced anywhere
    - If harness.go was extended to accept a shared `*obs.Provider`, the change is minimal (no new tests broken: `go test ./test/integration/...` still compiles)
  </acceptance_criteria>
  <done>
    Integration test exists, harness extension (if any) committed, RED state recorded.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 2: GREEN — wire env-var-driven TracerProvider in forwarder + add TraceContext propagator to both gRPC handlers</name>
  <files>internal/forwarder/forwarder.go, internal/forwarder/dial.go, internal/daemon/daemon.go</files>
  <read_first>
    - internal/forwarder/forwarder.go (current — line 25 `obs.Noop(logger.Handler())`; defer order at line 31 `defer conn.Close()`)
    - internal/forwarder/dial.go lines 66-72 (Phase-12 comment + existing handler installation)
    - internal/daemon/daemon.go lines 569-576 (D-12 comment + server handler installation)
    - internal/obs/tracing.go lines 75-93 (`WithTracing` constructor) and lines 1-13 (no-global rule)
    - internal/obs/obs.go lines 73-95 (`TracerProvider()` non-nil invariant; `ShutdownTracing` shape)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/forwarder/forwarder.go`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/forwarder/dial.go`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`internal/daemon/daemon.go`"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"Per-handler propagator install (no global state)"
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"Phase-/decision-tagged inline comments"
  </read_first>
  <behavior>
    - The forwarder reads `OTEL_EXPORTER_OTLP_ENDPOINT`. If non-empty, constructs a real Provider via `obs.WithTracing`. If empty, falls back to `obs.Noop` (degraded-optional D-09 from `obs/tracing.go:1-13`).
    - Both gRPC handlers (client in dial.go, server in daemon.go) carry `otelgrpc.WithPropagators(propagation.TraceContext{})`.
    - No `otel.SetTextMapPropagator` call anywhere (no-global rule).
    - The integration test from Task 1 turns GREEN.
  </behavior>
  <action>
    1. **`internal/forwarder/forwarder.go`** — replace the line `fwdProvider := obs.Noop(logger.Handler())` (currently around line 25) with:
       ```go
       // Phase 58 D-06: real TracerProvider when OTEL_EXPORTER_OTLP_ENDPOINT
       // is set; falls back to Noop otherwise (degraded-optional per
       // internal/obs/tracing.go:1-13 D-09). Construction never panics.
       endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
       fwdProvider := obs.WithTracing(logger.Handler(), obs.TracingConfig{
           Endpoint:    endpoint,
           ServiceName: "helix-forwarder",
           SampleRatio: 1.0,
       }, logger)
       ```
       Add `"os"` to imports if not already present.
       Add `defer fwdProvider.ShutdownTracing(context.Background())` near the existing `defer conn.Close()` at line ~31 so a real SDK provider flushes pending spans on exit (Noop has nothing to flush; safe in both paths).
    2. **`internal/forwarder/dial.go`** — extend the existing handler installation at lines 70-72:
       ```go
       grpc.WithStatsHandler(otelgrpc.NewClientHandler(
           otelgrpc.WithTracerProvider(tp),
           otelgrpc.WithPropagators(propagation.TraceContext{}),
       )),
       ```
       Add new import `"go.opentelemetry.io/otel/propagation"`. Update the inline comment at lines 66-69 to reference Phase 58 D-06 (per PATTERNS §"Phase-/decision-tagged inline comments"). Do NOT add a second `WithStatsHandler` line (Pitfall 5).
    3. **`internal/daemon/daemon.go`** — extend the existing handler installation at lines 572-576:
       ```go
       d.grpcServer = grpc.NewServer(
           grpc.StatsHandler(otelgrpc.NewServerHandler(
               otelgrpc.WithTracerProvider(d.obs.TracerProvider()),
               otelgrpc.WithPropagators(propagation.TraceContext{}),
           )),
       )
       ```
       Add new import `"go.opentelemetry.io/otel/propagation"`. Update the inline comment at lines 569-571 to note that the propagator option keeps the no-global rule intact (Phase 58 D-06).
    4. **No global propagator install anywhere.** Verify: `grep -RIn 'otel\.SetTextMapPropagator' .` returns zero hits across the repo.
    5. Run the integration test:
       ```bash
       go test ./test/integration/ -run TestE2ETraceContinuity -count=1 -v 2>&1 | tee /tmp/58-03-green.log
       ```
       Test MUST pass.
    6. Run the broader test net:
       ```bash
       go test ./internal/forwarder/... ./internal/daemon/... ./test/integration/ -count=1
       go vet ./internal/forwarder/... ./internal/daemon/... ./test/integration/
       ```
    7. Commit shape: `feat(58-03): unify forwarder.tools.call span via real TracerProvider + TraceContext propagator (GREEN)`.
  </action>
  <verify>
    <automated>go test ./test/integration/ -run TestE2ETraceContinuity -count=1 2&gt;&amp;1 | tee /tmp/58-03-green.log &amp;&amp; ! grep -q 'FAIL' /tmp/58-03-green.log &amp;&amp; go test ./internal/forwarder/... ./internal/daemon/... -count=1 &amp;&amp; go vet ./internal/forwarder/... ./internal/daemon/... ./test/integration/ &amp;&amp; ! grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go' &amp;&amp; grep -q 'propagation\.TraceContext{}' internal/forwarder/dial.go &amp;&amp; grep -q 'propagation\.TraceContext{}' internal/daemon/daemon.go &amp;&amp; grep -q 'obs\.WithTracing' internal/forwarder/forwarder.go</automated>
  </verify>
  <acceptance_criteria>
    - `TestE2ETraceContinuity` passes
    - `go test ./internal/forwarder/... ./internal/daemon/... ./test/integration/ -count=1` all green
    - `go vet` clean across all three packages
    - `internal/forwarder/forwarder.go` calls `obs.WithTracing` (`grep -c 'obs.WithTracing' internal/forwarder/forwarder.go` >= 1) and reads `OTEL_EXPORTER_OTLP_ENDPOINT` (`grep -c 'OTEL_EXPORTER_OTLP_ENDPOINT' internal/forwarder/forwarder.go` >= 1)
    - `internal/forwarder/forwarder.go` calls `ShutdownTracing` via defer (`grep -c 'ShutdownTracing' internal/forwarder/forwarder.go` >= 1)
    - `internal/forwarder/dial.go` carries `propagation.TraceContext{}` (`grep -c 'propagation.TraceContext' internal/forwarder/dial.go` >= 1)
    - `internal/daemon/daemon.go` carries `propagation.TraceContext{}` (`grep -c 'propagation.TraceContext' internal/daemon/daemon.go` >= 1)
    - **No-global rule held:** `grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go'` returns zero hits (entire repo)
    - **Pitfall-5 held:** exactly one `WithStatsHandler` call in dial.go (`grep -c 'WithStatsHandler' internal/forwarder/dial.go` returns 1) and exactly one `StatsHandler` in daemon.go's grpc.NewServer block
    - Inline comments reference `Phase 58 D-06` (`grep -c 'Phase 58 D-06' internal/forwarder/dial.go internal/daemon/daemon.go internal/forwarder/forwarder.go` >= 3)
  </acceptance_criteria>
  <done>
    Forwarder TracerProvider is env-var-driven; both handlers carry propagator option per-handler; integration test GREEN; no global OTel state introduced.
  </done>
</task>

<task type="auto" tdd="true">
  <name>Task 3: REFACTOR — factor propagator option into a shared obs/grpc.go helper</name>
  <files>internal/obs/grpc.go, internal/forwarder/dial.go, internal/daemon/daemon.go</files>
  <read_first>
    - internal/obs/obs.go (current package shape)
    - internal/obs/tracing.go (no-global D-01 rule preserved)
    - internal/forwarder/dial.go (post-Task-2 state)
    - internal/daemon/daemon.go (post-Task-2 state)
  </read_first>
  <behavior>
    - A new `internal/obs/grpc.go` exposes two helper constructors: `ClientStatsHandler(tp trace.TracerProvider) stats.Handler` and `ServerStatsHandler(tp trace.TracerProvider) stats.Handler`. Both internally call `otelgrpc.NewClientHandler` / `NewServerHandler` with the standard option set (TracerProvider + TraceContext propagator).
    - dial.go and daemon.go switch to the helpers — the per-handler propagator option is no longer hand-written at each call site.
    - All tests still pass; the integration test still asserts the same TraceID-equality contract.
  </behavior>
  <action>
    1. Create `internal/obs/grpc.go` with the two helpers:
       ```go
       package obs

       import (
           "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
           "go.opentelemetry.io/otel/propagation"
           "go.opentelemetry.io/otel/trace"
           "google.golang.org/grpc/stats"
       )

       // ClientStatsHandler returns the standard otelgrpc client handler with
       // TraceContext propagator pre-wired. Phase 58 D-06: keeps propagator
       // wiring in one place so dial sites cannot drift from server sites.
       func ClientStatsHandler(tp trace.TracerProvider) stats.Handler {
           return otelgrpc.NewClientHandler(
               otelgrpc.WithTracerProvider(tp),
               otelgrpc.WithPropagators(propagation.TraceContext{}),
           )
       }

       // ServerStatsHandler is the symmetric server-side helper.
       func ServerStatsHandler(tp trace.TracerProvider) stats.Handler {
           return otelgrpc.NewServerHandler(
               otelgrpc.WithTracerProvider(tp),
               otelgrpc.WithPropagators(propagation.TraceContext{}),
           )
       }
       ```
    2. Update `internal/forwarder/dial.go` to use `obs.ClientStatsHandler(tp)`. The `propagation` import becomes unnecessary in dial.go and should be removed (avoid unused-import lint failures).
    3. Update `internal/daemon/daemon.go` to use `obs.ServerStatsHandler(d.obs.TracerProvider())`. Remove the now-unused `propagation` import in daemon.go.
    4. Run all relevant tests:
       ```bash
       go test ./internal/obs/... ./internal/forwarder/... ./internal/daemon/... ./test/integration/ -count=1
       go vet ./internal/obs/... ./internal/forwarder/... ./internal/daemon/... ./test/integration/
       ```
    5. Commit shape: `refactor(58-03): factor propagator option into obs.{Client,Server}StatsHandler`.
  </action>
  <verify>
    <automated>test -f internal/obs/grpc.go &amp;&amp; go test ./internal/obs/... ./internal/forwarder/... ./internal/daemon/... ./test/integration/ -count=1 &amp;&amp; go vet ./internal/obs/... ./internal/forwarder/... ./internal/daemon/... ./test/integration/ &amp;&amp; grep -q 'obs.ClientStatsHandler' internal/forwarder/dial.go &amp;&amp; grep -q 'obs.ServerStatsHandler' internal/daemon/daemon.go &amp;&amp; ! grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go'</automated>
  </verify>
  <acceptance_criteria>
    - `internal/obs/grpc.go` exists and exposes both helpers
    - dial.go uses `obs.ClientStatsHandler(tp)` exactly once
    - daemon.go uses `obs.ServerStatsHandler(...)` exactly once
    - `propagation` import is gone from both dial.go and daemon.go
    - All previously-green tests remain green (`go test ./internal/obs/... ./internal/forwarder/... ./internal/daemon/... ./test/integration/ -count=1`)
    - `go vet` clean
    - No-global rule still held (`grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go'` returns zero hits)
  </acceptance_criteria>
  <done>
    Propagator wiring centralized; tests still green; no-global rule intact.
  </done>
</task>

<task type="auto">
  <name>Task 4: Document OTEL_EXPORTER_OTLP_ENDPOINT in CONTRIBUTING.md §Tracing</name>
  <files>CONTRIBUTING.md</files>
  <read_first>
    - CONTRIBUTING.md (current — full file; identify whether a §Tracing section exists or needs creation)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md Q-5 default (env-var path; document in CONTRIBUTING.md §Tracing)
  </read_first>
  <action>
    Add a `## Tracing` H2 section (or extend existing if present) to `CONTRIBUTING.md`. Required content:

    ```markdown
    ## Tracing

    Helix emits OpenTelemetry spans across the stdio→forwarder→daemon→kernel call
    chain. By default both the forwarder and the daemon use a no-op TracerProvider
    (no spans exported, no overhead).

    To enable real trace export, set `OTEL_EXPORTER_OTLP_ENDPOINT` to an OTLP/gRPC
    collector URL (for example `http://localhost:4317`):

    ```sh
    export OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4317
    helix daemon
    ```

    Trace propagation between the forwarder client and the daemon gRPC server
    uses W3C TraceContext via `otelgrpc`'s `WithPropagators(propagation.TraceContext{})`
    option, wired per-handler in `internal/obs/grpc.go`. There is no global
    `otel.SetTextMapPropagator` call anywhere in the codebase — the no-global
    OTel rule (see `internal/obs/tracing.go` D-01) keeps tracing setup explicit.

    The end-to-end trace-continuity contract is enforced by
    `test/integration/trace_continuity_test.go`: a single TraceID must cover
    the `forwarder.tools.call` client span and the
    `serena.v1.ForwarderService/StreamMCP` server span. If you change handler
    wiring, make sure that test still passes.
    ```

    Place the new H2 in a sensible location (after §Releasing or §Architecture, depending on the existing TOC). Use sh code fences (preserving the project convention from PATTERNS).

    Verify with grep:
    ```bash
    grep -q '## Tracing' CONTRIBUTING.md
    grep -q 'OTEL_EXPORTER_OTLP_ENDPOINT' CONTRIBUTING.md
    grep -q 'TraceContext' CONTRIBUTING.md
    ```
  </action>
  <verify>
    <automated>grep -q '## Tracing' CONTRIBUTING.md &amp;&amp; grep -q 'OTEL_EXPORTER_OTLP_ENDPOINT' CONTRIBUTING.md &amp;&amp; grep -q 'TraceContext' CONTRIBUTING.md &amp;&amp; grep -q 'trace_continuity_test' CONTRIBUTING.md</automated>
  </verify>
  <acceptance_criteria>
    - `CONTRIBUTING.md` contains an `## Tracing` H2 anchor
    - The section mentions `OTEL_EXPORTER_OTLP_ENDPOINT` exactly as written (no typos)
    - The section mentions `TraceContext` and the no-global rule
    - The section references `trace_continuity_test.go` so future contributors find the contract test
  </acceptance_criteria>
  <done>
    Tracing section is in CONTRIBUTING.md with all required keywords.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Forwarder process → daemon process | Trace context (`traceparent`) crosses gRPC metadata between separate processes |
| OTLP exporter → external collector | Optional outbound network when `OTEL_EXPORTER_OTLP_ENDPOINT` is set |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-58-05 | Tampering | Attacker injects malicious traceparent into forwarder gRPC metadata | accept (low impact) | Trace context is logging/metrics metadata only — never used for authz, routing, or business logic. The W3C TraceContext format restricts the field to fixed-shape hex strings; otelgrpc's parser silently rejects malformed values. No code path consumes traceparent for security decisions. Documented in CONTRIBUTING.md §Tracing. |
| T-58-05-aux | Information Disclosure | OTLP collector endpoint leaks span content | mitigate | OTEL_EXPORTER_OTLP_ENDPOINT is opt-in (default empty → noop). When set, the operator chose the destination. SampleRatio default of 1.0 is documented; operators expecting privacy concerns can override. |
| T-58-05-bug | Denial of Service | Misconfigured exporter causes forwarder startup failure | mitigate | `obs.WithTracing` follows degraded-optional pattern (D-09 from `internal/obs/tracing.go`): construction never panics. If exporter init fails, the forwarder logs a warning and falls back to noop. |
</threat_model>

<verification>
- `TestE2ETraceContinuity` is green
- `go test ./internal/forwarder/... ./internal/daemon/... ./internal/obs/... ./test/integration/` all green
- `go vet` clean across all four packages
- No `otel.SetTextMapPropagator` call anywhere in the repo (`grep -RIn 'otel\.SetTextMapPropagator' . --include='*.go'` returns zero hits)
- Both gRPC handlers carry the propagator option (via `obs.ClientStatsHandler` / `obs.ServerStatsHandler`)
- Forwarder reads `OTEL_EXPORTER_OTLP_ENDPOINT` and degrades to noop when empty
- CONTRIBUTING.md §Tracing exists with all required content
</verification>

<success_criteria>
- The PROJECT.md tech-debt entry "Phase 55 forwarder.tools.call span emits via Noop tracer" is closed (the post-phase update_project_md step moves it to Resolved at v1.10 — NOT a task in this plan, per CONTEXT §Deferred)
- A single TraceID covers stdio → forwarder → daemon → kernel
- The no-global OTel invariant is preserved (per-handler propagator option only)
- Centralized helper (`obs.ClientStatsHandler` / `obs.ServerStatsHandler`) prevents future drift between client and server propagator wiring
</success_criteria>

<output>
After completion, create `.planning/phases/58-v1-9-carryover-release-distribution/58-03-SUMMARY.md` documenting:
- Whether `harness.go` required extension to accept a shared `*obs.Provider`, and the shape of that extension
- The integration-test runtime in CI (latency target from 58-VALIDATION.md is < 600s for full integration suite)
- Confirmation that the no-global rule was held throughout
- Confirmation that PROJECT.md tech-debt entry is ready to be moved to "Resolved at v1.10" by the post-phase step
</output>
