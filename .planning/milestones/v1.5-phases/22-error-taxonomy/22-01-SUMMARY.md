---
phase: 22-error-taxonomy
plan: 01
subsystem: internal/errors
tags: [error-handling, taxonomy, mcp-tools]
dependency_graph:
  requires: []
  provides: [error-taxonomy-package, kind-type, error-struct, sentinel-vars]
  affects: [internal/kernel, internal/mcp, internal/skill]
tech_stack:
  added: []
  patterns: [kind-based-error-matching, builder-pattern, sentinel-vars-with-custom-Is]
key_files:
  created:
    - internal/errors/kinds.go
    - internal/errors/errors.go
    - internal/errors/errors_test.go
  modified: []
decisions:
  - Used external test package (errors_test) with serr alias per D-02 convention
  - Split into kinds.go and errors.go per research recommendation for navigability
  - Type alias anti-recursion pattern for MarshalJSON matching existing codebase pattern
metrics:
  duration_seconds: 110
  completed: "2026-04-15T07:52:45Z"
  tasks_completed: 1
  tasks_total: 1
  test_count: 14
  files_created: 3
---

# Phase 22 Plan 01: Error Taxonomy Package Summary

Typed error package with 7 Kind constants, builder-style Error struct, sentinel vars, and full errors.Is/As/Unwrap chain support using stdlib only.

## Task Results

| Task | Name | Commit(s) | Status |
|------|------|-----------|--------|
| 1 | Create internal/errors/ package with Kind type, Error struct, and tests | 0e5672de (RED), 5c67e413 (GREEN) | DONE |

## What Was Built

### internal/errors/kinds.go
- `type Kind string` with 7 constants: `NotFound`, `InvalidArgs`, `NoWorkspace`, `Unsupported`, `Internal`, `CircuitOpen`, `Timeout`
- 7 sentinel vars (`ErrNotFound`, `ErrInvalidArgs`, `ErrNoWorkspace`, `ErrUnsupported`, `ErrInternal`, `ErrCircuitOpen`, `ErrTimeout`) for `errors.Is` matching

### internal/errors/errors.go
- `Error` struct with `Kind`, `Message`, `Tool`, `Detail` (exported) and `cause` (unexported) fields
- `New(kind, message)` and `Wrap(kind, message, cause)` constructors
- `WithTool()` and `WithDetail()` builder methods (chainable, return `*Error`)
- `Error()` string formatting: `"kind: message"` or `"kind: message (detail)"`
- `Unwrap()` returns cause for error chain traversal
- `Is()` compares Kind values, enabling `errors.Is(err, serr.ErrNotFound)` through wrapped chains
- `MarshalJSON()` with type alias anti-recursion pattern; omits empty `tool` and `detail` fields

### internal/errors/errors_test.go
- 14 test functions covering all behaviors:
  - `TestKinds`: all 7 Kind constants with correct string values
  - `TestNew`, `TestWrap`: constructor behavior
  - `TestWithTool`, `TestWithDetail`: builder methods
  - `TestErrorString`: string formatting with and without detail
  - `TestIsKindMatch`, `TestIsKindMismatch`: Kind-based errors.Is
  - `TestIsChainWrapped`: errors.Is through fmt.Errorf %w wrapping
  - `TestIsWrappedCause`: errors.Is on wrapped cause (io.EOF)
  - `TestAs`: errors.As extraction of Tool and Kind
  - `TestMarshalJSON`: JSON output with all fields and omitempty
  - `TestNilError`: nil interface safety (Pitfall 1)
  - `TestSentinels`: all 7 sentinel vars exist with correct Kind
  - `TestBuilderChaining`: multi-method chaining

## Verification Results

- `go test ./internal/errors/... -v -count=1`: 14/14 PASS
- `go vet ./internal/errors/...`: clean
- `go build ./cmd/serena`: success
- `gofmt -l`: no formatting issues

## Deviations from Plan

None -- plan executed exactly as written.

## Decisions Made

1. **External test package**: Used `package errors_test` with `serr` import alias per D-02 convention, testing exclusively through the public API.
2. **File split**: `kinds.go` for Kind type + constants + sentinels, `errors.go` for Error struct + methods, per research recommendation.
3. **MarshalJSON pattern**: Reused the type alias anti-recursion pattern from `internal/mcp/errors.go` (`type alias Error`).

## Self-Check: PASSED

- [x] internal/errors/kinds.go exists
- [x] internal/errors/errors.go exists
- [x] internal/errors/errors_test.go exists
- [x] Commit 0e5672de exists (RED)
- [x] Commit 5c67e413 exists (GREEN)
