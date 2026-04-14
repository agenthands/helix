---
phase: 18-harness-extraction-foundation
plan: 02
subsystem: test-oracle
tags: [testing, oracle, build-tags, smoke-test]
dependency_graph:
  requires: [importable-test-harness]
  provides: [oracle-directory-structure, build-tag-taxonomy-verified]
  affects: [test/oracle/]
tech_stack:
  added: []
  patterns: [build-tag-hierarchy, oracle-package-layout]
key_files:
  created:
    - test/oracle/protocol/doc.go
    - test/oracle/contract/doc.go
    - test/oracle/scenario/doc.go
    - test/oracle/llm/doc.go
    - test/oracle/judge/doc.go
    - test/oracle/protocol/smoke_test.go
  modified: []
decisions:
  - "Build tag expressions encode tier hierarchy explicitly per D-06"
  - "External test package (protocol_test) for smoke test to validate importability"
metrics:
  duration: 5min
  completed: 2026-04-11
  tasks_completed: 2
  tasks_total: 2
  files_created: 6
  files_modified: 0
---

# Phase 18 Plan 02: Oracle Directory Stubs & Smoke Test Summary

Oracle directory structure with 5 build-tag-gated stub packages and a 3-test smoke suite proving cross-package harness import and build tag taxonomy correctness.

## What Was Done

### Task 1: Create oracle directory stubs with build tags
**Commit:** 8962fc91

Created 5 `doc.go` files establishing the oracle package structure per D-09. Each file carries the correct build tag expression per D-06:

| Package | Build Tag | Tier |
|---------|-----------|------|
| test/oracle/protocol/ | `integration \|\| llm \|\| llmjudge` | Deterministic |
| test/oracle/contract/ | `integration \|\| llm \|\| llmjudge` | Deterministic |
| test/oracle/scenario/ | `integration \|\| llm \|\| llmjudge` | Deterministic |
| test/oracle/llm/ | `llm \|\| llmjudge` | LLM behavioral |
| test/oracle/judge/ | `llmjudge` | Judge only |

All pass `go vet` with their respective tags.

### Task 2: Smoke test proving harness import and build tag taxonomy
**Commit:** 3e8c8846

Created `test/oracle/protocol/smoke_test.go` with 3 tests validating the harness is importable and functional from an oracle package:

1. **TestSmokeImportHarness** -- StartRunner + ListSessionTools cross-package (found 33 tools)
2. **TestSmokeFixture** -- PrepareFixture("go") copies fixture with main.go
3. **TestSmokeProjectRoot** -- ProjectRoot resolves to repo root with go.mod

Build tag verification matrix results:

| Command | Expected | Actual |
|---------|----------|--------|
| `go test ./test/oracle/...` (no tags) | matched no packages | matched no packages |
| `go test -tags integration ./test/oracle/protocol/...` | 3 PASS | 3 PASS (0.8s) |
| `go test -tags integration ./test/oracle/llm/...` | matched no packages | matched no packages |
| `go test ./...` (no tags) | no oracle/harness tests | no oracle/harness tests |
| `git diff test/integration/` | empty | empty |
| Go integration regression tests | PASS | PASS |

## Verification Results

| Check | Result |
|-------|--------|
| `go vet -tags integration ./test/oracle/protocol/...` | PASS |
| `go vet -tags integration ./test/oracle/contract/...` | PASS |
| `go vet -tags integration ./test/oracle/scenario/...` | PASS |
| `go vet -tags llm ./test/oracle/llm/...` | PASS |
| `go vet -tags llmjudge ./test/oracle/judge/...` | PASS |
| `go test -tags integration ./test/oracle/protocol/... -v -count=1` | 3/3 PASS |
| `git diff test/integration/` | empty (D-11 satisfied) |
| Existing Go integration tests | PASS |

## Deviations from Plan

None -- plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | 8962fc91 | feat(18-02): create oracle directory stubs with build tag taxonomy |
| 2 | 3e8c8846 | test(18-02): add smoke test proving harness import and build tag taxonomy |
