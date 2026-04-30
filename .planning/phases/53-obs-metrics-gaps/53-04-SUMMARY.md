---
phase: 53-obs-metrics-gaps
plan: 04
subsystem: observability
tags: [observability, edit, fileops, mcp, metrics, helix]
requires: [53-01]
provides:
  - mcp.RecordEditOutcome package-level sink (atomic.Pointer setter pattern, Phase 53 D-16)
  - mcp.EditOutcomeEnumForTest, mcp.StrategyEnumForTest, mcp.SetEditOutcomeSinkForTest accessors
  - edit.ClassifyEditError shared outcome classifier (used by both edit + fileops handlers)
  - 7 edit/fileops tool handlers emitting helix_edit_outcome_total at return
  - fuzzy.ErrNoMatch + fuzzy.ErrAmbiguous exported error sentinels
affects:
  - daemon-time wiring (InstallMiddleware now wires both renameStrategySink AND editOutcomeSink)
  - downstream Plan 53-06: USAGE.md will document the new metric + its labels
tech-stack:
  added: []
  patterns:
    - parallel atomic.Pointer setter sink (mirror of Phase 47 D-07 RecordRenameStrategy)
    - closed-enum drop-unknown discipline at emission (provider.Metrics().EditOutcomeInc layer)
    - shared classifier function (edit.ClassifyEditError) reused across packages — DRY for the D-10 outcome map
    - sentinel-error wrapping via serr.Wrap to satisfy BOTH errors.Is(err, fuzzy.ErrNoMatch) and errors.Is(err, serr.ErrInvalidArgs)
key-files:
  created:
    - internal/mcp/record_edit_outcome_test.go (+141 lines)
    - internal/kernel/edit/outcome_emission_test.go (+155 lines)
    - internal/kernel/fileops/outcome_emission_test.go (+97 lines)
  modified:
    - internal/fuzzy/match.go (+29 lines: 2 sentinels + ambiguityError/failureError refactor)
    - internal/fuzzy/fuzzy_test.go (+26 lines: 2 sentinel tests)
    - internal/mcp/middleware.go (+127 lines: sink + InstallMiddleware wiring + 2 closed enums + 3 ForTest accessors)
    - internal/kernel/edit/tools.go (+191 lines: ClassifyEditError + appendVerifyInfoWithStatus + 5 handler instrumentations)
    - internal/kernel/fileops/tools.go (+45 lines: 2 handler instrumentations + edit-package import)
decisions:
  - "Wave 0 commit introduced fuzzy.ErrNoMatch and fuzzy.ErrAmbiguous sentinels BEFORE Task 4.2 instrumentation (per W4 advisory). The classifier-based approach in 53-04-PLAN.md was unworkable without these because both no-match and ambiguity flowed through serr.New(serr.InvalidArgs) with only message text to distinguish them."
  - "ls_error and validation_failed are NOT classified by edit.ClassifyEditError — they are set directly at the call sites that produce them (acquire-session/didChange paths for ls_error; appendVerifyInfoWithStatus boolean flip for validation_failed). The classifier docstring records this v1.2 limitation."
  - "Q-3 under-classification preserved: missing-required-field validations bucket as outcome=internal — keeps the locked D-10 6-value enum; v1.3 will add typed errors to refine."
  - "Q-4 enforced structurally: the strategy enum is 4 values {exact, whitespace_normalized, indentation_flexible, none}. fuzzy.StrategyFailed.String() (\"failed\") never reaches mcp.RecordEditOutcome because Match returns ErrNoMatch on that branch, classifier maps to outcome=no_match, and strategy stays at its initial \"none\" value. Verified by:\n  ! grep -qE 'EditOutcomeInc\\([^)]*\"failed\"\\)' internal/kernel/{edit,fileops}/tools.go\n  ! grep -qE 'strategy *[:=] *\"failed\"' internal/kernel/{edit,fileops}/tools.go"
  - "Phase 47 D-07 helix_rename_strategy_total contract preserved verbatim — rename_symbol calls BOTH mcp.RecordRenameStrategy (Phase 47) AND mcp.RecordEditOutcome (Phase 53) per D-11."
  - "edit.ClassifyEditError is exported (capital C) to allow internal/kernel/fileops/ to reuse it without duplication — DRY for the D-10 outcome map. No import cycle: edit/ does not import fileops/."
  - "Refactored appendVerifyInfo into appendVerifyInfoWithStatus that returns (text, hasErrors). The original appendVerifyInfo is preserved as a thin wrapper for backwards compatibility."
