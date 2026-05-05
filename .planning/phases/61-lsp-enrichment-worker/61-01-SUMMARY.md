---
id: 61-01
phase: 61-lsp-enrichment-worker
plan: 01
subsystem: lsp-enrichment / kernel-semantic-seam
tags: [phase-61, lsp-enrichment, lease-acquirer, lane-queue, bulk-suppression, vet-analyzer, single-source-of-truth]
requires:
  - phase-60-lspqueue (Phase 60 P04 lspqueue.RevalidateFileJob + Queue)
  - phase-60-handler  (Phase 60 P04 handler.Handler producer site)
  - phase-60-coalescer (CoalesceEvents bulk-collapse)
  - phase-60-overlay  (BeginOverlayTx + per-workspace mutex)
  - phase-59-migrations-002 (extraction_partial + partial_reason columns on semantic_files)
  - phase-56-lspool   (*Pool.AcquireLease, RWMutex pool)
provides:
  - lspenrich.LeaseAcquirer (kernel/semantic seam, ENRICH-01)
  - lspenrich.Lane + lspenrich.LaneQueue (strict-priority 2-lane queue, ENRICH-02)
  - lspenrich.Outcome + 5 closed-enum constants (B3 SSOT)
  - lspenrich.MetricsSink interface (B3 SSOT)
  - lspenrich.OverlayStore interface (B3 SSOT)
  - *lspool.Pool.ForegroundBusy + *lspool.Pool.SetYieldCheckWindow (D-04)
  - *lspool.Pool.lastForegroundLease + yieldCheckWindow fields
  - *store.OverlayTx.MarkFileSemanticPending (D-05)
  - handler.LSPLaneEnqueuer (renamed + extended; B5 SSOT)
  - handler.markBulkPending + selectLane helpers
  - live.SourceChangeEvent.Paths field (populated on ChangeBulkUpdate)
  - cmd/vet-nosemantic2kernel + Makefile wiring (mechanical ENRICH-01 enforcement)
affects:
  - internal/daemon/live_wiring.go (lspQueue retyped to *lspenrich.LaneQueue)
  - internal/daemon/live_e2e_test.go (CR-04 producer-side check uses Depth())
  - internal/semantic/store/migrations.go (closed-enum doc-comment block)
  - internal/semantic/live/handler/lspqueue_emit_test.go (uses *lspenrich.LaneQueue)
  - internal/semantic/live/handler/noop_test.go (fakeTx grew MarkFileSemanticPending)
tech-stack:
  added:
    - golang.org/x/tools/go/analysis/singlechecker (Phase 60 dep, reused for the new vet binary)
  patterns:
    - Closed-enum runtime validator (partialReasonClosedEnum) — Phase 59 D-05 carried forward
    - Test-only seam (stampForegroundLeaseForTesting) — Phase 56 worker-test pattern
    - Public 3-arg + private *WithKind variant — preserves scheduler.IncrementalHandler contract
    - Single source of truth for cross-plan contracts (B3 + B5)
key-files:
  created:
    - internal/semantic/lspenrich/doc.go
    - internal/semantic/lspenrich/acquirer.go
    - internal/semantic/lspenrich/queue.go
    - internal/semantic/lspenrich/queue_test.go
    - internal/semantic/lspenrich/types.go
    - internal/semantic/lspenrich/types_test.go
    - internal/lint/nosemantic2kernel/analyzer.go
    - internal/lint/nosemantic2kernel/analyzer_test.go
    - internal/lint/nosemantic2kernel/testdata/src/...  (5 testdata packages)
    - cmd/vet-nosemantic2kernel/main.go
    - internal/kernel/lspool/pool_foreground_busy_test.go
    - internal/semantic/store/overlay_pending_test.go
    - internal/semantic/live/handler/handler_lane_test.go
  modified:
    - Makefile (vet target installs + runs vet-nosemantic2kernel)
    - internal/kernel/lspool/pool.go (lastForegroundLease + yieldCheckWindow + ForegroundBusy + SetYieldCheckWindow)
    - internal/semantic/store/overlay.go (partialReasonClosedEnum + MarkFileSemanticPending)
    - internal/semantic/store/migrations.go (closed-enum doc-comment in schema2Statements)
    - internal/semantic/live/signal.go (SourceChangeEvent.Paths field)
    - internal/semantic/live/coalescer/coalesce.go (populate Paths on bulk collapse)
    - internal/semantic/live/handler/handler.go (rename + extend interface; markBulkPending; selectLane; bulk-suppression branch)
    - internal/semantic/live/handler/noop_test.go (fakeTx satisfies extended OverlayTx)
    - internal/semantic/live/handler/lspqueue_emit_test.go (lane-aware producer assertion)
    - internal/daemon/live_wiring.go (LaneQueue construction + adapter MarkFileSemanticPending forward)
    - internal/daemon/live_e2e_test.go (Depth-based producer-side check)
