---
phase: 86-crosscodeeval-repobench-adapters-multi-oracle-completion-gat
plan: 02
subsystem: bench/evaluators
tags: [crosscodeeval, completion-gate, verified-correctness, multi-oracle, abstain, tdd, security-invariant]
requires:
  - bench/evaluators/exactmatch.EM (Plan 01 CM-EM scorer)
  - bench/evaluators/editsim.ES (Plan 01 CM-ES scorer)
  - bench/evaluators/identmatch.Match (Plan 01 IM-EM/IM-F1 scorer)
  - bench/evaluators/metrics.go (Metrics.VerifiedCorrectness *bool; consumed, not modified)
provides:
  - bench/evaluators/completion_gate.Grade (multi-oracle gate -> verified_correctness)
  - bench/evaluators/completion_gate.GateConfig (per-oracle ESThreshold knob)
  - bench/evaluators/completion_gate.GateResult.ApplyToMetrics (additive completion-path producer)
  - bench/evaluators/VERIFIED.md (SC#3 acceptance doc)
  - make verify-verified-md (hard-fail doc gate)
affects:
  - 86-04+ (any wiring that needs a completion-path verified_correctness producer)
  - REQUIREMENTS.md VERIFIED-03 (reconciled)
tech-stack:
  added: []
  patterns:
    - composition-of-leaf-scorers (gate imports only the three Plan 01 subpackages + parent evaluators)
    - explicit-&false fail-closed abstain (mirrors test_runner.Result *bool pattern; never nil-drop, never false-true)
    - additive completion-path producer (ApplyToMetrics) mirroring coordinator.go:77-87 non-short-circuit assignment
    - grep-anchored ^## header presence gate (verify-verified-md), mirroring verify-no-docker-sdk hard-gate shape
key-files:
  created:
    - bench/evaluators/completion_gate/gate.go
    - bench/evaluators/completion_gate/gate_test.go
    - bench/evaluators/VERIFIED.md
  modified:
    - Makefile
decisions:
  - "verified_correctness=true iff EM && ES>=cfg.ESThreshold && IM-EM (all-three-required); a single failing oracle yields an explicit *false, never a partial pass."
  - "Abstain is fail-closed and oracle-independent: Grade(...,abstain=true) returns &false WITHOUT consulting the scorers, distinct from the reserved could-not-score path (nil pointer + MetricError). Even pred==gold abstains to false."
  - "ESThreshold is a live GateConfig field (DefaultESThreshold=0.9), proven configurable by a knob test where the same ES value straddles 0.5 and 0.99."
  - "ApplyToMetrics mutates ONLY Metrics.VerifiedCorrectness; additive, no result.v2.schema.json field added (verified via git diff over bench/schema/ = empty)."
  - "verify-verified-md is a grep ^## header presence gate (anchored so prose mentions don't satisfy it), proven to have teeth: a renamed header exits non-zero (2)."
metrics:
  duration: ~3min
  completed: 2026-06-21
---

# Phase 86 Plan 02: Multi-Oracle Completion Gate + VERIFIED.md Summary

The multi-oracle completion gate (VERIFIED-03) — the non-test-bearing sibling producer of `verified_correctness` that `test_runner.go:37-40` anticipates — composes the three Plan 01 CrossCodeEval leaf scorers (EM AND edit-similarity>=configurable-threshold AND identifier-match, all-three-required) and fails CLOSED on abstain with an explicit non-nil `*bool false`, documented in the new `bench/evaluators/VERIFIED.md` and guarded by a `make verify-verified-md` hard gate with teeth.

## What Was Built

- **`bench/evaluators/completion_gate/gate.go`** — `Grade(pred, gold string, cfg GateConfig, abstain bool) GateResult`. On the oracle path it computes `em := exactmatch.EM`, `es := editsim.ES`, `id, _ := identmatch.Match`, and sets `verified_correctness = em && es >= cfg.ESThreshold && id` (all-three-required, T-86-02-02), returning all four pointers (`VerifiedCorrectness`, `EM`, `IDMatch`, `ES`) for auditability. On `abstain=true` it returns `VerifiedCorrectness: &false` WITHOUT consulting the oracles (the security-relevant VERIFIED-03 / T-86-02-01 fail-closed invariant). `GateConfig{ESThreshold float64}` is the per-oracle configurable knob (`DefaultESThreshold = 0.9`). `GateResult.ApplyToMetrics(*evaluators.Metrics)` is the additive completion-path producer that assigns onto the existing `VerifiedCorrectness *bool` (metrics.go:21) and returns `Errs` — mirroring the coordinator's non-short-circuit assignment (coordinator.go:77-87). No `result.v2.schema.json` field is added.
- **`bench/evaluators/completion_gate/gate_test.go`** — the SOLE hermetic proof (no network, no `HELIX_BIN`). Covers: all-three-pass→true; single-oracle-fail (EM)→false even with ES>=threshold and IDMatch true; ES-below-threshold→false; the threshold-knob (same ES straddles 0.5 and 0.99); the ES boundary (ES==1.0 meets threshold 1.0→true); and the load-bearing abstain assertion `res.VerifiedCorrectness != nil && *res.VerifiedCorrectness == false`. Plus `TestApplyToMetrics` proving the additive producer carries both the true verdict and the abstain explicit-false onto a `Metrics` record.
- **`bench/evaluators/VERIFIED.md`** — SC#3 acceptance doc with `## Oracles` (all-three-required table + verbatim composite rule), `## Threshold` (named `GateConfig.ESThreshold`, default 0.9, knob-test citation), `## Abstain` (quoting the SC#3 wording; abstain `false` vs could-not-score `nil` distinction), `## Tokenizer` (the `[A-Za-z_][A-Za-z0-9_]*` regex + the full keyword set cited verbatim from `identmatch.go`, Pitfall 4), `## EditSimilarity` (the `1 - lev/max(runeLen)` definition, explicitly NOT git-numstat, Pitfall 2), and `## Proof`.
- **`Makefile`** — `verify-verified-md` target (added to `.PHONY`): a grep-based presence gate over the six required `^## ` headers that hard-fails (non-zero) on a missing file or any absent header, mirroring the `verify-no-docker-sdk` recipe shape.

## TDD Gate Compliance

Task 1 followed RED → GREEN as atomic commits:

| Gate | Commit | Proof |
|------|--------|-------|
| RED (test) | `287249f8` | `go test` failed to compile (`undefined: Grade / GateConfig`) before any implementation |
| GREEN (impl) | `332289fb` | `go test ./bench/evaluators/completion_gate/...` → ok |

No REFACTOR commit was needed. Task 2 is `type=auto` (doc + Makefile gate), committed as `9b80da11`.

## Verification

- `go test ./bench/evaluators/completion_gate/...` — ok (the sole authoritative proof; abstain test `TestGrade_Abstain_ExplicitFalse` PASS).
- `go build ./...` — clean.
- `go vet ./bench/evaluators/...` — clean.
- `go test ./bench/evaluators/...` — all 11 packages ok.
- `make vet` — clean (all 6 custom vettools pass).
- `make verify-verified-md` — exits 0 with the committed VERIFIED.md.
- **Teeth check:** renaming `## Abstain` → `## AbstainXX` made `make verify-verified-md` exit non-zero (2) with `::error::SC#3 violation: ... missing required section '## Abstain'`; the file was restored and the gate returned 0. The gate is not a rubber stamp.
- **No schema bump:** `git diff HEAD~4 --stat -- bench/schema/` is empty — the producer is additive over the existing `verified_correctness` field.

## VERIFIED-03 Reconciliation

VERIFIED-03 acceptance ("gate documented in `bench/evaluators/VERIFIED.md`; threshold per oracle configurable") is satisfied: the gate exists with all-three-required logic, `GateConfig.ESThreshold` is the configurable per-oracle threshold, abstain emits explicit `false`, and `VERIFIED.md` documents all of it behind a hard gate.

## Deviations from Plan

None — plan executed exactly as written. (The plan's `gate_test.go` table is implemented faithfully; two of the named ES-threshold cases are realized as `TestGrade_ThresholdKnob` plus an honest `TestGrade_ThresholdKnob_ESOnlyDimension` boundary test, because EM==true forces ES==1.0 — so the composite gate's ES dimension is exercised at the threshold==1.0 boundary while the pure knob test isolates the ES-vs-threshold comparison on a non-exact pair. This is faithful to the plan's intent, with the constraint documented in the test comments.)

## Known Stubs

None. The gate, its producer, the doc, and the Makefile gate are fully implemented; the reserved "could-not-score" `nil + MetricError` path is intentionally unused today (all current scorers are total) and documented as such in `gate.go` and `VERIFIED.md`.

## Self-Check: PASSED

All 3 created files exist on disk; the `verify-verified-md` Makefile target is present; all 3 task commits (`287249f8` RED, `332289fb` GREEN, `9b80da11` doc) are in git history.
