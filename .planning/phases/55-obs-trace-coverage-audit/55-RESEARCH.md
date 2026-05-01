# Phase 55: obs-trace-coverage-audit — Research

**Researched:** 2026-05-01
**Domain:** OpenTelemetry tracing audit & coverage closure (Go)
**Confidence:** HIGH

## Summary

Phase 55 closes the one known gap in the v1.2 tracing pipeline (outbound LS JSON-RPC calls have zero span coverage today) and ships three durable artifacts: a registry-driven Go test that fail-closes on missing coverage, a TRACE-AUDIT.md hygiene review, and a USAGE.md sampling section + Jaeger smoke runbook.

The bulk of this research is **verifying the assumptions already locked into CONTEXT.md** rather than re-deciding. Two non-trivial findings:

1. **CONTEXT.md says "wrap at `internal/kernel/jsonrpc/codec.go` Call layer."** That is location-incorrect: `codec.go` contains only Content-Length framing helpers; the `Conn` type and its `Call` / `Notify` / `Send` methods live in `internal/kernel/jsonrpc/conn.go`. The architectural decision (wrap at the `jsonrpc.Conn.Call/Notify` layer) is unchanged and still correct. The planner should target `conn.go`, not `codec.go`. [VERIFIED: file read, conn.go:85 + codec.go reading]
2. **A pre-existing `ls.request` span EVENT already lives in `Worker.Request` (`internal/kernel/lspool/worker.go:309-314`)** with attributes `lsp.method`, `lsp.language`, `lsp.duration_ms`, gated on `span.IsRecording()`. The new `lspool.lsp.{method}` child span will sit one layer below this event (Worker → Conn). The audit must reconcile both — the event is at the wrong granularity (it's an event on the parent kernel.tool span, not a child span of its own), so Phase 55 closes the gap by adding the child span at the deeper layer. The event is preserved (D-06 of Phase 12 froze it), but TRACE-AUDIT.md must explicitly call out this dual representation and PASS both. [VERIFIED: worker.go read]

**Primary recommendation:** The trace audit test pattern from `dashboards_test.go` ports cleanly. Inject the tracer into `jsonrpc.Conn` via a constructor parameter (`NewConn(rwc, sessionPrefix, tracer)`), defaulting to `tracenoop` so existing call sites stay valid. Wrap the synchronous body of `Call` / `Notify` in `tracer.Start(ctx, "lspool.lsp."+method)` + `defer span.End()`. Add a single `attribute.String("lsp.method", method)` — nothing else.

## User Constraints (from CONTEXT.md)

### Locked Decisions

1. **Audit Mechanism:** Go test (`internal/obs/trace_audit_test.go`), registry-driven, fail-closed, runs in default `go test ./...` (NOT behind `//go:build integration`). Mirrors `dashboards_test.go` pattern.
2. **LS-Call Span Coverage:** Wrap at the `jsonrpc.Conn.Call` layer (CONTEXT says `codec.go`; **actual file is `conn.go`** — correction noted above). Span name `lspool.lsp.{method}`. Only attribute `lsp.method`. Symmetric `Notify` wrapper named `lspool.lsp.notify.{method}`.
3. **TRACE-AUDIT.md Format:** Per-span H2 sections; each section has table with columns `Attribute | Source | PII risk | Cardinality | Verdict`. Located at `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md`.
4. **Sampling Documentation:** New H3 `### Trace Sampling` BEFORE existing `### Prometheus Metrics` H3 in USAGE.md. Document only what ships (head-based `ParentBased(TraceIDRatioBased(...))`). One-line "future scope" note for tail-sampling.
5. **Smoke-Trace Artifact:** `docs/runbooks/trace-smoke.md` runbook + one Jaeger screenshot at `docs/images/trace-smoke-jaeger.png`. Manual checkpoint placeholder in executor task list (Phase 54-05 pattern).

### Claude's Discretion

- Sampling Documentation Scope (already merged into Decision 4 above).

### Deferred Ideas (OUT OF SCOPE)

- Tail-sampling implementation
- `helix trace-audit` CLI subcommand
- Scripted smoke test against in-process OTLP collector
- Tracing for non-MCP/LS surfaces (HTTP transport, channel telemetry, RepoMap extraction)
- Span links between sibling LS calls

## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| OBS-04 | Audit and close trace coverage gaps — every MCP tool handler and every outbound LS call has a span; sampling configuration is documented; trace attributes pass a hygiene review (no PII, no unbounded cardinality). | This entire research file. Coverage gap closed by `Conn.Call/Notify` wrapping (§ "Conn.Call wrapping shape"). Audit enforced by `internal/obs/trace_audit_test.go` (§ "Test pattern"). Hygiene review materializes as TRACE-AUDIT.md (§ "Initial span inventory"). Sampling configuration documented in USAGE.md (§ "Sampling docs insertion"). |

## Project Constraints (from CLAUDE.md)

- **Go-only product** — single binary; no Python in Phase 55.
- **`go vet ./...` and `go test ./...` MUST pass before completing any Go task.** Phase 55 audit runs in default `go test ./...`, so a CI-green check satisfies both the project rule and the phase success criterion.
- **SMTC-first tool routing** — researcher/planner/executor should use SMTC tools (`mcp__smtc__find_references`, `mcp__smtc__goto_definition`) rather than grep when locating Go symbols. (Note: `smtc` server is described in MCP server instructions but no Go capability is documented as activated for this workspace today; falls back to standard grep / Read here, which is acceptable for the scope of this phase.)
- **GSD workflow gate:** all file changes flow through `/gsd:execute-phase`.
- **Benchmarks are local-only** — irrelevant to Phase 55 (no bench changes).

## Architectural Responsibility Map

| Capability | Primary Tier | Secondary Tier | Rationale |
|------------|-------------|----------------|-----------|
| MCP tool span coverage (`kernel.tool.{name}`) | Kernel (handler wrapper) | — | Already owned by `WrapToolSpan` in `internal/kernel/spanwrap.go`. Phase 55 only audits, does not modify. |
| MCP tool attributes (`tool_name`, `profile`, `mode`, `language`, `outcome`) | MCP Middleware | — | Owned by `TelemetryMiddleware` (`internal/mcp/middleware.go:316`). D-07: kernel does NOT touch these. |
| Outbound LS span coverage (`lspool.lsp.{method}`) | jsonrpc.Conn (transport layer) | — | NEW in Phase 55. Single insertion point covers every outbound request and notification automatically. |
| `lsp.method` attribute on outbound spans | jsonrpc.Conn | — | Method string is already available at the call site; nothing else from the LSP request body is exposed. |
| Sampler config (head-based ratio) | obs package (`internal/obs/tracing.go`) | Config (`internal/config/config.go`) | Already shipped in Phase 12. Phase 55 only documents in USAGE.md. |
| Audit enforcement | Test (`internal/obs/trace_audit_test.go`) | — | NEW in Phase 55. Lives next to `dashboards_test.go` and shares the project-root + registry-walking pattern. |
| Smoke-trace evidence | Runbook (`docs/runbooks/`) + Image (`docs/images/`) | — | Manual checkpoint, not automated; mirrors Phase 54-05 dashboard screenshot pattern. |

## Standard Stack

### Core
| Library | Version | Purpose | Why Standard |
|---------|---------|---------|--------------|
| `go.opentelemetry.io/otel` | (already used by Phase 12) | Tracer interface, attributes, span kinds | Already in `go.mod`; the *only* tracing library used in the codebase. [VERIFIED: import statements in `internal/obs/tracing.go`, `internal/kernel/spanwrap.go`, `internal/mcp/middleware.go`] |
| `go.opentelemetry.io/otel/sdk/trace/tracetest` | (already used) | `InMemoryExporter` for unit tests | Used by `internal/obs/tracing_test.go` and `internal/kernel/spanwrap_test.go`. [VERIFIED: file reads] |
| `go.opentelemetry.io/otel/trace/noop` (`tracenoop`) | (already used) | `NewTracerProvider().Tracer("...")` for noop default | Used by `WrapToolSpan` test (`spanwrap_test.go:13`). [VERIFIED] |

### Supporting
- None. Phase 55 ships zero new dependencies.

### Alternatives Considered
| Instead of | Could Use | Tradeoff |
|------------|-----------|----------|
| In-memory exporter for the audit test | Real OTLP collector + testcontainers | Adds dep + CI cost; Phase 12 already proves `tracetest` is sufficient. CONTEXT.md decision rejected this path. |
| Wrapping at `jsonrpc.Conn.Call` | Wrapping at `lspool.Worker.Request` | Worker.Request already has the `ls.request` span EVENT; wrapping a layer up at Worker would miss notifications dispatched directly via `Conn.Notify` (e.g., `initialized`, `textDocument/didOpen` buffered notifications). Conn-level wrapping is the architectural floor for "every outbound JSON-RPC frame." |

**Installation:** No new deps — all transitive imports already in `go.mod`.

**Version verification:** N/A — uses existing pinned versions.

## Architecture Patterns

### System Architecture Diagram

```
                    ┌──────────────────────────────┐
                    │  Forwarder (stdio transport) │
                    │  forwarder.tools.call (root) │ ← already exists, forwarder.go:140
                    └──────────────┬───────────────┘
                                   │ gRPC StreamMCP
                                   ▼
                    ┌──────────────────────────────┐
                    │  TelemetryMiddleware         │
                    │  daemon.mcp.tools.call       │ ← already exists, middleware.go:316
                    │  attrs: tool_name, profile,  │
                    │         mode, language,      │
                    │         outcome              │
                    └──────────────┬───────────────┘
                                   │
                                   ▼
                    ┌──────────────────────────────┐
                    │  WrapToolSpan                │
                    │  kernel.tool.{name}          │ ← already exists, spanwrap.go:30
                    │  attrs: NONE (D-07)          │
                    └──────────────┬───────────────┘
                                   │ tool handler body executes
                                   ▼
                    ┌──────────────────────────────┐
                    │  Worker.Request              │
                    │  (NO new span — keeps event) │ ← worker.go:309-314
                    │  span event "ls.request"     │
                    │    attrs: lsp.method,        │
                    │           lsp.language,      │
                    │           lsp.duration_ms    │
                    └──────────────┬───────────────┘
                                   │
                                   ▼
                    ┌──────────────────────────────┐
                    │  Conn.Call (or Conn.Notify)  │
                    │  *** NEW SPAN IN PHASE 55 ***│ ← conn.go:85 / conn.go:136
                    │  lspool.lsp.{method}         │
                    │  lspool.lsp.notify.{method}  │
                    │  attrs: lsp.method ONLY      │
                    └──────────────────────────────┘
```

The Phase 55 audit asserts the full chain `daemon.mcp.tools.call → kernel.tool.{name} → lspool.lsp.{method}` for any tool that exercises an LS call.

### Recommended Project Structure

No new directories. New files:
```
.planning/phases/55-obs-trace-coverage-audit/
├── 55-RESEARCH.md                  # this file
├── 55-PLAN-*.md                    # planner output
└── TRACE-AUDIT.md                  # success criterion 2

internal/obs/
└── trace_audit_test.go             # NEW — registry-driven audit test

internal/kernel/jsonrpc/
└── conn.go                         # MODIFIED — add tracer plumbing + span wraps

docs/runbooks/
└── trace-smoke.md                  # NEW — Jaeger smoke procedure

docs/images/
└── trace-smoke-jaeger.png          # NEW — placeholder until manual capture

USAGE.md                            # MODIFIED — new "### Trace Sampling" H3
```

### Pattern 1: Tracer injection into `jsonrpc.Conn`

**What:** Add a `tracer trace.Tracer` field to `Conn`, populated via constructor. Default to `tracenoop` if nil so existing call sites in tests don't break.

**When to use:** This is the *only* layering-clean way to give Conn access to a tracer without (a) importing `internal/obs` (would create a cycle since obs depends on nothing kernel-side) or (b) calling `otel.GetTracerProvider()` (BANNED per Phase 12 D-01).

**Example:**
```go
// internal/kernel/jsonrpc/conn.go (MODIFIED)
import (
    "go.opentelemetry.io/otel/attribute"
    "go.opentelemetry.io/otel/trace"
    tracenoop "go.opentelemetry.io/otel/trace/noop"
)

type Conn struct {
    rwc           io.ReadWriteCloser
    reader        *bufio.Reader
    sessionPrefix string
    nextID        atomic.Int64
    mu            sync.Mutex
    pending       map[string]chan *Response
    closed        bool
    OnNotification NotificationFunc
    tracer        trace.Tracer  // NEW
}

func NewConn(rwc io.ReadWriteCloser, sessionPrefix string, tracer trace.Tracer) *Conn {
    if tracer == nil {
        tracer = tracenoop.NewTracerProvider().Tracer("jsonrpc-noop")
    }
    return &Conn{
        rwc: rwc, reader: bufio.NewReader(rwc),
        sessionPrefix: sessionPrefix,
        pending: make(map[string]chan *Response),
        tracer: tracer,
    }
}
```

**Caller update:** `internal/kernel/lspool/process.go:85` becomes `p.conn = jsonrpc.NewConn(rwc, sessionPrefix, p.tracer)`. The `ProcessHandle` already has access to a tracer via `Worker` plumbing (Worker creates ProcessHandle), so threading it down is straightforward.

**Source:** Pattern mirrors how `WrapToolSpan` takes `trace.Tracer` as a parameter rather than calling `otel.Tracer(...)` (`internal/kernel/spanwrap.go:23-24`). [VERIFIED: file read]

### Pattern 2: Wrapping `Conn.Call`

**What:** Single insertion at the top of `Call` so the span covers the full request/response lifecycle including the `<-ctx.Done()` / `<-ch` select.

**Example:**
```go
// internal/kernel/jsonrpc/conn.go (MODIFIED)
func (c *Conn) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
    ctx, span := c.tracer.Start(ctx, "lspool.lsp."+method,
        trace.WithAttributes(attribute.String("lsp.method", method)),
    )
    defer span.End()

    id := c.nextRequestID()
    // ... rest of existing body unchanged ...

    select {
    case <-ctx.Done():
        // ...
        return ctx.Err()
    case resp := <-ch:
        if resp.Error != nil {
            span.RecordError(resp.Error) // optional — Phase 12 WrapToolSpan also records
            return resp.Error
        }
        // ...
    }
}
```

**Notes:**
- The `attribute.String("lsp.method", method)` duplicates the span name suffix. This is intentional and matches OTel semconv guidance — name is for span search, attribute is for filtering/aggregation. The TRACE-AUDIT.md will declare both PASS.
- `RecordError` on the response error path is a free-cost noop when tracing is off (D-17 — `span.IsRecording()` short-circuits). Recommended but optional.
- Notify gets the symmetric wrap: `c.tracer.Start(ctx, "lspool.lsp.notify."+method, ...)`. Notify's body is already short.

### Pattern 3: Registry-driven audit test (mirrors dashboards_test.go)

**What:** Walk the MCP tool registry, invoke each tool against a `tracetest.InMemoryExporter`, assert two specific spans appear with the expected names.

**The challenge:** Unlike `dashboards_test.go` (which only validates static JSON files), the trace audit test must *invoke* each tool to generate spans. Real tools require workspaces, LS workers, file fixtures — too heavy for a unit test.

**Resolution — three-layer test (RECOMMENDED):**

1. **Per-tool `kernel.tool.{name}` span coverage** — verify every registered tool name has a matching `kernel.tool.{name}` span emitted by `WrapToolSpan`. This is structural (does the wrapper get applied at registration?), not behavioral. Approach:
   - Walk `mcpServer.Registry().Names()` to enumerate registered tools.
   - For each name `n`, hand-call `WrapToolSpan(tracer, n, trivialHandler)` and assert it produces span `kernel.tool.<n>`. This is what `spanwrap_test.go:35-55` already does — Phase 55 generalizes it to *every* registered tool name.
   - Subtle: this proves `WrapToolSpan` produces the right name for any name, not that every tool's `RegisterTools` call site actually calls `WrapToolSpan`. To enforce the latter, use a static-lint approach: walk every `mcpsdk.AddTool(server.SDK(), ...)` call site (via `go/ast` or `grep`) and assert the second-arg expression is `kernel.WrapToolSpan(...)`. **Recommendation: use grep-based lint inside the test** — open every `internal/kernel/*/tools.go`, regex-match `mcpsdk.AddTool\(server\.SDK\(\), &mcpsdk\.Tool\{[\s\S]*?Name: \s*"([^"]+)"[\s\S]*?\},\s*kernel\.WrapToolSpan\(`. Tools NOT in this set must be on a documented allowlist (currently `ping`, `echo`, `activate_project` in `internal/mcp/server.go` — these are pre-WRK-01 dummy tools that bypass WrapToolSpan).

2. **`daemon.mcp.tools.call` span coverage** — single assertion: middleware emits this span on every `tools/call`. Already implicit (one code path covers all tools). Assert via mocked `next` handler + middleware invocation against the in-memory exporter.

3. **`lspool.lsp.{method}` span coverage** — assert that calling `Conn.Call(ctx, "textDocument/definition", ...)` against a fake/loopback `io.ReadWriteCloser` produces a span named `lspool.lsp.textDocument/definition`. Single representative call, not every LSP method.

**Fail-closed conditions:**
- `len(registry.Names()) == 0` → fail (mirrors dashboards_test empty-glob fail).
- Any registered tool name fails the WrapToolSpan name check → fail.
- Any tool registered via `mcpsdk.AddTool` outside the allowlist that is not wrapped by `kernel.WrapToolSpan` → fail.
- `Conn.Call` against the fake transport produces no span with the expected name → fail.

**Source:** Pattern lifted from `internal/obs/dashboards_test.go:268-299`. The structural-lint approach (grep-based) mirrors how `dashboards_test.go::extractPromQLFromDashboard` walks JSON. [VERIFIED]

### Anti-Patterns to Avoid

- **`otel.GetTracerProvider()` in jsonrpc package** — violates Phase 12 D-01 (no global tracer).
- **Adding `lsp.language` attribute to `lspool.lsp.{method}` span** — already on parent `kernel.tool.{name}` span via `TelemetryMiddleware`'s `language` attr. Per the cardinality discipline in CONTEXT.md ("`language` is already on the parent kernel.tool span via TelemetryMiddleware — do NOT re-attach"), redundant. PASS verdict only if attribute is on parent only. [CITED: 55-CONTEXT.md decisions block]
- **Adding `lsp.params` or any request-body bytes** — unbounded cardinality + PII risk (file URIs). REFUSE in audit.
- **Wrapping at `Worker.Request`** — would miss `Conn.Notify` paths (`initialized`, buffered `textDocument/didOpen`). Single source of truth must be `Conn`.
- **Renaming `daemon.mcp.tools.call` or `kernel.tool.{name}`** — Phase 12 contract; downstream Jaeger queries / dashboards key off these names. OUT OF SCOPE per CONTEXT.

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Span recording in tests | Custom span recorder | `tracetest.InMemoryExporter` (already used in Phase 12 tests) | Battle-tested upstream OTel SDK helper. |
| Noop tracer fallback | Custom no-op `Tracer` impl | `go.opentelemetry.io/otel/trace/noop` (`tracenoop.NewTracerProvider().Tracer(...)`) | Already used in `spanwrap_test.go:13`. |
| Tool registry walk | Reflection / build-tag tricks | `mcpServer.Registry().Names()` (existing API at `internal/mcp/registry.go:51-59`) | Already there. |
| Source-code lint in test | external linter | Read files via `os.ReadFile` + regex, mirrors `dashboards_test.go::extractPromQLFromDashboard` | Already proven pattern in Phase 54. |
| OTLP collector for smoke trace | testcontainers + collector mock | One-time manual `docker run jaegertracing/all-in-one` per `docs/runbooks/trace-smoke.md` | CONTEXT explicitly rejected scripted path. |

**Key insight:** Phase 55 is mostly *audit*, not *build*. The only real code change is the `Conn.Call`/`Conn.Notify` wrap (~15 LOC) plus the tracer-injection plumbing (~20 LOC across `conn.go`, `process.go`, `worker.go`).

## Common Pitfalls

### Pitfall 1: Inserting span at codec.go (the wrong file)

**What goes wrong:** CONTEXT.md decision text says "wrap at the `jsonrpc.Conn.Call` layer in `internal/kernel/jsonrpc/codec.go`." But `codec.go` only has `ReadMessage` / `WriteMessage` framing helpers (69 LOC, no `Conn` type). The planner/executor following CONTEXT verbatim would either (a) put the span on `WriteMessage` (which fires for both Call and Notify with no method context — wrong) or (b) get confused.

**Why it happens:** CONTEXT was authored before the file was opened; the actual `Conn` definition is in `conn.go` (added in Phase 56-prep work).

**How to avoid:** **Plan task targets `internal/kernel/jsonrpc/conn.go` line 85 (`Call`) and line 136 (`Notify`). Insert at the top of each method body. Do NOT modify `codec.go`.**

**Warning signs:** Diff modifies `codec.go` instead of `conn.go`.

### Pitfall 2: Tracer plumbing breaks existing tests

**What goes wrong:** `NewConn` signature changes from `NewConn(rwc, sessionPrefix)` to `NewConn(rwc, sessionPrefix, tracer)`. Every existing call site (`process.go:85`, `codec_test.go:180`, possibly more) breaks.

**Why it happens:** Mandatory third arg.

**How to avoid:** Add the tracer arg as the third parameter and accept `nil` → noop fallback inside the constructor. Update `process.go`'s call site. Update `codec_test.go::TestConn_Call` to pass `nil` (which becomes noop). [VERIFIED: only two production call sites — `process.go:85` and tests in `codec_test.go`]

**Warning signs:** Compile errors after the conn.go edit; missing tracer in any test.

### Pitfall 3: Audit test invokes tool handlers that need workspaces

**What goes wrong:** Naive audit test attempts `mcpServer.SDK().CallTool(ctx, "find_references", ...)` and gets `ErrWorkspaceNotActive` (no workspace registered). Test depends on heavy fixtures.

**Why it happens:** Real tool handlers need a kernel + workspace + LS worker.

**How to avoid:** Use the **three-layer test** (Pattern 3 above): structural lint via grep-on-source for WrapToolSpan coverage; mock `next` for middleware coverage; fake `io.ReadWriteCloser` for Conn coverage. Never invoke real tools.

**Warning signs:** Test imports `internal/daemon`, sets up workspace, takes >1s.

### Pitfall 4: ls.request span EVENT and lspool.lsp span both reported

**What goes wrong:** TRACE-AUDIT.md reviewer sees both the `ls.request` event on `kernel.tool.{name}` AND the new child `lspool.lsp.{method}` span. Looks like double-instrumentation; reviewer asks to remove one.

**Why it happens:** `Worker.Request` (`worker.go:309-314`) emits the event before the new span was added. Phase 12 D-06 ("span EVENT, not child span") froze the event.

**How to avoid:** TRACE-AUDIT.md must explicitly call out the dual representation under both `kernel.tool.{name}` (event subsection) and `lspool.lsp.{method}` (child-span section), with a one-line rationale: "Event is the v1.2 timing breadcrumb on the parent; child span is the v1.9 first-class operation. Both PASS — neither leaks PII or unbounded values, and they serve different consumer queries (event for in-context aggregate, child span for dedicated `lspool.*` filter in Jaeger)."

**Warning signs:** TRACE-AUDIT.md silently de-dups; reviewer flags the difference.

### Pitfall 5: Trace Sampling section placement vs. existing Enable Tracing

**What goes wrong:** CONTEXT says insert `### Trace Sampling` BEFORE `### Prometheus Metrics` (line 660). But USAGE.md *already* has `### Enable Tracing` at line 734, downstream of Prometheus Metrics. Operator reading top-down hits "Trace Sampling" before knowing tracing exists at all.

**Why it happens:** CONTEXT focused on the metrics-section-relative placement (mirrors Phase 54-05 dashboard insertion at line 634 before Prometheus Metrics). It overlooked the existing tracing subsection at 734.

**How to avoid:** **Planner should reconcile by inserting `### Trace Sampling` AS A NEW H3 IMMEDIATELY AFTER `### Enable Tracing` (line 734)**, not before Prometheus Metrics. This keeps tracing-related H3s adjacent and produces a coherent operator flow: Enable Tracing → Trace Sampling → Enable pprof. **This is the one CONTEXT decision that should be re-litigated during plan-check** (small reconciliation, not a re-design).

Alternative: the planner can keep CONTEXT verbatim and place at line 660 (before Prometheus Metrics). The cost is operator-flow incoherence; not a hard fail.

**Warning signs:** Final USAGE.md has `### Trace Sampling` separated from `### Enable Tracing` by ~70 lines of unrelated content.

### Pitfall 6: Tool allowlist drift

**What goes wrong:** Audit test allowlist (`ping`, `echo`, `activate_project` — pre-WRK-01 dummy tools that don't go through WrapToolSpan) gets stale when those tools are removed/renamed in a future phase.

**Why it happens:** Allowlist is a hardcoded constant in the test.

**How to avoid:** Allowlist comment includes "remove entries when their underlying registration moves into a kernel package via WrapToolSpan." Comment lives next to the constant.

**Warning signs:** Test fails when a dummy tool is removed.

## Code Examples

### Example 1: Conn.Call wrap (verified pattern)

```go
// internal/kernel/jsonrpc/conn.go — Call method, modification
//
// Insert at top of method body, before id := c.nextRequestID().
func (c *Conn) Call(ctx context.Context, method string, params interface{}, result interface{}) error {
    ctx, span := c.tracer.Start(ctx, "lspool.lsp."+method,
        trace.WithAttributes(attribute.String("lsp.method", method)),
    )
    defer span.End()

    // ... existing body unchanged ...
}
```

Source pattern: identical shape to `WrapToolSpan` (`internal/kernel/spanwrap.go:30-37`) and `TelemetryMiddleware` (`internal/mcp/middleware.go:316-317`). [VERIFIED]

### Example 2: Audit test scaffolding

```go
// internal/obs/trace_audit_test.go (NEW — sketch)
package obs

import (
    "os"
    "path/filepath"
    "regexp"
    "runtime"
    "testing"

    "go.opentelemetry.io/otel/sdk/trace/tracetest"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// Allowlist: pre-WRK-01 dummy tools registered without WrapToolSpan in
// internal/mcp/server.go. Remove entries when their registration moves
// into a kernel package via WrapToolSpan.
var trace_audit_allowlist = map[string]bool{
    "ping":             true,
    "echo":             true,
    "activate_project": true,
}

var addToolWrappedRe = regexp.MustCompile(
    `mcpsdk\.AddTool\(server\.SDK\(\),\s*&mcpsdk\.Tool\{[\s\S]*?Name:\s*"([^"]+)"[\s\S]*?\},\s*kernel\.WrapToolSpan\(`,
)

func TestEveryRegisteredToolWrappedWithKernelSpan(t *testing.T) {
    root := projectRoot(t)
    wrapped := map[string]bool{}
    matches, _ := filepath.Glob(filepath.Join(root, "internal", "kernel", "*", "tools.go"))
    if len(matches) == 0 {
        t.Fatal("no kernel tool registration files found")
    }
    for _, p := range matches {
        data, err := os.ReadFile(p)
        if err != nil { t.Fatalf("%s: %v", p, err) }
        for _, m := range addToolWrappedRe.FindAllSubmatch(data, -1) {
            wrapped[string(m[1])] = true
        }
    }
    if len(wrapped) == 0 {
        t.Fatal("static lint found zero WrapToolSpan-wrapped registrations — regex broke")
    }
    // Cross-check vs registry — handled in a separate test that
    // builds an mcpServer and walks its registry.
}

func TestConnCallProducesLspoolSpan(t *testing.T) {
    exp := tracetest.NewInMemoryExporter()
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
        sdktrace.WithSyncer(exp),
    )
    tracer := tp.Tracer("test")

    // Loopback Conn: a fake io.ReadWriteCloser that responds to one Call.
    conn := jsonrpc.NewConn(loopbackPipe(), "test", tracer)
    go conn.Listen(t.Context())
    var result struct{}
    _ = conn.Call(t.Context(), "textDocument/definition", nil, &result)

    spans := exp.GetSpans()
    found := false
    for _, s := range spans {
        if s.Name == "lspool.lsp.textDocument/definition" {
            found = true
            for _, attr := range s.Attributes {
                if string(attr.Key) == "lsp.method" && attr.Value.AsString() == "textDocument/definition" {
                    return // success
                }
            }
            t.Errorf("span found but missing lsp.method attribute")
        }
    }
    if !found { t.Fatal("expected lspool.lsp.textDocument/definition span; not found") }
}
```

(Sketch — planner refines.)

### Example 3: TRACE-AUDIT.md initial span inventory

The audit covers exactly these spans (enumerated by `grep -rn "tracer\.Start" internal/`):

| Span Name | Source File:Line | Owner |
|-----------|------------------|-------|
| `forwarder.tools.call` | `internal/forwarder/forwarder.go:140` | Forwarder root span |
| `daemon.mcp.tools.call` | `internal/mcp/middleware.go:316` | TelemetryMiddleware |
| `kernel.tool.{name}` | `internal/kernel/spanwrap.go:30` | WrapToolSpan |
| `lspool.lsp.{method}` | `internal/kernel/jsonrpc/conn.go` (NEW) | Conn.Call |
| `lspool.lsp.notify.{method}` | `internal/kernel/jsonrpc/conn.go` (NEW) | Conn.Notify |

Plus: span event `ls.request` on the `kernel.tool.{name}` span (from `internal/kernel/lspool/worker.go:309-314`). This is an EVENT, not a span — TRACE-AUDIT.md groups it under the `kernel.tool.{name}` H2 section.

[VERIFIED via grep against current HEAD, 2026-05-01]

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| `parser.ParseExpr` (PromQL) | `parser.NewParser(parser.Options{}).ParseExpr` | prometheus@v0.311.x | N/A to Phase 55 (irrelevant — different package). Mentioned only because `dashboards_test.go` uses the new form. |
| `otel.SetTracerProvider` (global) | Inject `trace.Tracer` into constructors | Phase 12 (Helix-internal D-01) | Phase 55 follows — pass tracer to `NewConn`, do not register globally. |
| Worker.Request span EVENT (single layer) | Worker.Request EVENT + Conn child span (two layers) | Phase 55 (this phase) | Provides both v1.2 backward-compat and v1.9 first-class operation span. Reviewer sees both in TRACE-AUDIT.md. |

**Deprecated/outdated:** None relevant.

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | Dummy tools `ping`, `echo`, `activate_project` are the *only* tools registered without `WrapToolSpan`. | Audit allowlist (Pitfall 6) | Test fails or allowlist needs expanding; low risk — easily detected and corrected on first run. |
| A2 | Pattern 3's grep regex correctly matches every `mcpsdk.AddTool(... WrapToolSpan(...))` call site across the codebase. | Audit test (Example 2) | False negatives → test passes when it shouldn't (silent drift). Mitigation: include a `_catchesDrift` companion test mirroring `dashboards_test.go::TestDashboardsAndRunbooksReferenceRegisteredMetrics_catchesDrift` that feeds a synthetic non-wrapped-AddTool string and asserts the regex flags it. |
| A3 | `Conn.Listen` correctly propagates context through Call's select loop so the new span is on the active context throughout. | Pattern 2 | If Listen runs in a separate goroutine with its own ctx (it does — see `worker.go:193-198`), the *response delivery* doesn't carry the span, only the *Call invocation* does. This is fine — span ends when Call returns, regardless of which goroutine wrote to the response channel. Verified by reading `conn.go:115-132`. |
| A4 | Adding the third parameter `tracer trace.Tracer` to `NewConn` only affects 2 production call sites + 1 test file. | Pitfall 2 | If hidden call sites exist, the Phase 55 plan needs more file edits. Verified via grep `jsonrpc.NewConn` — only `process.go:85` in production. [VERIFIED] |
| A5 | The `### Trace Sampling` section should be placed AFTER `### Enable Tracing` (line 734), not before `### Prometheus Metrics` (line 660). | Pitfall 5 | If the user prefers CONTEXT verbatim, the planner reverts to before-Prometheus placement. Either is shippable; flagged for plan-check user review. |