decisions:
  - "Narrow nosemantic2kernel.checkedPkgPrefix to internal/semantic/lspenrich (not the broader internal/semantic) to honor the must_haves truth verbatim and avoid falsely flagging Phase 60 internal/semantic/live/service which legitimately implements kernel.EditNotifier."
  - "Use semantic_files.extraction_partial column (NOT bare 'partial') because that is the actual Phase 59 column name on this table; the bare 'partial' column lives only on semantic_symbols/semantic_references."
  - "MarkFileSemanticPending is a no-op when no semantic_files row exists for (repo_id, path) — extraction may not yet have processed the path; the next extraction cycle will surface the row with the correct state."
  - "Preserve scheduler.IncrementalHandler 3-arg UpdateChangedFile public contract: ship the lane-aware variant as private updateChangedFileWithKind and route Dispatch to it; the 3-arg public method defaults to LaneBackground (scheduler-driven incremental work is not foreground edit traffic)."
  - "Extend live.SourceChangeEvent with Paths []string and update CoalesceEvents to populate it on bulk-collapse — without this, the producer loses per-file granularity at the bulk threshold and markBulkPending cannot stamp affected files."
  - "Switch internal/daemon liveBundle.lspQueue from *lspqueue.Queue to *lspenrich.LaneQueue rather than retro-fitting EnqueueLane on lspqueue.Queue (which would create a circular import lspqueue → lspenrich)."
metrics:
  duration_seconds: 1442
  completed_date: 2026-05-05
  tasks_total: 4
  tasks_completed: 4
  files_created: 13
  files_modified: 12
  commits: 8
---

# Phase 61 Plan 01: LeaseAcquirer + Shared Types + 2-Lane Queue + ForegroundBusy + Producer Rewiring + Vet Analyzer Summary

**One-liner:** Phase 61 P01 ships the foundational kernel/semantic seam (LeaseAcquirer + 2-lane queue + ForegroundBusy/SetYieldCheckWindow + producer-side bulk suppression + closed-enum partial_reason API + nosemantic2kernel vet analyzer + Wave-2-shared types.go) so P02 worker and P03 manager can compile against P01 alone.

## Truths Satisfied

