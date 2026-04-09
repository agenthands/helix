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

## Two-step rollout (addresses revision BLOCKER 4)

Benchmark numbers are architecture-, kernel-, and runner-specific.
Capturing the initial baseline on a developer machine (e.g. darwin/arm64)
and enforcing it on `ubuntu-latest` would produce meaningless deltas and
PR failures on every run. The baseline **must** be captured on the same
runner class that will later enforce the gate.

We therefore roll out in three steps:

### Step A — commit the placeholder (this phase)

- `v1.1-github-hosted.txt` is committed as a placeholder with a
  header stanza only (no `Benchmark` lines).
- `benchgate` and `.github/workflows/bench.yml` both reference this
  real on-disk path so everything builds and runs.
- Developer machines **must not** regenerate this file locally. The
  placeholder carries a prominent `PLACEHOLDER` marker to discourage
  accidental `go test -bench` captures.

### Step B — first CI run in `--warn-only` mode

- `bench.yml` runs the full bench suite on `ubuntu-latest` with
  `GOMAXPROCS=4`, `-count=10`, `-short` (skipping `BenchmarkFullRepoSmoke`
  per Pitfall 11).
- The captured benchfmt output is uploaded as the artifact
  `v1.1-baseline-candidate.txt`.
- `benchgate` is invoked with `--warn-only`, which prints the full
  delta report but **always exits 0** — no PR is ever blocked during
  this phase.
- A human downloads the artifact, eyeballs it for sanity (no zeroes,
  no obvious cold-start outliers, expected benchmarks present), and
  commits it to this directory as a dedicated re-baseline PR titled
  `bench: seed v1.1 baseline from first CI capture`.

### Step C — flip to blocking mode

- A follow-up PR removes `--warn-only` from `bench.yml`.
- From that point on, `benchgate` enforces the D-01 tiered thresholds
  and any significant regression blocks merge.

## Baseline capture command (CI only)

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

## Refresh policy

Per `.planning/phases/09-benchmark-harness-v1-1-baseline/09-RESEARCH.md`
Pitfall 8, baselines are **refreshed only on intentional re-baseline
PRs** during release cuts. A re-baseline PR must:

1. Be titled `bench: re-baseline <reason>`.
2. Include the raw CI artifact that produced the new numbers.
3. Cite measurement justification in the PR body (what changed, why
   the old numbers are no longer representative).
4. Be reviewed by someone other than the PR author.

**The committed file IS the v1.1 reference.** Do not modify it without
an intentional re-baseline PR with measurement justification. Developer
machines **must not** regenerate this file locally — only CI captures
are authoritative.
