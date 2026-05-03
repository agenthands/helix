---
phase: 58-v1-9-carryover-release-distribution
plan: 04
type: execute
wave: 1
depends_on: []
files_modified:
  - CONTRIBUTING.md
  - .planning/REQUIREMENTS.md
  - .planning/milestones/v1.10-ROADMAP.md
autonomous: true
requirements: [REL-05]
must_haves:
  truths:
    - "CONTRIBUTING.md contains the literal phrase 'Pass-3 limitation' with the trade-off + reopen-path rationale"
    - "REQUIREMENTS.md REL-02/03/04 carry `- [~]` markers with the EXACT rationale string from CONTEXT D-01"
    - "REQUIREMENTS.md has a Status legend section explaining `- [ ]` / `- [x]` / `- [~]`"
    - "v1.10-ROADMAP.md Phase 58 entry lists only REL-01, REL-05, REL-06 in Requirements (REL-02/03/04 removed)"
    - "v1.10-ROADMAP.md Phase 58 Success Criteria contains only the SC-1 + SC-4-equivalent items (SC-2 brew/scoop and SC-3 native Linux removed)"
    - "v1.10-ROADMAP.md Phase 58 entry references CONTEXT D-01 for the won't-do rationale"
  artifacts:
    - path: "CONTRIBUTING.md"
      provides: "Reproducibility section with Pass-3 limitation language and reopen-path rationale"
      contains: "Pass-3 limitation"
    - path: ".planning/REQUIREMENTS.md"
      provides: "REL-02/03/04 marked as won't-do with rationale; status legend"
      contains: "won't-do (v1.10)"
    - path: ".planning/milestones/v1.10-ROADMAP.md"
      provides: "Shrunken Phase 58 entry (REL-01/05/06 only) + post-shrink Success Criteria"
      contains: "[REL-01, REL-05, REL-06]"
  key_links:
    - from: ".planning/REQUIREMENTS.md"
      to: ".planning/milestones/v1.10-ROADMAP.md Phase 58 Requirements line"
      via: "matching REL-IDs (REL-01, REL-05, REL-06 only)"
      pattern: "REL-01.*REL-05.*REL-06"
    - from: ".planning/REQUIREMENTS.md REL-02/03/04 rationale"
      to: ".planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md D-01"
      via: "exact rationale string match"
      pattern: "won't-do \\(v1\\.10\\)"
---

<objective>
Three doc-only edits with grep-verifiable acceptance criteria:

1. **REL-05 — Reproducibility gate Pass-3 limitation documentation** in `CONTRIBUTING.md`. The literal phrase `Pass-3 limitation` must appear; the rationale must include the trade-off AND the reopen path ("if release-artifact divergence becomes a concern, a comparison job can be added") so a future maintainer can flip the decision without re-deriving context.
2. **D-01 won't-do recording for REL-02/03/04** in `.planning/REQUIREMENTS.md`. Each of the three lines flips from `- [ ]` to `- [~]` and appends the EXACT rationale string from CONTEXT D-01.
3. **v1.10-ROADMAP Phase 58 shrink** in `.planning/milestones/v1.10-ROADMAP.md`. The Phase 58 entry's Requirements list shrinks to `REL-01, REL-05, REL-06`. Success Criteria SC-2 (brew/scoop) and SC-3 (native Linux) are removed; surviving items are renumbered. A short note points at Phase 58 CONTEXT D-01 for the won't-do rationale.

This plan is independent of Plans 01/02/03 (no file overlap) and runs in Wave 1.

Purpose: REL-05 (D-05 reproducibility-gate documentation) + D-01 won't-do recording. Closes the second of three v1.9-tech-debt entries (the third — Phase 55 forwarder span — is closed by Plan 03; the post-phase `update_project_md` step moves all three entries to "Resolved at v1.10").
Output: Three documentation files in their final post-Phase-58 state.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-RESEARCH.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md
@.planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md
@CONTRIBUTING.md
@.planning/REQUIREMENTS.md
@.planning/milestones/v1.10-ROADMAP.md
</context>

<tasks>