metrics:
  duration: "~50 minutes"
  completed: "2026-04-30"
  tasks: 4 (Wave 0 + 3 plan tasks)
  files_modified: 8
  lines_added: 800
---

# Phase 53 Plan 04: Edit-Tool Outcome Emission Summary

**One-liner:** Wired `helix_edit_outcome_total` emission into all 7 edit + fileops tool handlers via a parallel `mcp.RecordEditOutcome` sink mirroring Phase 47 D-07's atomic.Pointer pattern; introduced `fuzzy.ErrNoMatch` / `fuzzy.ErrAmbiguous` sentinels in a Wave 0 commit so the classifier can programmatically distinguish the two paths.

## What Landed

### Wave 0: Error sentinels (`feat 311fcaa4`)

The plan's `classifyEditError` design depended on `errors.Is(err, fuzzy.ErrNoMatch)` and `errors.Is(err, fuzzy.ErrAmbiguous)`. Neither sentinel existed — both paths flowed through `serr.New(serr.InvalidArgs, "no fuzzy match found")` and `serr.New(serr.InvalidArgs, "ambiguous match: ...")` distinguished only by message text. Per the `.continue-here.md` W4 advisory, I split these into a separate Wave 0 commit BEFORE the Task 4.2 instrumentation:

- `internal/fuzzy/match.go`: added `ErrNoMatch` and `ErrAmbiguous` exported sentinels.
- Refactored `failureError(...)` to use `serr.Wrap(serr.InvalidArgs, ..., ErrNoMatch)` so `errors.Is(err, fuzzy.ErrNoMatch)` AND `errors.Is(err, serr.ErrInvalidArgs)` both work.
- Refactored `ambiguityError(...)` likewise wrapping `ErrAmbiguous`.
- Added `TestMatch_ErrNoMatchSentinel` and `TestMatch_ErrAmbiguousSentinel` proving both `errors.Is` checks coexist.

The legacy `Kind=InvalidArgs` matchers (e.g. existing tests) continue to work — the wrap chain preserves Kind via `*Error.Is`.

### Task 4.1: RecordEditOutcome sink (`feat 18868d69`)

`internal/mcp/middleware.go`:
- `editOutcomeSink atomic.Pointer[func(...)]` + `setEditOutcomeSink` + `RecordEditOutcome` — exact-shape mirror of Phase 47 `renameStrategySink` block at lines 18–44.
- `InstallMiddleware` wires the new sink alongside the existing rename sink under the same `provider != nil { if m := provider.Metrics(); m != nil ... }` guard.
- `editOutcomeEnum` (6 values per D-10) and `strategyEnum` (4 values per Q-4) closed-enum constants.
- `EditOutcomeEnumForTest`, `StrategyEnumForTest`, `SetEditOutcomeSinkForTest` accessors.

`internal/mcp/record_edit_outcome_test.go` (NEW):
- `TestRecordEditOutcome` with 3 sub-tests:
  1. `noop_without_sink`: safe no-op when no sink wired.
  2. `roundtrip_through_installed_sink`: closure observes the (toolName, outcome, strategy) triple verbatim.
  3. `install_middleware_routes_to_obs_metrics`: after `InstallMiddleware` runs with a real `obs.Provider`, `RecordEditOutcome` increments `helix_edit_outcome_total` on the provider's registry — round-trip via the canonical wiring path.
- `TestEditOutcomeEnumForTest_Closed`: 6-value vocabulary preserved.
- `TestStrategyEnumForTest_Closed`: 4-value vocabulary; `"failed"` MUST be absent (Q-4 enforcement).

### Task 4.2: Edit-package handlers (`feat 1c869c29`)

`internal/kernel/edit/tools.go`:
- `ClassifyEditError(err) string` — exported; the single source of truth for the D-10 outcome bucket map. Uses `errors.Is(err, fuzzy.ErrAmbiguous)` and `errors.Is(err, fuzzy.ErrNoMatch)` (Wave 0 sentinels) for fuzzy classification. All other errors map to `"internal"` per Q-3 under-classification.
- `appendVerifyInfoWithStatus(ctx, diagStore, uri, text) (string, bool)` — refactored from `appendVerifyInfo`. The boolean signals whether the post-edit verifier reported `VerifyResult.HasErrors`. Handlers use it to flip outcome from `success` to `validation_failed` on the otherwise-clean-edit path.
- 5 handlers wrapped with the canonical defer pattern:
  ```go
  outcome, strategy := "success", "none"
  defer func() { mcp.RecordEditOutcome(ctx, "<tool>", outcome, strategy) }()
  ```
  - `replace_symbol_body`: strategy from `fuzzyInfo.Strategy` on the fuzzy success path; `"none"` otherwise.
  - `insert_before_symbol`, `insert_after_symbol`, `safe_delete_symbol`, `rename_symbol`: strategy stays `"none"` throughout (non-fuzzy tools).
  - LS-side acquisition/notify failures classify directly to `"ls_error"` at the call site (the classifier does not infer ls_error from generic errors — see "ls_error v1.2 limitation" below).
  - `safe_delete_symbol` refused-with-references is bucketed as `success` (the tool did exactly what it promised — refused to delete a referenced symbol).

