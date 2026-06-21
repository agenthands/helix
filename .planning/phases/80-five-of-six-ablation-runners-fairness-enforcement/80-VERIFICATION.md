---
phase: 80-five-of-six-ablation-runners-fairness-enforcement
verified: 2026-06-19T13:20:00Z
status: passed
score: 20/20 must-haves verified
overrides_applied: 0
re_verification:
  previous_status: none
  note: initial verification
---

# Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement Verification Report

**Phase Goal:** All 6 ablation modes are operational end-to-end on the Go ToolBench corpus, holding the same-model-same-budget invariant via the Phase 75 fairness contract. `your_agent_full` + `baseline_plain` + `no_lsp` + `no_structured_edit` produce real per-mode `result.v2.json` rows; `no_semantic` scaffolding is in place but the kernel flag (ABLATE-06) lands in Phase 81; `baseline_rag` runner stub exists but its real implementation lands in Phase 83.
**Verified:** 2026-06-19T13:20:00Z
**Status:** passed
**Re-verification:** No — initial verification

## Goal Achievement

### Observable Truths (ROADMAP Success Criteria + 5 PLAN must_have sets)

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| SC1 | Each of 6 modes resolves via `bench/runners/<mode>/MODE.md`; runs end-to-end on a smoke task producing schema-valid `result.v2.json` rows tagged with the mode | ✓ VERIFIED | All 6 MODE.md dirs exist (baseline_plain, baseline_rag, no_lsp, no_structured_edit, your_agent_full, your_agent_no_semantic). `TestFiveOfSixSmoke` PASSED with a real spawned daemon (1.41s): 4 real schema-valid rows + 1 partial + 0 baseline_rag rows. |
| SC2 | `baseline_plain` reuses `internal/profile/profiles/baseline.yaml` (MODE.md says `profile: baseline`, no new bench-* YAML); empty inventory documented in `bench/BENCH.md` | ✓ VERIFIED | `baseline_plain/MODE.md` → `profile: baseline`. `baseline.yaml` has `skills: []`, `tools: []`, `exclude_tools: []` (empty control inventory). `bench/BENCH.md:51-58` documents the empty Helix inventory + "no new YAML" decision and the shell/grep/read/edit/test native surface. No `bench-baseline-plain.yaml` exists. |
| SC3 | Same-model-same-budget invariant enforced at runner-startup (`RunCell` calls `DefaultContract.Validate()`); unconditional CI contract test asserts effective config == DefaultContract | ✓ VERIFIED | `cell.go:289` calls `runners.DefaultContract.Validate()` as a fatal gate AFTER profile resolution and BEFORE sandbox/daemon. `contract_test.go::TestEffectiveConfigMatchesContract` is unconditional + hermetic (no model/network), iterates all 6 modes, asserts projected `model_id == DefaultContract.ModelID` and `Validate()==nil`. Passes. |
| SC4 | The 3 ablation deltas (full−baseline_plain, full−no_lsp, full−no_structured_edit) compute and surface in per-mode result rows (`deltas.go::ComputeAndWriteDeltas`) | ✓ VERIFIED | `deltas.go` defines `ComputeAndWriteDeltas` with exactly 3 fixed delta operands (lines 45-47), writes `ablation_deltas` back into each of the 4 real rows, re-validates against schema, writes atomically. Smoke asserts each real row carries exactly 3 deltas + re-validates. `cmd/helix-bench/main.go:265` wires it post-RunMatrix after wg.Wait(). |
| P1.1 | Each of 5 new modes resolves to its profile via MODE.md (no Go resolver change) | ✓ VERIFIED | `mode_resolver.go` is filesystem-table-driven (reads MODE.md frontmatter); `mode_resolver_test.go` asserts all 5 resolve. |
| P1.2 | baseline_plain resolves to profile baseline (not a new bench-* YAML) | ✓ VERIFIED | MODE.md `profile: baseline`; no bench-baseline-plain.yaml. |
| P1.3 | your_agent_no_semantic MODE.md documents Phase 81 deferred guarantee | ✓ VERIFIED | MODE.md contains "Deferred guarantee — `ablation_status: guarantee_pending_phase_81`" + Phase 81 prose. |
| P1.4 | baseline_rag MODE.md documents Phase 83 RAG deferral | ✓ VERIFIED | MODE.md "Deferred to Phase 83" + ABLATE-04 prose. |
| P1.5 | BENCH.md documents baseline_plain empty inventory + no_semantic deferred guarantee | ✓ VERIFIED | `bench/BENCH.md:51-66`. |
| P2.1 | BuildResult projects non-empty AblationStatus into `ablation_status` | ✓ VERIFIED | `result.go:165` `AblationStatus: in.AblationStatus`; field json `ablation_status,omitempty` (line 118). |
| P2.2 | Row with `guarantee_pending_phase_81` still passes validate-on-write | ✓ VERIFIED | Smoke writes the partial row; `result_test.go` + smoke re-validate clean. |
| P2.3 | Honest mode (empty AblationStatus) omits `ablation_status` (omitempty) | ✓ VERIFIED | `omitempty` tag (result.go:118); smoke asserts real-mode rows carry NO ablation_status. |
| P3.1 | RunCell calls Validate() at startup, non-nil is fatal (no daemon) | ✓ VERIFIED | `cell.go:289-291` returns before sandbox.New. |
| P3.2 | RunCell short-circuits baseline_rag BEFORE sandbox/daemon, NO result row, distinct Deferred | ✓ VERIFIED | `cell.go:301-305` sets Deferred + returns nil err before `benchsandbox.New` (line 309). Smoke asserts baseline_rag writes no file. |
| P3.3 | RunCell sets `guarantee_pending_phase_81` on your_agent_no_semantic only | ✓ VERIFIED | `ablationStatusFor` (cell.go:58-61) returns the marker only for that mode; used at cell.go:577. |
| P3.4 | baseline_rag counted as neither success nor infra error | ✓ VERIFIED | `matrix.go` CellOutcome has `Deferred` field; smoke asserts Deferred==true, Success==false, Err==nil. |
| P4.1 | Unconditional CI test asserts every runner's projected effective config matches DefaultContract | ✓ VERIFIED | `contract_test.go:37-87`. |
| P4.2 | Asserts projected model_id == DefaultContract.ModelID per mode + Validate() clean | ✓ VERIFIED | Lines 52, 82-84. |
| P4.3 | Test requires no live model / no network (hermetic) | ✓ VERIFIED | Pure loop over filesystem resolver + compile-time contract; no net imports. |
| P5.x | 3 deltas computed; surface in rows (re-validated); smoke = 4 real + 1 partial + 0 rag; missing-mode tasks skipped | ✓ VERIFIED | `deltas.go` + `five_of_six_test.go:33-136`; `report.Skipped` empty in smoke, skip path present in ComputeAndWriteDeltas. |

