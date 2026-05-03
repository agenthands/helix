---
phase: 47
plan: 03
subsystem: rust-rename
tags: [rust, lsp, rust-analyzer, rename, integration-test, docs, usage, bug-fix]
requires:
  - 47-01 (RustAnalyzerAdapter + readiness gate)
  - 47-02 (RenameStrategy + RustAnalyzerRenameOverride + dispatcher)
provides:
  - Green TestEdit_RustFixture/rename integration test with strategy-tag assertion
  - USAGE.md Troubleshooting entry describing hybrid lsp-native / rust-client-side strategy
  - Budget-protected readiness gate in edit.RenameSymbol (prevents WaitUntilRenameReady from starving the fallback path)
affects:
  - internal/kernel/edit/rename.go (dispatcher deadline split)
  - internal/kernel/edit/rename_override.go (error detail on fallback references failure)
  - test/integration/rust_test.go (unskipped rename subtest)
  - USAGE.md (Troubleshooting section rewrite)
tech-stack:
  added: []
  patterns:
    - "context-deadline budget split (50/50 readiness-gate vs. dispatch work) for hybrid fallback paths"
key-files:
  modified:
    - test/integration/rust_test.go
    - internal/kernel/edit/rename.go
    - internal/kernel/edit/rename_override.go
    - USAGE.md
  created: []
decisions:
  - "Rule 1 bug fix: cap readiness-wait at remaining/2 so the client-side rename fallback retains enough ctx budget"
  - "Retain redundant WaitUntilRenameReady call inside RustAnalyzerRenameOverride (harmless once the quiescent flag is set; kept for explicit override semantics when called directly from tests)"
metrics:
  duration: ~25m
  tasks_completed: 2
  completed: 2026-04-24
---

# Phase 47 Plan 03: Integration Test Unskip + USAGE Docs Summary

Unskipped the rust rename integration test, added the D-07 strategy-tag assertion, fixed a budget-starvation bug in the rename dispatcher that was preventing the hybrid fallback from running, and rewrote the USAGE.md Troubleshooting entry to document the Phase 47 hybrid strategy.

## What Shipped

- `TestEdit_RustFixture/rename` now runs under the default `go test` integration tag and passes on a dev machine with rust-analyzer 1.90 installed.
- The test asserts the `rename_symbol` response body matches `strategy: (lsp-native|rust-client-side)`.
- `USAGE.md` Troubleshooting entry rewritten in place: describes the hybrid dispatcher, both strategy values, the D-05 semantic limits of the client-side fallback, and a BUG-DEFER-02 pointer for persistent-workspace-accurate rename.
- Budget-protection fix in `edit.RenameSymbol`: the readiness-gate `WaitUntilRenameReady` call is now bounded to at most half the remaining ctx deadline, so the client-side fallback still has budget to run `textDocument/references` and apply edits when the native path fails and quiescent never fires.

## Strategy That Triggered

During integration-test verification on the dev machine, the hybrid dispatcher landed on:

```
strategy: lsp-native
```

captured from `rust_test.go:131: rename: Renamed to "renamed_helper": 1 files changed, 3 edits applied / strategy: lsp-native / Post-edit verification: OK (no errors)`.

This is the preferred path (full rust-analyzer fidelity). The `rust-client-side` path remains exercised by the unit-test matrix in package `edit` (dispatcher tests seam `overriderResolverFn` and `tryNativeRenameFn`).

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Budget starvation between readiness gate and dispatcher work**
- **Found during:** Task 1 (running the unskipped integration test).
- **Issue:** `edit.RenameSymbol` passed the tool's full ctx (10s `TimeoutEdit`) into `WaitUntilRenameReady`. When rust-analyzer never emits `experimental/serverStatus.quiescent=true` (the exact BUG-02 failure mode), the wait consumed the entire budget, leaving zero time for the native-rename retry AND the client-side fallback's `textDocument/references` call. The fallback then failed with `context deadline exceeded`, which was wrapped through `serr.Internal: references` and surfaced as `unsupported: ... (internal: rust-client-side rename: find references)`.
- **Fix:** In `rename.go`, derive a child context capped at `remaining/2` for the readiness wait, so at least half the tool budget is preserved for either the native rename or the fallback.
- **Files modified:** `internal/kernel/edit/rename.go`
- **Commit:** 99cf89a6

**2. [Rule 3 - Blocking diagnosis] Fallback error lost its cause chain**
- **Found during:** Task 1 (diagnosing the budget-starvation bug).
- **Issue:** `RustClientSideRename` wrapped the references error without setting `.WithDetail(err.Error())`, so the underlying cause (`context deadline exceeded`) never reached the user-facing error message. Diagnosis required temporary stderr prints to discover the deadline issue.
- **Fix:** Added `.WithDetail(err.Error())` to the wrap so future failures are diagnosable without ad-hoc logging.
- **Files modified:** `internal/kernel/edit/rename_override.go`
- **Commit:** 99cf89a6

### No-op Items

- Doc comments on `RustAnalyzerAdapter` (`internal/kernel/lspool/quirks.go`) and `RustAnalyzerRenameOverride` (`internal/kernel/edit/rename_override.go`) already carried D-05 + BUG-DEFER-02 language from Plans 47-01 and 47-02 — no additional code-doc edits were needed in this plan. The plan's Task 2 Step B acceptance criteria were satisfied by the existing content.

## Commits

| Commit | Message |
|--------|---------|
| 99cf89a6 | test(47-03): unskip rust rename integration test and assert strategy tag |
| 9970d0a7 | docs(47-03): document rust rename hybrid strategy in USAGE.md |

## Phase 47 Ship-Gate Output

```
go vet ./...                # clean (only pre-existing swift tree-sitter TOKEN_COUNT warning, not from this plan)
go test ./... -count=1      # all packages green
go test -tags integration -run 'TestEdit_RustFixture/rename' ./test/integration/... -count=1
  → PASS (13.11s) strategy: lsp-native
go test -tags integration -run 'TestSymbols_RustFixture' ./test/integration/... -count=1
  → PASS (1.26s) all 6 subtests green (D-09 regression gate)
```

All nine verification-block commands succeed:

- `grep -q 'strategy: rust-client-side' USAGE.md` → 0
- `grep -q 'BUG-DEFER-02' USAGE.md` → 0
- `grep -q 'BUG-DEFER-02' internal/kernel/lspool/quirks.go` → 0
- `grep -q 'BUG-DEFER-02' internal/kernel/edit/rename_override.go` → 0
- `! grep -nE 't\.Skip\(.*rename' test/integration/rust_test.go` → no match (expected)
- `test -s .planning/phases/47-bug-rust-analyzer-rename/47-RCA.md` → 0

## Phase 47 Success Criteria Status

| Criterion | Status |
|-----------|--------|
| SC #1: default `go test ./...` green; integration test runs on dev machine | PASS (test unskipped, passes with strategy: lsp-native) |
| SC #2: USAGE.md + doc comments document hybrid + fallback limits | PASS (USAGE.md rewritten; doc comments already carried D-05/BUG-DEFER-02 from Plans 01/02) |
| SC #3: TestSymbols_RustFixture still green | PASS (6/6 subtests) |
| SC #4: RCA committed | PASS (inherited from Plan 01) |

Phase 47 is ready for `/gsd-verify-work 47`.

## Self-Check: PASSED

- Files exist: `test/integration/rust_test.go`, `internal/kernel/edit/rename.go`, `internal/kernel/edit/rename_override.go`, `USAGE.md` — all present.
- Commits exist: 99cf89a6, 9970d0a7 — both present in `git log`.