`internal/kernel/edit/outcome_emission_test.go` (NEW):
- `TestEditTools_OutcomeEmission` with 10 sub-tests pinning the per-handler outcome/strategy contract and the Q-4 invariant.
- `TestClassifyEditError_NoFailedStrategyEverEmitted`: structural invariant.

### Task 4.3: Fileops-package handlers (`feat e502557c`)

`internal/kernel/fileops/tools.go`:
- Imported `internal/kernel/edit` and reused `edit.ClassifyEditError` directly — DRY: no duplication of the D-10 bucket map. No import cycle (edit/ does not import fileops/).
- `registerReplaceInFile`:
  - Strategy = `"exact"` on the literal-match success path (count > 0).
  - Strategy = `string(fResult.Strategy)` on the fuzzy fallback success path.
  - Errors classify via `edit.ClassifyEditError`.
- `registerFuzzyEdit`:
  - Always runs `fuzzy.Match`. Success path sets strategy = `string(result.Strategy)`.
  - Errors classify via `edit.ClassifyEditError`. `fuzzy.ErrNoMatch` correctly buckets as `no_match` with strategy staying `"none"` — `fuzzy.StrategyFailed.String()` never reaches the metric (Q-4).

`internal/kernel/fileops/outcome_emission_test.go` (NEW):
- `TestFileopsTools_OutcomeEmission` with 8 sub-tests covering both handlers.

## Where ClassifyEditError lives

- **Authoritative implementation:** `internal/kernel/edit/tools.go::ClassifyEditError` (exported).
- **Reused by:** `internal/kernel/fileops/tools.go` via `import "github.com/agenthands/helix/internal/kernel/edit"` and direct call to `edit.ClassifyEditError(err)`.
- **No duplication:** the bucket map exists in exactly one place. Future bucket changes (e.g. v1.3 typed-error classification) flow into both packages automatically.
- **No cycle:** `internal/kernel/edit/` does NOT import `internal/kernel/fileops/`. Verified via `grep "fileops" internal/kernel/edit/*.go` returning only doc-comment references.

## Actual error sentinels used in ClassifyEditError

The plan's draft listed `fuzzy.ErrNoMatch`, `fuzzy.ErrAmbiguous`, `verify.ErrValidationFailed`, and `edit.ErrLSError`. Of these, only the first two were practical to introduce:

| Sentinel | Status | Notes |
|---|---|---|
| `fuzzy.ErrNoMatch` | **Introduced (Wave 0)** | wraps the failureError() return |
| `fuzzy.ErrAmbiguous` | **Introduced (Wave 0)** | wraps the ambiguityError() return |
| `verify.ErrValidationFailed` | **Not introduced** — handled differently | `VerifyEdit` does NOT return validation failures as errors. The verifier returns a `*VerifyResult` with `HasErrors=true` on a successful (non-error) call. Bridged via the new `appendVerifyInfoWithStatus(...) (string, bool)` helper, which lets handlers flip outcome to `validation_failed` based on the boolean. |
| `edit.ErrLSError` | **Not introduced** — handled by direct call-site assignment | LS-side failures (acquire-session, didChange, apply rename edits) are not currently typed as a distinct sentinel. Introducing one would require refactoring most of `internal/kernel/edit/` and `internal/kernel/lspool/`. v1.2 ships `ls_error` as a direct call-site assignment at the known LS boundaries (`outcome = "ls_error"` immediately after acquire-session / lease-acquisition errors). The classifier's docstring documents this. v1.3 will introduce typed LS-error sentinels. |

`ClassifyEditError` therefore returns one of: `success`, `no_match`, `ambiguous_match`, `internal`. The remaining two buckets (`ls_error`, `validation_failed`) are set directly by the handler at the appropriate call site.

## Confirmation: StrategyFailed never propagates as a label value

Q-4 invariant proven by:

