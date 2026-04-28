---
status: partial
phase: 54-obs-inbinary-page
source: [54-VERIFICATION.md]
started: 2026-04-28
updated: 2026-04-28
---

## Current Test

[awaiting human testing]

## Tests

### 1. Browser smoke test of in-binary metrics page
expected: Start the daemon with `--admin-addr 127.0.0.1:9100`, open `http://127.0.0.1:9100/` in a browser. Page renders all seven sections (tool calls + p95, edit outcomes, lspool workers, eviction reasons, repomap cache, RSS, goroutines) with inline CSS and no broken layout. Pressing F5 re-renders from the live registry.
result: [pending]

### 2. Runbook readability for unfamiliar operators
expected: An operator unfamiliar with Serena internals can follow each of the four runbooks (ErrCircuitOpen, deadline-timeouts, ls-crash-restart, memory-pressure-eviction) end-to-end using only the symptoms/inspect/triage/remediate sections — no Grafana, no PromQL, no external services required.
result: [pending]

## Summary

total: 2
passed: 0
issues: 0
pending: 2
skipped: 0
blocked: 0

## Gaps
