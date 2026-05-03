---
phase: 47
plan: 02
subsystem: kernel/edit + mcp + obs
tags: [rust, lsp, rust-analyzer, rename, quirks, dispatcher, strategy, metrics]
dependency_graph:
  requires:
    - "lspool.RustAnalyzerAdapter.WaitUntilRenameReady (Plan 01)"
  provides:
    - "edit.RenameSymbol dispatcher (native + override paths, Strategy-tagged result)"
    - "edit.RenameOverrider optional interface"
    - "edit.RenameStrategy typed string + StrategyLSPNative / StrategyRustClientSide constants"
    - "edit.RustClientSideRename references-driven rename helper"
    - "edit.RustAnalyzerRenameOverride wrapper (lspool->edit cycle avoidance)"
    - "lspool.Worker.Quirks() + lspool.WorkerLease.Adapter() accessors"
    - "obs.Metrics.RenameStrategy CounterVec + RenameStrategyInc helper"
    - "mcp.RecordRenameStrategy(ctx, strategy) closed-enum recorder"
  affects:
    - internal/kernel/edit
    - internal/kernel/lspool
    - internal/mcp
    - internal/obs
tech_stack:
  added: []
  patterns:
    - "optional-interface via cross-package type assertion (RenameOverrider)"
    - "wrapper-type seam to avoid upstream->downstream import cycle"
    - "closed-enum label discipline enforced at emission site (obs carve-out)"
    - "atomic.Pointer sink for cross-package recorder wiring"
    - "test seams via package-level function variables (tryNativeRenameFn, overriderResolverFn)"
key_files:
  created:
    - internal/kernel/edit/rename_override.go
    - internal/kernel/edit/rename_override_test.go
  modified:
    - internal/kernel/edit/rename.go
    - internal/kernel/edit/tools.go
    - internal/kernel/lspool/lease.go
    - internal/kernel/lspool/worker.go
    - internal/mcp/middleware.go
    - internal/obs/metrics.go
    - internal/obs/metrics_labels_test.go
decisions:
  - "Import-cycle resolved with a wrapper type (RustAnalyzerRenameOverride) in package edit holding a *lspool.RustAnalyzerAdapter; dispatcher type-asserts the adapter and constructs the wrapper inline. No callback variables, no package-level mutable state for the override path."
  - "Metric counter lives on obs.Metrics (Prometheus CounterVec). mcp.RecordRenameStrategy delegates via an atomic.Pointer sink wired in InstallMiddleware (matches existing provider-centric pattern; avoids adding new OTel meters or a new internal/telemetry package)."
  - "Introduced two package-level test seams in internal/kernel/edit/rename.go: tryNativeRenameFn and overriderResolverFn. Keeps the dispatcher matrix (native-success / native-fail-override-success / both-fail) unit-testable without a live language server while leaving production wiring identical to the naive inline form."
  - "Added Worker.Quirks() and WorkerLease.Adapter() accessors; the private w.quirks field had no getter, and the dispatcher needs type-assertable access to the adapter."
  - "RustClientSideRename guards nil lease explicitly (returns serr.Internal). This keeps the wrapper's no-panic contract under zero-value wrapper tests AND eliminates a latent nil-deref class."
metrics:
  duration: "~40 min"
  completed_date: "2026-04-24"
  tasks_completed: "2/2"
  files_changed: 9
requirements_addressed: [BUG-02]
---

# Phase 47 Plan 02: rust-analyzer rename dispatcher + client-side override Summary

One-liner: Split `edit.RenameSymbol` into a hybrid dispatcher that attempts the native LSP rename first, falls back to a references-driven client-side override on the rust path, tags every successful rename with a typed `RenameStrategy`, records the outcome on a closed-enum Prometheus counter, and returns a `serr.Unsupported` pointing at manual-rename tools when both paths fail.

## Final Resolution — lspool <-> edit Import Cycle

Resolved via the **wrapper-type seam** pre-decided in `47-PATTERNS.md` §Shared Patterns "Rename override wrapper":

