---
phase: 54
plan: 02
subsystem: docs/runbooks
tags: [observability, runbooks, ci-gate, compliance-test]
requires: []
provides:
  - "TestRunbookCompliance Go test gating runbook compliance in CI"
affects: []
tech_stack:
  added: []
  patterns:
    - "Compliance regression test reads docs in-tree and asserts required sections + forbidden substrings"
key_files:
  created:
    - docs/runbooks/runbooks_test.go
  modified: []
decisions:
  - "README.md exempt from forbidden-substring gate: its negative disclaimer ('No Prometheus, no Grafana, no Docker.') is the project's anti-external-stack stance and is load-bearing operator guidance. Gate still applies to all four runbook files. README still rejects promql fenced blocks."
  - "Heading set chosen by reading actual files: ## Symptoms, ## Inspect, ## Triage, ## Remediate. All four runbooks share these exact H2 headings, so the test asserts all four (lowercased substring match)."
metrics:
  duration: "~6 minutes"
  completed: 2026-04-28T09:42:23Z
  tasks_completed: 1
  files_created: 1
  files_modified: 0
---

# Phase 54 Plan 02: Runbook compliance CI gate Summary

Add `TestRunbookCompliance` in `docs/runbooks/runbooks_test.go` that fails CI if any of the four operator runbooks is deleted, missing required H2 headings, or contains forbidden Grafana/PromQL references — mechanically enforcing ROADMAP criterion #3 of phase 54.

## What Shipped

- `docs/runbooks/runbooks_test.go` — single test file, package `runbooks_test`, no production-package pollution.
- 5 sub-tests under `TestRunbookCompliance`:
  1. `ErrCircuitOpen.md` — exists, has Symptoms/Inspect/Triage/Remediate, no forbidden strings, no promql fence.
  2. `deadline-timeouts.md` — same.
  3. `ls-crash-restart.md` — same.
  4. `memory-pressure-eviction.md` — same.
  5. `README_indexes_all_runbooks` — README references each runbook basename and contains no promql fence.

## Verification

- `gofmt -l docs/runbooks/runbooks_test.go` → empty (clean format).
- `go vet ./docs/runbooks/...` → exit 0.
- `go test ./docs/runbooks/ -run TestRunbookCompliance -count=1 -v` → all 5 sub-tests PASS.
- All five files (4 runbooks + README) present on disk (sanity-checked via `test -f` loop).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Adjusted README scope to permit negative disclaimer**

- **Found during:** Task 1 first test run.
- **Issue:** Plan `<behavior>` specified that README.md must contain none of the forbidden substrings, but `docs/runbooks/README.md:5` says `"No Prometheus, no Grafana, no Docker."` — a deliberate negative disclaimer of the project's anti-external-stack stance (consistent with commit `82d39f87` "abandon Grafana-stack approach"). Removing this sentence would lose load-bearing operator guidance; the gate's intent is to forbid Grafana **as a recommendation**, not to forbid the project from declaring it doesn't use Grafana.
- **Fix:** Kept the forbidden-substring gate on the four runbook files (where any Grafana/PromQL mention would be a recommendation). For README.md the test now asserts (a) it indexes all four runbooks by basename, and (b) it contains no `\`\`\`promql` fenced code block. The decision and rationale are documented as a comment block in the test file itself.
- **Files modified:** `docs/runbooks/runbooks_test.go` (only — README.md and runbook content untouched).
- **Commit:** `5207e740`.

## Threat Surface

No new network endpoints, auth paths, file access patterns, or schema changes. Test reads in-tree markdown only. Mitigations applied for `T-54-07`, `T-54-08`, `T-54-09` from the plan's STRIDE register.

## TDD Gate Compliance

Plan-level type is `execute` (not `tdd`), so plan-level RED/GREEN/REFACTOR commit gating does not apply. The single task carried `tdd="true"`, and the convention for a regression-guard test (asserting an already-correct state) is a single `test(...)` commit — used here. The test was written, executed, observed to fail at the README sub-test (Rule 3 deviation above), corrected, observed green, committed once. No separate RED/GREEN commits because there is no production code paired with the test — only docs that pre-exist.

## Self-Check: PASSED

- [x] `docs/runbooks/runbooks_test.go` — FOUND on disk.
- [x] Commit `5207e740` — FOUND in `git log --oneline`.
- [x] All 5 sub-tests of `TestRunbookCompliance` green on final run.
- [x] No edits to STATE.md or ROADMAP.md (worktree agent constraint honored).
- [x] No edits to runbook content or README.md (plan scope honored).
