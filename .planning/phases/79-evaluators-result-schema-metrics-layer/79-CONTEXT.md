# Phase 79: Evaluators & Result-Schema Metrics Layer - Context

**Gathered:** 2026-06-18
**Status:** Ready for planning

<domain>
## Phase Boundary

Build the evaluator packages under `bench/evaluators/{test_runner,patch_validator,token_meter,tool_trace_analyzer,regression_checker}/` that populate every per-task `result.v2.json` with all **17 normalized metrics** from real graders, plus a single merged OTel trace per `(task, mode, run_index)`.

The 17 metrics, their formulas, the provider-`usage` token-source rule, and the trace-merge requirement are **already locked by the ROADMAP success criteria** — this phase implements the graders, not the metric definitions. Scope is the grading/metrics layer only: NOT the multi-run aggregator/BCa/pass@k (Phase 82), NOT the ablation runners (Phase 80), NOT external benchmark adapters.

</domain>

<decisions>
## Implementation Decisions

### Token Metering (METRIC-03)
- **D-01:** For no-LLM / no-`usage` runs (the Phase 78 `scripted` agent replays MCP calls with no provider response), `tokens_input`/`tokens_output` (and the cached-token columns) are recorded as **explicit `null`** — never 0, never synthetically estimated. Rationale: keeps the Go ToolBench corpus honest (no fabricated token numbers) and matches the schema rule "missing metrics are explicit nulls, not omissions."
- **D-02:** The METRIC-03 source-of-truth rule (tokens come from the provider response's `usage` block, NOT Helix's MCP-side counter) and its **regression test target a real-LLM agent path** (`your_agent_full`), since the scripted agent has no `usage` block to assert against. The token_meter reads `usage` when present and emits null when absent.
- **D-03:** Cached-input tokens (`tokens_input_cached_read`, `tokens_input_cache_write`) remain separate columns (already in the schema) and follow the same present-or-null rule.

### edit_locality (METRIC-04)
- **D-04:** The denominator `total_files_in_repo_subtree` is the set of **git-tracked files under the task repo subtree** (`git ls-files`), excluding generated/vendored/untracked files. Deterministic and reproducible across machines; `edit_locality` reflects the real source surface. `modified_files` is the count of those tracked files the agent changed. Edge cases to unit-test per METRIC-04: root-only edit = 1.0, all-files edit ≈ 0.0.

### regression_rate (METRIC-05)
- **D-05:** The `regression_checker` runs the fixture's **full pre-existing test set twice**: once pre-patch (cache the passing set as the denominator `passing_pre-existing_tests_pre_patch`) and once post-patch (numerator = `failing_pre-existing_tests_post_patch` among that cached set). Accurate per the METRIC-05 formula; the Go ToolBench fixtures are tiny single-module repos so the double-run cost is negligible. **Cost tradeoff to revisit** when external SWE-bench-scale repos arrive (Phase 87) — a targeted/cached strategy may be needed then, but not now.

### Result Schema & Failure Semantics (METRIC-01)
- **D-06:** Formalize **all 17 metrics as explicitly-typed, nullable fields** under a `metrics` object in `bench/schema/result.v2.schema.json` (the 13 currently-untyped metrics get added alongside the existing 4 token fields). Makes METRIC-01's acceptance ("schema validates; missing metrics are explicit nulls, not omissions") machine-enforceable. Per Phase 75's additive-only policy, this is a **minor** schema bump (not v3).
- **D-07:** **Per-metric failure isolation**: if one evaluator fails (test_runner timeout, trace-tap finds no spans, token_meter gets no usage), that grader's metric(s) are set to `null` with a recorded **error annotation**, and all other metrics still populate — the `(task, mode, run_index)` row is still emitted and schema-valid. A failed grader never silently drops the whole row. (Chosen over whole-row-fail and over a core/auxiliary hybrid, for maximum data yield and aggregation robustness in Phase 82.)

