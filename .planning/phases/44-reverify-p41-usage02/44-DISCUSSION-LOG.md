# Phase 44: Re-Verify Phase 41 and USAGE-02 - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-04-23
**Phase:** 44-reverify-p41-usage02
**Areas discussed:** Verification re-run mechanics, Integration re-check scope, Handling residual findings, Plan decomposition

---

## Verification Re-Run Mechanics

### Q1: How should Phase 40's VERIFICATION.md be re-run?

| Option | Description | Selected |
|--------|-------------|----------|
| Update in place (Recommended) | Overwrite 40-VERIFICATION.md, flip the 3 gaps_found items to SATISFIED with new evidence. Git history preserves the prior version. | ✓ |
| Write 40-VERIFICATION-v2.md | Leave original intact, add a v2 file noting delta. More audit trail but splits truth across two files. | |
| Append a re-run section | Keep original, append '## Re-run 2026-04-23' section to the same file. Single file but mixes two passes. | |

**User's choice:** Update in place (Recommended)
**Notes:** Canonical single file; rely on git for history.

### Q2: How should Phase 41's new VERIFICATION.md be structured?

| Option | Description | Selected |
|--------|-------------|----------|
| Mirror 40-VERIFICATION.md template (Recommended) | Same req-ID-by-req-ID table format as Phase 40 for consistency. Covers INST-01, INST-02, CONT-01, CONT-02. | ✓ |
| Minimal pass/fail list | Shorter format — one line per requirement. Faster to produce but breaks template consistency. | |
| Full nyquist-compliant format | Full format including anti-patterns, evidence links, UAT scenarios. Most rigorous, longest. | |

**User's choice:** Mirror 40-VERIFICATION.md template (Recommended)

---

## Integration Re-Check Scope

### Q3: What scope for the v1.8 integration re-check?

| Option | Description | Selected |
|--------|-------------|----------|
| Full re-run of all 11 findings (Recommended) | Re-verify F-01 through F-13 against current doc state. Produces fresh status_total comparable to the original check. Most defensible for milestone close-out. | ✓ |
| Targeted — 3 criticals + Phase 43 deltas | Only re-verify F-01 (client list), F-11 (tool count), F-13 (Phase 41 unverified), plus any finding touched by Phase 43 plans. | |
| Criticals only | Re-verify just F-01, F-11, F-13. Fastest. Leaves warnings unconfirmed. | |

**User's choice:** Full re-run of all 11 findings (Recommended)

### Q4: Where should the re-run output live?

| Option | Description | Selected |
|--------|-------------|----------|
| Overwrite v1.8-INTEGRATION-CHECK.md (Recommended) | Replace the existing file with the re-run. Git preserves history. Keeps a single canonical milestone integration check. | ✓ |
| New file: v1.8-INTEGRATION-CHECK-v2.md | Write a second file. Both readable side by side at cost of doubled milestone artifacts. | |

**User's choice:** Overwrite v1.8-INTEGRATION-CHECK.md (Recommended)

---

## Handling Residual Findings

### Q5: If the re-run still surfaces criticals, what does Phase 44 do?

| Option | Description | Selected |
|--------|-------------|----------|
| Defer to Phase 45 (Recommended) | Phase 45 (Cross-link & Manual-Config Polish) is the designated follow-up. Phase 44 records residuals as handoff items and completes. Avoids phase bloat. | ✓ |
| Halt Phase 44 until zero criticals | Strict reading of SC-3. Could balloon scope if new issues surface. Forces fix-in-place. | |
| Fold critical fixes into Phase 44 | Middle ground — fix criticals here, defer warnings to Phase 45. Risks scope creep. | |

**User's choice:** Defer to Phase 45 (Recommended)

### Q6: How strict is "no remaining critical findings" (SC-3)?

| Option | Description | Selected |
|--------|-------------|----------|
| Zero open criticals OR all documented as deferred (Recommended) | Criticals must either be resolved or explicitly deferred with a target phase / backlog ID. Matches Phase 43's treatment of follow-ups. | ✓ |
| Zero open criticals — no deferrals | SC-3 fails unless all criticals are resolved in-phase. Strictest reading. | |

**User's choice:** Zero open criticals OR all documented as deferred (Recommended)

---

## Plan Decomposition

### Q7: How should Phase 44 be decomposed into plans?

| Option | Description | Selected |
|--------|-------------|----------|
| 3 plans, sequential (Recommended) | 44-01 write 41-VERIFICATION.md → 44-02 re-run 40-VERIFICATION → 44-03 re-run v1.8 integration check. Integration check reads both upstream verifications, so sequential is natural. | ✓ |
| 3 plans, 44-01 and 44-02 parallel | Write 41-VERIFICATION and re-run 40-VERIFICATION in parallel (independent phases), then 44-03 integration. Slightly faster, same artifact count. | |
| 2 plans | 44-01 combined per-phase verification (covers both 40 and 41) → 44-02 integration re-check. Larger plan scope each. | |
| 1 plan | Everything in one plan. Simplest but least checkpoint granularity. | |

**User's choice:** 3 plans, sequential (Recommended)

---

## Claude's Discretion

- Phrasing, section ordering, evidence formatting within each VERIFICATION.md (provided the template mirrors Phase 40).
- Which Phase 43 commit SHAs to cite as evidence for each re-flipped gap.
- Whether to preserve or adjust the integration-check heading structure for the re-run.
- Commit granularity inside each plan.

## Deferred Ideas

- README manual-config cross-reference polish (F-02) → Phase 45.
- Architecture terminology alignment across CLAUDE.md / CONTRIBUTING.md → Phase 45 candidate.
- CHANGELOG v1.7 client-count fixup (F-07) → Phase 45 if not absorbed by Phase 43.
- Nyquist-compliant upgrade of 40-VALIDATION.md / 39-VALIDATION.md → dedicated validation phase, out of v1.8 scope.
- Human UAT scenarios from 39-HUMAN-UAT.md → deferred indefinitely.