If the user agrees with all five assumptions, no decisions need re-litigation. A5 is the only one that meaningfully affects the deliverable shape.

## Open Questions

1. **Should `lspool.lsp.{method}` set `span.Status` to error on response-error path?**
   - What we know: `WrapToolSpan` calls `RecordError` but explicitly does NOT call `SetStatus` (see `spanwrap.go:32-35` + `spanwrap_test.go:103-107` comment).
   - What's unclear: should Phase 55 be consistent (RecordError only, no SetStatus) or richer (both)?
   - Recommendation: **Consistent with WrapToolSpan — RecordError only, no SetStatus.** TRACE-AUDIT.md notes this as a deliberate v1.2 design choice.

2. **Smoke runbook: is Jaeger the only acceptable collector, or should the runbook show a generic OTLP-collector path too?**
   - What we know: CONTEXT explicitly names Jaeger and `jaegertracing/all-in-one` Docker image. Screenshot is one-time.
   - What's unclear: a future operator running, e.g., Tempo or a vendor SaaS won't follow Jaeger steps verbatim.
   - Recommendation: runbook leads with Jaeger (concrete, fast); adds a one-line "Any OTLP/gRPC collector on `localhost:4317` works — Jaeger is the example because the all-in-one image is one command."

3. **Should the audit test's static-lint regex match against `internal/skill/**/tools.go` too?**
   - What we know: skill tools register via `mcpServer.AddSkillTool` (`server.go:222-244`), not `mcpsdk.AddTool(server.SDK(), ...)`. The grep regex won't catch them because they go through a different path.
   - What's unclear: do skill tools currently get a `kernel.tool.{name}` span?
   - Recommendation: spot-check `internal/skill/memory/` and `internal/skill/workflow/` registration paths during planning. If they bypass `WrapToolSpan`, EITHER (a) extend the allowlist to include skill-tool names, (b) add `WrapToolSpan` to the `AddSkillTool` path, or (c) document that skill tools intentionally don't get a kernel sub-span (and audit only the middleware-level `daemon.mcp.tools.call` for them). This is the highest-risk gap in Phase 55 — likely needs a Wave 0 spike to resolve before the executor commits to a plan.