1. **Source-level structural check:**
   ```sh
   ! grep -qE 'EditOutcomeInc\([^)]*"failed"\)' internal/kernel/{edit,fileops}/tools.go
   ! grep -qE 'strategy *[:=] *"failed"' internal/kernel/{edit,fileops}/tools.go
   grep -q '"failed"' internal/kernel/{edit,fileops}/tools.go  # 0 matches
   ```
2. **Closed-enum guard at the obs.Metrics layer:** `EditOutcomeInc(toolName, outcome, strategy)` drops unknown values silently per Plan 53-01. `strategyEnum` does not contain `"failed"`.
3. **Code-flow guarantee:** `fuzzy.Match` returns `(*Result, error)`. On the failed cascade, it returns `(nil, ErrNoMatch-wrapped)` BEFORE producing a `Result` with `Strategy=StrategyFailed`. Handlers only set `strategy = string(...)` from the `Result.Strategy` (or `fuzzyInfo.Strategy`), so by construction strategy can only be one of the three real cascade tiers on the success branch.
4. **Test invariant:** `TestClassifyEditError_NoFailedStrategyEverEmitted` and `TestStrategyEnumForTest_Closed` document the contract.

## Test results

```text
$ go vet ./internal/mcp/... ./internal/kernel/edit/... ./internal/kernel/fileops/... ./internal/obs/... ./internal/fuzzy/...
ok (no errors; only unrelated swift binding warnings)

$ go test ./internal/mcp/... ./internal/kernel/edit/... ./internal/kernel/fileops/... ./internal/obs/... ./internal/fuzzy/...
ok      github.com/agenthands/helix/internal/fuzzy             0.196s
ok      github.com/agenthands/helix/internal/mcp               (cached)
ok      github.com/agenthands/helix/internal/kernel/edit       1.005s
ok      github.com/agenthands/helix/internal/kernel/fileops    0.639s
ok      github.com/agenthands/helix/internal/obs               (cached)

$ go build ./cmd/helix
(builds successfully)

$ go test ./internal/mcp/... -run TestRecordEditOutcome -v
=== RUN   TestRecordEditOutcome
=== RUN   TestRecordEditOutcome/noop_without_sink
=== RUN   TestRecordEditOutcome/roundtrip_through_installed_sink
=== RUN   TestRecordEditOutcome/install_middleware_routes_to_obs_metrics
--- PASS: TestRecordEditOutcome (0.00s)
PASS

$ go test ./internal/kernel/edit/... -run TestEditTools_OutcomeEmission -v
[10 sub-tests, all PASS]

$ go test ./internal/kernel/fileops/... -run TestFileopsTools_OutcomeEmission -v
[8 sub-tests, all PASS]
```

## Acceptance criteria

| Criterion | Status |
|---|---|
| `RecordEditOutcome` is the single emission entry point for edit + fileops tools (D-16) | PASS |
| 5 edit-package handlers wired with classifier-based outcome + strategy | PASS (count: 5 `mcp.RecordEditOutcome` calls in edit/tools.go) |
| 2 fileops-package handlers wired | PASS (count: 2 `mcp.RecordEditOutcome` calls in fileops/tools.go) |
| StrategyFailed remap to no_match/none, never propagated | PASS (structural grep + closed-enum guard + code-flow proof) |
| Phase 47 D-07 contract preserved | PASS (RecordRenameStrategy + renameStrategySink + tools.go:397 emission untouched) |
| `go vet` ./internal/mcp/... ./internal/kernel/edit/... ./internal/kernel/fileops/... | PASS |
| `go test` same packages | PASS |
| `go build ./cmd/helix` | PASS |

## Commits

| Step | Type | Hash | Message |
|---|---|---|---|
| Wave 0 | feat | `311fcaa4` | add fuzzy.ErrNoMatch + fuzzy.ErrAmbiguous sentinels |
| Task 4.1 | feat | `18868d69` | add RecordEditOutcome sink + closed-enum constants |
| Task 4.2 | feat | `1c869c29` | instrument 5 edit handlers with helix_edit_outcome_total |
| Task 4.3 | feat | `e502557c` | instrument 2 fileops handlers with helix_edit_outcome_total |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Wave 0 sentinel introduction**

