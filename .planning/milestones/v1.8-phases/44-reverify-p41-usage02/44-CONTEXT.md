# Phase 44: Re-Verify Phase 41 and USAGE-02 - Context

**Gathered:** 2026-04-23
**Status:** Ready for planning

<domain>
## Phase Boundary

Close the v1.8 audit gaps by producing the missing Phase 41 VERIFICATION.md, re-running Phase 40's USAGE-02 verification now that Phase 43 has shipped the underlying doc fixes, and re-running the v1.8 milestone integration check so v1.8 can close with a clean audit trail.

This phase is procedural re-verification of already-shipped work. It does not re-open Phase 41 or Phase 40 implementations and does not add new documentation capabilities. Any residual doc-polish work surfaced by the re-check is deferred to Phase 45 (Cross-link & Manual-Config Polish).
</domain>

<decisions>
## Implementation Decisions

### Verification Re-Run Mechanics
- **D-01:** Re-run Phase 40's verification by overwriting `40-VERIFICATION.md` in place. Flip the three `gaps_found` items (get_repo_map parameter name, fuzzy_edit named-args example, rust-analyzer rename troubleshooting) to `SATISFIED` with new evidence citing the Phase 43 commits that fixed them. Git history preserves the prior version; do not create a `-v2.md`.
- **D-02:** Phase 41's new `41-VERIFICATION.md` mirrors the `40-VERIFICATION.md` template exactly — requirement-ID-by-requirement-ID table, same section order, same evidence-citation style. Covers INST-01, INST-02, CONT-01, CONT-02.
- **D-03:** Verification evidence must cite real artifacts (file:line, commit SHA, or `serena setup --output` output). No claims without a citation.

### Integration Re-Check Scope
- **D-04:** Re-run the full v1.8 integration check — all 11 findings (F-01 through F-13), not just the three criticals. Produces a `status_total` comparable to the original check, which is what SC-3 ("no remaining critical findings") actually requires to assert.
- **D-05:** Overwrite `.planning/v1.8-INTEGRATION-CHECK.md` with the re-run. Git preserves the prior version. A single canonical milestone integration check is the intended artifact.

### Residual Findings Handling
- **D-06:** Any residual critical findings that surface in the re-run are deferred to Phase 45, not fixed in Phase 44. Phase 45 is the designated follow-up for Cross-link & Manual-Config Polish. Keeping Phase 44 scoped to verification avoids scope bloat.
- **D-07:** SC-3 ("no remaining critical findings") is interpreted as: **zero open criticals OR every critical explicitly deferred** to Phase 45 (or the backlog) with a target phase / finding ID recorded. A critical tagged `deferred_to: 45` in the integration-check frontmatter counts as resolved for the purpose of closing Phase 44.

### Plan Decomposition
- **D-08:** Three plans, executed sequentially:
  1. `44-01` — Write `41-VERIFICATION.md` (INST-01, INST-02, CONT-01, CONT-02 from scratch).
  2. `44-02` — Re-run `40-VERIFICATION.md` (in-place update; flip three gaps to SATISFIED).
  3. `44-03` — Re-run v1.8 integration check (reads outputs from 44-01 and 44-02).
- **D-09:** Sequencing is required: `44-03` reads both verification files to assert `phases_verified: [39, 40, 41, 42]` in its frontmatter.

### Claude's Discretion
- Exact phrasing, section ordering, and evidence formatting within each VERIFICATION.md (provided the template mirrors Phase 40).
- Which commits from Phase 43 to cite as evidence for which re-flipped gap.
- Whether to re-use the existing integration-check section structure verbatim or adjust headings to make re-run deltas obvious.
- Commit granularity inside each plan (atomic per-file vs. combined).
</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Audit Inputs (what this phase is closing)
- `.planning/v1.8-MILESTONE-AUDIT.md` — Source of all audit gaps being resolved; defines INST-01/INST-02/CONT-01/CONT-02 as `orphaned` and USAGE-02 as `unsatisfied`.
- `.planning/v1.8-INTEGRATION-CHECK.md` — The integration check being re-run; contains F-01 through F-13 with severities and recommendations.

