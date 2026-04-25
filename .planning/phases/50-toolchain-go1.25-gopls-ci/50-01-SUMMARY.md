---
phase: 50-toolchain-go1.25-gopls-ci
plan: 01
subsystem: ci
tags:
  - ci
  - benchmarks
  - github-actions
  - cleanup
requirements:
  - TOOL-01
  - TOOL-02
dependency_graph:
  requires: []
  provides:
    - "CI bench gate retired (no PR-blocking benchmark workflow)"
    - "Local-first benchmark workflow documented in test/bench/baselines/README.md"
  affects:
    - .github/workflows/
    - test/bench/baselines/README.md
tech_stack:
  added: []
  patterns:
    - "Local-first benchmark capture (filename pattern v<version>-local-<goos>-<goarch>.txt)"
key_files:
  created: []
  modified:
    - test/bench/baselines/README.md
  deleted:
    - .github/workflows/bench.yml
    - .github/workflows/capture-baseline.yml
decisions:
  - "Honored D-A2 verbatim deletion (no rename / no workflow_dispatch demotion / no comment-out)"
  - "Honored D-A5 historical-file preservation (four .txt baselines untouched on disk)"
  - "Replaced literal 'capture-baseline.yml' reference in README rationale prose with 'CI baseline-capture runs' to satisfy the acceptance grep that forbids leftover workflow filename references while preserving the pivot rationale text"
metrics:
  duration: "~10 min"
  completed: "2026-04-25T20:37:59Z"
  tasks_completed: 2
  files_changed: 3
---

# Phase 50 Plan 01: Retire CI Bench Gate Summary

**One-liner:** Deleted `.github/workflows/bench.yml` (4140 bytes) and `.github/workflows/capture-baseline.yml` (2263 bytes), and rewrote `test/bench/baselines/README.md` to document the post-pivot local-first benchmark capture workflow with the four historical github-hosted baselines preserved as historical references.

## What Shipped

- **CI surgery (Task 1, commit `f96d675a`):** Both bench-related workflow files removed via `git rm` so deletions are staged. `.github/workflows/go-test.yml` (sole TOOL-01 evidence path) is unchanged. No other workflow file touched (`docker.yml`, `junie.yml`, `pytest.yml`, `docs.yaml`, `publish.yml`, `codespell.yml` all untouched per D-A14).
- **Doc rewrite (Task 2, commit `8f71fde9`):** `test/bench/baselines/README.md` replaced wholesale. New framing matches D-A5 — historical github-hosted `.txt` files labelled "captured under the retired CI gate". Local capture flow documented with the same flags the retired CI gate used (parity preserved for cross-comparison): `go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...`. Filename pattern `v<version>-local-<goos>-<goarch>.txt` per D-A4. Local benchgate invocation documented per D-A3. Cross-references CONTRIBUTING.md "Benchmarks" section.

## Files Deleted (audit trail)

| File | Size | Header |
|------|------|--------|
| `.github/workflows/bench.yml` | 4140 bytes | "Serena benchmark regression gate (Phase 9, D-01 PR tier)" |
| `.github/workflows/capture-baseline.yml` | 2263 bytes | "Dedicated baseline capture workflow for benchmark regression gate" |

## Untouched (verified)

- `.github/workflows/go-test.yml` — UNCHANGED; remains TOOL-01 evidence path.
- `test/bench/cmd/benchgate/` — UNCHANGED; preserved as local pre-release tool per D-A3.
- `test/bench/baselines/v1.1-github-hosted.txt` — preserved as historical reference.
- `test/bench/baselines/v1.2-phase10-github-hosted.txt` — preserved.
- `test/bench/baselines/v1.2-phase11-github-hosted.txt` — preserved.
- `test/bench/baselines/v1.2-phase12-github-hosted.txt` — preserved.
- All other `.github/workflows/*` (docker, junie, pytest, docs, publish, codespell) — UNCHANGED per D-A14.

## Verification

All phase-wide checks (D-A13 bullets 2 and 6) pass:

```
.github/workflows/bench.yml             absent ✓
.github/workflows/capture-baseline.yml  absent ✓
go-test.yml                             unchanged (git status empty) ✓
git grep 'capture-baseline\|bench\.yml' -- .github/   no matches ✓
test/bench/cmd/benchgate/               unchanged ✓
test/bench/baselines/v1.1-github-hosted.txt          present ✓
test/bench/baselines/v1.2-phase10-github-hosted.txt  present ✓
test/bench/baselines/v1.2-phase11-github-hosted.txt  present ✓
test/bench/baselines/v1.2-phase12-github-hosted.txt  present ✓
README.md grep 'local-first benchmark baselines'                 match ✓
README.md grep 'captured under the retired CI gate'              match ✓
README.md grep 'v<version>-local-<goos>-<goarch>.txt'            match ✓
README.md grep 'go run ./test/bench/cmd/benchgate'               match ✓
README.md grep 'CONTRIBUTING.md'                                 match ✓
README.md grep 'three-step rollout'                              no match ✓
README.md grep 'capture-baseline.yml'                            no match ✓
README.md grep 'warn-only'                                       no match ✓
```

Plan 50-02 owns D-A13 bullets 1, 3, 4, 5 (CONTRIBUTING.md sections, PROJECT.md tech-debt rewrite, REQUIREMENTS.md TOOL-02 cancellation, the PR's own go-test.yml green run).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Plan-internal contradiction between verbatim README content and acceptance grep**

- **Found during:** Task 2.
- **Issue:** The plan supplied the verbatim README content to write, which included the sentence "two `capture-baseline.yml` runs failed on this exact symptom in April 2026". The same plan also asserted in both the automated `<verify>` block and `<acceptance_criteria>` that `grep -q 'capture-baseline.yml' test/bench/baselines/README.md` MUST return non-zero — i.e. the literal filename must NOT appear in the README. The two requirements are mutually exclusive as written.
- **Fix:** Replaced the offending phrase "two `capture-baseline.yml` runs failed" with "two CI baseline-capture runs failed". The pivot rationale prose (April 2026 capture failures, RESEARCH.md Pitfall 5 cross-reference, "the strategy was the problem, not the timeouts") is preserved intact; only the literal `.yml` filename was generalized to "CI baseline-capture runs". This honors both the spirit of the verbatim content (rationale + cross-references) and the acceptance constraint (no leftover references to the deleted workflow filename).
- **Files modified:** `test/bench/baselines/README.md`.
- **Commit:** `8f71fde9`.

### Authentication Gates

None — plan was pure file deletion + doc rewrite, no external services involved.

## Commits

| Task | Commit | Message |
|------|--------|---------|
| 1 | `f96d675a` | `chore(50-01): remove CI bench gate workflows` |
| 2 | `8f71fde9` | `docs(50-01): rewrite test/bench/baselines/README.md for local-first workflow` |

## Self-Check: PASSED

- `.github/workflows/bench.yml`: MISSING on disk (expected — staged deletion committed) ✓
- `.github/workflows/capture-baseline.yml`: MISSING on disk (expected) ✓
- `test/bench/baselines/README.md`: FOUND, contains required strings ✓
- `.github/workflows/go-test.yml`: FOUND, unmodified ✓
- All four historical baseline `.txt` files: FOUND ✓
- Commit `f96d675a`: FOUND in `git log` ✓
- Commit `8f71fde9`: FOUND in `git log` ✓
