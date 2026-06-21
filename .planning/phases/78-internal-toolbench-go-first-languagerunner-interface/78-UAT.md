---
status: complete
phase: 78-internal-toolbench-go-first-languagerunner-interface
source:
  - 78-01-SUMMARY.md
  - 78-02-SUMMARY.md
  - 78-03-SUMMARY.md
  - 78-04-SUMMARY.md
  - 78-05-SUMMARY.md
started: 2026-06-18T14:42:59Z
updated: 2026-06-18T14:42:59Z
mode: developer-facing (CLI/build/test evidence; auto-verified by orchestrator)
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: helix + helix-bench build and run from scratch.
result: pass
evidence: helix built; full corpus run completed cleanly (exit 0).

### 2. LanguageRunner interface + GoRunner (test2json)
expected: GoRunner wraps `go test ./... -json`, parses per-test rows, Passed=exit==0, compile-failure → Passed=false/Tests=[].
result: pass
evidence: TestRunTestsPassing, TestRunTestsFailing, TestRunTestsCompileFailure, TestCapabilitiesAreTheTenClasses all PASS.

### 3. --languages axis + 4-way matrix
expected: helix-bench exposes `--languages`; ExpandMatrix is benchmark×language×mode×task with per-axis path-traversal rejection.
result: pass
evidence: cmd/helix-bench/main.go:147 registers --languages; TestExpandMatrixCartesianCount/SingleCell/RejectsPathTraversal/RejectsEmpty PASS.

### 4. 10-capability Go corpus exists + documented
expected: 10 IT-go-* task dirs under internal-toolbench/go covering all 10 capabilities; CAPABILITIES.md + crosswalk present.
result: pass
evidence: 10 task dirs present (patch_apply, semantic_view, lsp_diagnostics, call_graph, dependency_graph, rename_safety, fuzzy_search, context_min, failure_handling, incremental_update); CAPABILITIES.md + PHASE67_CROSSWALK.md present.

### 5. Full Go corpus runs green end-to-end
expected: running the full Go corpus through the harness yields all cells succeeded with success outcomes.
result: pass
evidence: `helix-bench run --benchmarks=internal-toolbench --languages=go --agent=scripted --parallel=4` → "10/10 cells succeeded" in 10.7s; all 10 result.v2.json outcome=success.

### 6. Coverage aggregator (declared ∩ covered) + gap detection
expected: Coverage() reports Go 10/10 from task.json capability fields; synthetic gap surfaces in Missing; namespace invariant enforced.
result: pass
evidence: TestCoverageGoIsTenOfTen, TestCoverageDetectsGap, TestCoverageDeduplicatesDeclared, TestNamespaceIsITGoZeroT67 all PASS.

### 7. Parallel per-cell store isolation (D-03)
expected: under --parallel, each cell gets a distinct ephemeral store (WithWorkingDir) — no shared-CWD DuckDB deadlock; no foreign-PID cross-talk.
result: pass
evidence: TestStoreIsolationParallel PASS (10.56s) with HELIX_BIN set — 2/2 succeeded, distinct ScratchDir roots, RejectedForeignPid==0.

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none]
