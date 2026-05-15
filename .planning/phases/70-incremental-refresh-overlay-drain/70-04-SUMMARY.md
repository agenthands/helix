---
phase: 70
plan: 04
subsystem: daemon-wiring
tags: [daemon, wiring, incremental-refresh, overlay-drain, semantic-store, wave2]
dependency_graph:
  requires:
    - "70-01: *Store.OverlayChangedPathsSince accessor"
    - "70-02: SnapshotMeta.BaseOverlayEpoch + SetBaseOverlayEpoch + LatestCommittedSnapshotBaseEpoch"
    - "70-03: helix_incremental_refresh_fallback_total metric + Service.FlushNow"
  provides:
    - "Extended StoreAccessor (CurrentOverlayEpoch, OverlayChangedPathsSince, LatestCommittedSnapshotBaseEpoch)"
    - "Extended LiveAccessor (FlushNow)"
    - "daemon-side adapter delegations (semStoreAdapter, semLiveAdapter)"
    - "collectCandidatePaths dispatcher with overlay-drain seam + closed-enum fallback classification"
    - "buildFn baseline-epoch capture at CommitSnapshot time (Pitfall 3 mitigation)"
    - "classifyEmptySeamFallback closed-enum helper"
  affects:
    - "Plan 70-05 (refresh_semantic_graph consumes LiveAccessor.FlushNow + new StoreAccessor methods)"
tech-stack:
  added: []
  patterns:
    - "Closed-enum fallback classification (mirrors LiveFileFactDiffInc reason-priority pattern)"
    - "Capture-baseline-before-commit pattern for incremental drain (next-refresh feeds the baseline back)"
    - "Verbatim helper extraction (fullWalkPaths) preserves byte-identical legacy behavior"
key-files:
  created: []
  modified:
    - internal/skill/semantic/accessors.go
    - internal/daemon/semantic_wiring.go
    - internal/daemon/semantic_wiring_test.go
    - internal/skill/semantic/runner_test.go
    - internal/skill/semantic/integration_test.go
    - internal/skill/semantic/status_e2e_external_test.go
    - internal/skill/semantic/tools_context_test.go
    - internal/skill/semantic/tools_refresh_test.go
decisions:
  - "Snapshot baseline capture uses snap.SetBaseOverlayEpoch (Plan 02 option a), not SnapshotSummary. Plan 02 already chose option a and exposes the setter; Plan 04 just calls it."
  - "A1 confirmed at execution time: overlay producer (handler.go:417 UpsertOverlayFile) passes the caller-supplied path verbatim; producers (fsnotify, kernel-edit) all hand it absolute paths. No filepath.Join translation needed on the incremental branch."
  - "overlay_rotated branch coverage via white-box helper test (TestClassifyFallbackReason) rather than store-backed harness — the production overlay code path cannot construct currentEpoch>baseEpoch with zero rows-above-baseEpoch without admin/migration plumbing that is out of scope here."
  - "Empty-overlay fallback test uses baseEpoch == currentEpoch (== 1) to force the empty_overlay branch deterministically; cold-start covers baseEpoch=0."
  - "Wave-1 test stubs across internal/skill/semantic (runner_test, integration_test, tools_context_test, tools_refresh_test, status_e2e_external_test) extended with cold-start zero-value stubs of the new interface methods. Plan 70-05 will swap the relevant stubs for recording variants when wiring the refresh tool through the new seam."
metrics:
  duration: "~35 min"
  completed: "2026-05-15"
requirements_completed: [REFRESH-01, REFRESH-03]
---

# Phase 70 Plan 04: Daemon Wiring — collectCandidatePaths Overlay-Drain Seam Summary