<task type="auto">
  <name>Task 1: Add the Pass-3 limitation paragraph to CONTRIBUTING.md (REL-05)</name>
  <files>CONTRIBUTING.md</files>
  <read_first>
    - CONTRIBUTING.md (full file — locate the existing reproducibility paragraph at line ~161 per PATTERNS; do NOT touch the §Tracing section that Plan 03 may have just added)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`CONTRIBUTING.md`" (Option A edit — insert "Pass-3 limitation" anchor)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md D-05 ("Form" — explicit Pass-1/Pass-2/Pass-3 statement; trade-off; reopen path)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md §Specifics ("must explicitly use the phrase 'Pass-3 limitation'")
    - .planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md §Deferred (reopen-path wording: "if release-artifact divergence becomes a concern, a comparison job can be added")
  </read_first>
  <action>
    1. Locate the existing reproducibility paragraph in `CONTRIBUTING.md` (line ~161 per PATTERNS — first sentence: "The CI-enforced reproducibility gate runs two consecutive snapshot builds with identical inputs and refuses to publish if their archive sha256s differ, catching most build-environment non-determinism (toolchain drift, mod_timestamp, trimpath, GOFLAGS) before publication.").
    2. **EITHER** under that paragraph (preferred — reuses existing context), **OR** as a new H3 if the surrounding section structure makes a sub-anchor cleaner, add the following paragraph verbatim (Option A from PATTERNS):

       ```markdown
       This is the documented "Pass-3 limitation": the gate compares Pass-1, Pass-2, and Pass-3 snapshot hashes within the same source revision and does NOT compare those hashes against the real-release artifacts that ship to users. The trade-off is intentional — adding a real-artifact comparison job would require regenerating expected hashes per release, which the v1.9 milestone audit explicitly judged not worth the ongoing maintainer toil. If release-artifact divergence becomes a concern in the future (e.g., users report binary mismatches), a comparison job can be added; the current decision deliberately keeps the gate scope narrow so divergence detection lives at the published-artifact layer (cosign signature verification) rather than at the build layer.
       ```

       (The exact wording above carries every keyword the acceptance criteria search for: literal phrase `Pass-3 limitation`, the keywords `Pass-1`, `Pass-2`, `Pass-3`, `not` against published artifacts, the trade-off, AND the reopen-path phrase `comparison job can be added`.)

    3. Do NOT modify any unrelated section. In particular: do NOT touch `## Releasing`, `## Tracing` (Plan 03), or any non-reproducibility content.

    4. Verify (grep gates from 58-VALIDATION.md row 58-P4):
       ```bash
       grep -q 'Pass-3 limitation' CONTRIBUTING.md
       grep -q 'comparison job can be added' CONTRIBUTING.md
       ```
  </action>
  <verify>
    <automated>grep -q 'Pass-3 limitation' CONTRIBUTING.md &amp;&amp; grep -q 'comparison job can be added' CONTRIBUTING.md &amp;&amp; grep -qE 'Pass-1.*Pass-2.*Pass-3' CONTRIBUTING.md</automated>
  </verify>
  <acceptance_criteria>
    - `grep -c 'Pass-3 limitation' CONTRIBUTING.md` returns exactly 1
    - `grep -c 'comparison job can be added' CONTRIBUTING.md` returns at least 1 (reopen-path rationale)
    - `grep -E 'Pass-1.*Pass-2.*Pass-3' CONTRIBUTING.md` matches at least one line (explicit Pass-1/2/3 enumeration)
    - The existing reproducibility paragraph (the "snapshot builds with identical inputs" sentence) is preserved
    - The §Tracing section added by Plan 03 is untouched (`grep -q '## Tracing' CONTRIBUTING.md` if Plan 03 has merged)
  </acceptance_criteria>
  <done>
    CONTRIBUTING.md carries the Pass-3 limitation paragraph with the trade-off + reopen-path rationale. Three grep gates pass.
  </done>
</task>

