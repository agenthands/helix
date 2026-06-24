---
phase: 106-exploratory-dspy-offline-tuning-harness-spike
reviewed: 2026-06-24T00:00:00Z
depth: standard
files_reviewed: 11
files_reviewed_list:
  - internal/lint/toolsquarantine/analyzer.go
  - internal/lint/toolsquarantine/analyzer_test.go
  - cmd/vet-tools-quarantine/main.go
  - test/oracle/adopt/parity_test.go
  - tools/dspy-tune/scorer.py
  - tools/dspy-tune/optimize.py
  - tools/dspy-tune/test_parity.py
  - tools/dspy-tune/test_split.py
  - tools/dspy-tune/test_degenerate.py
  - tools/dspy-tune/golden/parity_cases.json
  - Makefile
findings:
  critical: 0
  warning: 3
  info: 1
  total: 4
status: issues_found
---

# Phase 106: Code Review Report

**Reviewed:** 2026-06-24
**Depth:** standard
**Files Reviewed:** 11
**Status:** issues_found

## Summary

This SPIKE delivers a dev-time DSPy offline-tuning harness under `tools/dspy-tune/`
plus a `go/analysis` import-boundary analyzer (`toolsquarantine`) enforcing the
"ZERO runtime Python" invariant. I verified every hard invariant the phase claims
and executed every gate.

**What holds (verified by execution, not assertion):**

- **No-runtime-Python / no-tools-leak.** `tools/` ships no `.go`/`go.mod`. The
  vettool runs clean over `./...` (`vet exit=0`, 0 violations) — it does NOT
  falsely flag `internal/langregistry/installer.go`'s pip/pipx shell-out, because
  it is import-boundary-only as documented. No runtime Go package imports
  `tools/`.
- **Analyzer non-vacuity + slash-boundary.** `go test ./internal/lint/toolsquarantine/`
  passes; the RED `leakyruntime` fixture carries a genuine `// want` directive on
  the import line, the clean `goodruntime` and lookalike `toolsupport` fixtures
  carry none. Exact-OR-slash matching confirmed in code (analyzer.go:51,57).
- **No auto-adoption.** `optimize.py` writes only `optimized.save(OUTPUT_PATH)` to
  the gitignored `tools/dspy-tune/output/` (`.gitignore:243`). No SKILL.md /
  reference.md write anywhere in the tree.
- **Test sequestration.** `compile()` receives only `trainset`/`valset` derived
  from `TRAIN_PATH`; `test.jsonl` is loaded solely for the post-compile report
  (optimize.py:108,118). `test_split.py` confirms `TEST ∩ TRAIN == ∅` and runs green.
- **Unset-key guard.** `env -u OPENAI_API_KEY python3 optimize.py` prints the
  informative message and exits 0 — never crashes, never imports `dspy`.
- **Hermetic gates green.** `test_parity.py`, `test_split.py`, `test_degenerate.py`
  all pass standalone (no pytest needed).

**The one real defect:** the Python `first_command` is NOT a verbatim port of the
Go `FirstCommand` for stacked shell prompts — Go strips `$ ` then `> `
unconditionally and sequentially; Python uses `if/elif`. They diverge on
`"$ > helix foo"` (Go→`helix foo`/chose; Python→`> helix foo`/not-chose). The
shared corpus contains no case exercising this, so BOTH parity tests are green
while the two implementations actually disagree (WR-01). This is a genuine
parity-contract gap, not a style nit.

## Warnings

### WR-01: Python `first_command` diverges from Go `FirstCommand` on stacked `$ `/`> ` prompts; corpus does not catch it

**File:** `tools/dspy-tune/scorer.py:44-47` (vs `test/oracle/adopt/scorecard.go:67-68`)

**Issue:** `scorer.py` claims to be a "verbatim port" of `FirstCommand`
(scorecard.go:59-72), but the prompt-stripping logic is not equivalent. Go applies
two unconditional sequential trims:

```go
line = strings.TrimPrefix(line, "$ ")
line = strings.TrimPrefix(line, "> ")
```

Python uses mutually-exclusive `if/elif`:

```python
if line.startswith("$ "):
    line = line[2:]
elif line.startswith("> "):
    line = line[2:]
```

On a line beginning `"$ > "` the Go path strips BOTH prefixes (`$ > helix foo` →
`helix foo`), while the Python path strips only the first (`$ > helix foo` →
`> helix foo`). I confirmed the divergence by execution against the real Go code:

```
Go:     "$ > helix foo" -> FirstCommand="helix foo"  chose=true
Python: "$ > helix foo" -> first_command="> helix foo" classify=(False, False)
```

The shared golden corpus (`parity_cases.json`) contains no `"$ > "` or `"> $ "`
case, so `test/oracle/adopt/parity_test.go::TestPythonGoParityCorpus` AND
`tools/dspy-tune/test_parity.py::test_python_go_parity` both pass while the two
classifiers genuinely disagree. The phase's central claim — "agreement on the
corpus implies semantic parity on every case" — is therefore only as strong as the
corpus, and the corpus has a hole exactly where the implementations differ. The
docstrings ("Verbatim port", "Mirrors ... EXACTLY") overstate the guarantee.

