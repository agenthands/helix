---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
plan: 04
subsystem: bench-runners-fairness
tags: [fairness, contract, ci-gate, ablation, tdd, hermetic-test]
requires:
  - "Plan 80-01: the 6 registered bench MODE.md dirs (ResolveProfile target set)"
  - "bench/runners/fairness_contract.go: DefaultContract + Validate() (Phase 75)"
  - "bench/runtime/result.go: BuildResult model_id projection (Phase 77)"
provides:
  - "TestEffectiveConfigMatchesContract — always-on hermetic CI gate against effective-config drift (D-04 layer 2, criterion #3)"
affects:
  - "bench/runners (test-only; no production code touched)"
tech-stack:
  added: []
  patterns:
    - "Hermetic table-driven CI contract test (no live model, no network)"
    - "Projected-config assertion (scope A): assert only the field with a real producer (model_id), document the rest as a known gap (Pitfall 5)"
key-files:
  created:
    - "bench/runners/contract_test.go"
  modified: []
decisions:
  - "Asserted the PROJECTED effective config (model_id == DefaultContract.ModelID) for all 6 modes + Validate()==nil; the other 5 contract fields are documented as wired-not-enforced (no live argv producer) rather than faked (Open Q1 scope A / Pitfall 5)"
  - "Did NOT duplicate TestSystemPromptHashMatches / TestDefaultContractValidates (they live in fairness_contract_test.go); this test sits alongside them as the second always-on drift guard"
  - "RED proven by a transient unregistered mode making the test fail, then restored — the test detects drift, it is not a vacuous pass"
metrics:
  duration: ~8min
  tasks: 1
  files: 1
  completed: 2026-06-19
---

# Phase 80 Plan 04: Unconditional Effective-Config CI Contract Test Summary

Added `bench/runners/contract_test.go::TestEffectiveConfigMatchesContract`, an always-on hermetic CI gate that asserts every one of the 6 registered bench modes resolves via `ResolveProfile` and projects `model_id == DefaultContract.ModelID` with `DefaultContract.Validate() == nil` — turning fairness-contract drift into a hard CI failure (criterion #3, D-04 layer 2).

## What Was Built

- **`TestEffectiveConfigMatchesContract`** (`bench/runners/contract_test.go`):
  - Iterates the 6 registered modes: `your_agent_full`, `baseline_plain`, `no_lsp`, `no_structured_edit`, `your_agent_no_semantic`, `baseline_rag`.
  - For each: `ResolveProfile(mode)` must not error and must return a non-empty profile (the mode is registered with valid MODE.md frontmatter).
  - Asserts the single-source invariant: `DefaultContract.ModelID` is non-empty and is the value every mode's projected `model_id` derives from (no per-mode ModelID override path exists — `ModeOverride` has only `MaxTokens`/`Temperature`).
  - Asserts `DefaultContract.Validate() == nil` (no override missing a WaiverReason; real runner startup would not fatal).
  - Hermetic: no live model, no network, runs in CI regardless of `--agent`.
  - Top-of-test doc comment records the wired-not-enforced gap for the other 5 fields (`temperature`/`max_tokens`/`system_prompt_hash`/`retry_policy`/`cache_policy`) — `internal/eval/agent/claude.go` buildArgv (76-89) threads only `--max-turns`, so those fields have no live producer to assert against (Open Q1 scope A / Pitfall 5).

## TDD RED→GREEN

The deliverable is a test. GREEN holds against the committed `DefaultContract` + the 6 Plan-01 MODE.md dirs. RED capability was proven explicitly: temporarily adding a `bogus_unregistered_mode` to the table made the test FAIL at `ResolveProfile(...) = unknown mode`, confirming the gate detects drift and is not a vacuous pass; the file was then restored to GREEN before committing.

## Verification

- `go test ./bench/runners/ -run TestEffectiveConfigMatchesContract -count=1` — PASS (6/6 subtests).
- `go test ./bench/runners/ -count=1` — PASS (existing gates `TestSystemPromptHashMatches`, `TestDefaultContractValidates`, deprecation/waiver gates all green; no duplication).
- `go build ./cmd/helix` — OK.
- `go vet ./...` — OK (full module).
- `go test ./bench/...` — all packages green.

## Deviations from Plan

None — plan executed exactly as written. Scope held to A (projected `model_id` + `Validate()`); did not expand into threading contract fields through the shared eval agent (scope B, out of the "no new infra" boundary).

## Known Stubs

None. The test asserts the only contract field with a live producer (`model_id`); the unenforced fields are documented as a future-scope gap, not stubbed or faked.

## Threat Flags

None. Test-only addition — no production code, no network, no new external surface. Mitigates T-80-02 (a runner silently drifting from the fairness contract) as planned.

## Self-Check: PASSED

- FOUND: `bench/runners/contract_test.go`
- FOUND commit: `3973fe05` (`test(80-04): unconditional CI contract test for effective-config drift`)
