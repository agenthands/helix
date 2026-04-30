---
phase: 53-obs-metrics-gaps
plan: 02
subsystem: observability
tags: [observability, lspool, metrics, helix]
requires:
  - "Plan 53-01: *obs.Metrics.LSPoolLookup helper + helix_lspool_lookups_total vector + cardinality bound test"
provides:
  - extended lspool.MetricsSink interface with LSPoolLookup(language, result string)
  - LookupHit / LookupMiss closed-enum constants on the lspool package
  - NoopSink stub for the new method
  - Pool.AcquireLease branch emission at the canonical D-02 share/spawn boundary
  - extended recordingSink test fixture tracking lookup events
affects:
  - downstream: operator dashboards / PromQL hit-ratio queries on helix_lspool_lookups_total
  - downstream: any future lspool sink implementer must add LSPoolLookup(language, result string) — wiring_test.go compile-time assertion catches drift
tech-stack:
  added: []
  patterns:
    - "branch-emission at the canonical observable boundary (mirror of evictWorkerLocked emission shape)"
    - "closed-enum constants on the consumer package (LookupHit/LookupMiss live in lspool, not obs)"
    - "miss-before-gate ordering so refusals (circuit-open, max-workers, dirty bypass) all count as misses (D-02)"
key-files:
  created: []
  modified:
    - internal/kernel/lspool/metrics.go (+15 lines: interface method, constants block, NoopSink stub)
    - internal/kernel/lspool/metrics_test.go (+115 lines: lookupEvent + recordingSink extension + 4 tests + snapshot signature update)
    - internal/kernel/lspool/pool.go (+7 lines: 2 emission lines + comments)
decisions:
  - "Pre-warmed worker pattern in TestPool_AcquireLease_LookupEmission/share_path_emits_hit uses NewWorker(...) directly rather than the shared fakeWorker(...) helper, because fakeWorker hard-codes a /tmp/test-<lang> WorkDir that does NOT match testKey().RepoRoot — workerForKeyLocked would reject it. NewWorker(id, lang, key.RepoRoot, \"true\", nil, testLogger()) + state.Store(WorkerReady) gets the share path to fire."
  - "spawn_path_emits_miss + dirty_path_emits_miss use MaxWorkers=0 to force ErrMaxWorkersReached AFTER the miss emit, avoiding any real LS process start in unit tests. Per D-02 the miss is emitted BEFORE circuit/max-workers checks, so refusals still count."
  - "Snapshot signature extended (workers, evictions, circuit, restarts, lookups) — lookups added as the LAST return value to keep call-site updates mechanical (existing tests use _ for unused trailing returns)."
metrics:
  duration: "~14 minutes"
  completed: "2026-04-30"
  tasks: 2
  files_modified: 4
  lines_added: 137
---

# Phase 53 Plan 02: lspool AcquireLease Lookup Instrumentation Summary

**One-liner:** Extended `lspool.MetricsSink` with `LSPoolLookup(language, result string)` and instrumented `Pool.AcquireLease` at the share-vs-spawn boundary so `helix_lspool_lookups_total{language,result}` now reports operator-facing cache hit-ratio for the LS worker pool.

## What Landed

### Interface extension (`internal/kernel/lspool/metrics.go`)

**Before:**
```go
type MetricsSink interface {
    LSPoolWorkersSet(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitStateSet(language string, state float64)
    LSPoolRestart(language string)
}
```

**After:**
```go
type MetricsSink interface {
    LSPoolWorkersSet(language string, delta float64)
    LSPoolEviction(language, reason string)
    LSPoolCircuitStateSet(language string, state float64)
    LSPoolRestart(language string)

    // LSPoolLookup increments the helix_lspool_lookups_total counter for a
    // (language, result) pair. result must be one of the LookupXxx constants
    // below (Phase 53 D-04 closed enum). Emitted at the share-vs-spawn
    // boundary in Pool.AcquireLease (Phase 53 D-02). Phase 53 D-14.
    LSPoolLookup(language, result string)
}
```