**Score:** 20/20 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
| -------- | -------- | ------ | ------- |
| `bench/runners/{6 modes}/MODE.md` | 6 mode definitions with correct profile frontmatter | ✓ VERIFIED | All 6 present; profiles: baseline / baseline / bench-no-lsp / bench-no-structured-edit / bench-no-semantic / bench-full. |
| `bench/runners/mode_resolver_test.go` | resolver test for all modes | ✓ VERIFIED | Asserts all 6 resolve incl. baseline_rag→baseline. |
| `bench/BENCH.md` | empty-inventory + deferral docs | ✓ VERIFIED | Lines 45-66. |
| `bench/runtime/result.go` | AblationStatus field + omitempty + BuildResult wiring | ✓ VERIFIED | Lines 60-66, 116-118, 165. |
| `bench/schema/result.v2.schema.json` | optional ablation_status, additive | ✓ VERIFIED | Line 178; additionalProperties open at top level (smoke write-back of ablation_deltas re-validated clean). |
| `bench/runtime/cell.go` | Validate() gate + baseline_rag fail-close + ablation_status + Deferred fields | ✓ VERIFIED | Lines 58-61, 155-163, 289-305, 577. |
| `bench/runtime/matrix.go` | distinct Deferred CellOutcome | ✓ VERIFIED | Lines 71-76. |
| `bench/runners/contract_test.go` | TestEffectiveConfigMatchesContract + gap doc | ✓ VERIFIED | Lines 10-87. |
| `bench/runtime/deltas.go` | single-run 3-delta helper | ✓ VERIFIED | ComputeAndWriteDeltas, 3 fixed operands, re-validate + atomic write. |
| `cmd/helix-bench/main.go` | post-RunMatrix delta-pass invocation | ✓ VERIFIED | Line 265 ComputeAndWriteDeltas over summary.Outcomes after wg.Wait barrier. |
| `bench/runtime/five_of_six_test.go` | scripted multi-mode smoke | ✓ VERIFIED | Asserts 4 real + 1 partial + 0 rag rows; PASSED with real daemon. |

### Key Link Verification

| From | To | Via | Status |
| ---- | -- | --- | ------ |
| mode_resolver_test.go | MODE.md | ResolveProfile reads frontmatter | ✓ WIRED |
| result.go BuildResult | resultDoc.AblationStatus | `AblationStatus: in.AblationStatus` | ✓ WIRED |
| cell.go RunCell | DefaultContract.Validate() | startup fatal gate | ✓ WIRED |
| cell.go RunCell | result row ablation_status | `ablationStatusFor(cfg.Mode)` | ✓ WIRED |
| contract_test.go | DefaultContract.ModelID | per-mode projected assertion | ✓ WIRED |
| cmd/helix-bench/main.go runBench | delta pass | post-RunMatrix over Outcomes | ✓ WIRED |
| deltas.go | per-mode rows | read ResultPath, write ablation_deltas, re-Validate | ✓ WIRED |

