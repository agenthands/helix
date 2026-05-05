---
phase: 60
plan: 04
subsystem: semantic-live
tags: [semantic, live, coalescer, classifier, scheduler, edit-notifier, tdd, phase-60]
requires:
  - "60-02 (overlay writer + epoch contract)"
  - "60-03 (kernel.EditNotifier seam)"
provides:
  - "internal/semantic/live package (signal types, classifier, sub-packages)"
  - "internal/semantic/live/coalescer (pure CoalesceEvents + per-workspace goroutine)"
  - "internal/semantic/live/handler (Dispatch + UpdateChangedFile/HandleFileDeleted/Renamed/BulkUpdate)"
  - "internal/semantic/live/service (Service implementing kernel.EditNotifier; per-workspace registry)"
  - "internal/semantic/live/lspqueue (typed buffered handoff for Phase 61)"
  - "scheduler.IncrementalHandler interface + Scheduler.SetIncrementalHandler setter"
  - "scheduler.ScheduleIncremental real dispatch (replaces phase60-incremental-stub sentinel)"
  - "semantic.RepoID typed alias"
affects:
  - "Phase 60 P05A (watcher) — consumes Service.OnWorkspaceChanged"
  - "Phase 60 P05B (manifest scanner + daemon wiring) — wires Service via Kernel.SetEditNotifier + Scheduler.SetIncrementalHandler"
  - "Phase 61 (LSP enrichment) — drains lspqueue.Queue.Channel()"
  - "Phase 64 (refresh_semantic_graph MCP) — reaches Scheduler.ScheduleIncremental"
tech-stack:
  added:
    - "Per-package import-cycle break: live/coalescer ↔ live + live/service composes both"
    - "atomic.Uint64 for non-blocking-enqueue drops counter (per-workspace)"
    - "time.AfterFunc-based debounce + max-batch-delay timer pair (mirrors internal/memory/watcher.go pattern)"
  patterns:
    - "MergeChange applied at ACCEPT time (not just flush) to preserve created+deleted=drop signal"
    - "Compile-time var _ Iface = (*T)(nil) pins on both kernel.EditNotifier and scheduler.IncrementalHandler"
    - "Sub-package placement (live/service) breaks live/coalescer ↔ live cycle without lifting types"
key-files:
  created:
    - "internal/semantic/live/signal.go"
    - "internal/semantic/live/classifier.go"
    - "internal/semantic/live/classifier_test.go"
    - "internal/semantic/live/coalescer/coalesce.go"
    - "internal/semantic/live/coalescer/coalesce_test.go"
    - "internal/semantic/live/coalescer/bulk_test.go"
    - "internal/semantic/live/coalescer/coalescer.go"
    - "internal/semantic/live/coalescer/coalescer_test.go"
    - "internal/semantic/live/handler/handler.go"
    - "internal/semantic/live/handler/noop_test.go"
    - "internal/semantic/live/service/service.go (NB: sub-package, not live/service.go)"
    - "internal/semantic/live/service/service_test.go"
    - "internal/semantic/live/lspqueue/queue.go"
    - "internal/semantic/live/lspqueue/queue_test.go"
    - "internal/semantic/scheduler/scheduler_incremental_test.go"
  modified:
    - "internal/semantic/types.go (added RepoID typed alias)"
    - "internal/semantic/scheduler/scheduler.go (IncrementalHandler interface + SetIncrementalHandler setter + real ScheduleIncremental body)"
decisions:
  - "EXECUTOR: classifier short-circuits on source=helix_edit (returns ChangeHelixEdit without I/O); kernel hook fires AFTER write so file is on disk by classifier-call time (T-60-04-07 mitigation)"
  - "EXECUTOR: classifier returns ChangeFileModified for known+exists regardless of hash equality; handler short-circuits if hash unchanged (60-04 PLAN RED 2 ALTERNATIVE — hasher param reserved for future opt)"
  - "EXECUTOR: classifier uses os.Lstat (not os.Stat) to reject dangling symlinks as missing (T-60-04-03 — matches repomap walker invariant)"
  - "EXECUTOR: HandleFileDeleted calls tx.MarkFileDeleted from 60-02 single-owner API (real tombstone row, NOT empty-hash upsert placeholder)"
  - "EXECUTOR: IncrementalHandler interface lives in scheduler package (NOT live) — keeps import direction one-way (scheduler ← live/handler) and avoids cycle"
  - "EXECUTOR: Service moved to internal/semantic/live/service sub-package — breaks live/coalescer ↔ live cycle (live/coalescer imports live for types; Service composes both, so cannot live in package live)"
  - "EXECUTOR: MergeChange applied at ACCEPT time inside Coalescer.accept (not just at flush) — Rule 1 bug fix; otherwise last-write-wins discards created+deleted drop signal"
  - "EXECUTOR: ScheduleIncremental nil-safe — pre-wiring callers get state transition + JobID with no dispatch (no panic)"