| # | Truth                                                                                                                                                                                  | Verification                                                                                                                                                                                              |
| - | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| 1 | `internal/semantic/lspenrich` does not import `internal/kernel` (only `internal/kernel/lspool` + `internal/workspace`) and a vet analyzer mechanically enforces this                   | `go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/semantic/lspenrich \| grep internal/kernel` returns only `internal/kernel/lspool`. `make vet` exits 0 (vet-nosemantic2kernel runs).        |
| 2 | LeaseAcquirer interface in `internal/semantic/lspenrich` exposes exactly two methods: AcquireLease and ForegroundBusy                                                                  | `grep -c "AcquireLease\|ForegroundBusy" internal/semantic/lspenrich/acquirer.go` == 5 (one type decl line + two method names appearing twice each in the interface body and method-set view).             |
| 3 | `*lspool.Pool` implements LeaseAcquirer; ForegroundBusy returns true within yield_check_window after a non-enrichment AcquireLease, false after window elapses                         | F2/F3/F4 tests in `pool_foreground_busy_test.go` pass; SY1/SY2 tests confirm the default 200ms and 500ms override.                                                                                        |
| 4 | `*lspool.Pool` exposes SetYieldCheckWindow(d time.Duration) so daemon can populate from cfg.YieldCheckWindowMs (RWMutex-protected; defaults to 200ms when unset)                       | `grep -c "func (p \*Pool) SetYieldCheckWindow" internal/kernel/lspool/pool.go` == 1; SY3 race-clean test passes under `-race`.                                                                            |
| 5 | Lane-typed queue drains high before background — strict priority, no aging                                                                                                              | L2 test "100 background then 1 high → high drained first, background FIFO afterwards" passes (acceptance #3 at unit level).                                                                               |
| 6 | `handler.UpdateChangedFile` selects lane via ev.Kind: ChangeHelixEdit -> high, all others -> background                                                                                | H1 (helix_edit → LaneHigh) and H2 (file_modified → LaneBackground) and H3 (created/renamed → LaneBackground) tests pass.                                                                                  |
| 7 | `handler.Dispatch` on Kind=ChangeBulkUpdate calls markBulkPending and does NOT call EnqueueLane                                                                                        | H4 test asserts ZERO EnqueueLane calls + 250 MarkFileSemanticPending calls + 1 LSPEnrichmentBulkSuppressed metric bump on a bulk dispatch over 250 paths.                                                  |
| 8 | OverlayTx.MarkFileSemanticPending stamps semantic_files.partial_reason with closed-enum value (preempted \| bulk_update_pending \| lsp_unavailable \| budget exhausted)                 | P1 (single-reason persisted), P2 (all 4 reasons round-trip), P3 (garbage rejected without write) tests pass.                                                                                              |
| 9 | `lspenrich.Outcome` typed string + 5 closed-enum constants live in `lspenrich/types.go` (single source of truth — P02 cascade.go imports them, P03 manager/status import them; this resolves Wave-2 P03→P02 hidden dependency) | T4-1, T4-2 tests pass; gauntlet `grep -rn 'type Outcome string' internal/semantic/lspenrich/` returns exactly 1 line.                                                                                      |
| 10| `lspenrich.MetricsSink` + `lspenrich.OverlayStore` interfaces live in `lspenrich/types.go` so P03 manager + status compile against P01 alone                                            | T4-3, T4-4 tests pass; both gauntlet greps return exactly 1 line each.                                                                                                                                    |
| 11| Existing handler interface (was LSPRevalidationEnqueuer) is renamed and extended in-place — single Enqueuer / LSPLaneEnqueuer source of truth, no parallel interfaces                | `grep -rn 'LSPRevalidationEnqueuer\|lspqueue\.Enqueuer' internal/ \| grep -v lspenrich \| wc -l` returns 0; H7 test compile-time-asserts `*lspenrich.LaneQueue` satisfies `handler.LSPLaneEnqueuer`.        |

All 11 truths satisfied.

## Artifacts Created

See `key-files.created` in the frontmatter. Highlights:

- **`internal/semantic/lspenrich/`** — new package; doc.go pins ENRICH-01; acquirer.go ships the 2-method interface; queue.go ships the 2-lane queue; types.go ships the 3 cross-plan contracts.
- **`internal/lint/nosemantic2kernel/`** + **`cmd/vet-nosemantic2kernel/`** — symmetric sibling of the existing nokernel2semantic analyzer, narrowed to `internal/semantic/lspenrich` scope to honor the plan's must_haves and avoid Phase 60 false-positives.
- **`internal/semantic/lspenrich/queue_test.go`** + **`pool_foreground_busy_test.go`** + **`overlay_pending_test.go`** + **`handler_lane_test.go`** + **`types_test.go`** — comprehensive test surfaces covering all 11 truths.

## Key Links Verified

| From                                                            | To                                  | Via                                              | Pattern grep                                                                                                                                                          |
| --------------------------------------------------------------- | ----------------------------------- | ------------------------------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `internal/semantic/lspenrich/acquirer.go`                       | `internal/kernel/lspool`            | imports lspool types only                        | `grep "kernel/lspool" internal/semantic/lspenrich/acquirer.go` → 1 line                                                                                              |
| `internal/semantic/live/handler/handler.go`                     | `internal/semantic/lspenrich.Lane`  | EnqueueLane(lane, job) on lane-aware queue       | `grep "EnqueueLane" internal/semantic/live/handler/handler.go` → 3 (interface decl + selectLane invocation + UpdateChangedFile lane-aware path)                       |
| `internal/kernel/lspool/pool.go`                                | `lsp-enrichment:` prefix            | AcquireLease stamps lastForegroundLease only when sessionID lacks prefix | `grep "lsp-enrichment:" internal/kernel/lspool/pool.go` → 4 (const + AcquireLease HasPrefix + 2 doc references) |
| `internal/semantic/lspenrich/types.go`                          | P02 cascade.go AND P03 manager/status.go | Outcome + MetricsSink + OverlayStore imported from same package | All 3 gauntlets return exactly 1 declaration in lspenrich/types.go.                                                            |

## Threats Mitigated

| Threat ID    | Disposition | Status   | Notes                                                                                                                                                                                 |
| ------------ | ----------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| T-61-01-01   | mitigate    | DONE     | nosemantic2kernel ships + Makefile wires it; `make vet` runs the singlechecker on every invocation. Narrow scope (lspenrich only) avoids false-positives on Phase 60 live/service.   |
| T-61-01-02   | mitigate    | DONE     | RWMutex; ForegroundBusy is RLock-only; SetYieldCheckWindow is bootstrap-time write-locked. SY3 test runs concurrent reads + writes under -race; clean.                                |
| T-61-01-03   | accept      | n/a      | Bulk-update events are coalescer-bounded (bulk_change_threshold=200 default); per-path UPDATEs batched in one tx. Internal trust domain only.                                          |
| T-61-01-04   | accept      | n/a      | sessionID format unchanged; same exposure surface as foreground sessions.                                                                                                              |
| T-61-01-05   | mitigate    | DONE     | All AcquireLease edits are additive (one if-statement under existing lock); existing pool_test.go suite passes unchanged (`go test ./internal/kernel/lspool/... -race` all green).     |
| T-61-01-06   | mitigate    | DONE     | partialReasonClosedEnum runtime validator + P3 test (garbage rejected without write).                                                                                                  |
| T-61-01-07   | mitigate    | DONE     | All three Wave-2 contract types (Outcome / MetricsSink / OverlayStore) live in lspenrich/types.go — gauntlet greps each return exactly 1 declaration.                                  |
| T-61-01-08   | mitigate    | DONE     | LSPRevalidationEnqueuer renamed in-place; gauntlet `grep -rn LSPRevalidationEnqueuer internal/` returns 0 (after a final comment-edit pass).                                            |

## Deviations from Plan

The plan was internally inconsistent in several places where the action text disagreed with either the must_haves truth, the established schema, or established Phase 60 architecture. Each deviation is a Rule-1/2/3 fix-required adjustment; no Rule-4 architectural change.

### Auto-fixed Issues

**1. [Rule 1 — Bug] Narrowed nosemantic2kernel checkedPkgPrefix from `internal/semantic` to `internal/semantic/lspenrich`.**
- **Found during:** Task 1 GREEN verification (`make vet` failed on pre-existing `internal/semantic/live/service/service.go` importing `internal/kernel`).
- **Issue:** The plan's Task 1 action #3 set `checkedPkgPrefix = "github.com/agenthands/helix/internal/semantic"` (broad). Phase 60's `live/service` legitimately implements `kernel.EditNotifier` and imports `internal/kernel`; broadening the analyzer's checked scope would falsely flag established Phase 60 architecture.
- **Resolution:** Narrowed `checkedPkgPrefix` to match the plan's must_haves truth #1 and acceptance criterion #1 (both anchor the invariant to `internal/semantic/lspenrich`, not the whole semantic tree). Added a 5th testdata package (`internal/semantic/siblingok`) that asserts the analyzer stays silent on a sibling semantic package importing `internal/kernel`.
- **Files modified:** `internal/lint/nosemantic2kernel/analyzer.go`, `internal/lint/nosemantic2kernel/analyzer_test.go`, testdata moved under `internal/semantic/lspenrich/`.
- **Commit:** 81213a06.

**2. [Rule 1 — Bug] Adjusted MarkFileSemanticPending SQL to use `extraction_partial` instead of `partial`.**
- **Found during:** Task 2 GREEN — running the new tests against the actual schema.
- **Issue:** Plan SQL said `SET partial=TRUE`. The actual Phase 59 column on `semantic_files` is `extraction_partial` (only `semantic_symbols` and `semantic_references` got the bare `partial` column). Plan also referenced an `updated_at` column on `semantic_files` that doesn't exist (the table has `indexed_at`).
- **Resolution:** SQL writes `extraction_partial = TRUE, partial_reason = ?` only. No `updated_at`. WHERE clause uses `repo_id=? AND path=?` to match every snapshot row for the path (semantically: "this path is partial pending re-enrichment across all snapshots").
- **Files modified:** `internal/semantic/store/overlay.go`.
- **Commit:** 9cd80c06.

**3. [Rule 1 — Bug] MarkFileSemanticPending is no-op (returns nil) when no row exists.**
- **Found during:** Task 2 GREEN.
- **Issue:** Plan P4 expected "creates row if not exists" via INSERT...ON CONFLICT. `semantic_files`'s primary key is `(snapshot_id, file_id)` and the row carries Phase 59 fact-emitter NOT NULL metadata that the live handler does not own (file_id, content_hash, size_bytes, line_count, indexed_at). UPSERT-with-defaults from the live handler would corrupt the fact-emitter's invariants.
- **Resolution:** Pure UPDATE; UPDATE 0 rows is not an error. Documented in the doc-comment that paths the extractor has not yet processed simply have no marker — the next extraction cycle surfaces the correct state.
- **Files modified:** `internal/semantic/store/overlay.go`, `internal/semantic/store/overlay_pending_test.go` (P4 test renamed to `_NoRowIsNoOp`).
- **Commit:** 9cd80c06.

**4. [Rule 3 — Blocking] Added `Paths []string` field to `live.SourceChangeEvent`.**
- **Found during:** Task 3 RED design review.
- **Issue:** Plan H4 test expected `markBulkPending` to be called with 250 paths from a `ChangeBulkUpdate` event. Reality: `live.SourceChangeEvent` had no `Paths` field, and the coalescer's bulk-collapse path emitted a synthetic event with only `RepoID + Kind` (no per-file path data).
- **Resolution:** Extended `SourceChangeEvent` with `Paths []string` (populated only on `ChangeBulkUpdate`, nil for other kinds) and updated `CoalesceEvents` to populate the field from the merged byKey map at the bulk-collapse threshold (deterministic alphabetic order for reproducible dispatch).
- **Files modified:** `internal/semantic/live/signal.go`, `internal/semantic/live/coalescer/coalesce.go`.
- **Commit:** 2d194e9f.

**5. [Rule 3 — Blocking] Preserved `scheduler.IncrementalHandler.UpdateChangedFile` 3-arg public signature.**
- **Found during:** Task 3 GREEN.
- **Issue:** Plan called for `UpdateChangedFile(ctx, repoID, path, kind)`. But `scheduler.IncrementalHandler` declares it as 3-arg (no kind), and `var _ scheduler.IncrementalHandler = (*Handler)(nil)` is a compile-time assertion at the bottom of handler.go.
- **Resolution:** Kept the 3-arg public method as the scheduler-facing entrypoint (defaults to `LaneBackground` — scheduler-driven incremental work is not foreground edit traffic). Routed the Dispatch switch through a private `updateChangedFileWithKind` variant that threads `ev.Kind` to `selectLane`. Same split for `HandleFileRenamed`.
- **Files modified:** `internal/semantic/live/handler/handler.go`.
- **Commit:** 2d194e9f.

**6. [Rule 3 — Blocking] Daemon liveBundle.lspQueue retyped from `*lspqueue.Queue` to `*lspenrich.LaneQueue`.**
- **Found during:** Task 3 GREEN — `go build ./...` failed on `internal/daemon/live_wiring.go`.
- **Issue:** The interface rename (`LSPRevalidationEnqueuer` → `LSPLaneEnqueuer`) added an `EnqueueLane(lane, job)` method that `*lspqueue.Queue` cannot satisfy without taking a circular import on `lspenrich.Lane`.
- **Resolution:** Switched daemon construction to `lspenrich.NewLaneQueue(1024, 1024)`. Updated CR-04 e2e test to sum per-lane depths via `Depth(LaneHigh) + Depth(LaneBackground)` and added a Phase 61 D-01 sanity check that ChangeHelixEdit lands in LaneHigh.
- **Files modified:** `internal/daemon/live_wiring.go`, `internal/daemon/live_e2e_test.go`.
- **Commit:** 2d194e9f.

**7. [Rule 2 — Critical correctness] storeOverlayTxAdapter grew MarkFileSemanticPending forwarder.**
- **Found during:** Task 3 GREEN — `go build ./...` failed because `*storeOverlayTxAdapter` no longer satisfied the extended `handler.OverlayTx` interface.
- **Issue:** Adding a method to `handler.OverlayTx` without updating the daemon's adapter would have left production wiring unable to invoke the new path.
- **Resolution:** Added one-line forwarder `(*storeOverlayTxAdapter).MarkFileSemanticPending` calling through to `*store.OverlayTx.MarkFileSemanticPending`.
- **Files modified:** `internal/daemon/live_wiring.go`.
- **Commit:** 2d194e9f.

### Plan Acceptance Criterion Note

The plan's acceptance criterion `go list -deps ./internal/semantic/lspenrich/... | grep -v 'internal/kernel/lspool' | grep -c 'internal/kernel'` is intended to return 0. In practice it returns 1 because `internal/kernel/lspool` itself transitively depends on `internal/kernel/jsonrpc`, which is filtered through the `-deps` recursion but not the grep filter. The intent (lspenrich does not directly import internal/kernel except the lspool sub-package) is honored — verified via direct-imports listing:

```
$ go list -f '{{range .Imports}}{{.}}{{"\n"}}{{end}}' ./internal/semantic/lspenrich | grep internal/kernel
github.com/agenthands/helix/internal/kernel/lspool
```

The acceptance grep was authored without considering transitives. The gauntlet's intent is satisfied.

## Authentication Gates

None encountered. All work was local-tree only (no LS startup, no external auth).

## Test Coverage

All new behaviour gated by tests at the boundary it affects:

- **Analyzer:** 5 testdata packages × 5 test functions covering forbidden-import / lspool carve-out / workspace carve-out / out-of-scope / sibling-semantic.
- **LaneQueue:** 7 tests (basic enqueue, strict-priority, blocking-on-ctx, full-lane-drop, lane-label, legacy alias, closed-enum reject).
- **Pool ForegroundBusy:** 5 tests (empty, registers within window, enrichment-prefix-ignored, expires after window, per-wsKey isolation).
- **Pool SetYieldCheckWindow:** 3 tests (default 200ms, override expands, race-clean under `-race`).
- **MarkFileSemanticPending:** 5 tests (single-reason persisted, all 4 reasons round-trip, garbage rejected, no-row no-op, epoch advances per Commit).
- **Handler bulk + lane:** 7 tests (helix_edit→high, modified→background, created/renamed→background, bulk-suppression with 250 paths, nil-LSPQueue safety, per-path-error best-effort, B5 compile-time SSOT).
- **Types:** 4 tests (Outcome typed, 5 distinct constants, MetricsSink shape, OverlayStore shape).

`go test ./internal/semantic/... ./internal/kernel/lspool/... ./internal/lint/... ./internal/daemon/... -race` all green.

## Self-Check: PASSED

- All committed files present in tree.
- All commits present in git log.
- `make vet` exits 0.
- `go test ./...` shows no FAIL packages.
- All B3 + B5 single-source-of-truth gauntlet checks return PASS.
- Existing CR-04 producer-side regression test (`live_e2e_test.go`) updated and passing.
- Existing pool, store, handler, daemon test suites all passing — no Phase 60 / Phase 56 / Phase 59 regression.

## Known Stubs

None. All new code paths are wired end-to-end. The Phase 61 P02 worker (which consumes `LeaseAcquirer`, `LaneQueue.Drain`, `Outcome`, `MetricsSink`, `OverlayStore`) and P03 daemon manager (which constructs the adapter, calls `SetYieldCheckWindow`, populates `Handler.Metrics`) are out of scope for this plan but every contract they need is in place.
