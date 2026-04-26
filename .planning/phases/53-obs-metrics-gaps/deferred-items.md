# Phase 53 Deferred Items

Out-of-scope discoveries logged during plan execution. None of these are
blockers for Phase 53 success criteria; they are pre-existing failures
unrelated to the metrics-wiring work.

## Pre-existing test failures

### test/integration/TestSymbols_JavaFixture (`get_hover_info`, `find_references_cross_file`)

- **Discovered during:** Plan 53-03 final `go test ./...` gate.
- **Verification:** Failure reproduces on the parent commit `1e0f7e00`
  before any Plan 53-03 Task 3 changes were applied (verified via
  `git stash && go test ./test/integration/ -run TestSymbols_JavaFixture`).
- **Surface:** jdtls integration — hover returns empty, cross-file
  references find nothing for the fixture project.
- **Out of scope:** This Phase modifies metric wiring only; no edits to
  symbol retrieval, jdtls adapter, or the Java fixture. Phase 56 owns
  jdtls readiness.

### test/integration/TestEdit_JavaFixture (`replace_body`)

- **Discovered during:** same gate as above.
- **Surface:** `replace_symbol_body` returns `not_found: symbol not found
  (helper)` for the Java fixture — a downstream consequence of the same
  jdtls readiness issue.
- **Out of scope:** as above.

## Follow-ups from Phase 53 code review

### IN-01: Rename `phase="timeout"` to `phase="worker_idle_evicted"`

- **Source:** `.planning/phases/53-obs-metrics-gaps/53-REVIEW.md` IN-01.
- **Issue:** `serena_session_lifecycle_total{phase="timeout"}` actually
  records *worker-level idle eviction*, not user-session timeout. The
  metric label is misleading. Phase 53 strengthened the USAGE.md warning
  (iteration 2 fix) but did not rename the enum value.
- **Out of scope:** Renaming requires updating the closed-enum guard in
  `internal/obs/metrics.go`, the `kernel.PhaseTimeout` constant, the
  cardinality cap test, USAGE.md, and any downstream dashboards/alerts.
  Tracked for Phase 54+ as a coordinated rename.

### IN-06: Pointer-receiver discipline for `RenameOverrider`

- **Source:** `.planning/phases/53-obs-metrics-gaps/53-REVIEW.md` IN-06.
- **Issue:** `defaultOverriderResolver` in `internal/kernel/edit/rename.go`
  uses `any(adapter).(RenameOverrider)` plus a special-case for
  `*lspool.RustAnalyzerAdapter`. A future adapter that implements
  `RenameOverrider` only on a pointer receiver but is returned by value
  from `lease.Adapter()` will silently fall through to the
  RustAnalyzer-only branch, returning nil and degrading rename to
  no-fallback.
- **Out of scope:** Pre-dates Phase 53 and the reviewer explicitly tagged
  it "Out of scope for Phase 53 to fix". Captured here so future adapter
  authors add `var _ RenameOverrider = (*MyAdapter)(nil)` compile-time
  assertions and prefer pointer-receiver implementation.

## Pre-existing gofmt drift (untouched files)

- `internal/cli/{activate,deactivate,nudge,nudge_test,setup}.go`
- These files are not modified by Phase 53; reformatting is out of scope
  per executor SCOPE BOUNDARY (Rule). A future cleanup phase can run
  `gofmt -w internal/cli/` in isolation.
