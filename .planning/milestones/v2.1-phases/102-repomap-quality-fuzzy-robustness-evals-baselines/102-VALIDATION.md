---
phase: 102
slug: repomap-quality-fuzzy-robustness-evals-baselines
status: draft
nyquist_compliant: false
wave_0_complete: false
created: 2026-06-23
---

# Phase 102 — Validation Strategy

> Per-phase validation contract for feedback sampling during execution.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | Go `testing` + `testify` (`require`/`assert`) |
| **Config file** | none — standard `go test` |
| **Quick run command** | `go test ./bench/evaluators/repomapeval/ ./bench/evaluators/fuzzyrobust/` |
| **Full suite command** | `go test ./... && go vet ./...` |
| **Estimated runtime** | ~30 seconds (hermetic; no binary, no network) |

---

## Sampling Rate

- **After every task commit:** Run `go test ./bench/evaluators/repomapeval/ ./bench/evaluators/fuzzyrobust/ ./bench/aggregator/`
- **After every plan wave:** Run `go test ./... && go vet ./...`
- **Before `/gsd-verify-work`:** Full suite green; hermetic golden tests pass with NO `HELIX_BIN` and NO network.
- **Max feedback latency:** ~30 seconds

---

## Per-Task Verification Map

| Task ID | Plan | Wave | Requirement | Threat Ref | Secure Behavior | Test Type | Automated Command | File Exists | Status |
|---------|------|------|-------------|------------|-----------------|-----------|-------------------|-------------|--------|
| 102-RM-metrics | repomapeval | 1 | REPOEVAL-01 | — | N/A | unit | `go test ./bench/evaluators/repomapeval/ -run TestMetrics` | ❌ W0 | ⬜ pending |
| 102-RM-leaf | repomapeval | 1 | REPOEVAL-01 | — | leaf imports stdlib + editsim only | unit (self-test) | `go test ./bench/evaluators/repomapeval/ -run TestLeafImports` | ❌ W0 | ⬜ pending |
| 102-RM-floor | repomapeval | 1 | REPOEVAL-02 | — | per-language size floor enforced | unit | `go test ./bench/evaluators/repomapeval/ -run TestCorpusFloor` | ❌ W0 | ⬜ pending |
| 102-RM-disc | repomapeval | 1 | REPOEVAL-02 | — | reversed AND random ranker fail by margin | unit (anti-vacuity) | `go test ./bench/evaluators/repomapeval/ -run TestDiscriminator` | ❌ W0 | ⬜ pending |
| 102-FZ-strategy | fuzzyrobust | 1 | FUZZBENCH-01 | — | 4-strategy selection + refusal via editsim.ES | unit | `go test ./bench/evaluators/fuzzyrobust/ -run TestStrategy` | ❌ W0 | ⬜ pending |
| 102-FZ-floor | fuzzyrobust | 1 | FUZZBENCH-02 | — | drift per-tier, strategy from drift type, floor | unit | `go test ./bench/evaluators/fuzzyrobust/ -run TestCorpusFloor` | ❌ W0 | ⬜ pending |
| 102-FZ-ambig | fuzzyrobust | 1 | FUZZBENCH-02 | — | duplicate-block ambiguous case MUST be refused | unit (anti-vacuity) | `go test ./bench/evaluators/fuzzyrobust/ -run TestAmbiguousRefused` | ❌ W0 | ⬜ pending |
| 102-BL-repro | aggregator | 2 | BASELINE-02 | — | committed baseline schema-ok, byte-reproducible | unit | `go test ./bench/aggregator/ -run TestRepoMapEvalBaseline` | ❌ W0 | ⬜ pending |
| 102-BL-antivac | aggregator | 2 | BASELINE-02 | — | stripped metric fails the baseline assertion | unit (anti-vacuity) | `go test ./bench/aggregator/ -run TestRepoMapEvalBaselineAntiVacuity` | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `bench/evaluators/repomapeval/repomapeval_test.go` — covers REPOEVAL-01 (metrics)
- [ ] `bench/evaluators/repomapeval/discriminator_test.go` — covers REPOEVAL-02 (anti-vacuity)
- [ ] `bench/evaluators/repomapeval/testdata/` — committed gold corpus + captured ranking fixtures
- [ ] `bench/evaluators/fuzzyrobust/fuzzyrobust_test.go` — covers FUZZBENCH-01
- [ ] `bench/evaluators/fuzzyrobust/ambiguous_test.go` — covers FUZZBENCH-02 (must-refuse)
- [ ] `bench/aggregator/repomap_eval_baseline_test.go` + `fuzzy_robust_baseline_test.go` — covers BASELINE-02 (mirror `aider_edit_baseline_test.go`)
- [ ] `.gitignore` allowlist entries for `bench/reports/repomap-eval-baseline/` + `fuzzy-robust-baseline/` (mirror lines 275-276)
- [ ] `Makefile` regen targets (mirror `bench-aider-edit` at line 160)
- [ ] Leaf import self-test (since `vet-ablation-leakage` does NOT gate `bench/evaluators/*`)

---

## Manual-Only Verifications

| Behavior | Requirement | Why Manual | Test Instructions |
|----------|-------------|------------|-------------------|
| Live `HELIX_BIN`-gated ranking capture produces a non-empty captured-ranking artifact | BASELINE-02 | Requires a built `helix` binary + warm daemon; the committed leaf test is hermetic and cannot exercise the live capture leg | With `HELIX_BIN` set to a freshly built helix, run the regen target and confirm it FAILS (not SKIPs) on empty/missing output, then produces the committed artifact byte-identically on a second render |

*All other phase behaviors have automated, hermetic verification.*

---

## Validation Sign-Off

- [ ] All tasks have `<automated>` verify or Wave 0 dependencies
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] Wave 0 covers all MISSING references
- [ ] No watch-mode flags
- [ ] Feedback latency < 30s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** pending