## Environment Availability

| Dependency | Required By | Available | Version | Fallback |
|------------|------------|-----------|---------|----------|
| Go | All build/test work | ✓ (project default) | (project go.mod pins) | — |
| `go.opentelemetry.io/otel*` | All code paths | ✓ (already in go.mod via Phase 12) | (existing) | — |
| Docker (for smoke runbook) | Manual smoke trace step (Task: capture screenshot) | local-only requirement | n/a | Defer screenshot, commit 1×1 placeholder per Phase 54-05 pattern. |
| Jaeger Docker image (`jaegertracing/all-in-one`) | Manual smoke trace | only at smoke-trace capture time | latest | Same as above. |

**Missing dependencies with no fallback:** None.

**Missing dependencies with fallback:** Docker + Jaeger — manual checkpoint task in the executor list, defer with placeholder if executor cannot drive it (mirrors Phase 54-05 dashboard screenshot deferral).

## Validation Architecture

`workflow.nyquist_validation` is `true` in `.planning/config.json`. This section is REQUIRED.

### Test Framework

| Property | Value |
|----------|-------|
| Framework | Go's stdlib `testing` (existing) + `go.opentelemetry.io/otel/sdk/trace/tracetest` (existing dep) |
| Config file | `go.mod` (no per-test config) |
| Quick run command | `go test ./internal/obs/ -run TestTraceAudit -count=1` |
| Full suite command | `go test ./... -count=1` |

