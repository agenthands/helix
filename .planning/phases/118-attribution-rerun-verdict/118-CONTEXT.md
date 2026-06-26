# Phase 118: Re-run Attribution + Verdict - Context

**Gathered:** 2026-06-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Measure a meaningful delta on the fixed harness. Phases 115–117 fixed the agent so it actually uses tools. Now we run the attribution pipeline to see if the fixes produced a measurable improvement.

**In scope:**
- HARNESS-05a: Re-run v2.4 attribution pipeline on fixed harness
- HARNESS-05b: Verify agent tool-call rate ≥ 80% on held-out
- HARNESS-05c: Record ON/OFF delta with per-arm cost
- HARNESS-05d: Gate for TUNE-FUT-03 (adoption path ready if delta significant)

**Out of scope:**
- Task-solving prompt (Phase 115 — already complete)
- Feedback loop (Phase 115 — already complete)
- Verb-arg hardening (Phase 116 — already complete)
- GEPA module rebuild (Phase 117 — already complete)

</domain>

<decisions>
## Implementation Decisions

### D-01: Use existing attribution infrastructure
The `attribution.py` and `run_real.py` scripts from v2.4 are already set up for ON/OFF attribution. We reuse them with the fixed harness.

### D-02: Measure tool-call rate
Phase 115-117 fixed the agent to use tools. We must verify the tool-call rate is now ≥ 80% on held-out (it was ~0% before due to prose-only answers).

### D-03: Cost tracking
The metered LLM wrapper from v2.4 (`attribution.MeteredLLM`) already tracks token costs. We record per-arm costs in the report.

### D-04: Honest verdict
If the delta is small/noisy, we document "not yet ready for adoption" rather than overstating. The v2.4 delta (+0.0392) was noise because the agent didn't use tools.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Attribution Files
- `tools/dspy-tune/attribution.py` — ON/OFF attribution with metered LLM
- `tools/dspy-tune/run_real.py` — Real harness runner
- `tools/dspy-tune/optimize.py` — GEPA optimization (now with trace emission)
- `tools/dspy-tune/REPORT.md` — Verdict report (to be updated)

### Requirements and Roadmap
- `.planning/REQUIREMENTS.md` — HARNESS-05a/b/c/d requirements
- `.planning/ROADMAP.md` — Phase 118 success criteria and exit gate
- `.planning/.continue-here.md` — Root cause: GEPA was no-op, now fixed

### Corpus Location
- `tools/dspy-tune/corpus/` — 103-task Aider corpus (already materialized)

</canonical_refs>

<code_context>
## Existing Code Insights

### Attribution Pipeline (attribution.py)
The `run_attribution()` function:
1. Loads held-out test split from `output/heldout_test.json`
2. Runs ON steering (candidate) and OFF steering (control)
3. Records per-task results: passed, tests_run, cost_usd, agent_reason
4. Computes delta: ON - OFF

### Metered LLM (attribution.py)
`MeteredLLM` wraps any LLM and tracks:
- Token usage (prompt + completion)
- Cost in USD
- Per-task costs for attribution

### Corpus (build_corpus.py + corpus/)
The 103-task Aider corpus is already at `AIDER_TASKS_DIR`:
- 26 train, 26 val, 51 held-out (sequestered)
- `val_size = 51 > 50` passes the adoption gate

</code_context>

<specifics>
## Specific Ideas

- Run attribution on the fixed harness (Phases 115-117 fixes in place)
- Compare tool-call rate before (v2.4) vs after (v2.5)
- Record honest delta with cost breakdown
- Update REPORT.md with findings

</specifics>

<deferred>
## Deferred Ideas

None — this is the final phase of v2.5. Adoption (TUNE-FUT-03) is a separate human-gated step.

</deferred>

---

*Phase: 118-Attribution-Rerun-Verdict*
*Context gathered: 2026-06-26*