---
phase: 55-obs-trace-coverage-audit
plan: 04
subsystem: observability
tags: [observability, tracing, audit, documentation, hygiene-review]
requires: [55-01, 55-02, 55-03]
provides: [trace-attribute-hygiene-review]
affects: [.planning/phases/55-obs-trace-coverage-audit/]
tech_stack_added: []
patterns: [per-span-h2-hygiene-review, pass-flag-fail-verdict-legend]
files_created:
  - .planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md
files_modified: []
decisions:
  - per-span H2 layout (one section per span name) with attribute table per CONTEXT.md "TRACE-AUDIT.md Format" decision
  - dual representation of LS observability (ls.request event + lspool.lsp.{method} child span) explicitly reconciled with PASS verdicts
  - zero FAIL attributes shipped — only PASS and LSP-spec-bounded FLAG verdicts
metrics:
  duration_seconds: 114
  completed: 2026-05-02
  tasks_completed: 1
  files_changed: 1
---

# Phase 55 Plan 04: TRACE-AUDIT.md Hygiene Review Summary

Authored `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` — the per-span attribute hygiene review that closes OBS-04 success criterion 2 ("a review artifact lists every span attribute and certifies: no PII, no unbounded cardinality").

## What shipped

A 193-line documentation artifact with:

- A "Spans Covered" master table (six entries: 5 spans + the `ls.request` event reconciliation note).
- A "Verdict Legend" defining PASS / FLAG / FAIL semantics consistent with CONTEXT.md.
- Per-span H2 sections for `forwarder.tools.call`, `daemon.mcp.tools.call`, `kernel.tool.{name}` (kernel + skill paths in one section), `lspool.lsp.{method}`, and `lspool.lsp.notify.{method}`.
- Inside the `kernel.tool.{name}` section, a `### Span Event: ls.request` subsection that reconciles the v1.2 timing breadcrumb event with the new v1.9 first-class child span (both PASS, justified dual representation).
- A "Verdict Summary" with explicit attribute counts.
- A "Maintenance Contract" instructing future contributors how to extend the document when adding a new attribute or span.
- A "Code references" footer with file:line citations for every span definition site.

## Final attribute counts

| Verdict | Count | Where                                                                                                                                                                                     |
| ------- | ----- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| PASS    | 7     | `tool_name`, `profile`, `mode`, `language`, `outcome` on `daemon.mcp.tools.call`; `lsp.language`, `lsp.duration_ms` on the `ls.request` event                                              |
| FLAG    | 3     | `lsp.method` on the `ls.request` event, `lspool.lsp.{method}`, and `lspool.lsp.notify.{method}` — all three bounded by the LSP spec (~50 methods); information-rich but not unbounded     |
| FAIL    | 0     | —                                                                                                                                                                                         |

`forwarder.tools.call` and both `kernel.tool.{name}` paths emit zero attributes by design (Phase 12 D-07: TelemetryMiddleware owns identifying labels). Those are recorded in the doc as table rows with em-dashes plus a rationale paragraph rather than counted in the totals.

## Discoveries vs. CONTEXT / RESEARCH

No surprises. Reading source-of-truth files line-by-line confirmed:

1. **`daemon.mcp.tools.call` attribute set is exactly the five CONTEXT-listed keys** — `tool_name`, `profile`, `mode`, `language`, `outcome` (verified at `internal/mcp/middleware.go:351-357`). No previously-undocumented attributes.
2. **Both `kernel.tool.{name}` emitters are zero-attribute** — `WrapToolSpan` at `internal/kernel/spanwrap.go:30` and `AddSkillTool` at `internal/mcp/server.go:247`. The skill-path doc comment at `server.go:244-246` explicitly cites D-07. So the audit can group them under one H2 section.
3. **`ls.request` event has exactly three attributes** — `lsp.method`, `lsp.language`, `lsp.duration_ms` (verified at `internal/kernel/lspool/worker.go:322-326`). Matches Pitfall 4's reconciliation requirement.
4. **`lspool.lsp.{method}` and `lspool.lsp.notify.{method}` each carry only `lsp.method`** — verified at `internal/kernel/jsonrpc/conn.go:102-103` and `:160-161`. Matches CONTEXT's "cardinality discipline: language is intentionally NOT attached here".
5. **Forwarder root span is zero-attribute** — verified at `internal/forwarder/forwarder.go:140` (`tracer.Start(ctx, "forwarder.tools.call")` with no `WithAttributes`). The audit acknowledges this as deliberate.

The line numbers in the resulting TRACE-AUDIT.md are the current line numbers at HEAD, all spot-checked via `sed -n` post-write.

## Sanity checks (executed before commit)

- `wc -l TRACE-AUDIT.md` → 193 (≥120 required by acceptance).
- `grep -cE '^## (forwarder\.tools\.call|daemon\.mcp\.tools\.call|kernel\.tool\.\{name\}|lspool\.lsp\.\{method\}|lspool\.lsp\.notify\.\{method\})'` → 5 (one H2 per span).
- `grep -F '### Span Event: \`ls.request\`'` → matches.
- `grep -F 'FAIL: 0'` → matches.
- `grep -E '\| FAIL \|'` → no matches (zero FAIL verdicts in any row).
- Spot-checked 7 cited file:line pairs (`forwarder.go:140`, `middleware.go:316`, `conn.go:102`, `conn.go:160`, `worker.go:322`, `server.go:247`, `spanwrap.go:30`); all resolve to the expected source line.

## Deviations from Plan

None — plan executed exactly as written.

## Commits

- `71aded95` — `docs(55-04): author TRACE-AUDIT.md per-span attribute hygiene review`

## Self-Check: PASSED

- File `.planning/phases/55-obs-trace-coverage-audit/TRACE-AUDIT.md` exists.
- Commit `71aded95` exists in `git log --oneline`.
- All seven cited file:line pairs in TRACE-AUDIT.md "Code references" resolve to the expected source content.