### Phase Requirements → Test Map

| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| OBS-04 | Every registered MCP tool emits `kernel.tool.{name}` span | unit (static lint + structural) | `go test ./internal/obs/ -run TestEveryRegisteredToolWrappedWithKernelSpan -count=1` | ❌ Wave 0 |
| OBS-04 | Every `tools/call` emits `daemon.mcp.tools.call` span (TelemetryMiddleware) | unit (mocked next handler) | `go test ./internal/obs/ -run TestMiddlewareEmitsToolsCallSpan -count=1` | ❌ Wave 0 |
| OBS-04 | Every outbound `Conn.Call` emits `lspool.lsp.{method}` span with `lsp.method` attribute | unit (loopback Conn + InMemoryExporter) | `go test ./internal/obs/ -run TestConnCallProducesLspoolSpan -count=1` | ❌ Wave 0 |
| OBS-04 | Every outbound `Conn.Notify` emits `lspool.lsp.notify.{method}` span | unit (loopback Conn) | `go test ./internal/obs/ -run TestConnNotifyProducesLspoolNotifySpan -count=1` | ❌ Wave 0 |
| OBS-04 | Static lint regex catches drift (catches a non-wrapped `mcpsdk.AddTool` registration) | unit (synthetic input) | `go test ./internal/obs/ -run TestEveryRegisteredToolWrappedWithKernelSpan_catchesDrift -count=1` | ❌ Wave 0 |
| OBS-04 | TRACE-AUDIT.md exists and lists every Phase 55 span | manual review | `test -f .planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` (existence check); content reviewed at `/gsd-verify-work` | ❌ Wave 0 (created by executor) |
| OBS-04 | USAGE.md `### Trace Sampling` exists | static (grep) | `grep -c '^### Trace Sampling$' USAGE.md` returns ≥1 | ❌ Wave 0 |
| OBS-04 | `docs/runbooks/trace-smoke.md` exists | static (file existence) | `test -f docs/runbooks/trace-smoke.md` | ❌ Wave 0 |
| OBS-04 | Smoke screenshot exists (placeholder OK; real capture is manual checkpoint) | static (file magic) | `file docs/images/trace-smoke-jaeger.png \| grep -q "PNG image"` | ❌ Wave 0 |
| OBS-04 | Noop-path zero-allocation invariant (D-17) preserved | reference existing Phase 12 tests | `go test ./internal/obs/ -run TestDefaultSamplerOff -count=1` (already exists at `internal/obs/tracing_test.go:16`) | ✅ |

