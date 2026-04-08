---
phase: 07-symbol-editing-multi-language-fixtures
plan: 02
subsystem: test/integration, testdata/fixtures
tags: [python, typescript, integration-tests, multi-language, fixtures]
dependency_graph:
  requires: [07-01]
  provides: [python-fixture, typescript-fixture, python-integration-tests, typescript-integration-tests]
  affects: [test/integration]
tech_stack:
  added: [pyright-langserver, typescript-language-server]
  patterns: [requireLS-skip, per-subtest-fixture-isolation]
key_files:
  created:
    - testdata/fixtures/python/main.py
    - testdata/fixtures/python/utils.py
    - testdata/fixtures/typescript/main.ts
    - testdata/fixtures/typescript/greeter.ts
    - testdata/fixtures/typescript/tsconfig.json
    - test/integration/python_test.go
    - test/integration/typescript_test.go
  modified:
    - test/integration/helpers.go
decisions:
  - "Added requireLS(t, binary) to helpers.go since Plan 07-01 had not created it yet"
  - "Edit subtests each get own PrepareFixture + StartTestDaemon to avoid dirty session contamination"
metrics:
  duration: 2min
  completed: "2026-04-08T21:29:00Z"
  tasks: 2
  files: 8
---

# Phase 07 Plan 02: Python & TypeScript Fixtures + Integration Tests Summary

Python and TypeScript fixture projects with known symbol layouts, plus integration tests for symbol retrieval, cross-file references, and edit round-trips via pyright and typescript-language-server.

## Task Summary

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Create Python and TypeScript fixture projects | 87663c0e | testdata/fixtures/python/, testdata/fixtures/typescript/ |
| 2 | Create Python and TypeScript integration tests | 9308804e | test/integration/python_test.go, typescript_test.go, helpers.go |

## What Was Built

### Python Fixture (testdata/fixtures/python/)
- `main.py`: DemoClass with value field, helper() for edit tests, unused_func() for safe_delete, using_helper() for cross-reference, import from utils
- `utils.py`: greet() function, Greeter class with cross-file reference chain

### TypeScript Fixture (testdata/fixtures/typescript/)
- `main.ts`: DemoClass, helper(), unusedFunc(), usingHelper(), Greeter import from greeter
- `greeter.ts`: IGreeter interface, Greeter class implementing IGreeter
- `tsconfig.json`: strict mode, ES2020 target, bundler moduleResolution for LS indexing

### Integration Tests
- `python_test.go`: TestSymbols_PythonFixture (search_symbols, get_symbol_overview, find_references, get_hover_info, find_references_cross_file) + TestEdit_PythonFixture (replace_body, rename)
- `typescript_test.go`: TestSymbols_TypeScriptFixture (search_symbols, get_symbol_overview, find_references, get_hover_info, find_references_cross_file, find_implementations) + TestEdit_TypeScriptFixture (replace_body, rename)

### Helper Addition
- Added `requireLS(t, binary)` to helpers.go for generic LS skip pattern (Rule 3: Plan 07-01 had not created it yet)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Added requireLS helper inline**
- **Found during:** Task 2
- **Issue:** Plan 07-01 was expected to create requireLS but it did not exist in helpers.go
- **Fix:** Added requireLS(t, binary) to test/integration/helpers.go using exec.LookPath pattern matching requireGopls
- **Files modified:** test/integration/helpers.go
- **Commit:** 9308804e

## Verification

- `go vet -tags integration ./test/integration/...` passes cleanly
- `go vet ./...` passes cleanly
- All fixture files exist with expected symbols
- Tests skip gracefully when pyright-langserver or typescript-language-server not installed
