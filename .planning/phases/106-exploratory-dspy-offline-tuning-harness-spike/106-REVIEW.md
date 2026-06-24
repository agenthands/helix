---
phase: 106-exploratory-dspy-offline-tuning-harness-spike
reviewed: 2026-06-24T00:00:00Z
depth: standard
files_reviewed: 8
files_reviewed_list:
  - tools/dspy-tune/scorer.py
  - tools/dspy-tune/golden/parity_cases.json
  - tools/dspy-tune/test_parity.py
  - tools/dspy-tune/test_degenerate.py
  - tools/dspy-tune/optimize.py
  - tools/dspy-tune/test_split.py
  - test/oracle/adopt/parity_test.go
  - internal/lint/toolsquarantine/analyzer.go
findings:
  critical: 0
  warning: 1
  info: 2
  total: 3
status: issues_found
---

# Phase 106: Code Review Report (Iteration 2)

**Reviewed:** 2026-06-24
**Depth:** standard
**Files Reviewed:** 8
**Status:** issues_found

## Summary

Re-review after the three iteration-1 Warnings were fixed. I confirmed all three
fixes are GENUINELY resolved (not vacuous), then went hunting for remaining /
newly-introduced divergence per the scrutiny brief. One real but low-materiality
parity divergence remains in `first_command` whitespace handling (WR-01-A,
below). Everything else is clean.

**Verification of the three prior fixes (all GENUINELY resolved):**

- **WR-01 (parity bug) — RESOLVED, non-vacuous.** `scorer.py::first_command` now
  strips `"$ "` then `"> "` with two SEPARATE sequential `if`s (lines 51–54),
  matching Go's two back-to-back `strings.TrimPrefix` calls
  (`scorecard.go:67-68`). I replayed the OLD if/elif version against the new
  corpus case `"$ > helix find-references --symbol X"`: it yields
  `"> helix find-references --symbol X"` (chose=False) versus the corpus-pinned
  `"helix find-references --symbol X"` (chose=True) — so the new case WOULD have
  caught the old bug. The Go side independently agrees with the corpus on that
  case (`FirstCommand` returns `"helix find-references --symbol X"`), and
  `TestPythonGoParityCorpus` PASSES. Non-vacuity confirmed.
  - Also checked the OTHER stripping steps the brief flagged: fence skip
    (`startswith("```")` vs `HasPrefix(line, "```")`), backtick trim
    (`strip("`")` vs `Trim(line, "`")`), reversed prompt order `"> $ helix foo"`,
    nested/multiple backticks `"``helix foo``"`, and `` `$ helix foo` ``. All
    AGREE between Go and Python on every probe. No remaining if/elif-vs-sequential
    or ordering divergence in the prompt-strip logic.

- **WR-02 (degenerate guard) — RESOLVED, non-vacuous.** `test_degenerate.py` is
  broadened (`_DEGENERATE_PATTERNS`) and now flags all 5 realistic always-helix
  phrasings (`test_realistic_degenerate_phrasings_flagged` PASSES), while the
  legitimate conditional steering text is NOT flagged. The flag/no-flag pair
  proves discrimination. The docstring honestly scopes it as a "pre-review smell
  test, not a sound gate." (See IN-01 for the documented residual evasion.)

- **WR-03 (val carve) — RESOLVED, no off-by-one.** For every TRAIN size n ≥ 2,
  `split = max(1, n//2)` gives `examples[:split]` / `examples[split:]` as a true
  disjoint partition with a non-empty valset (since `max(1, n//2) < n` for n ≥ 2).
  n < 2 fails loudly (prints + returns 0, never reaches `compile()`). The
  structural-disjointness `assert`s (optimize.py:118-122) back this up. I verified
  n = 0..7 exhaustively: disjoint and val-nonempty in every shippable case.

**Invariants re-verified after the edits (all STILL hold):**

- **No runtime Python / no `.go` under tools/:** `find tools/` for `*.go`/`go.mod`
  returns NONE — the tree is invisible to `go build`/`go test`. The
  `toolsquarantine` analyzer is wired into `make vet` via
  `cmd/vet-tools-quarantine/main.go`, has tests + testdata, and passes.
- **No auto-adopt:** `optimize.py` writes ONLY to the git-ignored
  `output/optimized.json` via `optimized.save()`. Its only `open()` calls are
  read-mode (`_load_jsonl`). No write to SKILL.md / reference.md / refgen.
- **Unset-key guard:** the `OPENAI_API_KEY` check (lines 48-62) returns 0 BEFORE
  any `import dspy` / `from scorer import ...`, so the harness no-ops cleanly with
  neither key nor `dspy` installed.
