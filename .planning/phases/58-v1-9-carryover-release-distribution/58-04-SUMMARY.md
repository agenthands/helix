---
phase: 58-v1-9-carryover-release-distribution
plan: 04
subsystem: docs
tags: [docs, planning, requirements, roadmap, wontdo, REL-05]
requires: []
provides:
  - "CONTRIBUTING.md Pass-3 limitation paragraph (REL-05)"
  - "REQUIREMENTS.md REL-02/03/04 marked won't-do with rationale"
  - "REQUIREMENTS.md Status legend section"
  - "v1.10-ROADMAP.md shrunken Phase 58 entry (REL-01/05/06 only)"
affects:
  - CONTRIBUTING.md
  - .planning/REQUIREMENTS.md
  - .planning/milestones/v1.10-ROADMAP.md
tech-stack:
  added: []
  patterns:
    - "Status marker [~] for won't-do rejected requirements"
    - "Em-dash separator for rationale tail on requirements lines"
    - "D-01 cross-reference for won't-do rationale linkage"
key-files:
  created:
    - .planning/phases/58-v1-9-carryover-release-distribution/58-04-SUMMARY.md
  modified:
    - CONTRIBUTING.md
    - .planning/REQUIREMENTS.md
    - .planning/milestones/v1.10-ROADMAP.md
decisions:
  - "Pass-3 limitation paragraph added BELOW the existing reproducibility paragraph (Option A from PATTERNS) — reuses existing context rather than introducing a new H3"
  - "Status legend added as a fresh top-level section under the H1, above v1.10 Requirements (no pre-existing legend in the file)"
metrics:
  completed: 2026-05-03
  duration_minutes: ~10
  tasks_completed: 3
  files_modified: 3
---

# Phase 58 Plan 04: REL-02/03/04 Won't-Do Recording + REL-05 Pass-3 Limitation Doc

**Three doc-only edits closing REL-05 (D-05 reproducibility-gate documentation) and recording the D-01 won't-do decision for REL-02/03/04 in REQUIREMENTS.md and v1.10-ROADMAP.md.**

## What Shipped

### Task 1: CONTRIBUTING.md Pass-3 limitation (REL-05)

Added a new paragraph immediately after the existing reproducibility paragraph in `## Releasing`. The paragraph carries:

- The literal anchor phrase `Pass-3 limitation`
- Explicit Pass-1 / Pass-2 / Pass-3 enumeration
- The trade-off rationale (v1.9 milestone audit deemed real-artifact comparison job toil unjustified)
- The reopen-path wording (`comparison job can be added` if release-artifact divergence becomes a concern)

The §Tracing section (added by Plan 03 in a prior commit) is preserved untouched; the Pass-3 paragraph is purely additive in `## Releasing`.

**Verification:**
- `grep -c 'Pass-3 limitation' CONTRIBUTING.md` → 1
- `grep -c 'comparison job can be added' CONTRIBUTING.md` → 1
- `grep -E 'Pass-1.*Pass-2.*Pass-3' CONTRIBUTING.md` → matches one line
- `grep -q '## Tracing' CONTRIBUTING.md` → preserved

### Task 2: REQUIREMENTS.md REL-02/03/04 won't-do recording

Three lines flipped from `- [ ]` to `- [~]` with the EXACT D-01 rationale string appended via em-dash separator:

```
won't-do (v1.10) — self-contained binary is the only distribution channel; package channels add maintenance burden without reaching the agent-targeted audience
```

REL-01, REL-05, REL-06 status markers are untouched (they remain `- [ ]` until shipped by sibling plans 58-01/02/03 in this same phase).

A new "Status legend" section was added as a fresh top-level H2 under the file's H1 (just above `## v1.10 Requirements`). No pre-existing legend was present anywhere in the file, so this is a brand-new section, not an extension.

