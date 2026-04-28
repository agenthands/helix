---
status: pending
phase: 55-obs-trace-coverage-audit
source: [55-VERIFICATION.md]
started: 2026-04-28
updated: 2026-04-28
---

## Current Test

[awaiting human testing]

## Tests

### 1. Smoke trace shows full request path with no orphan spans (criterion #4)

expected: Run the dev-only OTLP collector via the docker recipe in
USAGE.md "Smoke-Testing the Pipeline" (uses
`otel/opentelemetry-collector-contrib` with the `debug` exporter exposing
:4317). Configure Serena with `observability.tracing_endpoint:
127.0.0.1:4317` and `observability.tracing_sample_ratio: 1.0`. Start the
daemon (`SIGTERM`-clean shutdown, never `SIGKILL`). Invoke any MCP tool
that triggers a language-server call (e.g., `find_symbol` against a Go
file, which fans out to gopls). Within ~5 seconds, the collector's
stdout shows a trace tree containing exactly: one
`daemon.mcp.tools.call` parent span (with `tool_name`, `profile`,
`mode`, `language`, `outcome` attributes), one `kernel.tool.find_symbol`
child (zero attributes — D-07), and at least one `ls.request`
grandchild with `lsp.method=textDocument/...`, `lsp.language=go`, and
`lsp.duration_ms` attributes. Every span has a non-zero `Parent.SpanID`
EXCEPT the top-level `daemon.mcp.tools.call`. NO span other than the
root has `Parent.SpanID == 0`. NO span surfaces as an `AddEvent` named
`ls.request` (regression check — events would not appear as nodes in
the trace waterfall).

result: [pending]

### 2. Skill tool produces a `skill.tool.*` child span (criterion #1 end-to-end)

expected: With the same collector running, invoke a skill tool (e.g.,
`memory_write` from the memory skill, or `get_repo_map` from the repomap
skill). The collector shows `daemon.mcp.tools.call` parent +
`skill.tool.<name>` child (e.g., `skill.tool.memory_write`). The
`skill.tool.*` child has ZERO attributes (D-07 — the parent already
carries `tool_name` etc.). NO orphan spans. NO `ls.request` grandchild
unless the skill happens to call into a kernel tool that uses an LS
(memory_write does not; repomap may).

result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps

## Operator Setup Notes

- **Docker required only for the collector.** Serena itself remains a
  single binary with no runtime dependencies — the smoke-test collector
  is a dev-only verification tool, not a runtime requirement (see
  TRACE-AUDIT.md and the user-memory note "Observability stays
  in-binary").

- **Recipe lives in USAGE.md.** Cross-reference USAGE.md
  "Smoke-Testing the Pipeline" for the copy-paste docker run + YAML
  config. Do NOT duplicate the recipe here — single source of truth in
  USAGE.md (`tracing_sample_ratio: 1.0` is mandatory for the smoke
  test; the default `0.0` silently drops every span).

- **`debug` vs `logging` exporter.** The current
  `otel/opentelemetry-collector-contrib` uses `debug` (renamed from
  `logging` in collector v0.86, 2023-09). If your collector image is
  older, swap `debug:` for `logging:` in
  `otel-collector-config.yaml` — the verbosity flag and behaviour are
  identical.

- **Clean shutdown is mandatory.** Send `SIGTERM` (not `SIGKILL`) when
  stopping the daemon. The `BatchSpanProcessor` flushes any queued
  spans during `obs.Provider.ShutdownTracing` (already wired at
  `internal/daemon/shutdown.go:48`) — a kill-9 will lose the smoke
  trace's final batch and produce a confusing "fewer spans than tool
  calls" result.

- **What to capture for `result:`.** Replace `[pending]` with either a
  brief stdout snippet from the collector showing the trace tree, OR
  a short prose description of the observed parent/child structure
  ("daemon.mcp.tools.call → kernel.tool.find_symbol → ls.request
  (lsp.method=textDocument/definition, lsp.language=go), no orphan
  spans"). Redact any incidental file paths or workspace identifiers
  before pasting (T-55-09 in the plan threat model — TRACE-AUDIT.md's
  anti-list means well-behaved Serena should not produce sensitive
  content, but operator-side redaction is the belt-and-braces step).

- **If a span is missing or orphaned**, the most likely causes are:
  (a) `tracing_sample_ratio` left at the default `0.0`,
  (b) collector started after the daemon (so the gRPC export retried
      against a closed connection — restart the daemon after the
      collector is up),
  (c) daemon killed with `SIGKILL` (final batch lost — see above).
  Note the cause in `result:` and re-run before marking the test as
  failing.
