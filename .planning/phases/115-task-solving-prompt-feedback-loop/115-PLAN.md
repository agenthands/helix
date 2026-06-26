---
phase: "115"
plan: "01"
type: tdd
wave: 1
depends_on: []
files_modified:
  - tools/dspy-tune/agent/react.py
  - tools/dspy-tune/test_agent.py
autonomous: true
requirements:
  - HARNESS-01a
  - HARNESS-01b
  - HARNESS-01c
  - HARNESS-01d
must_haves:
  truths:
    - Agent receives task description with solution file named or extractable
    - System prompt declares "success = hidden tests pass"
    - Agent does NOT answer in prose without tool calls
    - Anti-vacuity test asserts FAIL when agent returns prose-only answer
  artifacts:
    - tools/dspy-tune/agent/react.py (modified: _TASK_SOLVING_PROMPT added)
    - tools/dspy-tune/test_agent.py (modified: anti-vacuity test added)
  key_links:
    - build_system_prompt() injects task-solving section under STEERING_SENTINEL
    - ReActAgent.run() detects prose-only answers and triggers retry-nudge-or-fail
---

<objective>
Inject a task-solving system prompt that forces editing and forbids prose answers. The prompt names the solution file, declares "success = hidden tests pass", and instructs the agent to edit files, not answer in prose. An anti-vacuity test ensures prose-only answers are rejected.

Purpose: v2.4 investigation showed ~40% of tasks had 0 tool calls — the agent answered in prose and quit. This prompt makes editing the explicit goal.
Output: Modified react.py with task-solving prompt, anti-vacuity test in test_agent.py.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/115-task-solving-prompt-feedback-loop/115-CONTEXT.md

# Key source files (executor MUST read before implementing)
@tools/dspy-tune/agent/react.py
@tools/dspy-tune/test_agent.py
</context>

<tasks>

<task type="tdd">
<name>Task 1: Anti-vacuity test for prose refusal (HARNESS-01d)</name>
<files>tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: FakeLLM returns ONLY prose (no tool_calls) on turn 0
- Expected behavior: Agent either retries with nudge or terminates with reason != "done"
- The test asserts that a prose-only answer is NOT accepted as a successful completion
- Break-the-invariant: if the agent accepts prose without tool calls, test FAILS
</behavior>
<read_first>
<file>tools/dspy-tune/test_agent.py</file>
<file>tools/dspy-tune/agent/react.py</file>
</read_first>
<action>
Add a new test function `test_prose_only_answer_fails` to test_agent.py following the existing pattern (hermetic, FakeLLM, monkeypatch).

The test must:
1. Create a FakeLLM that returns a FakeAssistantMessage with `content="Here is the answer..."` and `tool_calls=None` on turn 0 (no tool calls, just prose)
2. Assert that the Transcript does NOT have `reason="done"` with the prose answer accepted
3. Either: (a) the agent injects a retry nudge and tries again, or (b) the agent terminates with reason="prose_refused" or similar failure reason
4. The test MUST fail if the agent silently accepts prose without tool calls

Per D-04: retry nudge then fail on repeat. The test should verify that prose-only triggers the nudge mechanism, and repeated prose triggers task failure.

Pattern: Follow existing test structure (test_loop_emits_and_observes, test_degenerate_always_grep_scores_zero). Use monkeypatch.delenv for API keys, FakeLLM with scripted messages, stub run_verb if needed.
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_prose_only_answer_fails -v</automated>
</verify>
<done>
Test exists, runs hermetically (no network, no API keys), and asserts FAIL when agent returns prose-only answer. The test proves prose is not accepted without tool calls.
</done>
</task>

<task type="tdd">
<name>Task 2: Task-solving system prompt (HARNESS-01a/b/c)</name>
<files>tools/dspy-tune/agent/react.py, tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: build_system_prompt() with steering="on" includes task-solving section
- Test: Prompt includes solution file instruction, success criterion, prose prohibition
- Expected: STEERING_SENTINEL marks the section; prompt content matches requirements
</behavior>
<read_first>
<file>tools/dspy-tune/agent/react.py</file>
</read_first>
<action>
Add `_TASK_SOLVING_PROMPT` constant to react.py with the task-solving instructions per D-01/D-02/D-03:

