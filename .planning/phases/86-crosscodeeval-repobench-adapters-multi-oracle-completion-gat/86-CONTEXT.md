# Phase 86: CrossCodeEval + RepoBench Adapters + Multi-Oracle Completion Gate - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Mid-size completion-only public benchmarks via pure `dataset-loader-only` adapters — CrossCodeEval covers Python/Java/TS/C# (the only public coverage for C# we ship), RepoBench-R/-C/-P covers Python and Java. Multi-oracle gate (EM + edit-similarity + identifier match all required to pass, with abstain mode for low-confidence completions) is the v1.12-specific contribution.

**Requirements:** ADAPTER-CCE-01, ADAPTER-REPO-01, VERIFIED-03

**Success Criteria (what must be TRUE):**

1. CrossCodeEval smoke run scores at least one task per language (Python, Java, TS, C#); EM + edit-similarity + identifier-match scorers are unit-tested against CCE paper examples; HF dataset fetched via the cached parquet pipeline.
2. RepoBench smoke run for each sub-task (RepoBench-R retrieval `acc@k`, RepoBench-C completion `EM`/`ES`, RepoBench-P pipeline) covers Python and Java; metrics match published reference values on a sampled subset.
3. Multi-oracle gate documented in `bench/evaluators/VERIFIED.md`: EM + edit-similarity + identifier match all required to pass; per-oracle threshold is configurable; abstain mode emits a `verified_correctness = false` row instead of a false-positive `true`.
4. Both adapters use the canary-emission probe (per Phase 75 INFRA pattern) and flag potentially-contaminated tasks; canary pass-rate column populated in their leaderboard rows.

**Depends on:** Phase 85 (Python + Java + TS + C# LanguageRunners), Phase 82 (aggregator), Phase 75 (HF dataset fetcher infra via `gomlx/go-huggingface` + arrow-go fallback)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

- Both adapters are `dataset-loader-only` (no Docker, no upstream harness) — load HF datasets via the Phase 75 fetcher infra (`gomlx/go-huggingface` + arrow-go fallback) into the cached parquet pipeline; research must locate that infra.
- The scorers (Exact-Match, edit-similarity/Levenshtein, identifier-match) and the multi-oracle gate (all-three-required + per-oracle configurable threshold + abstain → `verified_correctness=false`) are PURE logic — unit-test hermetically against CCE/RepoBench paper examples + committed reference values. These are the load-bearing, fully-testable core.
- The canary-emission probe reuses the Phase 75 INFRA contamination-canary pattern — research must locate it.
- **Environment reality:** HF dataset fetching needs network + parquet; gate the live fetch/smoke-run tests on network availability (SKIP cleanly when absent), with committed fixture rows (small CCE/RepoBench-shaped samples) so the loader + scorers + gate are hermetically tested without network. Honestly record the live "metrics match published reference values" confirmation as network-gated. No live test may be the sole proof.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- Phase 75 HF dataset fetcher infra (`gomlx/go-huggingface` + arrow-go fallback) + the cached parquet pipeline + `$HELIX_CACHE_DIR` convention.
- Phase 75 INFRA contamination-canary pattern (the canary-emission probe).
- Phase 85 Python/Java/TS/C# LanguageRunners + the `language` result field + aggregator per-language slice.
- Phase 82 aggregator (leaderboard rows; add a canary-pass-rate column).
- Phase 79 evaluators / scoring DSL (where EM/ES/identifier scorers live or hook in).
- The `make verify-*` hard-gate pattern + bench/evaluators/ conventions.

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond ADAPTER-CCE-01, ADAPTER-REPO-01, VERIFIED-03 and the four success criteria above — discuss phase skipped.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
