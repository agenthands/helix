# Phase 15: Benchmark Gate Hardening - Research

**Researched:** 2026-04-10
**Domain:** CI benchmark gating, GitHub Actions workflows, benchfmt baselines
**Confidence:** HIGH

## Summary

Phase 15 is a narrow, well-scoped gap closure: remove `--warn-only` from `bench.yml` and replace the PLACEHOLDER baseline file with real benchmark numbers. All tooling (benchgate CLI, bench.yml workflow, baselines directory structure) already exists and is tested. The phase completes Phase 9's three-step rollout (placeholder, warn-only, blocking).

The primary challenge is that real baseline numbers MUST come from a CI run on `ubuntu-latest` -- capturing locally on darwin/arm64 would produce architecture-specific values that cause false regressions on every subsequent PR. The CONTEXT.md decision D-01/D-02 addresses this by creating a dedicated `capture-baseline.yml` workflow that auto-commits results.

**Primary recommendation:** Create the capture-baseline workflow first, then modify bench.yml to remove --warn-only. The baseline file must contain benchfmt output from ubuntu-latest with GOMAXPROCS=4, -count=10, -short flags matching the gate workflow exactly.

<user_constraints>
## User Constraints (from CONTEXT.md)

### Locked Decisions
- **D-01:** Create a dedicated baseline capture workflow (e.g., `.github/workflows/capture-baseline.yml`) separate from the gate workflow (`bench.yml`). This keeps the gate logic clean and provides a reusable tool for re-baselining.
- **D-02:** The baseline workflow auto-commits results directly to the triggering branch. Requires `contents: write` permission. No manual review step -- fast and friction-free.
- **D-03:** Remove `--warn-only` from `bench.yml` immediately. No escape hatch, no conditional logic. The gate starts blocking PRs as soon as real baselines are committed.
- **D-04:** Re-baseline per milestone. At the start of each new milestone, trigger the baseline capture workflow to refresh numbers. Document this policy in `test/bench/baselines/README.md`.
- **D-01 (Phase 9):** Tiered thresholds -- PR tier (GitHub-hosted, relaxed): >15% time / >25% allocs at p<0.05; release tier (self-hosted, tight): >10% time / >20% allocs at p<0.05
- **D-02 (Phase 9):** Self-hosted runner setup documented but not blocking for v1.2
- **D-03 (Phase 9):** PR runs use `-count=10`, release runs `-count=20`

### Claude's Discretion
None specified -- all decisions locked.

### Deferred Ideas (OUT OF SCOPE)
None.
</user_constraints>

<phase_requirements>
## Phase Requirements

| ID | Description | Research Support |
|----|-------------|------------------|
| BENCH-05 | CI benchstat regression gate with tiered thresholds per 09-CONTEXT.md D-01 | benchgate CLI already implements thresholds; only `--warn-only` removal needed in bench.yml line 93 |
| BENCH-06 | v1.1 baselines committed to `test/bench/baselines/` | capture-baseline.yml workflow captures real numbers; v1.1-github-hosted.txt replacement |
</phase_requirements>

## Architecture Patterns

### Current Benchmark Infrastructure (already built)

```
.github/workflows/
  bench.yml                          # PR gate workflow (--warn-only to remove)
  capture-baseline.yml               # NEW: dedicated baseline capture
test/bench/
  baselines/
    README.md                        # Rollout docs + refresh policy
    v1.1-github-hosted.txt           # PLACEHOLDER -> real numbers
    v1.2-phase{10,11,12}-github-hosted.txt  # Phase delta baselines (reference)
  cmd/benchgate/
    main.go                          # Gate CLI (fully functional)
    main_test.go                     # Gate unit tests
  *_bench_test.go                    # 12 benchmark functions across 7 files
```

### Benchmark Functions That Will Appear in Baseline

