# Phase 80: Five-of-Six Ablation Runners + Fairness Enforcement - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-18
**Phase:** 80-five-of-six-ablation-runners-fairness-enforcement
**Areas discussed:** baseline_plain architecture, scaffold-mode boundary (no_semantic / baseline_rag), fairness enforcement, ablation delta computation

---

## Pre-discussion: roadmap promotion

Phase 80 existed in `.planning/milestones/v1.12-ROADMAP.md` but not the working
`.planning/ROADMAP.md` (which surfaced only the 75–79 active window), so
`init.phase-op` reported `phase_found: false`. User chose **"Promote it, then
discuss"** — the Phase 80 detail block (Goal, Depends-on, Requirements, 4
Success Criteria) was mirrored into the working ROADMAP.md to unblock the
discuss/plan tooling, then discussion proceeded.

---

## baseline_plain runtime architecture

| Option | Description | Selected |
|--------|-------------|----------|
| Spawn daemon w/ baseline profile | Keep cell path uniform: spawn daemon with baseline.yaml (zero tools via ProfileFilterMiddleware); agent gets only native shell/grep/read/edit. Daemon-tap leg present for trace symmetry. | ✓ |
| Skip daemon, bare agent | No Helix MCP server at all — cleanest control semantics but forks the cell pipeline; no daemon-tap leg; trace shape differs. | |
| Discuss the trade-off | Walk through trace-symmetry vs control-purity first. | |

**User's choice:** Spawn daemon with baseline profile (uniform cell path).
**Notes:** `baseline.yaml` already strips all Helix tools via empty lists
(verified by `TestBaselineExposesZeroHelixTools`, P67 D-02), so "no Helix tools"
is enforced by the profile filter rather than by omitting the daemon. Trace
symmetry across all 6 modes was the deciding factor.

---

## Scaffold-mode boundary (no_semantic / baseline_rag)

User initially selected **"Discuss the boundary"** rather than a fixed contract.
Deep-dive established the two modes are asymmetric, then a follow-up question
resolved the concrete contract.

| Option | Description | Selected |
|--------|-------------|----------|
| Asymmetric (recommended) | no_semantic RUNS via bench-no-semantic.yaml → real row marked `guarantee_pending_phase_81` (zero-DuckDB NOT yet asserted); baseline_rag = registered fail-closed stub, no row. | ✓ |
| Both run via profile | Force symmetry — both produce rows now. Rejected: baseline_rag has no real RAG tools, row would be meaningless; contradicts "stub only" Goal text. | |
| Both registered-only | Force symmetry — neither produces a row. Rejected: throws away the no_semantic row we CAN produce (profile exists). | |

**User's choice:** Asymmetric.
**Notes:** Matches the phase-title parenthetical (`… + scaffolding for
no_semantic`) and the Goal text (`baseline_rag runner stub`). Five-of-six = five
modes have a runnable runner this phase.

### Follow-up: deferred-guarantee marker location

| Option | Description | Selected |
|--------|-------------|----------|
| result row field + MODE.md | Machine-checkable `ablation_status` field in the no_semantic result.v2.json row AND prose in MODE.md. Phase 81 flips it to `enforced`. | ✓ |
| MODE.md prose only | Document deferral only in MODE.md; no schema field. Rejected: row looks identical to a clean one when consumed by the Phase 82 aggregator. | |

**User's choice:** result row field + MODE.md (belt-and-suspenders).
**Notes:** result.v2 schema is additive-open, so `ablation_status` is a new
OPTIONAL field with no major bump.

---

## Fairness enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Startup gate + CI contract test | Runner calls `DefaultContract.Validate()` at startup (fatal) AND a CI contract test asserts effective `(model_id, temperature, max_tokens, system_prompt_hash, retry_policy, cache_policy)` == DefaultContract. | ✓ |
| CI contract test only | Single CI test; skip per-startup Validate() since scripted smoke has no real model. Rejected: no runtime guard on real claude runs. | |
| Discuss scripted vs real | Talk through whether the gate applies to the hermetic smoke. | |

**User's choice:** Startup gate + CI contract test (two layers).
**Notes:** The result.v2 schema comment already names "the runtime fairness
loader (Phase 80+)" as the rejecter of empty waiver_reason — confirming the
runtime gate is the planned home. Whether `Validate()` is called unconditionally
or only on `--agent=claude` is left to Claude's discretion; the CI contract test
must be unconditional.

---

## Ablation delta computation

| Option | Description | Selected |
|--------|-------------|----------|
| Post-run pass, minimal in-phase | After all modes complete, compute the 3 deltas (full−baseline_plain, full−no_lsp, full−no_structured_edit) and surface in per-mode rows. Full aggregator stays Phase 82. | ✓ |
| Separate deltas artifact | Standalone deltas.json; result.v2.json stays immutable. Rejected: criterion #4 says "surface in the per-mode result rows". | |
| Discuss the P80/P82 boundary | Clarify how much belongs here vs Phase 82. | |

**User's choice:** Post-run pass, minimal in-phase.
**Notes:** Single-run, three-fixed-deltas only. The multi-run aggregator, BCa
bootstrap, pass@k, and variance gate stay Phase 82 — the delta helper must not
grow into that.

---

## Claude's Discretion

- Full ABLATE-01 MODE.md frontmatter schema beyond `mode` + `profile` (P77 D-05
  handed the full convention to Phase 80).
- Exact `ablation_status` field name / enum values.
- Whether the startup `Validate()` call is unconditional or gated on the real
  claude agent (CI contract test is unconditional regardless).
- Exact shape/location of the delta helper (row vs row + sibling deltas.json),
  as long as deltas surface in per-mode rows.
- no_semantic directory name — `your_agent_no_semantic/` preferred for forward
  compat with Phase 81 criterion #4.

## Deferred Ideas

- Kernel `disable_semantic_subsystem` flag / honest no_semantic ablation → Phase 81.
- Real `baseline_rag` (`cmd/helix-bench-rag` + embedding index) → Phase 83.
- Multi-run aggregator, BCa bootstrap, pass@k, variance gate, leaderboard → Phase 82.
- Container runtime / per-language runners / external benchmarks → Phases 84–88.
