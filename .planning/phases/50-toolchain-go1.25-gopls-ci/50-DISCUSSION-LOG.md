# Phase 50: toolchain-go1.25-gopls-ci - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-25
**Phase:** 50-toolchain-go1.25-gopls-ci
**Areas discussed:** Baseline strategy, gopls strategy doc, PR threshold tuning, Verification scope

---

## Baseline strategy

### Q1: How should we handle the benchmark baseline files for ubuntu-latest?

| Option | Description | Selected |
|--------|-------------|----------|
| New v1.9 baseline only (Recommended) | Run capture-baseline.yml with milestone=v1.9, switch bench.yml + benchgate to compare against v1.9-github-hosted.txt. Old darwin baselines stay as historical reference. | ✓ |
| New v1.9 baseline, delete old placeholders | Capture v1.9 AND delete v1.1/v1.2-phase10/11/12 darwin/arm64 placeholder files. | |
| Refresh existing v1.1 file in place | Re-run capture-baseline.yml with milestone=v1.1, overwriting the placeholder file. No bench.yml or benchgate changes. | |

**User's choice:** New v1.9 baseline only

### Q2: Should the v1.9 baseline cover the full bench suite or only the existing -short subset?

| Option | Description | Selected |
|--------|-------------|----------|
| -short subset (matches PR gate) (Recommended) | Mirror bench.yml exactly: -short, -count=10, skips BenchmarkFullRepoSmoke. Pitfall 1 parity preserved. | ✓ |
| Full suite incl. FullRepoSmoke | Capture full bench so release-tier gate has data too. ~10-15min runtime. | |

**User's choice:** -short subset

---

## gopls strategy doc

### Q1: How should the gopls strategy be documented in CONTRIBUTING.md?

| Option | Description | Selected |
|--------|-------------|----------|
| Pin section + bump policy (Recommended) | Subsection: current pin (v0.21.1), why, where it's set, bump policy citing test/bench/baselines/README.md. | ✓ |
| Full incident writeup | History of v0.17.1 incompatibility, why upgrade chosen over patch/replacement, plus pin and policy. | |
| Minimal mention | One line in a 'CI invariants' bullet list. No history, no policy text. | |

**User's choice:** Pin section + bump policy

### Q2: Should the gopls version be deduplicated to a single source of truth?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep duplicated env vars (Recommended) | Leave GOPLS_VERSION declared in bench.yml and capture-baseline.yml. CONTRIBUTING.md note that they MUST match (Pitfall 1). | ✓ |
| Single source via reusable workflow | Composite/reusable workflow defines GOPLS_VERSION once. | |
| Single source via repo file | Tracked file (e.g., .github/versions.env). | |

**User's choice:** Keep duplicated env vars

---

## PR threshold tuning

### Q1: What approach to PR-tier thresholds (currently 15% time / 25% allocs at p<0.05)?

| Option | Description | Selected |
|--------|-------------|----------|
| Keep as-is, tune later if noisy (Recommended) | Land v1.9 baseline, ship with current 15%/25%, observe first ~5-10 PR runs. | ✓ |
| Capture noise sample first, then set | Run capture-baseline.yml 3-5 times back-to-back, set thresholds to ~3σ above noise. | |
| Loosen to 20%/30% pre-emptively | Start more permissive, tighten if false-positive rate is low. | |

**User's choice:** Keep as-is, tune later if noisy

### Q2: If a PR run is noisy/flaky, what's the escape hatch?

| Option | Description | Selected |
|--------|-------------|----------|
| Manual re-run only (Recommended) | Trust GitHub Actions 'Re-run failed jobs' button. No in-workflow retry logic. | ✓ |
| Auto-retry once on bench step failure | Wrap bench step in retry (e.g., nick-fields/retry@v3). | |
| Document a 'best-of-3' manual procedure | No CI changes; CONTRIBUTING.md note saying 'if benchgate fails, re-run; fail 2/3 = real regression.' | |

**User's choice:** Manual re-run only

---

## Verification scope

### Q1: How do we prove 'green CI on ubuntu-latest' for SC #1 and #2?

| Option | Description | Selected |
|--------|-------------|----------|
| Verification PR (Recommended) | Open a PR containing baseline + CONTRIBUTING.md updates + tech-debt removal. PR's own go-test.yml + bench.yml runs are evidence. | ✓ |
| workflow_dispatch dry-run before PR | Manually trigger workflows on the working branch first, then open the PR. | |
| Trust first merge to main | Land changes; fix-forward if main goes red. | |

**User's choice:** Verification PR

### Q2: Is auditing/fixing the other Go-touching workflows in scope for Phase 50?

| Option | Description | Selected |
|--------|-------------|----------|
| Out of scope — only bench.yml + go-test.yml (Recommended) | Phase 50 SC explicitly names build/vet/test/bench. Keep tight. | ✓ |
| Audit all, fix anything broken | Walk every Go workflow, ensure each is green. | |
| Audit only, defer fixes | List state of every workflow, only fix bench.yml + go-test.yml. Others become follow-ups. | |

**User's choice:** Out of scope — only bench.yml + go-test.yml

---

## Claude's Discretion

- Exact placement / heading wording of the gopls subsection in CONTRIBUTING.md.
- Whether the v1.9 baseline is committed via the existing auto-commit action in capture-baseline.yml or pulled into the verification PR by hand.
- Whether the PROJECT.md edit also adjusts surrounding sentences for flow.

## Deferred Ideas

- Single source of truth for `GOPLS_VERSION` (reusable workflow or `.github/versions.env`).
- Audit of `docker.yml`, `junie.yml`, `pytest.yml`, `publish.yml`, `docs.yaml`, `codespell.yml` for Go 1.25 / ubuntu-latest correctness.
- Pre-launch noise sampling to data-derive PR-tier thresholds.
- Bench retry / auto-retry logic in bench.yml.
- Release-tier baseline (full bench, no `-short`) including BenchmarkFullRepoSmoke.
