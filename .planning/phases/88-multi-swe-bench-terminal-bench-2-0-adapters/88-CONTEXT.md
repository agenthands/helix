# Phase 88: Multi-SWE-bench + Terminal-Bench 2.0 Adapters - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Final external coverage — Multi-SWE-bench (1,632 instances × Java/TS/JS/Go/Rust/C/C++; Mini set acceptable at ship, full set is the reach goal) and Terminal-Bench 2.0 (89 hard containerized long-horizon tasks driven through `tb run`). Both via `subprocess-shellout`. Per-language slicing is the v1.12 contribution; long-wall scheduler accommodates Terminal-Bench tasks whose wall-time exceeds a day.

**Requirements:** ADAPTER-MULTI-01, ADAPTER-TERM-01

**Success Criteria (what must be TRUE):**

1. Multi-SWE-bench Mini set runs end-to-end via `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`; per-language slicing (Java, TS, JS, Go, Rust, C, C++) is exposed in the reporter; full set documented as reach goal with the license resolution status (defer to v1.13 if unresolved per Phase 75 INFRA-02).
2. Terminal-Bench 2.0 smoke run of ≥ 5 tasks completes via `tb run` CLI; container-isolation invariant holds (per-task fresh container, no cross-task filesystem leakage); `tb` JSON output ingested into `result.v2.json`.
3. Long-wall scheduler accommodates tasks whose expected wall-time exceeds a day; per-cell checkpointing means a 24h Terminal-Bench task can resume after harness restart.
4. Both adapters carry forward Phase 87's run-all-tests override pattern where applicable; per-language ablation slicing built into the reporter (not the upstream harness).

**Depends on:** Phase 87 (subprocess-shellout pattern proven on SWE-bench), Phase 85 (7 per-language toolchain images + runners)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

- Both adapters reuse the Phase 87 `subprocess-shellout` pattern (`bench/evaluators/swebench/harness.go`: fixed-argv + strict env allowlist + controlled WorkDir + no shell) and the harness-JSON → result.v2 ingestion shape: Multi-SWE-bench shells to `python -m multi_swe_bench.harness.run_evaluation --config <config.json>`; Terminal-Bench shells to the `tb run` CLI.
- Per-language slicing (Java/TS/JS/Go/Rust/C/C++) is built into the REPORTER (the aggregator, reusing the Phase 85 `language` field + ByLanguage slice), NOT the upstream harness.
- Long-wall scheduler + per-cell checkpointing (SC#3) is pure state-machine logic: a cell's progress is checkpointed so a >24h Terminal-Bench task resumes after a harness restart. Hermetically testable (checkpoint write/read + resume-from-checkpoint).
- Multi-SWE-bench full-set license status: document the resolution status; defer the full set to v1.13 if unresolved (per Phase 75 INFRA-02). Ship the Mini set.
- Container-isolation invariant (SC#2): per-task fresh container, no cross-task filesystem leakage — assert via the adapter's container handling (reuse Phase 84 `bench/container` engine where the harness doesn't own Docker; tb/multi_swe_bench own their own Docker, so we shell to them).
- **Environment reality:** Multi-SWE-bench needs Docker + the `multi_swe_bench` Python package; Terminal-Bench needs the `tb` CLI + Docker — NOT available here. The adapter code (subprocess wiring, config.json producer, harness-JSON → result.v2 ingestion, per-language slicing, the long-wall scheduler + per-cell checkpointing, the tb JSON ingestion) MUST be hermetically tested against committed fixtures (sample multi_swe_bench config + report, sample tb JSON output, a checkpoint resume case). The live Mini-set + tb smoke runs SKIP cleanly (Docker/tb/swebench absent). No live test may be the sole proof; the checkpoint-resume + per-language-slicing logic MUST be hermetic.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- Phase 87 `bench/evaluators/swebench/` (harness.go fixed-argv subprocess wrapper, ingest.go harness-JSON→result.v2, the run-all-tests override) — the template both adapters clone.
- Phase 85 `language` result field + aggregator `ByLanguage` per-language slice (for per-language slicing).
- Phase 84 `bench/container/` (engine, container-isolation) where applicable.
- result.v2 container_id/exit_code keys (Phase 87) + the aggregator additive-column pattern.
- bench/datasets pinned-rev fetch + `$HELIX_CACHE_DIR` cache; the HELIX_BENCH_NETWORK/Docker-availability gate idiom.
- Phase 75 INFRA-02 (license resolution / TOS) for the Multi-SWE-bench full-set license status.

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond ADAPTER-MULTI-01, ADAPTER-TERM-01 and the four success criteria above — discuss phase skipped.

</specifics>

<deferred>
## Deferred Ideas

- Multi-SWE-bench FULL set (1,632 instances) — reach goal; defer to v1.13 if the license is unresolved (per Phase 75 INFRA-02). Mini set ships in this phase.

</deferred>
