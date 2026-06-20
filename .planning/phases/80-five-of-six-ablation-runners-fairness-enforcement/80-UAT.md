---
status: complete
phase: 80-five-of-six-ablation-runners-fairness-enforcement
source:
  - 80-01-SUMMARY.md
  - 80-02-SUMMARY.md
  - 80-03-SUMMARY.md
  - 80-04-SUMMARY.md
  - 80-05-SUMMARY.md
started: 2026-06-19T10:25:00Z
updated: 2026-06-19T10:40:00Z
---

## Current Test

[testing complete]

## Tests

### 1. Cold Start Smoke Test
expected: Clean `go build ./cmd/helix ./cmd/helix-bench`, clean `go vet ./bench/...`, and `go test ./bench/... -count=1` all green.
result: pass

### 2. Six-mode ablation matrix resolves (ABLATE-01/03)
expected: All 6 bench modes — your_agent_full, baseline_plain, no_lsp, no_structured_edit, your_agent_no_semantic, baseline_rag — resolve to their declared profiles with zero Go change (table-driven via bench/runners/<mode>/MODE.md). `go test ./bench/runners/ -run TestModeResolver` passes; BENCH.md documents the mode→profile table.
result: pass

### 3. Fairness startup gate fatals on unfair contract (D-04)
expected: RunCell calls DefaultContract.Validate() unconditionally right after profile resolution and BEFORE spawning a daemon; an override deviating from the shared budget without a WaiverReason is fatal (no daemon spawned). The committed contract has no overrides so CI never fatals. `go test ./bench/runtime/ -run TestFairnessGate` passes.
result: pass

### 4. baseline_rag fail-close → Deferred, no result row (D-02)
expected: Running mode baseline_rag short-circuits before any sandbox/daemon, writes NO result.v2.json, and is reported as a distinct Deferred outcome (not Success, not infra Err) whose reason names Phase 83 (ABLATE-04). `go test ./bench/runtime/ -run TestBaselineRagFailClose` passes.
result: pass

### 5. no_semantic row carries deferred-guarantee marker (D-03)
expected: The your_agent_no_semantic row emits ablation_status: guarantee_pending_phase_81 (honest modes omit the key via omitempty). The marker is documented in result.v2.schema.json as an additive optional property (schema stays v2, no major bump). `go test ./bench/runtime/ -run TestAblationStatus` passes.
result: pass

### 6. Ablation deltas surfaced in the 4 real-mode rows (D-05)
expected: After RunMatrix, a single-run 3-delta pass (full_minus_baseline_plain / full_minus_no_lsp / full_minus_no_structured_edit) is surfaced under an open ablation_deltas property in each of the 4 real-mode result.v2 rows; tasks missing any real mode are reported skipped, not computed against nil. The five-of-six smoke proves 4 real + 1 partial + 0 baseline_rag rows end-to-end. `go test ./bench/runtime/ -run 'TestAblationDeltas|TestFiveOfSix'` passes.
result: pass

### 7. Always-on CI fairness contract test (D-04 layer 2)
expected: TestEffectiveConfigMatchesContract iterates all 6 registered modes, asserts each resolves via ResolveProfile and projects model_id == DefaultContract.ModelID with DefaultContract.Validate()==nil — turning fairness-contract drift into a hard CI failure, hermetically (no live model, no network). `go test ./bench/runners/ -run TestEffectiveConfigMatchesContract` passes 6/6.
result: pass

## Summary

total: 7
passed: 7
issues: 0
pending: 0
skipped: 0

## Gaps

[none yet]
