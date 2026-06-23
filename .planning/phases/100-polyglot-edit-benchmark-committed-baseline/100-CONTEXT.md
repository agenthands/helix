# Phase 100: Polyglot Edit Benchmark + Committed Baseline - Context

**Gathered:** 2026-06-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The model's edit is routed through helix EDIT verbs against the warm daemon via the reused verb-agnostic `RunExercise` loader, surfaced as a new filesystem-table bench mode with an additive `edit_format_applied` result key, and a byte-reproducible committed polyglot-edit baseline is captured `HELIX_BIN`-gated, fail-not-skip.

**Requirements:** EDITBENCH-01, EDITBENCH-02, EDITBENCH-03, BASELINE-01

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints carried from research (SUMMARY.md / ARCHITECTURE.md / PITFALLS.md) + Phase 99 outputs:
- **Reuse `RunExercise` VERBATIM** (Phase 85, `bench/datasets/aider-polyglot/loader.go`) — the v2.1 delta is supplying an EDIT-verb `AgentFn` (in `bench/runtime`, daemon-dialing), NOT rebuilding the loader. The WR-01 anti-tamper pristine-test restore MUST stay intact.
- **EDITBENCH-01:** the `AgentFn` routes the model's edit through the helix EDIT verbs: `replace-symbol-body` / `fuzzy-edit` / `replace-in-file` / `insert-before-symbol` / `insert-after-symbol`, dialed against the warm daemon (the CLI→gRPC path; reuse the Phase 77 subprocess daemon lifecycle in `bench/runtime`).
- **EDITBENCH-02:** new `bench/runners/aider_edit/MODE.md` via the filesystem-as-table pattern (Phase 80 grew 1→6 modes with ZERO mode-resolver Go change). Use the vendored fixtures from Phase 99 (offline) — `bench/datasets/aider-polyglot/fixtures/` (py/go/rust; java reserved/future).
- **EDITBENCH-03:** additive `edit_format_applied` (`*bool`, `omitempty`) open key on `result.v2.json` — NO `schema_version` v3 bump (mirror the `swebench_raw_resolved`/`embedder_id`/`language` additive-open-key precedent).
- **BASELINE-01 — the load-bearing determinism decision:** a committed baseline must be BYTE-REPRODUCIBLE, but a real LLM is non-deterministic. So the COMMITTED baseline is driven by a DETERMINISTIC/scripted agent (apply the exercism reference `files.solution` through the helix EDIT verbs — a fixed, reproducible transformation), NOT a live model. The real-LLM (`--agent=claude`) arm is wired-but-NOT-the-baseline (mirror Phase 77's scripted `CCTapResult` synthesis + the wired-not-gating claude branch). Commit only deterministic metrics (pass/fail, edit_format_applied, file/edit counts) — EXCLUDE latency/tokens-of-a-live-model from the committed baseline (latency → local `bench-micro`).
- **HELIX_BIN fail-not-skip (cross-cutting, MANDATORY):** the bench-surface tests must FAIL (not silently SKIP) when `HELIX_BIN` is set but no `result.v2.json` / empty bucket / missing metric is produced. Ship a hermetic golden sibling (no binary, no network) as the sole authoritative proof, plus a "did it RUN" sentinel when HELIX_BIN is set. (Project memory: bench smoke tests are false-green without HELIX_BIN.)
- **Determinism guards:** route reports through the existing deterministic `renderAll` / seed any RNG / sort-before-emit; the committed baseline artifact (`bench/reports/<run>/BENCH-RESULTS.md` + `result.v2.json`) must regenerate byte-identically.
- Respect the `vet-ablation-leakage` leaf-import boundary: the daemon-dialing AgentFn lives in `bench/runtime` (NOT a stdlib leaf); stdlib evaluator leaves stay import-clean.
- Benches stay local-only (no CI benchstat gate). A built helix binary is available at `/tmp/helix-v2.1-test`; the executor may rebuild a fresh one for HELIX_BIN.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Relevant existing files: `bench/datasets/aider-polyglot/loader.go` (RunExercise, the verb-agnostic AgentFn seam, WR-01 restorePristineTests), `bench/runtime/{sandbox,subprocess,cell}.go` (Phase 77 daemon lifecycle + cell spine + CCTapResult synthesis), `bench/runtime/result.go` (result.v2 builder + additive open keys), `bench/runners/<mode>/MODE.md` + `mode_resolver.go` (filesystem-table modes), `bench/aggregator/` (deterministic renderAll / BCa / pass@k), the Phase 99 vendored `fixtures/` tree + VENDOR-MANIFEST.

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP success criteria — discuss phase skipped. Refer to ROADMAP Phase 100 description and its 4 success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
