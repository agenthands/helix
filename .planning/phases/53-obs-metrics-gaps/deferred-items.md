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

## Pre-existing gofmt drift (untouched files)

- `internal/cli/{activate,deactivate,nudge,nudge_test,setup}.go`
- These files are not modified by Phase 53; reformatting is out of scope
  per executor SCOPE BOUNDARY (Rule). A future cleanup phase can run
  `gofmt -w internal/cli/` in isolation.