- **TEST sequestration:** `test.jsonl` is loaded only for the final held-out
  report print (line 140), never passed to `compile()`. `test_split.py` asserts
  TEST ∩ TRAIN == ∅ and passes.

Note (per brief): unresolved `dspy` / `scorer` imports are dev-time-only and NOT
flagged.

## Warnings

### WR-01-A: `first_command` still diverges from Go on ASCII separator chars (U+001C–U+001F)

**File:** `tools/dspy-tune/scorer.py:47` (and `:55`); contract pinned at `scorer.py:1-9`, `:36-39`
**Issue:** The docstring claims this module "mirrors test/oracle/adopt/scorecard.go
EXACTLY" and is a "Verbatim port of FirstCommand." It is not byte-exact: Python's
`str.strip()` and Go's `strings.TrimSpace` (used at `scorecard.go:61,66,69`) disagree
on which code points are whitespace. Concretely, Python `str.strip()` strips the
ASCII separators U+001C, U+001D, U+001E, U+001F (FS/GS/RS/US — they ARE whitespace in
Python's Unicode table), but Go's `unicode.IsSpace` does NOT treat them as space.

Replayed both implementations on `"\x1chelix foo"`:
- Go `FirstCommand` → `"\x1chelix foo"` → `ClassifyChoice` = (chose=False) — unclassified.
- Python `first_command` → `"helix foo"` → `classify_choice` = (chose=True) — a choice.

That is a genuine classification disagreement on the same input — the exact thing the
shared-corpus parity contract exists to prevent. It is NOT caught by
`TestPythonGoParityCorpus` / `test_parity.py` because no corpus case begins with a
separator char (the corpus only proves agreement on the 12 listed cases, not over the
whole input domain — see `parity_test.go:12-24`).

Materiality is LOW: U+001C–U+001F essentially never appear at the start of a real LLM
transcript line, so the choice_rate metric is not realistically perturbed. This is
classified WARNING (not BLOCKER) because the divergence is real and contradicts the
"EXACTLY / verbatim" contract, but cannot be triggered by realistic model output.
(The earlier-suspected U+0085 NEL and U+00A0 NBSP do NOT diverge — both sides strip
them — so the separator chars are the only residual gap.)

**Fix:** Either (a) make the port byte-exact by restricting the Python strip to Go's
`TrimSpace` cutset, or (b) soften the docstring's "EXACTLY / verbatim" claim to
"semantically equivalent over realistic transcripts" and add a one-line note that
ASCII C0 separators are out of contract. Option (a) is the tighter fix — define a
cutset string of exactly Go's `unicode.IsSpace` code points (`\t\n\v\f\r`, space,
U+0085, U+00A0, and the U+2000-range spaces) and strip with that instead of the
default `str.strip()` / `.strip()` at lines 47 and 55. If byte-exactness is not worth
the complexity for a dev-time spike, option (b) (docstring softening) is acceptable —
but the current "EXACTLY"/"verbatim" wording should not stand unqualified while this
divergence exists.

## Info

### IN-01: Documented residual evasion in `is_degenerate_steering` (conditional-marker over-veto)

**File:** `tools/dspy-tune/test_degenerate.py:100-101`
**Issue:** A degenerate always-helix instruction that incidentally contains `" if "`
or `" when "` is rescued to False, e.g. `is_degenerate_steering("Always emit helix
first if you can.")` returns False despite being unconditional. This is a false
negative. It is EXPLICITLY disclosed in the module docstring (WR-02 scope: "can still
be evaded by phrasings outside its pattern list... pre-review smell test, not a sound
gate") and the real backstop is the no-auto-adopt human-review re-entry gate. Logged
as Info (not Warning) because the scope limitation is honestly documented and the
non-vacuity flag/no-flag pair still holds. No action required for a spike; if this
ever gets promoted to a wired gate, narrow the veto to scoped markers
("when the question is", "fall back", "for free-text") and drop the broad `" if "` /
`" when "` substrings.

### IN-02: Parity corpus proves agreement only on enumerated cases, not over the input domain

**File:** `tools/dspy-tune/test_parity.py:46-58`, `test/oracle/adopt/parity_test.go:41-47`
**Issue:** Both parity tests iterate the 12 committed corpus cases and assert
Go == Python == expected on each. This proves point-wise agreement, not domain-wide
equivalence — WR-01-A is precisely a domain point outside the corpus where they
disagree. This is the correct and conventional shape for a shared-golden contract
(and is honestly described as such), so it is Info, not a defect. Consider adding a
separator-char case to the corpus if WR-01-A is fixed via option (a), so the contract
pins the now-converged behavior.

---

_Reviewed: 2026-06-24_
_Reviewer: Claude (gsd-code-reviewer)_
_Depth: standard_