Phase 70's central behavior change. `collectCandidatePaths` now dispatches on `mode`: `incremental` queries `*Store.OverlayChangedPathsSince(baseEpoch)` (the Plan 70-01 seam) and returns its result verbatim on hit; empty/error path classifies the reason (`cold_start` / `overlay_rotated` / `empty_overlay` / `error`), emits the bounded-label `helix_incremental_refresh_fallback_total` counter, slog.Warns with the same closed-enum reason token, and falls back to `fullWalkPaths`. `mode=full` delegates straight to `fullWalkPaths` (byte-identical to pre-Phase-70 behavior). `makeProductionBuildFn` now captures `CurrentOverlayEpoch` immediately before `CommitSnapshot` and persists it via `snap.SetBaseOverlayEpoch` (Pitfall 3 mitigation — closes the "what baseline does the NEXT refresh consume?" question). `StoreAccessor` and `LiveAccessor` interfaces are extended with the four new methods Plan 70-05 will consume.

## Tasks Completed

| Task | Name                                                                              | Commits                                     |
| ---- | --------------------------------------------------------------------------------- | ------------------------------------------- |
| 1    | Extend StoreAccessor + LiveAccessor; daemon adapters delegate to *Store/*Service  | RED `0d1798f2`, GREEN `7e3a4353`            |
| 2    | collectCandidatePaths rewrite + buildFn baseline-epoch capture + annotation swap  | RED `7600c0c0`, GREEN `eb9c38c3`            |

## What Was Built

### Interface extensions (`internal/skill/semantic/accessors.go`)

`StoreAccessor` gains:
- `CurrentOverlayEpoch(ctx, repoID) (uint64, error)`
- `OverlayChangedPathsSince(ctx, repoID, baseEpoch) (paths []string, currentEpoch uint64, err error)`
- `LatestCommittedSnapshotBaseEpoch(ctx, repoID) (epoch uint64, ok bool, err error)`

`LiveAccessor` gains:
- `FlushNow(ctx, ws) error`

Doc-block on `LiveAccessor` calls out the semantics: `FlushNow` is a *synchronous* coalescer drain, NOT a snapshot write. `StoreAccessor` extensions documented as pure read accessors safe to call concurrently with the existing surface.

### Daemon adapter delegations (`internal/daemon/semantic_wiring.go`)

- `semStoreAdapter.CurrentOverlayEpoch` → `a.store.CurrentOverlayEpoch(ctx, repoID)` (nil-guard returns `(0, nil)`).
- `semStoreAdapter.OverlayChangedPathsSince` → `a.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)` (nil-guard returns `(nil, 0, nil)`).
- `semStoreAdapter.LatestCommittedSnapshotBaseEpoch` → `a.store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)` (nil-guard returns `(0, false, nil)`).
- `semLiveAdapter.FlushNow` → `a.live.service.FlushNow(ctx, ws)` (nil-guard returns nil for unwired adapter chain; upstream service already returns nil for unregistered workspaces).

### `collectCandidatePaths` rewrite

New signature: `func (b *semanticBundle) collectCandidatePaths(ctx context.Context, ws workspace.WorkspaceKey, mode string, baseEpoch uint64) []string`.

Dispatch:
- `mode != "incremental"` → `b.fullWalkPaths(ws)` (preserves all behavior including `mode=""` legacy path).
- `mode == "incremental"`:
  - Calls `b.store.OverlayChangedPathsSince(ctx, ws.Hash(), baseEpoch)`.
  - **Error path:** slog.Warn `"collectCandidatePaths: overlay seam error; falling back to full-walk"` with `repo` + `err`; emits `IncrementalRefreshFallbackInc(reason=error, repo=repoID)`; returns `fullWalkPaths(ws)`.
  - **Hit (`len(paths)>0`):** returns the seam paths verbatim. Inline comment documents the A1 invariant: paths are absolute per RESEARCH.md §Open Questions RESOLVED — overlay producer (handler.go:417) writes the verbatim caller-supplied absolute path; no `filepath.Join` translation required.
  - **Empty seam:** classifies via `classifyEmptySeamFallback(baseEpoch, currentEpoch)`, slog.Warn `"collectCandidatePaths: incremental fell back to full-walk"` with `reason` + `base_epoch` + `current_epoch`; emits the bounded-label metric; returns `fullWalkPaths(ws)`.

`classifyEmptySeamFallback(baseEpoch, currentEpoch) string`:
- `baseEpoch == 0` → `cold_start` (caller never observed an epoch).
- `currentEpoch > baseEpoch` → `overlay_rotated` (overlay advanced with no rows above baseEpoch).
- otherwise → `empty_overlay` (overlay quiet since baseline; the no-op-refresh case).

`fullWalkPaths(ws)` is the verbatim extraction of the pre-Phase-70 `filepath.WalkDir` body — including the dot-directory exclusion, symlink rejection, and regular-file filter — so `mode=full` is byte-identical.

### buildFn baseline-epoch capture (`makeProductionBuildFn`)

- For `mode="incremental"`: derive `baseEpoch` via `b.store.LatestCommittedSnapshotBaseEpoch(ctx, repoID)`. `(0, false, nil)` means cold-start; `collectCandidatePaths` will classify the empty seam as `cold_start` and fall back. Read errors slog.Warn and set `baseEpoch=0` (cold-start fallback signal).
- Immediately before `CommitSnapshot`, call `baseOverlayEpoch, err := b.store.CurrentOverlayEpoch(ctx, repoID)`. Capture errors slog.Warn with `event="capture_base_overlay_epoch_failed"` and persist 0. Then `snap.SetBaseOverlayEpoch(baseOverlayEpoch)` so `CommitSnapshot` writes the column atomically with the `status='committed'` flip (Plan 70-02 schema).

### Refresh-degraded annotation removal (CONTEXT.md D6)

The multi-paragraph "refresh-degraded" annotation that headed the legacy `collectCandidatePaths` is replaced with the exact 3-line note from CONTEXT.md D6:

```go
// collectCandidatePaths builds the candidate path set the production buildFn
// classifies + extracts.
//   - mode=full: walk ws.RepoRoot via filepath.WalkDir (.helix, .git, dot-dirs excluded).
//   - mode=incremental: query OverlayChangedPathsSince(baseEpoch); on empty
//     result, fall back to full-walk and emit the bounded-label fallback metric.
```

Grep gate `refresh-degraded` returns 0 matches.

## Tests Added

| Test                                                                | Coverage                                                                                                 |
| ------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------- |
| `TestCollectCandidatePaths_Incremental_HitReturnsSeamPaths`         | Overlay row → seam HIT path returned verbatim; NO fallback metric emitted.                               |
| `TestCollectCandidatePaths_Incremental_EmptySeamFallback_ColdStart` | `baseEpoch=0` + empty overlay → reason=`cold_start`; full-walk result returned.                          |
| `TestCollectCandidatePaths_Incremental_EmptySeamFallback_EmptyOverlay` | One overlay row at epoch 1 + `baseEpoch=1` → reason=`empty_overlay`; full-walk result returned.       |
| `TestClassifyFallbackReason`                                        | White-box coverage of all 5 (baseEpoch, currentEpoch) classification cases, including `overlay_rotated`. |
| `TestCollectCandidatePaths_Full_UsesWalker`                         | `mode=full` returns walked path; NO fallback metric emitted.                                             |

Tests use a real DuckDB-backed `*semanticstore.Store` and a real `*obs.Metrics` (via `obs.Noop()`); fallback emission is asserted by `Registry().Gather()` inspection. The `overlay_rotated` branch is covered via direct white-box helper test because the production overlay code path cannot construct `currentEpoch>baseEpoch ∧ rows_above_baseEpoch=0` without admin/migration plumbing.

## Verification

| Check | Command | Result |
|---|---|---|
| New tests pass | `go test ./internal/daemon/ -race -count=1 -run 'TestCollectCandidatePaths\|TestClassifyFallbackReason'` | PASS (2.6s) |
| Full daemon + skill + store tests pass | `go test ./internal/daemon/ ./internal/skill/semantic/ ./internal/semantic/store/ -race -count=1` | PASS (4-8s each) |
| `go vet ./...` | clean (only pre-existing tree-sitter swift cgo warnings) | OK |

Grep gates (acceptance criteria):

```
internal/daemon/semantic_wiring.go:1609:func (b *semanticBundle) fullWalkPaths(ws workspace.WorkspaceKey) []string {
internal/daemon/semantic_wiring.go:422:func (a *semStoreAdapter) OverlayChangedPathsSince(ctx context.Context, ...
internal/daemon/semantic_wiring.go:1556:    paths, currentEpoch, err := b.store.OverlayChangedPathsSince(ctx, repoID, baseEpoch)
internal/daemon/semantic_wiring.go:1563:        b.metrics.IncrementalRefreshFallbackInc(obs.IncrementalRefreshFallbackReasonError, repoID)
internal/daemon/semantic_wiring.go:1582:    b.metrics.IncrementalRefreshFallbackInc(reason, repoID)
internal/daemon/semantic_wiring.go:1521:    snap.SetBaseOverlayEpoch(baseOverlayEpoch)
internal/daemon/semantic_wiring.go:1568:    // A1: paths are absolute per RESEARCH.md §Open Questions RESOLVED —
grep 'refresh-degraded' internal/daemon/semantic_wiring.go → 0 matches
grep '^// collectCandidatePaths builds the candidate path set' → 1
internal/skill/semantic/accessors.go:40: CurrentOverlayEpoch(ctx context.Context, repoID string) ...
internal/skill/semantic/accessors.go:45: OverlayChangedPathsSince(ctx context.Context, ...
internal/skill/semantic/accessors.go:50: LatestCommittedSnapshotBaseEpoch(ctx context.Context, ...
internal/skill/semantic/accessors.go:99: FlushNow(ctx context.Context, ws workspace.WorkspaceKey) error
```

All gates match as required.

## Deviations from Plan

- **[Rule 3 — Blocking issue, test-stub fanout]** Extending `StoreAccessor` and `LiveAccessor` broke compile across five test files in `internal/skill/semantic/` (`runner_test.go`, `integration_test.go`, `status_e2e_external_test.go`, `tools_context_test.go`, `tools_refresh_test.go`) that define stub implementations of those interfaces. Extended each stub with cold-start zero-value implementations of the new methods. This is mechanical compile-fix work that does not change any production behavior. Plan 70-05 will swap the relevant stubs (`recorderStoreAccessor`, `mockLiveAccessor`) for recording variants when wiring the refresh tool through the new seam.

- **`overlay_rotated` test approach.** The plan asked for `TestCollectCandidatePaths_Incremental_EmptySeamFallback_OverlayRotated` driven through the store-backed harness. The production overlay code path allocates a `write_epoch` row on every `BeginOverlayTx+upsert`, so reaching `currentEpoch>baseEpoch ∧ rows_above_baseEpoch=0` requires admin/migration plumbing that is out of scope here. Instead, added `TestClassifyFallbackReason` as a focused white-box test that drives `classifyEmptySeamFallback` directly with all 5 (baseEpoch, currentEpoch) tuples — gives stronger and more deterministic coverage of the closed-enum classification function. The store-backed harness still exercises `cold_start`, `empty_overlay`, and the hit path end-to-end. (Documented inline in the `overlay_rotated` test that was replaced with the white-box approach.)

- **D-09 comment block at accessors.go:127-131.** The plan referenced "the D-09 comment block (accessors.go:~127-131)". The actual comment block at that line range belongs to `CompactorAccessor`'s D-13 invariant (refresh handler MUST NOT call OnFlush), not a D-09 block. Augmented `StoreAccessor` and `LiveAccessor` doc-comments directly with the requested Phase 70-04 notes (pure-read-accessor + synchronous-flush-not-snapshot-write).

## Decisions Made

- **Snapshot baseline plumbing path.** Plan 70-02 already chose option (a) — `SnapshotMeta.BaseOverlayEpoch` + `(*Snapshot).SetBaseOverlayEpoch` setter — and shipped the setter. Plan 70-04 just calls it. No re-litigation of the option (a) vs (b) decision.

- **A1 absolute-path invariant verified at execution time.** Per the plan's read-before-edit mandate, confirmed `internal/semantic/live/handler/handler.go:392-417` does in fact pass the caller-supplied path verbatim through `tx.UpsertOverlayFile(ctx, path, hash)` with no normalization. Producers (fsnotify watcher, kernel-edit `OnEdit`) all pass absolute paths. The `collectCandidatePaths` full-walk branch also returns absolute paths via `filepath.WalkDir(ws.RepoRoot, …)`. Both branches agree on shape; no `filepath.Join` translation needed. Documented inline with `// A1:` comment so the next reader does not have to re-derive the invariant.

- **Empty-overlay test deterministic construction.** Uses `baseEpoch == currentEpoch == 1` (one overlay row at epoch 1, ask with `baseEpoch=1`) to force the empty_overlay branch deterministically. Cold-start uses `baseEpoch=0` with empty overlay.

## Known Stubs

None. All new code paths have production wiring; test stubs in `internal/skill/semantic` are explicitly labeled as Plan 70-05 swap targets where they will be replaced with recording variants.

## Threat Flags

None — the new code only consumes already-trusted seams (semantic store accessors, live service flush) and emits observability signals. No new network/auth/file surface introduced. The `repo` label on the fallback counter inherits the existing carve-out from Plan 70-03.

## Commits

| Hash | Message |
|---|---|
| `0d1798f2` | `test(70-04): extend StoreAccessor + LiveAccessor with overlay-drain seam` |
| `7e3a4353` | `feat(70-04): wire daemon adapters for StoreAccessor + LiveAccessor extensions` |
| `7600c0c0` | `test(70-04): add failing tests for collectCandidatePaths overlay-drain seam` |
| `eb9c38c3` | `feat(70-04): rewrite collectCandidatePaths around overlay-drain seam` |

## TDD Gate Compliance

Both tasks followed strict RED → GREEN sequence: failing test commit precedes implementation commit. No REFACTOR commits were needed (implementation landed cleanly on first GREEN pass). Compile-time interface guards in `semantic_wiring.go` (`var _ semantic.StoreAccessor = (*semStoreAdapter)(nil)` etc.) served as the failing-test mechanism for Task 1; the new unit/integration tests in `semantic_wiring_test.go` served Task 2.

## Self-Check: PASSED

Files exist:
- internal/skill/semantic/accessors.go ✓ (StoreAccessor + LiveAccessor extended)
- internal/daemon/semantic_wiring.go ✓ (semStoreAdapter / semLiveAdapter delegations + collectCandidatePaths rewrite + fullWalkPaths + classifyEmptySeamFallback + buildFn baseline capture)
- internal/daemon/semantic_wiring_test.go ✓ (5 new tests / sub-tests)
- internal/skill/semantic/{runner_test,integration_test,status_e2e_external_test,tools_context_test,tools_refresh_test}.go ✓ (cold-start stub extensions)

Commits exist:
- 0d1798f2 ✓ test(70-04): extend StoreAccessor + LiveAccessor with overlay-drain seam
- 7e3a4353 ✓ feat(70-04): wire daemon adapters for StoreAccessor + LiveAccessor extensions
- 7600c0c0 ✓ test(70-04): add failing tests for collectCandidatePaths overlay-drain seam
- eb9c38c3 ✓ feat(70-04): rewrite collectCandidatePaths around overlay-drain seam

All grep gates from the plan acceptance criteria match; `refresh-degraded` returns 0 matches; tests pass under `-race -count=1`.