**Verification:**
- `grep -cE '^- \[~\] \*\*REL-(02|03|04)' .planning/REQUIREMENTS.md` → 3
- exact rationale string match count → 3
- `grep -q 'Status legend' .planning/REQUIREMENTS.md` → present
- REL-01/05/06 still on `- [ ]` (verified by direct grep)

### Task 3: v1.10-ROADMAP.md Phase 58 shrink

Three sub-edits in this file:

1. **Top-of-file Phase 58 one-line summary (line 13):** rewrote to remove every occurrence of `minisign`, `Homebrew`, `Scoop`, `native Linux package` and added `cosign`. New text: "first signed Helix release using sigstore cosign keyless signing, Phase 51 reproducibility-gate Pass-3 limitation documented, Phase 55 `forwarder.tools.call` span unified with the gRPC server span".

2. **Phase 58 detail block:**
   - `**Requirements**:` line shrunk from `REL-01..REL-06` to `REL-01, REL-05, REL-06`.
   - `**Success Criteria**` shrunk from 4 items to 2: SC-1 (cosign-signed release with Rekor inclusion-proof) and the renumbered SC-2 (the previous SC-4: CONTRIBUTING.md Pass-3 doc + forwarder span unification). Original SC-2 (brew/scoop) and SC-3 (native Linux package) removed.
   - `**Goal**:` lightly edited so "re-scope or land the deferred distribution channels" → "explicitly de-scope the deferred distribution channels (Homebrew, Scoop, native Linux package)" — keeps wording aligned with the won't-do decision.
   - One-line note added below Success Criteria pointing at `.planning/phases/58-v1-9-carryover-release-distribution/58-CONTEXT.md` D-01 with a `> ...` blockquote.
   - `**Plans**: TBD` replaced with `**Plans**: 4 plans` and four bullet entries naming each plan file.

3. **No other phase entries modified.** `git diff --stat` confirms changes are localized to lines 13 and the Phase 58 detail block (lines 42-60ish post-edit). Phase 57, Phase 59, and onward are byte-identical.

The Progress table row for Phase 58 (`0/0`) was deliberately NOT updated — that's the orchestrator's responsibility per the parallel-executor instructions (do not modify shared orchestrator artifacts).

**Verification:**
- Line 13 contains `cosign` and not `minisign`/`Homebrew`/`Scoop`/`native Linux package`
- Phase 58 `**Requirements**:` line: `REL-01, REL-05, REL-06`
- Phase 58 block contains `D-01` reference
- Phase 58 block enumerates all 4 plan files (`58-01..58-04`)
- `git diff --stat` confirms only `.planning/milestones/v1.10-ROADMAP.md` changed for this task; only Phase 58 + line 13 within the file

## Output Notes (per plan §output)

- The Pass-3 limitation paragraph was added **under the existing reproducibility paragraph** (Option A — preferred per PATTERNS), not as a new H3.
- The Status legend in REQUIREMENTS.md was added **fresh** as a new H2 section; no similar legend existed elsewhere in the file.
- **No other roadmap phase entries were modified** — only Phase 58 + the line-13 top-of-file summary.
- The §Tracing section in CONTRIBUTING.md (Plan 03's contribution) was **preserved** — confirmed by `grep -q '## Tracing'`.

## Deviations from Plan

None — plan executed exactly as written. All grep gates from 58-VALIDATION.md row 58-P4 pass.

## Threat Flags

None. This plan is doc-only; T-58-doc-01..03 mitigations are satisfied by the literal anchor phrases (`Pass-3 limitation`, `won't-do (v1.10)`) being present where the threat model said they must be, providing future maintainers grep-anchored decision context.

## Commits

- `5621cb36` — docs(58-04): document Pass-3 limitation in CONTRIBUTING.md (REL-05)
- `515829e7` — docs(58-04): mark REL-02/03/04 won't-do with D-01 rationale
- `bf86b8e1` — docs(58-04): shrink Phase 58 entry in v1.10-ROADMAP (drop REL-02/03/04, SC-2/SC-3)

## Self-Check

Verification of created/modified files and commit hashes performed in the next step.
