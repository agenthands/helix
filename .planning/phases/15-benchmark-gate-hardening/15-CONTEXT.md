# Phase 15: Benchmark Gate Hardening - Context

**Gathered:** 2026-04-10
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the partial BENCH-05/BENCH-06 gaps from v1.2 milestone audit. Make the CI benchstat gate enforce real thresholds against real baseline numbers. This is the final step of the Phase 9 three-step rollout (placeholder → warn-only → blocking).

</domain>

<decisions>
## Implementation Decisions

### Baseline Capture Strategy
- **D-01:** Create a dedicated baseline capture workflow (e.g., `.github/workflows/capture-baseline.yml`) separate from the gate workflow (`bench.yml`). This keeps the gate logic clean and provides a reusable tool for re-baselining.
- **D-02:** The baseline workflow auto-commits results directly to the triggering branch. Requires `contents: write` permission. No manual review step — fast and friction-free.

### Gate Rollout
- **D-03:** Remove `--warn-only` from `bench.yml` immediately. No escape hatch, no conditional logic. The gate starts blocking PRs as soon as real baselines are committed. This completes Phase 9's three-step rollout.

### Baseline Freshness Policy
- **D-04:** Re-baseline per milestone. At the start of each new milestone, trigger the baseline capture workflow to refresh numbers. Document this policy in `test/bench/baselines/README.md`.

### Carried Forward from Phase 9
- **D-01 (Phase 9):** Tiered thresholds — PR tier (GitHub-hosted, relaxed): >15% time / >25% allocs at p<0.05; release tier (self-hosted, tight): >10% time / >20% allocs at p<0.05
- **D-02 (Phase 9):** Self-hosted runner setup documented but not blocking for v1.2
- **D-03 (Phase 9):** PR runs use `-count=10`, release runs `-count=20`

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Benchmark Infrastructure
- `.github/workflows/bench.yml` — Current CI gate workflow with `--warn-only` flag (line 93) to be removed
- `test/bench/cmd/benchgate/main.go` — Benchgate CLI tool, fully functional, defines tiered thresholds
- `test/bench/baselines/README.md` — Three-step rollout documentation and refresh policy
- `test/bench/baselines/v1.1-github-hosted.txt` — PLACEHOLDER file to be replaced with real numbers

### Phase 9 Context
- `.planning/phases/09-benchmark-harness-v1-1-baseline/09-CONTEXT.md` — Original benchmark decisions (D-01 through D-03 for tiered thresholds, runner strategy, count values)

### Milestone Audit
- `.planning/v1.2-MILESTONE-AUDIT.md` — Documents BENCH-05/BENCH-06 as partial gaps with evidence

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `test/bench/cmd/benchgate/` — Fully functional benchmark regression gate CLI. Parses benchfmt, applies Welch's t-test, supports `--warn-only` and `--release-tier` flags.
- `.github/workflows/bench.yml` — Complete CI workflow with gopls pinning, GOMAXPROCS lock, artifact upload. Only needs `--warn-only` removal and baseline file update.
- `test/bench/baselines/` — Directory structure and README already in place. v1.2-phase{10,11,12} baselines exist as reference for format.

### Established Patterns
- Benchgate uses `golang.org/x/perf/benchfmt` for parsing (Pitfall 9 mitigation)
- Baseline files follow `v{milestone}-{runner}.txt` naming convention
- GOMAXPROCS=4 pinned for noise reduction on GitHub-hosted runners

### Integration Points
- `bench.yml` line 93: `--warn-only` flag to remove
- `test/bench/baselines/v1.1-github-hosted.txt`: PLACEHOLDER content to replace
- `test/bench/baselines/README.md`: Refresh policy documentation to update

</code_context>

<specifics>
## Specific Ideas

No specific requirements — the success criteria are fully defined by the ROADMAP.md and milestone audit gap descriptions.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope.

</deferred>

---

*Phase: 15-benchmark-gate-hardening*
*Context gathered: 2026-04-10*
