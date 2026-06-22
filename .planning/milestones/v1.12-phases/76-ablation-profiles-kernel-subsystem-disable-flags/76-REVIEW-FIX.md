---
phase: 76-ablation-profiles-kernel-subsystem-disable-flags
fixed_at: 2026-06-16T00:00:00Z
review_path: .planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/76-REVIEW.md
iteration: 1
findings_in_scope: 6
fixed: 4
skipped: 2
status: partial
---

# Phase 76: Code Review Fix Report

**Fixed at:** 2026-06-16
**Source review:** .planning/phases/76-ablation-profiles-kernel-subsystem-disable-flags/76-REVIEW.md
**Iteration:** 1

**Summary:**
- Findings in scope: 6 (WR-01, WR-02, WR-03, IN-01, IN-02, IN-03)
- Fixed: 4 (WR-01, WR-02, WR-03, IN-01)
- Skipped: 2 (IN-02, IN-03)

All fixes were applied in an isolated git worktree and fast-forwarded onto
the working branch. Verification (`go build ./...`, `go vet ./...`, and the
affected-package test suites) is green. The pre-existing `test/bench`
53-vs-47 tool-drift failure is unrelated and was not touched.

## Fixed Issues

### WR-01: `outcome="unsupported"` dropped by the closed edit-outcome enum

**Files modified:** `internal/obs/metrics.go`, `internal/mcp/middleware.go`, `internal/mcp/record_edit_outcome_test.go`, `internal/obs/metrics_labels_test.go`
**Commit:** 387c90f3
**Applied fix:** Took option (a) from the review (preferred). Added
`"unsupported"` to the closed edit-outcome enum in both the `mcp`
middleware (`editOutcomeEnum` + new `editOutcomeUnsupported` const) and the
`obs` `EditOutcomeInc` switch, so the four structured-edit ablation guards'
emissions are now counted instead of silently dropped. Lifted the
cardinality bound 168 → 196 (7 tools × 7 outcomes × 4 strategies) and
updated `TestMetrics_CardinalityBounds_EditOutcome`. Updated
`TestEditOutcomeEnumForTest_Closed` to the 7-value vocabulary and added
`TestMetrics_EditOutcomeInc_UnsupportedCounted` asserting the value is
emitted (not dropped).

### WR-02: no kernel runtime guard on LS-leasing symbol/rename tools under `DisableLSPSubsystem`

**Files modified:** `internal/kernel/symbols/tools.go`, `internal/kernel/symbols/lsp_disabled_test.go` (new), `internal/kernel/edit/tools.go`, `internal/kernel/edit/structured_edit_disabled_test.go`
**Commit:** 9743b46a
**Applied fix:** Applied the contained guard (not deferred), because the nine
symbol-retrieval handlers (`go_to_definition`, `find_references`, hover,
implementations, call/type hierarchy, blast radius) all route through a single
shared chokepoint, `acquireLease(ctx, k, wsKeyFn)` in `symbols/tools.go`. A
single `k.LSPSubsystemDisabled()` guard there closes the entire direct
symbol-tool surface with one check, mirroring the `StructuredEditDisabled()`
backstop. `rename_symbol` leases directly (does not use `acquireLease`), so it
got a handler-level guard emitting `outcome="unsupported"` for the edit metric.
Both refuse with `serr.Unsupported` carrying the greppable `subsystem_disabled:`
prefix before `AcquireSession`. Added tests: `TestAcquireLeaseLSPDisabled`
(flag ON → Unsupported before workspace lookup; flag OFF → falls through to
NoWorkspace) and `TestRenameSymbolLSPDisabled` (flag ON → Unsupported; flag
OFF → guard not hit). This was feasible as the smallest correct fix thanks to
the existing chokepoint, so it was applied rather than deferred.

**Requires human verification:** the guard placement is a behavioral/logic
change to the LS-leasing path; please confirm the chokepoint covers every
intended tool and that the flag-OFF live arm is unaffected (tests assert
both, but the live LS path is not exercised end-to-end in this phase's
harness).

### WR-03: `ProfileStore.Validate()` not re-run after `LoadOverrides`

**Files modified:** `internal/config/loader.go`, `internal/config/loader_test.go`
**Commit:** 35889806
**Applied fix:** In `ResolveProfile`, stopped swallowing the override-load
error (`_ = profile.LoadOverrides(...)` → surfaced as a wrapped error) and
re-ran `store.Validate()` after overrides are applied, surfacing its result.
A disk override that introduces an unknown `default_mode` / transition (or a
new profile) now fail-closes at resolve time instead of silently producing a
nil allow-list at session start. Added `TestResolveProfile_OverrideUnknownModeRejected`
(mirrors the embedded unknown-mode test from 76-02, exercising the override
path) and `TestResolveProfile_MalformedOverrideSurfaced` (the parse error is
no longer swallowed).

### IN-01: redundant comma-ok blank in `DefaultProfile()`

**Files modified:** `internal/profile/profile.go`
**Commit:** f37adf5f
**Applied fix:** `p, _ := s.profiles["full"]; return p` → `return s.profiles["full"]`.

## Skipped Issues

### IN-02: racy `activeWSLang` closure capture in diag `leaseFn`

**File:** `internal/daemon/daemon.go:648-654`
**Reason:** skipped by design — pre-existing pattern, out of phase scope.
The by-reference capture of `activeWSKey` / `activeWSLang` is a pre-existing
pattern shared with the repomap enrich/fallback closures (acknowledged by the
CR-03 comment at `daemon.go:824-828` as intentional and LazyInit-ordered),
NOT introduced by Phase 76. It is dead under the no_lsp arm (the stub
`leaseFn` ignores both variables). The review itself classifies it as "not
independently actionable here." Fixing it would touch unrelated live-arm
behavior and risk regressing the working enabled arm; the documented remedy
(promote to a mutex-guarded/atomic holder) is deferred to a future
multi-workspace change. Recorded as skipped-with-rationale per finding
guidance.

### IN-03: `vet-ablation-leakage` proven only by testdata until Phase 80

**File:** `internal/lint/ablationleakage/analyzer.go:24-26`, `Makefile:60-61`
**Reason:** skipped — informational by design, non-actionable. The analyzer
correctly restricts its scan to the `github.com/agenthands/helix/bench/runners`
namespace, which does not yet exist; this is an explicitly documented D-08
honest-scope decision and the slash-boundary lookalike regression is already
covered. The review's "Fix" is a Phase 80 forward-action (land a real
`bench/runners/*` package), not a Phase 76 change. Nothing to fix here.

---

_Fixed: 2026-06-16_
_Fixer: Claude (gsd-code-fixer)_
_Iteration: 1_
