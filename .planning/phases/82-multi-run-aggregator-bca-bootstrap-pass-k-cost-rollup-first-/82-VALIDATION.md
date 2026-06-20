---
phase: 82
slug: multi-run-aggregator-bca-bootstrap-pass-k-cost-rollup-first-leaderboard
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-20
---

# Phase 82 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.
> Source: 82-RESEARCH.md § Validation Architecture. The aggregator is pure-Go and
> mostly unit-testable WITHOUT `HELIX_BIN` (D-01) — only an end-to-end smoke needs real run data.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go stdlib `testing` (table-driven) |
| **Config file** | none — `go test` convention |
| **Quick run command** | `go test ./bench/aggregator/...` |
| **Full suite command** | `go test ./... && go vet ./... && make vet` |
| **Estimated runtime** | ~10–30 seconds (pure unit; BCa 10k resamples is fast) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/aggregator/...`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite + `make vet` must be green
- **Max feedback latency:** ~30 seconds

> Note: any task that touches `bench/runtime/` matrix code or adds an end-to-end aggregator
> smoke MUST be re-run with `HELIX_BIN="$(pwd)/helix"` set — bench integration tests SKIP
> (false-green) when HELIX_BIN is unset. Pure aggregator unit tests do NOT need HELIX_BIN.

---

## Per-Task Verification Map

> Task IDs are assigned by the planner; this maps each phase requirement to its automated proof
> (from 82-RESEARCH.md § Phase Requirements → Test Map). Every requirement has an automated test.

| Requirement | Behavior | Test Type | Automated Command | File (Wave 0) | Status |
|-------------|----------|-----------|-------------------|---------------|--------|
| STATS-01 | N-gate fail-closes when a (task,mode) cell has < expectedN valid rows; no reports written | unit | `go test ./bench/aggregator/ -run TestNGate` | `bench/aggregator/load_test.go` | ⬜ pending |
| STATS-02 | BCa CI: closed-form coverage + width sanity + BCa≠percentile on skew + determinism | unit | `go test ./bench/aggregator/ -run TestBCa` | `bench/aggregator/bootstrap_test.go` | ⬜ pending |
| STATS-03 | pass@1/pass@k match published + hand-computed reference values; product form == lgamma form | unit | `go test ./bench/aggregator/ -run TestPassAtK` | `bench/aggregator/passk_test.go` | ⬜ pending |
| STATS-04 | overlap warning rendered for synthetic-overlap fixture, absent for non-overlap | unit | `go test ./bench/aggregator/ -run TestOverlapGate` | `bench/aggregator/report_test.go` | ⬜ pending |
| COST-02 | cost_per_solved_task matches hand-computed golden fixture; freshness-gate fail-closed | unit | `go test ./bench/aggregator/ -run TestCostPerSolved` | `bench/aggregator/cost_test.go` | ⬜ pending |
| COST-03 | cost_quality.md renders cost+BCa CI + FAIR-03 variance warning for sample run | unit | `go test ./bench/aggregator/ -run TestCostQualityRender` | `bench/aggregator/report_test.go` | ⬜ pending |
| D-04 | ExpandMatrix emits N cells with RunIndex 0..N-1 | unit | `go test ./bench/runtime/ -run TestExpandMatrixRuns` | `bench/runtime/matrix_test.go` | ⬜ pending |
| D-08 | same seed ⇒ byte-identical reports | unit | `go test ./bench/aggregator/ -run TestDeterministic` | `bench/aggregator/report_test.go` | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/aggregator/bootstrap_test.go` — STATS-02 (closed-form coverage, CI-width sanity, BCa≠percentile on skew, determinism)
- [ ] `bench/aggregator/passk_test.go` — STATS-03 (published + hand-computed reference values, product-form == lgamma-form, k≥2 case to catch the naive estimator)
- [ ] `bench/aggregator/cost_test.go` — COST-02 (golden hand-computed cost_per_solved_task) + cost-table freshness-gate fail-closed
- [ ] `bench/aggregator/report_test.go` — STATS-04 overlap gate + FAIR-03 variance warning + COST-03 render + determinism
- [ ] `bench/aggregator/load_test.go` — STATS-01 N-gate (deficient cell → hard error, no reports written; expected-N from manifest/flag, not disk count)
- [ ] `bench/aggregator/testdata/` — synthetic `result.v2.json` fixtures (skewed, overlap, high-variance, golden-cost) + golden `.md` outputs
- [ ] `bench/runtime/matrix_test.go` extension — D-04 N-cell expansion
- [ ] Framework install: none — Go stdlib `testing` already in use

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| End-to-end aggregate over a real multi-run matrix | STATS-01/02 (integration) | Requires a real daemon-produced N-run reports tree | Build `helix`, run a small N≥3 matrix, run `helix-bench aggregate <run_dir>` with `HELIX_BIN` set; confirm leaderboard.md + cost_quality.md render |

*All requirement-level behaviors have automated unit verification; the manual item is an optional integration confidence check.*

---

## Validation Sign-Off

- [ ] All tasks have automated verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter (set by planner once plans satisfy the map)

**Approval:** pending
