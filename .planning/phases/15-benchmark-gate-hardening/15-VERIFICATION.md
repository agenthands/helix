---
phase: 15-benchmark-gate-hardening
verified: 2026-04-10T21:30:00Z
status: gaps_found
score: 4/5 must-haves verified
overrides_applied: 0
gaps:
  - truth: "v1.1-github-hosted.txt contains real benchmark numbers from an ubuntu-latest CI run, replacing the PLACEHOLDER content"
    status: failed
    reason: "The baseline file still contains the PLACEHOLDER header with no Benchmark lines. The capture-baseline.yml workflow was created to produce real numbers but has not been run yet — it can only execute on GitHub Actions."
    artifacts:
      - path: "test/bench/baselines/v1.1-github-hosted.txt"
        issue: "Still contains PLACEHOLDER content (0 Benchmark lines). Line 1: '# PLACEHOLDER -- populated by the first ubuntu-latest CI run.'"
    missing:
      - "Run capture-baseline.yml workflow on GitHub Actions to populate v1.1-github-hosted.txt with real benchmark numbers"
      - "Verify the resulting file has at least 5 Benchmark lines before merging to main"
human_verification:
  - test: "Trigger capture-baseline.yml workflow on GitHub Actions"
    expected: "Workflow runs bench suite on ubuntu-latest, captures real numbers, auto-commits to branch. v1.1-github-hosted.txt should contain 10+ Benchmark lines."
    why_human: "Requires GitHub Actions execution environment — cannot be verified locally or programmatically in this context"
  - test: "Open a PR after baseline is captured and verify benchgate blocks"
    expected: "benchgate runs in blocking mode and exits 0 when no regressions exceed thresholds, or exits non-zero if regressions are found"
    why_human: "End-to-end CI gate behavior requires a real PR against main with the bench workflow running"
---

# Phase 15: Benchmark Gate Hardening Verification Report

**Phase Goal:** Complete the partial BENCH-05/BENCH-06 requirements -- make the CI benchstat gate enforce real thresholds against real baseline numbers
**Verified:** 2026-04-10T21:30:00Z
**Status:** gaps_found
**Re-verification:** No -- initial verification

## Goal Achievement

### Observable Truths

| # | Truth | Status | Evidence |
|---|-------|--------|----------|
| 1 | bench.yml invokes benchgate WITHOUT --warn-only -- the CI gate blocks regressing PRs | VERIFIED | `grep -i "warn-only" bench.yml` returns exit 1 (zero matches). Benchgate invoked at lines 87-90 without --warn-only flag. Step name is "Run benchgate" (no WARN-ONLY suffix). |
| 2 | v1.1-github-hosted.txt contains real benchmark numbers from ubuntu-latest CI run, replacing PLACEHOLDER | FAILED | File still contains PLACEHOLDER header (line 1). `grep -c "^Benchmark"` returns 0. The capture-baseline.yml workflow exists to produce these numbers but has not been triggered yet. |
| 3 | A dedicated capture-baseline.yml workflow exists for re-baselining on ubuntu-latest | VERIFIED | `.github/workflows/capture-baseline.yml` exists with workflow_dispatch trigger, ubuntu-latest runner, matching env (GOMAXPROCS="4", GOPLS_VERSION=v0.17.1, Go 1.25.x). |
| 4 | capture-baseline.yml auto-commits results to the triggering branch with contents:write | VERIFIED | Line 22: `contents: write` permission. Line 64: `stefanzweifel/git-auto-commit-action@v5`. File pattern restricted to `test/bench/baselines/*.txt`. |
| 5 | baselines/README.md documents the per-milestone re-baseline policy and capture workflow usage | VERIFIED | "Re-baseline workflow" section present. "refreshed per milestone" policy documented. Step-by-step trigger instructions (Actions > capture-baseline > Run workflow). Three-step rollout updated to COMPLETED status. |

**Score:** 4/5 truths verified

### Required Artifacts

| Artifact | Expected | Status | Details |
|----------|----------|--------|---------|
| `.github/workflows/capture-baseline.yml` | Dedicated baseline capture workflow per D-01 | VERIFIED | Contains workflow_dispatch, contents:write, git-auto-commit-action@v5, matching env/flags |
| `.github/workflows/bench.yml` | PR benchmark gate in blocking mode per D-03 | VERIFIED | No warn-only references. Benchgate invoked with --baseline and --new flags only. |
| `test/bench/baselines/README.md` | Updated refresh policy documenting capture workflow per D-04 | VERIFIED | Contains capture-baseline references, per-milestone policy, trigger instructions |
| `test/bench/baselines/v1.1-github-hosted.txt` | Real benchmark numbers from CI | FAILED | Still PLACEHOLDER with 0 Benchmark lines |

### Key Link Verification

