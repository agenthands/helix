---
phase: 106-exploratory-dspy-offline-tuning-harness-spike
fixed_at: 2026-06-24T00:00:00Z
review_path: .planning/phases/106-exploratory-dspy-offline-tuning-harness-spike/106-REVIEW.md
iteration: 1
findings_in_scope: 3
fixed: 3
skipped: 0
status: all_fixed
---

# Phase 106: Code Review Fix Report

**Fixed at:** 2026-06-24
**Source review:** .planning/phases/106-exploratory-dspy-offline-tuning-harness-spike/106-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 3 (3 Warning; the 1 Info finding IN-01 is out of `critical_warning` scope and was not addressed)
- Fixed: 3
- Skipped: 0

All three Warnings were fixed as genuine source changes (not doc-only deferrals),
each verified against the Go truth and/or executed gates, and committed atomically.
No Go files, `go.mod`, or `tools/`-level `.go` files were touched. The full Go gate
set (`go vet ./...`, `make vet` incl. `vet-tools-quarantine`, `go run
./cmd/helix-refgen --check` byte-clean, `go test ./test/oracle/adopt/`) and the
LM-free Python gates (`test_split.py`, `test_parity.py`, `test_degenerate.py`) are
green after the fixes.

## Fixed Issues

### WR-01: Python `first_command` diverges from Go `FirstCommand` on stacked `$ `/`> ` prompts; corpus does not catch it

**Files modified:** `tools/dspy-tune/scorer.py`, `tools/dspy-tune/golden/parity_cases.json`
**Commit:** 16becc08
**Applied fix:**
- Made `scorer.py::first_command` a verbatim port of Go's `FirstCommand`: the
  second prompt strip is now a separate **`if`** (not `elif`), so `"$ "` then
  `"> "` are stripped UNCONDITIONALLY and SEQUENTIALLY, exactly like Go's two
  back-to-back `strings.TrimPrefix` calls (scorecard.go:67-68). The fence/backtick
  stripping was already equivalent; no other if/elif-vs-sequential divergence
  exists.
- Added a DISCRIMINATING corpus case to the shared `parity_cases.json`:
  `{"response": "$ > helix find-references --symbol X", "first_command": "helix
  find-references --symbol X", "chose": true, "fell_back": false}`. The expected
  values were computed from the GO classifier truth via an in-tree probe test (not
  hand-guessed): Go yields `FirstCommand="helix find-references --symbol X"`,
  `chose=true`, `fell_back=false`.
- **Non-vacuity proven:** the OLD `if/elif` scorer on this case produced
  `first_command="> helix find-references --symbol X"` / `chose=False`, which
  diverges from the Go truth — so the new corpus case would have caught the bug
  in both parity tests.
- **Gates green:** `go test ./test/oracle/adopt/ -run Parity` (Go side pins the
  corpus to Go truth) PASS; `python3 -m pytest tools/dspy-tune/test_parity.py -q`
  PASS (fixed scorer now agrees on the new case).

### WR-02: `is_degenerate_steering` is a narrow regex matcher that misses most degenerate phrasings

**Files modified:** `tools/dspy-tune/test_degenerate.py`
**Commit:** 0fc89b56
**Applied fix:** Did BOTH options from the fix guidance —
- **Broadened the heuristic patterns** to cover realistic always-helix phrasings
  (`always|start|begin|respond|answer`; universal quantifiers
  `every|any|each|all` over `time|task|prompt|query|input|case|answer`; `in all
  cases`; verb-led `emit|use|run|output|prefer ... helix`). The conditional-marker
  veto was tightened so universal quantifiers and `regardless`/`no matter`/
  `unconditionally`/`in all cases` are treated as unconditional cues that a stray
  `if`/`fall back` can NOT rescue.
- **Scoped the claim** in the module docstring: `is_degenerate_steering` is now
  explicitly documented as a deliberately broadened HEURISTIC / pre-review smell
  test, NOT a complete metric-gaming classifier, with the human-review re-entry
  gate (optimize.py never writes SKILL.md/reference.md) named as the real backstop.
- **Anti-vacuity pair preserved AND strengthened:** the legitimate conditional
  steering text is still NOT flagged; all 5 of the reviewer's realistic degenerate
  phrasings (previously 4 slipped through) are now flagged via a new
  `test_realistic_degenerate_phrasings_flagged` test.
- **Gate green:** `python3 -m pytest tools/dspy-tune/test_degenerate.py -q` → 3
  passed.

### WR-03: `optimize.py` val carving collapses to `trainset == valset` for tiny TRAIN

**Files modified:** `tools/dspy-tune/optimize.py`
**Commit:** 3125ec69
**Applied fix:**
- Removed the `examples[split:] or examples[:split]` trap. Added an explicit
  small-TRAIN guard: when `len(examples) < 2`, the harness prints an informative
  "cannot form a disjoint train/val split" message and returns 0 (no-ship is a
  legitimate outcome) instead of silently overfitting.
- For `len(examples) >= 2`, the carve is the clean disjoint partition
  `trainset, valset = examples[:split], examples[split:]`, with assertions pinning
  `valset` non-empty and `trainset ∩ valset == ∅` (by identity) so a future
  refactor that reintroduces overlap fails loudly.
- **Verified by simulation:** `n=1` → no-ship guard; `n=2` → (1,1) disjoint;
  `n=3` → (1,2); `n=8` → (4,4) — the `trainset == valset` collapse is eliminated.
- **Constraints preserved:** the unset-`OPENAI_API_KEY` guard still exits 0 without
  crashing or importing `dspy`; `optimize.py` still writes ONLY the git-ignored
  `output/optimized.json` (`optimized.save`) and never SKILL.md/reference.md.
- **Gate green:** `python3 -m pytest tools/dspy-tune/test_split.py -q` → 2 passed.

## Notes

- **IN-01 (Info)** is out of the `critical_warning` fix scope and was intentionally
  not addressed in this iteration.
- **Logic-touching fixes** WR-01 (sequential strip ordering) and WR-03 (split carve
  boundary) were verified beyond syntax: WR-01 against the executed Go classifier
  truth (probe + Go parity test), WR-03 against an enumerated split simulation. Both
  are pinned by green parity/split tests, so they are reported as `fixed` rather than
  `fixed: requires human verification`.

---

_Fixed: 2026-06-24_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