Content requirements (per CONTEXT.md decisions):
1. D-01: Solution file naming — prompt instructs agent to identify solution file from task context (explicit or extracted from description)
2. D-02: Success criterion — "You are done when hidden tests pass" as termination condition
3. D-03: Prose prohibition — "Do NOT answer in prose. You must edit files. You are not done until the code is implemented."
4. Instruct agent to use `get-diagnostics` to discover test files and compilation errors
5. Instruct agent to use `run-tests` (verb to be added in Plan 2) to verify work

Modify `build_system_prompt()` to include the task-solving section when steering="on":
- Place it after `_BASE_SYSTEM_PROMPT` and before/within the STEERING_SENTINEL section
- Keep OFF_CONTROL_PROMPT unchanged (independent constant, provably omits sentinel)

Add test `test_task_solving_prompt_included` that verifies:
1. ON prompt includes task-solving instructions
2. OFF prompt does NOT include task-solving instructions
3. Both solution file and success criterion are present in ON prompt

Implementation pattern: Add `_TASK_SOLVING_PROMPT` constant (similar to `_BASE_SYSTEM_PROMPT`), concatenate it in `build_system_prompt()` when steering="on".
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_task_solving_prompt_included -v</automated>
</verify>
<done>
Task-solving prompt exists, is injected into ON steering, OFF control omits it. Prompt contains solution file instruction, success criterion, prose prohibition.
</done>
</task>

<task type="tdd">
<name>Task 3: Prose retry-nudge mechanism (D-04)</name>
<files>tools/dspy-tune/agent/react.py, tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: Agent receives prose-only answer, injects nudge, LLM retries with tool call
- Test: Agent receives prose-only twice, terminates with failure reason
- Expected: One retry allowed, then task marked as failed
</behavior>
<read_first>
<file>tools/dspy-tune/agent/react.py</file>
</read_first>
<action>
Implement the retry-nudge mechanism in `ReActAgent.run()` per D-04:

1. After LLM responds with no tool calls (prose answer), detect this condition
2. Inject a follow-up prompt into the messages: "You must edit files, not answer in prose. Please make a tool call."
3. Allow ONE retry per prose turn
4. If the agent persists with prose after nudge, terminate with reason="prose_refused" (not "done")
5. The nudge message is appended to the messages list, then LLM is called again

Add test `test_prose_nudge_retries_then_fails`:
1. Turn 0: FakeLLM returns prose (no tool_calls)
2. Turn 1: After nudge, FakeLLM returns prose again (no tool_calls)
3. Assert: Transcript reason is NOT "done", final is None or error message
4. Assert: Messages list contains the nudge prompt

This ensures the anti-vacuity gate: prose-only answers are NOT silently accepted.
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_prose_nudge_retries_then_fails -v</automated>
</verify>
<done>
Agent detects prose-only answers, injects nudge, retries once, then fails on repeated prose. Tests verify the mechanism works hermetically.
</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| LLM → Agent | LLM output is untrusted; agent must validate tool calls |
| Agent → helix CLI | Subprocess output is untrusted; agent must handle errors |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-115-01 | Tampering | LLM output | low | accept | LLM may return prose; nudge mechanism handles gracefully |
| T-115-02 | Denial of Service | Agent loop | low | mitigate | max_turns cap prevents infinite retry loops |
</threat_model>

<verification>
- All tests pass hermetically (no API keys, no network)
- Anti-vacuity test `test_prose_only_answer_fails` fails when prose is silently accepted
- Task-solving prompt is present in ON steering, absent in OFF control
- `make vet` green, `go test ./...` green
</verification>

<success_criteria>
1. System prompt explicitly names solution file (D-01)
2. Success criterion "hidden tests pass" is declared (D-02)
3. Prose answers are forbidden (D-03)
4. Anti-vacuity test proves prose-only answers result in failure (HARNESS-01d)
5. All tests pass hermetically
6. No new Go deps (go.mod unchanged)
</success_criteria>