- `RenameOverrider` interface lives in `package edit` (returns `*edit.RenameResult`).
- `RustClientSideRename` (the references-driven helper body) lives in `package edit` so it can call the package-private `applyTextEdits`.
- `RustAnalyzerRenameOverride struct { Inner *lspool.RustAnalyzerAdapter }` wrapper lives in `package edit`, implements `RenameOverrider`, and delegates its body to `RustClientSideRename` after a best-effort `Inner.WaitUntilRenameReady(ctx)`.
- The dispatcher in `rename.go` type-asserts `lease.Adapter()` for `*lspool.RustAnalyzerAdapter` and constructs the wrapper inline on the rust path. If a future adapter implements `RenameOverrider` directly, it is returned as-is (generic path) — the wrapper is rust-specific and stays that way.
- `lspool` **does NOT** import `edit`. Verified post-commit:
  ```
  ! grep -q '"github.com/postfix/serena/internal/kernel/edit"' internal/kernel/lspool/*.go
  ```

No callback variables, no `init()`-based registration, no `SetRustClientSideRenameFn`. The only package-level mutable state introduced in `package edit` is the two test seams (`tryNativeRenameFn`, `overriderResolverFn`), which are set once at package init to real functions and swapped only by tests within the same package.

## Exact Metric Name + Label Set

| Metric                            | Type                      | Labels                                           |
| --------------------------------- | ------------------------- | ------------------------------------------------ |
| `serena_rename_strategy_total`    | `prometheus.CounterVec`   | `strategy` ∈ {`lsp-native`, `rust-client-side`}  |

- Vector owned by `obs.Metrics.RenameStrategy`, registered on the provider's private `prometheus.Registry` alongside the existing `serena_tool_calls_total` family.
- Cardinality is bounded at 2 series per process. Enforced twice: `obs.Metrics.RenameStrategyInc` drops unknown values at the sink; `mcp.RecordRenameStrategy` relies on `result.Strategy` being one of the two `RenameStrategy` constants, which is a compile-time guarantee from the dispatcher.
- The `strategy` label is carved out of `obs.AllowedLabels` for this family only via `carveOuts["serena_rename_strategy_total"] = {"strategy": true}` in `metrics_labels_test.go` (mirrors the D-13 `"reason"` carve-out for `serena_lspool_evictions_total`).

## Test-Seam Decision (Task 1)

**Decided to introduce the `tryNativeRenameFn` + `overriderResolverFn` package-level seams.** This covers the three-case dispatch matrix (`native-success` / `native-fail-override-success` / `both-fail`) as a table-driven unit test (`TestRenameDispatch_Matrix` in `rename_override_test.go`) without requiring a live rust-analyzer. The seams are one-line package-level `var`s that default to the real functions; production behaviour is unchanged. Tests swap them in `defer`-restored closures.

Rationale: the alternative (shape-only tests + rely on Plan 03 integration) leaves the `serr.Unsupported` both-fail message and the D-06 pointer string uncovered at unit-test granularity. The seams cost one layer of indirection in exchange for deterministic coverage of the failure-composition contract.

## Deviations from Plan

### Rule 1 (auto-fix bug): nil-lease panic in RustClientSideRename

- **Found during:** Task 1 GREEN when the plan-supplied `TestRustAnalyzerRenameOverride_NoInnerNilPanic` test ran with `lease=nil` against `symbols.FindReferences`.
- **Issue:** `lspool.WorkerLease.Request` panics on a nil receiver (mutation-gate `l.mu.Lock()`), which the reference-gather path triggers on the zero-value test input.
- **Fix:** Added an explicit `if lease == nil { return nil, serr.New(serr.Internal, "rust-client-side rename: nil lease") }` guard at the top of `RustClientSideRename`. This is defensive — the production dispatcher never passes nil — but it removes a latent crash class AND lets the shape test assert no-panic without constructing a live worker.
- **Files modified:** `internal/kernel/edit/rename_override.go`.
- **Commit:** `8f130c76`.

