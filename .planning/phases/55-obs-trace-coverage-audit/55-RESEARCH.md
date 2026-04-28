# Phase 55: obs-trace-coverage-audit - Research

**Researched:** 2026-04-28
**Domain:** OpenTelemetry tracing audit (Go OTel SDK v1.43, in-binary observability)
**Confidence:** HIGH

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-04 | Audit and close trace coverage gaps — every MCP tool handler and every outbound LS call has a span, sampling configuration is documented, and trace attributes pass a hygiene review (no PII, no unbounded cardinality). | Phase 12 wired the SDK pipeline (`internal/obs/tracing.go`); kernel handlers are span-wrapped via `kernel.WrapToolSpan` (`internal/kernel/spanwrap.go`); MCP middleware emits `daemon.mcp.tools.call` (`internal/mcp/middleware.go:203`); LS calls currently emit a span EVENT, NOT a child span (`internal/kernel/lspool/worker.go:309-315`) — this is the primary gap to close. Skill tools registered via `AddSkillTool` are NOT span-wrapped (`internal/mcp/server.go:201-225`) — secondary gap. |
</phase_requirements>

## Summary

Phase 12 (v1.2) shipped the OTel pipeline: `internal/obs` owns the SDK TracerProvider, OTLP/gRPC exporter, `ParentBased(TraceIDRatioBased(ratio))` sampler, and a noop fallback. Phase 55 is an audit + targeted gap-fix pass on top of that pipeline. Three concrete coverage gaps were discovered in the existing wiring:

1. **Outbound LS JSON-RPC calls emit a span EVENT, not a child span** (`internal/kernel/lspool/worker.go:309-315`). Success criterion #1 requires a child span. This is a real semantic change — events do not establish parent/child links and are invisible to per-LS-method latency aggregations in the trace backend.
2. **Skill-provided tools (memory: 7, workflow: 2, repomap: 2) are not span-wrapped.** `SerenaMCPServer.AddSkillTool` (`internal/mcp/server.go:201-225`) calls `mcpsdk.AddTool` directly without `kernel.WrapToolSpan`. They DO get the parent `daemon.mcp.tools.call` span from middleware, but no `kernel.tool.{name}` child span — so per-tool span filtering in the backend silently misses 11 tools.
3. **No automated audit gate.** Coverage today is "the developer remembered to wrap it." The phase needs a reflection/registry-driven test that fails when a registered tool name has no corresponding span emission, plus an attribute-allowlist test mirroring the Phase 11 / Phase 53 `metrics_labels_test.go` carve-out pattern.

The codebase already provides the audit primitives: `mcp.ToolRegistry.Names()` enumerates all 41+ registered tools (`internal/mcp/registry.go:51`), and `tracetest.InMemoryExporter` is already used in `internal/mcp/telemetry_span_test.go` and `internal/kernel/spanwrap_test.go`. Span attributes are currently a closed five-element set on the parent span (`tool_name`, `profile`, `mode`, `language`, `outcome`) plus three on the LS event (`lsp.method`, `lsp.language`, `lsp.duration_ms`) — small enough to certify exhaustively in TRACE-AUDIT.md.

**Primary recommendation:** Three plans. (55-01) Convert the LS span event to a child span + add `kernel.tool.{name}` wrapping inside `AddSkillTool` + introduce an `attribute_allowlist_test.go` mirroring the metrics carve-out. (55-02) Ship the automated coverage audit (registry-driven test that exercises every tool against an in-memory exporter and asserts span shape). (55-03) Author TRACE-AUDIT.md certifying every attribute, expand USAGE.md Observability section with sampling guidance, capture a smoke trace against `otel-collector --config logging-exporter` (dev-only Docker, not a runtime requirement).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| OTel SDK TracerProvider + OTLP exporter + sampler construction | `internal/obs` | — | Phase 12 D-04: third-party OTel deps live ONLY in `internal/obs`. Sampler is `ParentBased(TraceIDRatioBased(ratio))` (`tracing.go:62`). |
| Parent `daemon.mcp.tools.call` span + RED attributes | `internal/mcp` (TelemetryMiddleware) | `internal/obs` (Tracer accessor) | One span per `tools/call`; never wraps `tools/list`/`initialize`. Attributes set behind `IsRecording()` for D-17 budget. |
| Child `kernel.tool.{name}` spans for kernel-resident tools | `internal/kernel` (`spanwrap.go`) | each kernel package's `RegisterTools` | `WrapToolSpan` is applied at registration time; per Phase 12 D-07 NO attributes on kernel spans (the parent already has them). |
| Child spans for skill-resident tools (memory, workflow, repomap, profile) | `internal/mcp` (`AddSkillTool`) — **gap** | `internal/skill/*` registrations | Today not wrapped. Plan 55-01 inserts `WrapToolSpan` inside `AddSkillTool` so all skills get free coverage without touching skill packages. |
| Child spans for outbound LS JSON-RPC calls | `internal/kernel/lspool` (`worker.go:Request`) — **gap** | `internal/obs` (Tracer accessor passed via existing kernel `Tracer()` plumbing) | Today emits an event; criterion #1 requires a child span (`ls.request.{method}` recommended). Worker already has a `language` field for the attribute. |
| Attribute hygiene certification (PII / cardinality) | `internal/obs/attribute_allowlist_test.go` (NEW, mirror of `metrics_labels_test.go`) | — | Mechanical CI gate; failing test = attribute review needed. |
| Coverage audit (every registered tool emits spans) | `internal/daemon` or `internal/mcp` (NEW `coverage_test.go`) | `internal/mcp/registry.go` (Names) + `tracetest.InMemoryExporter` | Registry already enumerates names; test exercises each tool through middleware against in-mem exporter. |
| TRACE-AUDIT.md review artifact | `.planning/phases/55-obs-trace-coverage-audit/` | — | Static checklist deliverable per success criterion #2. |

