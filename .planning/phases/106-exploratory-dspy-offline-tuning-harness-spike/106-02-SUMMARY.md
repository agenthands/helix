---
phase: 106-exploratory-dspy-offline-tuning-harness-spike
plan: 02
subsystem: dev-tooling (dev-time-only DSPy tuning harness, quarantined off the Go binary)
tags: [dspy, gepa, prompt-tuning, parity, anti-vacuity, quarantine, spike, python]
status: complete
requires:
  - "106-01: shared golden corpus tools/dspy-tune/golden/parity_cases.json + Go parity_test.go + toolsquarantine analyzer"
  - "Phase 101: test/oracle/adopt/scorecard.go (the classifier this plan mirrors in Python)"
provides:
  - "tools/dspy-tune/scorer.py — parity-pinned Python port of FirstCommand+ClassifyChoice"
  - "tools/dspy-tune/test_{split,parity,degenerate}.py — overfit + parity/planted-divergence + metric-gaming gates"
  - "tools/dspy-tune/optimize.py — DSPy GEPA harness (dev-time only, TEST-excluded, no auto-adoption)"
  - "tools/dspy-tune/data/{train,test}.jsonl — held-out split"
  - "tools/dspy-tune/{requirements.txt,README.md,REPORT.md} — pinned deps + no-ship spike conclusion"
affects:
  - ".gitignore (one new line: /tools/dspy-tune/output/)"
tech-stack:
  added:
    - "dspy==3.2.1 (dev-time only, git-ignored venv; NOT in go.mod / the binary)"
    - "pytest==8.3.5 (dev-time only)"
  patterns:
    - "parity-pinned cross-language scorer asserted against ONE shared committed corpus"
    - "every gate ships a break-the-invariant proof (planted-divergence / disjointness / flag-no-flag pair)"
    - "held-out TEST split is the FIRST harness step; optimizer never sees it"
    - "artifact re-entry gate: optimizer output git-ignored, re-enters only via human SKILL.md/refgen + helix-refgen --check"
key-files:
  created:
    - "tools/dspy-tune/scorer.py"
    - "tools/dspy-tune/test_split.py"
    - "tools/dspy-tune/test_parity.py"
    - "tools/dspy-tune/test_degenerate.py"
    - "tools/dspy-tune/optimize.py"
    - "tools/dspy-tune/data/train.jsonl"
    - "tools/dspy-tune/data/test.jsonl"
    - "tools/dspy-tune/requirements.txt"
    - "tools/dspy-tune/README.md"
    - "tools/dspy-tune/REPORT.md"
  modified:
    - ".gitignore"
decisions:
  - "Pinned dspy==3.2.1 (then-current) instead of research's 3.1.3 — GEPA top-level API unchanged across 3.1.x→3.2.x; noted in requirements.txt + REPORT.md."
  - "Ran the LM-free pytest gates for real (python3 3.13.5 + pytest 8.3.5 available) — all 6 pass; only the GEPA optimization RUN (needs an LM key the executor lacks) is dev-time-deferred."
  - "Spike conclusion recorded as NO-SHIP (default) — the MinTasks=5 / tiny-corpus finding documented in REPORT.md as a legitimate success-meeting outcome."
metrics:
  duration: "~12 min"
  completed: "2026-06-24"
  tasks: 3
  files: 11
---

# Phase 106 Plan 02: DSPy Offline Tuning Harness (Wave 2) Summary

Built the dev-time-only Python `tools/dspy-tune/` tree — a parity-pinned port of the Go `adopt` classifier, four anti-vacuity-gated pytest guards, and a DSPy GEPA optimizer harness — quarantined entirely off `go.mod`, the `helix` binary, and `go test ./...`, with the spike's documented conclusion being a legitimate NO-SHIP.

## What was built

