---
phase: 57-semantic-store-foundation-pipeline-dag-library
plan: 05
status: complete
gap_closure: true
closes_findings: [SC-1, CR-01, CR-02, WR-01, WR-02, WR-03]
requirements_addressed: [STORE-01, STORE-03, STORE-06]
tags: [phase-57, semantic-store, hardening, security, gap-closure]
files_changed:
  - internal/lint/noduckdb/analyzer.go
  - internal/lint/noduckdb/analyzer_test.go
  - internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go-sibling/sibling.go
  - internal/lint/noduckdb/testdata/src/siblingpkg/imports.go
  - internal/semantic/store/migrations.go
  - internal/semantic/store/migrations_test.go
  - internal/semantic/store/duckdb.go
  - internal/semantic/store/duckdb_test.go
  - internal/semantic/store/duckdb_internal_test.go
  - internal/semantic/store/duckdb_winarm64.go
  - internal/kernel/health/tools.go
  - internal/kernel/health/tools_semantic_test.go
  - internal/daemon/daemon.go
commits:
  - 66e4c31f test(57-05): add failing test for noduckdb sibling-namespace bleed (WR-02)
  - f52c86f4 fix(57-05): tighten noduckdb matcher to exact-package + slash-prefix (WR-02)
  - 3be9ec80 test(57-05): add failing test for non-atomic migration001 (WR-01)
  - 3716a8c1 fix(57-05): wrap applyMigration001 in a transaction (WR-01)
  - fdc94552 test(57-05): add failing test for path-traversal rejection (CR-01)
  - ef5c3fa3 fix(57-05): refuse path-traversal segments in store.Open (CR-01)
  - 361bb7df fix(57-05): bound schema-version probe with 5s timeout (CR-02)
  - 3fdd5d0c test(57-05): add failing tests for reopen-error classifier (WR-03)
  - 6d20469f fix(57-05): classify transient reopen errors and retry once before quarantine (WR-03)
  - 32684b85 test(57-05): add failing test for semantic_store block in get_health (SC-1)
  - 4de3c36e feat(57-05): wire Daemon.SemanticStore() into get_health (SC-1)
key-decisions:
  - "Probe interface lives in kernel/health, daemon implements it — keeps internal/semantic out of the kernel import graph."
  - "JSON envelope wraps existing HealthReport with additive `semantic_store` field — legacy consumers parsing only `workspaces` are unaffected."
  - "Path-traversal rejection scans for `..` segments via filepath.ToSlash split — refuses rather than silently rewrites caller-supplied input."
  - "Transient-vs-corruption classifier: errors.Is(syscall.{EBUSY,EINTR,EAGAIN,ETXTBSY}) → transient; substring scan against {checksum, corrupt, header, malformed} → corruption; else unknown."
  - "Schema-probe timeout reuses the existing 5s budget already used for PingContext — no fifth quarantine reason added (D-07 closed-enum carve-out preserved)."
  - "applyStatementsTx is a sibling helper to applyMigration001 — testable in isolation; applyMigration002/003 stay outside scope (Phase 59 / 60 own those)."
---

# Phase 57 Plan 05: Gap-Closure & Hardening Pass — Summary

**One-liner:** Closes SC-1 by surfacing the semantic store readiness in `get_health`, plus five high-signal REVIEW.md findings (CR-01 path-traversal mitigation, CR-02 schema-probe timeout, WR-01 transactional migration, WR-02 noduckdb prefix bleed, WR-03 transient-error retry classifier) — all six findings closed in one PR's worth of 11 atomic commits.

## Findings Closed (6)

### SC-1 — `get_health` semantic_store block (literal Phase 57 ROADMAP success criterion)

**Truth:** `get_health` JSON output now contains a `semantic_store` block with `state ∈ {disabled, ready, unhealthy}`; `disabled` when the daemon's `SemanticStoreProbe` is nil or `Available()==false`; `ready` when a 1s `SELECT 1` probe succeeds; `unhealthy` with reason on probe failure.

**Verification gate:** `grep -c '"semantic_store"' internal/kernel/health/tools.go` returns `1`. The kernel package remains free of `internal/semantic` imports — the daemon-side `semanticStoreProbe` adapter satisfies the kernel-defined `SemanticStoreProbe` interface.