metrics:
  duration: "~45 minutes (single agent, sequential RED/GREEN per task)"
  tasks_completed: 7
  files_created: 14
  files_modified: 2
  test_count_added: 36
  completed_date: "2026-05-05"
---

# Phase 60 Plan 04: Live-Update Pipeline Spine Summary

**One-liner:** Shipped the in-process spine of the live-update pipeline — `internal/semantic/live` with the typed signal + path classifier + pure SPEC §16.2 coalescer + per-workspace single-goroutine debouncer + handler dispatch + `Service` implementing `kernel.EditNotifier` — plus filled the Phase 59 `ScheduleIncremental` stub with a real per-FileChange dispatch backed by a new `scheduler.IncrementalHandler` interface.

## Package Layout

```
internal/semantic/live/                    ← types + classifier (no goroutines)
  signal.go              WorkspaceChangeSignal, ChangeSource enum,
                         SourceChangeKind enum, SourceChangeEvent
  classifier.go          ClassifyPathChange (D-01 single owner)
  classifier_test.go     7 tests (5 decision rules + helix_edit short-circuit
                         + lookup-error propagation + dangling-symlink rejection)

  coalescer/             ← merge rules (pure) + per-workspace goroutine
    coalesce.go          CoalesceEvents + MergeChange (pure SPEC §16.2)
    coalescer.go         Coalescer struct (debounce + max-batch + drops)
    coalesce_test.go     11 subtests (6 merge rules + determinism + drop-on-keep=false + empty)
    bulk_test.go         2 tests (collapse + threshold-zero passthrough)
    coalescer_test.go    6 tests (debounce, max-batch ceiling, non-blocking enqueue,
                         no-op-flush guard, per-workspace isolation, per-event-error
                         batch invariant)

  handler/               ← dispatch to overlay tx writes
    handler.go           Handler with Dispatch / UpdateChangedFile / HandleFileDeleted /
                         HandleFileRenamed / HandleBulkUpdate; OverlayWriter +
                         OverlayTx + IncrementalScheduler interfaces
    noop_test.go         7 tests including TestNoOpDoesNotAdvanceEpoch (acceptance #8)

  service/               ← daemon glue (sub-package to break import cycle)
    service.go           Service implementing kernel.EditNotifier; per-workspace
                         coalescer registry; Start/Stop/OnEdit/OnWorkspaceChanged
    service_test.go      6 tests including OnEdit_NonBlocking (<100us avg under
                         saturation) + EditNotifier interface satisfaction

  lspqueue/              ← typed buffered handoff for Phase 61 worker
    queue.go             Queue + RevalidateFileJob; non-blocking Enqueue
    queue_test.go        2 tests
```

Plus changes outside live/:

- `internal/semantic/types.go` — added `type RepoID string`
- `internal/semantic/scheduler/scheduler.go` — added `IncrementalHandler` interface, `SetIncrementalHandler` setter, and the real `ScheduleIncremental` dispatch body
- `internal/semantic/scheduler/scheduler_incremental_test.go` — 4 tests

## Public API Surface (file:line)

