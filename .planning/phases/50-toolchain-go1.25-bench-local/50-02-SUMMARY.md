---
phase: 50
plan: 02
subsystem: bench-local-make-surface
tags:
  - infra
  - build-tooling
  - benchmarks
dependencies:
  requires:
    - 50-01  # bench infra removal preceded this plan
  provides:
    - local-bench-make-targets
    - gitignored-local-baseline-path
    - local-only-baselines-readme
  affects:
    - Makefile
    - .gitignore
    - test/bench/baselines/
tech_stack:
  added: []
  patterns:
    - self-documenting-make-target (Makefile `bench-jdtls-warm` analog)
    - exact-path-gitignore (Pitfall 3 — avoid directory-prefix that would hide README.md)
key_files:
  created: []
  modified:
    - Makefile
    - .gitignore
    - test/bench/baselines/README.md
  deleted: []
key_decisions:
  - "Used `tee` form for `bench-baseline` (prints to stdout AND captures) per RESEARCH §Code Examples and Plan 02 Task 1 instruction; matches the deleted bench.yml's `tee` idiom."
  - "Inlined the `benchstat` install/invoke instructions in the README rather than cross-linking to CONTRIBUTING.md, per Plan 02 Task 3 mechanics ('inline is the chosen form')."
  - "Wrote the README content verbatim from Plan 02 Task 3 — no deviations from suggested wording."
metrics:
  duration_minutes: 6
  tasks_completed: 3
  files_modified: 3
  lines_added: 31
  lines_removed: 122
  completion_date: "2026-04-28"
requirements_satisfied:
  - TOOL-02 (second half — local-only Make surface, gitignore, README)
---

# Phase 50 Plan 02: Local Bench Make Surface Summary

Stood up the local-only bench command surface that replaces Wave 1's deleted hosted-CI plumbing: two new self-documenting Makefile targets (`bench`, `bench-baseline`), gitignored canonical baseline path (`test/bench/baselines/local.txt`), and a single-screen rewrite of `test/bench/baselines/README.md` for the local-only flow.

## Plan Goal

TOOL-02's second half (the first half was the Wave 1 deletions in Plan 01) — give contributors the simplest possible local interface for capturing and consulting bench baselines, locked under decisions D-05 (README local-only framing), D-07 (Make as the documented surface), D-08 (single fixed path, no `NAME=`), D-09 (gitignored canonical path).

## What Shipped

- 2 new Makefile targets in the `bench-jdtls-warm` self-doc style: `bench` (single-shot, stdout) and `bench-baseline` (tee → `test/bench/baselines/local.txt`)
- `.PHONY` extended to declare both targets
- 1-line `.gitignore` entry inside the existing `# Go` group: `test/bench/baselines/local.txt` (exact path, NOT directory prefix — Pitfall 3)
- `test/bench/baselines/README.md` fully rewritten (35 lines from a 134-line v1.1-CI-framing original) as a local-only-flow doc
- `go vet ./...` green after every commit (exit 0)
- `make -n bench` and `make -n bench-baseline` both expand correctly
- `git check-ignore test/bench/baselines/local.txt` returns 0; `git check-ignore test/bench/baselines/README.md` returns 1 (Pitfall 3 protection verified)

### Per-task commits

| Task | Commit | Subject |
|------|--------|---------|
| 1    | `4ac956ef` | feat(50-02): add bench and bench-baseline self-doc Makefile targets |
| 2    | `adf058c3` | chore(50-02): gitignore canonical local baseline path |
| 3    | `7c1d3154` | docs(50-02): rewrite test/bench/baselines/README.md for local-only flow |

## Diffs

### Task 1 — Makefile

```diff
 .PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm
+.PHONY: build clean proto test vet fmt docs clean-jdtls-cache bench-jdtls-warm bench bench-baseline
```

(shown as separate lines for clarity — actually one line replacement)

Appended after `bench-jdtls-warm`:

```make

bench: ## Run the bench suite once and print results to stdout
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/...

bench-baseline: ## Capture a local baseline into test/bench/baselines/local.txt (gitignored, overwrites)
	$(GO) test -short -bench=. -benchmem -count=10 -run=^$$ ./test/bench/... | tee test/bench/baselines/local.txt
```

Indentation: literal TAB (`^I` confirmed via `cat -et Makefile`).

### Task 2 — `.gitignore`

```diff
 # Go
 /serena
 *.exe
 /bin/
 /api/proto/**/*.pb.go
+test/bench/baselines/local.txt
```

