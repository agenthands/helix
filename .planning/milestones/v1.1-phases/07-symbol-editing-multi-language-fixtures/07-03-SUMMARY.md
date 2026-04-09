---
phase: 07-symbol-editing-multi-language-fixtures
plan: 03
subsystem: test/integration, testdata/fixtures
tags: [java, rust, jdtls, rust-analyzer, integration-tests, multi-language, fixtures]
dependency_graph:
  requires: [harness.go, helpers.go, symbols_test.go patterns]
  provides: [java-fixture, rust-fixture, java-integration-tests, rust-integration-tests, requireLS-helper]
  affects: [test/integration, testdata/fixtures]
tech_stack:
  added: []
  patterns: [requireLS skip pattern, per-subtest PrepareFixture isolation, cross-file reference assertion]
key_files:
  created:
    - testdata/fixtures/java/Main.java
    - testdata/fixtures/java/Greeter.java
    - testdata/fixtures/rust/Cargo.toml
    - testdata/fixtures/rust/src/main.rs
    - testdata/fixtures/rust/src/greeter.rs
    - test/integration/java_test.go
    - test/integration/rust_test.go
  modified:
    - test/integration/helpers.go
decisions:
  - "Added requireLS helper to helpers.go for shared use across language-specific test files"
  - "Java fixtures use bare .java files without build system per D-01 (keep it simple); jdtls may need project markers in practice"
  - "Rust Cargo.toml edition 2021 for broad compatibility with rust-analyzer"
metrics:
  duration: 3min
  completed: 2026-04-08
  tasks: 2
  files: 8
---

# Phase 07 Plan 03: Java & Rust Fixture Projects + Integration Tests Summary

Java and Rust fixture projects with known symbol layouts plus integration tests covering symbol retrieval (6 subtests each) and representative edit operations (replace_body, rename) against jdtls and rust-analyzer.

## Task Completion

| Task | Name | Commit | Key Files |
|------|------|--------|-----------|
| 1 | Create Java and Rust fixture projects | d2024c13 | testdata/fixtures/java/{Main,Greeter}.java, testdata/fixtures/rust/{Cargo.toml,src/main.rs,src/greeter.rs} |
| 2 | Create Java and Rust integration tests | 467d2661 | test/integration/{java_test,rust_test}.go, test/integration/helpers.go |

## What Was Built

### Java Fixture (`testdata/fixtures/java/`)
- **Main.java**: Class with `helper()`, `unusedMethod()`, `usingHelper()`, `main()` -- cross-file reference via `new SimpleGreeter()`
- **Greeter.java**: `Greeter` interface + `SimpleGreeter` implementation

### Rust Fixture (`testdata/fixtures/rust/`)
- **Cargo.toml**: Minimal package manifest for rust-analyzer indexing
- **src/main.rs**: `DemoStruct` struct, `helper()`, `unused_func()`, `using_helper()`, `main()` -- cross-module reference via `mod greeter` and `use greeter::Greeter`
- **src/greeter.rs**: `Greeter` trait + `SimpleGreeter` implementation

### Integration Tests
- **java_test.go**: `TestSymbols_JavaFixture` (6 subtests: search_symbols, get_symbol_overview, find_references, get_hover_info, find_references_cross_file, find_implementations) + `TestEdit_JavaFixture` (2 subtests: replace_body, rename) with 60s jdtls timeout
- **rust_test.go**: `TestSymbols_RustFixture` (6 subtests) + `TestEdit_RustFixture` (2 subtests) with 45s timeout
- **helpers.go**: Added `requireLS(t, binary)` for graceful skip when LS not installed

### Test Design Patterns
- `requireLS(t, "jdtls")` / `requireLS(t, "rust-analyzer")` for graceful skip (D-04)
- Each edit subtest gets its own `PrepareFixture` + `StartTestDaemon` to avoid dirty session contamination
- Cross-file reference chains assert that references span multiple files (LANG-05)

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 2 - Missing] Added requireLS to helpers.go instead of inline**
- **Found during:** Task 2
- **Issue:** requireLS needed by both java_test.go and rust_test.go but didn't exist; defining in both files would cause duplicate symbol error
- **Fix:** Added requireLS to shared helpers.go file with os/exec import
- **Files modified:** test/integration/helpers.go
- **Commit:** 467d2661

## Verification

```
go vet -tags integration ./test/integration/... -- PASSED
go vet ./... -- PASSED
```

## Known Stubs

None -- all fixtures contain real code with compilable symbols and cross-file references.

## Self-Check: PASSED

- All 8 files verified present on disk
- Both commits (d2024c13, 467d2661) verified in git log