### Claude's Discretion (left to research/planner)
- Trace-merge **reuse vs rebuild**: strong default is to **reuse `internal/eval/trace.Merge` + the daemon/CC taps** (the Phase 67/77 trace-tap METRIC-06 explicitly says to reuse) rather than build a fresh merger; planner/researcher confirm the exact seam for the `tool_trace_analyzer`.
- Where each evaluator writes (directly into `result.v2.json` vs a per-grader fragment merged by a coordinator), the package boundaries among the 5 evaluators, and the error-annotation shape on null metrics.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Phase scope & requirements
- `.planning/ROADMAP.md` §"Phase 79" — goal + 4 success criteria (the locked metric list, formulas, token-source rule, trace-merge).
- `.planning/milestones/v1.12-ROADMAP.md` §"Phase Details > Phase 79" — full detail.
- `.planning/REQUIREMENTS.md` — METRIC-01 (12 base metrics), METRIC-02 (5 extended), METRIC-03 (provider-usage token source), METRIC-04 (edit_locality formula), METRIC-05 (regression_rate formula), METRIC-06 (OTel trace merge, reuses Phase 67 trace-tap).

### Schema & upstream substrate
- `bench/schema/result.v2.schema.json` — the result schema to extend (currently types only the 4 token fields; Phase 75 left it open/additive, additive-only = minor bump per its `$comment`).
- `.planning/phases/75-schema-fairness-contract-tree-skeleton/` — schema versioning policy + fairness contract (D-03/D-04 of Phase 75).

### Trace-tap to reuse (METRIC-06)
- `internal/eval/trace/merge.go` — `Merge(MergeInput) (MergedTrace, error)`; the existing trace merger.
- `internal/eval/trace/tap.go` — `TapDaemonLog(path, expectedPid)` + `TapCCStream(path)`; daemon-OTel and agent-CLI taps with PID gating (no cross-cell leakage).

### Output artifact this phase must write
- `bench/evaluators/METRICS.md` — must document the `edit_locality` and `regression_rate` definitions (per METRIC-04/05 acceptance).

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `internal/eval/trace` (`Merge`, `TapDaemonLog`, `TapCCStream`): the Phase 67/77 trace-tap infrastructure — the trace-merge half of METRIC-06 should reuse this, not rebuild. PID-gated taps already prevent cross-cell span leakage (proven green in Phase 78's `store_isolation_test`/`cross_cell_test`).
- `internal/eval/{score,judge,report}`: existing grading/scoring/report packages — candidate building blocks for the evaluators; researcher to assess fit vs net-new `bench/evaluators/*`.
- `bench/languages/go.GoRunner.RunTests` (`go test ./... -json`, structured `TestOutcome`): Phase 78's structured test output is what `test_runner`/`regression_checker` grade — `regression_checker` can re-invoke it pre/post patch.
- `bench/runtime/cell.go` (`RunCell`): the per-cell run path that produces the daemon log + agent stream the evaluators consume; the store-ON `WithWorkingDir` isolation (Phase 78 D-03) bounds where graders read.

### Established Patterns
- Phase 75 schema policy: additive fields = minor bump, `additionalProperties` open at top level — so adding the 13 typed metric fields is a minor change (D-06).
- `result.v2.json` is the single per-task artifact; evaluators populate it (the bench already writes it per cell).

### Integration Points
- Evaluators read: the per-cell daemon OTel log + agent CLI subprocess stream + bench harness span (for trace merge), the fixture's `go test -json` outcome (test_runner/regression_checker), the provider response `usage` (token_meter), and the git working tree diff (patch_validator/edit_locality).
- Evaluators write: the `metrics` object + merged-trace reference into `result.v2.json` per `(task, mode, run_index)`.

</code_context>

<specifics>
## Specific Ideas

- The scripted-agent/no-`usage` tension is the load-bearing nuance of this phase: the Go ToolBench (the only corpus that exists today) runs the no-LLM scripted agent, so token metrics are null there by design (D-01/D-02). The provider-usage source-of-truth assertion (METRIC-03) is exercised on a real-LLM (`your_agent_full`) path.

</specifics>

<deferred>
## Deferred Ideas

- **Targeted/cached regression-test strategy** for large external repos — deferred to Phase 87 (SWE-bench Verified), where full-suite double-runs become expensive. Recorded here so the cost tradeoff (D-05) is revisited then, not silently inherited.
- Multi-run aggregation, BCa bootstrap, pass@k, cost rollup — Phase 82 (this phase produces the per-run metrics they consume).

None of the above are in Phase 79 scope.

</deferred>

---

*Phase: 79-evaluators-result-schema-metrics-layer*
*Context gathered: 2026-06-18*