<task type="auto">
  <name>Task 2: Mark REL-02/03/04 as won't-do in REQUIREMENTS.md with the exact rationale string</name>
  <files>.planning/REQUIREMENTS.md</files>
  <read_first>
    - .planning/REQUIREMENTS.md (full file — focus on lines 121-126 per PATTERNS; preserve the `**REQ-ID** (origin tag): description.` line shape)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`.planning/REQUIREMENTS.md`" (literal rewrite shape + status legend)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md D-01 (EXACT rationale string — quoted below)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-VALIDATION.md row 58-P4 (`grep -E '^- \[~\] \*\*REL-(02\|03\|04)' .planning/REQUIREMENTS.md \| wc -l` expects 3)
  </read_first>
  <action>
    1. **Edit the three lines for REL-02, REL-03, REL-04** (currently `- [ ]` per PATTERNS):

       Change the status marker from `- [ ]` to `- [~]` and append the EXACT rationale string from CONTEXT D-01 (PATTERNS confirms this is literal):
       ```
        — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
       ```

       After-edit shape (the three lines must read):
       ```markdown
       - [~] **REL-02** (was PKG-DEFER-03): A Homebrew tap publishes `helix` … — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
       - [~] **REL-03** (was PKG-DEFER-04): A Scoop bucket publishes `helix` … — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
       - [~] **REL-04** (was PKG-DEFER-05): A native Linux package … — won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
       ```

       The `…` placeholders represent whatever description text already exists in those lines — preserve the existing description verbatim and append the rationale after it via the ` — ` em-dash separator (per PATTERNS "**REQ-ID** (origin tag): description." line shape rule).

    2. **Add the status legend** (per PATTERNS §"`.planning/REQUIREMENTS.md`" — Status legend NEW). Place at the top of the REL-section (just above REL-01 line, OR at the top of the file under the H1, whichever matches existing legend conventions in the file — read first to decide):
       ```markdown
       ## Status legend

       - `- [ ]` pending
       - `- [x]` complete
       - `- [~]` won't-do (rejected with rationale appended after em-dash)
       ```

       If a similar status legend already exists elsewhere in the file, do NOT duplicate — extend the existing legend with the `- [~]` row instead.

    3. Do NOT touch REL-01, REL-05, REL-06 status markers (those remain `- [ ]` until they ship via Plans 01/02/03).

    4. Verify (literal command from 58-VALIDATION.md):
       ```bash
       count=$(grep -E '^- \[~\] \*\*REL-(02|03|04)' .planning/REQUIREMENTS.md | wc -l | tr -d ' ')
       [ "$count" = "3" ]
       ```
  </action>
  <verify>
    <automated>[ "$(grep -cE '^- \[~\] \*\*REL-(02|03|04)' .planning/REQUIREMENTS.md)" = "3" ] &amp;&amp; grep -c "won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience" .planning/REQUIREMENTS.md | grep -qE '^[3-9]' &amp;&amp; grep -q 'Status legend' .planning/REQUIREMENTS.md &amp;&amp; grep -q '\\- \[~\\] won.t-do' .planning/REQUIREMENTS.md</automated>
  </verify>
  <acceptance_criteria>
    - `grep -cE '^- \[~\] \*\*REL-(02|03|04)' .planning/REQUIREMENTS.md` returns exactly 3
    - The exact rationale string `won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience` appears at least 3 times (once per REL-02/03/04 line)
    - REL-01, REL-05, REL-06 still carry their pre-edit status markers (`- [ ]` until they ship via the other plans — do NOT advance their status here)
    - A "Status legend" section exists and explains `- [~]`
    - The line shape preserves the `**REL-XX** (was PKG-DEFER-YY):` prefix and the original description text (via the em-dash-separated rationale tail per PATTERNS)
  </acceptance_criteria>
  <done>
    REL-02/03/04 carry `- [~]` markers with the literal rationale; status legend explains the new marker.
  </done>
</task>

