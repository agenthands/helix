---
phase: 47
plan: 01
subsystem: kernel/lspool
tags: [rust, lsp, rust-analyzer, rename, rca, quirks, readiness]
dependency_graph:
  requires: []
  provides:
    - "lspool.RustAnalyzerAdapter.WaitUntilRenameReady(ctx) bool"
    - "lspool.ExperimentalCapabilities optional interface"
    - "lspool.RustAnalyzerAdapter implements NotificationHandlers for experimental/serverStatus"
  affects: [internal/kernel/lspool, internal/kernel/edit]
tech_stack:
  added: []
  patterns:
    - "optional-interface via type assertion (mirrors ArgsModifier precedent)"
    - "atomic.Bool + sync.Mutex-guarded channel for readiness transitions"
key_files:
  created:
    - ".planning/phases/47-bug-rust-analyzer-rename/47-RCA.md"
  modified:
    - "internal/kernel/lspool/quirks.go"
    - "internal/kernel/lspool/quirks_test.go"
    - "internal/kernel/lspool/worker.go"
decisions:
  - "Chose experimental/serverStatus.quiescent=true as the rename readiness signal; empirically fires at ~2.4-3.2s on the Phase 47 rust fixture (well inside the 45s LSTimeout)."
  - "Added ExperimentalCapabilities as a fresh optional interface (ArgsModifier-style) rather than extending the base QuirkAdapter interface — preserves compile-time compatibility for the 12 existing adapters."
  - "renameReadinessTimeout = 10s — generous headroom over observed quiescence (2.4-3.2s) without letting a stuck server wedge the rename path indefinitely (T-47-02 mitigation)."
metrics:
  duration: "~35 min"
  completed_date: "2026-04-24"
  tasks_completed: "2/2"
  files_changed: 4
requirements_addressed: [BUG-02]
---

# Phase 47 Plan 01: rust-analyzer rename RCA + readiness wiring Summary

One-liner: Captured the rust-analyzer 1.90 rename-in-cold-workspace failure as a reproducible LSP wire trace, identified `experimental/serverStatus.quiescent=true` as the deterministic readiness signal, and landed the adapter-side plumbing (capability advertisement + notification handler + `WaitUntilRenameReady` gate) that Plan 02's dispatcher will consume.

## Readiness Signal Chosen

Per **47-RCA.md §4**: `experimental/serverStatus.quiescent=true` with bounded `prepareRename` retry fallback (the latter is Plan 02 scope; Plan 01 ships the notification-driven primary path).

The RCA §3 captures two runs of a standalone LSP client against the exact Phase 47 fixture:

- **Run A (no wait after `initialized`+`didOpen`):** hover returned `null`, references returned `[]`, `prepareRename` returned `error "No references found at position"`, `rename` returned the identical error. All four within 51ms of process start, before the `quiescent=true` notification (which arrived at t=2407ms).
- **Run B (8s wait):** all four calls succeeded with full responses, with `rename` returning a complete 3-edit `WorkspaceEdit`.
- **Run A retry after the quiescent notification (no code change, same params):** succeeded with an identical `WorkspaceEdit`.

Verdict: **CONFIRMED** that the trigger is the rename path running against a cold per-position analysis state; waiting for `quiescent=true` eliminates the failure deterministically.

## Exported Surface Added (for Plan 02)

### `internal/kernel/lspool/quirks.go`

```go
// Package-scope constant.
const renameReadinessTimeout = 10 * time.Second

// New optional interface (ArgsModifier-style).
type ExperimentalCapabilities interface {
    ExperimentalCapabilities() map[string]any
}

// New methods on *RustAnalyzerAdapter:
func (r *RustAnalyzerAdapter) ExperimentalCapabilities() map[string]any
func (r *RustAnalyzerAdapter) NotificationHandlers() map[string]func(json.RawMessage) // non-nil; registers "experimental/serverStatus"
func (r *RustAnalyzerAdapter) WaitUntilRenameReady(ctx context.Context) bool
```

`WaitUntilRenameReady` contract:

- Returns `true` immediately if the adapter has already observed `quiescent=true`.
- Otherwise blocks until one of: (a) `quiescent=true` arrives (returns `true`), (b) `ctx` is cancelled (returns `false`), (c) `renameReadinessTimeout` elapses (returns `false`).
- Safe for concurrent invocation.

### `internal/kernel/lspool/worker.go`

Initialize-params assembly now merges `ec.ExperimentalCapabilities()` into `ClientCapabilities.Experimental` if the quirk adapter implements the new optional interface. No existing adapter regressed (none currently implement `ExperimentalCapabilities`; all 12 legacy adapters compile unchanged).

