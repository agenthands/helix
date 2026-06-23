# Phase 97: Generated Per-Verb Reference + Deterministic Adoption Contract - Context

**Gathered:** 2026-06-22
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss)

<domain>
## Phase Boundary

An agent has a complete, registry-generated per-verb reference installed alongside the terse skill, and a deterministic merge-gating contract proves both that the reference covers every frozen verb and that the nudge steers each standard-tool shape to the specific correct `helix` verb.

**Requirements:** REF-01, REF-02, REF-03, ADOPT-01

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
All implementation choices are at Claude's discretion — discuss phase was skipped per user setting. Use ROADMAP phase goal, success criteria, and codebase conventions to guide decisions.

Key constraints carried from research (SUMMARY.md / ARCHITECTURE.md / PITFALLS.md):
- The per-verb reference MUST be generated from the tool registry (reuse `cmd/docgen` walk + `internal/kernel/help`), NOT hand-written — inherit the docgen blank-import-parity-with-daemon rule and a `--check` drift gate.
- The one structural code change: `internal/cli/skill.go` `embeddedSkillMD string` → `embed.FS` so `installSkill` ships `reference.md` alongside `SKILL.md`, preserving the existing `withinSkillRoot` containment + atomic write, with the SKILL-04 idle-cost bound still asserted on `SKILL.md` only.
- The adoption-contract completeness list MUST be sourced from `internal/cli/verbs_gen.go` (`VerbToolNames()`) as the authority — NOT the generator's own output (anti-vacuity Pitfall 1).
- Anti-vacuity is mandatory: ship a deliberate break-the-invariant → assert-RED test (delete a verb from reference → completeness gate RED; break a nudge-shape mapping → contract RED). Reject empty-bucket-as-pass. Key nudge detectors on the emitted command, not substring.

</decisions>

<code_context>
## Existing Code Insights

Codebase context will be gathered during plan-phase research. Relevant existing files: `internal/cli/skill.go` (embedded SKILL.md), `internal/cli/nudge.go` (PreToolUse advisory classifier), `internal/cli/verbs_gen.go` + `VerbToolNames()`, `cmd/docgen/` (registry-walk generator + `--check`), `internal/kernel/help/help.go` (`ExtractParamDocs`/`FormatHelp`), `internal/cli/setup_clients.go` (`installSkill`, `withinSkillRoot`).

</code_context>

<specifics>
## Specific Ideas

No specific requirements beyond the ROADMAP success criteria — discuss phase skipped. Refer to ROADMAP Phase 97 description and the four success criteria.

</specifics>

<deferred>
## Deferred Ideas

None — discuss phase skipped.

</deferred>
