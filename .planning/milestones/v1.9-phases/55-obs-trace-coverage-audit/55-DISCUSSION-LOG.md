---
phase: 55-obs-trace-coverage-audit
gathered: 2026-05-01
mode: discuss (interactive, --interactive autonomous)
---

# Phase 55: Discussion Log

Human-reference record of the discuss-phase session. Not consumed by downstream agents — they read CONTEXT.md.

## Gray Areas Selected by User

User selected all 4 of 4 candidate gray areas (sampling-doc scope deferred to Claude's discretion).

## Q1: Audit mechanism

**Options presented:**
1. Go test in `internal/obs/` — registry-driven validator, fail-closed (Recommended)
2. CLI command `helix trace-audit` — runtime check, ops-flavored
3. Hybrid: test + CLI — both surfaces

**User selected:** Go test in `internal/obs/` (Recommended)

**Notes:** Mirrors Phase 54 `dashboards_test.go` pattern. Runs in default `go test ./...`, not `//go:build integration`.

---

## Q2: LS-call span coverage

**Options presented:**
1. At `jsonrpc.Conn.Call` layer in `internal/kernel/jsonrpc/codec.go` (Recommended)
2. At Worker.Call (per-LS in lspool)
3. At individual call sites in symbols/edit/diag

**User selected:** At `jsonrpc.Conn.Call` layer (Recommended)

**Notes:** Single insertion point covers every outbound LS request. Span name `lspool.lsp.{method}`. Notifications get symmetric `lspool.lsp.notify.{method}` wrapper. Only attribute is `lsp.method` (bounded LSP enum).

---

## Q3: TRACE-AUDIT.md format

**Options presented:**
1. Per-span table — H2 per span name, attribute table inside (Recommended)
2. Attribute-index — one row per unique attribute name across all spans
3. Both — index up top + per-span detail below

**User selected:** Per-span table (Recommended)

**Notes:** Mirrors how a reviewer mentally walks a trace tree. Acceptable repetition of common attributes (e.g., `tool_name`) across spans. Cardinality verdict: PASS / FLAG / FAIL.

---

## Q4: Smoke-trace artifact

**Options presented:**
1. `docs/runbooks/trace-smoke.md` runbook + Jaeger screenshot (Recommended)
2. Scripted smoke test against in-process OTLP collector (testcontainers)
3. Checked-in JSON span dump

**User selected:** Runbook + Jaeger screenshot (Recommended)

**Notes:** Matches Phase 54-05 runbook + screenshot pattern. Manual checkpoint in executor's task list (placeholder image first, real capture on completion). No new test infra (testcontainers) added.

---

## Claude's Discretion (not asked)

- **Sampling doc scope:** Document only what ships (head-based `ParentBased(TraceIDRatioBased(...))`), one-line note that tail-sampling is future scope.
- **USAGE.md insertion point:** New H3 `### Trace Sampling` BEFORE existing `### Prometheus Metrics` (mirrors Phase 54-05 pattern).
- **Cardinality discipline on lspool span:** ONLY `lsp.method` attribute. Language already on parent kernel.tool span via TelemetryMiddleware — do NOT re-attach.

## Deferred Ideas

- Tail-sampling (own phase if/when needed)
- CLI helix trace-audit (oncall demand-driven)
- Scripted in-process OTLP smoke test (replace manual runbook if it proves lossy)
- Tracing for non-MCP/LS surfaces (HTTP, channels, RepoMap)
- Span links between sibling LS calls

## Scope Creep Avoided

User did not propose scope creep. All 4 selected areas were on-domain (audit + close known gap + document existing pipeline).
