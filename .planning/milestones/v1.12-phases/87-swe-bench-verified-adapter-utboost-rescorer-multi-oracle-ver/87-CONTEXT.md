# Phase 87: SWE-bench Verified Adapter + UTBoost Rescorer + Multi-Oracle `verified_correctness` - Context

**Gathered:** 2026-06-21
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The headline external benchmark — SWE-bench Verified 500-task adapter via `subprocess-shellout` to upstream `python -m swebench.harness.run_evaluation`, with raw upstream score and UTBoost-augmented rescored score reported side-by-side. Multi-oracle `verified_correctness` (canonical tests pass AND augmented tests pass AND no pre-existing tests regress) ships in the same phase because they are intrinsically coupled — shipping the adapter without UTBoost regresses the milestone's `verified_correctness` claim.

**Requirements:** ADAPTER-SWE-01, VERIFIED-01, VERIFIED-02

**Success Criteria (what must be TRUE):**

1. SWE-bench Verified smoke run of 5 tasks completes via `subprocess-shellout` to upstream harness; `predictions.jsonl` produced by the agent; result JSON ingested into `result.v2.json` schema with container ID + exit code preserved.
2. `verified_correctness` is computed independently of `task_success` — a known-buggy patch that passes only canonical tests gets `task_success=true` AND `verified_correctness=false`; the metric requires (a) canonical tests pass AND (b) UTBoost-augmented tests pass AND (c) no pre-existing tests regress.
3. SWE-bench Verified report shows both raw upstream score and UTBoost-augmented rescored score side-by-side; UTBoost augmented suite is ingested from the published source and reproducible from a `--run-id`.
4. Run-all-tests override is wired (not just PR-modified tests as upstream's default); `bench/evaluators/swebench/differential.go` consumes the gold patch alongside the agent patch and emits diff-overlap signal.

**Depends on:** Phase 84 (container runtime + GHCR mirror), Phase 85 (Python LanguageRunner + pre-baked Python toolchain image), Phase 86 (multi-oracle pattern proven on completion benchmarks)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use the ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

- The adapter is `subprocess-shellout` to upstream `python -m swebench.harness.run_evaluation` — fixed argv + strict env allowlist (reuse the Phase 84 `bench/container/engine.go` discipline / the aider/CCE clone discipline); ingest the harness result JSON → `result.v2.json` preserving container ID + exit code.
- The multi-oracle `verified_correctness` (VERIFIED-01/02) reuses the Phase 86 gate PATTERN but over TEST-EXECUTION oracles: (a) canonical tests pass AND (b) UTBoost-augmented tests pass AND (c) no pre-existing tests regress. It is computed INDEPENDENTLY of `task_success` (the existing `evaluators.Metrics.VerifiedCorrectness` field — additive producer, no schema bump). Fail-closed: any oracle missing/abstain → `verified_correctness=false`, never a false `true`.
- UTBoost augmented suite is INGESTED from the published source (pinned), reproducible from a `--run-id`.
- `differential.go` consumes the gold patch alongside the agent patch and emits a diff-overlap signal; run-all-tests override (not just PR-modified tests).
- **Environment reality:** SWE-bench needs Docker + the upstream `swebench` Python package + per-instance images — NOT available here. The adapter code (subprocess wiring, predictions.jsonl producer, harness-result→result.v2 ingestion, UTBoost rescorer, the 3-condition verified_correctness producer, differential.go) MUST be hermetically tested against committed fixtures (a sample SWE-bench instance, a sample harness-output JSON, a sample UTBoost augmented suite, a known-buggy-patch case for SC#2). The live 5-task smoke run + container execution are gated on Docker/swebench/network availability and SKIP cleanly. No live test may be the sole proof; the SC#2 known-buggy-patch (`task_success=true` AND `verified_correctness=false`) MUST be a hermetic test.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors:
- Phase 84 `bench/container/` (engine, Run(--network=none), image cache) — the container substrate for the SWE-bench harness.
- Phase 85 Python LanguageRunner + the `language` result field.
- Phase 86 multi-oracle gate (`bench/evaluators/completion_gate/`) + `evaluators.Metrics.VerifiedCorrectness` + the abstain-fail-closed pattern + VERIFIED.md — the PATTERN to apply to test-execution oracles.
- The aider/CCE/RepoBench subprocess + fixed-argv + strict-env discipline; the HELIX_BENCH_NETWORK / Docker-availability gating idiom.
- `bench/runtime/result.go` result.v2 (container ID + exit code fields) + the aggregator (raw vs rescored side-by-side columns).
- bench/datasets pinned-rev fetch + `$HELIX_CACHE_DIR` cache.

</code_context>

<specifics>
## Specific Ideas

No additional requirements beyond ADAPTER-SWE-01, VERIFIED-01, VERIFIED-02 and the four success criteria above — discuss phase skipped.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
