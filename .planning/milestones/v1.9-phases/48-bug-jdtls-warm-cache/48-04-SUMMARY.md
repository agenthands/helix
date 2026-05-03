---
phase: 48
plan: 04
subsystem: build-orchestration
tags: [makefile, build, jdtls, bench]
requires: []
provides:
  - "make clean-jdtls-cache: idempotently wipes $XDG_CACHE_HOME/serena-test/jdtls/"
  - "make bench-jdtls-warm: runs Java integration suite cold-then-warm with wall-clocks"
affects:
  - Makefile
tech-stack:
  added: []
  patterns:
    - "POSIX shell expansion `$${XDG_CACHE_HOME:-$$HOME/.cache}` for cross-platform cache root"
    - "Recursive `$(MAKE) clean-jdtls-cache` from bench target to share cleanup logic"
    - "`## description` help-text convention (matches existing `docs` target)"
key-files:
  created: []
  modified:
    - Makefile
decisions:
  - "Append-only Makefile edit: existing targets (build/clean/proto/test/vet/fmt/docs) byte-identical post-change"
  - "Single-line .PHONY (per acceptance criteria) extended with both new target names"
  - "Bench target targets ONLY `-run 'Java'` (not the full suite) to keep cold/warm delta jdtls-specific"
  - "No `-tags=integration` in bench target — Plan 03 will drop the tag from java_test.go; until then `go test` reports 'no tests to run' which is acceptable per plan interface contract"
  - "POSIX-only Makefile target; Windows users invoke via Git Bash (documented in Plan 05 USAGE.md)"
metrics:
  duration_seconds: 180
  tasks_completed: 1
  files_created: 0
  files_modified: 1
  completed_date: "2026-04-25"
---

# Phase 48 Plan 04: Makefile clean-jdtls-cache + bench-jdtls-warm Summary

Two new `.PHONY` Makefile targets satisfying CONTEXT decisions D-07 (manual cache wipe) and D-12 (cold/warm benchmark): `clean-jdtls-cache` idempotently wipes `$XDG_CACHE_HOME/serena-test/jdtls/`, and `bench-jdtls-warm` recursively invokes the clean target then runs the Java integration suite twice under `time` to surface the wall-clock delta required by D-14.

## Objective Achieved

- [x] `clean-jdtls-cache` deletes `$XDG_CACHE_HOME/serena-test/jdtls/` (or `$HOME/.cache/serena-test/jdtls/` fallback) and exits 0
- [x] `bench-jdtls-warm` runs the Java suite twice — cold then warm — and prints both wall-clocks
- [x] Both targets are listed in `.PHONY` (single-line, no shadowed file)
- [x] Both targets carry `## description` help text matching the existing `docs` target style
- [x] Existing targets (`build`, `clean`, `proto`, `test`, `vet`, `fmt`, `docs`) untouched (verified via `git diff`)

## Tasks Completed

| Task | Name                                                          | Commit   | Files    |
| ---- | ------------------------------------------------------------- | -------- | -------- |
| 1    | Append clean-jdtls-cache and bench-jdtls-warm to Makefile     | fc1b847b | Makefile |

## Verification