Single line appended at the end of the existing `# Go` group. Exact path, NOT `test/bench/baselines/` or `test/bench/baselines/*`.

### Task 3 — `test/bench/baselines/README.md`

Full rewrite — 121 lines removed, 23 lines added, net 134 → 35 lines. Content matches Plan 02 Task 3 verbatim (no deviations in wording). Required tokens present: `local`, `gitignored` (twice). Forbidden tokens absent: `benchgate`, `bench.yml`, `capture-baseline.yml`, `github-hosted`.

## Chosen Baseline Path

`test/bench/baselines/local.txt` — verified ignored:

```
$ git check-ignore test/bench/baselines/local.txt
test/bench/baselines/local.txt
$ echo $?
0

$ git check-ignore test/bench/baselines/README.md
$ echo $?
1   # NOT ignored (Pitfall 3 protection — README is preserved)
```

## Verification Results

```
=== Makefile gates ===
.PHONY contains bench bench-baseline:  ok
make -n bench:                         ok (resolves to: go test -short -bench=. -benchmem -count=10 -run=^$ ./test/bench/...)
make -n bench-baseline:                ok (resolves to: go test ... | tee test/bench/baselines/local.txt)

=== Gitignore correctness ===
git check-ignore test/bench/baselines/local.txt:    exit 0 (ignored)
git check-ignore test/bench/baselines/README.md:    exit 1 (NOT ignored — Pitfall 3 ok)
no directory-prefix pattern in .gitignore:          ok

=== README content gates ===
local:                  present
gitignored:             present (twice)
make bench-baseline:    documented
benchgate:              absent
bench.yml:              absent
capture-baseline.yml:   absent
github-hosted:          absent

=== Build green ===
go vet ./...:           exit 0
```

(Out-of-scope CGO warning observed in `internal/treesitter/bindings/swift/src/scanner.c` — `TOKEN_COUNT` macro redefined; pre-existing in the worktree base, not caused by this plan, not fixed here per scope-boundary.)

## Deviations from Plan

None — plan executed exactly as written. The plan's Task 3 was the only place where some inline judgement was permissible (cross-link vs. inline benchstat instructions); the plan itself stated "inline is the chosen form above" so no deviation.

## Authentication Gates

None — fully autonomous Makefile/.gitignore/README edits, no external services touched.

## Auto-fixed Issues

None — no Rule 1/2/3 deviations triggered. Plan instructions were sufficient.

## Threat Model — applied mitigations

| Threat ID | Disposition | Verified By |
|-----------|-------------|-------------|
| T-50-04 (Information Disclosure — gitignore shape) | mitigate | `git check-ignore test/bench/baselines/README.md` returns exit 1 (Task 2 verification) |
| T-50-05 (Tampering — Make targets) | accept | Recipes use `$(GO)` only, no user-controlled interpolation, no privileged writes outside the gitignored target file |
| T-50-06 (Repudiation — overwriting local.txt) | accept | Per D-08 explicit trade-off; `NAME=` deferred |

No new threat surface introduced beyond what the threat model anticipated.

## Self-Check: PASSED

- `Makefile` exists and contains the two new targets in the `bench-jdtls-warm` self-doc style: confirmed (`grep -E '^bench: ## ' Makefile`, `grep -E '^bench-baseline: ## ' Makefile`)
- `.gitignore` line `test/bench/baselines/local.txt` present in `# Go` group: confirmed (`grep -F` exit 0)
- `test/bench/baselines/README.md` is the single-screen local-only-flow rewrite: confirmed (35 lines, all token gates pass)
- `make -n bench` and `make -n bench-baseline` both succeed: confirmed
- `go vet ./...` exit 0: confirmed
- Commits `4ac956ef`, `adf058c3`, `7c1d3154` exist on the worktree branch: confirmed via `git log --oneline -5`

## Notes for Plan 03 / Phase Wrap

- Plan 03 still needs to clean the surviving `bench.yml` / `capture-baseline.yml` mentions in `CONTRIBUTING.md`, `USAGE.md`, `CHANGELOG.md`, and the `// upload-artifact` comment in `test/bench/memory_bench_test.go:16` so the post-phase canonical grep is empty across active code/docs/build (Plan 01 SUMMARY catalogued these).
- The new `make bench` / `make bench-baseline` targets are documented in `test/bench/baselines/README.md` here, but Plan 03's CONTRIBUTING.md edits should reference them in the "Running Benchmarks" section per the cross-link this README points to ("See `CONTRIBUTING.md` → 'Running Benchmarks' for the full rationale.").
- No further deferred items.
