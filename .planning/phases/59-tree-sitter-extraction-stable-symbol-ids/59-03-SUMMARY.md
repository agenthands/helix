---
phase: 59
plan: 03
subsystem: semantic
tags: [semantic, scheduler, ready-gate, lifecycle, priority-queue, state-machine]
requires: [59-01, 59-02]
provides:
  - internal/semantic/scheduler package
  - ExtractionScheduler interface (5 methods including RequireReady)
  - SemanticIndexState 6-value closed enum (not_started|indexing|ready|partial|failed|stale)
  - SemanticStatus / IndexError / JobID / InitialExtraction{,Mode} / FileChange types
  - Idempotent ScheduleInitialExtraction (D-04 invariant — concurrent calls return same JobID)
  - ScheduleIncremental Phase 60 stub (returns sentinel JobID, body deferred)
  - Subscribe channel-based fanout (cap 8, non-blocking send → drop on full)
  - RequireReady — sole semantic readiness API (no time.Sleep polling, D-04 acceptance #9)
  - ReadyPolicy / ReadyResult types + DefaultReadyPolicy() returning D-04 defaults
  - PriorityQueue with 4-tier ordering and deterministic lexical tie-break
  - ClassifyByExtension — bounded {go|typescript|python|other} label for SemanticExtraction metric
  - semantic.WorkspaceID type alias (added to internal/semantic/types.go)
  - TestSetStatus / HasInflightJob test-only helpers (T-59-03-03 mitigation: gated to _test.go)
affects:
  - 59-04 (per-language providers — independent in Wave 2; integrates via 59-05 wiring)
  - 59-05 (daemon bootstrap — wires Scheduler into kernel.ActivateWorkspace, supplies registry+inflight+repomap sets)
  - 60-* (live overlay watcher — fills ScheduleIncremental body, may add Unsubscribe)
  - 64-* (semantic_graph_status MCP tool — wraps Scheduler.Status / Subscribe)
tech-stack:
  added: []
  patterns:
    - "Channel-based readiness wait (Subscribe + time.NewTimer + ctx.Done() escape paths)"
    - "Static-grep test enforcing D-04 acceptance #9 (no time.Sleep in non-test sources)"
    - "Min-heap priority queue with (priority, path) ordering for deterministic tie-break"
    - "Test-only state injectors via export_test.go with TestSetStatus/HasInflightJob"
    - "Idempotent admission via per-workspace in-flight-job map (D-04)"
key-files:
  created:
    - internal/semantic/scheduler/doc.go
    - internal/semantic/scheduler/state.go
    - internal/semantic/scheduler/scheduler.go
    - internal/semantic/scheduler/scheduler_test.go
    - internal/semantic/scheduler/export_test.go
    - internal/semantic/scheduler/ready.go
    - internal/semantic/scheduler/ready_test.go
    - internal/semantic/scheduler/priority.go
    - internal/semantic/scheduler/priority_test.go
  modified:
    - internal/semantic/types.go (added WorkspaceID type alias — see Deviations)
decisions:
  - "RequireReady ranks states via stateRank() helper (Ready=3, Partial=2, Indexing/Stale=1, NotStarted/Failed=0). MinState=SemanticPartial accepts both partial and ready transitions. Cleaner than the RESEARCH.md's `st.State >= policy.MinState` direct compare which would couple ordering to enum string values."
  - "Subscribe AFTER fast-path returns (not before). Fast-path callers (ready / partial+AllowPartial / failed) avoid allocating a channel entirely. Wave 2 does NOT ship Unsubscribe — scheduler-lifetime-bound subscriptions are acceptable because scheduler lifetime == daemon lifetime; P05 may add explicit Unsubscribe when wiring deactivation."
  - "BuildPriorityQueue takes inflight + repomap sets as parameters rather than importing internal/repomap directly. D-01 allows the import but the pure-data API keeps priority.go testable without a running repomap. P05 (daemon wiring) computes the sets at scheduling time and passes them in."
  - "Test-only helpers (TestSetStatus, HasInflightJob) live in export_test.go gated behind the build tag implied by the _test.go suffix. T-59-03-03 mitigation: production code transitions only via ScheduleInitialExtraction or the (Phase 60) extraction-completion callback."
  - "ScheduleIncremental returns a sentinel JobID 'phase60-incremental-stub' rather than panicking — Wave 2 ships the interface so consumers compile today; the body lands in Phase 60."
metrics:
  duration_minutes: 11
  tasks_completed: 3
  tests_added: 18
  tests_passing: 18
  go_vet: clean
  files_created: 9
  files_modified: 1
  lines_of_go: 1166
completed_date: 2026-05-04
---

# Phase 59 Plan 03: Scheduler — ExtractionScheduler, RequireReady gate, priority queue

Lands the `internal/semantic/scheduler/` package: ExtractionScheduler interface, idempotent initial-walk admission, centralized RequireReady consumer-side gate (no time.Sleep polling — D-04 acceptance #9), SemanticIndexState 6-value enum, and the priority-queue infrastructure that consumes injected inflight + repomap-PageRank sets for 4-tier initial-walk ordering. P02 owns the *what* (Provider interface, fact types, stable-IDs); P03 owns the *when* (scheduling, lifecycle, readiness). Wave 2.

## Package Layout

| File | Responsibility |
|------|----------------|
| `doc.go` | Package docstring with D-04 invariants and allowed/forbidden imports |
| `state.go` | `SemanticIndexState` enum, `SemanticStatus`, `IndexError`, `JobID`, `InitialExtraction{Mode}`, `FileChange` |
| `scheduler.go` | `ExtractionScheduler` interface + concrete `*Scheduler`: `NewScheduler`, `ScheduleInitialExtraction` (idempotent), `ScheduleIncremental` (Phase 60 stub), `Status`, `Subscribe`, `transitionUnlocked`, `ErrSemanticFailed` |
| `ready.go` | `ReadyPolicy`, `ReadyResult`, `DefaultReadyPolicy()`, `(*Scheduler).RequireReady`, `stateRank` |
| `priority.go` | `PriorityQueue` handle, `PoppedFile`, `FilePriority` 4-tier constants, `BuildPriorityQueue`, `ClassifyByExtension` |
| `export_test.go` | Test-only `TestSetStatus` and `HasInflightJob` helpers |
| `*_test.go` | 18 tests (4 scheduler + 10 ready + 4 priority) |

## ExtractionScheduler Interface Contract

```go
type ExtractionScheduler interface {
    ScheduleInitialExtraction(ws semantic.WorkspaceID, req InitialExtraction) JobID
    ScheduleIncremental(ws semantic.WorkspaceID, changes []FileChange) JobID
    Status(ws semantic.WorkspaceID) SemanticStatus
    Subscribe(ws semantic.WorkspaceID) <-chan SemanticStatus
    RequireReady(ctx context.Context, ws semantic.WorkspaceID, policy ReadyPolicy) (ReadyResult, error)
}
```

D-04 invariants honored:
- Idempotent `ScheduleInitialExtraction` (concurrent calls return the same in-flight JobID).
- One active extraction job per workspace.
- `ScheduleIncremental` is a Phase 60 stub — returns `"phase60-incremental-stub"` sentinel.
- `Subscribe` returns a buffered channel (cap 8) with non-blocking send (T-59-03-02: slow consumer cannot stall publisher).
- `RequireReady` is the SOLE readiness API — `TestScheduler_NoTimeSleepPolling` enforces no `time.Sleep` in non-test sources via static grep (D-04 acceptance #9).

## RequireReady Semantics

`DefaultReadyPolicy()` returns the D-04 defaults verbatim:

| Field | Default | Meaning |
|-------|---------|---------|
| `Timeout` | `30 * time.Second` | Max wait; on expiry returns best-partial |
| `AllowPartial` | `true` | `SemanticPartial` counts as ready |
| `MinState` | `SemanticPartial` | Lowest state that satisfies the wait |
| `TriggerIfCold` | `true` | Kick `ScheduleInitialExtraction` if `not_started` |

Decision tree (verified by 10 tests in `ready_test.go`):

| Input state | Behavior |
|-------------|----------|
| `SemanticReady` | Return immediately, `Ready=true` |
| `SemanticPartial` + `AllowPartial=true` | Return immediately, `Ready=true, Partial=true` |
| `SemanticPartial` + `AllowPartial=false` | Wait for `SemanticReady` transition |
| `SemanticFailed` | Return `ErrSemanticFailed`, `Ready=false` |
| `SemanticNotStarted` + `TriggerIfCold=true` | Kick scheduler then wait |
| `SemanticIndexing` / `SemanticStale` | Wait for `MinState` transition or timeout |
| Timeout | Return best-partial (`Ready=true` iff `FilesDone>0`), no error |
| `ctx.Done()` | Return latest status with `ctx.Err()` |
| `SemanticFailed` arrives via Subscribe | Return `ErrSemanticFailed` mid-wait |

`stateRank()` orders states for `MinState` comparison: Ready=3, Partial=2, Indexing/Stale=1, others=0.

## Priority Queue 4-Tier Ordering

`BuildPriorityQueue(files, inflightToolReferenced, repomapImportant)` assigns priorities:

| Tier | Constant | Trigger |
|------|----------|---------|
| 1 (highest) | `PriorityInflightToolReferenced` (0) | File is in `inflightToolReferenced` set |
| 2 | `PriorityRepomapImportant` (1) | File is in `repomapImportant` set (top-quartile PageRank) |
| 3 | `PriorityFirstClassSource` (2) | First-class language (.go / .ts / .tsx / .js / .jsx / .mjs / .cjs / .py) |
| 4 (lowest) | `PriorityNonFirstClass` (3) | Non-first-class — file-row-only emit (D-05) |

Same-priority entries pop in lexical-path order (deterministic tie-break).

`ClassifyByExtension` returns one of `{"go", "typescript", "python", "other"}` — closed enum matching the bounded-label allowlist primed in `internal/obs/metrics.go` for `helix_semantic_extraction_total`'s `language` label. Case-insensitive on the file extension.

**Repomap import not needed in priority.go.** P03 keeps the queue as a pure data structure that consumes a `map[string]struct{}` for both inflight and repomap sets — D-01 *allows* `internal/repomap` from this package, but isolating the data path here makes the queue testable without a running repomap. P05 (daemon wiring) computes the sets and passes them at scheduling time.

## Notes for P05 (daemon wiring)

When the daemon constructs the Scheduler, it should:

1. **Inject the registry**:
   ```go
   sched := scheduler.NewScheduler(extractRegistry) // *extract.Registry from P02 wiring
   ```

2. **At workspace activation** (kernel callback):
   ```go
   sched.ScheduleInitialExtraction(ws, scheduler.InitialExtraction{
       Reason: "workspace_activation",
       Mode:   scheduler.ModeAuto,
   })
   ```
   The body of the actual walk lives where P05 wires the registry → store path; this plan ships only the orchestration surface.

3. **Build the priority queue inside the (P05-supplied) extraction worker**:
   ```go
   inflight := daemon.InflightToolReferencedFiles(ws)        // current MCP tool call's files
   repomapTop := daemon.RepomapTopQuartile(ws)               // PageRank top quartile (Phase 60+ enables; nil acceptable today)
   pq := scheduler.BuildPriorityQueue(walkedFiles, inflight, repomapTop)
   for pq.Len() > 0 {
       f := pq.PopFile()
       provider, ok := registry.Provider(f.Language)
       if !ok {
           // tier 4 / non-first-class: file-row-only emit (D-05)
       }
       // ... extract, write facts, transition status
   }
   ```

4. **State publishing**: extraction worker calls a P05-defined "completion callback" that publishes `SemanticReady` / `SemanticPartial` / `SemanticFailed` via `Scheduler.transitionUnlocked` (currently package-private; P05 may surface it as an exported method like `Complete(ws, status)` when the wiring lands).

5. **I10 deferral**: Pass `nil` for `repomapImportant` until Phase 60+ wires the live PageRank pipeline. The tier is structurally reachable but unused — verified by `TestBuildPriorityQueue_FourTierOrdering` covering both nil and non-nil sets.

6. **Consumers** (MCP tool handlers needing semantic data) call:
   ```go
   res, err := sched.RequireReady(ctx, ws, scheduler.DefaultReadyPolicy())
   if err != nil { /* SemanticFailed or ctx cancel */ }
   if !res.Ready { /* timeout best-partial with no files indexed */ }
   ```
   Never poll `Status()` directly — RequireReady is the SOLE readiness API.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 — Blocking] Added `semantic.WorkspaceID` type alias**

- **Found during:** Task 1 setup
- **Issue:** The plan's `<interfaces>` block claimed `WorkspaceID` was "already declared in internal/semantic/types.go" but it did not exist anywhere in the repo (verified by `grep -rn "type WorkspaceID" internal/`). P02 SUMMARY confirms only `ImportID`, `TypeFactID`, and `HeritageID` were added in P02. Without the type, the scheduler's interface signatures could not compile.
- **Fix:** Added `type WorkspaceID string` to `internal/semantic/types.go` with a doc comment matching CONTEXT.md D-04's expected use ("opaque identifier for a workspace registered with the daemon's kernel ... key under which the extraction scheduler tracks per-workspace state").
- **Files modified:** `internal/semantic/types.go`
- **Commit:** `a8c7385f` (Task 1 — bundled with the scheduler skeleton because the dependency is load-bearing for the package to build).

**2. [Plan structure] Split priority tests into a dedicated `priority_test.go`**

- **Found during:** Task 3 design
- **Issue:** Plan said "Add tests in `scheduler_test.go` (extending the file from Task 1)" for the priority-queue tests. Adding them to `scheduler_test.go` in Task 1 would have made Task 1's commit non-self-contained (the tests would reference `BuildPriorityQueue` / `PriorityQueue` types that don't exist until Task 3).
- **Fix:** Created `priority_test.go` as a separate test file. All 4 priority tests live there; `scheduler_test.go` only contains the scheduler-skeleton tests. Each task commit remains self-contained and atomically buildable.
- **Files affected:** `internal/semantic/scheduler/priority_test.go` (created instead of editing `scheduler_test.go`).
- **Rule:** Plan-internal cleanup; not Rule 1/2/3.

**3. [Plan structure] Added `TestRequireReady_TriggerIfCold_Disabled`, `TestRequireReady_FailedDuringWait`, `TestRequireReady_ContextCancel` to ready_test.go**

- **Found during:** Task 2 implementation
- **Issue:** The plan listed 7 ready tests; behavior coverage gaps existed for (a) `TriggerIfCold=false` not kicking the scheduler, (b) `SemanticFailed` arriving mid-wait, (c) `ctx.Done()` cancellation path.
- **Fix:** Added 3 supplementary tests (still inside `ready_test.go`). All 10 RequireReady tests pass. Documented as deviations because they extend the plan's test list.
- **Rule:** Rule 2 (auto-add missing critical functionality — these branches are reachable code paths and deserve coverage).

**4. [Plan structure] `BuildPriorityQueue_EmptyInputs` test added**

- Defensive test for nil/empty inputs — `BuildPriorityQueue(nil, nil, nil)` must yield `Len()==0`. Same Rule 2 reasoning.

**5. [Plan structure] `ClassifyByExtension` case-insensitivity coverage extended**

- Added `weird.GO`, `upper.PY`, `shouty.TSX` cases to verify the `strings.ToLower` step. Rule 2.

### Auth gates

None — fully autonomous Go-only execution.

## Tests

| Test | Result | Duration |
|------|--------|----------|
| TestScheduler_Idempotent | PASS | <1ms |
| TestScheduler_OneJobPerWorkspace | PASS | <1ms |
| TestScheduler_StateTransitions | PASS | <1ms |
| TestScheduler_NoTimeSleepPolling | PASS | <1ms |
| TestDefaultReadyPolicy | PASS | <1ms |
| TestRequireReady_ReadyImmediate | PASS | <1ms |
| TestRequireReady_PartialWithAllowPartial | PASS | <1ms |
| TestRequireReady_PartialWithoutAllowPartialWaits | PASS | 50ms |
| TestRequireReady_TriggerIfCold | PASS | 80ms |
| TestRequireReady_TriggerIfCold_Disabled | PASS | 50ms |
| TestRequireReady_TimeoutReturnsBestPartial | PASS | 30ms |
| TestRequireReady_FailedReturnsError | PASS | <1ms |
| TestRequireReady_FailedDuringWait | PASS | 30ms |
| TestRequireReady_ContextCancel | PASS | 20ms |
| TestBuildPriorityQueue_FourTierOrdering | PASS | <1ms |
| TestBuildPriorityQueue_DeterministicTieBreak | PASS | <1ms |
| TestBuildPriorityQueue_EmptyInputs | PASS | <1ms |
| TestClassifyByExtension_BoundedLabels | PASS | <1ms |

`go test ./internal/semantic/scheduler/ -count=1 -timeout 60s` — 18/18 PASS.
`go vet ./internal/semantic/scheduler/` — clean.
`grep -v '^[[:space:]]*//' internal/semantic/scheduler/*.go | grep -v '_test.go' | grep -c 'time\.Sleep'` — 0 (D-04 acceptance #9 holds).

## Commits

| # | Hash | Subject |
|---|------|---------|
| 1 | a8c7385f | feat(59-03): scheduler package skeleton with state enum and idempotent ScheduleInitialExtraction |
| 2 | 7ecf2bfd | feat(59-03): RequireReady gate with channel-based wait and ReadyPolicy defaults |
| 3 | ed7517a0 | feat(59-03): initial-walk priority queue with 4-tier ordering and bounded language classifier |

## Self-Check: PASSED

Files claimed:
- internal/semantic/scheduler/doc.go — FOUND
- internal/semantic/scheduler/state.go — FOUND
- internal/semantic/scheduler/scheduler.go — FOUND
- internal/semantic/scheduler/ready.go — FOUND
- internal/semantic/scheduler/priority.go — FOUND
- internal/semantic/scheduler/scheduler_test.go — FOUND
- internal/semantic/scheduler/ready_test.go — FOUND
- internal/semantic/scheduler/priority_test.go — FOUND
- internal/semantic/scheduler/export_test.go — FOUND
- internal/semantic/types.go — FOUND (modified)

Commits claimed:
- a8c7385f — FOUND in git log
- 7ecf2bfd — FOUND in git log
- ed7517a0 — FOUND in git log