| From | To | Via | Status | Details |
|------|----|----|--------|---------|
| capture-baseline.yml | test/bench/baselines/ | git-auto-commit-action | VERIFIED | Line 64: `stefanzweifel/git-auto-commit-action@v5` with file_pattern `test/bench/baselines/*.txt` |
| bench.yml | test/bench/baselines/v1.1-github-hosted.txt | --baseline flag | VERIFIED | Line 89: `--baseline test/bench/baselines/v1.1-github-hosted.txt` |

### Data-Flow Trace (Level 4)

Not applicable -- these are CI workflow YAML files, not components rendering dynamic data.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
|----------|---------|--------|--------|
| No warn-only in bench.yml | `grep -i "warn-only" bench.yml` | Exit 1 (no matches) | PASS |
| capture-baseline.yml has workflow_dispatch | `grep "workflow_dispatch" capture-baseline.yml` | Found at line 14 | PASS |
| Baseline file has real numbers | `grep -c "^Benchmark" v1.1-github-hosted.txt` | 0 lines | FAIL |
| Env parity: GOMAXPROCS | Both files contain `GOMAXPROCS: "4"` | Match confirmed | PASS |
| Env parity: GOPLS_VERSION | Both files contain `GOPLS_VERSION: v0.17.1` | Match confirmed | PASS |
| Commits verified | `git log --oneline 3f523884 893486f6` | Both exist | PASS |

### Requirements Coverage

| Requirement | Source Plan | Description | Status | Evidence |
|-------------|------------|-------------|--------|----------|
| BENCH-05 | 15-01-PLAN | CI benchstat regression gate with tiered thresholds | PARTIAL | Gate is in blocking mode (--warn-only removed) but baseline file is still PLACEHOLDER -- gate will fail to produce meaningful comparisons until real numbers are captured |
| BENCH-06 | 15-01-PLAN | v1.1 baselines committed to test/bench/baselines/ | PARTIAL | Capture workflow created but v1.1-github-hosted.txt still contains PLACEHOLDER content. Tooling is ready; execution on CI is needed. |

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
|------|------|---------|----------|--------|
| test/bench/baselines/v1.1-github-hosted.txt | 1 | PLACEHOLDER header still present | Blocker | benchgate will see 0 benchmark comparisons -- effectively a no-op gate until real numbers are captured |
| .github/workflows/bench.yml | 61 | TODO: pin to commit SHA for supply-chain hardening | Info | Pre-existing from Phase 9, not introduced by this phase |
| test/bench/baselines/README.md | 37,39 | "placeholder" in historical context (Step A description) | Info | Historical documentation describing the rollout process -- not an active placeholder |

### Human Verification Required

### 1. Trigger capture-baseline.yml on GitHub Actions

**Test:** Go to Actions > capture-baseline > Run workflow > enter milestone tag "v1.1" > select target branch
**Expected:** Workflow runs bench suite on ubuntu-latest, captures real Benchmark lines, auto-commits v1.1-github-hosted.txt with real numbers (replacing PLACEHOLDER content)
**Why human:** Requires GitHub Actions execution environment -- benchmarks must run on ubuntu-latest to produce architecture-appropriate numbers

### 2. Verify blocking gate on a real PR

**Test:** After baseline capture, open a PR against main and check the bench workflow run
**Expected:** benchgate runs without --warn-only, compares PR benchmark output against real baseline numbers, exits 0 if no significant regressions
**Why human:** End-to-end CI gate behavior requires a real PR with the bench workflow executing on GitHub-hosted runners

### Gaps Summary

One gap blocks full goal achievement: **the v1.1 baseline file still contains PLACEHOLDER content** (0 Benchmark lines). This is Roadmap Success Criterion #2 which explicitly requires "real benchmark numbers from an ubuntu-latest CI run, replacing the PLACEHOLDER content."

The phase successfully created all the tooling needed to close this gap:
- `capture-baseline.yml` workflow is ready to trigger on GitHub Actions
- `bench.yml` is in blocking mode with --warn-only removed
- `baselines/README.md` documents the capture process

The remaining step is operational: trigger the capture-baseline.yml workflow on GitHub Actions, which will auto-commit real numbers to v1.1-github-hosted.txt. This cannot be done locally -- it requires the CI environment.

**Root cause:** The phase plan focused on creating the capture workflow and removing --warn-only, but did not account for actually running the capture workflow to produce baseline numbers. This is an inherent constraint -- baseline numbers must come from CI, not local development.

**Impact on gate effectiveness:** Until real baseline numbers exist, the blocking benchgate will either (a) find 0 comparisons and exit 0 (false pass), or (b) error on parse (fail all PRs). Either outcome means the gate is not enforcing real thresholds yet.

---

_Verified: 2026-04-10T21:30:00Z_
_Verifier: Claude (gsd-verifier)_