```go
// internal/semantic/types.go:54
type WorkspaceID string
// internal/semantic/types.go:65
type RepoID string

// internal/semantic/live/signal.go:32-39
type ChangeSource string
const ChangeSourceHelixEdit, ChangeSourceFsnotify, ChangeSourceManifestScan ChangeSource
func ChangeSourceCoalescerInternal() ChangeSource

// internal/semantic/live/signal.go:62-67 (struct), 70-79 (Kind enum), 81-88 (event)
type WorkspaceChangeSignal struct{ WorkspaceID; Paths; Source; ObservedAt }
type SourceChangeKind string
const ChangeFileCreated, ChangeFileModified, ChangeFileDeleted,
      ChangeFileRenamed, ChangeHelixEdit, ChangeBulkUpdate SourceChangeKind
type SourceChangeEvent struct{ RepoID; Kind; Path; OldPath; Source; ObservedAt }

// internal/semantic/live/classifier.go:14-20 + 25-34 + 51-57
type FileHashLookup interface { EffectiveContentHash(...) (string, bool, error) }
type FileHasher func(absPath string) (string, error)
func ClassifyPathChange(ctx, repoID, absPath, lookup, hasher, source) (Kind, bool, error)

// internal/semantic/live/coalescer/coalesce.go:42 + 88
func CoalesceEvents(events []SourceChangeEvent, threshold int) []SourceChangeEvent
func MergeChange(prev, cur SourceChangeEvent) (SourceChangeEvent, bool)

// internal/semantic/live/coalescer/coalescer.go:14, 20-29, 33-40, 43-49, 81, 102, 110, 121
type EventHandler interface { Dispatch(ctx, ev) error }
type Logger interface { Warn(msg, args...) }
type Config struct { Debounce; MaxBatchDelay; BulkChangeThreshold; QueueSize }
func DefaultConfig() Config
func New(ws, cfg, handler, logger) *Coalescer
func (c *Coalescer) Enqueue(ev)
func (c *Coalescer) Drops() uint64
func (c *Coalescer) Run(ctx) error

// internal/semantic/live/handler/handler.go:31, 38-42, 48-50, 56, 62, 71, 78
type OverlayWriter interface { BeginOverlayTx(ctx, repoID) (OverlayTx, error) }
type OverlayTx interface { Epoch(); UpsertOverlayFile; MarkFileDeleted; Commit; Rollback }
type IncrementalScheduler interface { ScheduleIncremental(ws, changes) JobID }
type Hasher func(absPath string) (string, error)
type Logger interface { Warn; Info }
type Handler struct { Store; Hasher; Sched; Logger }
func New(store, hasher, sched, logger) *Handler
// methods: Dispatch, UpdateChangedFile, HandleFileDeleted, HandleFileRenamed, HandleBulkUpdate

// internal/semantic/live/service/service.go:39, 53, 65, 84, 100, 113, 132
type Service struct { ... }
type ClassifierFunc func(ctx, repoID, path, source) (Kind, bool, error)
func New(cfg, handler, classifier, repoIDFor, logger) *Service
func (s *Service) Start(ctx, ws)
func (s *Service) Stop(ws)
func (s *Service) OnEdit(ctx, ws, paths) error  // implements kernel.EditNotifier
func (s *Service) OnWorkspaceChanged(ctx, sig) error
var _ kernel.EditNotifier = (*Service)(nil)  // compile-time pin

// internal/semantic/live/lspqueue/queue.go:18, 23, 28, 39, 51, 56
type RevalidateFileJob struct { RepoID; Path }
type Queue struct { ... }
func New(buffer int) *Queue
func (q *Queue) Enqueue(job) bool
func (q *Queue) Channel() <-chan RevalidateFileJob
func (q *Queue) Len() int

// internal/semantic/scheduler/scheduler.go:46 (struct field), 56-66 (interface),
//   80-87 (setter), 100-141 (real dispatch)
type IncrementalHandler interface { UpdateChangedFile; HandleFileDeleted }
func (s *Scheduler) SetIncrementalHandler(h IncrementalHandler)
func (s *Scheduler) ScheduleIncremental(ws, changes) JobID  // real dispatch (no sentinel)

// internal/semantic/live/handler/handler.go:194 — compile-time pin
var _ scheduler.IncrementalHandler = (*Handler)(nil)
```

## Executor Decisions (PLAN-RECORDED)

### a. Classifier-with-helix_edit short-circuit — RESOLVED to "short-circuit"

`ClassifyPathChange` checks `source == ChangeSourceHelixEdit` first and returns `(ChangeHelixEdit, true, nil)` without any `os.Lstat` or `lookup` call. Justification (T-60-04-07):

- The kernel hook (60-03) fires AFTER `OverwriteFile` / `CreateFile` returns nil, so the on-disk content is already in the post-edit state.
- The hook may race with a follow-up tool call (e.g., delete-then-recreate inside a single agent turn), but the classifier's job is to label the signal with its source — distinguishing "helix edited this" from "fsnotify saw it move" is more useful for downstream coalescing than re-classifying via I/O.
- The handler's dispatch path treats `ChangeHelixEdit` identically to `ChangeFileModified` (both go to `UpdateChangedFile`), so the short-circuit only affects the metric label, not the overlay write.