## Deviations from Plan

### Auto-fixed Issues

None required Rules 1-3 deviations. The plan's `didOpenAllFilesRecursive` reference in the context does not actually exist in `quirks.go` (the file has `didOpenFirstFileRecursive`), but `RustAnalyzerAdapter.PostInitialize` is currently a no-op — touching PostInitialize was out of scope for this plan anyway, so no drift correction was needed.

### Plan Clarifications Applied

- **Capability plumbing choice (Task 2 step 6):** adapter.go had no pre-existing capability hook, so option (b) was taken: new `ExperimentalCapabilities` optional interface + single call-site extension in `worker.go` where `initParams.Capabilities` is assembled (worker.go:193-210). Documented inline with a short comment referencing the quirks.go interface.
- **Trace method (Task 1):** `internal/kernel/jsonrpc/codec.go` and `conn.go` have no env-gated dumper, so option 2 (build-tagged dumper) was the plan's next choice. In practice, a cleaner path was taken: a standalone Go program under `/tmp/rca-47/main.go` (outside the repo tree) that spawns `rust-analyzer` directly and captures payloads — no build-tagged file was ever added to `internal/kernel/jsonrpc/`. This satisfies the plan's "Do NOT merge the build-tagged file to main" constraint trivially.
- **Test file naming:** the plan specified "quirks_test.go" (existing); the new tests are appended to that file rather than creating a new quirks_rename_test.go.

## Authentication Gates

None — all work was local.

## Files Changed

| File                                                                 | Kind     | Purpose                                                                                      |
| -------------------------------------------------------------------- | -------- | -------------------------------------------------------------------------------------------- |
| `.planning/phases/47-bug-rust-analyzer-rename/47-RCA.md`             | created  | Wire-trace + trigger identification + readiness signal decision + upstream-issue draft.      |
| `internal/kernel/lspool/quirks.go`                                   | modified | Added ExperimentalCapabilities iface, serverStatus handler, WaitUntilRenameReady, constants. |
| `internal/kernel/lspool/quirks_test.go`                              | modified | Added 4 unit tests (readiness transitions, malformed payload, interface conformance, caps).  |
| `internal/kernel/lspool/worker.go`                                   | modified | Merges quirk ExperimentalCapabilities into ClientCapabilities.Experimental at initialize.    |

## Commits

- `527875d3` `docs(47): capture rust-analyzer rename RCA trace`
- `b57fd9dd` `test(47-01): add failing tests for RustAnalyzerAdapter serverStatus readiness` (RED)
- `fbfc2b2e` `feat(47-01): wire experimental/serverStatus readiness into RustAnalyzerAdapter` (GREEN)

## Verification

All of the following exit 0:

```
go vet ./...
go test ./internal/kernel/lspool/... -count=1
test -s .planning/phases/47-bug-rust-analyzer-rename/47-RCA.md
grep -q '## 4. Readiness Signal Decision' .planning/phases/47-bug-rust-analyzer-rename/47-RCA.md
grep -qE 'WaitUntilRenameReady|experimental/serverStatus' internal/kernel/lspool/quirks.go
```

Full short-test suite (`go test ./... -count=1 -short`) is green across all packages; the only warning is a pre-existing Swift tree-sitter cgo macro redefinition unrelated to this phase.

## TDD Gate Compliance

- RED commit: `b57fd9dd test(47-01): add failing tests for RustAnalyzerAdapter serverStatus readiness` (build failure with `undefined: WaitUntilRenameReady` / `undefined: ExperimentalCapabilities` — verified before commit).
- GREEN commit: `fbfc2b2e feat(47-01): wire experimental/serverStatus readiness into RustAnalyzerAdapter` — 4/4 tests pass.
- REFACTOR: none required.

## Known Stubs

None. `WaitUntilRenameReady` is a complete primary path; the plan-documented bounded `prepareRename` retry fallback is Plan 02 scope (dispatcher composition) and is explicitly deferred — not a stub on the Plan 01 surface.

## Self-Check: PASSED

- `.planning/phases/47-bug-rust-analyzer-rename/47-RCA.md` — FOUND
- `internal/kernel/lspool/quirks.go` — FOUND (modified)
- `internal/kernel/lspool/quirks_test.go` — FOUND (modified)
- `internal/kernel/lspool/worker.go` — FOUND (modified)
- Commit `527875d3` — FOUND
- Commit `b57fd9dd` — FOUND
- Commit `fbfc2b2e` — FOUND
