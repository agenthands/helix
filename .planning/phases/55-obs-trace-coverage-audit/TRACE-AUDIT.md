---
phase: 55-obs-trace-coverage-audit
artifact: trace-attribute-hygiene-review
last_reviewed: 2026-05-02
reviewers: claude/55-04-executor
---

# Phase 55 Trace Attribute Hygiene Review

Per OBS-04 success criterion 2: every span attribute on the helix tracing
surface listed below is certified for PII risk and cardinality. The review
walks the trace tree top-down — `forwarder.tools.call` →
`daemon.mcp.tools.call` → `kernel.tool.{name}` (kernel + skill paths) →
`lspool.lsp.{method}` / `lspool.lsp.notify.{method}` — and certifies that
zero attributes leak PII and zero attributes carry unbounded cardinality.

## Spans Covered

| Span Name                          | Source                                              | Phase Introduced |
| ---------------------------------- | --------------------------------------------------- | ---------------- |
| `forwarder.tools.call`             | `internal/forwarder/forwarder.go:140`               | 12               |
| `daemon.mcp.tools.call`            | `internal/mcp/middleware.go:316`                    | 12               |
| `kernel.tool.{name}` (kernel path) | `internal/kernel/spanwrap.go:30`                    | 12               |
| `kernel.tool.{name}` (skill path)  | `internal/mcp/server.go:247` (AddSkillTool)         | 55               |
| `lspool.lsp.{method}`              | `internal/kernel/jsonrpc/conn.go:102` (Conn.Call)   | 55               |
| `lspool.lsp.notify.{method}`       | `internal/kernel/jsonrpc/conn.go:160` (Conn.Notify) | 55               |

Plus one span EVENT — `ls.request` on the `kernel.tool.{name}` span — emitted
from `internal/kernel/lspool/worker.go:322` (Phase 12 D-06: frozen as an
event, not a child span).

## Verdict Legend

| Verdict | Meaning                                                                    |
| ------- | -------------------------------------------------------------------------- |
| PASS    | Bounded enum or registered/deterministic set; safe for production.         |
| FLAG    | High-cardinality but justified (e.g., bounded LSP method enum, ~50 vals). |
| FAIL    | Unbounded or PII-leaking. Phase 55 ships zero FAIL attributes.             |

## forwarder.tools.call

Root span produced by the stdio→gRPC forwarder at `sendWithSpan`
(`internal/forwarder/forwarder.go:140`) on every `tools/call` payload.

| Attribute | Source | PII risk | Cardinality | Verdict |
| --------- | ------ | -------- | ----------- | ------- |
| _(none)_  | —      | —        | —           | —       |

Rationale: the forwarder root span deliberately carries zero attributes; it
exists to anchor downstream spans into one trace. All identifying labels
(tool_name, profile, mode, language, outcome) live on the child
`daemon.mcp.tools.call` span where TelemetryMiddleware owns them. Nothing to
flag.

## daemon.mcp.tools.call

Produced by `TelemetryMiddleware` at `internal/mcp/middleware.go:316` for
every method == `"tools/call"` request. Attributes are set inside the
`span.IsRecording()` gate (line 350) so the tracing-off path allocates
nothing (D-17 budget).

| Attribute   | Source                                | PII risk                                       | Cardinality                         | Verdict |
| ----------- | ------------------------------------- | ---------------------------------------------- | ----------------------------------- | ------- |
| `tool_name` | `middleware.go:352` (`extractToolName`) | none — registered tool names only              | bounded (≈41 registered tools + dummies) | PASS    |
| `profile`   | `middleware.go:353` (`SessionInfo.Snapshot`) | none — closed enum                             | 5 (claude-code, codex, ide-assistant, ci-bot, full) | PASS    |
| `mode`      | `middleware.go:354` (`SessionInfo.Snapshot`) | none — closed enum                             | 4 (read, edit, review, admin)       | PASS    |
| `language`  | `middleware.go:355` (`SessionInfo.Snapshot`) | none — registry enum                           | 52 (langregistry entries)           | PASS    |
| `outcome`   | `middleware.go:356` (`classifyOutcome`) | none — closed enum                             | 7 (success, invalid_args, not_found, circuit_open, ls_crash, timeout, internal) | PASS    |