### b. Symlink rejection — RESOLVED to `os.Lstat` + treat-as-missing

`ClassifyPathChange` uses `os.Lstat` (not `os.Stat`) and additionally checks `info.Mode()&os.ModeSymlink != 0`; if true, the path is downgraded to "missing" before the known/unknown switch. T-60-04-03 mitigation: matches the existing repomap walker's symlink-rejection invariant. A dangling-symlink test (`TestClassifyPathChange_DanglingSymlinkClassifiedAsMissing`) skips on filesystems that don't support symlinks.

### c. Tombstone-API path — RESOLVED to `tx.MarkFileDeleted`

`HandleFileDeleted` calls `tx.MarkFileDeleted(ctx, path)` from 60-02's `OverlayTx` surface, NOT an empty-hash `UpsertOverlayFile` placeholder. The 60-02 SUMMARY confirms `MarkFileDeleted` writes a real tombstone row (`status='deleted'`, `write_epoch` advanced) and is the single-owner API per CONTEXT.md domain item #4.

### d. IncrementalHandler interface placement — RESOLVED to scheduler package

The interface lives in `internal/semantic/scheduler` (not in `internal/semantic/live`). Reason: `live/handler` imports `scheduler` for `FileChange` and `JobID`; declaring `IncrementalHandler` in `live` would force `scheduler → live` to satisfy the type. Putting the interface in `scheduler` (consumer-defined) and the implementation in `live/handler` keeps imports one-way (`scheduler ← live/handler`) and avoids the cycle.

### e. Service sub-package — REQUIRED by import-cycle structure

The Service composes types from both `live` (for `WorkspaceChangeSignal`, `SourceChangeEvent`) and `live/coalescer` (for `Config`, `Coalescer`, `EventHandler`, `Logger`). Because `live/coalescer` imports `live`, `Service` cannot live in package `live` (would form `live → live/coalescer → live`). Solution: `internal/semantic/live/service` sub-package. Documented in the package doc.

### f. MergeChange applied at accept time — Rule 1 bug fix

The original PLAN noted "last-write-wins at accept time; CoalesceEvents re-applies merge rules at flush". This loses the `created+deleted=drop` signal: by accept time the pending map only holds the last event (`deleted`), so when `CoalesceEvents` runs at flush it sees a single `[deleted]` event with no `prev`, and the event survives the no-op rule.

Fix: `Coalescer.accept` now applies `MergeChange(prev, cur)` against the existing pending entry (if any). When `keep=false`, the entry is deleted from `pending`, preserving the `created+deleted=drop` invariant. Caught by `TestCoalescer_NoOpFlushDoesNotDispatch`. Tagged as `[Rule 1 - Bug]` in the per-task commit message and documented inline.

## Test Counts per Acceptance Criterion

| Acceptance | Test File | Tests |
|------------|-----------|-------|
| #4 (CoalesceEvents pure, all 6 SPEC §16.2 rules) | coalesce_test.go | 9 subtests of TestCoalesceEvents_MergeRules + TestCoalesceEvents_DeterministicOrdering + TestCoalesceEvents_EmptyInput + TestMergeChange_DropOnCreatedDeleted = 11 |
| #5 (bulk-collapse threshold) | bulk_test.go | 2 (over-threshold collapse + threshold-zero disables collapse) |
| #8 (no-op flush does not advance epoch) | noop_test.go | TestNoOpDoesNotAdvanceEpoch (handler-side guard) + TestCoalescer_NoOpFlushDoesNotDispatch (coalescer-side guard) = 2 |
| Coalescer goroutine | coalescer_test.go | 6 (debounce, max-batch, non-blocking-enqueue, no-op-flush, per-ws isolation, per-event-error batch) |
| Classifier 5 rules + helix-edit + symlink | classifier_test.go | 7 |
| Handler dispatch (4 branches + rollback + unknown-kind + rename) | noop_test.go | 7 |
| Service implements EditNotifier | service_test.go | 6 (non-blocking, per-ws routing, OnWorkspaceChanged source-label, EditNotifier satisfaction, Start idempotent, Stop drains) |
| ScheduleIncremental real dispatch | scheduler_incremental_test.go | 4 (per-kind dispatch, real-JobID, state transition, nil-handler safe) |
| LSPQueue typed handoff | queue_test.go | 2 |