### Sampling Rate

- **Per task commit:** `go test ./internal/obs/... -count=1` (covers trace audit + dashboards validator + tracing tests; ≤ ~10s).
- **Per wave merge:** `go vet ./... && go test ./...` (full suite; existing project rule from CLAUDE.md).
- **Phase gate:** Full suite green before `/gsd-verify-work`.

### Wave 0 Gaps

- [ ] `internal/obs/trace_audit_test.go` — covers OBS-04 audit (5 tests above + `_catchesDrift` companion).
- [ ] `internal/kernel/jsonrpc/conn.go` — `tracer trace.Tracer` field + Call/Notify wraps.
- [ ] `internal/kernel/lspool/process.go:85` — pass tracer through `NewConn`.
- [ ] `internal/kernel/jsonrpc/codec_test.go` — update `TestConn_Call` to pass `nil` (or noop) tracer to `NewConn`.
- [ ] `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` — per-span hygiene tables.
- [ ] `USAGE.md` — `### Trace Sampling` H3 (placement TBD per Pitfall 5 / A5).
- [ ] `docs/runbooks/trace-smoke.md` — runbook with Phase 54-04 frontmatter (severity/since_phase/last_reviewed) + standard H2 order.
- [ ] `docs/images/trace-smoke-jaeger.png` — placeholder (defer real capture per Phase 54-05 pattern).
- [ ] Framework install: NONE — `tracetest` and `tracenoop` already pulled in via Phase 12 deps.

