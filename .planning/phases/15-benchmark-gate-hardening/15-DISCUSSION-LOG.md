# Phase 15: Benchmark Gate Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-10
**Phase:** 15-benchmark-gate-hardening
**Areas discussed:** Baseline capture strategy, Gate rollout approach, Baseline freshness policy

---

## Baseline Capture Strategy

| Option | Description | Selected |
|--------|-------------|----------|
| workflow_dispatch run | Trigger bench.yml manually via GitHub Actions workflow_dispatch. Capture output artifact, copy numbers into v1.1-github-hosted.txt, commit. | |
| Dedicated baseline workflow | Create a separate capture-baseline.yml that runs benchmarks and auto-commits the results. More automated but adds CI complexity. | ✓ |
| First real PR captures it | Modify bench.yml to detect empty baseline and auto-save on first run. Self-bootstrapping but adds conditional logic. | |

**User's choice:** Dedicated baseline workflow
**Notes:** None — straightforward preference for separation of concerns.

### Follow-up: Auto-commit behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Auto-commit to branch | Workflow writes baseline file and commits directly to triggering branch. Fast, no manual step. Requires contents:write. | ✓ |
| Artifact + manual commit | Workflow uploads results as downloadable artifact. Review and commit manually. | |
| Auto-commit with PR | Workflow creates a PR with new baseline. Review checkpoint without manual file handling. | |

**User's choice:** Auto-commit to branch
**Notes:** None

---

## Gate Rollout Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Remove immediately | Delete --warn-only from bench.yml. Gate starts blocking PRs right away once real baselines are committed. | ✓ |
| Remove with escape hatch | Remove --warn-only but add workflow input to temporarily re-enable for emergency merges. | |
| Conditional on baseline existence | Keep --warn-only until baseline file has real content, then auto-switch to blocking. | |

**User's choice:** Remove immediately
**Notes:** Completes Phase 9's three-step rollout (placeholder → warn-only → blocking).

---

## Baseline Freshness Policy

| Option | Description | Selected |
|--------|-------------|----------|
| Per-milestone re-baseline | Re-run baseline capture workflow at start of each new milestone. Document policy in baselines/README.md. | ✓ |
| Manual on-demand only | Refresh baselines whenever performance shifts are expected. Developer judgment. | |
| Quarterly schedule | Re-baseline on fixed cadence regardless of milestone timing. | |

**User's choice:** Per-milestone re-baseline
**Notes:** Aligns with version lifecycle.

## Claude's Discretion

No areas deferred to Claude's discretion.

## Deferred Ideas

None — discussion stayed within phase scope.
