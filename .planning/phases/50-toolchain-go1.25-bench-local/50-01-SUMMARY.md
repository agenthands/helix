---
phase: 50
plan: 01
subsystem: ci-bench-cleanup
tags:
  - infra
  - ci
  - cleanup
  - benchmarks
dependencies:
  requires: []
  provides:
    - clean-tree-no-hosted-bench
    - clean-tree-no-benchgate-references
  affects:
    - .github/workflows/
    - test/bench/cmd/
    - test/bench/baselines/
tech_stack:
  added: []
  patterns:
    - deletion-only (PATTERNS.md "Deletions (no analog)")
key_files:
  created: []
  modified:
    - test/bench/baselines/README.md
  deleted:
    - .github/workflows/bench.yml
    - .github/workflows/capture-baseline.yml
    - test/bench/cmd/benchgate/main.go
    - test/bench/cmd/benchgate/main_test.go
    - test/bench/baselines/v1.1-github-hosted.txt
    - test/bench/baselines/v1.2-phase10-github-hosted.txt
    - test/bench/baselines/v1.2-phase11-github-hosted.txt
    - test/bench/baselines/v1.2-phase12-github-hosted.txt
key_decisions:
  - "Surgically scrubbed `benchgate` strings from test/bench/baselines/README.md to satisfy Task 3 grep acceptance, instead of leaving the README untouched as Task 2 suggested. Plan 02 still rewrites the file in full."
metrics:
  duration_minutes: 7
  tasks_completed: 3
  files_deleted: 8
  files_modified: 1
  lines_removed: 1512
  lines_added: 8
  completion_date: "2026-04-28"
requirements_satisfied:
  - TOOL-02 (first half — bench infra removal)
---

# Phase 50 Plan 01: Bench Infra Removal Summary

Atomic teardown of all hosted-CI bench plumbing: 2 GitHub workflows, the `benchgate` Go CLI package, and 4 committed `*-github-hosted.txt` baselines deleted; `benchgate` grep across active code/docs/build is now empty.

## Plan Goal

Tear down all hosted-CI bench plumbing in one atomic plan per Phase 50 D-02/D-03/D-04, leaving `.github/workflows/go-test.yml` as the only Go CI gate. Plan 02 (Wave 2) adds the local Make targets and rewrites `test/bench/baselines/README.md`; Plan 03 cleans the surviving `bench.yml` / `capture-baseline.yml` mentions in CONTRIBUTING.md, USAGE.md, CHANGELOG.md, and `test/bench/memory_bench_test.go`.

## What Shipped

- 8 files deleted (177 lines from workflows + 1327 lines from benchgate sources & hosted baselines + 8 lines of stale benchgate prose in README)
- `test/bench/baselines/` directory and `README.md` preserved (Plan 02 rewrites README)
- `.github/workflows/go-test.yml` untouched (Phase 50 success criterion)
- `test/bench/tools_bench_test.go`, `test/bench/memory_bench_test.go`, `test/bench/rss/` bench suite preserved
- `go vet ./...` and `go test ./test/bench/... -count=1 -short` green after every commit

### Per-task commits

| Task | Commit | Subject |
|------|--------|---------|
| 1    | `69dc6e53` | chore(50-01): delete hosted-CI bench workflows |
| 2    | `07e53183` | chore(50-01): delete benchgate package and hosted-CI baselines |
| 3    | `58215f57` | chore(50-01): scrub benchgate references from baselines/README.md |

### Files deleted

| Path | Reason |
|------|--------|
| `.github/workflows/bench.yml` | Hosted-CI PR-tier benchstat regression gate (D-02 forbids on hosted CI) |
| `.github/workflows/capture-baseline.yml` | Hosted-CI baseline capture workflow (D-02 forbids) |
| `test/bench/cmd/benchgate/main.go` | In-tree benchstat regression-gate CLI (D-03 — moved to local Makefile target in Plan 02) |
| `test/bench/cmd/benchgate/main_test.go` | Same |
| `test/bench/baselines/v1.1-github-hosted.txt` | Committed CI-runner baseline (D-04 forbids "historical-local" preservation) |
| `test/bench/baselines/v1.2-phase10-github-hosted.txt` | Same |
| `test/bench/baselines/v1.2-phase11-github-hosted.txt` | Same |
| `test/bench/baselines/v1.2-phase12-github-hosted.txt` | Same |

### Files modified

| Path | Change |
|------|--------|
| `test/bench/baselines/README.md` | 6 inline `benchgate` references rewritten to "the regression gate" or removed; structure intact for Plan 02 to fully rewrite |

## Verification Results

```
=== Deletion checks ===
bench.yml:                    GONE
capture-baseline.yml:         GONE
test/bench/cmd/benchgate/:    GONE
test/bench/baselines/v1.*-github-hosted.txt: GONE

=== Reference cleanliness ===
benchgate canonical grep (active code/docs/build):  EMPTY (exit 1)

=== Build green ===
go vet ./...:                 exit 0
go test ./test/bench/... -count=1 -short:  PASS
```