## Security Domain

`security_enforcement` is not explicitly disabled in `.planning/config.json`; treating as enabled.

### Applicable ASVS Categories

| ASVS Category | Applies | Standard Control |
|---------------|---------|-----------------|
| V2 Authentication | no | Phase 55 adds no auth surface |
| V3 Session Management | no | n/a |
| V4 Access Control | no | n/a |
| V5 Input Validation | partial | The audit test enforces no unbounded-cardinality attribute. `lsp.method` is a closed enum from the LSP spec — no validation needed. |
| V6 Cryptography | no | n/a |
| V7 Error Handling & Logging | yes | Spans are a logging surface. Audit explicitly verifies NO PII in span attributes (file paths, URIs, request bodies). |
| V14 Configuration | yes | `ObservabilityConfig.TracingEndpoint` is a config-driven egress destination; an attacker controlling config could exfiltrate traces. Already mitigated by Phase 12 (config is admin-only; loopback admin listener). No new surface in Phase 55. |

### Known Threat Patterns for OTel tracing

| Pattern | STRIDE | Standard Mitigation |
|---------|--------|---------------------|
| PII in span attributes (file paths, URIs, request bodies) | Information Disclosure | TRACE-AUDIT.md per-attribute review certifies no PII. The `lspool.lsp.{method}` span attribute is `lsp.method` only (closed enum), NOT request params. |
| Unbounded cardinality crashing collector / blowing storage | Denial of Service | TRACE-AUDIT.md cardinality column verdicts. `lsp.method` is bounded by LSP spec (~50 methods). NO file paths or IDs as attributes. |
| Trace egress to attacker-controlled endpoint | Tampering / Information Disclosure | `TracingEndpoint` config is loopback-listener-gated (Phase 12); no Phase 55 change. |
| Span name injection (e.g., method = "../<malicious>") | Tampering | LSP method names come from the local jsonrpc.Conn caller (Worker / adapter), not from the network response. The LS server cannot inject a span name through the response path because `Call(method ...)` takes method as a parameter from internal callers. [VERIFIED — see `conn.go:85`] |

