# Requirements — Milestone v2.5 Agent Harness Rebuild (TUNE-FUT-06)

## Overview

**Goal:** Make the optimization agent actually solve tasks by editing — with a feedback loop and real GEPA tuning — so that steering deltas become meaningful.

**Why now:** v2.4's +0.0392 delta is noise between two broken configs. The agent makes 0 tool calls on ~40% of tasks (answers in prose), has no feedback loop, burns verb-error budget, and GEPA reflection is a no-op (AgentProgram emits no predictor trace). The pipeline plumbing works; the harness doesn't.

**Root causes (from v2.4 investigation):**

| Finding | Severity | Fix |
|---------|----------|-----|
| ~40% of tasks: 0 tool calls | **blocking** | Force editing in system prompt |
| No run-tests in loop | **blocking** | Add feedback loop |
| Verb errors burn budget | **blocking** | Harden arg usage |
| SKILL.md is wrong artifact | **advisory** | Build task-solving prompt |
| GEPA no-op | **blocking** | Real `dspy` module |

---

## HARNESS — Agent Harness Rebuild

### HARNESS-01: Task-solving System Prompt

- [ ] **HARNESS-01a:** System prompt names the solution file explicitly (e.g., "edit `solution.py` to solve the exercise")
- [ ] **HARNESS-01b:** Declares success criterion: "success = hidden tests pass"
- [ ] **HARNESS-01c:** Forbids prose answers: "do NOT answer in prose; not done until implemented"
- [ ] **HARNESS-01d:** Anti-vacuity test: prose-only answer on a task → assert FAIL

### HARNESS-02: Feedback Loop

- [ ] **HARNESS-02a:** Wire run-tests into ReAct loop (agent runs tests after edit)
- [ ] **HARNESS-02b:** Wire get-diagnostics into ReAct loop (agent checks for compilation errors)
- [ ] **HARNESS-02c:** Agent uses test results to drive next edit (iterate until pass or budget exhausted)
- [ ] **HARNESS-02d:** Anti-vacuity test: agent declares done on broken code without running tests → assert FAIL

### HARNESS-03: Verb-Arg Hardening

- [ ] **HARNESS-03a:** Audit verb call sites for positional-arg misuse (the v2.4 gap)
- [ ] **HARNESS-03b:** Add error-budget tracking (max N verb errors before abort)
- [ ] **HARNESS-03c:** Anti-vacuity test: malformed argv passes silently → assert FAIL

### HARNESS-04: Real GEPA Module

- [ ] **HARNESS-04a:** Rebuild agent as real `dspy.Module` (not ad-hoc Python)
- [ ] **HARNESS-04b:** AgentProgram emits reflectable predictor trace for GEPA
- [ ] **HARNESS-04c:** GEPA `forward` returns trace that reflective mutation can optimize
- [ ] **HARNESS-04d:** Anti-vacuity test: empty/constant trace on real run → assert FAIL

### HARNESS-05: Re-run Attribution

- [ ] **HARNESS-05a:** Re-run v2.4 attribution pipeline on fixed harness (same corpus, same split)
- [ ] **HARNESS-05b:** Verify agent tool-call rate ≥ 80% on held-out split (demonstrates engagement)
- [ ] **HARNESS-05c:** Record ON/OFF delta with per-arm cost (same attribution.py from v2.4)
- [ ] **HARNESS-05d:** Gate for TUNE-FUT-03: if delta > 0 AND significant, SKILL.md adoption path ready

---

## Constraint Cross-Check

| Constraint | Phase 1 | Phase 2 | Phase 3 | Phase 4 |
|------------|---------|---------|---------|---------|
| Zero new Go deps | ✓ | ✓ | ✓ | ✓ |
| Tuning in dev-venv Python | ✓ | ✓ | ✓ | ✓ |
| Anti-vacuity tests | ✓ | ✓ | ✓ | ✓ |
| Forced dependency chain | 01/02 first | 03 after 01/02 | 04 after 03 | 05 after 01-04 |

---

## Dependency Chain (Enforced)

```
Phase 1 (HARNESS-01/02) ──► Phase 2 (HARNESS-03) ──► Phase 3 (HARNESS-04) ──► Phase 4 (HARNESS-05)
```

- Phase 2 requires Phase 1 (can't harden verbs if agent doesn't use them)
- Phase 3 requires Phase 2 (can't tune GEPA if agent still fails on basics)
- Phase 4 requires Phases 1-3 (can't measure meaningful delta on broken agent)

---

## Out of Scope

- Corpus growth (already past `val_size>50` gate at 103 tasks)
- SWE-bench scale-up (TUNE-FUT-05)
- SKILL.md adoption (TUNE-FUT-03, human-gated separate step)
- Significance test in `decide_ship` (future enhancement)

---

## Traceability

*Phase mapping to be filled by ROADMAP.md*

| REQ-ID | Phase | Status |
|--------|-------|--------|
| HARNESS-01a | — | pending |
| HARNESS-01b | — | pending |
| HARNESS-01c | — | pending |
| HARNESS-01d | — | pending |
| HARNESS-02a | — | pending |
| HARNESS-02b | — | pending |
| HARNESS-02c | — | pending |
| HARNESS-02d | — | pending |
| HARNESS-03a | — | pending |
| HARNESS-03b | — | pending |
| HARNESS-03c | — | pending |
| HARNESS-04a | — | pending |
| HARNESS-04b | — | pending |
| HARNESS-04c | — | pending |
| HARNESS-04d | — | pending |
| HARNESS-05a | — | pending |
| HARNESS-05b | — | pending |
| HARNESS-05c | — | pending |
| HARNESS-05d | — | pending |