### Surviving `bench.yml` / `capture-baseline.yml` mentions (catalogue for Plan 03)

These are explicitly allowed by the plan to survive Wave 1 — Plan 03 cleans them:

| File | Line | Snippet |
|------|------|---------|
| `CHANGELOG.md` | 155 | `capture-baseline.yml` workflow for on-demand baseline capture on GitHub-hosted runners |
| `USAGE.md` | 529 | re-baseline benchmarks using the `capture-baseline.yml` CI workflow ... |
| `CONTRIBUTING.md` | 150 | CI runs the benchstat regression gate via `.github/workflows/bench.yml`. |
| `CONTRIBUTING.md` | 182 | `.github/workflows/bench.yml` runs on PRs ... |
| `CONTRIBUTING.md` | 184 | `.github/workflows/capture-baseline.yml` captures new baselines ... |
| `test/bench/memory_bench_test.go` | 16 | comment: `upload-artifact` step in `.github/workflows/bench.yml` |
| `test/bench/baselines/README.md` | 13, 41, 46, 52, 54, 57, 95, 103, 120 | bench.yml + capture-baseline.yml in workflow procedure prose |

Plan 02 rewrites `test/bench/baselines/README.md` end-to-end as a local-only baselines doc; Plan 03 sweeps the four top-level docs and the comment in `memory_bench_test.go`.

## Deviations from Plan

### [Rule 3 — blocking issue] Scrubbed `benchgate` from `test/bench/baselines/README.md`

- **Found during:** Task 3
- **Issue:** Task 2 said "Do NOT touch test/bench/baselines/README.md here — Plan 02 rewrites it." Task 3 said "If the `benchgate` grep returns ANY hit outside .planning/ and _superseded/ and legacy/, that hit MUST be removed (it indicates Task 2 missed a reference); add it to the deletion or fail the plan." `test/bench/baselines/README.md` had 6 surviving `benchgate` references. The two directives conflict; Task 3 grep is the load-bearing acceptance criterion (Phase 50 success criterion: "The `benchgate` grep returns no matches in active code/docs/build files").
- **Fix:** Surgical 8-line edit (8 in / 8 out) replacing each `benchgate` token with neutral phrasing ("the regression gate") or removing the bullet entirely. Structure of the README is preserved for Plan 02 full rewrite.
- **Files modified:** `test/bench/baselines/README.md`
- **Commit:** `58215f57`

### [Tooling note — non-deviation] Edit/Write tool sandboxing on this worktree

While executing Task 3 the `Edit` and `Write` tools reported success but did not persist changes to disk (verified via `git diff` and `xxd`/`stat`). The fix was applied via a `python3 -c "..."` Bash invocation that read the file and wrote the edited content. After the python edit, `git diff --stat` confirmed real changes; the resulting commit (`58215f57`) is byte-correct. The same workaround was used to write this SUMMARY file.

Recorded as a tooling observation, not a Rule deviation — no design decision was changed.

### Out-of-scope discoveries (not fixed, none deferred)

None observed beyond the catalogue above (which is the plan explicit Plan 03 hand-off).

## Authentication Gates

None — fully autonomous deletion plan, no external services touched.

## Auto-fixed Issues

| Rule | Issue | Resolution |
|------|-------|------------|
| Rule 3 | `benchgate` grep would not pass with README.md untouched | See deviation above; surgical scrub via `python3` (Edit tool sandboxed). |

## Self-Check: PASSED

- `! test -f .github/workflows/bench.yml`: confirmed
- `! test -f .github/workflows/capture-baseline.yml`: confirmed
- `! test -d test/bench/cmd/benchgate`: confirmed
- `! ls test/bench/baselines/v1.*-github-hosted.txt`: confirmed
- `grep -rn 'benchgate' --include='*.go' --include='*.md' --include='*.yml' --include='*.yaml' --include='Makefile' --exclude-dir={_superseded,.planning,legacy} .`: empty (exit 1)
- `go vet ./...`: exit 0
- `go test ./test/bench/... -count=1 -short`: PASS
- Commits `69dc6e53`, `07e53183`, `58215f57` exist in `git log`

Pre-existing failures observed (out of scope per scope-boundary rule):

- `test/integration` `TestSymbols_JavaFixture` and `TestEdit_JavaFixture` fail at the plan base commit `a3796d44` too — they are jdtls cold-start flakes documented in `PROJECT.md` "Known tech debt" and gated by `SERENA_TEST_LS_TIMEOUT` in CI. Not caused by this plan deletions; not fixed here.

## Notes for Plan 02 / Plan 03

- Plan 02 must rewrite `test/bench/baselines/README.md` end-to-end as a local-only baselines doc (10 surviving `bench.yml` / `capture-baseline.yml` mentions in this file).
- Plan 03 must clean the 6 mentions in CONTRIBUTING.md, USAGE.md, CHANGELOG.md, and the `// upload-artifact` comment in `test/bench/memory_bench_test.go:16` so the post-phase canonical grep `bench\.yml|capture-baseline\.yml` is empty across active code/docs/build.
