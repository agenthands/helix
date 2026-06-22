# Phase 89: Reports, CI Policy & Contamination Canary - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The publication artifact — `leaderboard.md` + `per_language.md` + `ablations.md` + `cost_quality.md` byte-reproducible from a `--run-id`, with the CI cost-policy split that protects the milestone budget (`make bench-quick` ≤ 5 min on PR, full `make bench` nightly or maintainer-gated). Contamination canary closes the loop on Pitfall 1 — emit a known-novel pattern in select tasks; if a model emits it verbatim, the task is excluded from headline numbers with a footnote.

**Requirements:** REPORT-01, REPORT-02, REPORT-03, REPORT-04, REPORT-05, INFRA-04, INFRA-05

**Success Criteria (what must be TRUE):**

1. `helix-bench report --run-id <id>` regenerates all 4 reports byte-identically (`diff` on regenerated vs original is empty); `leaderboard.md` shows `(mode × benchmark) → pass@1, verified_correctness, cost_per_solved` with BCa CIs and non-overlap markers; `per_language.md` lists languages with no benchmark coverage as `n/a`, not omitted.
2. `ablations.md` delta tables (`full vs no_lsp`, `full vs no_semantic`, `full vs no_structured_edit`, `full vs baseline_plain`, `full vs baseline_rag`) compute correctly with CI overlap analysis; `cost_quality.md` scatter (cost vs verified_correctness) renders as ASCII/svg and cites cost-table `valid_until`.
3. CI workflow file exists: `make bench-quick` runs on PR (ToolBench Go-only, no LLM cost, hard 5-min cap); full `make bench` runs nightly or on-demand, gated on a maintainer label; documented cost budget.
4. Contamination canary: a known-novel pattern emitted in select tasks; a synthetic contaminated-response test trips the flag; flagged tasks are listed in `leaderboard.md` footnote and excluded from headline numbers.

**Depends on:** Phase 82 (aggregator + first leaderboard), Phase 83 (baseline_rag rows), Phase 87 (SWE-bench Verified headline), Phase 88 (final external coverage)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

- Build on the Phase 82 aggregator, which ALREADY emits `leaderboard.md` + `cost_quality.md` (seed-deterministic, byte-identical goldens). This phase adds `per_language.md` + `ablations.md`, wires the `helix-bench report --run-id` command (replacing/extending the Phase 86 real fetch-datasets-era `report` stub), and makes ALL 4 reports byte-reproducible from a `--run-id`.
- Determinism is load-bearing (SC#1 `diff` empty): reuse the Phase 82 seeded-PCG / zero-RNG-in-render discipline so regeneration is byte-identical. Golden `.md` fixtures are the hermetic proof.
- `ablations.md` reuses the Phase 80/82 delta machinery (`bench/runtime/deltas.go` / aggregator) + the Phase 82 BCa CIs + CI-overlap gate; `cost_quality.md` already exists (Phase 82) — extend with the scatter + cost-table `valid_until` citation.
- CI policy (SC#3): a CI workflow file (`.github/workflows/bench.yml` or similar) with `make bench-quick` on PR (ToolBench-Go-only, no LLM cost, hard 5-min cap — `make bench-quick` already exists from Phase 77) and full `make bench` nightly/maintainer-label-gated; document the cost budget. Reuse the Phase 84 `bench-mirror.yml` / Phase 58 CI conventions.
- Contamination canary (SC#4): the REAL canary, extending the Phase 86 minimal `bench/canary` probe (`Sentinel`/`InjectPrompt`/`IsContaminated` + `DocKeyContaminated`) — emit a known-novel pattern in select tasks; a synthetic contaminated-response test trips the flag; flagged tasks are listed in a `leaderboard.md` footnote and EXCLUDED from headline numbers (the aggregator already has the `CanaryPassRate` column from Phase 86 — wire the exclusion + footnote).
- **Environment reality:** the report renderers + canary exclusion + CI-workflow-file are pure logic / static files — hermetically testable against committed golden reports + a multi-run fixture tree. No live benchmark run is needed for this phase (it consumes existing result.v2 rows). The CI workflow YAML is verified by inspection + a lint/parse test, not by a live CI run.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- Phase 82 `bench/aggregator/` (leaderboard.md + cost_quality.md renderers, BCa CIs, CI-overlap gate, seed-deterministic byte-identical goldens, `--runs`) + the `helix-bench aggregate` subcommand.
- Phase 86 `bench/canary/` (Sentinel/InjectPrompt/IsContaminated/DocKeyContaminated) + the aggregator `CanaryPassRate` column — extend for the real canary exclusion + footnote.
- Phase 80/82 `bench/runtime/deltas.go` delta machinery (for ablations.md).
- Phase 85 `language`/`ByLanguage` slice (for per_language.md).
- The `cmd/helix-bench` `report` subcommand stub + the Phase 75 `make bench`/`make bench-quick` targets + Phase 84 `bench-mirror.yml` / Phase 58 CI conventions.
- Phase 87 `verified_correctness` (the leaderboard column).

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond REPORT-01..05 + INFRA-04/05 and the four success criteria above — discuss phase skipped. This is the v1.12 capstone/publication phase.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
