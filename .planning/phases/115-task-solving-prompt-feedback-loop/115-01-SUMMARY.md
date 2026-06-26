---
phase: "115"
plan: "01"
type: tdd
wave: 1
depends_on: []
files_modified:
  - tools/dspy-tune/agent/react.py
  - tools/dspy-tune/agent/tools.py
  - tools/dspy-tune/test_agent.py
autonomous: true
requirements:
  - HARNESS-01a
  - HARNESS-01b
  - HARNESS-01c
  - HARNESS-01d
  - HARNESS-02a
  - HARNESS-02b
  - HARNESS-02c
  - HARNESS-02d
status: complete
---

# Phase 115-01 Summary: Task-Solving Prompt + Feedback Loop

## Objective

Make the agent actually edit files and verify success — the foundation for all tuning. This phase implements HARNESS-01 and HARNESS-02 requirements to fix the core harness.

## What Was Built

### HARNESS-01: Task-Solving System Prompt

1. **`_TASK_SOLVING_PROMPT` constant** (`react.py`):
   - Declares solution file naming (D-01)
   - Success criterion: "hidden tests pass" (D-02)
   - Prose prohibition: "Do NOT answer in prose" (D-03)
   - Test discovery instructions: use `get-diagnostics` and `run-tests`
   - Iteration instruction: edit, run tests, repeat

2. **Modified `build_system_prompt()`** (`react.py`):
   - ON steering: includes `_TASK_SOLVING_PROMPT` under `STEERING_SENTINEL`
   - OFF steering: provably omits both the task-solving prompt AND the sentinel

3. **Prose retry-nudge mechanism** (`react.py`, `ReActAgent.run()`):
   - Detects prose-only answers (no tool calls when `steps` is empty)
   - Injects `_PROSE_NUDGE_PROMPT` on first prose-only turn
   - On second consecutive prose: terminates with `reason="prose_refused"`
   - Legitimate final answers (after tool calls) are accepted normally

### HARNESS-02: Feedback Loop with Test Verification

1. **`run-tests` verb** (`tools.py`):
   - Added to `_VERB_SPECS` with `path` (required) and `extra_args` (optional)
   - `run_verb()` dispatches `run-tests` to `_run_tests_verb()` which calls `uv run pytest` directly
   - Returns `VerbResult` with exit code and stdout

2. **`get-diagnostics` already exists** (`tools.py`):
   - Already in `_VERB_SPECS` from Phase 107
   - Agent can discover test files and compilation errors

3. **Test result iteration** (`react.py`):
   - ReAct loop naturally observes tool results
   - Agent runs tests, sees failures, edits, re-runs tests

## Anti-Vacuity Tests

All 8 new tests pass hermetically (no API keys, no network):

| Test | Requirement | What it proves |
|------|-------------|-----------------|
| `test_prose_only_answer_fails` | HARNESS-01d | Prose-only answers are rejected |
| `test_task_solving_prompt_included` | HARNESS-01a/b/c | ON prompt has task-solving, OFF omits it |
| `test_prose_nudge_retries_then_fails` | D-04 | Retry-nudge mechanism works |
| `test_run_tests_verb_exists` | HARNESS-02a | run-tests is in toolkit |
| `test_run_tests_calls_pytest` | HARNESS-02a | run-tests calls pytest subprocess |
| `test_get_diagnostics_in_tool_schemas` | HARNESS-02b | get-diagnostics available |
| `test_agent_observes_test_results` | HARNESS-02c | Agent iterates on test results |
| `test_done_on_broken_without_tests_fails` | HARNESS-02d | Cannot declare done without tests (anti-vacuity) |

## Behavioral Changes

1. **Agent with steering="on"** receives task-solving prompt that:
   - Names solution file (explicit or extracted)
   - Declares success = hidden tests pass
   - Forbids prose answers
   - Instructs to use `get-diagnostics` and `run-tests`

2. **Agent cannot short-circuit to "done" with prose-only**:
   - First prose: inject nudge
   - Second prose: terminate with `prose_refused`

3. **Agent can run tests** via `run-tests` verb and observe results

4. **Agent can discover tests** via `get-diagnostics`

## Files Modified

- `tools/dspy-tune/agent/react.py`:
  - Added `_TASK_SOLVING_PROMPT` constant
  - Added `_PROSE_NUDGE_PROMPT` constant
  - Modified `build_system_prompt()` to inject task-solving prompt when steering="on"
  - Modified `ReActAgent.run()` with prose retry-nudge mechanism
  - Added `prose_refused` to `Transcript.reason` enum

- `tools/dspy-tune/agent/tools.py`:
  - Added `run-tests` to `_VERB_SPECS`
  - Added `_run_tests_verb()` helper for pytest execution
  - Modified `run_verb()` to dispatch `run-tests` to pytest instead of helix

- `tools/dspy-tune/test_agent.py`:
  - Added `test_prose_only_answer_fails` (HARNESS-01d)
  - Added `test_task_solving_prompt_included` (HARNESS-01a/b/c)
  - Added `test_prose_nudge_retries_then_fails` (D-04)
  - Added `test_run_tests_verb_exists` (HARNESS-02a)
  - Added `test_run_tests_calls_pytest` (HARNESS-02a)
  - Added `test_get_diagnostics_in_tool_schemas` (HARNESS-02b)
  - Added `test_agent_observes_test_results` (HARNESS-02c)
  - Added `test_done_on_broken_without_tests_fails` (HARNESS-02d)

## Verification

```bash
cd tools/dspy-tune && uv run pytest test_agent.py -v
# 15 passed (7 pre-existing + 8 new)
```

```bash
cd /home/john/go/src/github.com/agenthands/helix && make vet
# OK (no new warnings)

cd /home/john/go/src/github.com/agenthands/helix && go test ./...
# FAIL: TestCLI_ActivateEnablesFileVerb (pre-existing daemon lock issue)
# FAIL: cmd/helix-bench (external huggingface 404 - pre-existing)
# All Python tests pass
```

## Decisions

- **D-01**: Solution file naming — prompt instructs agent to identify from task context
- **D-02**: Success criterion — "hidden tests pass" declared in prompt
- **D-03**: Prose prohibition — explicit instruction not to answer in prose
- **D-04**: Retry nudge — one retry allowed, then fail on repeated prose
- **D-05**: Both verbs — `get-diagnostics` and `run-tests` available for test discovery

## Constraints Verified

- ✅ All tests pass hermetically (no API keys, no network)
- ✅ `make vet` green (no new warnings)
- ✅ No new Go dependencies (`go.mod` unchanged)
- ✅ Python-only changes (`tools/dspy-tune/`)

## Next Steps

Phase 116 (Verb-Arg Hardening) can now proceed:
- HARNESS-03a: Audit verb call sites for positional-arg misuse
- HARNESS-03b: Add error-budget tracking
- HARNESS-03c: Anti-vacuity test for malformed argv