---
status: testing
phase: 79-evaluators-result-schema-metrics-layer
source: [79-VERIFICATION.md]
started: 2026-06-18T16:20:00Z
updated: 2026-06-18T16:20:00Z
---

## Current Test

number: 1
name: Jaeger import of the merged trace.json (METRIC-06 visual continuity)
expected: |
  A single trace from the bench.run_index/run_id root down to LSP leaves, with no
  orphan spans and no cross-cell PID leakage.
awaiting: user response

## Tests

### 1. Jaeger import of the merged trace.json (METRIC-06 visual continuity)
expected: |
  Import a merged trace.json (e.g. `bench/reports/<ts>/IT-go-patch-apply-1/your_agent_full/0/trace.json`)
  into Jaeger. You should see a single trace from the `bench.run_index`/`run_id` root down to
  LSP leaves, with no orphan spans and no cross-cell PID leakage.
why_human: |
  Jaeger UI import is a visual confirmation. The structural invariants behind it —
  single `trace.Merge` per cell, `tool_call_summary.total` parity with `tool_calls`,
  `rejected_foreign_pid` clear — are automated and PASS, but the end-to-end visual
  root→leaf continuity is a manual check per 79-VALIDATION.md § Manual-Only Verifications.
result: [pending]

## Summary

total: 1
passed: 0
issues: 0
pending: 1
skipped: 0
blocked: 0

## Gaps
