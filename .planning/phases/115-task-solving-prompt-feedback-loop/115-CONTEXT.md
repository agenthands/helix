# Phase 115: Task-Solving Prompt + Feedback Loop - Context

**Gathered:** 2026-06-26
**Status:** Ready for planning

<domain>
## Phase Boundary

Make the agent actually edit files and verify success — the foundation for all tuning. This phase fixes the core harness so the agent uses tools instead of answering in prose, runs tests to verify work, and iterates based on results. HARNESS-01 (task-solving prompt) and HARNESS-02 (feedback loop) are the scope. Verb-arg hardening (HARNESS-03) and GEPA module (HARNESS-04) are explicit out-of-scope dependencies (Phase 116 and Phase 117 respectively).

**In scope:**
- System prompt that forces editing (names solution file, declares success criterion, forbids prose)
- Feedback loop wiring (run-tests / get-diagnostics in ReAct loop)
- Anti-vacuity tests for both (prose → FAIL, done-on-broken → FAIL)
- Tool-call rate ≥ 80% demonstrated on held-out split

**Out of scope:**
- Verb-arg hardening (Phase 116)
- GEPA module rebuild (Phase 117)
- Attribution re-run (Phase 118)
- Corpus changes (already past val_size>50 gate)
- SKILL.md adoption (TUNE-FUT-03, separate human-gated step)

</domain>

<decisions>
## Implementation Decisions

### Solution File Conveyance

- **D-01:** Both paths for solution file naming. When the task explicitly names a file to edit, the prompt receives it as context. When the task description implies the file but doesn't name it explicitly, the prompt instructs the agent to extract and identify the solution file from the task description before editing. Fallback extraction keeps the prompt robust across corpus variations.

### Success Criterion Declaration

- **D-02:** Both discovery paths for success criterion. The system prompt declares "you are done when hidden tests pass" as the termination condition. The agent discovers test files via `get-diagnostics` (finding test files in the project) and validates work by running tests. This dual approach ensures the agent knows the goal AND can find the tests.

### Feedback Loop Integration

- **D-03:** Hybrid approach for feedback loop. The agent explicitly calls `run-tests` / `get-diagnostics` as tool calls (agent-driven by default). The system prompt instructs the agent to check diagnostics after edits. The anti-vacuity test (HARNESS-02d) enforces that the agent actually runs tests before declaring done — a test that asserts FAIL when the agent declares done on broken code without running tests.

### Prose Enforcement

- **D-04:** Retry nudge then fail on repeat. If the agent returns text without tool calls (prose answer), the harness injects a follow-up prompt reminding it to edit files. One retry allowed per turn. If the agent persists with prose, the task is marked as failed. The anti-vacuity test (HARNESS-01d) catches this by asserting that a prose-only answer results in FAIL.

### Test Discovery

- **D-05:** Both verbs for test discovery. `get-diagnostics` discovers test files and compilation errors. `run-tests` executes test commands. The agent uses both: first discover what tests exist, then run them to verify work. The harness does NOT hardcode test file locations — the agent discovers them.

### Claude's Discretion

No areas where the user said "you decide" — all key decisions were discussed and captured above.

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Core Harness Files
- `tools/dspy-tune/agent/react.py` — Current ReAct loop implementation, `ReActAgent` class, `_BASE_SYSTEM_PROMPT`, termination conditions
- `tools/dspy-tune/agent/tools.py` — Verb interface, `_VERB_SPECS` (curated verb list), `run_verb()` subprocess shim
- `tools/dspy-tune/optimize.py` — GEPA optimizer context (to be rebuilt in Phase 117, not modified this phase)

### Requirements and Roadmap
- `.planning/REQUIREMENTS.md` — HARNESS-01a/b/c/d, HARNESS-02a/b/c/d requirements
- `.planning/ROADMAP.md` — Phase 115 success criteria and exit gate
- `.planning/.continue-here.md` — Root-cause investigation: 0 tool calls on ~40% of tasks, no feedback loop, wrong steering artifact

### Test Infrastructure
- `tools/dspy-tune/conftest.py` — Test fixtures for anti-vacuity tests
- `tools/dspy-tune/test_agent.py` — Existing agent tests, patterns for test structure

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- **`ReActAgent` bounds:** `max_turns`, `no_progress`, `tool_error_budget` — all three bounds are already implemented and work. The loop correctly terminates on these conditions.
- **`_VERB_SPECS` curated verbs:** The verb interface already exists with proper `--flags` pattern (fixed in v2.4). No verb interface changes needed this phase.
- **`run_verb()` subprocess:** Already returns exit code and stdout — sufficient for test execution feedback.
- **`Transcript` dataclass:** Already captures steps, reason, final answer — can be extended for anti-vacuity tracking.

### Established Patterns
- **ReAct loop pattern:** Think (LLM) → Act (tool call) → Observe (tool result) → repeat. The loop is already correct; what's missing is prompt content and test integration.
- **Steering sentinel:** `STEERING_SENTINEL` pattern (<<HELIX_STEERING>>) already separates ON vs OFF control. This can be extended to mark the task-solving prompt section.
- **Anti-vacuity test pattern:** Break-the-invariant tests (agent does X → assert FAIL) are the established pattern from v2.2/v2.3. Follow this pattern for HARNESS-01d and HARNESS-02d.

### Integration Points
- **`react.py:build_system_prompt()`** — Needs new task-solving prompt section added to `_BASE_SYSTEM_PROMPT` or as a steering overlay.
- **`react.py:run()`** — May need injection point for prose retry nudge (after line 111 where `reason="done"`).
- **`tools.py:_VERB_SPECS`** — Already has `get-diagnostics` verb; verify `run-tests` verb exists or add it.

</code_context>

<specifics>
## Specific Ideas

- The user confirmed the prompt should explicitly name the solution file when available, with extraction fallback for implied files.
- The user confirmed the success criterion "hidden tests must pass" should be declared in the prompt, AND the agent should discover tests via get-diagnostics.
- The user confirmed agent-driven test execution (both verbs: get-diagnostics, run-tests) with prompt instruction, not automatic injection.
- The user confirmed prose enforcement: retry nudge first, then fail task if agent persists.

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope. All scope-adjacent ideas (verb hardening, GEPA rebuild, attribution re-run) are already in their respective phases (116, 117, 118).

</deferred>

---

*Phase: 115-Task-Solving-Prompt-Feedback-Loop*
*Context gathered: 2026-06-26*