- **Found during:** Plan reconnaissance (before Task 4.2 instrumentation).
- **Issue:** The plan's `classifyEditError` helper depends on `errors.Is(err, fuzzy.ErrNoMatch/ErrAmbiguous)` and `errors.Is(err, verify.ErrValidationFailed/edit.ErrLSError)`. None of these sentinels existed — both fuzzy paths flowed through `serr.New(serr.InvalidArgs, ...)` distinguished only by message text. The `.continue-here.md` W4 advisory mandated splitting sentinel introduction into its own Wave 0 commit if missing. Without sentinels, the instrumentation commit would silently introduce them, conflating two concerns.
- **Fix:** Wave 0 commit (`311fcaa4`) introduces `fuzzy.ErrNoMatch` and `fuzzy.ErrAmbiguous`, refactors `failureError`/`ambiguityError` to wrap them, and adds two sentinel tests. Tasks 4.2 and 4.3 then depend on these sentinels via `errors.Is`. The other two sentinels (`verify.ErrValidationFailed`, `edit.ErrLSError`) were NOT introduced — see "Actual error sentinels used" above for rationale.
- **Files modified:** internal/fuzzy/match.go, internal/fuzzy/fuzzy_test.go.
- **Commit:** `311fcaa4`.

**2. [Rule 2 - Critical functionality] appendVerifyInfo refactor for validation_failed**

- **Found during:** Task 4.2 (RESEARCH §"Edit-Tool Outcome Map" requires `validation_failed` outcome on `VerifyResult.HasErrors`).
- **Issue:** The original `appendVerifyInfo` returned only the formatted string and discarded the `vr.HasErrors` boolean. Without that signal, handlers cannot flip `outcome` from `success` to `validation_failed` per D-10.
- **Fix:** Added `appendVerifyInfoWithStatus(...) (string, bool)` returning both. The original `appendVerifyInfo` is preserved as a thin wrapper for any future caller that does not need the status bit.
- **Files modified:** internal/kernel/edit/tools.go.
- **Commit:** `1c869c29`.

**3. [Documentation note] ls_error classifier limitation**

- **Found during:** Task 4.2 design.
- **Issue:** The plan's classifier sketch suggests mapping `errors.Is(err, edit.ErrLSError)` → `"ls_error"`. No such sentinel exists in `internal/kernel/edit/`, and introducing one would require touching every LS call site in `lspool/`, `replace.go`, `rename.go`, etc.
- **Fix:** `ClassifyEditError` returns `"internal"` for unclassified errors (Q-3 alignment). `ls_error` is set DIRECTLY at the known LS call sites (`outcome = "ls_error"` after acquire-session/lease errors). The classifier's docstring documents this v1.2 limitation. v1.3 will introduce typed LS-error sentinels and bridge them through the classifier.
- **Tracking:** documented in `ClassifyEditError` docstring; no separate deferred-items entry needed (this is a v1.3 typed-error scope item already implied by the matching `outcomeEnum` TODOs at `middleware.go:102-105`).

### Threat surface scan

No new threat surface introduced beyond what Plan 53-04's `<threat_model>` already covers. T-53-10 (cardinality DoS) is mitigated by:

- 7-tool closed `tool_name` set, 6-value `outcome` enum, 4-value `strategy` enum (cardinality bound 168, well under the 256 ceiling pinned in Plan 53-01's cardinality test).
- Closed-enum drop-unknown at the `*obs.Metrics.EditOutcomeInc` layer rejects any unknown values silently.
- Q-4 structural invariant: `"failed"` never appears in tools.go as a strategy value (verified by grep).

## Self-Check: PASSED

- internal/mcp/middleware.go modified (editOutcomeSink + InstallMiddleware wiring + 2 closed enums + 3 ForTest accessors)
- internal/mcp/record_edit_outcome_test.go created (141 lines)
- internal/kernel/edit/tools.go modified (ClassifyEditError + appendVerifyInfoWithStatus + 5 handler instrumentations)
- internal/kernel/edit/outcome_emission_test.go created (155 lines)
- internal/kernel/fileops/tools.go modified (2 handler instrumentations + edit import)
- internal/kernel/fileops/outcome_emission_test.go created (97 lines)
- internal/fuzzy/match.go modified (2 sentinels + ambiguityError/failureError refactor)
- internal/fuzzy/fuzzy_test.go modified (2 sentinel tests)
- Commits 311fcaa4, 18868d69, 1c869c29, e502557c all present in git log
- 5 RecordEditOutcome calls in edit/tools.go (one per handler)
- 2 RecordEditOutcome calls in fileops/tools.go (one per handler)
- 0 literal `"failed"` strings in either tools.go file
- 0 known stubs in modified files
- Phase 47 D-07 RecordRenameStrategy + renameStrategySink + tools.go:397 emission unchanged
- All test packages green (mcp, kernel/edit, kernel/fileops, obs, fuzzy)
- `go build ./cmd/helix` succeeds