The following 12 benchmarks exist in `test/bench/`. With `-short`, `BenchmarkFullRepoSmoke` is skipped (Pitfall 11). The remaining 11 benchmarks will produce lines in the baseline file: [VERIFIED: grep of test/bench/*_test.go]

| Benchmark | File | Notes |
|-----------|------|-------|
| BenchmarkTools (subtests for 38 tools) | tools_bench_test.go | Produces many sub-benchmark lines |
| BenchmarkLSPIndex_Cold | lsp_index_bench_test.go | Cold LS startup |
| BenchmarkLSPIndex_Warm | lsp_index_bench_test.go | Warm LS reuse |
| BenchmarkMemory | memory_bench_test.go | RSS + heap profiling |
| BenchmarkFullRepoSmoke | fullrepo_smoke_test.go | SKIPPED with -short |
| BenchmarkBaselineMiddleware | metrics_bench_test.go | Phase 11 hot-path |
| BenchmarkTelemetryMiddleware | metrics_bench_test.go | Phase 11 hot-path |
| BenchmarkTelemetryMiddleware_ToolsList | metrics_bench_test.go | Phase 11 variant |
| BenchmarkTracingOffPath | tracing_bench_test.go | Phase 12 tracing disabled |
| BenchmarkTracingOnPath | tracing_bench_test.go | Phase 12 tracing enabled |
| BenchmarkSlogHotPath_Baseline | obs_bench_test.go | Phase 10 slog |
| BenchmarkSlogHotPath_WithContextHandler | obs_bench_test.go | Phase 10 slog+ctx |

### capture-baseline.yml Pattern

The new workflow must: [VERIFIED: bench.yml for matching parameters]

1. **Match bench.yml environment exactly:** Go 1.25.x, GOMAXPROCS=4, gopls pinned to v0.17.1, ubuntu-latest runner
2. **Use identical bench flags:** `-short -bench=. -benchmem -count=10 -run=^$`
3. **Trigger:** `workflow_dispatch` only (manual trigger for re-baselining)
4. **Permissions:** `contents: write` (per D-02, auto-commits to triggering branch)
5. **Auto-commit the output** to `test/bench/baselines/v1.1-github-hosted.txt`

### bench.yml Change

Single surgical edit -- remove `--warn-only` from line 93: [VERIFIED: bench.yml line 92-95]

```yaml
# Before (current):
      - name: Run benchgate (WARN-ONLY during baseline rollout)
        run: |
          go run ./test/bench/cmd/benchgate \
            --warn-only \
            --baseline test/bench/baselines/v1.1-github-hosted.txt \
            --new /tmp/bench-new.txt

# After:
      - name: Run benchgate
        run: |
          go run ./test/bench/cmd/benchgate \
            --baseline test/bench/baselines/v1.1-github-hosted.txt \
            --new /tmp/bench-new.txt
```

### Anti-Patterns to Avoid
- **Capturing baseline locally:** darwin/arm64 numbers on ubuntu-latest gate = false regressions on every PR
- **Conditional --warn-only:** D-03 explicitly forbids escape hatches or conditional logic
- **Using bench.yml for baseline capture:** D-01 requires a separate workflow to keep gate logic clean

## Don't Hand-Roll

| Problem | Don't Build | Use Instead | Why |
|---------|-------------|-------------|-----|
| Benchfmt parsing | Custom text parser | `golang.org/x/perf/benchfmt` | Already used by benchgate; format is complex (Pitfall 9) |
| Statistical comparison | Manual delta calculation | `golang.org/x/perf/benchmath` | Welch's t-test already implemented in benchgate |
| Git auto-commit in CI | Shell script with git config | `stefanzweifel/git-auto-commit-action@v5` | Handles git config, empty commit skip, branch detection |

## Common Pitfalls

### Pitfall 1: Baseline-Gate Parameter Mismatch
**What goes wrong:** Baseline captured with different GOMAXPROCS, -count, or -short flags than the gate run. Benchstat/benchgate sees incompatible distributions.
**Why it happens:** Two workflows drift independently.
**How to avoid:** capture-baseline.yml must mirror bench.yml's env block and bench flags exactly. Document the single-source-of-truth requirement in comments.
**Warning signs:** benchgate reporting "benchmark in baseline not present in new run" warnings.

### Pitfall 2: Empty Baseline After Capture
**What goes wrong:** Some benchmarks require gopls or other external tools not installed in the capture workflow. The baseline file has fewer benchmark lines than expected.
**How to avoid:** capture-baseline.yml must install pinned gopls just like bench.yml does. Verify the captured file has the expected benchmark count before committing.
**Warning signs:** benchgate "missing in new" or "missing in base" warnings.

### Pitfall 3: Auto-Commit Permissions Failure
**What goes wrong:** GitHub Actions workflow_dispatch with `contents: write` may fail if the repository has branch protection requiring PR reviews.
**Why it happens:** Branch protection rules override workflow permissions.
**How to avoid:** The capture workflow should target a branch that allows direct pushes, or the baseline commit should be part of a PR. Since D-02 says "auto-commits directly to triggering branch" with "no manual review step," ensure branch protection on main allows workflow commits or target a non-protected branch.
**Warning signs:** Git push fails in CI with permission denied.

### Pitfall 4: Ordering -- Gate Blocks Before Baseline Exists
**What goes wrong:** If --warn-only is removed before real baseline numbers are committed, every PR fails because benchgate cannot parse the PLACEHOLDER file (no Benchmark lines = 0 comparisons, or parse errors).
**Why it happens:** Wrong commit ordering.
**How to avoid:** The plan must ensure real baseline numbers are committed BEFORE --warn-only is removed. Either same commit or baseline-first ordering.
**Warning signs:** All PRs fail benchgate with parse errors or "0 comparisons."

### Pitfall 5: Stale Step Name in bench.yml
**What goes wrong:** The step name still says "WARN-ONLY during baseline rollout" after removing --warn-only.
**How to avoid:** Update the step name and associated comments when removing the flag.

## Code Examples

### capture-baseline.yml Workflow

```yaml
# Source: constructed from bench.yml pattern + D-01/D-02 decisions
name: capture-baseline

on:
  workflow_dispatch:
    inputs:
      milestone:
        description: 'Milestone tag for baseline file (e.g., v1.1)'
        required: true
        default: 'v1.1'

permissions:
  contents: write

jobs:
  capture:
    name: Capture baseline (ubuntu-latest)
    runs-on: ubuntu-latest
    timeout-minutes: 30
    env:
      GOMAXPROCS: "4"
      GOPLS_VERSION: v0.17.1
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: '1.25.x'
          cache: true
      - name: Install pinned gopls
        run: go install golang.org/x/tools/gopls@${GOPLS_VERSION}
      - name: Install benchstat
        run: go install golang.org/x/perf/cmd/benchstat@latest
      - name: Run benchmark suite
        run: |
          set -euo pipefail
          go test -short -bench=. -benchmem -count=10 -run=^$ \
            ./test/bench/... | tee test/bench/baselines/${{ inputs.milestone }}-github-hosted.txt
      - name: Verify capture
        run: |
          # Ensure we got actual benchmark lines, not just headers
          if ! grep -q "^Benchmark" test/bench/baselines/${{ inputs.milestone }}-github-hosted.txt; then
            echo "ERROR: No benchmark lines captured"
            exit 1
          fi
      - name: Commit baseline
        uses: stefanzweifel/git-auto-commit-action@v5
        with:
          commit_message: "bench: capture ${{ inputs.milestone }} baseline from ubuntu-latest CI"
          file_pattern: 'test/bench/baselines/*.txt'
```

### benchgate Invocation Without --warn-only

```yaml
# Source: bench.yml line 90-95, modified per D-03
      - name: Run benchgate
        run: |
          go run ./test/bench/cmd/benchgate \
            --baseline test/bench/baselines/v1.1-github-hosted.txt \
            --new /tmp/bench-new.txt
```

## State of the Art

| Old Approach | Current Approach | When Changed | Impact |
|--------------|------------------|--------------|--------|
| PLACEHOLDER baseline | Real CI-captured numbers | Phase 15 | Gate has meaningful comparison target |
| --warn-only (always exit 0) | Blocking mode (exit 1 on regression) | Phase 15 | PRs with regressions are actually blocked |
| Manual baseline capture | Dedicated workflow_dispatch workflow | Phase 15 | Reusable re-baselining for future milestones |

## Assumptions Log

| # | Claim | Section | Risk if Wrong |
|---|-------|---------|---------------|
| A1 | `stefanzweifel/git-auto-commit-action@v5` is the appropriate action for auto-committing in CI | Code Examples | LOW -- could use raw git commands instead; action is well-known but version not verified against registry |
| A2 | Branch protection on main allows workflow-initiated commits | Pitfalls | MEDIUM -- if branch protection requires PR reviews, auto-commit to main will fail; may need to target a non-protected branch |
| A3 | All 11 non-skipped benchmarks will complete successfully on ubuntu-latest within 30 minutes | Architecture | LOW -- bench.yml already has 30min timeout and runs on ubuntu-latest |

## Open Questions

1. **Branch protection vs auto-commit**
   - What we know: D-02 says auto-commit to triggering branch, no manual review
   - What's unclear: Whether main has branch protection that would block direct pushes from workflows
   - Recommendation: The capture workflow is `workflow_dispatch` -- the user triggers it on a specific branch. If main is protected, trigger on a feature branch and merge via PR. The plan should document this.

2. **Baseline file ordering in single plan**
   - What we know: Success criteria require both real baseline AND --warn-only removal
   - What's unclear: Whether both changes can land in a single commit or need sequencing
   - Recommendation: Per Pitfall 4, baseline numbers must exist before --warn-only is removed. Since the capture workflow runs in CI, the plan must either: (a) include placeholder real numbers captured from a prior CI run artifact, or (b) sequence as two commits -- first commit real numbers, then remove --warn-only. Given this is a single plan (15-01), the simplest approach is to capture numbers from an existing CI run artifact and commit them alongside the --warn-only removal.

3. **Source of real baseline numbers**
   - What we know: bench.yml uploads artifacts including `/tmp/bench-new.txt` on every PR run
   - What's unclear: Whether a previous CI run's artifact is available to download for seeding the baseline
   - Recommendation: The plan should either (a) use the capture-baseline workflow to generate numbers first, or (b) check GitHub Actions artifacts for a recent bench run. The capture workflow is the cleaner path per D-01.

## Validation Architecture

### Test Framework
| Property | Value |
|----------|-------|
| Framework | Go testing + benchgate unit tests |
| Config file | None (standard `go test`) |
| Quick run command | `go test ./test/bench/cmd/benchgate/ -v` |
| Full suite command | `go test ./test/bench/cmd/benchgate/ -v && go vet ./...` |

### Phase Requirements -> Test Map
| Req ID | Behavior | Test Type | Automated Command | File Exists? |
|--------|----------|-----------|-------------------|-------------|
| BENCH-05 | benchgate blocks on regression (no --warn-only) | unit | `go test ./test/bench/cmd/benchgate/ -run TestBlockingMode -v` | Existing tests cover blocking behavior |
| BENCH-05 | bench.yml invokes benchgate without --warn-only | manual-only | Verify by reading bench.yml | N/A -- workflow file inspection |
| BENCH-06 | v1.1 baseline contains real benchmark numbers | smoke | `grep -c "^Benchmark" test/bench/baselines/v1.1-github-hosted.txt` | N/A -- file content check |

### Sampling Rate
- **Per task commit:** `go test ./test/bench/cmd/benchgate/ -v && go vet ./...`
- **Per wave merge:** Same (single plan phase)
- **Phase gate:** benchgate unit tests green + v1.1 baseline file has Benchmark lines + bench.yml has no --warn-only

### Wave 0 Gaps
None -- existing test infrastructure covers all phase requirements. benchgate already has comprehensive unit tests for both blocking and warn-only modes.

## Security Domain

Not applicable -- this phase modifies CI workflow configuration and benchmark data files. No authentication, access control, input validation, or cryptography changes. The only permission change is `contents: write` on the new capture-baseline workflow, which is standard for workflows that commit results.

## Sources

### Primary (HIGH confidence)
- `.github/workflows/bench.yml` -- current gate workflow, verified line-by-line
- `test/bench/cmd/benchgate/main.go` -- gate CLI implementation, verified
- `test/bench/baselines/v1.1-github-hosted.txt` -- PLACEHOLDER content verified
- `test/bench/baselines/README.md` -- three-step rollout documentation verified
- `.planning/phases/15-benchmark-gate-hardening/15-CONTEXT.md` -- locked decisions

### Secondary (MEDIUM confidence)
- `test/bench/baselines/v1.2-phase10-github-hosted.txt` -- reference baseline format (darwin/arm64, not ubuntu-latest)

### Tertiary (LOW confidence)
- `stefanzweifel/git-auto-commit-action@v5` -- [ASSUMED] well-known action for auto-commits, version not verified

## Metadata

**Confidence breakdown:**
- Standard stack: HIGH -- all tooling already built and tested in Phase 9
- Architecture: HIGH -- changes are surgical (remove flag, replace file, add workflow)
- Pitfalls: HIGH -- well-documented in Phase 9 research and baselines/README.md

**Research date:** 2026-04-10
**Valid until:** 2026-05-10 (stable -- no fast-moving dependencies)