<output>
Create `.planning/phases/115-task-solving-prompt-feedback-loop/115-01-SUMMARY.md` when done.
</output>

---
---
phase: "115"
plan: "02"
type: tdd
wave: 2
depends_on: ["115-01"]
files_modified:
  - tools/dspy-tune/agent/react.py
  - tools/dspy-tune/agent/tools.py
  - tools/dspy-tune/test_agent.py
autonomous: true
requirements:
  - HARNESS-02a
  - HARNESS-02b
  - HARNESS-02c
  - HARNESS-02d
must_haves:
  truths:
    - Agent runs tests after editing files (run-tests verb exists)
    - Agent checks diagnostics to find compilation errors (get-diagnostics wired)
    - Agent uses test results to drive next edit iteration
    - Anti-vacuity test asserts FAIL when agent declares done on broken code without tests
  artifacts:
    - tools/dspy-tune/agent/tools.py (modified: run-tests verb added)
    - tools/dspy-tune/agent/react.py (modified: feedback loop logic)
    - tools/dspy-tune/test_agent.py (modified: feedback loop tests)
  key_links:
    - run-tests verb in _VERB_SPECS calls pytest/subprocess
    - ReAct loop observes test results and iterates
    - Agent declares done only after tests pass (not before)
</must_haves>

<objective>
Wire the feedback loop so the agent runs tests and uses results to iterate. Add run-tests verb to the agent's toolkit. The agent must verify work before declaring done. An anti-vacuity test ensures agents don't declare done on broken code.

Purpose: v2.4 investigation showed the agent declared "done" on broken code without running tests. This feedback loop closes the gap.
Output: run-tests verb in tools.py, feedback logic in react.py, anti-vacuity tests.
</objective>

<execution_context>
@$HOME/.claude/gsd-core/workflows/execute-plan.md
@$HOME/.claude/gsd-core/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/115-task-solving-prompt-feedback-loop/115-CONTEXT.md

# Key source files (executor MUST read before implementing)
@tools/dspy-tune/agent/react.py
@tools/dspy-tune/agent/tools.py
@tools/dspy-tune/test_agent.py
</context>

<tasks>

<task type="tdd">
<name>Task 1: Add run-tests verb to agent toolkit (HARNESS-02a)</name>
<files>tools/dspy-tune/agent/tools.py, tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: run-tests verb is in _VERB_SPECS
- Test: run_verb("run-tests", {"path": "test_file.py"}) executes pytest subprocess
- Test: VerbResult contains exit code, stdout from pytest
- Expected: Agent can call run-tests like other verbs
</behavior>
<read_first>
<file>tools/dspy-tune/agent/tools.py</file>
</read_first>
<action>
Add `run-tests` to `_VERB_SPECS` in tools.py:

```python
"run-tests": ("Run tests for a file or directory.", {
    "path": ("--path", "string", True),
    "extra_args": ("--extra-args", "string", False),  # optional pytest args
}),
```

Modify `run_verb()` to handle non-helix verbs:
1. If verb is "run-tests", call `_run_tests_verb()` instead of helix subprocess
2. Create `_run_tests_verb(call, cwd)` that:
   - Extracts `path` from arguments
   - Runs `["uv", "run", "pytest", path, "--tb=short", "-q"]` via subprocess
   - Returns `VerbResult(argv, exit, stdout, stderr)`
3. Keep all other verbs as helix subprocess calls

Add test `test_run_tests_verb_exists`:
1. Assert "run-tests" is in VERB_NAMES
2. Assert TOOL_SCHEMAS includes run-tests with expected schema

Add test `test_run_tests_calls_pytest` (hermetic):
1. Stub subprocess.run to capture the pytest call
2. Call run_verb with run-tests and a path
3. Assert the stubbed subprocess received ["uv", "run", "pytest", ...]
4. Assert VerbResult has exit code and stdout

Per D-05: Both verbs (get-diagnostics, run-tests) are available. get-diagnostics already exists. run-tests is new.
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_run_tests_verb_exists test_agent.py::test_run_tests_calls_pytest -v</automated>
</verify>
<done>
run-tests verb exists in _VERB_SPECS, callable by agent, executes pytest subprocess (hermetic test proves it).
</done>
</task>

