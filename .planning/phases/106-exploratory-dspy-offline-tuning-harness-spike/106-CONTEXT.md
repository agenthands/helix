# Phase 106: Exploratory DSPy Offline Tuning Harness (spike) - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

An opt-in, dev-time-only DSPy harness can optimize the agent-facing skill/steering text against the Phase 101 adoption scorecard metric and report whether tuning beats the deterministic baseline — with a possible no-ship outcome — while the shipped `helix` binary and `go test ./...` remain 100% Python-free.

**Requirements:** TUNE-01
**Depends on:** Phase 105 (tunes against the stabilized 104/105 surface and a frozen metric; tuning a moving target wastes optimizer budget)
**Spike discipline:** exploratory, possible no-ship; held-out TEST split is the FIRST harness task; pair `choice_rate` with a correctness/quality oracle to defend against metric-gaming; clean fallback is a hand-rolled Go candidate-search loop keeping the milestone 100% Go.

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions. This is a SPIKE — exploratory; a no-ship conclusion is a legitimate, success-meeting outcome (it is NOT a hard adoption-delta gate).

Binding success criteria (from ROADMAP):
1. The DSPy harness lives under `tools/` as an opt-in dev-time tree (its own pinned `requirements.txt` + git-ignored venv/output), is excluded from `go test ./...`, and adds NO runtime Python dependency to the `helix` binary or `helix setup`.
2. The harness optimizes the agent-facing skill/steering text against the Phase 101 adoption metric, re-implemented in Python with a golden parity cross-check against the Go `test/oracle/adopt` classifier (the same `choice_rate`/`fallback_rate` scorer drives both the Go gate and the Python optimizer).
3. Overfit and metric-gaming guards are in place — a held-out TEST split the optimizer never sees, and a degenerate-steering inspection — and the harness may legitimately conclude no-ship (the clean fallback being a hand-rolled Go candidate-search loop keeping the milestone 100% Go).
4. Any adopted output re-enters only as a human-reviewed commit through SKILL.md/refgen and passes `helix-refgen --check`; a `make vet`-style analyzer asserts no Python/optimizer coupling leaks into the runtime/merge path (the artifact is gated, never the optimizer process).

### Milestone-wide invariant (enforced here)
No runtime Python / single-binary preserved: DSPy is dev-time/offline only — no `helix` subcommand shells to Python, no `go.mod`/`helix setup` edge, off the default `go test ./...` / merge path. Gate the committed ARTIFACT, never the optimizer PROCESS (LLM optimization isn't bit-reproducible).

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors: the Phase 101 adoption scorecard metric and the Go `test/oracle/adopt` classifier (the `choice_rate`/`fallback_rate` scorer — `StripDecisionMatrix` lives here too); the `tools/` tree convention; existing `make vet`-style standalone analyzers (e.g. `cmd/vet-noduckdb`, `internal/lint/*`) as the template for the leakage analyzer; the `.gitignore`; the corpus (`MinTasks=5` — small, so a held-out TEST split is the first task); `cmd/helix-refgen --check` as the artifact re-entry gate.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria. This phase is explicitly flagged for phase-level research (corpus-split design, Python↔Go classifier parity contract, metric-AND-quality-oracle composition, leakage-analyzer design).

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
