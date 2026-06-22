---
phase: 79
plan: 03
subsystem: bench/evaluators
tags: [tdd, trace-derived-graders, token-metrics, tool-trace-metrics, nullable-metrics]
requires:
  - "bench/evaluators.Metrics + MetricError (79-01)"
  - "internal/eval/trace.MergedTrace (Phase 67/77 trace substrate)"
provides:
  - "bench/evaluators/token_meter.MeterTokens(mt, usagePresent) — provider-usage-sourced token pointers (+cached), null when absent"
  - "bench/evaluators/tool_trace_analyzer.Analyze(mt) — tool_calls/wall_time/files_read/bytes_read/lsp_diagnostics_used/semantic_tool_calls/retry_count from the merged trace"
  - "token_meter testdata/cc_stream_with_usage.jsonl fixture (real provider usage block)"
affects:
  - "Plan 79-04 coordinator (threads usagePresent + wires the 7 trace metrics into Metrics)"
tech-stack:
  added: []
  patterns:
    - "trace-derived grader as a pure transform over MergedTrace (no re-merge, METRIC-06)"
    - "present-or-null on an out-of-band usagePresent signal (D-01/D-02/D-03)"
    - "registry-anchored tool-name sets (package vars, Assumption A4)"
key-files:
  created:
    - bench/evaluators/token_meter/token_meter.go
    - bench/evaluators/token_meter/token_meter_test.go
    - bench/evaluators/token_meter/testdata/cc_stream_with_usage.jsonl
    - bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go
    - bench/evaluators/tool_trace_analyzer/tool_trace_analyzer_test.go
  modified: []
decisions:
  - "MeterTokens reads ONLY mt.Usage for token values; daemon counters (tool-call tally, ResultSizeBytes) are never a token source (METRIC-03/D-02)"
  - "usagePresent=false yields all-nil token pointers + a token_meter MetricError; never a fabricated *int(0) (D-01)"
  - "present-and-zero cached tokens return pointer-to-0, distinct from scripted nil (D-03)"
  - "tool_calls pinned to mt.ToolCallSummary.Total; analyzer never re-merges (METRIC-06)"
  - "semantic-tool-name set anchored on the 9 internal/kernel/symbols tools (NOT the mcp__smtc__* client aliases) — those are the strings landing in ByTool/Event.Tool"
metrics:
  duration: ~25m
  completed: 2026-06-18
---

# Phase 79 Plan 03: Trace-Derived Graders (token_meter + tool_trace_analyzer) Summary

Two TDD graders that consume the already-merged `trace.MergedTrace`: `token_meter` sources tokens_input/output plus the two cached columns exclusively from the provider `usage` block (null when absent), and `tool_trace_analyzer` derives the 7 trace-shaped metrics as a pure transform without re-merging.

## What Was Built

### token_meter (Task 1)
- `MeterTokens(mt trace.MergedTrace, usagePresent bool) (in, out, cachedRead, cacheWrite *int, errs []evaluators.MetricError)`.
- Reads ONLY `mt.Usage.{InputTokens,OutputTokens,CacheReadTokens,CacheCreationTokens}` (METRIC-03/D-02). Daemon counters are never consulted.
- `usagePresent=false` → all-nil + a single `MetricError{Metric:"tokens_input", Grader:"token_meter", Reason:"no provider usage block (scripted run)"}` (D-01).
- `usagePresent=true` with zero cached tokens → present pointers-to-0 (D-03).
- `testdata/cc_stream_with_usage.jsonl`: captured CC `--output-format=stream-json` lines with a `result` event carrying a real `usage` block (`input_tokens:1234, output_tokens:567, cache_read_input_tokens:89, cache_creation_input_tokens:42`). The test parses it via `trace.TapCCStream`.

