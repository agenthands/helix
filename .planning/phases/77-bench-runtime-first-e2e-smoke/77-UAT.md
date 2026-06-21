---
status: complete
phase: 77-bench-runtime-first-e2e-smoke
source:
  - 77-01-SUMMARY.md
  - 77-02-SUMMARY.md
  - 77-03-SUMMARY.md
  - 77-04-SUMMARY.md
  - 77-05-SUMMARY.md
started: 2026-06-18T14:41:38Z
updated: 2026-06-18T14:41:38Z
mode: developer-facing (CLI/build/test evidence; auto-verified by orchestrator)
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: `go build -o helix ./cmd/helix` succeeds; helix-bench binary runs.
result: pass
evidence: bench-quick rebuilt helix from scratch and ran cleanly (exit 0).

### 2. helix-bench run subcommand
expected: `cmd/helix-bench` exposes a `run` subcommand (the operator-facing surface).
result: pass
evidence: cmd/helix-bench/main.go:118 `Use: "run"`; smoke invoked it successfully.

### 3. End-to-end bench smoke (make bench-quick)
expected: hermetic scripted bench cell runs end-to-end within the ≤90s CI budget; ≥1 cell succeeds.
result: pass
evidence: `make bench-quick` → "1/1 cells succeeded" in 3.77s (well under 90s budget); reports written to bench/reports/20260618T144138Z.

### 4. result.v2 produced + schema-valid (validate-on-write gate)
expected: cell emits a result.v2.json that passes the embedded Draft2020 schema with the pinned provenance keys.
result: pass
evidence: result.v2.json has schema_version=v2, outcome=success, fairness object, model_id set; written through BuildResult+Validate gate without error. tokens_input/output null (scripted agent fabricates no tokens — correct per D-02).

### 5. Two-leg trace merge (daemon tap + synth CC leg)
expected: result carries a trace_ref combining the real daemon tap with the synthesized CC leg, no double-counting.
result: pass
evidence: result.v2.json trace_ref present; trace.json emitted alongside; TestSynthCCTapShape/Timestamps/Merge PASS.

### 6. Matrix expansion + bounded parallel dispatch
expected: benchmark×mode×task cartesian expansion with path-traversal rejection; --parallel bound honored.
result: pass
evidence: TestExpandMatrixCartesianCount/SingleCell/RejectsPathTraversal/RejectsEmpty PASS; TestDispatchRespectsParallelBound, TestDispatchAggregatesSuccess, TestRunMatrixNilContext PASS.

### 7. Bench runtime unit suites green
expected: sandbox/subprocess/resolver/cell/runtime/schema packages compile and test green.
result: pass
evidence: `go test ./bench/... ./cmd/helix-bench/` all ok.

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
