---
status: complete
plan: 260511-gpq-fix-f-07-wire-subprocess-eval-daemon-tap
completed_at: 2026-05-12
duration_min: ~150
commits:
  - "e7bc406f fix(F-07): emit tap-compatible \"tool call\" JSONL from TelemetryMiddleware"
  - "ab8ad6cb fix(F-07): wire sandbox.StartDaemon in Runner.RunTask + integration regression test"
  - "a928acbe fix(F-08): emit \"receipt issued\" JSONL from guardrails.Store + tap+merge consumers"
  - "93426cc4 fix(F-09): derive DiagnosticsClean from daemon-tap evidence (no more TestsPass tautology)"
  - "25b4b60d fix(F-10): compute ContextPrecision/Recall from MergedTrace + expected_tools.yaml"
  - "31c727c3 fix(F-01): populate FileFactDiffRecorder from production (best-effort, graph_version now advances)"
  - "a432a01a fix(F-05): lazy-init activation also schedules initial extraction"
  - "8dab9b75 ci(F-06): warn-only smoke for helix upgrade daemon-detect short-circuit"
  - "caaf4cf7 docs(F-58): verify PROJECT.md post-phase sweep — minisign + PKG-DEFER-03/04/05 confirmed won-t-do"
  - "ba5d3643 docs(v1.10): close-out audit — verdict PRODUCTION-READY, all 8 findings resolved"
findings_closed:
  - F-07 (BLOCKER)
  - F-01 (HIGH, best-effort with DEF-67-F01-FULL-DIFF)
  - F-08 (MEDIUM)
  - F-09 (LOW)
  - F-10 (LOW)
  - F-05 (LOW)
  - F-06 (LOW)
  - F-58 (MEDIUM tech-debt)
deferred:
  - DEF-67-F01-FULL-DIFF (full-precision FileFactDiff — Tier-3 synthetic marker ships active)
out_of_scope_dispositions:
  - F-03 (4 vet analyzers wired via make vet @ 280a6ef9; cosmetic CI-gate split is a follow-up)
  - F-11 (Makefile fixture-count reconciled @ 00154e5c)
verification:
  go_vet: clean
  go_test_internal: passing
  eval_quick: 36/36 tasks succeeded
audit_verdict: PRODUCTION-READY
---

# 260511-gpq-fix-f-07-wire-subprocess-eval-daemon-tap — SUMMARY

## What shipped

Ten atomic commits closing the 8 substantive findings from
`.planning/v1.10-MILESTONE-AUDIT.md`. v1.10 is now PRODUCTION-READY.

## Commit-by-commit

| # | Commit | Finding | Headline |
|---|--------|---------|----------|
| 1 | `e7bc406f` | F-07 leg A | `TelemetryMiddleware` emits tap-compatible `"tool call"` JSONL with the 7 required fields (tool, outcome, duration_ms int64, pid, trace_id, session_id, guardrail) |
| 2 | `ab8ad6cb` | F-07 leg B | `Runner.RunTask` wires `sandbox.StartDaemon`; integration regression test (`daemon_tap_integration_test.go`) boots a real daemon and asserts `MergedTrace.ToolCallSummary.Total >= 1` |
| 3 | `a928acbe` | F-08 | `guardrails.Store.Issue` emits `"receipt issued"` JSONL; tap+merge populate `MergedTrace.Guardrails.ReceiptsIssued` |
| 4 | `93426cc4` | F-09 | `DiagnosticsClean` derived from `merged.ToolCallSummary.ByOutcome` (`ls_crash` + `internal` counters), with `TestsPass` fallback only on empty tap |
| 5 | `25b4b60d` | F-10 | `report.ComputeContextMetrics` populates `ContextPrecision`/`Recall` from `MergedTrace` + `expected_tools.yaml`; nil only when no ground truth |
| 6 | `31c727c3` | F-01 | Three-tier production populator in `internal/semantic/live/handler/difffacts.go`; Tier-3 synthetic marker ships active and guarantees `graph_version` advances |
| 7 | `a432a01a` | F-05 | `lazyActivateFn` in `daemon.go` mirrors `SetActivateCallback`'s `ScheduleInitialExtraction` call with `Reason=workspace_activation_lazy` |
| 8 | `8dab9b75` | F-06 | Warn-only CI step in `go-test.yml` builds helix, sets `HELIX_RUNNING_AS_DAEMON=1`, runs `helix upgrade --check`, asserts the daemon-detect short-circuit message |
| 9 | `caaf4cf7` | F-58 | PROJECT.md hygiene sweep verified — minisign + PKG-DEFER-03/04/05 confirmed won't-do per Phase 58 D-01/D-02 |
| 10 | `ba5d3643` | audit | `.planning/v1.10-MILESTONE-AUDIT.md` refreshed — `status: resolved`, `verdict: PRODUCTION-READY`, 8 entries in `resolved:` block with real SHAs |

## Diff stats (cumulative across the 10 commits)

Files modified:

| file | nature |
|------|--------|
| `internal/mcp/middleware.go` | +31/-2 — `"tool call"` JSONL emission + `traceIDFromCtx` helper |
| `internal/eval/runner/runner.go` | wire `StartDaemon`, `profileForMode`, `diagnosticsCleanFromTrace`, F-10 metrics |
| `internal/eval/runner/daemon_tap_integration_test.go` | NEW — F-07 regression guard, ~200 lines |
| `internal/eval/runner/runner_diagnostics_test.go` | NEW — F-09 unit tests |
| `internal/eval/sandbox/sandbox.go` | `--http-addr=` + `--json` flags so per-mode daemons don't collide on :8080 and emit JSONL |
| `internal/eval/trace/schema.go` | `KindReceiptIssued` enum + `ReceiptClass` Event field |
| `internal/eval/trace/tap.go` | switch on `msg` ("tool call" \| "receipt issued"), both PID-gated |
| `internal/eval/trace/merge.go` | aggregate `KindReceiptIssued` into `Guardrails.ReceiptsIssued` |
| `internal/eval/trace/tap_test.go` | `TestTapDaemonLog_ReceiptIssued` |
| `internal/eval/report/eval_result.go` | doc comment update — TODO removed |
| `internal/eval/report/context_metrics.go` | NEW — `ComputeContextMetrics` |
| `internal/eval/report/context_metrics_test.go` | NEW — 4-case unit test |
| `internal/eval/score/rules.go` | `ExpectedToolNames()`, package-level `IsRelevant` |
| `internal/guardrails/store.go` | call `emitReceiptIssued` in success path of `Store.Issue` |
| `internal/guardrails/store_jsonl.go` | NEW — `SetJSONLLogger` + `emitReceiptIssued` |
| `internal/semantic/live/handler/handler.go` | invoke `populateRecorderForFile` when no test seam |
| `internal/semantic/live/handler/difffacts.go` | NEW — three-tier populator (Tier 3 active) |
| `internal/semantic/live/handler/recorder_test.go` | empty-populator seam for short-circuit test; new production-path test |
| `internal/daemon/daemon.go` | `guardrails.SetJSONLLogger(logger)` + F-05 ScheduleInitialExtraction parity |
| `.github/workflows/go-test.yml` | F-06 warn-only smoke step |
| `.planning/PROJECT.md` | F-58 clarifying parenthetical + last-hygiene-sweep stamp |
| `.planning/deferred-items.md` | DEF-67-F01-FULL-DIFF entry |
| `.planning/v1.10-MILESTONE-AUDIT.md` | full refresh — PRODUCTION-READY |

## Sample observability evidence (post-fix)

Excerpt from a real `daemon.log` produced during the F-07 integration test
(`internal/eval/runner/daemon_tap_integration_test.go`):

```jsonl
{"time":"...","level":"INFO","msg":"tool call","tool":"get_health","outcome":"success","duration_ms":0,"pid":67635,"trace_id":"","session_id":"","guardrail":null}
```

That same line, parsed by `trace.TapDaemonLog`, lands as a `KindToolCall`
`Event` and feeds `MergedTrace.ToolCallSummary.Total >= 1` —
the regression-guard assertion the new integration test pins.

## `make eval-quick` result

```
helix-eval quick complete: 36/36 tasks succeeded
Reports written to: eval/reports/20260512T095602Z
```

No regressions from the 10 commits.

## Deferred / out-of-scope

- **DEF-67-F01-FULL-DIFF** (full-precision FileFactDiff). Tier-3 synthetic
  marker ships active and guarantees the load-bearing F-01 contract
  (`graph_version` advances on every live edit). Tier 1 (full diff) and
  Tier 2 (added-only) require a snapshot-store prior-FileFact accessor +
  per-file extractor that don't yet exist. Cost: ~1-2 engineer-days when
  scheduled. Trigger: a rank-aware MCP tool consumer reports stale scores
  after a live edit.
- **F-03** — 4 vet analyzers are wired via `make vet` (commit `280a6ef9`).
  Splitting into a distinct CI gate is cosmetic; listed in
  `.planning/v1.10-MILESTONE-AUDIT.md` gaps.medium for completeness.
- **F-11** — already reconciled at commit `00154e5c` (Makefile fixture
  count). Listed in gaps.low for completeness.

## Verification

- `go vet ./internal/mcp/... ./internal/eval/... ./internal/semantic/... ./internal/daemon/... ./internal/guardrails/... ./internal/upgrade/...` — clean.
- `go test ./internal/mcp/... ./internal/eval/... ./internal/semantic/live/handler/... -count=1` — passing.
- `make eval-quick` — 36/36 tasks succeeded.
- `python3 yaml.safe_load(.planning/v1.10-MILESTONE-AUDIT.md frontmatter)` confirms `status=resolved`, `verdict=PRODUCTION-READY`, `len(resolved)=8`.

## Self-check

Files claimed to be created:
- `internal/eval/runner/daemon_tap_integration_test.go` — FOUND
- `internal/eval/runner/runner_diagnostics_test.go` — FOUND
- `internal/eval/report/context_metrics.go` — FOUND
- `internal/eval/report/context_metrics_test.go` — FOUND
- `internal/guardrails/store_jsonl.go` — FOUND
- `internal/semantic/live/handler/difffacts.go` — FOUND

Commit hashes:
- `e7bc406f`, `ab8ad6cb`, `a928acbe`, `93426cc4`, `25b4b60d`, `31c727c3`, `a432a01a`, `8dab9b75`, `caaf4cf7`, `ba5d3643` — all FOUND in `git log`.

## Self-Check: PASSED