### tool_trace_analyzer (Task 2)
- `Analyze(mt trace.MergedTrace) (TraceMetrics, []evaluators.MetricError)` — pure transform, no `trace.Merge` call (METRIC-06).
- `TraceMetrics{ToolCalls, WallTimeSeconds, FilesRead, BytesRead, LSPDiagnosticsUsed, SemanticToolCalls, RetryCount *…}`.
- Derivations: `tool_calls = mt.ToolCallSummary.Total`; `wall_time_seconds = float64(mt.DurationMs)/1000`; `semantic_tool_calls = Σ ByTool over the semantic set`; `files_read`/`bytes_read` from read/list events (`ResultSizeBytes` summed); `lsp_diagnostics_used` from `get_diagnostics` events; `retry_count` from `KindAPIRetry` events.
- Registry-anchored package vars: `semanticToolNames` (9 `internal/kernel/symbols` tools), `diagnosticToolNames` (`get_diagnostics`), `readToolNames` (`read_file`,`list_dir`).

## TDD Cycle (RED → GREEN)

| Task | RED commit | GREEN commit |
|------|-----------|--------------|
| 1: token_meter | `b7aaab54` test(79-03): failing usage source-of-truth + scripted-null | `3c88344a` feat(79-03): token_meter provider-usage source-of-truth |
| 2: tool_trace_analyzer | `6df96614` test(79-03): failing trace-derived metric tests | `d1d93f35` feat(79-03): tool_trace_analyzer over merged trace |

RED precedes GREEN for both tasks (build-fail RED: `MeterTokens`/`Analyze` undefined).

## Plan 04 Wiring Contract

- `usagePresent` is an out-of-band signal the coordinator (Plan 04) must thread: `agent kind == "claude"` AND a CC `result` event with a usage block was parsed. Pass it into `MeterTokens`. For the scripted Go-ToolBench corpus it is `false` → token metrics are explicit null.
- `Analyze(mt)` consumes `RunCell`'s already-merged `MergedTrace` (`res.Merged`). Do NOT re-merge.
- Map the returned pointers onto `evaluators.Metrics`: `TokensInput/TokensOutput/TokensInputCachedRead/TokensInputCacheWrite` (token_meter) and `ToolCalls/WallTimeSeconds/FilesRead/BytesRead/LSPDiagnosticsUsed/SemanticToolCalls/RetryCount` (tool_trace_analyzer). Append both graders' `[]MetricError`.

## Deviations from Plan

None — plan executed exactly as written. Test fixture semantic-tool counts were authored coherently (go_to_definition + find_references = 3 semantic; read_file/write_file non-semantic) to match the plan's "3 semantic + 2 non-semantic" behavior.

## Acceptance Criteria Verification

- `go test ./bench/evaluators/token_meter/ -count=1` — PASS (all four behaviors).
- `go test ./bench/evaluators/tool_trace_analyzer/ -count=1` — PASS (all behaviors).
- `grep -nE '\.Usage\.(InputTokens|OutputTokens|CacheReadTokens|CacheCreationTokens)' token_meter.go` — matches (Usage-sourced).
- `grep -nE 'ToolCallSummary|ResultSizeBytes' token_meter.go` — no match (no daemon-counter sourcing).
- `grep -n 'trace\.Merge\b' tool_trace_analyzer.go` — no match (no re-merge).
- `go build ./cmd/helix`, `go vet ./bench/evaluators/...`, `go test ./bench/...` — all PASS.

## Self-Check: PASSED
- bench/evaluators/token_meter/token_meter.go — FOUND
- bench/evaluators/token_meter/token_meter_test.go — FOUND
- bench/evaluators/token_meter/testdata/cc_stream_with_usage.jsonl — FOUND
- bench/evaluators/tool_trace_analyzer/tool_trace_analyzer.go — FOUND
- bench/evaluators/tool_trace_analyzer/tool_trace_analyzer_test.go — FOUND
- Commits b7aaab54, 3c88344a, 6df96614, d1d93f35 — FOUND