<task type="auto">
  <name>Task 3: Shrink Phase 58 entry in v1.10-ROADMAP.md (drop REL-02/03/04 + SC-2 + SC-3)</name>
  <files>.planning/milestones/v1.10-ROADMAP.md</files>
  <read_first>
    - .planning/milestones/v1.10-ROADMAP.md (full file — Phase 58 section at lines 42-51 per PATTERNS; Phase 57 at lines 36-40 as the desired post-shrink shape template)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-PATTERNS.md §"`.planning/milestones/v1.10-ROADMAP.md`" (concrete shrink instructions: drop REL-02/03/04 from Requirements; drop SC-2 and SC-3 from Success Criteria; renumber)
    - .planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md D-01 ("Recording mechanism" — explicitly says SC-2 and SC-3 are removed)
  </read_first>
  <action>
    1. **Edit the `**Requirements**:` line of the Phase 58 entry**: drop `REL-02`, `REL-03`, `REL-04` (in whatever delimiter shape the existing line uses — comma-separated list, bracketed list, etc. — match PATTERNS). After edit, the Requirements line MUST list only `REL-01, REL-05, REL-06` (in that order).

    2. **Edit the `**Success Criteria**` block of the Phase 58 entry**:
       - DROP the SC-2 item (Homebrew tap / Scoop bucket — the brew/scoop SC per PATTERNS)
       - DROP the SC-3 item (native Linux package — .deb/.rpm)
       - Surviving items: SC-1 (cosign-signed release with valid `.sigstore.json`) and the current SC-4 (whatever the original SC-4 text says — likely about reproducibility-gate doc OR the forwarder span). Renumber the surviving SC-4 to SC-2 (sequential numbering per PATTERNS).

    3. **Add a one-line note** below the Success Criteria (or as an inline comment at the top of the Phase 58 entry) referencing CONTEXT D-01. Suggested wording:
       ```markdown
       > Phase 58 originally inherited REL-02/03/04 (Homebrew, Scoop, Linux pkg) from v1.9 deferral. These are recorded as won't-do per `.planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md` D-01; see REQUIREMENTS.md for rationale.
       ```

    4. **Update the `**Plans**:` line** for Phase 58. After this plan ships there will be 4 plans (58-01, 58-02, 58-03, 58-04). Replace the current `**Plans**: TBD` (per PATTERNS) with:
       ```markdown
       **Plans**: 4 plans
       - [ ] 58-01-release-side-cosign-PLAN.md — release-side cosign keyless migration [REL-01]
       - [ ] 58-02-verifier-rewrite-PLAN.md — internal/upgrade/ cosign verifier rewrite [REL-01]
       - [ ] 58-03-forwarder-otel-unification-PLAN.md — forwarder.tools.call span unified with daemon gRPC server span [REL-06]
       - [ ] 58-04-wontdo-and-repro-doc-PLAN.md — REL-02/03/04 won't-do recording + REL-05 Pass-3 limitation doc [REL-05]
       ```

    5. Do NOT touch any other phase's entry in this roadmap. Verify Phase 57 / Phase 59 shapes are unchanged.

    6. Verify:
       ```bash
       grep -A 20 '^### Phase 58' .planning/milestones/v1.10-ROADMAP.md | grep -E 'Requirements.*REL-01.*REL-05.*REL-06'
       grep -A 20 '^### Phase 58' .planning/milestones/v1.10-ROADMAP.md | grep -v 'REL-02\|REL-03\|REL-04'
       ```
  </action>
  <verify>
    <automated>awk '/^### Phase 58/,/^### Phase [0-9]/' .planning/milestones/v1.10-ROADMAP.md | grep -E 'Requirements.*REL-01.*REL-05.*REL-06' &amp;&amp; ! awk '/^### Phase 58/,/^### Phase [0-9]/' .planning/milestones/v1.10-ROADMAP.md | grep -E 'REL-02|REL-03|REL-04' &amp;&amp; awk '/^### Phase 58/,/^### Phase [0-9]/' .planning/milestones/v1.10-ROADMAP.md | grep -q '58-01-release-side-cosign-PLAN\.md' &amp;&amp; awk '/^### Phase 58/,/^### Phase [0-9]/' .planning/milestones/v1.10-ROADMAP.md | grep -q '58-04-wontdo-and-repro-doc-PLAN\.md' &amp;&amp; awk '/^### Phase 58/,/^### Phase [0-9]/' .planning/milestones/v1.10-ROADMAP.md | grep -q 'D-01'</automated>
  </verify>
  <acceptance_criteria>
    - The Phase 58 `**Requirements**:` line lists exactly `REL-01, REL-05, REL-06` (in that order); no REL-02/03/04 hits anywhere within the Phase 58 entry
    - The Phase 58 `**Success Criteria**` has SC-2 and SC-3 removed; surviving items are renumbered sequentially
    - The Phase 58 `**Plans**:` line names all 4 plan files (`58-01-release-side-cosign-PLAN.md`, `58-02-verifier-rewrite-PLAN.md`, `58-03-forwarder-otel-unification-PLAN.md`, `58-04-wontdo-and-repro-doc-PLAN.md`)
    - The Phase 58 entry contains a reference to CONTEXT D-01 (`grep -q 'D-01'` passes within the Phase 58 block)
    - No other phase's entry in this roadmap is changed
  </acceptance_criteria>
  <done>
    Phase 58 entry in v1.10-ROADMAP.md lists only REL-01/05/06; SC-2/SC-3 removed; 4 plan files enumerated; D-01 cross-reference present.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Plan author → future maintainer (docs) | Reproducibility-gate Pass-3 wording is the authoritative reference for whether to add a real-artifact comparison job |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-58-doc-01 | Repudiation | Future maintainer cannot find the reasoning behind the Pass-3 limitation decision | mitigate | The literal phrase `Pass-3 limitation` + reopen-path wording (`comparison job can be added`) provides a grep anchor; the rationale paragraph names the v1.9 milestone audit so the historical decision context is recoverable |