### Data-Flow Trace (Level 4)

| Artifact | Data | Source | Real Data | Status |
| -------- | ---- | ------ | --------- | ------ |
| result.v2.json rows | ablation_status / ablation_deltas / metrics | RunCell → BuildResult; ComputeAndWriteDeltas reads real on-disk rows | Yes — smoke produced 5 real rows from a spawned daemon running scripted agent against a real seed task; deltas computed from real metric values and re-validated | ✓ FLOWING |

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| Build clean | `go build ./cmd/helix` | BUILD_OK | ✓ PASS |
| Vet clean (bench + helix-bench) | `go vet ./bench/... ./cmd/helix-bench/...` | clean | ✓ PASS |
| Hermetic bench tests | `go test ./bench/runners/ ./bench/runtime/` | ok / ok | ✓ PASS |
| End-to-end five-of-six | `HELIX_BIN=./helix go test ./bench/runtime -run TestFiveOfSixSmoke` | PASS (1.41s) | ✓ PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
| ----------- | ----------- | ----------- | ------ | -------- |
| ABLATE-01 | 80-01..05 | 6 modes implemented, each runs end-to-end, MODE.md per mode | ✓ SATISFIED | 6 MODE.md + smoke pass; baseline_rag/no_semantic deferral is the locked scope of this "five-of-six" phase (ABLATE-04→83, ABLATE-06→81). |
| ABLATE-03 | 80-01, 80-04 | baseline_plain reuses baseline.yaml, empty Helix inventory | ✓ SATISFIED | profile: baseline + empty tool list + BENCH.md doc. |

**Boundary requirements (must remain not-done — verified intact):**
- ABLATE-04 (baseline_rag standalone MCP server) → REQUIREMENTS.md line 48 `[ ]`, traceability Phase 83 / Pending. Correct.
- ABLATE-06 (disable_semantic_subsystem kernel flag) → REQUIREMENTS.md line 50 `[ ]`, traceability Phase 81 / Pending. Correct.

> Note: REQUIREMENTS.md traceability rows for ABLATE-01 / ABLATE-03 still read "TBD / Pending" (lines 167, 169) despite the `[x]` checkboxes. This is stale tracking text (already flagged in the v1.12 audit commit 8213cc4f "traceability stale"), not a code gap — it does not affect goal achievement and is a documentation-bookkeeping item, not a phase-80 deliverable.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| cmd/helix-bench/main.go | 66,68,87 | "not yet implemented" | ℹ️ Info | Refers to `fetch-datasets` / `report` subcommands — introduced in Phase 75 (commit 8c06cc77), NOT phase-80 deliverables. The `run` command + delta pass (phase-80 scope) are fully implemented. No debt markers (TBD/FIXME/XXX) anywhere in phase-80 files or MODE.md. |

### Intentional Deferrals (NOT gaps — locked scope per CONTEXT.md / phase goal)

- `your_agent_no_semantic` emits a REAL row marked `ablation_status: guarantee_pending_phase_81`; kernel zero-DuckDB guarantee (ABLATE-06) is Phase 81.
- `baseline_rag` is a fail-closed stub (no result row, distinct Deferred outcome); real RAG arm (ABLATE-04) is Phase 83.
- Contract test asserts only the projected `model_id`; the other 5 fields are documented wired-not-enforced (locked Open-Q1 scope-A).
- Delta pass is single-run / 3-fixed-deltas only; aggregator / BCa bootstrap / variance gate / leaderboard is Phase 82.

### Human Verification Required

None. The phase produces runnable code, and the daemon-spawning end-to-end smoke (`TestFiveOfSixSmoke`) was executed in-process against the freshly built binary and passed, giving direct behavioral evidence for all four success criteria.

### Gaps Summary

No gaps. All 4 ROADMAP success criteria and all 20 PLAN must_have truths are verified against the codebase with passing build, vet, hermetic tests, and a passing real-daemon end-to-end five-of-six smoke. Requirement IDs ABLATE-01 and ABLATE-03 are satisfied; boundary requirements ABLATE-04 (Phase 83) and ABLATE-06 (Phase 81) correctly remain not-done. The only observation is a stale documentation row in REQUIREMENTS.md traceability (already known from the milestone audit), which is not a code gap.

---

_Verified: 2026-06-19T13:20:00Z_
_Verifier: Claude (gsd-verifier)_
