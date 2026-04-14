---
phase: 18-harness-extraction-foundation
plan: 01
subsystem: test-harness
tags: [testing, harness, extraction, mcp]
dependency_graph:
  requires: []
  provides: [importable-test-harness, runner-api, golden-store]
  affects: [test/harness/]
tech_stack:
  added: []
  patterns: [exported-harness-api, golden-store-struct]
key_files:
  created:
    - test/harness/doc.go
    - test/harness/runner.go
    - test/harness/fixture.go
    - test/harness/tools.go
    - test/harness/golden.go
    - test/harness/runner_test.go
  modified: []
decisions:
  - "Evolved naming: TestDaemon->Runner, Options->RunnerOptions, StartTestDaemon->StartRunner per D-02"
  - "GoldenStore as struct with RootDir field for flexible golden directory configuration"
  - "Package harness (not harness_test) for importability by oracle packages"
metrics:
  duration: 5min
  completed: 2026-04-11
  tasks_completed: 2
  tasks_total: 2
  files_created: 6
  files_modified: 0
---

# Phase 18 Plan 01: Harness Extraction Foundation Summary

Importable test/harness/ package extracted from test/integration/ with evolved naming, exported symbols, and 7 self-tests validating the harness API.

## What Was Done

### Task 1: Create test/harness/ package with evolved API
**Commit:** b965bded

Extracted test infrastructure from `test/integration/` (package `integration_test`, non-importable) into `test/harness/` (package `harness`, importable). All files carry the `//go:build integration || llm || llmjudge` build tag.

Key renames per D-02:
- `TestDaemon` -> `Runner` (with exported `Daemon` field)
- `Options` -> `RunnerOptions`
- `StartTestDaemon` -> `StartRunner`
- `projectRoot` -> `ProjectRoot` (exported)
- `defaultTestConfig` -> `DefaultTestConfig` (exported)
- `requireGopls` -> `RequireGopls`, `requireLS` -> `RequireLS`
- All tool helpers exported: `CallTool`, `TextContent`, `CallToolExpectError`, `ListSessionTools`

New `GoldenStore` struct with `AssertTools` method and standalone `AssertGolden` function.

Zero modifications to `test/integration/` (D-11).

### Task 2: Add harness self-tests
**Commit:** d5f04456

Seven self-tests validating the harness API:
1. `TestProjectRoot` - go.mod exists at root
2. `TestDefaultTestConfig` - config defaults (full profile, 4 workers, 30s TTL)
3. `TestPrepareFixture` - go fixture copy with main.go
4. `TestStartRunnerSmoke` - Runner creation with non-nil Session/Daemon
5. `TestCallToolSmoke` - tool registration via ListSessionTools
6. `TestTextContent` - text extraction from CallToolResult (with and without content)
7. `TestGoldenStoreRoundTrip` - golden write/read cycle with sorted output

## Verification Results

| Check | Result |
|-------|--------|
| `go build -tags integration ./test/harness/...` | PASS |
| `go vet -tags integration ./test/harness/...` | PASS |
| `go test -tags integration ./test/harness/... -v -count=1` | 7/7 PASS (0.8s) |
| `go vet ./...` | PASS (no regressions) |
| `git diff test/integration/` | empty (D-11 satisfied) |
| Integration regression (TestHarness_StartAndCallTool) | PASS |

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Commit | Description |
|------|--------|-------------|
| 1 | b965bded | feat(18-01): extract test harness into importable test/harness/ package |
| 2 | d5f04456 | test(18-01): add harness self-tests |
