---
phase: 20-scenarios-runtime
plan: 01
subsystem: test-fixtures
tags: [fixtures, polyglot, collision, unsupported, oracle]
dependency_graph:
  requires: []
  provides:
    - testdata/fixtures/polyglot/
    - testdata/fixtures/unsupported/
    - testdata/fixtures/collision/
    - test/oracle/runtime/
  affects:
    - test/oracle/scenario/ (plans 02-03 consume fixtures)
    - test/oracle/runtime/ (plan 04 uses runtime package)
tech_stack:
  added: []
  patterns: [build-tag-constrained test packages]
key_files:
  created:
    - testdata/fixtures/polyglot/ (10 files, Go+Python+TypeScript monorepo)
    - testdata/fixtures/unsupported/ (2 .xyz files, no language markers)
    - testdata/fixtures/collision/ (6 files, Config/Handler/Parse in 3 languages)
    - test/oracle/runtime/doc.go
  modified: []
decisions:
  - Matched existing fixture patterns exactly (go/main.go, python/main.py, typescript/main.ts)
  - Used package runtime (not runtime_test) to match test/oracle/scenario/ convention
metrics:
  duration: 177s
  completed: 2026-04-11
---

# Phase 20 Plan 01: Test Fixtures and Runtime Oracle Stub Summary

Three test fixture directories for polyglot, unsupported-language, and name-collision scenarios plus runtime oracle package stub following existing oracle patterns.

## Task Completion

| Task | Name | Commit | Files |
|------|------|--------|-------|
| 1 | Create polyglot, unsupported, and collision fixtures | 1d8668cb | 17 files across testdata/fixtures/{polyglot,unsupported,collision}/ |
| 2 | Create runtime oracle package stub | a7794a09 | test/oracle/runtime/doc.go |

## Deviations from Plan

None -- plan executed exactly as written.

### Notes

- Plan specifies `package runtime_test` but `test/oracle/scenario/doc.go` uses `package scenario` (not `scenario_test`). Followed the established convention: `package runtime`. The plan itself noted to check and match the pattern.
- `go vet ./test/oracle/runtime/` without tags reports "build constraints exclude all Go files" which is expected and consistent with `test/oracle/scenario/` behavior. With `-tags integration` it passes cleanly.

## Verification Results

- All three fixture directories exist with correct structure
- Polyglot fixture has go.mod, pyproject.toml, tsconfig.json at root level
- Unsupported fixture has .xyz files only, no language markers
- Collision fixture has Config/Handler/Parse in Go, Python, TypeScript
- All collision marker files at root level (not nested)
- `go test ./...` passes (no regressions)
- `go vet -tags integration ./test/oracle/runtime/` passes

## Self-Check: PASSED

- testdata/fixtures/polyglot/go.mod: FOUND
- testdata/fixtures/unsupported/main.xyz: FOUND
- testdata/fixtures/collision/backend/config/main.go: FOUND
- test/oracle/runtime/doc.go: FOUND
- Commit 1d8668cb: FOUND
- Commit a7794a09: FOUND
