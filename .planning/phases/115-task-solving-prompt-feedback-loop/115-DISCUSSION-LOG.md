# Phase 115: Task-Solving Prompt + Feedback Loop - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-26
**Phase:** 115-Task-Solving-Prompt-Feedback-Loop
**Areas discussed:** Solution file conveyance, Success criterion, Feedback loop integration, Prose enforcement, Test discovery

---

## Solution File Conveyance

| Option | Description | Selected |
|--------|-------------|----------|
| Extract from task | Prompt instructs agent to identify the solution file from the task description (e.g., 'edit `solution.py` as described') | |
| Pass as context | The harness passes the solution file path as a separate context variable alongside the task | |
| Both paths | HARNESS-01a: Explicit path when available, prompt instructs extraction as fallback | ✓ |

**User's choice:** Both paths
**Notes:** The user selected the hybrid approach — when the task explicitly names a file, use it; when implied, extract from description.

---

## Success Criterion Declaration

| Option | Description | Selected |
|--------|-------------|----------|
| Prompt declares it | System prompt says 'you are done when hidden tests pass' — agent discovers tests via get-diagnostics | |
| Harness provides test file | The harness passes the test file path as context, prompt says 'tests in <file> must pass' | |
| Both discovery paths | HARNESS-02a: Prompt declares criterion + get-diagnostics in loop finds tests automatically | ✓ |

**User's choice:** Both discovery paths
**Notes:** The prompt declares the goal ("hidden tests must pass"), and the agent discovers test files via get-diagnostics. No hardcoding of test locations.

---

## Feedback Loop Integration

| Option | Description | Selected |
|--------|-------------|----------|
| Agent-driven | Agent explicitly calls run-tests / get-diagnostics as tool calls (current verb set) — agent decides when to check | |
| Harness-injected | After each edit, harness automatically runs get-diagnostics and injects results into the next turn | |
| Hybrid | HARNESS-02c: Agent-driven by default, but prompt instructs to check after edits; anti-vacuity test enforces it | ✓ |

**User's choice:** Hybrid
**Notes:** The agent chooses when to run tests (driven by prompt instruction), not automatic injection. The anti-vacuity test ensures the agent actually runs tests before declaring done.

---

## Prose Enforcement

| Option | Description | Selected |
|--------|-------------|----------|
| Retry with nudge | If agent returns text without tool calls, inject a follow-up prompt reminding it to edit files. One retry allowed per turn. | |
| Prompt forbids + test | System prompt explicitly forbids prose: 'do NOT answer in prose; not done until implemented.' Anti-vacuity test (HARNESS-01d) catches violations. | |
| Both + fail on repeat | HARNESS-01d: Retry nudge on first prose attempt, fail the task if agent persists. Test asserts fail on repeated prose. | ✓ |

**User's choice:** Both + fail on repeat
**Notes:** First prose attempt gets a nudge to edit. If agent persists with prose, the task is failed. The anti-vacuity test enforces this behavior.

---

## Test Discovery

| Option | Description | Selected |
|--------|-------------|----------|
| get-diagnostics only | get-diagnostics discovers all test files, agent decides which to run. No special handling for Aider's hidden-test pattern. | |
| Run-tests verb | Run test commands (e.g., 'pytest') after edits. Harness doesn't know test files — agent discovers them. | |
| Both verbs | HARNESS-02a/b: get-diagnostics to find test files, but prompt instructs agent to check diagnostics after edits. Both work together. | ✓ |

**User's choice:** Both verbs
**Notes:** The agent uses get-diagnostics to find test files and run-tests to execute them. Both verbs are available; the agent discovers tests, the harness doesn't hardcode locations.

---

## Claude's Discretion

No areas deferred to Claude's discretion. All decisions were discussed and user-selected.

---

## Deferred Ideas

None — discussion stayed within phase scope. Verb-arg hardening (Phase 116), GEPA rebuild (Phase 117), and attribution re-run (Phase 118) are already in their respective phases.