```
$ make -n clean-jdtls-cache
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/serena-test/jdtls"
echo "cleared jdtls warm cache at ${XDG_CACHE_HOME:-$HOME/.cache}/serena-test/jdtls"

$ make -n bench-jdtls-warm
/Library/Developer/CommandLineTools/usr/bin/make clean-jdtls-cache
rm -rf "${XDG_CACHE_HOME:-$HOME/.cache}/serena-test/jdtls"
echo "cleared jdtls warm cache at ${XDG_CACHE_HOME:-$HOME/.cache}/serena-test/jdtls"
echo "=== jdtls COLD run ==="
time go test -run 'Java' ./test/integration/... -count=1
echo "=== jdtls WARM run ==="
time go test -run 'Java' ./test/integration/... -count=1

$ make clean-jdtls-cache
cleared jdtls warm cache at /Users/Janis_Vizulis/.cache/serena-test/jdtls

$ mkdir -p "$HOME/.cache/serena-test/jdtls/sentinel"
$ make clean-jdtls-cache
cleared jdtls warm cache at /Users/Janis_Vizulis/.cache/serena-test/jdtls
$ test ! -d "$HOME/.cache/serena-test/jdtls/sentinel"
# OK — sentinel removed

$ XDG_CACHE_HOME=/tmp/testxdg make clean-jdtls-cache
cleared jdtls warm cache at /tmp/testxdg/serena-test/jdtls
# Verified XDG override resolves at runtime via /bin/sh expansion

$ grep -c '^\.PHONY:.*clean-jdtls-cache' Makefile
1
$ grep -c '^\.PHONY:.*bench-jdtls-warm' Makefile
1
$ grep -c '^clean-jdtls-cache:' Makefile
1
$ grep -c '^bench-jdtls-warm:' Makefile
1
$ awk '/^\.PHONY:/' Makefile | wc -l
       1
```

Recipe lines verified to begin with literal TAB characters via `cat -et Makefile` (lines marked `^I`).

### Acceptance Criteria

- [x] `grep -c '^clean-jdtls-cache:' Makefile` == 1
- [x] `grep -c '^bench-jdtls-warm:' Makefile` == 1
- [x] Single-line .PHONY containing both names (verified via `awk` count == 1)
- [x] `make -n clean-jdtls-cache` includes `rm -rf` + `serena-test/jdtls`
- [x] `make -n bench-jdtls-warm` prints two `go test -run 'Java'` invocations and invokes `clean-jdtls-cache` first
- [x] `make clean-jdtls-cache` exits 0 when dir absent AND when it contains files (sentinel test)
- [x] All recipe lines indented with literal TAB (verified via `cat -et`)
- [x] `XDG_CACHE_HOME=/tmp/testxdg` resolves to `/tmp/testxdg/serena-test/jdtls` at runtime
- [x] Existing targets unchanged — `git diff` shows ONLY the `.PHONY` line update + appended block

## Deviations from Plan

None — plan executed exactly as written. Recipe text matches PATTERNS.md lines 230-243 verbatim.

Note: the plan's verification command suggests `XDG_CACHE_HOME=/tmp/testxdg make -n clean-jdtls-cache` should show `/tmp/testxdg/...` in dry-run output. In practice, `make -n` prints recipe text without invoking `/bin/sh`, so the `${VAR:-default}` form is shown unexpanded. The expansion was verified by an actual `make clean-jdtls-cache` run with `XDG_CACHE_HOME` set, which printed `cleared jdtls warm cache at /tmp/testxdg/serena-test/jdtls`. Documented here so future readers do not chase the cosmetic dry-run output.

## Threat Model Compliance

All four threats in the plan's `<threat_model>` are either `mitigate` (T-48-04-01: path-append structure ensures `XDG_CACHE_HOME=/` resolves to `/serena-test/jdtls`, never `/`) or `accept` (T-48-04-02 sudo, T-48-04-03 manual-only, T-48-04-04 stdout). Implementation satisfies the single `mitigate` disposition exactly as planned — no additional surface introduced.

## Downstream Contract

Plan 05 (USAGE.md doc + CI) may now reference:

```
make clean-jdtls-cache    # idempotent; succeeds whether dir exists or not
make bench-jdtls-warm     # depends on clean; runs Java suite twice under time
```

Both targets are POSIX-only; Windows contributors invoke via Git Bash. The bench target intentionally scopes to `-run 'Java'` only — running the full suite twice would mask the jdtls-specific wall-clock delta.

## Self-Check: PASSED

- FOUND: Makefile (modified)
- FOUND commit: fc1b847b (feat 48-04 Makefile targets)
- VERIFIED: `make -n clean-jdtls-cache` exits 0 with rm -rf output
- VERIFIED: `make -n bench-jdtls-warm` exits 0 with cold+warm legs
- VERIFIED: `make clean-jdtls-cache` idempotent (sentinel test passed)
- VERIFIED: existing targets byte-identical (`git diff` shows only intended changes)
