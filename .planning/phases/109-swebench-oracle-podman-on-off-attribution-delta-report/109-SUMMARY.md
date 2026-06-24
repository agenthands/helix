---
phase: 109
phase_name: SWE-bench Oracle via Podman + ON/OFF Attribution-Delta Report
milestone: v2.3
requirements: [ORACLE-02, TUNE-04]
status: complete
executed: inline (uv)
date: 2026-06-24
---

# Phase 109 SUMMARY — SWE-bench Oracle + ON/OFF Attribution Delta

Executed inline with `uv` (the same way 107/108 shipped), on branch
`docs/readme-langsupport-lineage`. Both requirements complete; all gates green.

## What shipped

### ORACLE-02 — honest SWE-bench task-success grader
- **`tools/dspy-tune/grade_swebench.py`** — wraps the upstream
  `swebench==4.1.0` harness (`python -m swebench.harness.run_evaluation`) on
  Podman. It is a **parity mirror** of the Go authority
  `bench/evaluators/swebench/harness.go`:
  - **Fixed argv builder** (`build_argv`) mirroring `RunArgs` element-for-element,
    pinned by the shared golden `golden/swebench_argv.json`.
  - **2-name dataset-org allowlist** (`princeton-nlp/SWE-bench_Verified`,
    `Bertsekas/SWE-Bench_Verified_UTBoost`) — closes the `princeton-nlp/` (dataset
    org) vs `SWE-bench/` (repo org) drift trap; an out-of-allowlist name is
    refused before any argv.
  - **Strict env allowlist** (`PATH/HOME/HELIX_CACHE_DIR` + `DOCKER_*`), set-keys
    only, never the parent env; `DOCKER_HOST` carries the Podman socket; empty
    `PATH` fails closed.
  - **Total, regexp-free validators** (run_id / instance_id / predictions_path)
    with leading-`-` flag-smuggling refusal.
  - **FAIL_TO_PASS + PASS_TO_PASS resolution contract** (`grade_report`):
    "resolved" requires every FAIL_TO_PASS to pass, every PASS_TO_PASS to stay
    green, and a non-empty FAIL_TO_PASS set. **0 tests evaluated ⇒ hard
    `GradeError`** (the Phase-81/108 vacuous-pass class), never a pass.
  - **Pluggable behind `taskmetric.score_task`** via the SAME
    `grader() -> (passed, tests_run)` contract `grade_aider` exposes — the GEPA
    metric is grader-agnostic.
  - Reuses `grade_aider.GradeError` so both graders raise one honest-failure type.
- **`bench/evaluators/swebench/argv_parity_test.go`** — asserts `RunArgs` equals
  the shared golden (the Go side of the parity guard; the swebench analog of
  `native_cmd_parity_test.go`).
- **`requirements.txt`** — pinned `swebench==4.1.0` (dev venv only).

### TUNE-04 — ON-vs-OFF attribution delta + ship/no-ship REPORT
- **`tools/dspy-tune/attribution.py`** — `delta = success(ON) − success(OFF)`
  on the SAME held-out split:
  - Pure core (`ArmResult`, `Attribution`, `decide_ship`, `render_report`) is
    fully hermetic (injected per-arm results).
  - **`decide_ship`** ships ONLY when `val_size > 50` (the strict TUNE-03 gate,
    imported from `optimize.VAL_SIZE_GATE` — single source of truth) AND the delta
    is strictly positive. no-ship is a legitimate, success-meeting outcome.
  - **`render_report`** records the ON/OFF rates, the delta, `val_size`, and
    **per-arm cost**, plus the human-gated-adoption note (Phase 110 owns the edit).
  - **`compute_attribution`** runs both arms on the SAME split and refuses unequal
    arms (an apples-to-oranges delta).
  - **`MeteredLLM`** — non-invasive cost meter; records REAL token cost when the
    provider returns a usage block, honest 0.0 otherwise. The Phase-107 agent/LLM
    is untouched.
  - **Gated `__main__`** mirrors `optimize.py`'s guards: no LM key / no corpus ⇒
    informative message + exit 0, never a fabricated delta. The filled REPORT
    artifact is the Phase-110 deliverable.

## Anti-vacuity (mutation-confirmed BEFORE sign-off)

Per the milestone discipline, the new guards were mutation-tested, not just run
green. Each mutation went RED, then was reverted (suite back to 51 green):
1. Defeat the 0-test refusal → `test_zero_tests_is_hard_error` RED.
2. Force `resolved=True` → 3 resolution-contract tests RED.
3. Disable the `val_size>50` gate → `test_val_size_gate_boundary_50_noship_51_ship` RED.

The fail-not-skip leg is covered: `test_fail_not_skip_default_reader_no_report`
asserts a *requested* real run that produced no `report.json` raises `GradeError`
(never a silent pass — the Phase-81 false-green class).

## Boundary (ADOPT-04, cross-cutting) — re-verified

- **Zero new Go module deps**: `git diff go.mod go.sum` empty.
- **`make vet` green** (incl. `toolsquarantine` import-boundary analyzer).
- New deps are dev-venv Python pins only (`swebench==4.1.0`); nothing shells from
  `helix` to Python; off `go.mod` / `helix setup` / default `go test ./...` /
  merge path.
- `grep -E 'skills/helix|reference\.md'` over the new harness files == 0;
  `optimize.py` still == 0.
- ALL Python via `uv` (`cd tools/dspy-tune && uv run pytest`).

## Tests

- `uv run pytest` — **51 passed** (was 25; +26: `test_swebench.py` 18,
  `test_attribution.py` 8).
- `go test ./bench/evaluators/swebench/` — ok (incl. the new argv-parity test).
- `go build ./...` — ok. `make vet` — ok.

## Files

- NEW `tools/dspy-tune/grade_swebench.py`
- NEW `tools/dspy-tune/attribution.py`
- NEW `tools/dspy-tune/golden/swebench_argv.json`
- NEW `tools/dspy-tune/test_swebench.py`
- NEW `tools/dspy-tune/test_attribution.py`
- NEW `bench/evaluators/swebench/argv_parity_test.go`
- EDIT `tools/dspy-tune/requirements.txt` (+`swebench==4.1.0`)

## Deferred / next

- The filled ship/no-ship REPORT (real ON/OFF numbers) is **Phase 110**'s
  deliverable; the live attribution run needs a corpus sized so `val_size > 50`
  (TUNE-FUT-01) — the committed corpus is still below it, so a live run no-ships
  by design (the v2.2 root cause, honestly surfaced not papered over).
