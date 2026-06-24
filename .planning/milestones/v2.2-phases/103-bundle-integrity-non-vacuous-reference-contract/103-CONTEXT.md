# Phase 103: Bundle Integrity & Non-Vacuous Reference Contract - Context

**Gathered:** 2026-06-23
**Status:** Ready for planning
**Mode:** Auto-generated (discuss skipped via workflow.skip_discuss) — enriched with v2.2 milestone research.

<domain>
## Phase Boundary

`helix setup` installs exactly the two skill-bundle files (`SKILL.md` + `reference.md`) and nothing else; the embedded bundle can no longer leak stray files into the binary or onto users' disks; and the reference-completeness gate is hardened to be discriminating (exact count, known-absent-verb) BEFORE any reference/skill text churns in Phases 104/105.

**Requirements:** BUNDLE-01, BUNDLE-02

</domain>

<decisions>
## Implementation Decisions

### Claude's Discretion
Discuss was skipped per `workflow.skip_discuss`. Use the ROADMAP Phase 103 goal + 4 success criteria, the v2.2 research, and codebase conventions. The constraints below are LOCKED by the milestone research/requirements — plan around them, do not re-litigate.

Key constraints (from `.planning/research/SUMMARY.md` + `internal/cli/skills/helix/SKILL-ISSUE.md`, verified against the tree):
- **Embed leak is LIVE:** `internal/cli/skill.go` uses `//go:embed skills/helix/*` and `installSkill` loops over EVERY `ReadDir("skills/helix")` entry, so `SKILL-ISSUE.md` (18 KB) is currently embedded into the binary AND written by `helix setup`. Fix: a closed-set allowlist `bundleFiles = {SKILL.md, reference.md}` applied ONCE up front to BOTH the install and uninstall loops, placed BEFORE the 2-pass stage→rename, leaving atomicity and the `withinSkillRoot` containment guarantee UNCHANGED. Move `SKILL-ISSUE.md` OUT of `internal/cli/skills/helix/` (e.g. to `.planning/` or a non-embedded `docs/`), updating the ROADMAP Backlog reference path.
- **Bundle test must be a CLOSED-SET assertion:** the existing `TestInstallSkillWritesBundle` (or equivalent) is positive-only (asserts the two files are present) — it must assert the installed set equals EXACTLY `{SKILL.md, reference.md}` and go RED on any stray file (anti-vacuity: a deliberate stray-file test runs RED before the fix).
- **Harden the reference-completeness contract (BUNDLE-02):** `internal/cli/reference_contract_test.go` (the `reference ⊇ VerbToolNames()` check) must assert EXACT count `== len(VerbToolNames())` (currently 50) AND discriminate a known-absent verb (RED when a frozen verb is missing) — a deliberate break-the-invariant assertion that runs RED before the fix. This lands in Phase 103 so the gate cannot pass vacuously (`∅ ⊇ ∅`) through the 104/105 format churn.
- **TDD:** `workflow.tdd_mode` is on — RED→GREEN→REFACTOR for the testable gates (closed-set bundle test, hardened contract test).
- **Preserve invariants:** the `## Decision matrix` StripDecisionMatrix anchor in SKILL.md, the `helix-refgen --check` byte-reproducibility, and `reference ⊇ VerbToolNames()` all stay green. ZERO new Go deps (edits inside already-vendored packages).

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Milestone research (v2.2)
- `.planning/research/SUMMARY.md` — synthesized research + build order.
- `.planning/research/ARCHITECTURE.md` — installSkill allowlist design + the generator root cause (for context).
- `.planning/research/PITFALLS.md` — the live embed leak + positive-only-test pitfall + anti-vacuity discipline.

### Source analysis
- `internal/cli/skills/helix/SKILL-ISSUE.md` — maintainer analysis (this file itself is the one currently leaking; move it out).

### Code under change
- `internal/cli/skill.go` — `//go:embed skills/helix/*`, `installSkill`/`uninstallSkill` (the ReadDir loop to allowlist), `withinSkillRoot`.
- `internal/cli/skill_test.go` (or wherever `TestInstallSkillWritesBundle` lives) — make the bundle assertion closed-set.
- `internal/cli/reference_contract_test.go` — harden to exact-count + known-absent-verb discriminator.
- `cmd/helix-refgen/` — the `--check` drift gate + `VerbToolNames()` authority (preserve).

</canonical_refs>

<specifics>
## Specific Ideas

Land Phase 103 FIRST and smallest-blast-radius: it stops the live leak and makes the gate non-vacuous before Phases 104 (generator fixes) and 105 (SKILL.md rewrite) churn the `skills/helix/` dir and the reference format.

</specifics>

<deferred>
## Deferred Ideas

None — bundle hardening + contract hardening only. Generator fixes are Phase 104; SKILL.md rewrite is Phase 105; DSPy is Phase 106.

</deferred>