<task type="tdd">
<name>Task 2: Wire get-diagnostics into ReAct loop (HARNESS-02b)</name>
<files>tools/dspy-tune/agent/react.py, tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: Agent can call get-diagnostics verb
- Test: Task-solving prompt instructs agent to check diagnostics
- Expected: get-diagnostics is already in _VERB_SPECS, prompt now mentions it
</behavior>
<read_first>
<file>tools/dspy-tune/agent/tools.py</file>
<file>tools/dspy-tune/agent/react.py</file>
</read_first>
<action>
Verify get-diagnostics is already in _VERB_SPECS (it is per line 47-49).

Update `_TASK_SOLVING_PROMPT` from Plan 1 to explicitly instruct:
- "Use `get-diagnostics` to find test files and compilation errors in your solution"
- "Use `run-tests` to execute tests and verify your work"
- "Iterate: edit, run tests, check results, repeat until tests pass"

Add test `test_get_diagnostics_in_tool_schemas`:
1. Assert "get-diagnostics" is in VERB_NAMES
2. Assert TOOL_SCHEMAS includes get-diagnostics

This is largely verification that existing infrastructure works; the prompt update is the key change.
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_get_diagnostics_in_tool_schemas -v</automated>
</verify>
<done>
get-diagnostics is available in agent toolkit, prompt instructs agent to use it for test discovery.
</done>
</task>

<task type="tdd">
<name>Task 3: Agent uses test results to iterate (HARNESS-02c)</name>
<files>tools/dspy-tune/agent/react.py, tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: Agent runs test, sees failure, makes edit, runs test again, sees pass
- Test: Agent uses test output to decide next action
- Expected: ReAct loop includes test-run-observe cycle
</behavior>
<read_first>
<file>tools/dspy-tune/agent/react.py</file>
</read_first>
<action>
The ReAct loop already observes tool results (lines 118-148 in react.py). The agent naturally:
1. Thinks (LLM call)
2. Acts (tool call: run-tests, get-diagnostics, edit verbs)
3. Observes (tool result with exit code and stdout)
4. Repeats

The task-solving prompt from Plan 1 already instructs the agent to iterate. The key verification is that test failures appear in the observation and the agent can act on them.

Add test `test_agent_observes_test_results`:
1. FakeLLM returns tool call for run-tests on turn 0
2. Stub run_verb returns a VerbResult with exit=1 (test failed) and stdout showing failure
3. Turn 1: FakeLLM returns tool call for edit verb
4. Stub run_verb returns exit=0 (edit succeeded)
5. Turn 2: FakeLLM returns tool call for run-tests again
6. Stub run_verb returns exit=0 (tests pass)
7. Turn 3: FakeLLM returns final answer
8. Assert: Transcript has 3 steps (run-tests, edit, run-tests), final reason="done"

This verifies the observe-test-result-and-iterate pattern works in the ReAct loop.
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_agent_observes_test_results -v</automated>
</verify>
<done>
Test verifies agent can run tests, observe results, edit, and re-run tests in the ReAct loop.
</done>
</task>

<task type="tdd">
<name>Task 4: Anti-vacuity test for done-on-broken (HARNESS-02d)</name>
<files>tools/dspy-tune/test_agent.py</files>
<behavior>
- Test: Agent declares done without running tests on code that is actually broken
- Test: Assert FAIL — agent should NOT accept "done" without test verification
- Expected: Test fails if agent can short-circuit to "done" on broken code
</behavior>
<read_first>
<file>tools/dspy-tune/agent/react.py</file>
<file>tools/dspy-tune/test_agent.py</file>
</read_first>
<action>
Add test `test_done_on_broken_without_tests_fails`:

The anti-vacuity gate: agent MUST run tests before declaring done. This test proves the gate works.

Pattern:
1. FakeLLM on turn 0: returns an edit verb (fuzzy-edit or replace-in-file)
2. Stub run_verb: returns exit=0 (edit "succeeded")
3. FakeLLM on turn 1: returns final answer "done: I fixed it" (NO test run)
4. Assert: This is NOT acceptable — the test should verify that the agent cannot declare done without test verification