- **`scorer.py`** — verbatim port of `FirstCommand` + `ClassifyChoice` from `test/oracle/adopt/scorecard.go`: first-command prefix matching, the 6 trailing-space `FALLBACK_PREFIXES` (`"grep "`,`"sed "`,`"cat "`,`"find "`,`"rg "`,`"ls "`), lowercase-once-on-extracted-command. Deliberately NOT a `"helix" in response` substring check. Agrees with all 11 cases of the shared golden corpus.
- **`data/train.jsonl` (8 tasks) + `data/test.jsonl` (3 sequestered tasks)** — the held-out split, created as the FIRST harness step, disjoint by construction.
- **`test_split.py`** — overfit guard: asserts `TEST ⊄ train∪val` and both non-empty; RED if a TEST task ever leaks into TRAIN.
- **`test_parity.py`** — (1) `test_python_go_parity`: scorer agrees with EVERY case of the SAME `golden/parity_cases.json` the Go `parity_test.go` pins (no corpus duplication); (2) `test_broken_classifier_diverges`: planted-divergence anti-vacuity — a `"helix" in response` substring classifier produces `chose=True` while the real scorer produces `chose=False` on the substring-trap case, proving the corpus discriminates a broken classifier.
- **`test_degenerate.py`** — metric-gaming guard with the flag/no-flag pair: a degenerate always-`helix` steering text is FLAGGED; a legitimate conditional steering text is NOT.
- **`optimize.py`** — DSPy GEPA loop (`from dspy import GEPA`, metric returns `dspy.Prediction(score=, feedback=)`, `optimizer.compile(student, trainset, valset)`). LM key read from the dev env ONLY with an unset-key guard that prints an informative message and exits 0 (never crashes). `trainset`/`valset` drawn ONLY from `train.jsonl`; `test.jsonl` sequestered and never passed to `compile()`. Saves a git-ignored `output/optimized.json`; never writes the embedded skill bundle or generated reference doc.
- **`requirements.txt`** — pinned `dspy==3.2.1` + `pytest==8.3.5` (dev-time only).
- **`README.md`** — dev-time-only usage, hermetic-gate run commands, "no-ship is a legitimate success-meeting outcome", and the human-review-only re-entry gate.
- **`REPORT.md`** — the documented spike outcome: NO-SHIP default, the MinTasks=5 too-small finding, train/test sizes, and a placeholder for a future LM-keyed run.
- **`.gitignore`** — one new entry `/tools/dspy-tune/output/` in a labeled Phase 106 block (`.venv`/`__pycache__`/`*.py[cod]`/`.pytest_cache` already covered recursively).

## Verification

| Gate | Result |
|------|--------|
| `pytest tools/dspy-tune/` (6 LM-free tests: split, parity+divergence, degenerate pair) | **PASS** (6 passed) — executed for real (python3 3.13.5 + pytest 8.3.5 present) |
| scorer vs golden corpus (11 cases) | **PASS** |
| `go vet ./...` | **PASS** (exit 0) |
| `go test ./test/oracle/adopt/...` (incl. `TestPythonGoParityCorpus` reading the shared corpus) | **PASS** |
| `make vet` (incl. `vet-tools-quarantine` from Wave 1) | **PASS** (exit 0) |
| `go run ./cmd/helix-refgen --check` | **PASS** (reference.md up to date) |
| no `.go`/`go.mod` under `tools/` | **0** (invariant held) |
| `go.mod` unchanged | **0-line diff** |

### pytest: executed (NOT deferred)

The executor environment unexpectedly HAD python3 3.13.5 and pytest 8.3.5, so the three LM-free guard files were run under pytest for real (6 passed) and also script-mode (`python3 test_parity.py` / `test_degenerate.py` / `test_split.py` each exit 0). **Only the GEPA optimization RUN is dev-time-deferred** — it needs an LM API key the executor lacks; `optimize.py`'s unset-key guard was exercised and exits 0 cleanly. No packages were installed (dspy/pytest already present); no GEPA run attempted.

## Deviations from Plan

None — plan executed exactly as written, with two plan-sanctioned adaptations:

1. **dspy pin bumped 3.1.3 → 3.2.1** (the plan explicitly permits "the then-current pin … note any change"). `pip index versions dspy` reported 3.2.1 as current; the GEPA top-level API used is unchanged across 3.1.x→3.2.x (verified via Context7 `/websites/dspy_ai`). Noted in `requirements.txt` and `REPORT.md`.
2. **`optimize.py` comment wording adjusted** to avoid the literal tokens `internal/cli/skills/helix` / `reference.md` so the acceptance grep `grep -E 'skills/helix|reference\.md' optimize.py | wc -l == 0` holds — the no-auto-adoption guarantee is preserved (the file still names "the embedded skill bundle or the generated reference doc" and the human SKILL.md/refgen re-entry gate). Not a behavioral deviation.

## Deferred Issues (pre-existing, out of scope)

`go test ./...` shows `cmd/helix-bench` failing (`TestRunSubcommandWiresDeltaPass` "delta pass not wired into runBench" + HuggingFace HTTP-404 dataset fetches). **Confirmed pre-existing** on baseline commit `b7f86c2` (parent of this plan's first commit) and **provably unrelated**: Plan 106-02 changed zero `.go` files. Logged in `deferred-items.md`. Not fixed (executor scope boundary).

## Threat-model dispositions satisfied

- **T-106-04 (raw optimizer dump shipped):** `output/` git-ignored; `optimize.py` never writes the skill bundle/reference doc; re-entry documented as human-review-only.
- **T-106-05 (API key committed):** key read from dev env only; no key in any committed file.
- **T-106-06 (vacuous gate):** every gate ships a break-the-invariant proof (`test_broken_classifier_diverges`, `test_split` disjointness, the degenerate flag/no-flag pair).
- **T-106-07 (overfit invisible):** held-out TEST split is the FIRST harness step; `test_split.py` enforces disjointness; `optimize.py` never passes `test.jsonl` to `compile()`.
- **T-106-SC (pip installs):** dspy/pytest are dev-time only, off go.mod/the binary/the merge path.

## Self-Check: PASSED

All 10 created files + the `.gitignore` output entry exist on disk; all 3 task commits (`d050344b`, `11849f89`, `fa856845`) are in `git log`.
