# Phase 104: Reference Generator Per-Verb Correctness - Context

**Gathered:** 2026-06-24
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

Every verb's "use this, not that" and "Output" lines in the generated `reference.md` are correct for that verb's actual semantics, fixed at the generator root cause (the group collapse), with the corrected `reference.md` regenerated, committed, and reproducible.

**Requirements:** REFGEN-01
**Depends on:** Phase 103 (clean embed dir → no stray-file noise in the generated bundle or its tests)

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Binding success criteria (from ROADMAP):
1. The group-collapse root cause is fixed in the GENERATOR (`cmd/helix-refgen`/`cmd/helix-cligen`) via a per-verb override map with group-default fallback — NOT by hand-editing the generated `reference.md` — so verbs that previously inherited the wrong group's `memory`-query prose (e.g. `switch-mode`, `get-token-budget`, `onboard-project`, `get-health`, `get-tool-help`, mutating memory verbs) now read correctly.
2. The regenerated `reference.md` is committed and passes `helix-refgen --check` byte-for-byte (`git diff` is empty after a fresh regen).
3. The `reference ⊇ VerbToolNames()` contract and the Phase 97 `--check` drift gate stay green, and the blank-import parity between the generator and the daemon is re-verified when touching refgen.
4. Vacuity guards prove the override map is real: no override key is a non-verb, and an overridden Output line differs from the old group default it replaced (deliberate break-the-invariant test).

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research.

</code_context>

<specifics>
## Specific Ideas

No specific requirements — discuss phase skipped. Refer to ROADMAP phase description and success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
