---
phase: 55-obs-trace-coverage-audit
gathered: 2026-05-01
status: ready_for_research
mode: discuss (interactive)
---

# Phase 55: obs-trace-coverage-audit — Context

<domain>
## Phase Boundary

Audit the existing OTel tracing pipeline (built in Phase 12) for coverage and hygiene, close the one known gap (outbound LS JSON-RPC calls lack their own child spans), publish a span-attribute hygiene review, document sampling configuration, and capture a smoke-trace artifact proving the full request path renders correctly in a real OTLP collector.

**Requirement satisfied:** OBS-04.

**In scope (HOW questions to answer):**
1. Adding span coverage at `internal/kernel/jsonrpc/codec.go` Call() so every outbound LS request gets a `lspool.lsp.{method}` child span.
2. A registry-driven Go test asserting every MCP tool emits `daemon.mcp.tools.call` (middleware) AND `kernel.tool.{name}` (kernel WrapToolSpan).
3. `TRACE-AUDIT.md` per-span hygiene review (PII + cardinality verdict per attribute).
4. USAGE.md Observability section documenting `ObservabilityConfig.{TracingEndpoint, TracingSampleRatio, ServiceName}` and the head-based `ParentBased(TraceIDRatioBased(...))` sampler.
5. `docs/runbooks/trace-smoke.md` runbook + one Jaeger screenshot at `docs/images/trace-smoke-jaeger.png` proving the live OTLP collector path.

**Out of scope (must NOT be added in Phase 55):**
- Tail-sampling implementation (head-only ships today; tail is a future phase if needed).
- Tracing for new subsystems beyond MCP tools + outbound LS calls (e.g., HTTP transport spans, internal channel telemetry).
- Reworking the existing Phase 12 TracerProvider construction or OTLP exporter wiring.
- Adding new span attributes for their own sake — additions only happen if the audit identifies a gap.

</domain>

<decisions>
## Implementation Decisions

### Audit Mechanism
**Decision:** Go test (`internal/obs/trace_audit_test.go`) reading the tool registry, mirroring the Phase 54 `dashboards_test.go` pattern.

- Registry-driven: enumerate `mcpsdk.Server.Tools()` (or equivalent kernel tool registry) at test time.
- Fail-closed: any registered tool that does not produce both `daemon.mcp.tools.call` AND `kernel.tool.{name}` spans on invocation against a `tracetest.InMemoryExporter` fails the test.
- Outbound LS coverage: the same test asserts that triggering a representative LS call (via a fake/loopback Conn) produces a `lspool.lsp.{method}` span as a child of the kernel.tool span.
- Build-tag: same as `dashboards_test.go` — runs in default `go test ./...`, not behind `//go:build integration`.

**Why:** Reuses the proven Phase 54 registry-driven validator pattern; runs in CI on every push without extra plumbing; cannot drift silently because the test fails when a new tool is added without span coverage.

**Rejected:** CLI command (`helix trace-audit`) — would duplicate test infrastructure for ops-time runtime check that isn't a current need; can be added later if oncall requests it. Hybrid (test + CLI) — same reason, plus doubled cost.

### LS-Call Span Coverage
**Decision:** Wrap at the `jsonrpc.Conn.Call` layer in `internal/kernel/jsonrpc/codec.go`.

- Single insertion point in Call() so every outbound LS request automatically gets a child span named `lspool.lsp.{method}`.
- Span attributes: `lsp.method` (bounded enum from LSP spec — known finite cardinality). NO request payload, NO file-path strings as attributes. NO arbitrary URIs (use a hash if any path-like value must be on the span).
- Notifications: `Conn.Notify` gets the same wrapper symmetric with Call (named `lspool.lsp.notify.{method}`) — the Phase 56 dispatcher path is the inbound side, not relevant here.
- The wrapper is no-op when no parent span is on ctx (i.e., the existing `Noop()` path in `internal/obs/tracing.go` already gives `tracer.Start` zero-allocation behavior when tracing is off — D-17 of tracing.go).

**Why:** Single point of coverage; no per-call-site sprinkling; LSP method names are a bounded enum so cardinality is safe; matches the existing `WrapToolSpan` wrapping pattern (one helper, applied at registration time, individual handlers untouched).

**Rejected:** Worker.Call wrapping — would miss notifications dispatched through the Conn loop; Worker is also one layer above the actual JSON-RPC frame, so attributes like `ls_method` would be less natural to attach there. Per-call-site wrapping — high churn, easy to forget when adding new tools.

**Cardinality discipline:** `lsp.method` is the ONLY high-information attribute on the lspool span. `language` is already on the parent kernel.tool span via TelemetryMiddleware — do NOT re-attach.

### TRACE-AUDIT.md Format
**Decision:** Per-span table format. One H2 section per span name; each section has a table:

```
| Attribute       | Source                     | PII risk | Cardinality                | Verdict |
|-----------------|----------------------------|----------|----------------------------|---------|
| tool_name       | TelemetryMiddleware        | none     | bounded (registered tools) | PASS    |
| profile         | TelemetryMiddleware        | none     | enum (5 profiles)          | PASS    |
| outcome         | TelemetryMiddleware        | none     | enum (success/timeout/...) | PASS    |
```

Spans covered (initial set — researcher verifies completeness):
- `daemon.mcp.tools.call` (TelemetryMiddleware)
- `kernel.tool.{name}` (WrapToolSpan, one section per tool family — group find_references, goto_definition, etc. into "kernel.tool.{symbols-family}" if attribute set is identical, otherwise itemize)
- `lspool.lsp.{method}` (new in this phase)
- Any other spans the audit test discovers (e.g., daemon-level startup spans if they exist)

**Why:** Per-span structure mirrors how a reviewer would mentally walk a trace tree. Auditor can spot-check one span at a time. Some attribute repetition (e.g., `tool_name` appears under both daemon.mcp.tools.call and kernel.tool.{name}) is acceptable — clarity over dedup.

**Rejected:** Attribute-index format — compact but loses span-tree shape; reviewer can't tell at a glance which attributes co-occur on which span. Both — overhead not justified for a one-time audit.

**Cardinality verdict guidance:** PASS = bounded enum or registered/deterministic-set; FLAG = high-cardinality but justified (e.g., `lsp.method` is bounded by LSP spec but >50 distinct values); FAIL = unbounded (file paths, user input, IDs).

### Smoke-Trace Artifact
**Decision:** `docs/runbooks/trace-smoke.md` runbook + one Jaeger screenshot at `docs/images/trace-smoke-jaeger.png`.

- Runbook procedure: spin up Jaeger via `docker run -p 4317:4317 -p 16686:16686 jaegertracing/all-in-one`; start helix with `--config` pointing to a config that sets `observability.tracing_endpoint=localhost:4317` and `observability.tracing_sample_ratio=1.0`; trigger a representative tool flow (e.g., `goto_definition` from any client); open Jaeger UI, expand the trace, verify `daemon.mcp.tools.call` → `kernel.tool.goto_definition` → `lspool.lsp.textDocument/definition` chain renders with no orphan spans.
- Screenshot is a one-time capture at the time of phase completion; the runbook is the durable artifact (anyone can re-run it).
- Manual checkpoint in the executor's task list — same pattern as Phase 54-05's `helix-overview-dashboard.png` placeholder.

**Why:** Cheap to capture, durable as documentation, no new test infrastructure (testcontainers + OTLP collector mocks) added to the suite. Aligns with the Phase 54 runbook + screenshot pattern, which the reviewer already knows.

**Rejected:** Scripted smoke test against local collector — adds testcontainers dep and an OTLP-receiver mock to maintain; runtime cost in CI; the Phase 12 unit tests already cover `WithTracing` correctness with `tracetest.InMemoryExporter`. Checked-in JSON span dump — large, frozen-in-time, no reproducibility past the moment of capture.

### Sampling Documentation Scope (Claude's discretion)
**Decision:** USAGE.md gets a new H3 `### Trace Sampling` subsection under the existing Observability H2.

- Document: head-based `ParentBased(TraceIDRatioBased(SampleRatio))` from `internal/obs/tracing.go:65-67`. Default `tracing_sample_ratio: 0.0` = off; `1.0` = sample everything.
- Document: the noop short-circuit when `tracing_endpoint` is empty (D-11 / D-17: zero allocation when off).
- Document: how to set the env var / config key for ops (mirrors how Phase 54-05 documented metrics).
- Out of scope: tail-sampling. Add a one-line note: "Tail-sampling is not implemented today; see future-work backlog if downstream filtering of expensive operations becomes necessary."

</decisions>

<canonical_refs>
## Canonical References

Downstream agents (researcher, planner, executor) MUST read these:

- `.planning/ROADMAP.md` — Phase 55 success criteria (must remain authoritative)
- `.planning/REQUIREMENTS.md` — OBS-04 row
- `.planning/phases/53-obs-metrics-gaps/53-04-SUMMARY.md` — Prior art for registry-driven obs validators (informs the audit-mechanism pattern)
- `.planning/phases/54-obs-dashboards-runbooks/54-01-SUMMARY.md` — `dashboards_test.go` validator pattern that 55's audit test mirrors directly
- `.planning/phases/54-obs-dashboards-runbooks/54-04-SUMMARY.md` — Runbook authoring pattern (frontmatter, H2 order, "## Code references" footer)
- `.planning/phases/54-obs-dashboards-runbooks/54-05-SUMMARY.md` — USAGE.md insertion pattern + manual screenshot checkpoint
- `internal/obs/tracing.go` — Existing Phase 12 TracerProvider; D-01/D-09/D-11/D-17 design rules at file head MUST be honored (no `otel.SetTracerProvider`, degraded-optional, noop on empty endpoint, hot-path budget)
- `internal/kernel/spanwrap.go` — Existing `WrapToolSpan` helper + D-07 ("NO attributes on kernel spans — TelemetryMiddleware owns those")
- `internal/mcp/middleware.go` — `TelemetryMiddleware` `daemon.mcp.tools.call` span at line 316; attribute set is the source of truth for the hygiene review
- `internal/kernel/jsonrpc/codec.go` — Insertion target for new lspool.lsp.{method} wrapper; planner verifies no existing span code there
- `internal/config/config.go:37-58` — `ObservabilityConfig` struct (Tracing fields)
- `internal/obs/dashboards_test.go` — Pattern for registry-driven, fail-closed validator the audit test mirrors

