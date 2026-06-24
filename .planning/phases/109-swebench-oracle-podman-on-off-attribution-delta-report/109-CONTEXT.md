---
phase: 109
phase_name: SWE-bench Oracle via Podman + ON/OFF Attribution-Delta Report
milestone: v2.3
requirements: [ORACLE-02, TUNE-04]
mode: skip_discuss
research_flagged: true
created: 2026-06-24
---

# Phase 109 Context (skip_discuss)

Discussion was skipped per the `.continue-here.md` checkpoint: the plan was
fully pinned at v2.3 roadmap time and re-confirmed at the 108→109 boundary. This
file records the grounding so execution is reproducible.

## Goal

Add the heaviest grader — SWE-bench task-success via the upstream `swebench==4.1.0`
harness on Podman — behind the existing GEPA task-success metric, and make every
reported improvement an honest ON-vs-OFF attribution delta on the held-out split.

## Requirements

- **ORACLE-02** — SWE-bench task-success graded via the upstream harness on
  Podman (`podman system service` + `DOCKER_HOST`→podman socket), enforcing the
  FAIL_TO_PASS + PASS_TO_PASS resolution contract; dataset id pinned to the
  correct org; fetch-resolution + nonzero task-count asserted before any run.
- **TUNE-04** — any reported improvement is an ON-vs-OFF attribution delta
  (`success(ON) − success(OFF)`) on the held-out split, recorded with `val_size`
  and per-arm cost.

## Grounding (reuse, don't fork)

- **`bench/evaluators/swebench/harness.go`** — the Go authority `grade_swebench.py`
  mirrors for parity (the way `grade_aider.py` mirrors `loader.go`):
  - Fixed argv: `-m swebench.harness.run_evaluation --dataset_name <name>
    --predictions_path <path> --run_id <id> --max_workers <n> --cache_level <lvl>
    --instance_ids <iid>...`
  - Dataset-org allowlist (2 names): `princeton-nlp/SWE-bench_Verified`,
    `Bertsekas/SWE-Bench_Verified_UTBoost`. Closes the org-drift trap
    (`princeton-nlp/` datasets vs the `SWE-bench/` repo org).
  - Env allowlist (set-keys-only, never the full parent env): `PATH`, `HOME`,
    `HELIX_CACHE_DIR`, `DOCKER_HOST`, `DOCKER_TLS_VERIFY`, `DOCKER_CERT_PATH`.
    `DOCKER_HOST` carries the podman socket. Fail-closed on empty `PATH`.
  - Total, regexp-free validators for run_id / instance_id / predictions_path
    (leading-`-` flag-smuggling refusal).
- **`tools/dspy-tune/taskmetric.py`** — `grade_swebench.grade_task` plugs in
  behind the same `grader() -> (passed, tests_run)` contract `grade_aider` uses;
  `score_task` is unchanged.
- **`tools/dspy-tune/agent/react.py`** — `build_system_prompt(text, "on"|"off")`
  is the steering arm switch the attribution delta drives.

## Honesty / anti-vacuity invariants (carried)

- A run where ZERO FAIL_TO_PASS+PASS_TO_PASS tests resolved is a hard `GradeError`
  (the Phase-81/108 vacuous-pass class) — never a pass.
- Hermetic fake-harness test asserts the EXACT argv + env allowlist + dataset
  pin (no live python/podman/network needed).
- The live leg is gated: it SKIPs when offline, but FAILS LOUDLY on a *requested*
  real run that yields no result (HELIX_BIN fail-not-skip class).
- Each new gate ships a deliberate break-the-invariant → assert-RED test.

## Boundary (ADOPT-04, cross-cutting)

- `swebench==4.1.0` + `openai==2.43.0` stay dev-venv Python pins; keep `dspy==3.2.1`.
- Zero new Go module deps; nothing shells from `helix` to Python; off `go.mod`,
  `helix setup`, default `go test ./...`, the merge path. `make vet`
  (`toolsquarantine`) stays green.
- ALL Python via `uv`/`uvx` + venv (`cd tools/dspy-tune && uv run pytest`).

## Open questions resolved at implementation time

- Dataset org: pinned to `princeton-nlp/SWE-bench_Verified` (matches the Go
  allowlist; the harness reads the FAIL_TO_PASS/PASS_TO_PASS spec from the row).
- SWE-bench is NOT blocked on Podman — `podman system service --time=0 &` +
  `DOCKER_HOST=unix://$XDG_RUNTIME_DIR/podman/podman.sock` is configuration.
