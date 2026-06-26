# Phase 116: Verb-Arg Hardening - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-06-26
**Phase:** 116-verb-arg-hardening
**Areas discussed:** Budget mechanics, validation approach, test shape

---

## Budget Scope

| Option | Description | Selected |
|--------|-------------|----------|
| Fixed at 5 (current) | Budget is a constant in the agent code, same for all tasks. Simple and predictable. | ✓ |
| Per-task scaling (5% of max_turns) | Budget scales with task complexity. Larger tasks get more error tolerance. | |
| Configurable parameter | Budget is an argument to run() with a default. Callers can override per-corpus or per-experiment. | |

**User's choice:** Fixed at 5 (current)
**Notes:** The value 5 matches the existing `_TOOL_ERROR_BUDGET` constant. It's appropriate for the ~12-turn `max_turns` cap. No scaling or per-task configuration needed — the agent should learn from errors, not get infinite retries.

---

## Reset Behavior

| Option | Description | Selected |
|--------|-------------|----------|
| Strictly cumulative (Recommended) | All errors count toward the budget. No forgiveness. Simple and predictable. | ✓ |
| Reset on success | If the agent makes a successful tool call after errors, reset the counter. Forgives transient issues. | |
| Sliding window (N consecutive) | Allow N consecutive errors before abort, but reset the counter on any success. Tolerates bursts of failures. | |

**User's choice:** Strictly cumulative (Recommended)
**Notes:** If an agent makes 4 errors then succeeds, it still has only 1 error of budget remaining. This enforces discipline: every error counts, and the agent must recover without burning additional attempts.

---

## What Counts Toward Budget

| Option | Description | Selected |
|--------|-------------|----------|
| Exit code only (current) | Only `exit != 0` counts. stderr is informational. Simple and matches subprocess contract. | ✓ |
| Exit + stderr non-empty | Both `exit != 0` AND non-empty stderr count. Catches commands that print warnings but succeed. | |
| Exit code + timeout separately | Exit for hard failures, timeout (separate from budget) for hung verbs. Budget only counts hard failures. | |

**User's choice:** Exit code only (current)
**Notes:** Timeout is handled separately by `subprocess.run(timeout=...)` raising `TimeoutExpired`. stderr warnings do NOT count — they're informational. The budget tracks hard failures only.

---

## Validation Approach

| Option | Description | Selected |
|--------|-------------|----------|
| Post-hoc counting only (Recommended) | Trust helix to reject bad calls. Agent sees the error and can retry. Simpler implementation, matches current behavior. | ✓ |
| Pre-call schema validation | Add a validation layer that checks args against `_VERB_SPECS` before subprocess spawn. Catches malformed calls earlier but adds complexity. | |

**User's choice:** Post-hoc counting only (Recommended)
**Notes:** No pre-call schema validation — that would add complexity and duplicate what helix already does. The anti-vacuity test (HARNESS-03c) verifies the budget abort works correctly.

---

## Anti-Vacuity Test Shape

| Option | Description | Selected |
|--------|-------------|----------|
| Budget exhaustion abort (Recommended) | Test that the agent aborts when budget is exhausted. Use a mock LLM that returns verb calls that fail, verify abort reason is "tool_error_budget". | ✓ |
| Malformed args fail the task | Test that a test agent with malformed args (wrong flags, missing required args) hits budget and fails the task. | |
| Both rejection and exhaustion | Test both: malformed args are rejected, AND budget exhaustion aborts correctly. | |

**User's choice:** Budget exhaustion abort (Recommended)
**Notes:** The test verifies that when the error budget is exhausted, the agent aborts with `reason="tool_error_budget"`. This is a break-the-invariant test: if budget tracking silently stopped working, the test would FAIL (agent would continue past 5 errors).

---

## Claude's Discretion

No areas where the user said "you decide" — all key decisions were discussed and captured above.

---

## Deferred Ideas

None — discussion stayed within phase scope. All scope-adjacent ideas (GEPA rebuild, attribution re-run) are already in their respective phases (117, 118).