Implementation approaches (choose based on prompt design):
a) The task-solving prompt instructs "run tests before done" — this test verifies the agent follows it
b) The ReAct loop detects early "done" without test run and injects nudge
c) The agent's final answer must mention test results

For HARNESS-02d, the test proves that simply claiming "done" after an edit (without running tests) triggers a nudge or failure.

If the prompt instruction is the enforcement mechanism (recommended for Phase 1):
- The test verifies that a compliant agent (following prompt) runs tests before done
- A non-compliant agent (ignoring prompt) would fail the task-solving evaluation

This is the break-the-invariant test: if the agent can short-circuit to "done" on broken code, the test FAILS.
</action>
<verify>
<automated>cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune && uv run pytest test_agent.py::test_done_on_broken_without_tests_fails -v</automated>
</verify>
<done>
Anti-vacuity test proves agent cannot declare done without running tests. The test fails if the agent short-circuits to done on broken code.
</done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| pytest subprocess | Test output is untrusted; agent parses stdout |
| Agent → test results | Agent must handle test failures gracefully |

## STRIDE Threat Register

| Threat ID | Category | Component | Severity | Disposition | Mitigation Plan |
|-----------|----------|-----------|----------|-------------|-----------------|
| T-115-03 | Tampering | Test output | low | accept | Agent observes raw pytest output; no privileged interpretation |
| T-115-04 | Denial of Service | Test runner | low | mitigate | Subprocess timeout prevents hanging tests |
</threat_model>

<verification>
- run-tests verb exists and calls pytest
- get-diagnostics is available for test discovery
- Agent observes test results and iterates in ReAct loop
- Anti-vacuity test proves done-on-broken is rejected
- All tests pass hermetically
- `make vet` green, `go test ./...` green
</verification>

<success_criteria>
1. run-tests verb callable by agent (HARNESS-02a)
2. get-diagnostics available for test discovery (HARNESS-02b)
3. Agent iterates based on test results (HARNESS-02c)
4. Anti-vacuity test proves done-on-broken fails (HARNESS-02d)
5. All tests pass hermetically
6. No new Go deps
</success_criteria>

<output>
Create `.planning/phases/115-task-solving-prompt-feedback-loop/115-02-SUMMARY.md` when done.
</output>

<artifacts_produced>
## Artifacts this phase produces

### Code artifacts
- `tools/dspy-tune/agent/react.py`:
  - `_TASK_SOLVING_PROMPT`: constant with task-solving instructions
  - `build_system_prompt()`: modified to inject task-solving section
  - `ReActAgent.run()`: modified with prose retry-nudge mechanism
- `tools/dspy-tune/agent/tools.py`:
  - `_VERB_SPECS`: added `run-tests` entry
  - `run_verb()`: modified to handle non-helix verbs (run-tests → pytest)
  - `_run_tests_verb()`: new helper function for pytest execution
- `tools/dspy-tune/test_agent.py`:
  - `test_prose_only_answer_fails`: anti-vacuity test for HARNESS-01d
  - `test_task_solving_prompt_included`: verifies prompt injection
  - `test_prose_nudge_retries_then_fails`: verifies retry mechanism
  - `test_run_tests_verb_exists`: verifies run-tests in toolkit
  - `test_run_tests_calls_pytest`: verifies pytest execution
  - `test_get_diagnostics_in_tool_schemas`: verifies diagnostics available
  - `test_agent_observes_test_results`: verifies iterate-on-test-results
  - `test_done_on_broken_without_tests_fails`: anti-vacuity test for HARNESS-02d

### Behavioral changes
- Agent with steering="on" receives task-solving prompt that:
  - Names solution file (explicit or extracted)
  - Declares success = hidden tests pass
  - Forbids prose answers
  - Instructs to use get-diagnostics and run-tests
- Agent cannot short-circuit to "done" with prose-only answer
- Agent can run tests via run-tests verb and observe results
- Agent iterates based on test feedback
</artifacts_produced>