## Sources

### Primary (HIGH confidence)
- `internal/obs/tracing.go` — frozen Phase 12 design (D-01, D-09, D-11, D-17 in file header)
- `internal/obs/tracing_test.go` — InMemoryExporter usage pattern (TestDefaultSamplerOff)
- `internal/obs/dashboards_test.go` — registry-driven validator pattern (PATTERN COPIED)
- `internal/kernel/spanwrap.go` + `spanwrap_test.go` — WrapToolSpan + parent-linking + InMemoryExporter pattern
- `internal/mcp/middleware.go:280-330` — TelemetryMiddleware span emission
- `internal/mcp/registry.go` — `Names()` enumeration API
- `internal/kernel/jsonrpc/conn.go` — actual Conn definition and Call/Notify methods
- `internal/kernel/lspool/worker.go:299-336` — existing `ls.request` event + Conn.Call/Notify call sites
- `internal/forwarder/forwarder.go:140` — forwarder root span
- `.planning/phases/55-obs-trace-coverage-audit/55-CONTEXT.md` — locked decisions
- `.planning/phases/54-obs-dashboards-runbooks/54-04-SUMMARY.md` — runbook frontmatter pattern (D-15/D-16/D-17)
- `.planning/phases/54-obs-dashboards-runbooks/54-05-SUMMARY.md` — USAGE.md insertion + screenshot deferral pattern

