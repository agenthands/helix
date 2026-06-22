---
phase: 89
slug: reports-ci-policy-contamination-canary
status: planned
nyquist_compliant: true
wave_0_complete: false
created: 2026-06-21
---

# Phase 89 — Validation Strategy

> Seeded from the "Validation Architecture" section of 89-RESEARCH.md. Governing rule (MEMORY
> false-green + Phase 81 vacuous-gate): the 4 report renderers, the verified_correctness reduce, the
> canary exclusion + footnote, the ablations deltas, and the `report --run-id` byte-reproducibility are
> all pure logic — their HERMETIC golden-report tests over a committed multi-run fixture tree are the
> SOLE authoritative proof. The CI workflow YAML is verified by a parse/lint test + inspection, NOT a
> live CI run. No live benchmark run is needed (this phase consumes existing result.v2 rows).
> THE LOAD-BEARING PROOF (SC#1): `report --run-id` regenerated reports `diff`-empty vs the originals
> (a double-render golden test). Additive columns/footnote regenerate the frozen Phase-82/85/86/87
> goldens — those golden guards MUST be updated in lockstep (Pitfall 1), not bypassed.

---

## Test Infrastructure

| Property | Value |
|----------|-------|
| **Framework** | go test (committed multi-run fixture tree + golden .md reports) |
| **Config file** | none — standard Go toolchain |
| **Quick run command** | `go vet ./bench/... ./cmd/helix-bench/... && go test ./bench/aggregator/... ./bench/canary/... ./cmd/helix-bench/...` |
| **Full suite command** | `go test ./...` (no live bench run; CI YAML parse-tested) |
| **Estimated runtime** | ~60–120 seconds (fully hermetic) |

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
| (planner fills from RESEARCH Validation Architecture) | | | REPORT-01..05 / INFRA-04 / INFRA-05 | unit(golden) | | ❌ W0 | ⬜ pending |

*Status: ⬜ pending · ✅ green · ❌ red · ⚠️ flaky*

---

## Wave 0 Requirements

- [ ] `reduceVerifiedCorrectness` + the leaderboard `verified_correctness` column (REPORT-01) golden test
- [ ] `per_language.md` renderer (languages with no coverage shown as `n/a`, NOT omitted) golden test (REPORT-02/REPORT-04)
- [ ] `ablations.md` delta tables (full vs no_lsp / no_semantic / no_structured_edit / baseline_plain / baseline_rag) with BCa CI-overlap analysis; `full vs no_semantic` computed at aggregate-time golden test (REPORT-03)
- [ ] `cost_quality.md` scatter (cost vs verified_correctness) ASCII/svg + cost-table `valid_until` citation golden test (REPORT-03/REPORT-05)
- [ ] Canary: production `InjectPrompt` caller + aggregate-time EXCLUSION of contaminated rows from headline + `leaderboard.md` footnote listing flagged tasks; synthetic contaminated-response test trips the flag (INFRA-05)
- [ ] `helix-bench report --run-id <id>` regenerates ALL 4 reports BYTE-IDENTICALLY (double-render diff-empty golden test) (REPORT-01, the load-bearing proof)
- [ ] CI workflow file (`make bench-quick` on PR, hard 5-min cap, ToolBench-Go-only no-LLM-cost; full `make bench` nightly/maintainer-label-gated; documented cost budget) + a YAML parse/lint test (INFRA-04)
- [ ] Frozen Phase-82/85/86/87 leaderboard/cost goldens updated in lockstep with the new columns/footnote (NOT bypassed)

---

## Manual-Only / Inspection-Gated Verifications

| Behavior | Requirement | Why Gated | Test Instructions |
|----------|-------------|-----------|-------------------|
| Live CI `make bench-quick` ≤5min on PR + full `make bench` nightly | INFRA-04 | Needs the live GitHub Actions runner + a real PR | After merge, open a PR; confirm bench-quick runs ≤5min and full bench is maintainer-label-gated |

*All report rendering, byte-reproducibility, canary exclusion, ablation deltas, and the CI-YAML structure have hermetic automated proof; only the live CI execution is inspection/runner-gated.*

---

## Validation Sign-Off

- [ ] All 4 renderers + verified_correctness reduce + canary exclusion + ablation deltas each have a hermetic golden test (sole proof)
- [ ] `report --run-id` byte-reproducibility (diff-empty) proven by a double-render golden test
- [ ] Frozen prior-phase goldens updated in lockstep (not bypassed); CI YAML parse-tested
- [ ] No live test is the sole proof of a success criterion
- [ ] Sampling continuity: no 3 consecutive tasks without automated verify
- [ ] No watch-mode flags
- [ ] Feedback latency < 120s
- [ ] `nyquist_compliant: true` set in frontmatter

**Approval:** planned