Plus a new constants block:
```go
const (
    LookupHit  = "hit"
    LookupMiss = "miss"
)
```

And the matching `NoopSink` stub:
```go
func (NoopSink) LSPoolLookup(string, string) {}
```

The pre-existing `var _ MetricsSink = NoopSink{}` assertion at `metrics.go:73` auto-validates the new method on `NoopSink` at compile time. The `var _ lspool.MetricsSink = (*obs.Metrics)(nil)` assertion in `internal/daemon/wiring_test.go:17` auto-validates that `*obs.Metrics` (whose `LSPoolLookup(language, result string)` helper landed in Plan 53-01 commit `a287a8f0`) still satisfies the extended interface.

### Pool.AcquireLease instrumentation (`internal/kernel/lspool/pool.go`)

Two new emission lines, no other logic changed:

```go
// If not dirty, look for existing warm worker for this language+workDir.
if !dirty {
    if w := p.workerForKeyLocked(wsKey); w != nil {
        lease := NewWorkerLease(sessionID, w, false)
        p.leases[sessionID] = lease
        p.logger.Info("shared lease acquired", "session", sessionID, "worker", w.ID())
        // Phase 53 D-02: share-path is the canonical "cache hit" boundary.
        p.metrics.LSPoolLookup(wsKey.Language, LookupHit)        // <-- NEW
        return lease, nil
    }
}

// Phase 53 D-02: miss is recorded BEFORE circuit/max-workers checks so
// refusals count as misses. Dirty acquires also reach this point because
// they bypass the workerForKeyLocked share branch by design.
p.metrics.LSPoolLookup(wsKey.Language, LookupMiss)               // <-- NEW

// Need a new worker. Check circuit breaker first.
cb := p.circuitForLanguage(wsKey.Language)
```

`grep -c 'p.metrics.LSPoolLookup' internal/kernel/lspool/pool.go` returns exactly **2** — no double-counting.

### Test scaffolding (`internal/kernel/lspool/metrics_test.go`)

- New `lookupEvent` struct mirroring the existing `evictionEvent` shape.
- `recordingSink.lookups []lookupEvent` field + mutex-guarded `LSPoolLookup` method.
- `snapshot()` extended to a 5-tuple `(workers, evictions, circuit, restarts, lookups)`; existing call sites updated to `_, _, ..., _` for trailing unused returns.
- New `TestPool_AcquireLease_LookupEmission` with three sub-tests pinning the D-02 boundary:
  - `share_path_emits_hit` — pre-warmed worker matching `testKey().RepoRoot`, `dirty=false` → 1 hit, 0 miss
  - `spawn_path_emits_miss` — fresh pool with `MaxWorkers=0`, `dirty=false` → 1 miss
  - `dirty_path_emits_miss` — pre-warmed worker, `dirty=true` (with `MaxWorkers=0`) → 1 miss (cache bypassed)
- New `TestMetricsSink_LookupResultConstants` mirroring `TestMetricsSink_EvictionReasonConstants`.
- `TestNoopSink_safe` extended with `LSPoolLookup` calls.

## Test Results

```text
$ go test ./internal/kernel/lspool/... -run 'TestPool_AcquireLease_LookupEmission|TestMetricsSink_LookupResultConstants' -v
=== RUN   TestMetricsSink_LookupResultConstants
--- PASS: TestMetricsSink_LookupResultConstants (0.00s)
=== RUN   TestPool_AcquireLease_LookupEmission
=== RUN   TestPool_AcquireLease_LookupEmission/share_path_emits_hit
=== RUN   TestPool_AcquireLease_LookupEmission/spawn_path_emits_miss
=== RUN   TestPool_AcquireLease_LookupEmission/dirty_path_emits_miss
--- PASS: TestPool_AcquireLease_LookupEmission (0.00s)
    --- PASS: TestPool_AcquireLease_LookupEmission/share_path_emits_hit (0.00s)
    --- PASS: TestPool_AcquireLease_LookupEmission/spawn_path_emits_miss (0.00s)
    --- PASS: TestPool_AcquireLease_LookupEmission/dirty_path_emits_miss (0.00s)
PASS

$ go test ./internal/kernel/lspool/...
ok      github.com/agenthands/helix/internal/kernel/lspool      4.943s

$ go test ./internal/daemon/... -run TestObsMetricsIsLSPoolSink -v
=== RUN   TestObsMetricsIsLSPoolSink
--- PASS: TestObsMetricsIsLSPoolSink (0.00s)

$ go test ./internal/obs/...
ok      github.com/agenthands/helix/internal/obs        10.570s

$ go vet ./internal/kernel/lspool/... ./internal/obs/... ./internal/daemon/...
ok
```