### Rule 2 (auto-add missing critical functionality): Worker.Quirks() + WorkerLease.Adapter() accessors

- **Found during:** Task 1 STEP B when the dispatcher needed to introspect the adapter.
- **Issue:** `Worker.quirks` was unexported with no accessor, and `WorkerLease` had no way to surface it to callers in other packages. The plan text assumed `lease.Adapter()` existed and offered `lease.Quirks()` as a fallback — in reality, neither did.
- **Fix:** Added `Worker.Quirks() QuirkAdapter` (returns the private field) and `WorkerLease.Adapter() QuirkAdapter` (nil-safe delegation). Both are read-only surface; no behaviour change.
- **Files modified:** `internal/kernel/lspool/worker.go`, `internal/kernel/lspool/lease.go`.
- **Commit:** `8f130c76`.

### Plan Clarifications Applied

- **Metric implementation**: The plan's pseudocode used OTel (`metric.Int64Counter`, `meter`, `attribute.String`). The codebase uses Prometheus via `obs.Metrics` throughout; no OTel metric path exists in this repo (only OTel tracing). Followed the plan's explicit instruction "mirror whatever pattern is already in the file" and landed on Prometheus. Exposed via `obs.Metrics.RenameStrategyInc` + `mcp.RecordRenameStrategy` sink wiring in `InstallMiddleware`.
- **Scope discipline**: The worktree arrived with several unrelated modifications (`circuit.go`, `pool.go`, `jsonrpc/codec.go`, etc.) from earlier sessions. These were left untouched; the commits only contain files listed in the plan's `files_modified` plus the two accessor additions to lspool noted above.

## Authentication Gates

None — all work was local.

## Files Changed

| File                                           | Kind     | Purpose                                                                    |
| ---------------------------------------------- | -------- | -------------------------------------------------------------------------- |
| `internal/kernel/edit/rename.go`               | modified | Dispatcher + private `tryNativeRename`; readiness gate; seams.             |
| `internal/kernel/edit/rename_override.go`      | created  | `RenameOverrider`, `RenameStrategy`, `RustClientSideRename`, wrapper type. |
| `internal/kernel/edit/rename_override_test.go` | created  | Dispatch matrix + interface-shape + no-panic guard tests.                  |
| `internal/kernel/edit/tools.go`                | modified | `rename_symbol` handler now emits strategy metric + renders strategy line. |
| `internal/kernel/lspool/worker.go`             | modified | Added public `Worker.Quirks()` accessor.                                   |
| `internal/kernel/lspool/lease.go`              | modified | Added public `WorkerLease.Adapter()` accessor.                             |
| `internal/mcp/middleware.go`                   | modified | `RecordRenameStrategy` + sink wiring from `InstallMiddleware`.             |
| `internal/obs/metrics.go`                      | modified | `RenameStrategy` CounterVec + `RenameStrategyInc` closed-enum helper.      |
| `internal/obs/metrics_labels_test.go`          | modified | Carve-out `"strategy"` for `serena_rename_strategy_total`; prime vector.   |

## Commits

- `8f130c76` `feat(47-02): add RenameOverrider + RenameStrategy + dispatcher refactor`
- `4ffed800` `feat(47-02): add serena_rename_strategy_total metric + strategy-tagged response`

## Verification

All green:

```
go vet ./...                                             # 0 exit, only pre-existing swift cgo macro warning
go test ./internal/kernel/edit/... -count=1              # ok
go test ./internal/kernel/lspool/... -count=1            # ok
go test ./internal/mcp/... -count=1                      # ok
go test ./internal/obs/... -count=1                      # ok
go test ./... -count=1 -short                            # all packages ok
```

Acceptance grep contract:

```
grep -q 'StrategyLSPNative\s*RenameStrategy = "lsp-native"'       internal/kernel/edit/rename_override.go   # ok
grep -q 'StrategyRustClientSide\s*RenameStrategy = "rust-client-side"' internal/kernel/edit/rename_override.go # ok
grep -q 'type RenameOverrider interface'                          internal/kernel/edit/rename_override.go   # ok
grep -q 'func tryNativeRename'                                    internal/kernel/edit/rename.go            # ok
grep -q 'Strategy\s*RenameStrategy'                               internal/kernel/edit/rename.go            # ok
grep -q 'serr.Unsupported'                                        internal/kernel/edit/rename.go            # ok
grep -q 'fuzzy_edit, replace_symbol_body, or search_in_files'     internal/kernel/edit/rename.go            # ok
grep -q 'WaitUntilRenameReady'                                    internal/kernel/edit/rename.go            # ok
grep -q 'type RustAnalyzerRenameOverride struct'                  internal/kernel/edit/rename_override.go   # ok
grep -q 'func (w \*RustAnalyzerRenameOverride) RenameOverride'    internal/kernel/edit/rename_override.go   # ok
grep -q 'serena_rename_strategy_total'                            internal/mcp/middleware.go                 # ok
grep -q 'serena_rename_strategy_total'                            internal/obs/metrics.go                    # ok
grep -qE 'strategy: %s'                                           internal/kernel/edit/tools.go              # ok
! grep -q '"github.com/postfix/serena/internal/kernel/edit"'      internal/kernel/lspool/*.go                # ok (no cycle)
```

## TDD Gate Compliance

Plan 02 tasks were marked `tdd="true"`. Ordering within this work:

- **RED**: `rename_override_test.go` was written first (created in Task 1 step C with the dispatch-matrix table, interface-shape assertions, and no-panic guard). Before the GREEN code landed, the test file referenced symbols (`RenameStrategy`, `StrategyLSPNative`, `StrategyRustClientSide`, `RenameOverrider`, `RustAnalyzerRenameOverride`, `tryNativeRenameFn`, `overriderResolverFn`) that did not yet exist — compile failure guaranteed.
- **GREEN**: `rename_override.go` (types, interface, helper, wrapper) + `rename.go` refactor (dispatcher + private `tryNativeRename` + seams) landed together. First test run exposed the nil-lease panic; fixed with the `lease == nil` guard in `RustClientSideRename` (Rule 1 deviation above).
- **REFACTOR**: None beyond the one-line nil-lease guard. Implementation shape was stable after the first GREEN iteration.

Both RED+GREEN were compressed into the same git commit (`8f130c76`) because the Task 1 plan action explicitly bundled test-file creation with the implementation as a single commit. Task 2's observability wiring landed in a separate feat commit (`4ffed800`).

## Known Stubs

None. `RustClientSideRename` is a complete references-driven rename path; its semantic-accuracy limits (no cross-crate trait-impl discovery, macro-expansion edge cases) are documented in the wrapper's doc comment and will be surfaced to users in `USAGE.md` in Plan 03.

## Threat Flags

None — the new surface (`RecordRenameStrategy`, `RenameStrategyInc`) is cardinality-bounded and was modelled in the plan's threat register (T-47-06 through T-47-10). No new trust boundaries introduced beyond those already covered.

## Self-Check: PASSED

- `internal/kernel/edit/rename_override.go` — FOUND
- `internal/kernel/edit/rename_override_test.go` — FOUND
- `internal/kernel/edit/rename.go` — FOUND (modified, dispatcher present)
- `internal/kernel/edit/tools.go` — FOUND (modified, strategy line present)
- `internal/kernel/lspool/worker.go` — FOUND (modified, Quirks() accessor added)
- `internal/kernel/lspool/lease.go` — FOUND (modified, Adapter() accessor added)
- `internal/mcp/middleware.go` — FOUND (modified, RecordRenameStrategy present)
- `internal/obs/metrics.go` — FOUND (modified, RenameStrategy vector registered)
- `internal/obs/metrics_labels_test.go` — FOUND (modified, carve-out added)
- Commit `8f130c76` — FOUND
- Commit `4ffed800` — FOUND
