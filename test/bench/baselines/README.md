# `test/bench/baselines/` — committed v1.1 reference baselines

This directory holds the **immutable performance reference** against which
every v1.2+ phase publishes its benchstat / benchgate delta. Per
`.planning/phases/09-benchmark-harness-v1-1-baseline/09-CONTEXT.md` D-01,
the goal is a tiered regression gate that prevents silent drift during
the v1.2 Performance & Production Hardening milestone.

## Files

| File | Purpose |
|------|---------|
| `v1.1-github-hosted.txt` | PR-tier baseline captured on GitHub-hosted `ubuntu-latest`. Consumed by `.github/workflows/bench.yml` on every PR via `benchgate`. |

A future `v1.1-self-hosted.txt` will follow once the release-tier
self-hosted runner is provisioned (D-02, post-v1.2).

## Why baselines are committed at all

Benchstat-based regression gates compare a fresh run against an
*archived* reference run. Without a committed reference the gate has
nothing to compare against, and "only fail on regressions relative to
the previous PR" is unsafe — drift compounds silently across PRs. A
committed baseline gives every PR a stable anchor and makes any
intentional re-baselining explicit in git history.

## Three-step rollout (COMPLETED)

Benchmark numbers are architecture-, kernel-, and runner-specific.
Capturing the initial baseline on a developer machine (e.g. darwin/arm64)
and enforcing it on `ubuntu-latest` would produce meaningless deltas and
PR failures on every run. The baseline **must** be captured on the same
runner class that will later enforce the gate.

The rollout completed in three steps:

### Step A — commit the placeholder

- `v1.1-github-hosted.txt` was committed as a placeholder with a
  header stanza only (no `Benchmark` lines).
- `benchgate` and `.github/workflows/bench.yml` both reference this
  real on-disk path so everything builds and runs.

### Step B — first CI run in `--warn-only` mode

- `bench.yml` ran the full bench suite on `ubuntu-latest` with
  `GOMAXPROCS=4`, `-count=10`, `-short` (skipping `BenchmarkFullRepoSmoke`
  per Pitfall 11).
- `benchgate` was invoked with `--warn-only`, which printed the full
  delta report but always exited 0 — no PR was blocked during this phase.

### Step C — flip to blocking mode (automated via capture-baseline.yml)

- `--warn-only` has been removed from `bench.yml` (Phase 15).
- `benchgate` now enforces the D-01 tiered thresholds and any
  significant regression blocks merge.
- Re-baselining is automated via the `capture-baseline.yml` workflow
  — no manual artifact download/commit needed.

## Baseline capture procedure (CI only)

This is the exact command the workflow runs. **Do not run it on a
developer machine**; only CI captures are authoritative.

```sh
# PR tier — GitHub-hosted ubuntu-latest (D-01)
GOMAXPROCS=4 go test -short -bench=. -benchmem -count=10 -run=^$ \
  ./test/bench/... > /tmp/bench-new.txt

# Release tier — self-hosted (future, post-v1.2)
GOMAXPROCS=4 go test -bench=. -benchmem -count=20 -run=^$ \
  ./test/bench/... > /tmp/bench-new.txt
```

`GOMAXPROCS=4` is pinned per Pitfall 5 to reduce noise on shared
runners (ubuntu-latest has varying core counts depending on when GitHub
rolls out new hardware). `gopls` is pinned in the workflow per
Pitfall 4.

## Tiered threshold table (D-01)

| Tier | Runner | `-count` | Time | Allocs | Alpha |
|------|--------|----------|-----:|-------:|------:|
| PR | `ubuntu-latest` (github-hosted) | 10 | 15% | 25% | 0.05 |
| Release | self-hosted (future) | 20 | 10% | 20% | 0.05 |

`benchgate` enforces `(delta > tier_threshold) AND (p < alpha)` via
Welch's t-test (`benchmath.AssumeNormal.Compare`). p99 is reported by
benchstat for trend-watching but is deliberately **not** gated per
phase Q5 + Pitfall 10 (p99 is too noisy for a hard gate).

## Re-baseline workflow

Per D-04, baselines are **refreshed per milestone** using the dedicated
`capture-baseline.yml` workflow. This replaces the manual
download-and-commit process from the original three-step rollout.

**How to re-baseline:**

1. Go to **Actions > capture-baseline > Run workflow**.
2. Enter the milestone tag (e.g., `v1.2`) and select the target branch.
3. The workflow runs the full bench suite on `ubuntu-latest` with the
   same env/flags as `bench.yml`, verifies the output, and auto-commits
   the results to the triggering branch.
4. No manual download or commit needed — the workflow handles everything.

**When to re-baseline:**

- At the start of each new milestone, to refresh numbers for the new
  development cycle.
- After intentional performance changes that shift the baseline
  (e.g., algorithm improvements, dependency upgrades).
- After Go version bumps or gopls version bumps that affect benchmark
  characteristics.

## Refresh policy

Per `.planning/phases/09-benchmark-harness-v1-1-baseline/09-RESEARCH.md`
Pitfall 8, baselines are **refreshed only intentionally** — never
automatically on every PR. Use the `capture-baseline.yml` workflow to
capture new numbers. For manual re-baseline PRs (e.g., when the capture
workflow cannot be used), the PR must:

1. Be titled `bench: re-baseline <reason>`.
2. Include the raw CI artifact that produced the new numbers.
3. Cite measurement justification in the PR body (what changed, why
   the old numbers are no longer representative).
4. Be reviewed by someone other than the PR author.

**The committed file IS the v1.1 reference.** Do not modify it without
an intentional re-baseline via the capture workflow or a dedicated PR
with measurement justification. Developer machines **must not**
regenerate this file locally — only CI captures are authoritative.
