---
phase: 48
plan: 02
subsystem: kernel/lspool
tags: [jdtls, lspool, quirks, env-var, tdd]
requires: [48-01]
provides:
  - "SERENA_TEST_JDTLS_DATA_DIR env var override in JdtlsAdapter.ExtraArgs"
  - "Test contract consumed by Plan 48-03 harness (StartTestDaemon)"
affects:
  - internal/kernel/lspool/quirks.go
tech-stack:
  added: []
  patterns:
    - "Env-var injection (option c from PATTERNS.md): zero-API-churn test hook"
    - "TDD RED→GREEN with t.Setenv auto-restoration"
key-files:
  created: []
  modified:
    - internal/kernel/lspool/quirks.go
    - internal/kernel/lspool/quirks_test.go
decisions:
  - "Used env-var only (no constructor field on JdtlsAdapter) to keep pool API surface unchanged"
  - "Empty env var treated identically to unset for safety in CI shells that may export blanks"
metrics:
  duration: "~5 minutes"
  completed: "2026-04-25"
  tasks_completed: 1
  files_changed: 2
---

# Phase 48 Plan 02: JdtlsAdapter Env-Var Override Summary

Added `SERENA_TEST_JDTLS_DATA_DIR` env-var override to `JdtlsAdapter.ExtraArgs` so integration tests can redirect the jdtls `-data` workspace to a warm cache dir; production behavior is preserved unchanged.

## What Changed

### `internal/kernel/lspool/quirks.go`
- `JdtlsAdapter.ExtraArgs` now reads `SERENA_TEST_JDTLS_DATA_DIR`. If non-empty, that path is used verbatim as the `-data` argument. If empty/unset, falls back to the original `workDir/.jdtls-data`.
- `os.MkdirAll(dir, 0o755)` still runs on whichever path is chosen (existing safety preserved).
- Doc comment expanded to document the test-only contract.

### `internal/kernel/lspool/quirks_test.go`
- `TestJdtlsAdapter_ExtraArgs_DefaultDataDir` — env unset, default path used and created.
- `TestJdtlsAdapter_ExtraArgs_EnvOverride` — env set, override used and created; default not created.
- `TestJdtlsAdapter_ExtraArgs_EnvEmptyFallsBack` — empty env behaves like unset.
- `TestJdtlsAdapter_ExtraArgs_PreservesExtraArgs` — `-data <dir>` is prepended; tail args preserved in order.

## TDD Cycle

| Phase | Commit | Description |
|-------|--------|-------------|
| RED   | 6d040d94 | Add 4 failing tests for env-var override |
| GREEN | aacedff9 | Implement `os.Getenv("SERENA_TEST_JDTLS_DATA_DIR")` branch |
| REFACTOR | — | Skipped: implementation already minimal/clean |

## Verification

| Check | Result |
|-------|--------|
| `go vet ./internal/kernel/lspool/...` | clean |
| `go test ./internal/kernel/lspool/... -count=1 -run 'TestJdtlsAdapter_ExtraArgs'` | 4 PASS |
| `go test ./internal/kernel/lspool/... -count=1` | PASS (entire package) |
| `go vet ./...` | clean (only pre-existing C macro warning in unrelated swift binding) |

## Acceptance Criteria

- [x] `grep -c "SERENA_TEST_JDTLS_DATA_DIR" quirks.go` == 2 (doc + Getenv call)
- [x] `grep -c 'os.Getenv("SERENA_TEST_JDTLS_DATA_DIR")' quirks.go` == 1
- [x] Fallback `filepath.Join(workDir, ".jdtls-data")` still present
- [x] Single `func (j *JdtlsAdapter) ExtraArgs(` definition (no duplicate)
- [x] Four `TestJdtlsAdapter_ExtraArgs_*` tests in `quirks_test.go`, all passing
- [x] Diff scoped to `ExtraArgs` body + doc comment; no other adapter modified

## Deviations from Plan

None — plan executed exactly as written.

## Threat Flags

None — env-var contract matches the plan's `<threat_model>` exactly. No new trust boundary introduced.

## Self-Check: PASSED
- FOUND: internal/kernel/lspool/quirks.go (modified)
- FOUND: internal/kernel/lspool/quirks_test.go (modified)
- FOUND commit 6d040d94 (RED)
- FOUND commit aacedff9 (GREEN)