## Files Modified

| File | Lines added | Notes |
|---|---|---|
| `internal/kernel/lspool/metrics.go` | +15 | interface + constants + NoopSink stub |
| `internal/kernel/lspool/metrics_test.go` | +115 | lookupEvent + recordingSink + 4 tests + snapshot signature update |
| `internal/kernel/lspool/pool.go` | +7 | 2 emission lines + comments |
| `.planning/phases/53-obs-metrics-gaps/53-VALIDATION.md` | ±0 | TBD-04 / TBD-17 → 53-02-2.2 IDs, status flipped to ✅ green |
| **Total (code only)** | **+137** | |

No files deleted. No files renamed.

## Commits

| Task | Type | Hash | Message |
|---|---|---|---|
| 2.1 | test | `63e4401e` | add failing TestPool_AcquireLease_LookupEmission (Wave 0 RED) |
| 2.2 | feat | `b9629070` | instrument lspool AcquireLease at the share/spawn boundary |

## Verification

All success criteria met:

- [x] `lspool.MetricsSink` gains `LSPoolLookup(language, result string)` (D-14)
- [x] AcquireLease emits `result=hit` on the `workerForKeyLocked` share-path
- [x] AcquireLease emits `result=miss` on the spawn path, BEFORE the circuit/max-workers gate (D-02 lock — refusals count as misses)
- [x] Dirty acquires emit `result=miss` (D-02 lock — cache bypassed by design)
- [x] `*obs.Metrics` still satisfies the extended `lspool.MetricsSink` (existing wiring_test.go compile-time assertion auto-validates)
- [x] `grep -c 'p.metrics.LSPoolLookup' internal/kernel/lspool/pool.go` returns exactly 2 (no double-counting)
- [x] `go test ./internal/kernel/lspool/... ./internal/obs/... ./internal/daemon/...` all green
- [x] `go vet` clean

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] share_path_emits_hit failed because shared `fakeWorker` helper hard-codes `/tmp/test-<lang>` WorkDir.**