### Secondary (MEDIUM confidence)
- None — all claims sourced from primary repository files.

### Tertiary (LOW confidence)
- None.

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH — all dependencies already in go.mod, all patterns verified against existing files
- Architecture (Conn-level wrap + tracer plumbing): HIGH — wrapping shape verified against `WrapToolSpan` and `TelemetryMiddleware` precedents
- Audit test pattern: MEDIUM-HIGH — three-layer approach is the planner's call to refine; static-lint regex (Pattern 3) needs `_catchesDrift` companion to be safe
- Pitfalls: HIGH — five concrete pitfalls all verified against current code (file paths cited)
- Open Question 3 (skill-tool wrapping): **MEDIUM, flagged for Wave 0 spike** — could expand Phase 55 scope if skill tools bypass WrapToolSpan

**Research date:** 2026-05-01
**Valid until:** 2026-06-01 (30 days; codebase is stable in v1.9 polish)

## RESEARCH COMPLETE

**Phase:** 55 - obs-trace-coverage-audit
**Confidence:** HIGH

### Key Findings

- CONTEXT.md says wrap at `codec.go`; the actual `Conn.Call`/`Conn.Notify` live in `conn.go` (codec.go is framing-only). Planner targets `conn.go:85` and `conn.go:136`.
- Tracer must be injected into `Conn` via constructor (`NewConn(rwc, sessionPrefix, tracer)`); `otel.GetTracerProvider()` is BANNED per D-01. Default-noop fallback when nil.
- A pre-existing `ls.request` span EVENT lives in `Worker.Request` (`worker.go:309-314`) — TRACE-AUDIT.md must reconcile dual representation (event on parent + new child span at `Conn` layer).
- Audit test uses three-layer pattern: (1) static-lint grep for `WrapToolSpan` coverage in `internal/kernel/*/tools.go`, (2) mocked middleware for `daemon.mcp.tools.call`, (3) loopback `Conn` for `lspool.lsp.{method}` — no real workspaces needed. Allowlist for `ping`/`echo`/`activate_project` (pre-WRK-01 dummy tools).
- Sampling H3 placement (Pitfall 5 / A5): planner should re-evaluate CONTEXT's "before Prometheus Metrics" guidance — placing AFTER existing `### Enable Tracing` (line 734) yields a more coherent operator flow. Either is shippable.
- **Open Question 3 is the highest risk:** verify skill tools (`internal/skill/memory/`, `internal/skill/workflow/`) actually go through `WrapToolSpan` — if not, Wave 0 spike needed before executor commits.

### File Created
`.planning/phases/55-obs-trace-coverage-audit/55-RESEARCH.md`

### Confidence Assessment
| Area | Level | Reason |
|------|-------|--------|
| Standard Stack | HIGH | Zero new deps; all OTel imports already in go.mod |
| Architecture (Conn-level wrap + tracer injection) | HIGH | Pattern verified against `WrapToolSpan` and `TelemetryMiddleware` precedents |
| Audit test pattern | MEDIUM-HIGH | Three-layer approach verified; needs `_catchesDrift` companion to lock the static-lint |
| Pitfalls | HIGH | All five cite specific file:line locations |
| Skill-tool coverage (Q3) | MEDIUM | Flagged for Wave 0 spike |

### Open Questions
1. RecordError-only vs RecordError+SetStatus on Conn span error path → recommend RecordError-only (consistent with WrapToolSpan).
2. Generic OTLP collector vs Jaeger-only in smoke runbook → recommend Jaeger lead + one-line generic note.
3. **Skill tools through WrapToolSpan? — Wave 0 spike recommended.**

### Ready for Planning
Research complete. Planner should:
- Correct CONTEXT's `codec.go` reference to `conn.go`.
- Resolve Open Question 3 (skill-tool wrapping) in Wave 0.
- Reconcile Pitfall 5 (Trace Sampling H3 placement) with user.
- All other CONTEXT decisions stand verbatim.
