---
phase: 87
slug: swe-bench-verified-adapter-utboost-rescorer-multi-oracle-verified-correctness
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-21
---

# Phase 87 — Validation Strategy

> Seeded from the "Validation Architecture" section of 87-RESEARCH.md. Governing rule (MEMORY
> false-green + Phase 81 vacuous-gate): the harness-JSON→result.v2 ingestion, the UTBoost rescorer,
> the 3-condition `verified_correctness` producer, and `differential.go` are all pure logic — their
> HERMETIC tests against committed fixtures (sample predictions.jsonl, sample run-report JSON, sample
> per-instance report.json, sample UTBoost augmented report, a known-buggy-patch case) are the SOLE
> authoritative proof. The live 5-task Docker + upstream-swebench smoke run SKIPs cleanly (Docker +
> swebench package + images absent here). No live test may be the sole proof.
> THE LOAD-BEARING PROOF (SC#2): a hermetic test where a known-buggy patch yields `task_success=true`
> AND `verified_correctness=false`.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (committed SWE-bench/UTBoost JSON fixtures) |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./bench/... && go test ./bench/evaluators/... ./bench/datasets/... ./bench/runtime/... ./bench/aggregator/...` |
| **Full suite command** | `go test ./...` (live Docker + swebench smoke tests SKIP unless Docker + swebench + network present) |
| **Estimated runtime** | ~60–120 seconds (hermetic); live smoke gated/skipped |

---

## Sampling Rate

- **After every task commit:** Run the quick run command for touched packages
- **After every plan wave:** Run the full suite command
- **Before `/gsd-verify-work`:** Full suite must be green
- **Max feedback latency:** ~120 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|-----------|-------------------|-------------|--------|
| (planner fills from RESEARCH Validation Architecture) | | | ADAPTER-SWE-01 / VERIFIED-01 / VERIFIED-02 | unit(fixture) | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] Additive `container_id` (string omitempty) + `exit_code` (`*int`) keys on result.v2 (result.go + schema), backward-compat test (absent keys still validate)
- [ ] harness run-report + per-instance report.json → result.v2 ingestion hermetic test over committed fixtures (resolved + tests_status.FAIL_TO_PASS/PASS_TO_PASS)
- [ ] predictions.jsonl producer hermetic test (instance_id, model_name_or_path, model_patch)
- [ ] UTBoost rescorer hermetic test: pure compare of canonical vs augmented report JSON
- [ ] 3-condition `verified_correctness` producer (canonical ∧ augmented ∧ no-regress) cloned from completion_gate; abstain → explicit `&false`; computed INDEPENDENTLY of task_success
- [ ] **SC#2 LOAD-BEARING hermetic test: known-buggy patch → `task_success=true` AND `verified_correctness=false`**
- [ ] `differential.go` hermetic test: gold-patch vs agent-patch diff-overlap signal; run-all-tests override
- [ ] aggregator raw-vs-UTBoost-rescored side-by-side column (additive, goldens byte-identical); `--run-id` reproducibility

---

## Manual-Only / Docker-Gated Verifications

| Behavior | Requirement | Why Gated | Test Instructions |
|----------|-------------|-----------|-------------------|
| SWE-bench Verified 5-task smoke via subprocess to upstream harness | ADAPTER-SWE-01 | Needs Docker + the swebench Python package + per-instance images + network | On a Docker host with swebench installed, run the adapter against 5 pinned instances; confirm result.v2 rows with container_id + exit_code |
| Live UTBoost-augmented rescore vs raw upstream side-by-side | VERIFIED-02 | Needs the live harness run + the pinned UTBoost dataset | Run with `--dataset_name <UTBoost pin>`; compare raw vs rescored in the report |
| UTBoost dataset name + pin rev / report log-dir path / run-all-tests mechanism | VERIFIED-02 | 5 [ASSUMED] upstream details (A1–A5) | checkpoint:human-verify — confirm the pinned UTBoost dataset + harness paths against upstream |

*All ingestion/rescore/verified_correctness/differential decision logic has hermetic automated proof via committed fixtures; the SC#2 buggy-patch divergence is hermetic.*

---

## Validation Sign-Off

- [ ] Ingestion, UTBoost rescorer, 3-condition verified_correctness, and differential each have a hermetic fixture test (sole proof)
- [ ] SC#2 known-buggy-patch test (task_success=true AND verified_correctness=false) is hermetic and present
- [ ] No live/Docker test is the sole proof of a success criterion
- [ ] verified_correctness computed independently of task_success; abstain → explicit false (never nil, never false-true)
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** planned
