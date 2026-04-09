---
phase: 08-advanced-testing
plan: 01
subsystem: test-integration
tags: [testing, harness, golden-files, integration]
requirements: [ADV-01, ADV-02, ADV-03, ADV-04]
dependency_graph:
  requires:
    - Phase 6 test harness (test/integration/harness.go, helpers.go)
    - Phase 7 fixtures
  provides:
    - Options.Profile, Options.Mode, Options.MaxWorkers for contract and concurrency tests
    - listSessionTools helper (oracle for profile/mode golden tests)
    - assertGoldenTools + -update flag (golden-file machinery for plans 02/04)
  affects:
    - test/integration/ (no behavioral regressions; defaults preserved)
tech_stack:
  added: []
  patterns:
    - Golden file testing with -update flag (Go stdlib pattern)
    - Package-level flag.Bool registered exactly once per package
key_files:
  created:
    - test/integration/golden.go
  modified:
    - test/integration/harness.go
    - test/integration/helpers.go
decisions:
  - "Mode switch placed after session connect but before workspace activation so workspace-less tests can switch modes"
  - "Single updateGolden flag per package; GOLDEN_UPDATE=1 env var as CI-friendly fallback"
  - "Golden file format: sorted tool names, one per line, LF-terminated — trivial to diff in PR review"
metrics:
  duration: "~3min"
  completed: "2026-04-08"
  tasks: 3
  files: 3
---

# Phase 08 Plan 01: Shared Test Infrastructure Summary

Foundation primitives for Phase 8's four test suites: extended `Options` with Profile/Mode/MaxWorkers, `listSessionTools` tools/list oracle, and `golden.go` with `assertGoldenTools` + `-update` flag.

## What Changed

### Task 1: Extended Options and defaultTestConfig
Added `Profile string`, `Mode string`, `MaxWorkers int` to `Options`. `StartTestDaemon` now applies `opts.Profile` and `opts.MaxWorkers` to `cfg` before daemon construction. A new mode-switch block calls the `switch_mode` MCP tool immediately after session connect when `opts.Mode != ""`, so tests can exercise modes without activating a workspace. Defaults preserve Phase 6/7 behavior (Profile="full", MaxWorkers=2).

Files: `test/integration/harness.go`
Commit: `0d15ae2b`

### Task 2: listSessionTools helper
Added `listSessionTools(t, session)` to `helpers.go`. Uses `session.ListTools(ctx, &mcp.ListToolsParams{})` so the result is the post-`ProfileFilterMiddleware` view — exactly what downstream contract tests need. Returns a plain `[]string` of tool names (unsorted; caller sorts inside `assertGoldenTools`).

Files: `test/integration/helpers.go`
Commit: `a6647174`

### Task 3: golden.go with assertGoldenTools
New file `test/integration/golden.go` (`//go:build integration`, `package integration_test`). Declares package-level `var updateGolden = flag.Bool("update", ...)`. `assertGoldenTools(t, name, actual)` sorts, joins with LF, writes or compares against `testdata/profiles/<name>.tools.golden`. `GOLDEN_UPDATE=1` env var works as a CI-friendly alternative. The auto-create mkdir ensures first-run `-update` works without manual directory setup.

Files: `test/integration/golden.go` (new)
Commit: `a73e4d86`

## Verification

- `go vet -tags integration ./test/integration/...` — PASS
- `go build -tags integration ./test/integration/...` — PASS
- `go test -tags integration ./test/integration/... -run ^$ -count=1` — PASS (compile-only smoke, confirms no duplicate flag registration)

## Deviations from Plan

None — plan executed exactly as written.

## Authentication Gates

None.

## Known Stubs

None. This plan intentionally ships zero test cases — the primitives are exercised by downstream plans 02/03/04.

## Self-Check: PASSED

- FOUND: test/integration/harness.go (modified, Profile/Mode/MaxWorkers fields present)
- FOUND: test/integration/helpers.go (modified, listSessionTools present)
- FOUND: test/integration/golden.go (created, assertGoldenTools + updateGolden flag present)
- FOUND: commit 0d15ae2b
- FOUND: commit a6647174
- FOUND: commit a73e4d86
