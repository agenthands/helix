# Phase 51 — Deferred Items

Out-of-scope discoveries logged during plan execution. Not addressed by Phase 51 because they are pre-existing and unrelated to the goreleaser pipeline.

## Pre-existing test failures (Java LSP environment)

- **TestSymbols_JavaFixture/get_hover_info** — `get_hover_info` returns `{}` (jdtls returns empty hover for `helper`).
- **TestSymbols_JavaFixture/find_references_cross_file** — cross-file reference to `Greeter` not found.
- **TestEdit_JavaFixture/replace_body** — `replace_symbol_body` reports `not_found` for `helper`.

**Verification:** All three failures reproduce on the plan's base commit (`75c9afde`) before any Phase 51 edits — confirmed via `git stash -u && go test ./test/integration -run TestSymbols_JavaFixture`. Root cause appears to be a `jdtls` indexing/timing issue in this worktree's environment, not a regression introduced by Phase 51.

**Disposition:** Out of Phase 51 scope (Phase 51 = release pipeline only). Should be filed as a separate jdtls/integration-test ticket.
