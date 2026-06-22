---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
plan: 01
subsystem: infra
tags: [kernel, ablation, structured-edit, serr-unsupported, kernelconfig, tdd]

# Dependency graph
requires:
  - phase: 60-semantic-live-service
    provides: EditNotifier nil-check contract reused by the no-op LSP wiring path
  - phase: 53-edit-outcome-telemetry
    provides: RecordEditOutcome + ClassifyEditError outcome classification reused by the guards
provides:
  - kernel.KernelConfig.DisableLSPSubsystem (bool field, consumed by Plan 76-04)
  - kernel.KernelConfig.DisableStructuredEditSubsystem (bool field)
  - Kernel.LSPSubsystemDisabled() / Kernel.StructuredEditDisabled() accessors
  - Greppable subsystem_disabled: error-message convention on serr.Unsupported (D-06)
  - Unsupported runtime guard on the 4 structured-edit tools (replace_symbol_body, fuzzy_edit, insert_before_symbol, insert_after_symbol)
  - replace_in_file exact-match-only behavior under the structured-edit flag (D-05)
affects: [76-02-vet-ablation-leakage, 76-03, 76-04-no-lsp-daemon-wiring, bench-no-structured-edit]

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "Kernel subsystem-disable flag on KernelConfig + accessor (extend-in-place, no new struct)"
    - "Runtime ablation backstop: serr.Unsupported guard with greppable subsystem_disabled: prefix at handler entry"
    - "In-process tool-handler test harness via mcpsdk.NewInMemoryTransports (mirrors internal/mcp/server_trace_test.go)"

key-files:
  created:
    - internal/kernel/kernel_disable_flags_test.go
    - internal/kernel/edit/structured_edit_disabled_test.go
    - internal/kernel/fileops/replace_in_file_disabled_test.go
  modified:
    - internal/kernel/kernel.go
    - internal/errors/kinds.go
    - internal/kernel/edit/tools.go
    - internal/kernel/fileops/tools.go

key-decisions:
  - "Reused serr.Unsupported kind (no new kind) + standardized greppable subsystem_disabled: message prefix (D-06)"
  - "fuzzy_edit guard placed in internal/kernel/fileops/tools.go where the tool actually lives (plan named it under edit but it is registered by fileops)"
  - "Drove handlers through the in-memory SDK transport rather than the underlying functions, so the handler-entry guard itself is exercised end-to-end"

patterns-established:
  - "Pattern 1: subsystem-disable flag on KernelConfig with a one-line accessor next to Tracer()"
  - "Pattern 2: handler-entry Unsupported guard returning errorResult(serr.New(serr.Unsupported, \"subsystem_disabled: ...\").WithTool(...).Error())"
  - "Pattern 3: extend the fuzzy-fallback guard with && !k.StructuredEditDisabled() to fall through to the existing plain return"

requirements-completed: [ABLATE-07]

# Metrics
duration: ~35min
completed: 2026-06-16
---

# Phase 76 Plan 01: Kernel Subsystem-Disable Flags + Structured-Edit Unsupported Guard Summary

**Two opt-in KernelConfig disable flags (DisableLSPSubsystem, DisableStructuredEditSubsystem) with accessors, plus a runtime serr.Unsupported backstop on the four structured-edit tools and exact-match-only replace_in_file under the structured-edit flag (ABLATE-07).**

## Performance

- **Duration:** ~35 min
- **Started:** 2026-06-16T15:57:00Z (approx)
- **Completed:** 2026-06-16T16:32:19Z
- **Tasks:** 3 (all TDD: RED → GREEN)
- **Files modified:** 4 source + 3 new test files

## Accomplishments
- Extended `kernel.KernelConfig` with `DisableLSPSubsystem` (ABLATE-05, consumed by Plan 76-04) and `DisableStructuredEditSubsystem` (ABLATE-07), both defaulting to false (opt-in disable, D-02), plus `LSPSubsystemDisabled()` / `StructuredEditDisabled()` accessors next to `Tracer()`.
- Documented the greppable `subsystem_disabled:` message convention on the `Unsupported` kind in `internal/errors/kinds.go` — no new error kind (D-06).
- Added the runtime Unsupported guard to all four structured-edit handlers: `replace_symbol_body`, `insert_before_symbol`, `insert_after_symbol` (in `edit/tools.go`) and `fuzzy_edit` (in `fileops/tools.go`). Each returns `serr.Unsupported` with a `subsystem_disabled:` prefix when the flag is set, sitting after arg validation but before any workspace/session acquisition.
- Made `replace_in_file` exact-match-only under the flag by extending the fuzzy-fallback guard with `&& !k.StructuredEditDisabled()` (D-05): a literal no-match falls through to the existing plain `"0 replacement(s) made"` return with no `match_strategy:` line.

## Task Commits

Each task was committed atomically (TDD test → feat gates):