Error-path side effects (line 358-361): `span.RecordError(err)` and
`span.SetStatus(codes.Error, err.Error())`. The error message string IS
attached to the span via `SetStatus` (Otel semconv), but is not promoted to
a span attribute key — it lives on the `Status.Description` field whose
cardinality is collector-side, not metric-cardinality. PII risk is bounded
because error messages come from internal kernel error wrapping, not user
input. PASS.

## kernel.tool.{name}

Two emitting paths converge on the same span name:

1. **Kernel path** — `internal/kernel/spanwrap.go:30` (`WrapToolSpan`): wraps
   `RegisterTools`-time handlers for the 9 symbol / 6 edit / 7 fileop / 3
   diagnostic / kernel-resident health/help tools.
2. **Skill path** — `internal/mcp/server.go:247` (`AddSkillTool`): wraps
   skill-package tools (memory, workflow, repomap) that register through
   the generic SDK `AddTool` adapter.

Both paths emit zero attributes per D-07 (`spanwrap.go:14-18` doc comment +
`server.go:244-246` doc comment). TelemetryMiddleware's
`daemon.mcp.tools.call` parent owns all identifying labels.

| Attribute | Source | PII risk | Cardinality | Verdict |
| --------- | ------ | -------- | ----------- | ------- |
| _(none)_  | —      | —        | —           | —       |

Error-path side effects: both paths call `span.RecordError(err)` if
recording (`spanwrap.go:33-35`, `server.go:252-254`). `SetStatus` is
deliberately NOT called — Phase 12 design choice for v1.2 consistency
between kernel and skill paths. PASS.

### Span Event: `ls.request`

A v1.2 timing breadcrumb event on this span (NOT a child span — Phase 12 D-06
froze it as an event). Source: `internal/kernel/lspool/worker.go:322`. Emits
inside an `IsRecording()` gate (line 321) so the tracing-off path allocates
nothing.

| Attribute         | Source                  | PII risk | Cardinality                            | Verdict                                 |
| ----------------- | ----------------------- | -------- | -------------------------------------- | --------------------------------------- |
| `lsp.method`      | `worker.go:323`         | none     | bounded LSP enum (~50 methods)         | FLAG (justified — bounded by LSP spec)  |
| `lsp.language`    | `worker.go:324`         | none     | bounded language registry (~52)        | PASS                                    |
| `lsp.duration_ms` | `worker.go:325` (int64) | none     | continuous numeric (NOT a key explosion) | PASS                                    |

**Reconciliation with `lspool.lsp.{method}` child span (Phase 55):** the
event is the v1.2 timing breadcrumb on the parent kernel.tool span. The new
child span (Phase 55, `Conn.Call`/`Conn.Notify`) is the v1.9 first-class
operation. Both PASS — neither leaks PII or unbounded values, and they
serve different consumer queries (event for in-context aggregate reads on
the kernel-tool trace; child span for a dedicated `lspool.*` filter in
Jaeger). NOT double-instrumentation.

## lspool.lsp.{method}

Produced by `Conn.Call` at `internal/kernel/jsonrpc/conn.go:102` for every
outbound LS request.

| Attribute    | Source             | PII risk | Cardinality                    | Verdict                                |
| ------------ | ------------------ | -------- | ------------------------------ | -------------------------------------- |
| `lsp.method` | `conn.go:103` (Call) | none     | bounded LSP enum (~50 methods) | FLAG (justified — bounded by LSP spec) |

