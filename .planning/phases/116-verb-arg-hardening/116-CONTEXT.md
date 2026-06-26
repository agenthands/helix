# Phase 116: Verb-Arg Hardening - Context

**Gathered:** 2026-06-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Stop burning error budget on malformed calls. This phase hardens the verb interface to ensure agent errors are budgeted and bounded, so the agent can't waste turns on repeated failures. HARNESS-03 is the scope. Task-solving prompt/feedback loop (HARNESS-01/02, Phase 115) are explicit prerequisites. GEPA module rebuild (HARNESS-04, Phase 117) is out of scope.

**In scope:**
- Error-budget tracking verification (budget exists, confirm it works)
- Anti-vacuity test: budget exhaustion aborts correctly
- Audit of verb call sites (the v2.4 positional→flags fix is verified)
- Zero new Go deps (dev-venv Python only)

**Out of scope:**
- Task-solving prompt (Phase 115, already complete)
- Feedback loop (Phase 115, already complete)
- GEPA module rebuild (Phase 117)
- Attribution re-run (Phase 118)

</domain>

<decisions>
## Implementation Decisions

### Budget Scope (D-01)

- **D-01:** Budget is fixed at `_TOOL_ERROR_BUDGET = 5` (constant, not configurable). The value 5 matches the existing implementation and is appropriate for the ~12-turn `max_turns` cap. No scaling or per-task configuration needed — the agent should learn from errors, not get infinite retries.

### Reset Behavior (D-02)

- **D-02:** Errors are strictly cumulative across all turns. No reset on successful tool calls. If an agent makes 4 errors then succeeds, it still has only 1 error of budget remaining. This enforces discipline: every error counts, and the agent must recover without burning additional attempts.

### What Counts Toward Budget (D-03)

- **D-03:** Only `exit != 0` counts toward the budget. Timeout is a separate bound (handled by `subprocess.run(timeout=...)` raising `TimeoutExpired`). stderr warnings do NOT count — they're informational. The budget tracks hard failures only, matching the subprocess contract.

### Validation Approach (D-04)

- **D-04:** Post-hoc counting only. Trust helix to reject malformed calls with `exit != 0`, then count those failures toward the budget. No pre-call schema validation — that would add complexity and duplicate what helix already does. The anti-vacuity test (HARNESS-03c) verifies the budget abort works correctly.

### Anti-Vacuity Test Shape (D-05)

- **D-05:** The test verifies that when the error budget is exhausted, the agent aborts with `reason="tool_error_budget"`. Test uses a mock LLM that returns verb calls which consistently fail (e.g., nonexistent file paths), and asserts that after N failures the transcript terminates with the correct reason. This is a break-the-invariant test: if budget tracking silently stopped working, the test would FAIL (agent would continue past 5 errors).

### Verb Call Site Audit (D-06)

- **D-06:** The v2.4 fix (positional→flags) is verified as correct. `_VERB_SPECS` defines the flag surface, `_argv_for()` builds `--flag value` pairs, and `run_verb()` uses fixed-argv `subprocess.run([...])`. No shell, no string concatenation. The audit confirms no positional-arg paths exist in the call chain.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Harness Files
- `tools/dspy-tune/agent/react.py` — ReActAgent class, `_TOOL_ERROR_BUDGET`, budget tracking loop, termination reasons
- `tools/dspy-tune/agent/tools.py` — `_VERB_SPECS` dictionary, `_argv_for()` argv builder, `run_verb()` subprocess shim, fixed-argv security

### Requirements and Roadmap
- `.planning/REQUIREMENTS.md` — HARNESS-03a/b/c requirements
- `.planning/ROADMAP.md` — Phase 116 success criteria and exit gate
- `.planning/.continue-here.md` — Root-cause investigation: verb errors burn budget
- `.planning/phases/115-*/115-CONTEXT.md` — Prerequisite phase context (task-solving prompt + feedback loop)

### Test Infrastructure
- `tools/dspy-tune/conftest.py` — Test fixtures for anti-vacuity tests
- `tools/dspy-tune/test_agent.py` — Existing agent tests, patterns for budget exhaustion test

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`_TOOL_ERROR_BUDGET = 5`:** The budget constant already exists. Phase 116 verifies it works, not adds it.
- **`tool_errors` counter:** The `ReActAgent.run()` loop already increments `tool_errors` on `result.exit != 0`.
- **Budget enforcement:** The loop already aborts on `tool_errors >= _TOOL_ERROR_BUDGET` with `reason="tool_error_budget"`.
- **`_VERB_SPECS` schema:** The curated verb list already defines the correct flag surface for each verb.
- **`_argv_for()` fixed argv:** The argv builder already produces `--flag value` pairs, never shell commands.

### Established Patterns
- **Anti-vacuity test pattern:** Break-the-invariant tests (agent does X → assert FAIL) are the established pattern from v2.2/v2.3. Follow this pattern for HARNESS-03c.
- **Post-hoc counting:** The budget counts failures after they happen, not before. This matches the subprocess contract.
- **Strictly cumulative:** No reset on success — every error counts toward the lifetime budget.

### Integration Points
- **`react.py:run()` around line 85-95:** The budget check happens in the main loop after each tool call. The abort sets `reason="tool_error_budget"`.
- **`tools.py:run_verb()`:** The subprocess call returns `VerbResult(exit=...)` which the loop uses to count errors.
- **Test location:** New test goes in `tools/dspy-tune/test_agent.py` following the existing test patterns.

</code_context>

<specifics>
## Specific Ideas

- The user confirmed the budget should stay fixed at 5 (not configurable).
- The user confirmed errors are strictly cumulative (no reset on success).
- The user confirmed only `exit != 0` counts toward budget (no timeout/stderr).
- The user confirmed post-hoc counting only (no pre-call validation).
- The user confirmed the anti-vacuity test should verify budget exhaustion aborts correctly.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. HARNESS-04 (GEPA module) and HARNESS-05 (attribution re-run) are in their respective phases (117, 118).

</deferred>

---

*Phase: 116-Verb-Arg-Hardening*
*Context gathered: 2026-06-26*