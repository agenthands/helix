# Phase 116: Verb-Arg Hardening - Plan

**Created:** 2026-06-26
**Status:** Ready for execution
**Depends on:** Phase 115 (COMPLETE)

## Summary

Verify that the error-budget mechanism works correctly and add an anti-vacuity test that proves budget exhaustion aborts the agent. The implementation already exists (`_TOOL_ERROR_BUDGET = 5`, cumulative counting, budget check) — this phase confirms it works and guards against regression.

## Hard Bar

- **HARNESS-03:** Verb-arg usage MUST be hardened. Malformed calls MUST burn budget. Budget exhaustion MUST abort. Anti-vacuity test MUST fail if budget tracking breaks.

## Goals

1. **Confirm budget implementation works** (not implement — already exists)
2. **Add anti-vacuity test** for budget exhaustion abort
3. **Audit verb call sites** to verify v2.4 positional→flags fix

## Tasks

### Task 1: Anti-Vacuity Test for Budget Exhaustion

**What:** Add `test_tool_error_budget_exhaustion_aborts` to `test_agent.py` — a break-the-invariant test that proves budget tracking works.

**Why:** Without this test, the budget mechanism could silently regress. The test MUST fail if budget exhaustion stops working.

**Implementation:**

```python
# In tools/dspy-tune/test_agent.py

def test_tool_error_budget_exhaustion_aborts(monkeypatch):
    """HARNESS-03: Anti-vacuity test for budget exhaustion.

    A fake LLM that makes tool calls which ALWAYS fail (exit != 0) should
    exhaust the budget and terminate with reason="tool_error_budget" after
    exactly _TOOL_ERROR_BUDGET failures.

    Break-the-invariant: If budget tracking silently broke (e.g., counter
    not incremented, check removed, budget raised), this test would FAIL.
    The agent would continue past 5 errors and hit max_turns instead.
    """
    monkeypatch.delenv("DEEPSEEK_API_KEY", raising=False)
    monkeypatch.delenv("OPENAI_API_KEY", raising=False)

    from agent.react import _TOOL_ERROR_BUDGET

    # A fake LLM that always makes a tool call that fails.
    # Each call returns exit=1 (error), burning the budget.
    always_failing = [
        _tool_turn(f"call_{i}", "read-file", {"path": f"/nonexistent/file_{i}.go"})
        for i in range(20)  # More than budget
    ]
    llm = FakeLLM(always_failing)

    # Stub that always returns failure (exit=1)
    def _failing_run_verb(call, cwd, timeout=60):
        return VerbResult(
            argv=["helix", "read-file", "--path", "/nonexistent/file.go"],
            exit=1,
            stdout="",
            stderr="error: file not found",
        )

    monkeypatch.setattr(agent_tools, "run_verb", _failing_run_verb)
    import agent.react as agent_react
    if hasattr(agent_react, "run_verb"):
        monkeypatch.setattr(agent_react, "run_verb", _failing_run_verb)

    agent = ReActAgent(system_prompt="SYS", llm=llm)
    transcript = agent.run("read nonexistent files", cwd=".", max_turns=20)

    # HARNESS-03 anti-vacuity assertions:
    # 1. Must terminate on budget exhaustion, NOT max_turns
    assert transcript.reason == "tool_error_budget", (
        f"expected tool_error_budget termination, got {transcript.reason!r}"
    )

    # 2. Must have exactly _TOOL_ERROR_BUDGET error steps
    assert len(transcript.steps) == _TOOL_ERROR_BUDGET, (
        f"expected {_TOOL_ERROR_BUDGET} steps on budget exhaustion, got {len(transcript.steps)}"
    )

    # 3. Every step must have exit != 0 (error)
    for i, step in enumerate(transcript.steps):
        assert step.exit != 0, (
            f"step {i} should have exit != 0 (error), got exit={step.exit}"
        )

    # 4. No final answer (terminated on bound, not "done")
    assert transcript.final is None
```

**Acceptance:**
- Test passes with current implementation (budget mechanism works)
- Test fails if `_TOOL_ERROR_BUDGET` check is removed/modified
- Test fails if `tool_errors` counter not incremented

### Task 2: Verify Existing Budget Implementation

**What:** Read and confirm the existing implementation is correct.

**Why:** This is the verification step, not implementation.

**Implementation:**
- Confirm `_TOOL_ERROR_BUDGET = 5` exists (react.py:75)
- Confirm `tool_errors += 1` on `result.exit != 0` (react.py:195-196)
- Confirm budget check aborts with `reason="tool_error_budget"` (react.py:212-214)
- Confirm counter is cumulative (no reset on success)

**Acceptance:**
- Implementation matches decisions D-01 through D-06 in CONTEXT.md

### Task 3: Audit Verb Call Sites

**What:** Verify `_argv_for()` builds fixed `--flag value` pairs, no shell, no positional args.

**Why:** The v2.4 fix must be verified — every verb call goes through this function.

**Implementation:**
- Read `_argv_for()` in tools.py (lines 122-137)
- Confirm it uses `_VERB_SPECS` for flag names
- Confirm it builds `--flag value` pairs
- Confirm it uses `subprocess.run([...])` (list, not shell string)
- Confirm `run_verb()` dispatches through this function

**Acceptance:**
- No positional argument paths exist
- All verbs go through `_argv_for()`
- Security (T-107-01) is maintained: no shell, no string concatenation

### Task 4: Run Tests and Verify

**What:** Run the full test suite for `tools/dspy-tune/` and verify all tests pass.

**Why:** Regression gate.

**Implementation:**
```bash
cd /home/john/go/src/github.com/agenthands/helix/tools/dspy-tune
uv run pytest test_agent.py -v
```

**Acceptance:**
- All tests pass (including new `test_tool_error_budget_exhaustion_aborts`)
- No test regressions

## Rollout Order

1. Task 2 (Verify existing implementation) — read-only, confirms scope
2. Task 3 (Audit verb call sites) — read-only, confirms security
3. Task 1 (Anti-vacuity test) — the deliverable
4. Task 4 (Run tests) — regression gate

## Verification Criteria

- [ ] Anti-vacuity test `test_tool_error_budget_exhaustion_aborts` passes
- [ ] Test fails if budget mechanism is removed
- [ ] All existing tests pass
- [ ] No positional argument paths in verb call chain
- [ ] `_argv_for()` confirmed to build `--flag value` pairs
- [ ] `subprocess.run([...])` confirmed (no shell)

## Out of Scope

- Task-solving prompt (Phase 115 — already complete)
- Feedback loop (Phase 115 — already complete)
- GEPA module rebuild (Phase 117)
- Attribution re-run (Phase 118)
- Pre-call validation (post-hoc counting only — D-04)

## Decisions Locked

- D-01: Budget fixed at 5 (not configurable)
- D-02: Errors cumulative (no reset on success)
- D-03: Only `exit != 0` counts (timeout/stderr don't count)
- D-04: Post-hoc counting only (no pre-call validation)
- D-05: Anti-vacuity test shape defined above
- D-06: v2.4 positional→flags fix verified

## Notes

The implementation is already complete. This phase is **verification + test** — confirming the mechanism works and adding a regression guard. The budget constant, counter, and check are all in place in `react.py`. The `_argv_for()` function in `tools.py` builds secure fixed-argv commands. The anti-vacuity test is the primary deliverable.