1. **Task 1: KernelConfig flags + accessors** - `e88378f2` (test) → `f59aad1d` (feat)
2. **Task 2: Structured-edit Unsupported guard (edit pkg)** - `0f82b456` (test) → `37d1bf9f` (feat)
3. **Task 3: replace_in_file exact-match-only + fuzzy_edit guard (fileops pkg)** - `8e9a5985` (test) → `250e206c` (feat)

_Note: the fuzzy_edit Unsupported guard (a Task 2 D-04 concern by topic) was committed with Task 3 because fuzzy_edit is registered by the fileops package; its test `TestFuzzyEditDisabled` lives in the Task 3 fileops test file._

## Files Created/Modified
- `internal/kernel/kernel.go` - added two bool fields to KernelConfig + two accessors
- `internal/errors/kinds.go` - documented the subsystem_disabled: convention on the Unsupported kind (no new kind)
- `internal/kernel/edit/tools.go` - Unsupported guard on replace_symbol_body / insert_before_symbol / insert_after_symbol
- `internal/kernel/fileops/tools.go` - Unsupported guard on fuzzy_edit + exact-match-only fuzzy-fallback guard on replace_in_file
- `internal/kernel/kernel_disable_flags_test.go` - RED-first accessor test (TestKernelDisableFlags)
- `internal/kernel/edit/structured_edit_disabled_test.go` - RED-first guard test (TestStructuredEditDisabled)
- `internal/kernel/fileops/replace_in_file_disabled_test.go` - RED-first exact-match-only + fuzzy_edit guard test

## Decisions Made
- **Reuse serr.Unsupported, standardize a greppable message prefix (D-06):** no redundant error kind; the `subsystem_disabled:` prefix gives the `vet-ablation-leakage` analyzer and graders a concrete token to grep.
- **fuzzy_edit guard location:** the plan's Task 2 named four tools under `edit/tools.go`, but `fuzzy_edit` is registered by `internal/kernel/fileops`. The guard and its test were placed there; the plan frontmatter already lists `fileops/tools.go` in `files_modified`, so this is consistent with the plan's file set.
- **In-process handler testing via the SDK in-memory transport:** existing tests verified guard-adjacent behavior on the underlying functions (FuzzyEdit/ReplaceInFile) or via source-grep. To exercise the handler-entry guard itself, the new tests drive the registered tools through `mcpsdk.NewInMemoryTransports` (the harness shape from `internal/mcp/server_trace_test.go`). This works without a live language server because the guard returns before any LSP lease.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Passed a noop tracer to fileops.RegisterTools in the Task 3 test**
- **Found during:** Task 3 (replace_in_file test harness)
- **Issue:** `fileops.RegisterTools` takes a `trace.Tracer` parameter (unlike `edit.RegisterTools`, which derives it from `k.Tracer()` internally). Passing `nil` caused `WrapToolSpan`'s `tracer.Start` to panic at `spanwrap.go:30`.
- **Fix:** Constructed a `tracenoop.NewTracerProvider().Tracer("test")` and passed it into `RegisterTools` in the test harness. Test-only change; no production code affected.
- **Files modified:** internal/kernel/fileops/replace_in_file_disabled_test.go
- **Verification:** Test runs cleanly; GREEN passes.
- **Committed in:** 8e9a5985 (Task 3 RED test commit)

---

**Total deviations:** 1 auto-fixed (1 blocking, test-harness only)
**Impact on plan:** No production-code scope creep. All planned source edits landed exactly as specified.

## Issues Encountered
- The plan's `edit/structured_edit_disabled_test.go` Task 2 behavior listed `fuzzy_edit` as one of the four tools, but `fuzzy_edit` is not registered by the `edit` package. Resolved by covering `fuzzy_edit` in the fileops test (`TestFuzzyEditDisabled`) where the tool and its guard live. The edit-package test covers the three edit-package structured-edit tools.

## Known Stubs
None — both flags are fully wired to their accessors and all four guards are live. `DisableLSPSubsystem` is intentionally defined here but consumed by Plan 76-04 (per plan objective and D-12); this is documented forward-wiring, not a stub.

## Threat Flags
None — no new network endpoints, auth paths, or schema changes. The change reduces reachable surface (adds refusal guards) consistent with the plan's threat register (T-76-01 mitigate via D-04 guard, T-76-02 mitigate via D-05 fuzzy-fallback guard).

## Next Phase Readiness
- `LSPSubsystemDisabled()` accessor is ready for Plan 76-04 (no_lsp daemon wiring) to consume without re-touching kernel.go.
- `StructuredEditDisabled()` runtime backstop is in place for the `bench-no-structured-edit` arm and gives `vet-ablation-leakage` (Plan 76-02/76-03) a concrete enforcement floor.
- No blockers.

## Self-Check: PASSED

All 8 created/modified files verified present on disk; all 6 task commits (3 test + 3 feat) verified in git history. `go vet ./internal/kernel/... ./internal/errors/...` clean; `go build ./...` clean; `go test ./internal/kernel/ ./internal/kernel/edit/ ./internal/kernel/fileops/ -count=1` passes.

---
*Phase: 76-ablation-profiles-kernel-subsystem-disable-flags*
*Completed: 2026-06-16*
