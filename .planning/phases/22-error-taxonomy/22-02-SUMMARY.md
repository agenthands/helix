---
phase: 22-error-taxonomy
plan: 02
subsystem: internal/kernel/lspool, internal/mcp
tags: [error-handling, taxonomy, migration, backward-compat]
dependency_graph:
  requires: [error-taxonomy-package]
  provides: [circuit-open-serr-reexport, mcp-sentinels-serr-reexport]
  affects: [internal/kernel/lspool, internal/mcp]
tech_stack:
  added: []
  patterns: [sentinel-re-export, kind-based-is-matching, detail-string-flattening]
key_files:
  created: []
  modified:
    - internal/kernel/lspool/circuit_err.go
    - internal/kernel/lspool/circuit.go
    - internal/kernel/lspool/pool.go
    - internal/kernel/lspool/circuit_test.go
    - internal/mcp/errors.go
    - internal/mcp/server_test.go
decisions:
  - Flattened CircuitOpenError metadata (language, failures, backoff, retry_after) into serr.Error Detail string per D-05
  - Updated test assertions for new serr.Error string format (kind colon message) rather than keeping old message strings
  - Used serr.New() for ErrSessionExpired and ErrLSCrashed (need specific messages), sentinel re-exports for the other 3
metrics:
  duration_seconds: 236
  completed: "2026-04-15T08:00:05Z"
  tasks_completed: 2
  tasks_total: 2
  test_count: 0
  files_created: 0
  files_modified: 6
---

# Phase 22 Plan 02: Sentinel Re-export Migration Summary

Migrated CircuitOpenError and 5 mcp sentinels to re-export from internal/errors package, preserving backward compat for all callers via Kind-based errors.Is matching.

## Task Results

| Task | Name | Commit(s) | Status |
|------|------|-----------|--------|
| 1 | Migrate CircuitOpenError to serr.ErrCircuitOpen re-export | e0618802 | DONE |
| 2 | Migrate mcp/errors.go sentinels to serr re-exports | 5ea18925 | DONE |

## What Was Built

### Task 1: CircuitOpenError Migration

- **circuit_err.go**: Replaced entire `CircuitOpenError` struct with a single `var ErrCircuitOpen = serr.ErrCircuitOpen` re-export
- **pool.go**: Removed the old `var ErrCircuitOpen = errors.New(...)` sentinel (now defined in circuit_err.go)
- **circuit.go**: Updated `CircuitOpenErr()` to return `*serr.Error` with Kind `CircuitOpen` and metadata flattened into the Detail string (format: `language=X failures=N backoff_remaining=Xms retry_after=RFC3339`)
- **circuit_test.go**: Rewrote 3 tests (`TestCircuitOpenError_Fields`, `TestCircuitOpenError_Is`, `TestCircuitOpenError_Unwrap`) to use `serr.New(serr.CircuitOpen, ...)` instead of `&CircuitOpenError{}`; added `TestCircuitOpenErr_ReturnsSerrError` for the updated method

### Task 2: MCP Sentinel Migration

- **errors.go**: Replaced 5 `errors.New()` sentinels with serr re-exports:
  - `ErrSessionExpired` = `serr.New(serr.Timeout, "session expired")`
  - `ErrWorkspaceNotReady` = `serr.ErrNoWorkspace`
  - `ErrToolNotAvailable` = `serr.ErrUnsupported`
  - `ErrProjectNotFound` = `serr.ErrNotFound`
  - `ErrLSCrashed` = `serr.New(serr.Internal, "language server crashed")`
- **ErrorDetail** struct preserved for Phase 23 removal
- **server_test.go**: Updated `TestDomainErrors` assertions for new `"kind: message"` format; added cross-Kind `NotErrorIs` assertions

## Verification Results

- `go test ./internal/kernel/lspool/... -v -run Circuit`: 11/11 PASS
- `go test ./internal/mcp/... -v`: 31/31 PASS (including telemetry circuit_open classification)
- `go test ./...`: all packages PASS
- `go vet ./...`: clean
- `go build ./cmd/serena`: success

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Updated test assertions for changed error string format**
- **Found during:** Task 2
- **Issue:** `TestDomainErrors` used `assert.EqualError` with old message strings (e.g., `"session expired"`), but `serr.Error.Error()` returns `"kind: message"` format (e.g., `"timeout: session expired"`)
- **Fix:** Updated assertions to match new format; changed sentinel-only checks to `ErrorIs`; added `NotErrorIs` cross-Kind assertions
- **Files modified:** internal/mcp/server_test.go
- **Commit:** 5ea18925

**2. [Rule 3 - Blocking] Updated CircuitBreaker.CircuitOpenErr() return type**
- **Found during:** Task 1
- **Issue:** `CircuitOpenErr()` returned `*CircuitOpenError` which no longer exists after migration. Pool.go calls this method.
- **Fix:** Changed return type to `*serr.Error`, flattened metadata into Detail string
- **Files modified:** internal/kernel/lspool/circuit.go
- **Commit:** e0618802

## Decisions Made

1. **Detail string format**: Used `key=value` pairs (`language=go failures=3 backoff_remaining=5s retry_after=...`) for flattened CircuitOpenError metadata, keeping it human-readable and grep-friendly.
2. **Test assertion style**: Used `ErrorIs` for sentinel re-exports (ErrWorkspaceNotReady etc.) since their Error() string is just `"kind: "` with no message; used `EqualError` for sentinels with messages.
3. **serr.New vs sentinel re-export**: Used `serr.New()` for ErrSessionExpired and ErrLSCrashed (need domain-specific messages), direct sentinel pointer re-exports for the other 3 (map cleanly to a Kind).

## Self-Check: PASSED

- [x] All 6 modified files exist on disk
- [x] Commit e0618802 exists (Task 1)
- [x] Commit 5ea18925 exists (Task 2)
- [x] circuit_err.go contains serr.ErrCircuitOpen (2 refs), no CircuitOpenError struct
- [x] pool.go does not contain old ErrCircuitOpen sentinel
- [x] mcp/errors.go imports serr (8 refs), retains ErrorDetail (5 refs)
- [x] go test ./... passes, go vet clean, go build succeeds