- **Found during:** Task 2.2 verification (`go test ./internal/kernel/lspool -run TestPool_AcquireLease_LookupEmission -v`)
- **Issue:** The first run of the share-path sub-test triggered an actual `gopls` spawn instead of taking the share branch. Root cause: `fakeWorker(id, lang)` constructs a `Worker` with `WorkDir = "/tmp/test-"+lang` (`metrics_test.go:127`), but `testKey()` returns `WorkspaceKey{RepoRoot: "/tmp/test-project", Language: "go"}`. `Pool.workerForKeyLocked` requires `w.WorkDir() == wsKey.RepoRoot`, so the predicate failed and AcquireLease fell through to the spawn path which tried to start a real gopls process.
- **Fix:** Replace `fakeWorker(...)` with an inline `NewWorker("w-go-1", key.Language, key.RepoRoot, "true", nil, testLogger())` + `state.Store(int32(WorkerReady))` so the worker's WorkDir matches the test key's RepoRoot. Applied to both `share_path_emits_hit` and `dirty_path_emits_miss` (the latter does not strictly need the match because dirty=true bypasses the share branch, but using the same pattern keeps the test fixtures uniform).
- **Files modified:** `internal/kernel/lspool/metrics_test.go`
- **Commit:** `b9629070` (folded into the Task 2.2 GREEN commit since the test only becomes runnable once Task 2.2's interface extension lands)

**2. [Rule 3 - Blocking] Task 2.1 RED commit missing `context` and `require` imports.**

- **Found during:** Task 2.2 first vet pass (`go vet ./internal/kernel/lspool/...`)
- **Issue:** The test scaffolding I committed in Task 2.1 (`63e4401e`) referenced `context.Background()` and `require.NoError(...)` but the file's existing import block only had `sync`, `testing`, `time`, `stretchr/testify/assert`. The compile error would have been there at Task 2.1 already (alongside the planned RED for `LSPoolLookup` undefined), but since the plan explicitly stated Task 2.1 should compile RED ("Tests will be RED until Task 2.2 lands the interface extension"), I missed that two distinct compile errors landed in `63e4401e` instead of one.
- **Fix:** Added `context` and `require` to the import block as part of Task 2.2's commit (where the file flips to GREEN anyway). Net effect on the merged feature branch: identical to a single combined commit, but with cleaner per-task atomic structure for `git bisect`. Documented in commit `b9629070`'s message.
- **Files modified:** `internal/kernel/lspool/metrics_test.go`
- **Commit:** `b9629070`

These are the only deviations. No architectural changes (Rule 4) and no missing critical functionality (Rule 2) were discovered. The plan's `<action>` blocks for Tasks 2.1 and 2.2 were followed as written modulo the two auto-fixes above.

### VALIDATION.md sign-off

Per the Wave 2 anti-pattern guidance, I replaced the placeholder `TBD-04` and `TBD-17` Task IDs in `.planning/phases/53-obs-metrics-gaps/53-VALIDATION.md` with the concrete `53-02-2.2` plan-task ID and flipped both rows' status from `⬜ pending` to `✅ green`. Other rows (Plans 53-03, 53-04, 53-05, 53-06) remain at `TBD` and `⬜ pending` — they belong to other executors.

## Threat surface scan

No new threat surface introduced beyond what plan 53-02's `<threat_model>` already documents:

- T-53-04 (cardinality DoS on `helix_lspool_lookups_total`): mitigation already in place from Plan 53-01 — the `result` label is a closed enum (`hit`/`miss`) carved out in `metrics_labels_test.go`; the `language` label is bounded by the langregistry; the cardinality bound test `TestMetrics_CardinalityBounds_LSPoolLookups` (also Plan 53-01) fails the build if `combo count > 2 × 52`. Plan 53-02's emission sites use `wsKey.Language` (validated upstream) and the closed-enum `LookupHit`/`LookupMiss` constants, so no new attack surface is created.
- T-53-05 (recordingSink tampering): test-only, mutex-guarded, no production exposure — `accept` disposition unchanged.

No `threat_flag` entries to add.

## Self-Check: PASSED

- [x] `internal/kernel/lspool/metrics.go` exists and contains `LSPoolLookup(language, result string)` interface method, `LookupHit`/`LookupMiss` constants, and `func (NoopSink) LSPoolLookup(string, string) {}`
- [x] `internal/kernel/lspool/pool.go` contains exactly 2 occurrences of `p.metrics.LSPoolLookup` (one with `LookupHit`, one with `LookupMiss`)
- [x] `internal/kernel/lspool/metrics_test.go` contains `TestPool_AcquireLease_LookupEmission`, `TestMetricsSink_LookupResultConstants`, `type lookupEvent struct`, `lookups []lookupEvent`, and `func (r *recordingSink) LSPoolLookup`
- [x] No `t.Skip(...)` calls in the modified test file
- [x] All three sub-tests are present (`share_path_emits_hit`, `spawn_path_emits_miss`, `dirty_path_emits_miss`)
- [x] Commit `63e4401e` (Task 2.1 RED) exists in `git log`
- [x] Commit `b9629070` (Task 2.2 GREEN) exists in `git log`
- [x] `go test ./internal/kernel/lspool/... ./internal/obs/... ./internal/daemon/...` all green
- [x] `go vet ./internal/kernel/lspool/... ./internal/obs/... ./internal/daemon/...` clean
- [x] 0 known stubs in modified files
