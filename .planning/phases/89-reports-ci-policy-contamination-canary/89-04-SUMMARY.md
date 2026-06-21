---
phase: 89-reports-ci-policy-contamination-canary
plan: 04
subsystem: bench-ci
tags: [ci, github-actions, workflow, bench, cost-budget, docs, infra-04]
requires:
  - "Makefile bench / bench-quick targets (Phase 75/77)"
  - ".github/workflows/go-test.yml conventions (least-privilege perms, EVAL-07 forbid-judge gate)"
provides:
  - ".github/workflows/bench.yml — PR bench-quick (5-min hard cap, no LLM cost) + nightly/maintainer-gated bench-full"
  - "bench/ci_workflow_test.go — hermetic YAML-parse acceptance proof for the workflow structure"
  - "bench/BENCH.md ## CI Cost Policy — documented cost budget + contamination-canary policy"
affects:
  - "bench (new test-only package benchci)"
tech-stack:
  added: []
  patterns:
    - "Hermetic YAML-parse/lint test (gopkg.in/yaml.v3, already a direct dep) as INFRA-04 acceptance — NOT a live CI run"
    - "Least-privilege workflow permissions + event-name job gating (no untrusted input in run:/ref:)"
key-files:
  created:
    - .github/workflows/bench.yml
    - bench/ci_workflow_test.go
  modified:
    - bench/BENCH.md
decisions:
  - "Full-bench gate = schedule (nightly cron 06:00 UTC) + workflow_dispatch (maintainer on-demand), per gate_note — no label infrastructure needed; matches go-test.yml's workflow_dispatch convention"
  - "Reused gopkg.in/yaml.v3 (already a direct module dep used by bench/cost + bench/runners) for the parse test — zero new dependency, go.mod/go.sum unchanged"
  - "New test package benchci lives in bench/ (no prior package there); the test reads ../.github/workflows/bench.yml relative to the package dir"
metrics:
  duration: ~6m
  completed: 2026-06-21
---

# Phase 89 Plan 04: CI Cost-Policy Workflow + Hermetic YAML-Parse Proof + Cost/Canary Doc Summary

INFRA-04 CI cost-policy workflow authored and hermetically proven: a PR `make bench-quick` job with a hard 5-minute cap and no LLM cost, plus a `schedule`/`workflow_dispatch`-gated full `make bench` job, with the workflow structure asserted by a dependency-free Go YAML-parse test and the cost budget + contamination-canary policy documented in `bench/BENCH.md`.

## What Was Built

### Task 1 — `.github/workflows/bench.yml` (commit `3f0af4e7`)
- `permissions: {contents: read}` (least privilege, T-89-04-03).
- Triggers: `pull_request: {branches: [main]}`, `schedule: [{cron: "0 6 * * *"}]`, `workflow_dispatch:`.
- Job `bench-quick`: `if: github.event_name == 'pull_request'`, `runs-on: ubuntu-latest`, **hard `timeout-minutes: 5`**, steps = `actions/checkout@v4` + `actions/setup-go@v5` (`go-version: '1.25.x'`, `cache: true`) + `run: make bench-quick`. Hermetic — no provider secret referenced (T-89-04-01, T-89-04-02).
- Job `bench-full`: `if: github.event_name == 'schedule' || github.event_name == 'workflow_dispatch'`, `timeout-minutes: 30`, same checkout/setup-go + `run: make bench`. Never runs on `pull_request`.
- No LLM-judge reference (EVAL-07) — confirmed the file passes go-test.yml's forbid-judge grep gate.

### Task 2 — hermetic parse test + cost/canary doc (commit `e78dfca8`)
- `bench/ci_workflow_test.go` (new `package benchci`): `TestBenchWorkflow` reads `../.github/workflows/bench.yml`, parses it as YAML, and asserts:
  1. valid YAML (parse succeeds);
  2. `bench-quick` exists with `timeout-minutes: 5` and is `pull_request`-gated;
  3. its single `run:` step is exactly `make bench-quick`;
  4. `permissions.contents == read`;
  5. `bench-full` runs `make bench`, is gated on `schedule`/`workflow_dispatch`, and is NOT `pull_request`-gated;
  6. no LLM-judge reference (EVAL-07 mirror);
  - plus a no-secret assertion on the `bench-quick` job body (T-89-04-01).
- `bench/BENCH.md`: appended a `## CI Cost Policy` section (post-append-unique heading — absent before this task) with a cost-budget table and the contamination-canary policy (flagged `(task,mode)` cells excluded from headline numbers, listed in the `leaderboard.md` footnote, still measured by `CanaryPassRate`). Existing content untouched (append-only).

## Verification Results

- `go test ./bench/ -run 'TestBenchWorkflow|TestCIWorkflow' -count=1` → **ok** (hermetic structure proof).
- `grep -q '## CI Cost Policy' bench/BENCH.md` → pass; heading count = 1 (post-append-unique anchor with teeth — absent before Task 2).
- Task 1 gate: `make bench-quick` present, `timeout-minutes: 5` present, no `tool_behavior_judge` → all pass; YAML valid (`python3 yaml.safe_load`).
- `go build ./...` → OK; `go vet ./bench/...` → clean; `make vet` (incl. all project analyzers) → exit 0.
- No new dependency: `go.mod`/`go.sum` unchanged; the parse test reuses the already-direct `gopkg.in/yaml.v3`.

## Inspection-Gated Item (honest, per 89-VALIDATION.md)

The **LIVE CI execution** of `bench.yml` (bench-quick ≤5 min on a real PR; full bench maintainer/schedule-gated) is **inspection-gated** — it is verified post-merge by opening a PR, NOT proven by this plan. The **YAML STRUCTURE** is the hermetic local proof delivered here.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Removed literal `tool_behavior_judge` from a bench.yml comment**
- **Found during:** Task 1 verification.
- **Issue:** An explanatory comment in `bench.yml` used the literal string `tool_behavior_judge`, which tripped both the planned `! grep -q 'tool_behavior_judge'` gate and go-test.yml's forbid-judge CI gate (the gate is a substring grep, so even a comment mention fails it).
- **Fix:** Reworded the comment to "informational LLM judge" without the literal token; the parse test asserts the absence via a split string constant so the test source itself does not trip the same grep gate.
- **Files modified:** `.github/workflows/bench.yml` (and the test uses `"tool_behavior" + "_judge"` to avoid self-tripping).
- **Commit:** `3f0af4e7` / `e78dfca8`.

## Known Stubs

None — all artifacts are wired and verified.

## Threat Flags

None — no new security surface beyond the workflow itself, whose budget/secret/privilege risks are mitigated and asserted (T-89-04-01..04). The PR job uses no untrusted input in `run:`/`ref:` and references no secret.

## Self-Check: PASSED
- FOUND: `.github/workflows/bench.yml`
- FOUND: `bench/ci_workflow_test.go`
- FOUND: `bench/BENCH.md` (## CI Cost Policy)
- FOUND commit: `3f0af4e7` (Task 1)
- FOUND commit: `e78dfca8` (Task 2)
