---
phase: 57-semantic-store-foundation-pipeline-dag-library
plan: 05
fix_kind: follow_up
parent_review: 57-REVIEW-57-05.md
fixed_at: 2026-05-06T00:00:00Z
iteration: 1
findings_in_scope: 7
fixed: 7
skipped: 0
status: all_fixed
---

# Phase 57 Plan 05 — Review-Fix Report

**Source review:** `57-REVIEW-57-05.md`
**Fix scope:** `--all` (1 BLOCKER + 3 WARNING + 3 INFO)
**Iteration:** 1

## Summary

| Total in scope | Fixed | Skipped | Status |
|---|---|---|---|
| 7 | 7 | 0 | all_fixed |

## Per-finding outcomes

| Finding | Status | Commit | Notes |
|---|---|---|---|
| BL-01 | FIXED | `5d81645f` (RED) + `37e11ad8` (GREEN) | TDD pair — added `filepath.IsAbs` guard before existing `..` scan in `store.Open`; reworked test scaffolding (`configFor` test helper now chdirs to wsDir + returns relative path); 9 daemon/lspenrich/store test files updated to use workspace-relative paths under `t.Chdir`. |
| WR-NEW-01 | FIXED | `02c7313b` | Added closed-enum classifier `classifySemanticProbeError` mapping probe errors to `probe_timeout` / `nil_handle` / `db_error` / `unknown`; raw `err.Error()` no longer escapes into the MCP-exposed `Reason` field; full error logged via `slog.Warn` for operators. |
| WR-NEW-02 | FIXED | `b115fa00` | Added `decideReopenRetry` pure-function helper switching on second-attempt classification: persistent-transient → hard fail (preserve clean DB), corruption/unknown → quarantine. Unit-tested via `TestDecideReopenRetry_PersistentTransientHardFails`. |
| WR-NEW-03 | FIXED | `269a51ab` | Added `testdata/src/github.com/duckdb/duckdb-go-sibling/v2/sibling.go` stub package; extended `siblingpkg/imports.go` to import BOTH bare and `/v2` forms; expanded the existing `TestAnalyzer_AllowsSiblingNamespacePackage` to lock the slash boundary in both directions in a single fixture. |
| IN-NEW-01 | FIXED | `8a0152f2` | Documented `Store.DB()` cross-build contract (nil iff store not open; on windows/arm64 always nil because no DuckDB build); refreshed the windows/arm64 stub comment to cross-reference. Doc-only — no behaviour changes. |
| IN-NEW-02 | FIXED | `b453b91b` | Removed unreachable `db == nil` branch in `semanticStoreProbe.Probe`; wrapped surviving `p.s == nil` branch with `serr.ErrUnsupported` so the closed-enum classifier (WR-NEW-01) can disambiguate "feature disabled" from a transient DB error. |
| IN-NEW-03 | FIXED | `c637f0f3` | Added `TestApplyMigration001_UsesTransactionalHelper` build-time grep gate asserting `migrations.go` references `applyStatementsTx` ≥ 2 times (definition + applyMigration001 call). Coarse sentinel for future refactors that decouple the two. |

## Fixed Issues — detail

### BL-01: CR-01 absolute-path rejection gap

**Files modified:**
- `internal/semantic/store/duckdb.go` — added `filepath.IsAbs` guard
- `internal/semantic/store/duckdb_test.go` — RED test + reworked happy-path
- `internal/semantic/store/store_test.go` — `configFor` now takes `*testing.T` and chdirs
- `internal/semantic/store/migrations_test.go`, `overlay_test.go` — updated callers + sanity check
- `internal/daemon/daemon_semantic_test.go`, `daemon_grammar_test.go`, `daemon_extraction_test.go`, `live_e2e_test.go` — chdir + relative paths
- `internal/semantic/lspenrich/cascade_overlay_epoch_test.go` — chdir + relative path

**Commits:**
- `5d81645f` — `test(57-05): RED — assert Open rejects absolute paths (BL-01)`
- `37e11ad8` — `fix(57-05): reject absolute paths in store.Open per CR-01 contract (BL-01)`

**Applied fix:** Added `if filepath.IsAbs(path) { return nil, ...ErrInvalidArgs }` BEFORE the existing `..` segment scan in `store.Open`. The absolute-path guard runs first so an absolute path containing `..` surfaces the more specific "must be workspace-relative" error. All store/daemon/lspenrich tests that previously passed `filepath.Join(t.TempDir(), …)` now `t.Chdir(wsDir)` and pass the relative form `.helix/semantic.duckdb`.

**Followed prompt option (a)** — honor the doc rather than weaken it.

### WR-NEW-01: Probe.Reason raw error leak

**Files modified:**
- `internal/kernel/health/tools.go` — added closed enum + `classifySemanticProbeError`; updated `ComputeSemanticStoreStatus` to map and log via `slog.Warn`
- `internal/kernel/health/tools_semantic_test.go` — added `fakeProbeWithErr` helper + 3 tests covering each enum bucket; tightened existing unhealthy test to assert exact bucket and reject raw text leak

**Commit:** `02c7313b` — `fix(57-05): close-enum semantic_store reason field; never leak raw err text (WR-NEW-01)`