**Tests:** `TestSemanticStoreStatus_{Ready,Disabled,NilProbe,Unhealthy,JSONShape}` PASS in `internal/kernel/health/tools_semantic_test.go`. Existing `FilterReport_*` tests unaffected.

### CR-01 — Path-traversal rejection (security)

**Truth:** `store.Open` rejects any `cfg.Store.Path` containing a `..` segment with `errors.Is(err, serr.ErrInvalidArgs) == true`. The earlier silent `filepath.Clean` rewrite is gone — caller is responsible for handing us a workspace-rooted path.

**Verification gate:** `grep -c 'serr\.ErrInvalidArgs' internal/semantic/store/duckdb.go` returns `1`; `grep -v '^#' internal/semantic/store/duckdb.go | grep -c 'filepath.Clean(path)'` returns `0`.

**Tests:** `TestOpen_RejectsParentTraversal` covers `../../etc/passwd`, `subdir/../../escape.duckdb`, `..`, `a/../b/../c/../d.duckdb` — all PASS. `TestOpen_AcceptsCleanRelativePath` proves no false-positive on the happy path.

### CR-02 — Schema-probe timeout (liveness)

**Truth:** `classifyExisting`'s schema-version probe runs in a 5s child context, matching the existing `PingContext` budget. A wedged DuckDB read can no longer hang daemon startup indefinitely.

**Verification gate:** `awk '/func classifyExisting/,/^}/' internal/semantic/store/duckdb.go | grep -c 'context.WithTimeout(ctx, 5\*time.Second)'` returns `2` (one per ping + one per schema probe).

**TDD-skip rationale (logged in commit `361bb7df`):** Simulating a wedged DuckDB read in unit tests requires either a mock `*sql.DB`, hung-process injection, or a flaky filesystem-locking trick. The change is mechanical (5 lines, mirrors the existing `PingContext` budget pattern) and the closed-enum `reasonSchemaUnreadable` already absorbs `context.DeadlineExceeded` — D-07 carve-out preserved.

### WR-01 — Transactional `applyMigration001`

**Truth:** `applyMigration001` now delegates to `applyStatementsTx`, which runs every DDL statement inside a single `BeginTx` → `Commit` / `Rollback` envelope. Mid-migration failure leaves no partial schema on disk.

**Verification gate:** `grep -c 'BeginTx' internal/semantic/store/migrations.go` returns `1`; `grep -c 'applyStatementsTx' internal/semantic/store/migrations.go` returns `5` (definition + call from applyMigration001 + doc references).

**Tests:** `TestApplyMigration001_RollsBackOnFailure` injects a deliberately-broken DDL between two valid statements and asserts 0 surviving tables — PASSES. All pre-existing migration tests (`Fresh_v2`, `Existing_v2`, `ForwardIncompatible`, `Migration002_*`, `Migration003_*`) still PASS. Out of scope: `applyMigration002`/`003` (Phase 59 / 60 own those).

### WR-02 — `noduckdb` analyzer exact-package match

**Truth:** The lint analyzer matches `forbiddenImport` via `path == forbiddenImport || strings.HasPrefix(path, forbiddenImport+"/")` — sibling repos like `github.com/duckdb/duckdb-go-sibling` are no longer over-matched. A future major bump (`v3`) is still caught via the `/v3` subpath.

**Verification gate:** `grep -c 'forbiddenImport+"/"' internal/lint/noduckdb/analyzer.go` returns `1`.

**Tests:** Three analyzer tests run and PASS: `TestAnalyzer_RejectsImportFromBadpkg` (existing), `TestAnalyzer_AllowsImportFromGoodpkg` (existing), `TestAnalyzer_AllowsSiblingNamespacePackage` (NEW). New fixture `testdata/src/github.com/duckdb/duckdb-go-sibling/sibling.go` + `testdata/src/siblingpkg/imports.go` proves the tightened bound.

### WR-03 — Transient reopen-error retry classifier

**Truth:** `Open`'s Tier-1 reopen path classifies errors via `classifyReopenError(err)`. `errors.Is(err, syscall.{EBUSY,EINTR,EAGAIN,ETXTBSY})` → `reopenTransient` → 250ms backoff → one retry of `openExisting` before quarantine. Substring match against `{checksum, corrupt, header, malformed}` → `reopenCorruption` → immediate quarantine. Forward-incompat short-circuits as before.