### Phase 40 Artifacts (re-verify target)
- `.planning/phases/40-usage-refresh/40-VERIFICATION.md` — File to update in place; source of the three `gaps_found` items (get_repo_map param, fuzzy_edit example, rust-analyzer rename).
- `.planning/phases/40-usage-refresh/40-CONTEXT.md` — Phase 40 decisions; referenced to confirm what was in scope.
- `.planning/phases/40-usage-refresh/40-01-SUMMARY.md`, `40-02-SUMMARY.md`, `40-03-SUMMARY.md` — What Phase 40 shipped.

### Phase 41 Artifacts (verify target)
- `.planning/phases/41-install-contributing/41-CONTEXT.md` — Phase 41 scope and decisions.
- `.planning/phases/41-install-contributing/41-01-PLAN.md`, `41-02-PLAN.md` — What Phase 41 claimed.
- `.planning/phases/41-install-contributing/41-01-SUMMARY.md`, `41-02-SUMMARY.md` — What Phase 41 shipped.

### Phase 43 Artifacts (evidence that gaps were fixed)
- `.planning/phases/43-cross-doc-sync/` — All six plans and summaries. 44-02 evidence citations will point here, especially for strategy names and USAGE/CLAUDE.md truth sync.

### Source of Truth for Client List (F-01 anchor)
- `internal/cli/setup_clients.go:39-45` — Ground-truth client registry; any integration-check claim about "supported clients" must reconcile with this file.

### Requirements Definitions
- `.planning/REQUIREMENTS.md` — Definitions of INST-01, INST-02, CONT-01, CONT-02, USAGE-02 and their acceptance criteria.

### Milestone Scope
- `.planning/ROADMAP.md` Phase 44 entry — Success criteria SC-1, SC-2, SC-3 that this phase must satisfy.
- `.planning/MILESTONES.md` v1.8 section — Milestone exit criteria.
</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `.planning/phases/40-usage-refresh/40-VERIFICATION.md`: Template for Phase 41's new verification file. Already follows project convention (frontmatter + per-req table + evidence citations).
- `.planning/v1.8-INTEGRATION-CHECK.md`: Template for the re-run. Section structure (Summary, Cross-Doc Consistency, per-finding blocks with Evidence + Recommendation) should be preserved.
- `internal/cli/setup_clients.go`: Authoritative client registry — seven clients. Every integration-check claim about client counts reconciles against this file.

### Established Patterns
- Verification files use YAML frontmatter with `milestone`, `phase`, `verified`, `status`, `requirements_total`, and per-req status fields. Phase 41's new file follows this exactly.
- Evidence citations use `path/to/file.ext:line` format, sometimes with a commit SHA suffix for cross-phase evidence.
- Integration checks classify findings as `critical | warning | info` and track `phases_verified` / `phases_unverified` in frontmatter.

### Integration Points
- The v1.8 milestone audit (`v1.8-MILESTONE-AUDIT.md`) references the integration check by path. If the re-run overwrites the file in place (D-05), no cross-reference updates are needed.
- `/gsd-complete-milestone` will run against v1.8 once Phase 44 closes; it reads the integration check to assert milestone-exit criteria.
</code_context>

<specifics>
## Specific Ideas

- Use Phase 43's commits as the primary evidence anchor when flipping Phase 40's three gaps. Each flipped gap cites the specific commit SHA that closed it.
- The re-run integration check should add a short prologue noting the re-run date and which phases now have VERIFICATION.md (vs. the original's `phases_unverified: [41]` note).
- For residual criticals, record the deferral directly in the integration-check frontmatter as `deferred_to: 45` per finding ID so Phase 45 has a machine-readable inbox.
</specifics>

<deferred>
## Deferred Ideas

- README manual-config cross-reference polish (F-02) — belongs in Phase 45.
- Architecture-terminology alignment across CLAUDE.md / CONTRIBUTING.md (F-10, F-11 warning items if unresolved) — Phase 45 candidate.
- CHANGELOG v1.7 client-count fixup (F-07) — Phase 45 if not already absorbed by Phase 43.
- Nyquist-compliant upgrade of `40-VALIDATION.md` and `39-VALIDATION.md` — deferred to dedicated validation phase, not in v1.8 scope.
- Human UAT scenarios from `39-HUMAN-UAT.md` — product-positioning flow, not machine-verifiable, deferred indefinitely.
</deferred>

---

*Phase: 44-reverify-p41-usage02*
*Context gathered: 2026-04-23*
