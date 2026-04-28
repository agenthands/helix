---
phase: 55-obs-trace-coverage-audit
reviewed: 2026-04-28
reviewer: automated by 55-02 allowlist test (TestSpanAttributeAllowlist) + human sign-off pending Task 4 HUMAN-UAT
---

# Trace Attribute Audit (OBS-04)

This document is the human-readable certification of Serena's OpenTelemetry
tracing surface. Every span name and every attribute key emitted by the daemon
is enumerated below with a value-shape, cardinality bound, PII risk, and
disposition. The mechanical enforcement of this map lives in
`internal/obs/attribute_allowlist_test.go` (`allowedSpanAttrs`) and
`internal/mcp/attribute_allowlist_integration_test.go`
(`TestSpanAttributeAllowlist`); this file is the human review trail that
parallels those gates.

The audit covers the full span surface as of plan 55-02 — four span names
total, eight allowlisted attribute keys.

## Span Inventory

Coverage was confirmed by `grep -rn 'tracer\.Start\|\.Tracer().Start' internal/`
which returned exactly the four call sites listed below; no other span name is
emitted by daemon code today.

| Span Name | Origin (file:line) | Parent | Lifetime | Notes |
|-----------|--------------------|--------|----------|-------|
| `daemon.mcp.tools.call` | `internal/mcp/middleware.go:203` | none (root) | one per `tools/call` MCP request | Emitted by `TelemetryMiddleware`. Carries the five RED attributes; child spans below carry zero. |
| `kernel.tool.<name>` | `internal/kernel/spanwrap.go:30` (`WrapToolSpan`) | `daemon.mcp.tools.call` | one per kernel tool invocation | Emitted via `WrapToolSpan` registered by `internal/kernel/{symbols,edit,fileops,diag,health,help}/tools.go` (six call sites; ~28 kernel tools share the wrap helper). Zero attributes by D-07. |
| `skill.tool.<name>` | `internal/mcp/server.go` (`wrapSkillToolHandler`, threaded by `AddSkillTool`) | `daemon.mcp.tools.call` | one per skill tool invocation | Emitted by `internal/skill/{memory,workflow,repomap}/*` registrations (11 tools today: memory ×7, workflow ×2, repomap ×2). Zero attributes by D-07. |
| `ls.request` | `internal/kernel/lspool/worker.go` (`Worker.Request`) | `kernel.tool.<name>` (or any caller-supplied parent context) | one per outbound LSP JSON-RPC call | Uniform span name — `lsp.method` is an attribute, not a name suffix, to keep backend cardinality bounded (OQ#2 resolved 2026-04-28). |

## Attribute Certification

### `daemon.mcp.tools.call`

| Attribute Key | Type | Value Shape | Cardinality Bound | PII Risk | Disposition |
|---------------|------|-------------|-------------------|----------|-------------|
| `tool_name` | string | enum sourced from `mcp.ToolRegistry.Names()` | ≤ 50 (registry, currently 41+ tools) | none — registry-bounded internal identifier | ALLOW |
| `profile` | string | enum {`claude-code`, `codex`, `ide-assistant`, `ci-bot`, `full`} | 5 | none — operator-selected profile name | ALLOW |
| `mode` | string | enum {`read`, `edit`, `review`, `admin`} | 4 | none — operator-selected mode name | ALLOW |
| `language` | string | enum sourced from `internal/langregistry` | ≤ 52 | none — language identifier | ALLOW |
| `outcome` | string | enum {`success`, `invalid_args`, `not_found`, `circuit_open`, `ls_crash`, `timeout`, `internal`} (verbatim from `internal/mcp/middleware.go:111-119`) | 7 | none — closed classification enum | ALLOW |

### `kernel.tool.<name>`

ZERO attributes per Phase 12 D-07; the parent `daemon.mcp.tools.call` span
already carries `tool_name`/`profile`/`mode`/`language`/`outcome`. The
allowlist test enforces emptiness via the wildcard entry
`"kernel.tool.*": {}` (the empty map is a sentinel — `nil` would silently skip
enforcement, which is why `TestSpanAllowlistIsExhaustive` exists).

### `skill.tool.<name>`

ZERO attributes per the same D-07 rule. The wrap helper
(`wrapSkillToolHandler`) deliberately does not call `SetAttributes`; the
allowlist test enforces `"skill.tool.*": {}`. The unit test
`TestAddSkillToolNoAttributesOnChild` (added in plan 55-01) is the second
mechanical check that reinforces this invariant on the production code path.

### `ls.request`

| Attribute Key | Type | Value Shape | Cardinality Bound | PII Risk | Disposition |
|---------------|------|-------------|-------------------|----------|-------------|
| `lsp.method` | string | enum from LSP 3.17 method names (e.g., `textDocument/definition`, `textDocument/hover`, `workspace/symbol`) | ≤ ~30 in Serena's usage | none — LSP method identifier (specification-bounded) | ALLOW |
| `lsp.language` | string | enum sourced from `internal/langregistry` | ≤ 52 | none — language identifier | ALLOW |
| `lsp.duration_ms` | int64 | bounded by per-tool deadline (`degradation.timeout_*`, max ~300s) | unbounded continuous numeric — backend-side aggregated to histograms, not stored as a label | none — duration measurement | ALLOW |

## Anti-list (deliberately excluded)

The following attribute keys MUST NEVER be added to ANY span name without a
fresh hygiene review. They are excluded because they introduce PII, unbounded
cardinality, or both. Adding any of these requires a code-review sign-off
plus an update to this section justifying the change:

- `workspace_path` / `repo_root` — PII (operator filesystem layout) AND
  unbounded cardinality (one value per workspace).
- `file_path` / `file_uri` — PII AND unbounded (one value per file).
- `symbol_name` — sometimes PII (proprietary identifiers); unbounded.
- `user_id` / `user_email` / `session_id` — PII; per-user unbounded.
- `request_id` / `trace_id` — duplication of the OTel-native trace ID; the
  trace ID is already part of every span and must not be re-emitted as an
  attribute (would inflate label cardinality on the backend).
- Free-form `error` strings on attribute keys — use `span.RecordError(err)`
  + `span.SetStatus(codes.Error, ...)` instead, which is already the
  standard shape (`internal/mcp/middleware.go:246-247`,
  `internal/kernel/lspool/worker.go` `Request`).

If a future plan needs to surface workspace/file/symbol context, the
canonical mitigation is **bucketing** (e.g., `path.bucket` ∈ {`test`,
`src`, `vendor`, `other`}) or **truncated hashing** — never the raw value.

## Cardinality & PII Certification

As of 2026-04-28, this audit certifies that the four span names above and
their attribute sets contain no personally-identifiable information and have
a total cardinality bounded by the product of fixed enums:

- `daemon.mcp.tools.call` worst case: 50 tools × 5 profiles × 4 modes × 52 languages × 7 outcomes ≈ **364,000** unique parent span fingerprints.
- `kernel.tool.*` and `skill.tool.*`: zero attributes ⇒ cardinality ≤ count of registered tools (≤ 50 names today).
- `ls.request` worst case: 30 LSP methods × 52 languages = **1,560** unique LS span fingerprints (`lsp.duration_ms` is a continuous numeric handled by histogram aggregation, not a cardinality dimension).

Every value above is either a closed enum literal in source code, the
registry-bounded count of registered tools, the langregistry list, or the
LSP 3.17 method namespace. Mechanical enforcement is
`TestSpanAttributeAllowlist` (`internal/mcp/attribute_allowlist_integration_test.go`)
backed by the canonical `allowedSpanAttrs` literal in
`internal/obs/attribute_allowlist_test.go`.

## Trace Context Propagation Policy

Serena does **NOT** honor inbound trace context from MCP clients. All Serena
spans are root spans within the daemon (no `propagation.TraceContext`
extraction is wired into the MCP transport layer). This is a deliberate
property pending V2/V3 (authenticated MCP transport) — propagating
client-supplied trace IDs would let a malicious or curious client correlate
spans across operators' collectors, and without authenticated trust in the
client's trace ID we cannot accept it. Re-evaluate this policy when
authenticated MCP transport lands.

The sampler (`sdktrace.ParentBased(sdktrace.TraceIDRatioBased(ratio))`,
`internal/obs/tracing.go`) is configured with `ParentBased` solely for the
internal parent→child relationships (e.g., `ls.request` inheriting the
sampling decision of its `kernel.tool.*` parent), not for honoring any
external client decision.

## Mechanical Enforcement References

- **Coverage gate:** `internal/mcp/coverage_test.go::TestEveryRegisteredToolEmitsSpans` (plan 55-02) — fails CI when any tool registered via `ToolRegistry.Names()` lacks a `kernel.tool.*` or `skill.tool.*` child span parented to `daemon.mcp.tools.call`.
- **Bootstrap-sanity guard:** `internal/mcp/coverage_test.go::TestRegistryNamesNonEmpty` — prevents the audit from passing vacuously on an empty registry.
- **Attribute allowlist gate (canonical literal + static exhaustiveness):** `internal/obs/attribute_allowlist_test.go::TestSpanAllowlistIsExhaustive` and `TestLookupAllowedSpanAttrsWildcard` (plan 55-02).
- **Attribute allowlist gate (production-shaped emission):** `internal/mcp/attribute_allowlist_integration_test.go::TestSpanAttributeAllowlist` (plan 55-02) — drives all four span shapes against an `InMemoryExporter` and asserts every attribute key is in the allowlist.
- **Span shape regression tests (plan 55-01):** `internal/kernel/lspool/worker_span_test.go` (`TestRequestEmitsChildSpan`, `TestRequestSpanRecordsError`, `TestRequestSpanShape`); `internal/mcp/skill_tool_span_test.go` (`TestAddSkillToolEmitsChildSpan`, `TestAddSkillToolRecordsError`, `TestAddSkillToolNoAttributesOnChild`).
- **Source of truth for the sampler:** `internal/obs/tracing.go` (`ParentBased(TraceIDRatioBased(ratio))`).
- **Source of truth for shutdown flush:** `internal/daemon/shutdown.go:48` (`d.obs.ShutdownTracing(flushCtx)` — required for `BatchSpanProcessor` to drain before the process exits; relevant to the smoke-trace UAT in `55-HUMAN-UAT.md`).

## Update Procedure

If you add a span attribute, you MUST:

1. Update `allowedSpanAttrs` in `internal/obs/attribute_allowlist_test.go`
   AND `allowedSpanAttrsIntegration` in
   `internal/mcp/attribute_allowlist_integration_test.go` (the double-entry
   book — divergence shows up as a test failure).
2. Add a row to the appropriate "Attribute Certification" subsection here
   in TRACE-AUDIT.md including PII assessment, cardinality bound, and a
   one-line justification.
3. Get a code-review sign-off from a second reviewer (this doc is the
   review trail).

If you add a new span name:

1. Add it to BOTH allowlist literals (use the `.*` suffix only for per-tool
   span families — see existing `kernel.tool.*` / `skill.tool.*` entries).
2. Add a row to the "Span Inventory" table above and a new subsection under
   "Attribute Certification" — even if the attribute set is empty
   (explicitly state "ZERO attributes by D-07" so the intent is captured).
3. Confirm the coverage test (`TestEveryRegisteredToolEmitsSpans`) still
   passes — adding a new wrap helper without exercising a tool through it
   silently makes the audit weaker.

The CI gates from plan 55-02 will reject any change to the attribute
surface that bypasses step 1; this document keeps the human review trail
for steps 2 and 3.