Cardinality discipline (per CONTEXT.md decision block "Cardinality
discipline"): `lsp.language` is intentionally NOT attached here — it lives
on the parent `daemon.mcp.tools.call` span via TelemetryMiddleware.
Re-attaching would duplicate telemetry without adding query power. No
request payload, no file paths, no URIs are exposed.

Error-path side effect (line 144-146): `span.RecordError(resp.Error)` if
recording. Error code/message come from the LS server's JSON-RPC error
object — bounded by the LSP spec's error-code enum and the LS's own error
text (no client input echoed back). PASS.

## lspool.lsp.notify.{method}

Produced by `Conn.Notify` at `internal/kernel/jsonrpc/conn.go:160` for every
outbound LS notification (didOpen, didChange, initialized, etc.).

| Attribute    | Source                | PII risk | Cardinality                    | Verdict                                |
| ------------ | --------------------- | -------- | ------------------------------ | -------------------------------------- |
| `lsp.method` | `conn.go:161` (Notify) | none     | bounded LSP enum (~50 methods) | FLAG (justified — bounded by LSP spec) |

Same cardinality discipline as `lspool.lsp.{method}`. No params, no URIs, no
file paths.

## Verdict Summary

- PASS: 7 attributes (`tool_name`, `profile`, `mode`, `language`, `outcome` on `daemon.mcp.tools.call`; `lsp.language`, `lsp.duration_ms` on `ls.request` event)
- FLAG: 3 attributes (`lsp.method` on the `ls.request` event, `lspool.lsp.{method}`, and `lspool.lsp.notify.{method}` — all justified, all bounded by the LSP spec)
- FAIL: 0 attributes

Phase 55 ships with zero unbounded-cardinality attributes and zero
PII-bearing attributes. Span names follow the `{tier}.{subsystem}.{op}`
convention; no identifiers, file paths, or request bodies appear in any
attribute or span name. OBS-04 success criterion 2 satisfied.

## Maintenance Contract

Adding a new span attribute to any of the spans above requires:

1. Add a row to that span's table in this file with PII risk + Cardinality + Verdict.
2. If the verdict is FAIL, the attribute MUST NOT ship — change the design (drop the attribute, hash it, or move it to a debug-only sink).
3. Bump `last_reviewed` in the frontmatter to today's ISO date.
4. The audit test in `internal/obs/trace_audit_test.go` (Phase 55 Plan 03)
   guards the structural shape of the trace tree (every registered tool
   produces both `daemon.mcp.tools.call` and `kernel.tool.{name}`; every
   `Conn.Call` produces `lspool.lsp.{method}`); it does NOT enumerate every
   attribute. This document is the authoritative attribute-level review.

Adding a new span (not just a new attribute on an existing span) requires
adding a new H2 section here with its own attribute table BEFORE the new
span ships, plus a row in the "Spans Covered" table at the top.

## Code references

- `internal/forwarder/forwarder.go:140` — `forwarder.tools.call` root span (`sendWithSpan`)
- `internal/mcp/middleware.go:316` — `daemon.mcp.tools.call` span (TelemetryMiddleware)
- `internal/mcp/middleware.go:351-357` — TelemetryMiddleware attribute set (`tool_name`, `profile`, `mode`, `language`, `outcome`)
- `internal/kernel/spanwrap.go:30` — `WrapToolSpan` kernel path; D-07 zero-attribute invariant doc comment at line 14-18
- `internal/mcp/server.go:247` — `AddSkillTool` skill path; D-07 zero-attribute invariant doc comment at line 244-246
- `internal/kernel/jsonrpc/conn.go:102` — `lspool.lsp.{method}` (Conn.Call)
- `internal/kernel/jsonrpc/conn.go:160` — `lspool.lsp.notify.{method}` (Conn.Notify)
- `internal/kernel/lspool/worker.go:322` — `ls.request` span event (Phase 12 D-06 frozen)
- `internal/obs/trace_audit_test.go` — automated audit gate (Phase 55 Plan 03)
