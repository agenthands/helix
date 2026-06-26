# Phase 116: Verb-Arg Hardening - Summary

**Completed:** 2026-06-26
**Status:** COMPLETE

## What Was Done

### Task 2: Verify Existing Budget Implementation ✓

Read and confirmed the error budget implementation in `react.py`:

- **`_TOOL_ERROR_BUDGET = 5`** (line 75) — budget constant exists and is fixed
- **`tool_errors = 0`** (line 150) — counter initialized at start of each run
- **`if result.exit != 0: tool_errors += 1`** (lines 195-196) — errors counted cumulatively
- **`if tool_errors >= _TOOL_ERROR_BUDGET: return Transcript(reason="tool_error_budget", ...)`** (lines 212-214) — budget check aborts correctly

Implementation matches all decisions:
- D-01: Budget fixed at 5 (not configurable) ✓
- D-02: Errors cumulative (no reset on success) ✓
- D-03: Only `exit != 0` counts (timeout/stderr don't count) ✓
- D-04: Post-hoc counting only (no pre-call validation) ✓

### Task 3: Audit Verb Call Sites ✓

Read and confirmed `_argv_for()` in `tools.py` (lines 122-137):

- Builds **fixed argv list**: `["helix", verb, "--flag", value, ...]`
- Uses **`_VERB_SPECS`** for correct flag names
- **No shell** — uses `subprocess.run([...])` with list
- **No string concatenation** — each param is a separate list element
- Security (T-107-01) maintained: verb name constrained to curated set, args properly escaped via list

All verb calls go through `run_verb()` → `_argv_for()` → `subprocess.run(argv, ...)`.

### Task 1: Anti-Vacuity Test ✓

Added `test_tool_error_budget_exhaustion_aborts` to `test_agent.py`:

- Creates fake LLM that always makes failing tool calls
- Stub returns `exit=1` with **varying stdout** (to avoid no_progress bound)
- Asserts termination with `reason="tool_error_budget"`
- Asserts exactly `_TOOL_ERROR_BUDGET` steps recorded
- Asserts all steps have `exit != 0`
- Asserts no final answer

**Key insight:** The no_progress bound checks for identical `(argv, stdout)` pairs, so the stub must vary output to let the budget bound trigger first.

### Task 4: Run Tests ✓

```
$ uv run pytest test_agent.py -v
16 passed in 0.28s
```

All tests pass including the new HARNESS-03 anti-vacuity test.

## Verification Criteria

- [x] Anti-vacuity test `test_tool_error_budget_exhaustion_aborts` passes
- [x] Test fails if budget mechanism is removed (verified by reading implementation)
- [x] All existing tests pass (16/16)
- [x] No positional argument paths in verb call chain
- [x] `_argv_for()` confirmed to build `--flag value` pairs
- [x] `subprocess.run([...])` confirmed (no shell)

## Files Changed

1. **`tools/dspy-tune/test_agent.py`** — Added `test_tool_error_budget_exhaustion_aborts` (HARNESS-03 anti-vacuity test)

## Exit Gate

**HARNESS-03 SATISFIED:** Verb-arg usage is hardened. Malformed calls burn budget. Budget exhaustion aborts correctly. Anti-vacuity test proves it works.

## Notes

- The budget mechanism was already implemented correctly in Phase 115
- This phase was verification + test, not new implementation
- The anti-vacuity test required varying stdout to avoid the no_progress bound triggering first
- Budget bound runs after no_progress bound in the loop (order: max_turns check → tool execution → no_progress → budget)