## Standard Stack

This is internal instrumentation on existing infrastructure — no new library adoption. All OTel deps are pinned by Phase 12.

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.opentelemetry.io/otel` | v1.43.0 | Tracer / Span / attribute API. | [VERIFIED: go.mod:41] Pinned in Phase 12. |
| `go.opentelemetry.io/otel/sdk` | v1.43.0 | TracerProvider, batcher, sampler. | [VERIFIED: go.mod:43] |
| `go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc` | v1.43.0 | OTLP/gRPC exporter. | [VERIFIED: go.mod:42] |
| `go.opentelemetry.io/otel/sdk/trace/tracetest` | v1.43.0 (transitive) | `InMemoryExporter` for span assertions. | [VERIFIED: existing use in `internal/mcp/telemetry_span_test.go:15`, `internal/kernel/spanwrap_test.go`] |
| `go.opentelemetry.io/otel/trace/noop` | v1.43.0 | Noop tracer for the disabled path. | [VERIFIED: `internal/obs/obs.go:24`] |
| `go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc` | v0.68.0 | Already pinned; gRPC instrumentation (used by forwarder/daemon IPC). | [VERIFIED: go.mod:40] |

### Supporting
None — phase consumes existing pinned versions.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In-memory `tracetest.InMemoryExporter` for the coverage test | Real OTLP collector subprocess in test | Adds Docker / network dependency to CI. In-memory is the standard pattern (already used in this repo) and is canonical per OTel Go testing docs. |
| Reflection-driven coverage test | Hand-maintained allowlist in test | Allowlist drifts; reflection over `mcp.ToolRegistry.Names()` is self-updating and is the audit semantically required by criterion #1. |
| Recommend Jaeger all-in-one for the smoke test | otelcol-contrib with `logging` exporter | Jaeger is heavier; the `logging`/`debug` exporter prints spans to stdout — sufficient evidence for criterion #4 and lighter to script. |

**Installation:** N/A (libraries already pinned).

**Version verification:** Skipped — Phase 12 / 53 / 54 already locked the OTel surface; bumping minor versions is out of scope (Deferred Idea).

## Architecture Patterns

### System Architecture Diagram

Span lineage on a `tools/call` request, end to end:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│ MCP client (Claude Code, Codex, etc.)                                        │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 │ tools/call
                                 ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ TelemetryMiddleware (internal/mcp/middleware.go)                             │
│   tracer.Start(ctx, "daemon.mcp.tools.call")                                 │
│   attrs (post-handler): tool_name, profile, mode, language, outcome          │
│   RecordError + SetStatus(Error) on err                                      │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 │ ctx with parent span
                                 ▼
                ┌────────────────┴────────────────┐
                │                                 │
                ▼                                 ▼
┌──────────────────────────────┐  ┌──────────────────────────────────────────┐
│ Kernel tool                  │  │ Skill tool (memory, workflow, repomap)   │
│ kernel.WrapToolSpan          │  │ today: NO child span (gap #2)            │
│   tracer.Start(ctx,          │  │ after 55-01: WrapToolSpan inside         │
│     "kernel.tool.{name}")    │  │   AddSkillTool wraps generically         │
│   NO attributes (D-07)       │  │                                          │
│   RecordError on err         │  │                                          │
└──────────────┬───────────────┘  └──────────────────────────────────────────┘
               │ ctx with kernel.tool.* span
               ▼
┌──────────────────────────────────────────────────────────────────────────────┐
│ Worker.Request (internal/kernel/lspool/worker.go)                            │
│   today: span.AddEvent("ls.request", ...)  ← gap #1                          │
│   after 55-01: tracer.Start(ctx, "ls.request."+method) child span            │
│   attrs: lsp.method, lsp.language, lsp.duration_ms                           │
│   RecordError + SetStatus on JSON-RPC error                                  │
└────────────────────────────────┬─────────────────────────────────────────────┘
                                 │
                                 ▼
                       Language Server (gopls, jdtls, …)
```

### Recommended Project Structure

No new packages. New / modified files only:

