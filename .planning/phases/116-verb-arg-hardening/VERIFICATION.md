# Phase 116: Verb-Arg Hardening - Verification

**Verified:** 2026-06-26
**Verdict:** PASSED

## Verification Checklist

### 1. Budget Implementation (react.py) ✓

- [x] `_TOOL_ERROR_BUDGET = 5` constant at line 75
- [x] `tool_errors = 0` counter initialization at line 150
- [x] `tool_errors += 1` on `result.exit != 0` at line 196
- [x] Budget check `if tool_errors >= _TOOL_ERROR_BUDGET` at line 213
- [x] Returns `Transcript(reason="tool_error_budget", ...)` on exhaustion

### 2. Verb Call Sites (tools.py) ✓

- [x] `_argv_for()` builds fixed argv list (lines 122-137)
- [x] Uses `subprocess.run([...])` with list, not shell
- [x] All verbs go through `run_verb()` → `_argv_for()` path
- [x] No string concatenation, no shell invocation

### 3. Anti-Vacuity Test (test_agent.py) ✓

- [x] `test_tool_error_budget_exhaustion_aborts` exists (lines 750-808)
- [x] Test varies stdout to avoid no_progress bound
- [x] Asserts `reason == "tool_error_budget"`
- [x] Asserts exactly `_TOOL_ERROR_BUDGET` steps
- [x] Asserts all steps have `exit != 0`

### 4. All Tests Pass ✓

```
16 passed in 0.28s
```

All 16 tests pass including the new HARNESS-03 anti-vacuity test.

## Gaps Found

None. Implementation matches PLAN.md decisions D-01 through D-06.

## Decision Verification

- **D-01** Budget fixed at 5: ✓ (line 75)
- **D-02** Errors cumulative: ✓ (no reset on success)
- **D-03** Only exit != 0 counts: ✓ (line 196)
- **D-04** Post-hoc counting only: ✓ (no pre-call validation)
- **D-05** Anti-vacuity test shape: ✓ (test varies stdout)
- **D-06** v2.4 positional→flags fix: ✓ (_argv_for builds --flag value)

## Exit Gate

**HARNESS-03 SATISFIED:** Verb-arg usage is hardened. Malformed calls burn budget. Budget exhaustion aborts correctly. Anti-vacuity test proves it works.

---

**Verified by:** architect (in-band, subagent model unavailable)
**Date:** 2026-06-26