**Total new tests: 47** (counting subtests as units: 6 named CoalesceEvents subtests + 1 ordering + 1 empty + 1 drop = 9 wrapped; the rest are top-level).

## Verification Run (final)

| Gate | Result |
|------|--------|
| `go build ./...` | clean (only pre-existing CGO Swift binding warning) |
| `go vet ./...` | clean |
| `go test ./internal/semantic/live/... -count=1 -race` | ok all 4 sub-packages |
| `go test ./internal/semantic/scheduler/... -count=1 -race` | ok |
| `go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./internal/kernel/... ./internal/semantic/...` | clean (no kernel→semantic imports) |
| `go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./internal/semantic/live/...` | clean |
| `grep -c 'phase60-incremental-stub' internal/semantic/scheduler/scheduler.go` | 0 (stub fully replaced) |
| `grep -E 'var _ kernel\.EditNotifier' internal/semantic/live/service/service.go` | matches |
| `grep -E 'var _ scheduler\.IncrementalHandler' internal/semantic/live/handler/handler.go` | matches |

## Threat Model Outcomes

| Threat ID | Disposition | Realized mitigation |
|-----------|-------------|---------------------|
| T-60-04-01 | mitigate | One coalescer goroutine per workspace; bulk-collapse caps fan-out at 1 ChangeBulkUpdate event per flush |
| T-60-04-02 | mitigate | `Service.coalescers map[workspace.WorkspaceKey]*Coalescer` keyed on workspace key; `TestLiveService_OnEdit_RoutesToCorrectWorkspace` + `TestCoalescer_PerWorkspaceIsolation` pin the invariant |
| T-60-04-03 | mitigate | `ClassifyPathChange` uses `os.Lstat` + symlink-mode check; dangling symlink classified as missing (`TestClassifyPathChange_DanglingSymlinkClassifiedAsMissing`) |
| T-60-04-04 | mitigate | Coalescer.flush short-circuits `len(pending) == 0` AND `len(merged) == 0`; handler's `TestNoOpDoesNotAdvanceEpoch` pins the guard |
| T-60-04-05 | mitigate | `Coalescer.Enqueue` uses select-default-drop; `TestLiveService_OnEdit_NonBlocking` pins <100us avg latency under 1000-call saturation with a dead consumer |
| T-60-04-06 | accept-for-P04 | ENOSPC handling lives in 60-05A watcher; P04 logs are single-shot per dispatch error |
| T-60-04-07 | mitigate | Classifier short-circuits on `ChangeSourceHelixEdit` (no `os.Lstat`); kernel hook ordering (60-03 TestOnEditCalledOnSuccess) ensures the file is on disk by classifier-call time |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] MergeChange applied at ACCEPT time, not just flush**
- **Found during:** Task 4 (TestCoalescer_NoOpFlushDoesNotDispatch failed)
- **Issue:** Original `Coalescer.accept` did `c.pending[key] = ev` (last-write-wins). When created+deleted both arrive on the same path, pending holds `[deleted]`. At flush, `CoalesceEvents([deleted])` runs `MergeChange(no-prev, deleted)` which treats the event as a singleton and KEEPS it. Result: a no-op flush emits a spurious deleted Dispatch.
- **Fix:** `accept()` now looks up the existing entry, applies `MergeChange(prev, cur)`, and either replaces (`keep=true`) or deletes (`keep=false`). Preserves the SPEC §16.2 drop-out semantics at accept time so the pending map is always coalescer-correct.
- **Files modified:** `internal/semantic/live/coalescer/coalescer.go` (accept method)
- **Commit:** `57a55037`

**2. [Rule 3 - Blocking] Import cycle between live and live/coalescer**
- **Found during:** Task 5 (compile error after first attempt at `service.go` in package `live`)
- **Issue:** `live/coalescer` imports `live` (for `SourceChangeEvent`, `ChangeBulkUpdate`, etc.). Putting `Service` in package `live` would compose `live → live/coalescer → live` — illegal.
- **Fix:** Moved `Service` to `internal/semantic/live/service` sub-package. The package doc explains the cycle break.
- **Files modified:** `internal/semantic/live/service/service.go` (new), `internal/semantic/live/service/service_test.go` (new)
- **Commit:** `3428bad8`

### Plan-permitted EXECUTOR DECISIONS made

