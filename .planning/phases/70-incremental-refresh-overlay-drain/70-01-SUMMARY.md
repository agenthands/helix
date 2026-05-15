---
phase: 70
plan: 01
subsystem: semantic-store
tags: [duckdb, semantic-store, overlay, accessor, refresh]
requires:
  - semantic_live_overlay_meta.current_epoch (Phase 60)
  - semantic_live_overlay_files.write_epoch + idx_overlay_files_write_epoch (Phase 60)
  - CurrentOverlayEpoch pattern (Phase 63)
provides:
  - "*Store.OverlayChangedPathsSince accessor"
affects:
  - internal/semantic/store/overlay.go
  - internal/semantic/store/overlay_test.go
tech_stack:
  added: []
  patterns:
    - CurrentOverlayEpoch-style ErrNoRows → (zero, nil) handling
    - filefact_accessor.go multi-row Scan loop
key_files:
  created: []
  modified:
    - internal/semantic/store/overlay.go
    - internal/semantic/store/overlay_test.go
decisions:
  - "Locked signature (paths []string, currentEpoch uint64, err error) — distinct return slots so callers can disambiguate 'quiet since baseline' from 'no overlay activity'"
  - "Two-query path (epoch then paths) instead of CTE: each query maps cleanly to an existing index, avoids planner surprises, and keeps the ErrNoRows guard at the top"
metrics:
  duration: ~15min
  tasks_completed: 2
  files_modified: 2
  commit_count: 2
requirements_completed: [REFRESH-01]
---

# Phase 70 Plan 01: OverlayChangedPathsSince Read Accessor — Summary

Landed `*Store.OverlayChangedPathsSince(ctx, repoID, baseEpoch) → (paths, currentEpoch, err)` — the lock-free seam that `index_semantic_graph(mode=incremental)` and `refresh_semantic_graph` will both consume.

## What Was Built

- **`internal/semantic/store/overlay.go`** — Appended a 65-LOC accessor immediately after `CurrentOverlayEpoch`. Body order is the one locked by RESEARCH.md Pattern 1: nil-guard → empty-repoID guard → read `current_epoch` (ErrNoRows → `(nil, 0, nil)`) → `SELECT DISTINCT path … WHERE write_epoch > ?` → `rows.Err()` → return. Added `errors` to the import block (single new identifier; all other deps already imported).

- **`internal/semantic/store/overlay_test.go`** — Added `TestOverlayChangedPathsSince` with five `t.Run` sub-tests (`cold_start`, `single_change`, `multi_epoch`, `empty_since`, `no_meta_row`) plus two negative guards (`nil_store_errors`, `empty_repoID_errors`). Each sub-test uses a distinct `repoID` so they do not share state. Fixture setup uses the existing `openStoreForOverlayTest` / `BeginOverlayTx` / `UpsertOverlayFile` helpers; epochs are captured via `tx.Epoch()` (NOT `tx.WriteEpoch()` — the latter does not exist on `OverlayTx`).

## TDD Gate Compliance

- **RED** — `test(70-01): add failing test for OverlayChangedPathsSince` at `2645b44c`. Confirmed RED via `s.OverlayChangedPathsSince undefined (type *Store has no field or method OverlayChangedPathsSince)` (7 call sites).
- **GREEN** — `feat(70-01): implement OverlayChangedPathsSince accessor` at `463f46f0`. All 7 sub-tests / guards pass under `-race`.
- **REFACTOR** — Not needed; implementation matches the locked Body Order on first write.

## Verification

| Check | Command | Result |
|---|---|---|
| Sub-tests pass | `go test ./internal/semantic/store/ -race -run TestOverlayChangedPathsSince -count=1` | PASS (1.8s) |
| Sibling test still passes | `go test … -run 'OverlayChangedPathsSince\|TestCurrentOverlayEpoch'` | PASS |
| `go vet` | `go vet ./internal/semantic/store/...` | clean |
| Kernel→semantic boundary | `go vet -vettool=vet-nokernel2semantic ./...` | clean |
| D-09 invariant (no tx/write tokens in body) | `awk` over the function body | `D-09 OK` |

## Deviations from Plan

- **[Rule 3 — Blocking import]** The plan stated "Reuse existing imports (`context`, `database/sql`, `errors`, `fmt`) — no new imports." `errors` was NOT already imported by `overlay.go`; added it to the import block. This is a one-line auto-fix that does not alter any contract.
- **`tx.WriteEpoch()` vs `tx.Epoch()`** — The plan suggested capturing the write epoch via `tx.WriteEpoch()`. The actual method on `OverlayTx` is `Epoch()` (overlay.go:68). Tests use `tx.Epoch()` accordingly.

Otherwise the plan executed exactly as written: locked signature, locked body order, no schema changes, no new indexes.

## Decisions Made

1. **Locked return shape `(paths []string, currentEpoch uint64, err error)`** — Three slots so callers can distinguish three states: (a) `(nil, 0, nil)` = workspace never had overlay activity (fall back to full-walk OR record epoch=0 baseline), (b) `([], N>0, nil)` = workspace quiet since `baseEpoch` (no-op refresh), (c) `(non-empty, N, nil)` = incremental drain target list. CONTEXT.md D2.

2. **Two-query path rather than a CTE** — Each query maps cleanly to an existing index (`PK(repo_id)` on `semantic_live_overlay_meta`; `idx_overlay_files_write_epoch` on `(repo_id, write_epoch)`). Keeps the `ErrNoRows` guard at the top where it short-circuits the path scan entirely. RESEARCH.md Pattern 1.

3. **`baseEpoch == 0` returns ALL paths** — Strict `>` filter; `write_epoch` is allocated monotonically from 1 (D-04 contract), so `> 0` matches every row. This is the cold-start signal: the caller has not yet observed any epoch. No special-case branch in the body — the SQL does it naturally.

## Commits

| Hash | Message |
|---|---|
| `2645b44c` | `test(70-01): add failing test for OverlayChangedPathsSince` |
| `463f46f0` | `feat(70-01): implement OverlayChangedPathsSince accessor` |

## Self-Check: PASSED

- FOUND: `internal/semantic/store/overlay.go` (modified — function appended after `CurrentOverlayEpoch`)
- FOUND: `internal/semantic/store/overlay_test.go` (modified — `TestOverlayChangedPathsSince` added)
- FOUND commit: `2645b44c` (RED)
- FOUND commit: `463f46f0` (GREEN)
- D-09 invariant verified: function body contains zero `Begin|Commit|Abort|Write|tx.Exec` tokens
- Tests green under `-race`