```
internal/
├── mcp/
│   ├── server.go                 # MODIFY: AddSkillTool wraps with WrapToolSpan
│   ├── coverage_test.go          # NEW: registry-driven span coverage audit
│   └── attribute_allowlist_test.go  # NEW: mirror of metrics_labels_test.go
├── kernel/
│   └── lspool/
│       ├── worker.go             # MODIFY: Request emits child span, not event
│       └── worker_span_test.go   # NEW: span lineage assertions for Request
└── obs/
    └── tracing.go                # NO CHANGE (sampler config already correct)

.planning/phases/55-obs-trace-coverage-audit/
└── TRACE-AUDIT.md                # NEW: review artifact (success criterion #2)

USAGE.md                          # MODIFY: expand "Enable Tracing" section
```

### Pattern 1: Registry-driven coverage audit (NEW)

**What:** Drive `mcp.ToolRegistry.Names()` through an in-memory exporter and assert one parent span per tool plus a child span whose name starts with `kernel.tool.` or `skill.tool.`.

**When to use:** As the canonical CI gate that fails when a new tool is added without span wiring.

**Example:**
```go
// Source: pattern derived from internal/mcp/telemetry_span_test.go:50-84 + internal/mcp/registry.go:51
func TestEveryRegisteredToolEmitsSpans(t *testing.T) {
    exp := tracetest.NewInMemoryExporter()
    daemon := bootstrapTestDaemon(t, exp)  // helper: builds daemon w/ tracetest provider
    for _, name := range daemon.MCP().Registry().Names() {
        exp.Reset()
        invokeTool(t, daemon, name, map[string]any{}) // tolerates handler error; we only check spans
        spans := exp.GetSpans()
        require.NotEmpty(t, spans, "tool %q produced no spans", name)
        var sawParent, sawChild bool
        for _, s := range spans {
            if s.Name == "daemon.mcp.tools.call" { sawParent = true }
            if strings.HasPrefix(s.Name, "kernel.tool.") || strings.HasPrefix(s.Name, "skill.tool.") {
                sawChild = true
            }
        }
        assert.True(t, sawParent, "tool %q missing parent span", name)
        assert.True(t, sawChild, "tool %q missing child span", name)
    }
}
```

### Pattern 2: Attribute allowlist test (NEW, mirror of `metrics_labels_test.go`)

**What:** Single closed table of allowed attribute keys per span name. Test scans every emitted span and fails on any unlisted attribute.

**When to use:** Same role as Phase 11 D-13 metric-label allowlist — the CI-enforced PII / cardinality gate.

**Example:**
```go
// Source: pattern from internal/obs/metrics_labels_test.go (Phase 11 / extended Phase 53)
var allowedSpanAttrs = map[string]map[string]struct{}{
    "daemon.mcp.tools.call": {
        "tool_name": {}, "profile": {}, "mode": {}, "language": {}, "outcome": {},
    },
    "kernel.tool.*":  {}, // intentionally empty — D-07 forbids attributes here
    "ls.request.*": {
        "lsp.method": {}, "lsp.language": {}, "lsp.duration_ms": {},
    },
}
```

### Pattern 3: Convert span event to child span (the gap fix)

**What:** Replace `span.AddEvent(...)` with `tracer.Start(ctx, ...)` + `defer span.End()`.

**When to use:** Anywhere a discrete unit of work has its own latency budget (LS JSON-RPC calls qualify).

**Example:**
```go
// Source: pattern derived from internal/kernel/spanwrap.go:30 + current worker.go:299-317
func (w *Worker) Request(ctx context.Context, method string, params, result interface{}) error {
    if WorkerState(w.state.Load()) != WorkerReady { return ErrWorkerNotReady }
    ctx, span := w.tracer.Start(ctx, "ls.request."+method)
    defer span.End()
    if span.IsRecording() {
        span.SetAttributes(
            attribute.String("lsp.method", method),
            attribute.String("lsp.language", w.language),
        )
    }
    start := time.Now()
    err := w.process.Conn().Call(ctx, method, params, result)
    duration := time.Since(start)
    if span.IsRecording() {
        span.SetAttributes(attribute.Int64("lsp.duration_ms", duration.Milliseconds()))
    }
    if err != nil {
        if span.IsRecording() {
            span.RecordError(err)
            span.SetStatus(codes.Error, err.Error())
        }
        return err
    }
    w.metrics.OnReuse()
    return nil
}
```

Note: this requires plumbing `trace.Tracer` into `Worker` (currently uses `trace.SpanFromContext`). The kernel already holds a `trace.Tracer` (`internal/kernel/kernel.go:31`); pass it to the pool / worker constructor (mirror of how `MetricsSink` was plumbed in Phase 11).

### Anti-Patterns to Avoid

