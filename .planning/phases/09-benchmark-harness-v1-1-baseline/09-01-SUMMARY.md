---
phase: 09
plan: 01
subsystem: test-harness
tags: [benchmark, testing, refactor, wave-0]
requires: []
provides:
  - "TB-typed StartTestDaemon, PrepareFixture, WaitForLS, requireGopls"
  - "TB-typed callTool, callToolExpectError, listSessionTools, requireLS"
  - "TestDaemon struct with testing.TB field"
affects:
  - test/integration/harness.go
  - test/integration/helpers.go
tech_stack:
  added: []
  patterns:
    - "testing.TB interface for shared test/bench helpers"
key_files:
  created: []
  modified:
    - test/integration/harness.go
    - test/integration/helpers.go
decisions:
  - "NewHTTPSession kept on *testing.T — its callers are always tests (subtest-scoped HTTP smoke) and t.Cleanup semantics are tied to the concrete *testing.T lifecycle"
metrics:
  tasks_completed: 2
  tasks_total: 2
  files_modified: 2
  duration_minutes: 3
  completed: "2026-04-09"
requirements_completed: [BENCH-01-prereq]
---

# Phase 9 Plan 1: Harness testing.TB Refactor Summary

Refactored the integration test harness entry points to accept `testing.TB` instead of `*testing.T`, unblocking Wave 0 of Phase 9 benchmarks. `*testing.T` and `*testing.B` both satisfy `testing.TB`, so all existing integration tests compile unchanged while future `test/bench/` benchmarks can now reuse `StartTestDaemon`, `PrepareFixture`, `callTool`, and related helpers without duplication.

## What Was Built

### Task 1: harness.go TB refactor (commit `84c127f5`)
- `TestDaemon` struct field `t *testing.T` → `tb testing.TB`
- `StartTestDaemon(tb testing.TB, opts Options) *TestDaemon`
- `WaitForLS(tb testing.TB, session, timeout)`
- `PrepareFixture(tb testing.TB, lang string) string`
- `requireGopls(tb testing.TB)`
- `defaultTestConfig(tb testing.TB) *config.SerenaConfig`
- `NewHTTPSession(t *testing.T)` intentionally retained, with a code comment explaining why (subtest-scoped HTTP transport smoke; callers are always tests).

### Task 2: helpers.go TB refactor (commit `ef7626e7`)
- `callTool(tb testing.TB, ...)`
- `callToolExpectError(tb testing.TB, ...)`
- `listSessionTools(tb testing.TB, ...)`
- `requireLS(tb testing.TB, binary string)`

No `callToolBehavioral` helper was found — 09-CONTEXT.md's canonical_refs listed it speculatively; the package does not define one.

## Verification

- `go vet -tags integration ./test/integration/...` — exit 0
- `go test -tags integration -count=1 -run ThisDoesNotExist ./test/integration/...` — compiles the full integration test binary without errors (`ok ... [no tests to run]`). Every existing `*testing.T` call site still type-checks because `*testing.T` satisfies `testing.TB`.
- `grep -n "t \*testing\.T" test/integration/harness.go` → only `NewHTTPSession` remains, as intended.
- `grep -c "t \*testing\.T" test/integration/helpers.go` → 0.

Full `-tags integration` test run was not executed in this plan (most integration tests require `gopls`/other LSPs and are orthogonal to the refactor; the binary compiling cleanly proves the type-level contract).

## Deviations from Plan

None - plan executed exactly as written.

## Commits

| Task | Hash       | Message                                                              |
| ---- | ---------- | -------------------------------------------------------------------- |
| 1    | `84c127f5` | refactor(09-01): accept testing.TB in harness entry points           |
| 2    | `ef7626e7` | refactor(09-01): accept testing.TB in helpers.go tool-call helpers   |

## Key Decisions

1. **NewHTTPSession keeps `*testing.T`.** It is only ever called from tests (HTTP transport smoke tests), not benchmarks. Its `t.Cleanup(ts.Close)` is tied to the concrete `*testing.T` lifecycle. Documented with an inline comment so future maintainers understand the exception.
2. **No call-site changes.** Because `*testing.T` satisfies `testing.TB`, every existing integration test continues to pass `t` as the first argument with zero edits. This keeps the blast radius to two files.

## Threat Flags

None — refactor is test-only code under `//go:build integration`, not compiled into any production binary.

## Self-Check: PASSED

- FOUND: test/integration/harness.go (modified, `tb testing.TB` present on StartTestDaemon/PrepareFixture/WaitForLS/requireGopls/defaultTestConfig)
- FOUND: test/integration/helpers.go (modified, `tb testing.TB` present on callTool/callToolExpectError/listSessionTools/requireLS)
- FOUND: commit 84c127f5 (`git log --oneline | grep 84c127f5`)
- FOUND: commit ef7626e7 (`git log --oneline | grep ef7626e7`)
- `go vet -tags integration ./test/integration/...` → exit 0
- Integration test binary compiles (`go test -run ThisDoesNotExist` → ok)