| T-58-doc-02 | Repudiation | Future maintainer cannot find why REL-02/03/04 were dropped | mitigate | Exact rationale string is recorded BOTH in REQUIREMENTS.md (per-line tail) AND referenced from v1.10-ROADMAP.md Phase 58 entry; CONTEXT D-01 documents the full reasoning |
| T-58-doc-03 | Tampering | Drift between REQUIREMENTS.md status markers and roadmap Requirements list | mitigate | Both files updated in the same plan + same commit set; acceptance criteria require matching REL-IDs in both places |
</threat_model>

<verification>
- All three grep-gate sets from 58-VALIDATION.md row 58-P4 pass:
  - `grep -q 'Pass-3 limitation' CONTRIBUTING.md`
  - `grep -E '^- \[~\] \*\*REL-(02|03|04)' .planning/REQUIREMENTS.md | wc -l` returns 3
- v1.10-ROADMAP.md Phase 58 entry is consistent with REQUIREMENTS.md (only REL-01/05/06 in scope; D-01 cross-referenced)
- No accidental drift: REL-01, REL-05, REL-06 status markers untouched (they remain pending until shipped by Plans 01/02/03)
- Plan 03's §Tracing addition to CONTRIBUTING.md (if landed first) is preserved
</verification>

<success_criteria>
- All three doc files reach their final post-Phase-58 state in a single coordinated commit
- Future grep for `Pass-3 limitation` in CONTRIBUTING.md, `won't-do (v1.10)` in REQUIREMENTS.md, or REL-02/03/04 in v1.10-ROADMAP Phase 58 yields the expected results
- Plan 03's CONTRIBUTING.md edits do NOT conflict with this plan's CONTRIBUTING.md edits (different sections — §Tracing vs §Reproducibility/§Releasing)
- Post-phase `update_project_md` step (NOT in this plan; per CONTEXT §Deferred) can move the three v1.9-tech-debt entries to "Resolved at v1.10"
</success_criteria>

<output>
After completion, create `.planning/phases/58-v1-9-carryover-release-distribution/58-04-SUMMARY.md` documenting:
- Whether the Pass-3 limitation paragraph was added under the existing reproducibility paragraph or as a new H3
- Whether the status legend in REQUIREMENTS.md was added fresh or extended an existing legend
- Confirmation that no other roadmap phase entries were modified
- Confirmation that the §Tracing section (if Plan 03 landed first) was preserved
</output>