- **Setting `workspace_path`, `repo_root`, file paths, or symbol names as span attributes.** Unbounded cardinality; PII risk on shared collectors. Bucket if needed (e.g., `path.bucket=test|src|vendor|other`) or hash with truncation. Today's parent-span attribute set is clean — keep it that way.
- **Wrapping LS calls in a child span without `IsRecording()` gating on attribute construction.** Phase 12 D-17 already established that the SDK tracer allocates per-span even when sampled out; gate every `SetAttributes` behind `IsRecording()`.
- **Calling `otel.SetTracerProvider` or `otel.GetTracerProvider`.** Phase 12 D-01 forbids both — pass the provider explicitly via `obs.Provider.Tracer()`. The audit test must also use the explicit provider, not the global default.
- **Tail-based sampling.** OTel Go SDK only ships head-based samplers (`AlwaysSample`, `NeverSample`, `TraceIDRatioBased`, `ParentBased`); tail-based sampling requires the otel-collector. Document this honestly in USAGE.md (success criterion #3 says "head vs. tail" — the answer is "head only in-binary; for tail, run otel-collector").
- **Using `attribute.String("error", err.Error())`.** Use `span.RecordError(err)` + `span.SetStatus(codes.Error, ...)`; this is the OTel-canonical shape and is what existing code (`middleware.go:246-247`) already does.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| In-test span capture | A custom `mockTracer` | `go.opentelemetry.io/otel/sdk/trace/tracetest.InMemoryExporter` | Already used in this repo (`internal/mcp/telemetry_span_test.go:15`); canonical OTel testing pattern. |
| Sampler primitive | A custom `Sampler` impl | `sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))` | Already in `internal/obs/tracing.go:62`. Covers the head-sampling space; documented in OTel spec. |
| OTLP collector for the smoke test | A custom span receiver | `otel/opentelemetry-collector-contrib` with `debug` (formerly `logging`) exporter | One-line config, prints spans to stdout. Dev-only — does not violate the in-binary rule. |
| Per-tool wrapping for skills | Hand-edit each skill's `Tools()` to wrap | Wrap once inside `AddSkillTool` (`internal/mcp/server.go:209`) | Skills don't import `internal/kernel/spanwrap`; centralizing wrapping at the registration helper keeps the "skills don't know about tracing" property. |

**Key insight:** The OTel ecosystem is mature and the v1.43 API surface is stable. Every primitive needed for this phase already exists in the repo or in pinned deps; the work is auditing and connecting, not inventing.

## Runtime State Inventory

> Phase 55 is an audit + small-surface refactor of in-process telemetry. There is no stored data, no external service registration, and no OS-level state being renamed.

| Category | Items Found | Action Required |
|----------|-------------|------------------|
| Stored data | None — span data is exported in real time and not persisted by Serena. | None. |
| Live service config | OTLP collector endpoint configured via `observability.tracing_endpoint` in user/project YAML; behavior unchanged by this phase. | None — endpoint string is operator config, not Serena state. |
| OS-registered state | None. | None. |
| Secrets/env vars | None — `otlptracegrpc.WithInsecure()` is the only credential mode wired (Phase 12); TLS / auth deferred. | None. |
| Build artifacts | None — pure Go source changes; no codegen, no embedded assets. | None. |

**Nothing found in any category** — verified by reading `internal/obs/`, `internal/config/config.go:37-57`, and confirming there's no persistence layer for spans.

## Common Pitfalls

### Pitfall 1: Span event vs child span confusion (the actual existing gap)

**What goes wrong:** `span.AddEvent("ls.request", ...)` emits an event on the *current* span — it does NOT create a new span, has no duration, no status, no parent/child link, and is invisible to per-method latency aggregations in the trace UI.

**Why it happens:** Easy to confuse with `tracer.Start(ctx, "ls.request")`. The AddEvent path was chosen in Phase 12 to keep the hot path cheap, but criterion #1 of OBS-04 explicitly mandates a child span.

**How to avoid:** Use `tracer.Start` whenever the work has its own latency, error semantics, or needs to appear as a node in the waterfall. Use `AddEvent` only for instantaneous markers (e.g., "cache_lookup", "buffer_flush") within an existing span.

**Warning signs:** Trace waterfall in the backend shows a flat `daemon.mcp.tools.call` → `kernel.tool.{name}` and no LS work visible. (This is exactly what the smoke trace in criterion #4 will reveal.)

### Pitfall 2: Per-span allocation on the disabled path

**What goes wrong:** SDK `tracer.Start` allocates per call even when the sampler returns `Drop` — so wrapping every LS call adds ~1-3 allocs per call when tracing is off.

**Why it happens:** SDK tracer is not free. Phase 12 D-17 captured this and chose the noop tracer for the `endpoint==""` path (`internal/obs/tracing.go:11-13`). The noop tracer's `Start` IS free.

**How to avoid:** Confirm the noop path is preserved (it is, by `obs.Noop()`). Add a benchmark in the new worker test that asserts `Worker.Request` allocates ≤ N bytes when tracing is noop (mirror of `BenchmarkTracingOffPath` referenced in Phase 12 verification).

**Warning signs:** Allocation regression in `BenchmarkLSPoolRequest` after the gap fix.

### Pitfall 3: Sampling defaults silently drop the smoke trace

**What goes wrong:** Default `tracing_sample_ratio: 0.0` means even with a configured endpoint, zero spans flow. The operator runs the smoke test, sees nothing, concludes tracing is broken.

**Why it happens:** ParentBased sampler with ratio=0 drops all root spans. ALL Serena spans today are root spans (no inbound trace context from MCP clients).

**How to avoid:** USAGE.md MUST call out: "for smoke testing or local debugging, set `tracing_sample_ratio: 1.0`". Phase 55 plan 55-03 includes this as a required addition.

**Warning signs:** Operator opens an issue saying "tracing doesn't work" — the sample ratio is the first thing to check.

### Pitfall 4: TracerProvider not flushed on shutdown loses the smoke trace

**What goes wrong:** `BatchSpanProcessor` queues spans; if the process exits before flush, the smoke trace is empty.

**Why it happens:** Daemon shutdown path must call `obs.Provider.ShutdownTracing(ctx)` with a bounded context. Already wired (`internal/obs/obs.go:89-95`); confirm it's actually invoked in `daemon.Run` shutdown.

**How to avoid:** Spot-check `internal/daemon/daemon.go` shutdown path during planning. If the call is missing, that's a separate (small) fix folded into 55-01.

**Warning signs:** Smoke test produces fewer spans than tool calls invoked.

### Pitfall 5: Child span in `Worker.Request` ends after error returns to caller

**What goes wrong:** If the caller (kernel handler) catches the error and adds *more* attributes via `trace.SpanFromContext(ctx)`, those land on the kernel.tool span — not the ls.request span — once the LS span has ended via `defer`.

**Why it happens:** OTel span lifetimes are explicit. Standard Go `defer span.End()` is correct; just be aware that post-`End()` attribute writes are silently dropped.

**How to avoid:** Set all LS-call attributes BEFORE the work, then `RecordError` + `SetStatus` if needed, then return. This is what the Pattern 3 example does.

## Code Examples

### Common Operation 1: Wrap a skill tool with a child span (gap-fix for AddSkillTool)

```go
// Source: pattern combining internal/mcp/server.go:201 + internal/kernel/spanwrap.go:23
// Inside SerenaMCPServer.AddSkillTool:
mcpsdk.AddTool(s.sdk, tool, func(ctx context.Context, req *mcpsdk.CallToolRequest, args map[string]any) (*mcpsdk.CallToolResult, any, error) {
    ctx, span := s.tracer.Start(ctx, "skill.tool."+toolName) // NEW
    defer span.End()                                          // NEW
    result, err := executor.ExecuteTool(toolName, args)
    if err != nil {
        if span.IsRecording() { span.RecordError(err) }       // NEW
        return &mcpsdk.CallToolResult{ ... IsError: true }, nil, nil
    }
    return &mcpsdk.CallToolResult{ ... }, nil, nil
})
```

(Requires `SerenaMCPServer` to hold a `trace.Tracer` — currently it doesn't. Constructor change is small; mirror `*obs.Metrics` plumbing.)

### Common Operation 2: Iterate the registry for the coverage audit

```go
// Source: internal/mcp/registry.go:51
names := mcpServer.Registry().Names()
for _, name := range names { /* drive each through the middleware */ }
```

### Common Operation 3: Capture spans in test

```go
// Source: internal/mcp/telemetry_span_test.go:24-29
exp := tracetest.NewInMemoryExporter()
tp := sdktrace.NewTracerProvider(
    sdktrace.WithSyncer(exp),
    sdktrace.WithSampler(sdktrace.AlwaysSample()),
)
provider := obs.NewForTest(tp)
```

### Common Operation 4: Smoke-test OTLP collector (dev-only)

```yaml
# otel-collector-config.yaml — dev-only verification, NOT a runtime requirement
receivers:
  otlp:
    protocols:
      grpc:
        endpoint: 0.0.0.0:4317
exporters:
  debug:
    verbosity: detailed
service:
  pipelines:
    traces:
      receivers: [otlp]
      exporters: [debug]
```

```bash
# Run via Docker (dev-only):
docker run --rm -p 4317:4317 \
  -v $(pwd)/otel-collector-config.yaml:/etc/otelcol-contrib/config.yaml \
  otel/opentelemetry-collector-contrib:0.118.0
```

Then point Serena at it:
```yaml
observability:
  tracing_endpoint: "127.0.0.1:4317"
  tracing_sample_ratio: 1.0   # 100% for smoke test
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| OTel `logging` exporter | `debug` exporter (same behavior, renamed) | otel-collector v0.86 (2023-09) | Use `debug` in any new collector configs we ship as dev tooling. |
| Manual instrumentation everywhere | Centralized wrapping at registration time (`WrapToolSpan`) | Phase 12 (Serena, 2026-04) | Already adopted for kernel tools; this phase extends it to skill tools. |
| Tail-based sampling in-process | Tail-based sampling in collector only | OTel Go SDK never shipped tail-based; collector added it in 2021 | Document honestly: head-only in-binary; tail requires running otel-collector. |
| `attribute.String("error", ...)` | `span.RecordError(err) + SetStatus(codes.Error, ...)` | OTel spec stabilization (2022) | Already correct in `middleware.go:246-247`; preserve in new code. |

**Deprecated/outdated:**
- `logging` exporter name — use `debug` (same impl, renamed in collector).
- Setting global TracerProvider — `otel.SetTracerProvider`/`GetTracerProvider` are forbidden by Phase 12 D-01 (still standard outside this codebase, but our discipline is explicit injection).

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go `testing` + `testify` (`require`/`assert`) — already in `go.mod` |
| Config file | None (Go test discovery) |
| Quick run command | `go test ./internal/mcp/... ./internal/kernel/lspool/... ./internal/obs/...` |
| Full suite command | `go vet ./... && go test ./...` |

### Phase Requirements → Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-04 | Every registered MCP tool emits parent + child span when invoked | unit (registry-driven) | `go test ./internal/mcp -run TestEveryRegisteredToolEmitsSpans -v` | ❌ Wave 0 (NEW `internal/mcp/coverage_test.go`) |
| OBS-04 | Every outbound LS JSON-RPC call wraps in child span (not event) | unit | `go test ./internal/kernel/lspool -run TestRequestEmitsChildSpan -v` | ❌ Wave 0 (NEW `internal/kernel/lspool/worker_span_test.go`) |
| OBS-04 | Span attribute keys conform to closed allowlist | unit (CI gate) | `go test ./internal/obs -run TestSpanAttributeAllowlist -v` | ❌ Wave 0 (NEW `internal/obs/attribute_allowlist_test.go`) |
| OBS-04 | Noop tracer path remains zero-alloc on `Worker.Request` | benchmark | `go test ./internal/kernel/lspool -bench BenchmarkRequestNoopTracer -benchmem` | ❌ Wave 0 (extend `worker_span_test.go`) |
| OBS-04 | TRACE-AUDIT.md exists, lists every attribute, certifies hygiene | static (file-existence + section-presence) | `go test ./.planning -run TestTraceAuditDocument` (or extend `internal/obs/usage_test.go` pattern) | ❌ Wave 0 (or accept human review per phase 54 HUMAN-UAT precedent) |
| OBS-04 | USAGE.md Observability section documents sampling | regression (string-presence) | `go test . -run TestUsageDocumentsTracing` (extend `usage_test.go` pattern from Phase 54) | ❌ Wave 0 |
| OBS-04 | Smoke trace shows full request path against live OTLP collector | manual (HUMAN-UAT) | Docker collector + `serena setup` + invoke a tool; capture stdout from collector | ❌ Wave 0 — manual; document in `55-HUMAN-UAT.md` (mirror Phase 54) |

### Sampling Rate
- **Per task commit:** `go test ./internal/mcp/... ./internal/kernel/lspool/... ./internal/obs/...`
- **Per wave merge:** `go vet ./... && go test ./...`
- **Phase gate:** Full suite green + `55-HUMAN-UAT.md` smoke-trace evidence captured before `/gsd-verify-work`.

### Wave 0 Gaps
- [ ] `internal/mcp/coverage_test.go` — registry-driven coverage audit (covers OBS-04 #1)
- [ ] `internal/kernel/lspool/worker_span_test.go` — child-span lineage assertions + noop benchmark (covers OBS-04 #1)
- [ ] `internal/obs/attribute_allowlist_test.go` — attribute hygiene gate (covers OBS-04 #2)
- [ ] Extend root-level `usage_test.go` (Phase 54 pattern) with sampling-doc presence assertion (covers OBS-04 #3)
- [ ] `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` — review artifact (covers OBS-04 #2)
- [ ] `.planning/phases/55-obs-trace-coverage-audit/55-HUMAN-UAT.md` — smoke-trace evidence (covers OBS-04 #4, mirrors Phase 54)

## Security Domain

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | N/A — phase touches in-process tracing only. |
| V3 Session Management | no | N/A. |
| V4 Access Control | no | N/A. |
| V5 Input Validation | yes (light) | Span attribute values originate from internal sources (tool name from registry, language from worker config, lsp.method from constants). No user-supplied strings ever land on a span attribute key. The allowlist test enforces this structurally. |
| V6 Cryptography | no | N/A — `otlptracegrpc.WithInsecure()` is dev-mode only; TLS is operator-deferred (Phase 12 scope). |

### Known Threat Patterns for Serena tracing

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| PII leakage via span attribute (file paths, user IDs, secrets in URIs) | Information Disclosure | Closed attribute-key allowlist (mirror of metrics labels gate). TRACE-AUDIT.md certifies each attribute. |
| Cardinality blow-up via per-request unique attribute values | Denial of Service (against trace backend) | Same allowlist + an explicit "no `workspace_path`, no `file_path`, no `symbol_name`" rule certified in TRACE-AUDIT.md. Bucketing/hashing recommended in the audit doc if such an attribute is later proposed. |
| Spans leaking to a malicious collector | Information Disclosure | Operator config — endpoint is opt-in (`tracing_endpoint == ""` disables). Document the implications in USAGE.md alongside the 1.0 sample-ratio recommendation for smoke tests. |
| Trace context injection from MCP clients | Tampering | All Serena spans today are root spans; no inbound trace context is honored. Document this in TRACE-AUDIT.md as a deliberate property (we won't propagate client-supplied IDs into our pipeline until V3-style auth lands). |

## Project Constraints (from CLAUDE.md)

- **Always run `go vet` and `go test` before completing any Go task.** — applies to every plan in this phase.
- **GSD Workflow Enforcement:** all repo edits go through GSD; this phase IS the GSD entry. Direct edits forbidden.
- **Single Go binary, no Docker / Python / runtime dependencies.** The smoke-test OTLP collector (Docker) is dev-only verification, not a runtime requirement — explicitly call this out in TRACE-AUDIT.md and USAGE.md to preserve the in-binary promise (per the user-memory note: "Observability stays in-binary").
- **Layer separation:** OTel imports live ONLY in `internal/obs` (Phase 12 D-04 inherited from Phase 11 D-08 metrics rule). The new tests in `internal/mcp` and `internal/kernel/lspool` MAY import `go.opentelemetry.io/otel/sdk/trace` and `tracetest` because tests don't ship in the binary — but production code in those packages must continue to depend only on `internal/obs.Provider.Tracer()`.
- **Legacy `legacy/` directory is read-only** — phase touches only `internal/`, `cmd/`, `USAGE.md`, `.planning/`.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go toolchain | All Go work | ✓ | 1.25.1 (per Phase 50) | — |
| `go.opentelemetry.io/otel*` v1.43 | All span code | ✓ | v1.43.0 (pinned) | — |
| `tracetest.InMemoryExporter` | All span tests | ✓ | transitive of otel/sdk | — |
| Docker | Smoke-test OTLP collector (dev-only) | unknown — operator-dependent | — | otel-collector binary install OR delegate smoke-test to human via HUMAN-UAT (Phase 54 precedent) |
| `otelcol-contrib` image | Smoke-test only | unknown | latest stable (≥ 0.118.0 recommended for `debug` exporter clarity) | Same as above |

**Missing dependencies with no fallback:** None — phase is implementable end-to-end with Go toolchain alone if smoke trace is captured manually by a human (criterion #4 is intrinsically a human-in-the-loop verification).

**Missing dependencies with fallback:** Docker (smoke-test only). HUMAN-UAT pattern from Phase 54 already establishes the precedent of deferring "live external system" verifications to a checklist a human ticks off.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | The `debug` exporter (formerly `logging`) is the right minimal collector to recommend for the smoke test. | Code Examples #4 | Low — alternatives (Jaeger, Tempo) are heavier. If the operator prefers Jaeger, USAGE.md can list both. [ASSUMED based on otel-collector training knowledge] |
| A2 | `BatchSpanProcessor.Shutdown` is currently invoked on daemon shutdown via `obs.Provider.ShutdownTracing`. | Pitfall 4 | Medium — if NOT invoked today, the smoke test will appear to fail. Plan-checker must verify by reading `internal/daemon/daemon.go` shutdown path. [ASSUMED — not verified in this session] |
| A3 | Wrapping skill tools inside `AddSkillTool` adds ≤ 1 alloc per call when tracing is noop (matches the kernel pattern's profile). | Pitfall 2 | Low — pattern is identical to existing `WrapToolSpan`; if profile differs, the benchmark in 55-01 catches it. [ASSUMED — to be confirmed by benchmark in plan 55-01] |
| A4 | The four currently-set parent-span attributes (`tool_name`, `profile`, `mode`, `language`, `outcome`) are bounded enums and contain no PII. | Pattern 2 / Security Domain | Low — `tool_name` ≤ ~50 (registry-bounded), `profile` ∈ {claude-code, codex, ide-assistant, ci-bot, full}, `mode` ∈ {read, edit, review, admin}, `language` ≤ 52 (langregistry), `outcome` ∈ 7-value enum. Cross-referenced with Phase 11 metric label allowlist. [VERIFIED: internal/mcp/middleware.go:238-244 + internal/obs/metrics.go AllowedLabels] |
| A5 | No skill tool today calls `tracer.Start` or `trace.SpanFromContext` outside the kernel layer. | Architectural Map | Low — grep across `internal/skill/` returned zero hits. [VERIFIED: `grep -rn "Tracer\|trace\." internal/skill/` returned no production hits] |

**If this table is empty:** Two `[ASSUMED]` items (A1, A2) need confirmation during planning — A1 is cosmetic, A2 is a real correctness check the planner should add as a verification step in 55-03.

## Open Questions

1. **Should `kernel.tool.{name}` and `skill.tool.{name}` share a single namespace?**
   - What we know: Phase 12 D-07 chose `kernel.tool.*` deliberately because kernel handlers are first-class. Skill tools (memory, workflow, repomap) are equally valid MCP tools from the client's perspective.
   - What's unclear: Whether a unified `tool.{name}` namespace would make backend filtering cleaner, or whether keeping `kernel.tool.*` vs `skill.tool.*` distinct is operationally useful for separating "does Serena's code intelligence work" from "do auxiliary skills work."
   - Recommendation: Use `skill.tool.*` for parity-but-distinct (matches the architectural distinction in CLAUDE.md). Defer to discuss-phase if there's a strong reason to unify.

2. **Should the LS child span be `ls.request.{method}` (high cardinality on method) or `ls.request` with `lsp.method` attribute (low cardinality)?**
   - What we know: LSP method names are a closed enum (~30 in our usage). Per-method spans give better filtering in the backend; uniform name gives smaller cardinality and clearer aggregation.
   - What's unclear: Operator preference. Phase 12 made the analogous choice for events (uniform `ls.request` name + attribute) deliberately.
   - Recommendation: Keep `ls.request` as the span name with `lsp.method` attribute (matches existing event shape, preserves cardinality discipline). Discuss-phase can override.

3. **Does `Worker.Request` need its own injected `trace.Tracer`, or can it call `trace.SpanFromContext(ctx).TracerProvider().Tracer(...)`?**
   - What we know: The latter avoids constructor changes but couples Worker to the OTel API surface (forbidden by Phase 12 D-04 layering — OTel imports only in `internal/obs`).
   - What's unclear: Whether the layering rule applies to `trace.Tracer` parameter types in non-obs packages (the kernel already accepts `trace.Tracer` per `internal/kernel/kernel.go:31`, so precedent exists).
   - Recommendation: Inject `trace.Tracer` into `Worker` via the pool constructor (mirror of how `MetricsSink` is plumbed). Precedent in `internal/kernel/kernel.go` makes this clean.

4. **Should we add an OTLP/HTTP exporter alongside OTLP/gRPC?**
   - What we know: Today only OTLP/gRPC is wired (`tracing.go:43`).
   - What's unclear: Whether operators using HTTP-only collectors would benefit. Out of scope per phase scope ("audit + close gaps", not "expand exporter surface").
   - Recommendation: Defer to a separate phase if requested. Document the OTLP/gRPC-only constraint in USAGE.md.

## Sources

### Primary (HIGH confidence)
- Source code in this repo (cited inline by file:line throughout):
  - `internal/obs/obs.go` — Provider, Tracer accessor, ShutdownTracing
  - `internal/obs/tracing.go` — TracerProvider construction, sampler, OTLP exporter
  - `internal/obs/spancontext.go` — log/trace correlation
  - `internal/mcp/middleware.go:157-258` — TelemetryMiddleware span emission
  - `internal/mcp/telemetry_span_test.go` — canonical span-test pattern
  - `internal/mcp/registry.go:51` — `Names()` for the audit
  - `internal/mcp/server.go:201-225` — `AddSkillTool` (gap site #2)
  - `internal/kernel/spanwrap.go` — `WrapToolSpan` helper
  - `internal/kernel/spanwrap_test.go` — span lineage test pattern
  - `internal/kernel/lspool/worker.go:299-317` — `Request` (gap site #1)
  - `internal/kernel/symbols/tools.go`, `internal/kernel/edit/tools.go`, `internal/kernel/fileops/tools.go`, `internal/kernel/diag/tools.go`, `internal/kernel/health/tools.go`, `internal/kernel/help/tools.go` — `WrapToolSpan` call sites
  - `internal/skill/memory/skill.go`, `internal/skill/workflow/skill.go`, `internal/skill/repomap/*.go` — uninstrumented skill tools
  - `internal/daemon/daemon.go:240-300, 450-470` — registration topology
  - `internal/config/config.go:37-57` — ObservabilityConfig
  - `USAGE.md:336-344, 772-783` — current sampling docs (deficient — phase 55-03 expands)
- Phase 53 RESEARCH.md (template + carve-out pattern)
- Phase 12 VERIFICATION.md (sampler + degraded-fallback proofs)
- Phase 54 RESEARCH/HUMAN-UAT (HUMAN-UAT precedent for live-system verification)
- ROADMAP.md:165-175 (success criteria) and REQUIREMENTS.md:34 (OBS-04)

### Secondary (MEDIUM confidence)
- OTel Go SDK v1.43 API knowledge (sampler primitives, BatchSpanProcessor, RecordError/SetStatus semantics) — pinned via go.mod, behavior frozen for the phase. [CITED: opentelemetry.io/docs/specs/otel/trace/sdk/]

### Tertiary (LOW confidence)
- Recommendation of `otel/opentelemetry-collector-contrib:0.118.0` for the smoke-test exporter — version pin is best-effort current; operator can use any recent release. [ASSUMED — A1 in Assumptions Log]

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all libs already pinned and proven in production by Phase 12.
- Architecture: HIGH — gap sites located by direct grep + line numbers; fix shape mirrors existing `WrapToolSpan` pattern.
- Pitfalls: HIGH — derived from Phase 12 documented design rules + direct code inspection.
- Audit pattern: HIGH — registry already exposes `Names()`; in-memory exporter already used in repo.
- Smoke-test tooling: MEDIUM — Docker availability is operator-dependent; mitigated by HUMAN-UAT precedent.

**Research date:** 2026-04-28
**Valid until:** 2026-05-28 (30 days — Phase 12 / OTel surface is stable)
