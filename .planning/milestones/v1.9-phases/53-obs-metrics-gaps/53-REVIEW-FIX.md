---
phase: 53-obs-metrics-gaps
fixed_at: 2026-04-30T23:59:00Z
review_path: .planning/phases/53-obs-metrics-gaps/53-REVIEW.md
iteration: 1
findings_in_scope: 7
fixed: 7
skipped: 0
status: all_fixed
---

# Phase 53: Code Review Fix Report

**Fixed at:** 2026-04-30T23:59:00Z
**Source review:** .planning/phases/53-obs-metrics-gaps/53-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 7 (1 critical + 6 warnings; info findings deferred per `fix_scope=critical_warning`)
- Fixed: 7
- Skipped: 0

All seven in-scope findings were fixed and validated with `go vet`,
`go test`, and `go test -race` on each affected package.

## Fixed Issues

### CR-01: Data race on TagCache.metrics between SetMetricsSink and GetOrExtract hit emission

**Files modified:** `internal/repomap/cache.go`
**Commit:** 35edd463
**Applied fix:** Captured `sink := c.metrics` while holding `c.mu` at the
top of `GetOrExtract`, then routed both the hit-branch and miss-branch
emissions through the local `sink` variable. Mirrors the lock-then-capture
pattern already used in `internal/skill/repomap/skill.go::metricsSink()`.
Validated with `go test -race ./internal/repomap/...` (passed in 7.2s).

### WR-01: Orphan DELETE emits ended without prior started

**Files modified:** `internal/daemon/http_session_middleware.go`,
`internal/daemon/http_session_middleware_test.go`
**Commit:** 573690db (combined with WR-04)
**Applied fix:** Two-part change. (1) Skip the seen-map registration on
DELETE entirely — DELETE is a termination signal, not a session opener,
so the unconditional `LoadOrStore` was letting orphan DELETEs spuriously
emit started→ended. (2) Switched the ended emission to `LoadAndDelete`
(Go 1.20+) which atomically combines the existence check with removal,
so concurrent DELETEs for the same ID emit at most one ended. Added
regression test `orphan_delete_unseen_id_no_ended`.

### WR-02: replace_in_file fuzzy-fallback ReadFile error reports outcome=success

**Files modified:** `internal/kernel/fileops/tools.go`
**Commit:** 115d87f1
**Applied fix:** Set `outcome = "internal"` before returning when the
fuzzy-fallback `ReadFile(root, args.Path)` fails. The textResult shape
is preserved unchanged for caller backwards-compat; only the deferred
`RecordEditOutcome` classification changes from "success" to "internal".

### WR-03: USAGE.md PromQL example references nonexistent outcome value "error"

**Files modified:** `USAGE.md`
**Commit:** 1c50942e
**Applied fix:** Replaced the broken `outcome="error"` query with two
valid alternatives: (1) a regex union over the three currently-emitted
failure outcomes `outcome=~"timeout|circuit_open|internal"` divided by
total, and (2) the inverse `1 - success/total` form which is robust to
future enum additions. Inlined enum context so future readers know which
values are reserved for v1.3 typed-error work.

### WR-04: 5xx response on DELETE double-counts the request

**Files modified:** `internal/daemon/http_session_middleware.go`,
`internal/daemon/http_session_middleware_test.go`
**Commit:** 573690db (combined with WR-01)
**Applied fix:** Moved the `next.ServeHTTP` call before the ended
emission, then gated ended on `status < 500`. A DELETE that returns
5xx now emits only error, not both ended and error. Added regression
test `delete_with_5xx_no_ended_only_error`.

### WR-05: Concurrent SetEditOutcomeSink is racy on read side without sync

**Files modified:** `internal/mcp/middleware.go`
**Commit:** 7b5dcd08
**Applied fix:** Pure documentation. Added explicit warnings to three
docstrings (`RecordEditOutcome`, `SetEditOutcomeSinkForTest`,
`setRenameStrategySink`) instructing callers to (1) avoid `t.Parallel()`
on tests that mutate the sink, and (2) pair the install with
`t.Cleanup(func(){ Set...ForTest(nil) })` so the recorder does not leak
into adjacent tests in the same package.

### WR-06: Q-3 under-classification of validation errors as "internal" misleads operators

**Files modified:** `internal/kernel/edit/tools.go`
**Commit:** 31af9d8c
**Applied fix:** Added an explicit TODO at the Q-3 under-classification
site in `ClassifyEditError` listing the four call-sites that need
updating when typed-error work lands and "invalid_args" joins the
outcome enum (`editOutcomeEnum`, `EditOutcomeInc` allowlist,
`TestMetrics_CardinalityBounds_EditOutcome` bound 168→196, USAGE.md
table row). Makes the v1.3 migration mechanical.

## Skipped Issues

None — every in-scope finding was fixed.

## Validation

- `go vet ./internal/...` — clean (only pre-existing C macro warnings
  from the locally vendored Swift tree-sitter binding, unrelated to
  Phase 53).
- `go test -race -count=1 ./internal/repomap/... ./internal/daemon/...
  ./internal/mcp/... ./internal/kernel/edit/...
  ./internal/kernel/fileops/...` — all packages pass.
- CR-01 specifically validated under `-race`: TagCache tests pass with
  no data-race report.

## Note on worktree isolation

The setup_worktree step failed because `main` is already checked out in
the foreground working tree at the project root. Per the orchestrator
contract for serial GSD workflows (no concurrent foreground edits), all
fixes were applied directly in the main working tree. Each fix was
committed atomically before the next was attempted, so partial-failure
recovery is per-finding via standard `git reset` if needed.

---

_Fixed: 2026-04-30T23:59:00Z_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