- **Classifier short-circuits on helix_edit** (T-60-04-07). Documented in code comments and decision (a) above.
- **Hasher param reserved** (60-04 PLAN RED 2 ALTERNATIVE). The classifier signature still includes `_ FileHasher` so future hash-equality optimization can land without an API break.
- **os.Lstat + symlink rejection** (T-60-04-03). Documented in classifier.go and decision (b) above.
- **HandleFileDeleted via tx.MarkFileDeleted** (60-02 single-owner API). Documented in handler.go and decision (c) above.
- **IncrementalHandler interface in scheduler package** (not live). Documented in scheduler.go interface comment and decision (d) above.

### Architectural / Scope Changes

None. All deviations are within plan-permitted decision space (executor choices flagged in the plan) or Rule 1 bug fixes against the algorithmic core.

## Auth Gates

None.

## Known Stubs

- `internal/semantic/live/lspqueue/queue.go` is producer-only by design — Phase 61 wires the consumer worker. The package doc + Enqueue doc both say so explicitly. NOT a stub in the "incomplete plan" sense; a documented Phase 60/61 split.
- `Handler.HandleBulkUpdate` passes `nil` changes to `ScheduleIncremental` — the scheduler treats this as a "rewalk this workspace" hint. Phase 60 P05B is responsible for the actual manifest-scan-and-dispatch loop; this Handler emits the hint and lets the scheduler+live-handler chain re-walk the workspace.
- `Handler.HandleFileRenamed` ships the SPEC §16.5 fallback (delete + create), NOT the content-hash lineage check. The plan calls this out as the P60 form; the lineage check is reserved for a future revision.

These are documented limitations carried by design from the plan, not stubs that block plan completion.

## Threat Flags

None — no new network/auth/file-access surface beyond the existing kernel-side `ValidatePath` gate (which fires before any `OnEdit` emission per 60-03), and the classifier's `os.Lstat` (which already gates on symlinks per T-60-04-03 mitigation).

## DuckDB Schema-3 Quirks Observed

None directly in this plan — all interaction with the overlay schema goes through the 60-02 `OverlayTx` API which already absorbed the schema-3 quirks (DuckDB ALTER TABLE NOT NULL DEFAULT rejection, per-id IN-list iteration, sync.Once-guarded unlock). The handler tests use a fakeStore mock, so no live DuckDB driver is exercised at the P04 unit-test boundary.

## Self-Check: PASSED

- [x] `internal/semantic/live/signal.go` — created, exports WorkspaceChangeSignal + ChangeSource enum + SourceChangeKind enum + SourceChangeEvent
- [x] `internal/semantic/live/classifier.go` — created, exports ClassifyPathChange + FileHashLookup + FileHasher
- [x] `internal/semantic/live/coalescer/coalesce.go` — created, exports CoalesceEvents + MergeChange
- [x] `internal/semantic/live/coalescer/coalescer.go` — created, exports Coalescer + Config + EventHandler + Logger + DefaultConfig + New
- [x] `internal/semantic/live/handler/handler.go` — created, exports Handler + Dispatch + interfaces; var-_ pin on scheduler.IncrementalHandler present
- [x] `internal/semantic/live/service/service.go` — created (sub-package), exports Service + ClassifierFunc + New; var-_ pin on kernel.EditNotifier present
- [x] `internal/semantic/live/lspqueue/queue.go` — created, exports Queue + RevalidateFileJob
- [x] `internal/semantic/scheduler/scheduler.go` — modified, IncrementalHandler interface + SetIncrementalHandler + real ScheduleIncremental body
- [x] `internal/semantic/types.go` — modified, RepoID typed alias added
- [x] Commit `d4810a63` (CoalesceEvents) — found in git log
- [x] Commit `bcfafa95` (ClassifyPathChange) — found in git log
- [x] Commit `7a5e5c7f` (handler dispatch) — found in git log
- [x] Commit `57a55037` (Coalescer goroutine + accept-time MergeChange Rule 1 fix) — found in git log
- [x] Commit `3428bad8` (Service + EditNotifier seam) — found in git log
- [x] Commit `7ffec887` (ScheduleIncremental real dispatch) — found in git log
- [x] Commit `41e915c5` (lspqueue) — found in git log
- [x] All plan verification gates pass (go vet, go test -race, vet-nokernel2semantic, vet-noduckdb, sentinel-grep, var-_ greps)
