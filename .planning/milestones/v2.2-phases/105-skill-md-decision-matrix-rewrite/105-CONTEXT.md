# Phase 105: SKILL.md Decision-Matrix Rewrite - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

The hand-authored `SKILL.md` decision matrix routes an agent's single intent to a single correct tool, with no QUERY/ACTION row mixing, explicit "Not this" guidance on every row, indexed-graph prerequisite notes, and capability-based grouping — consistent with the now-correct generated reference.

**Requirements:** SKILL-01, SKILL-02, SKILL-03
**Depends on:** Phase 104 (the on-demand reference must be correct first, so SKILL.md and reference.md tell one consistent story)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Binding success criteria (from ROADMAP):
1. No decision-matrix row mixes a QUERY (read-state) verb with an ACTION (mutate-state) verb — query and action verbs occupy separate rows (resolves the SKILL-ISSUE.md rows 68/70/71/72/73 grouping errors).
2. Every decision-matrix row carries explicit "Not this" guidance — no `—` placeholders remain (the concrete grep/sed/cat/find fallback each verb displaces is named, per the CLAUDE.md routing table as canonical source).
3. Every indexed-graph verb (`get-semantic-graph-status`, `explain-cluster`, `explain-symbol-deep`, `get-change-impact-graph`, `validate-graph-edge`, `find-related-symbols`, `get-semantic-context`) carries a "requires `index-semantic-graph` first" prerequisite note, and the matrix is grouped by capability.
4. The `## Decision matrix` heading (the `StripDecisionMatrix` anchor) is preserved, the rewritten `SKILL.md` stays under the SKILL-04 idle-cost (size) cap, and a SKILL.md↔`VerbToolNames()` cross-check test holds.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Key anchors: the hand-authored `SKILL.md` bundle file (under `internal/cli/skills/helix/`), the `SKILL-ISSUE.md` analysis (now moved out of the embed dir per Phase 103), the `StripDecisionMatrix` anchor/test, the SKILL-04 size cap, the CLAUDE.md "Helix CLI tool routing" table (canonical "Not this" source), and `internal/cli.VerbToolNames()` (the 50-verb catalog the cross-check test pins against). The now-correct generated `reference.md` from Phase 104 is the consistency baseline.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