**Applied fix:** Reason field now always one of `probe_timeout` / `nil_handle` / `db_error` / `unknown`. Sentinel substring matches recognise the daemon-side `"DB handle nil"` and `"store unavailable"` strings. Raw error logged via `slog.Warn` with the structured `reason` field for operator triage.

### WR-NEW-02: Persistent transient quarantines clean DB

**Files modified:**
- `internal/semantic/store/duckdb.go` — added `reopenRetryDecision` enum + `decideReopenRetry` pure function; Open hot-path now `switch`es on the decision
- `internal/semantic/store/duckdb_internal_test.go` — added `TestDecideReopenRetry_PersistentTransientHardFails` covering all 4 transient sentinels + corruption + unknown

**Commit:** `b115fa00` — `fix(57-05): refuse to quarantine clean DB on persistent-transient reopen (WR-NEW-02)`

**Applied fix:** Followed prompt option (a) from REVIEW-57-05.md. The pure-function shape sidesteps the need to inject a fake `openExisting` constructor seam and keeps the Open hot-path readable. Persistent-transient errors now bubble up to the caller as a hard fail (supervisor restart path) rather than destroying a healthy DB via quarantine rename.

### WR-NEW-03: noduckdb analyzer reverse-direction lock

**Files modified/created:**
- `internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go-sibling/v2/sibling.go` — NEW stub package
- `internal/lint/noduckdb/testdata/src/siblingpkg/imports.go` — extended to import both bare and `/v2` forms; expanded comment

**Commit:** `269a51ab` — `test(57-05): lock noduckdb slash-boundary in both directions (WR-NEW-03)`

**Applied fix:** The existing `TestAnalyzer_AllowsSiblingNamespacePackage` now exercises both shapes in a single fixture. A regression that mistakenly removed the slash check would still pass the bare-only test (because `path == forbiddenImport` alone allows the bare); but our new combined fixture would catch a regression in either direction.

### IN-NEW-01: Store.DB() contract drift

**Files modified:**
- `internal/semantic/store/duckdb.go` — expanded `Store.DB()` doc comment to state the cross-build contract
- `internal/semantic/store/duckdb_winarm64.go` — refreshed stub comment to cross-reference

**Commit:** `8a0152f2` — `docs(57-05): document Store.DB() contract across CGO/winarm64 stubs (IN-NEW-01)`

**Applied fix:** Doc-only. The canonical method now documents `nil iff the store is not open; on windows/arm64 it always returns nil because this platform has no DuckDB build`. The stub points at this contract.

### IN-NEW-02: Probe error path unreachable nil-handle branch

**Files modified:**
- `internal/daemon/daemon.go` — removed unreachable `db == nil` branch; wrapped `p.s == nil` with `serr.ErrUnsupported`

**Commit:** `b453b91b` — `refactor(57-05): collapse unreachable nil-DB probe branch (IN-NEW-02)`

**Applied fix:** The kernel/health closed-enum classifier (WR-NEW-01) already maps the surviving "store unavailable" substring onto the `nil_handle` bucket. The `serr.ErrUnsupported` wrap is additive context for any future caller that wants programmatic disambiguation.

### IN-NEW-03: Migration rollback test doesn't anchor to applyMigration001

**Files modified:**
- `internal/semantic/store/migrations_test.go` — added `TestApplyMigration001_UsesTransactionalHelper`

**Commit:** `c637f0f3` — `test(57-05): anchor migration001 to applyStatementsTx (IN-NEW-03)`

**Applied fix:** Followed prompt's "cheapest path" — source-grep gate. The test reads `migrations.go` at test time and asserts `applyStatementsTx` appears at least twice. This is intentionally coarse so a future refactor that decouples `applyMigration001` from the helper is forced to update the test rather than silently regressing.

## Final verification

All five required commands ran clean against the fix branch.

| Command | Result |
|---|---|
| `go list ./... \| grep -v '^github.com/agenthands/helix/tmp/' \| xargs go build` | exit 0 (only pre-existing tree-sitter swift `TOKEN_COUNT` macro-redefinition warning) |
| `go vet $(go list ./... \| grep -v tmp/)` | exit 0 (only pre-existing tree-sitter swift warning) |
| `go test -short -timeout 180s ./internal/semantic/store/... ./internal/kernel/health/... ./internal/daemon/... ./internal/lint/noduckdb/... ./internal/config/... ./internal/phasegraph/...` | all packages OK |
| `go test -tags 'cgo integration' -timeout 180s -run TestDaemon_SemanticStore ./internal/daemon/...` | OK |
| `go install ./cmd/vet-noduckdb && go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./internal/... ./cmd/...` | exit 0 |

## Notes / deferred items

None. All seven findings landed within scope.

The BL-01 fix touches a wider blast radius than the prompt suggested (9 test files vs. the single `TestOpen_AcceptsCleanRelativePath` named in the review). This was unavoidable — every test that previously passed `filepath.Join(t.TempDir(), …)` straight into `Open` would have begun failing once the production guard landed. The chosen approach (add `t.Chdir(wsDir)` + relative path everywhere) keeps the test surface uniform and matches the doc-promised contract.

The TDD pair for BL-01 produced two commits as the prompt requested. The other six findings landed in single commits each (no TDD pair was demanded).

---

_Fixed: 2026-05-06_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