**Verification gate:** `grep -c 'classifyReopenError' internal/semantic/store/duckdb.go` returns `3`; `grep -c 'reopenTransient' internal/semantic/store/duckdb.go` returns `3`; `grep -c 'time.After(250' internal/semantic/store/duckdb.go` returns `1`.

**Tests:** `TestClassifyReopenError` (8 sub-cases) + `TestClassifyReopenError_ETXTBSY` PASS in `internal/semantic/store/duckdb_internal_test.go`. All pre-existing store tests PASS — the retry adds a 250ms delay only on the transient branch, which existing tests do not trigger.

**TDD-skip rationale (logged in commit `6d20469f`):** Simulating a transient `EBUSY` from `sql.Open` requires fault injection into the `database/sql` driver registration — invasive and brittle. The classifier helper is unit-tested directly; the retry-loop wiring is a 12-line conditional that mirrors the existing forward-incompat early-return shape.

## Phase-Level Verification (executor pre-merge gate)

| Command | Exit |
|---|---|
| `go test ./internal/lint/noduckdb/... -count=1` | 0 |
| `go test ./internal/semantic/store/... -count=1` | 0 |
| `go test ./internal/kernel/health/... -count=1` | 0 |
| `go test -tags 'integration' ./internal/daemon/... -run TestDaemon_SemanticStore -count=1` | 0 |
| `go build ./...` | 0 |
| `go vet ./...` | 0 |
| `make vet` (vet-nokernel2semantic + vet-nosemantic2kernel) | 0 |
| `go test -short ./...` | 0 |

## Deviations from Plan

**1. Build tag on `migrations_test.go`** — The plan suggested adding `//go:build !(windows && arm64)` to a new `migrations_test.go`. The file already existed (Phase 57-02), did not carry the build tag, and shipped fine. The new `TestApplyMigration001_RollsBackOnFailure` was appended without adding the tag, matching the existing file's convention. The `internal/semantic/store/duckdb.go` build-tag fence (`!(windows && arm64)`) covers `applyStatementsTx` itself; the test path is governed by the same tag transitively.

**2. `time.After` regex spacing in WR-03 acceptance gate** — The plan's automated grep `time.After(250\*time.Millisecond)` (no spaces) does not match gofmt's `time.After(250 * time.Millisecond)` output. Validated with the equivalent `grep -c 'time.After(250'` returning `1`. Spirit of the gate (250ms retry backoff exists in code) is met.

**3. Adapter for windows/arm64 stub** — The plan did not call out adding `DB() *sql.DB` to `internal/semantic/store/duckdb_winarm64.go`. The new daemon `semanticStoreProbe.Probe` calls `p.s.DB()`, which would not compile on `windows/arm64` without the stub method. Added a nil-returning `DB()` to the stub (Available() already returns false there, so the adapter early-exits before reaching DB()). Rule 3 (auto-fix blocking issue) — minimal additive change preserving the platform-stub contract.

**4. Test-helper variant** — The plan's `openWithPath` example referenced `obs.NewMetrics(obs.MetricsOptions{Registry: nil})`. The existing test helpers in `store_test.go` use `obs.Noop(silentLogger().Handler()).Metrics()` instead. Adopted the existing convention to avoid coupling to `obs.MetricsOptions` shape. Functionally equivalent; same observable behaviour.

## Self-Check: PASSED

**Commit hashes verified:** All 11 hashes (`66e4c31f` through `4de3c36e`) present in `git log --oneline`.

**Files verified:**
- `internal/lint/noduckdb/analyzer.go` — FOUND
- `internal/lint/noduckdb/analyzer_test.go` — FOUND
- `internal/lint/noduckdb/testdata/src/github.com/duckdb/duckdb-go-sibling/sibling.go` — FOUND
- `internal/lint/noduckdb/testdata/src/siblingpkg/imports.go` — FOUND
- `internal/semantic/store/migrations.go` — FOUND
- `internal/semantic/store/migrations_test.go` — FOUND
- `internal/semantic/store/duckdb.go` — FOUND
- `internal/semantic/store/duckdb_test.go` — FOUND
- `internal/semantic/store/duckdb_internal_test.go` — FOUND
- `internal/semantic/store/duckdb_winarm64.go` — FOUND
- `internal/kernel/health/tools.go` — FOUND
- `internal/kernel/health/tools_semantic_test.go` — FOUND
- `internal/daemon/daemon.go` — FOUND