</canonical_refs>

<code_context>
## Existing Code Insights

**Tracing pipeline already exists from Phase 12:**
- `internal/obs/tracing.go`: `TracerProvider` constructed via `WithTracing(inner, cfg, logger)`; OTLP/gRPC exporter; `ParentBased(TraceIDRatioBased(ratio))` sampler; noop fallback (D-09 degraded-optional). NO `otel.SetTracerProvider` call (D-01).
- `internal/kernel/spanwrap.go`: `WrapToolSpan[In, Out]` wrapper applied at `RegisterTools` time; produces `kernel.tool.{name}` span; NO attributes (D-07 — TelemetryMiddleware owns attributes).
- `internal/mcp/middleware.go:316`: `TelemetryMiddleware` produces `daemon.mcp.tools.call` span with attributes (`tool_name`, `profile`, `mode`, `language`, `outcome`, ...).
- `internal/config/config.go:37-58`: `ObservabilityConfig.{TracingEndpoint, TracingSampleRatio, ServiceName}`.

**Coverage gap confirmed by quick survey:**
- `grep -rn 'tracer\.Start\|StartSpan' --include='*.go' internal/kernel/jsonrpc/` → zero hits. Outbound LS JSON-RPC calls have NO span coverage today.
- This is the primary code change in Phase 55.

**Reusable validator pattern (Phase 54):**
- `internal/obs/dashboards_test.go` enumerates dashboards via prom registry, asserts every metric has a panel, fail-closed. Phase 55's `trace_audit_test.go` is the same shape against the tool registry + tracetest in-memory exporter.

**Documentation patterns to mirror:**
- Phase 54-04: runbook frontmatter (severity, on-call team, last-updated), H2 order (Symptoms → Diagnosis → Mitigation → Code references → Related runbooks).
- Phase 54-05: USAGE.md H3 subsection insertion before existing "### Prometheus Metrics".

</code_context>

<specifics>
## Specific Requirements

1. **Audit test must be in default `go test ./...`** — NOT behind `//go:build integration`. CI gates on it.
2. **Span attributes on the new `lspool.lsp.{method}` span are LIMITED to:** `lsp.method` (bounded LSP enum). NO file paths, NO request bodies, NO arbitrary URIs. Hash if absolutely needed.
3. **TRACE-AUDIT.md MUST be located at:** `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` (success criterion 2 specifies this exact path).
4. **USAGE.md Observability section:** insert `### Trace Sampling` BEFORE the existing `### Prometheus Metrics` H3 (mirrors the Phase 54-05 insertion pattern).
5. **Smoke runbook lives at:** `docs/runbooks/trace-smoke.md` (matches Phase 54-04 runbook directory). Screenshot at `docs/images/trace-smoke-jaeger.png`.
6. **Phase 12 design invariants are non-negotiable:** D-01 (no `otel.SetTracerProvider`), D-07 (no attributes on `kernel.tool.{name}` spans), D-09 (degraded-optional), D-11 (noop on empty endpoint), D-17 (hot-path budget — zero allocation when tracing off).

</specifics>

<deferred>
## Deferred Ideas

- **Tail-sampling** — head-based ratio is the only sampler shipped today. Tail-sampling (drop traces with all-success spans, keep error traces) is a real ops need but its own phase. Note in USAGE.md as future scope.
- **CLI helix trace-audit command** — runtime audit subcommand for ops. Not needed today (the unit test covers build-time enforcement); revisit if oncall asks.
- **Scripted smoke test against in-process OTLP collector** — would replace the manual runbook + screenshot. Higher-fidelity but adds testcontainers dep. Revisit if the manual runbook proves too lossy.
- **Tracing for non-MCP/LS surfaces** — HTTP transport, channel telemetry, RepoMap extraction. Each is a future phase; Phase 55 audits the contract surfaces only.
- **Span links between sibling LS calls** — e.g., when a single tool dispatches multiple LS requests, linking them. Possible nicety; not in OBS-04.

</deferred>

---

*Generated: 2026-05-01 via /gsd-discuss-phase 55 (interactive mode, --interactive autonomous)*