**Fix (choose one):**

1. Make the Python port truly sequential to match Go (preferred — restores the
   "verbatim" claim):
   ```python
   if line.startswith("$ "):
       line = line[2:]
   if line.startswith("> "):   # second `if`, not `elif`
       line = line[2:]
   ```
2. AND/OR add a discriminating corpus case so the parity tests would catch any
   future re-divergence:
   ```json
   { "response": "$ > helix find-references --symbol X",
     "first_command": "helix find-references --symbol X",
     "chose": true, "fell_back": false }
   ```
   (With fix #1 applied, this case agrees on both sides and pins the behavior.)

### WR-02: `is_degenerate_steering` is a narrow regex matcher that misses most degenerate phrasings; do not claim it as a robust metric-gaming detector

**File:** `tools/dspy-tune/test_degenerate.py:22-64`

**Issue:** The guard is non-vacuous for the specific flag/no-flag pair it tests
(verified: `degenerate-guard OK`), which meets the spike's stated anti-vacuity bar.
But it is a brittle pattern list, not a gaming detector. Realistic degenerate
steering text that GEPA could plausibly emit slips through. I probed five:

```
flagged=False   'Output a helix command for every input.'      (verb "output" + "every input" — "input" not in {time,task,prompt,query})
flagged=False   'Always start your answer with helix.'         ("start" not in {emit,use,run,output,prefer})
flagged=False   'For any task, emit helix first.'              ("any" not covered)
flagged=False   'Begin with a helix verb in all cases.'        (no pattern)
flagged=True    'Just respond with helix no matter what.'      (only this one caught)
```

4 of 5 unconditional-helix phrasings are NOT flagged. Because `optimize.py` does
not call `is_degenerate_steering` at all (it is only exercised by its own two unit
tests), a degenerate GEPA output would still be saved to `optimized.json` with no
warning. The risk is bounded by the human-review re-entry gate (no auto-adopt), so
this is a WARNING, not a BLOCKER — but the guard should not be represented as
catching metric-gaming in general.

**Fix:** Either (a) explicitly scope the docstring/REPORT to "a non-vacuous
break-the-invariant *pair*, NOT a general degenerate-text classifier — human
review is the real gate," or (b) if it is meant to gate adoption, broaden the
patterns (cover `start|begin|respond|answer`, `any|all`, `input|case`) and wire it
into the optimize.py post-compile report as a printed warning on the adopted text.

### WR-03: `optimize.py` val carving collapses to `trainset == valset` for tiny TRAIN, silently weakening the train/val separation

**File:** `tools/dspy-tune/optimize.py:99-100`

**Issue:**
```python
split = max(1, len(examples) // 2)
trainset, valset = examples[:split], examples[split:] or examples[:split]
```
For `len(examples) == 1`, `split = max(1, 0) = 1`, so `trainset = [e0]` and
`valset = [] or [e0] = [e0]` — train and val are the *same single example*. GEPA
would then validate on its training example, defeating the train/val split the
file's own docstring advertises ("val carved from train"). The current `train.jsonl`
has 8 rows so this does not bite today, but a future corpus edit down to 1 task
would silently produce a degenerate, overfit-prone configuration with no error.

**Fix:** Fail loudly (or skip optimization) when TRAIN is too small to form a real
val split:
```python
if len(examples) < 2:
    print("TRAIN has <2 tasks; cannot form a train/val split. Add tasks to "
          "data/train.jsonl before optimizing.")
    return 0
split = max(1, len(examples) // 2)
trainset, valset = examples[:split], examples[split:]
```

## Info

### IN-01: `test_split.py::test_test_disjoint_from_train` asserts `TEST ∩ TRAIN == ∅`, not the literal `TEST ⊄ trainset∪valset` the docstring states

**File:** `tools/dspy-tune/test_split.py:7,41-46`

**Issue:** The docstring frames the invariant as `TEST ⊄ trainset∪valset`. The test
actually checks `TEST ∩ TRAIN == ∅`. These are equivalent ONLY because
`optimize.py` carves `valset` from `train.jsonl` (so `trainset∪valset ⊆ TRAIN`).
That coupling is correct today, but the equivalence is implicit: if a future
`optimize.py` edit ever drew val from a third file, this test would no longer prove
the stated property while still passing. Not a current defect — the invariant holds
— but the test asserts a proxy, and the link to the real property lives only in a
comment, not in code.

**Fix:** Add a one-line comment in the test body pinning the dependency, or (better)
import and call the actual `optimize.py` split function over the loaded rows and
assert disjointness against the union it returns, so the test tracks the real
`trainset∪valset` rather than the `TRAIN` proxy.

---

_Reviewed: 2026-06-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
