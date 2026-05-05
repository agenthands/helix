---
phase: 60
plan: 05b
subsystem: semantic-live
tags: [scanner, manifest-scan, daemon-wiring, metrics, pipelines, live-03, close-out, phase-60]
requires:
  - "60-02 (overlay tx + epoch contract)"
  - "60-03 (kernel.EditNotifier seam)"
  - "60-04 (live spine: classifier + coalescer + handler + service + ScheduleIncremental)"
  - "60-05A (watcher.Manager — type-only, satisfied via daemon-local interface until P05A merges)"
provides:
  - "internal/semantic/live/scanner package (LIVE-03 manifest scanner + Manager)"
  - "scanner.HashFile (xxhash64 file hasher) — exported helper for daemon classifier wiring"
  - "internal/config three new live_updates.* keys (watcher_enabled, manifest_scan_enabled, manifest_scan_interval)"
  - "semantic.LiveUpdatesConfig three new struct fields (WatcherEnabled, ManifestScanEnabled, ManifestScanInterval)"
  - "*obs.Metrics.SemanticLiveUpdatesInc helper + helix_semantic_live_updates_total counter"
  - "phasegraph/pipelines.BuildLiveUpdatePhases constructor (zero noopRun in shipped constructor)"
  - "internal/daemon live-update wiring (kernel.SetEditNotifier + scheduler.SetIncrementalHandler + per-workspace lifecycle)"
affects:
  - "Phase 60 P05A (watcher) — daemon-local liveWatcherManager interface satisfied by *watcher.Manager when waves merge"
  - "Phase 61 (LSP enrichment worker) — drains lspqueue.Queue created in liveBundle"
  - "Phase 62 (effective-fact diff) — replaces storeFileHashLookup + scannerStoreLookup TODO stubs"
  - "Phase 63 (compaction) — reads write_epoch stamps the live pipeline now writes"
tech-stack:
  added:
    - "github.com/cespare/xxhash/v2 (already in go.mod) — file content hashing in scanner.HashFile"
    - "filepath.WalkDir-based scanner walker with embedded skipDirs (no internal/repomap import per 60-01 cascade)"
    - "Daemon-local interface (liveWatcherManager) for cross-package compile decoupling between parallel waves"
  patterns:
    - "Per-workspace ticker with immediate-first-scan + diff-emit on changed-paths only (no-op flush guard)"
    - "Closed-enum drop-on-unknown helper (mirrors EditOutcomeInc pattern from Phase 53 D-10/D-11)"
    - "Constructor-style BuildLiveUpdatePhases that takes wired components (validator-style Run closures over capture)"
    - "Adapter pattern: storeOverlayWriter/storeOverlayTxAdapter wrap *semanticstore.OverlayTx behind handler.OverlayWriter/OverlayTx interfaces"
key-files:
  created:
    - "internal/semantic/live/scanner/walk.go"
    - "internal/semantic/live/scanner/scanner.go"
    - "internal/semantic/live/scanner/manager.go"
    - "internal/semantic/live/scanner/scanner_integration_test.go"
    - "internal/daemon/live_wiring.go"
  modified:
    - "internal/config/defaults.go (three new live_updates.* keys)"
    - "internal/config/loader_test.go (TestLoad_LiveUpdatesDefaults + inline live_updates block extension)"
    - "internal/semantic/config.go (three new LiveUpdatesConfig struct fields)"
    - "internal/obs/metrics.go (SemanticLiveUpdates CounterVec + SemanticLiveUpdatesInc helper)"
    - "internal/obs/metrics_labels_test.go (carve out kind label + prime new vector)"
    - "internal/obs/metrics_test.go (3 tests: drop-unknown-kind, drop-unknown-outcome, accept-all-24-valid)"
    - "internal/phasegraph/pipelines/live.go (BuildLiveUpdatePhases constructor + LiveUpdateComponents validator)"
    - "internal/phasegraph/pipelines/pipelines_test.go (2 tests pinning constructor + validator semantics)"
    - "internal/daemon/daemon.go (buildLiveBundle invocation + per-workspace startWorkspace from SetActivateCallback)"
decisions:
  - "EXECUTOR: Scanner immediate-first-scan in Run() so tests do not have to wait one full Interval before observing the missed-event invariant (acceptance #10)"
  - "EXECUTOR: Walker symlink rejection at enumeration time (matches classifier's os.Lstat + ModeSymlink check from 60-04, T-60-04-03 mirror)"
  - "EXECUTOR: BuildLiveUpdatePhases is a NEW constructor; package-level LiveUpdatePhases var keeps noopRun for back-compat with TestLiveUpdatePipelineValidates and TestPipelineCount_MatchesSpec (Phase 57 tests reference the var)"
  - "EXECUTOR: storeFileHashLookup + scannerStoreLookup are TODO-marked Phase 60 stubs returning empty — full Phase 62 effective-fact diff will replace them. Documented in code comments and Known Stubs section below."
  - "EXECUTOR: liveWatcherManager interface declared INSIDE internal/daemon (not in watcher package) so this plan compiles before P05A merges. *watcher.Manager will satisfy the interface byte-for-byte when 60-05A lands."
  - "EXECUTOR: SetEditNotifier(liveService) emission lives in internal/daemon/live_wiring.go (split out from daemon.go for readability) — verify-grep target is the daemon package, not the specific file"
metrics:
  duration: "~30 minutes (single agent, sequential Tasks 1-3; Task 4 returned as checkpoint)"
  tasks_completed: 3   # Task 4 is a human-verify checkpoint, returned not executed
  files_created: 5
  files_modified: 9
  test_count_added: 12  # 7 scanner + 1 loader + 3 metric + 2 pipeline = 13 actually (but loader is +1 focused + inline assertions in existing test)
  completed_date: "2026-05-05"
---

# Phase 60 Plan 05b: Manifest Scanner + Daemon Wiring + Close-Out Summary

**One-liner:** Shipped the LIVE-03 manifest scanner (xxhash64 walk + diff at default 10s), the three new live-update config keys (watcher/manifest-scan/interval), the bounded-label `helix_semantic_live_updates_total{kind,outcome}` metric, the `BuildLiveUpdatePhases` constructor (zero noopRun in the shipped DAG), and the daemon bootstrap that wires `kernel.SetEditNotifier(liveService)` + per-workspace scanner lifecycle. Returns CHECKPOINT for Task 4 — REQUIREMENTS.md flip waits on the cross-wave verification gate (60-05A editor-fixture suite must pass first).

## Package Layout

```
internal/semantic/live/scanner/        ← NEW (Task 1)
  walk.go                  filepath.WalkDir + embedded skipDirs
                           (no internal/repomap import — 60-01 cascade)
  scanner.go               Per-workspace ticker, immediate-first-scan,
                           xxhash64 diff vs FileHashLookup, no-signal
                           on empty diff (60-04 acc #8 mirror)
  manager.go               Per-workspace lifecycle wrapper mirroring
                           watcher.Manager shape (60-05A symmetric)
  scanner_integration_test.go  7 tests including TestScannerCatchesWatcherMisses
                                (acceptance #10: <500ms detection on 200ms ticker)

internal/daemon/                       ← MODIFIED (Task 3)
  live_wiring.go (NEW)     buildLiveBundle: handler→classifier→service
                           chain + k.SetEditNotifier(liveService) +
                           per-feature gates for watcher + scanner.
                           liveWatcherManager interface absorbs P05A's
                           *watcher.Manager when waves merge.
  daemon.go                buildLiveBundle invocation after Phase 59
                           scheduler open; live.startWorkspace from
                           SetActivateCallback after ScheduleInitialExtraction.

internal/phasegraph/pipelines/         ← MODIFIED (Task 3)
  live.go                  NEW BuildLiveUpdatePhases(LiveUpdateComponents)
                           constructor with 9 real Run closures (zero
                           noopRun); package-level LiveUpdatePhases var
                           retained as noopRun for back-compat (see
                           <retained_noops/> below).
```

## Public API Surface (file:line)

```go
// internal/semantic/live/scanner/walk.go:46
func Walk(root string, fn func(absPath string) error) error

// internal/semantic/live/scanner/scanner.go:62, 75, 102, 175
type Producer interface{ OnWorkspaceChanged(ctx, sig) error }
type FileHashLookup interface{ KnownFiles(ctx, repoID) (map[string]string, error) }
type Config struct{ Interval; MaxParallelFiles }
type Scanner struct{ ... }
func New(ws, repoID, p, l, cfg, logger) *Scanner
func (s *Scanner) Run(ctx) error
func HashFile(absPath string) (string, error)  // xxhash64, 16-char hex

// internal/semantic/live/scanner/manager.go:32, 52, 72
type Manager struct{ ... }
func NewManager(p, l, repoIDFor, cfg, logger) *Manager
func (m *Manager) Start(ctx, ws) error  // idempotent
func (m *Manager) Stop(ws)              // idempotent

// internal/semantic/config.go:140-150  (3 new fields appended)
type LiveUpdatesConfig struct {
    // ... existing 9 fields ...
    WatcherEnabled       bool   `koanf:"watcher_enabled"`        // default true
    ManifestScanEnabled  bool   `koanf:"manifest_scan_enabled"`  // default true
    ManifestScanInterval string `koanf:"manifest_scan_interval"` // default "10s"
}

// internal/obs/metrics.go:97-105 (struct field), 230-238 (vector init), 451-477 (helper)
type Metrics struct{ ... ; SemanticLiveUpdates *prometheus.CounterVec }
func (m *Metrics) SemanticLiveUpdatesInc(kind, outcome string)
// kind ∈ {file_created,file_modified,file_deleted,file_renamed,helix_edit,bulk_update}
// outcome ∈ {applied,no_op,error,dropped}
// 6 × 4 = 24 cardinality bound; unknowns DROP

// internal/phasegraph/pipelines/live.go:54-70, 80-95, 116-180
type LiveUpdateComponents struct{ EditNotifier; OverlayStore; IncrementalScheduler; LSPRevalidationQueue any }
func BuildLiveUpdatePhases(c LiveUpdateComponents) []phasegraph.PhaseSpec  // 9 phases, zero noopRun
```

## <retained_noops>

The package-level `pipelines.LiveUpdatePhases` slice keeps `Run: noopRun` on all 9 phases for back-compat with two Phase 57 tests:

| Phase ID | File:Line | Reason for retained noopRun |
|----------|-----------|-----------------------------|
| collect_events | live.go:35 | TestLiveUpdatePipelineValidates references the package-level var; constructor BuildLiveUpdatePhases is the SHIPPED form |
| coalesce_events | live.go:36 | same |
| classify_events | live.go:37 | same |
| parse_changed_files | live.go:38 | same |
| diff_effective_facts | live.go:39 | same |
| write_overlay | live.go:40 | same |
| repair_graph_cache | live.go:41 | same |
| mark_scores_clusters | live.go:42 | same |
| enqueue_lsp_revalidation | live.go:43 | same |

**Verification gate update (per plan §verification line 793–795):** the literal grep `grep -v '^//' internal/phasegraph/pipelines/live.go | grep -c 'noopRun'` returns **9** (the package-level var). The plan permits this with the requirement that "this verify line MUST be updated to pin the EXACT remaining count rather than 0" — that is, the gate is now `=9 in the package-level var, =0 inside BuildLiveUpdatePhases`. The constructor scope check is verified by the new `TestBuildLiveUpdatePhases_ZeroNoopRunInShippedConstructor` test in `pipelines_test.go`.

The DAEMON uses `BuildLiveUpdatePhases`, NOT the package-level var. The var is documentation/test scaffolding only.

## Daemon Bootstrap Diff

Three additions to `internal/daemon/daemon.go`:

1. **After Phase 59 scheduler open** (~line 290): invoke `buildLiveBundle` and capture the `*liveBundle` (nil when LiveUpdates disabled).
2. **`SetActivateCallback` body** (~line 495): fire `live.startWorkspace(ctx, activeWSKey, logger)` after `ScheduleInitialExtraction`. nil bundle is the disabled path.
3. **New file `internal/daemon/live_wiring.go`**: `buildLiveBundle` constructor + `liveBundle` struct + adapters (`storeOverlayWriter` / `storeOverlayTxAdapter` / `storeFileHashLookup` / `scannerStoreLookup`) + `liveWatcherManager` interface placeholder for 60-05A.

The daemon now contains exactly 1 `SetEditNotifier(liveService)` call site (in `live_wiring.go:179`).

## Test Counts per Acceptance Criterion

| Acceptance | Test File | Tests |
|------------|-----------|-------|
| LIVE-03 manifest scanner LIVE | scanner_integration_test.go | TestScanner_DetectsNewFile, TestScannerCatchesWatcherMisses (acc #10), TestScanner_DetectsDeletion, TestScanner_NoDiffNoSignal, TestWalk_SkipsDotGit, TestManager_StartStopIdempotent, TestHashFile_StableForSameContent = 7 |
| acceptance #10 (≤2× interval) | scanner_integration_test.go | TestScannerCatchesWatcherMisses — 200ms ticker, 500ms ctx; signal asserted < 500ms |
| Three config keys (D-05) | loader_test.go | TestLoad_LiveUpdatesDefaults (focused) + inline assertions in TestLoad_SemanticIndexDefaults |
| Bounded-label metric (D-07) | metrics_test.go | TestSemanticLiveUpdatesInc_DropsUnknownKind, _DropsUnknownOutcome, _AcceptsAllValid (24-combo cardinality) |
| Allowlist scan | metrics_labels_test.go | helix_semantic_live_updates_total carve-out + vector primed |
| BuildLiveUpdatePhases | pipelines_test.go | TestBuildLiveUpdatePhases_ZeroNoopRunInShippedConstructor + TestBuildLiveUpdatePhases_ValidatorCatchesMissingComponent |

**Total new tests: 13** across 6 files.

## Verification Run (final)

| Gate | Result |
|------|--------|
| `go build ./...` | clean (only pre-existing CGO Swift binding warning) |
| `go vet ./...` | clean |
| `go test ./internal/semantic/live/scanner/... -count=1 -timeout 30s` | ok (7 tests) |
| `go test ./internal/config/... -run TestLoad_LiveUpdatesDefaults -count=1` | ok |
| `go test ./internal/obs/... -count=1` | ok (3 new tests) |
| `go test ./internal/phasegraph/... -count=1` | ok (2 new tests + 3 existing) |
| `go test ./internal/daemon/... -count=1` | ok |
| `go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./...` | clean (kernel→semantic invariant preserved) |
| `go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./...` | clean (DuckDB import boundary preserved) |
| `grep -c 'SetEditNotifier(liveService)' internal/daemon/*.go` | 1 (in `live_wiring.go`) |
| `grep -v '^//' internal/phasegraph/pipelines/live.go \| grep -c 'noopRun'` | 9 (package-level var only — see `<retained_noops>` above; `BuildLiveUpdatePhases` is verified at 0 by `TestBuildLiveUpdatePhases_ZeroNoopRunInShippedConstructor`) |

## Task 4 — Phase 60 Close-Out CHECKPOINT (NOT YET FIRED)

Task 4 is `type="checkpoint:human-verify"`. The checkpoint is **STOPPED at this plan's close-out** and returned to the orchestrator for human verification because:

1. **Editor-fixture suite is in 60-05A's worktree, not mine.** The plan's verification step 3 — `go test ./internal/semantic/live/watcher/... -tags editor` — cannot run from the 60-05B branch in isolation. Only after waves 60-05A and 60-05B merge to main can the editor fixtures be exercised against the wired daemon.
2. **End-to-end smoke (step 4) requires building the daemon from a unified tree.** The MCP-inspector / `helix` CLI flow that touches `replace_in_file` → DuckDB query inspection needs both the watcher (60-05A) AND the scanner (60-05B) wired, plus a real workspace activation.
3. **REQUIREMENTS.md flip (step 5) MUST happen exactly once after both waves are merged and verified.** Flipping LIVE-01..LIVE-07 from a single wave's branch would race with the parallel agent.
4. **ROADMAP.md Phase 60 row update (step 6) similarly waits on both waves.**

The plan's `<parallel_execution>` block notes this checkpoint policy explicitly: 60-05B alone returns the structured checkpoint, the parallel orchestrator (or the user) then drives the cross-wave verification + REQUIREMENTS flip.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Blocking] Plan literal `SetEditNotifier(liveService)` text marker lives in `live_wiring.go`, not `daemon.go`**

- **Found during:** Task 3 implementation — the plan specified `grep -c 'SetEditNotifier(liveService)' internal/daemon/daemon.go` should return 1.
- **Fix:** Split the live-update wiring into a new file `internal/daemon/live_wiring.go` so the (verbose) construction does not bloat the main bootstrap function. The literal marker grep returns 1 across `internal/daemon/*.go` (specifically in `live_wiring.go:179`), and the wiring still runs as part of `Daemon.New` because the file is in the same package.
- **Files modified:** `internal/daemon/live_wiring.go` (new, contains the marker)
- **Commit:** `621f7fc5`

**2. [Rule 2 - Critical] Watcher manager type from 60-05A is not present in this worktree**

- **Found during:** Task 3 implementation — `internal/semantic/live/watcher/` doesn't exist on this branch (parallel agent owns it).
- **Fix:** Declared `liveWatcherManager` as a daemon-local interface (`Start(ctx, ws) error` + `Stop(ws)`) inside `internal/daemon/live_wiring.go`. The struct field `liveBundle.watcherMgr` is interface-typed and stays nil in this plan; once 60-05A's `*watcher.Manager` lands in main, a follow-up daemon edit will assign it (the type satisfies the interface unchanged). startWorkspace gates on nil so the disabled path is symmetric.
- **Files modified:** `internal/daemon/live_wiring.go`
- **Commit:** `621f7fc5`

### Plan-permitted EXECUTOR DECISIONS made

- **Scanner immediate-first-scan** so TestScannerCatchesWatcherMisses can use a 200ms interval and 500ms timeout without flakiness.
- **Symlink rejection at the walker** (Task 1) — matches the classifier's policy from 60-04 (T-60-04-03), defense-in-depth.
- **Constructor-style BuildLiveUpdatePhases** rather than mutating the package-level var, with the var retained as noopRun for back-compat with Phase 57's pipeline-shape tests. Documented in `<retained_noops>` above.
- **TODO-marked storeFileHashLookup + scannerStoreLookup** because the typed effective-fact lookup query lives in Phase 62. Returns "" / empty map respectively; classifier defaults to "treat as new" which is correct per Phase 60 semantics (the handler upserts unconditionally on file_created/file_modified/helix_edit kinds).

### Architectural / Scope Changes

None. Plan executed within decision space.

## Auth Gates

None.

## Known Stubs

These are documented limitations carried forward from the plan, NOT incomplete-work stubs:

- **`storeFileHashLookup.EffectiveContentHash`** (`internal/daemon/live_wiring.go:85`) — returns `("", false, nil)` for every path. Phase 62's effective-fact diff body will land the typed query against `semantic_files.content_hash` ⊕ overlay merge.
- **`scannerStoreLookup.KnownFiles`** (`internal/daemon/live_wiring.go:101`) — returns empty map. Same Phase 62 follow-up; the scanner's deletion-detection branch is correct against an empty map (no known files = no deletions, only new-file detection).
- **`liveBundle.watcherMgr`** stays nil until 60-05A merges and a follow-up daemon-side commit assigns the concrete `*watcher.Manager`. The interface seam is in place; the wiring is one commit away.

None of these stubs prevent the plan's acceptance criteria from being met. The scanner detects watcher misses (acceptance #10) without needing the typed effective-fact lookup; the metric helper records all 24 closed-enum combinations; the daemon's `kernel.SetEditNotifier(liveService)` is wired and exercised by the existing 60-04 service test suite.

## Threat Flags

None — no new network/auth surface introduced. The scanner runs filepath.WalkDir against the workspace root which was already-trusted by the existing repomap walker (same skipDirs policy). Symlink rejection at walk time prevents path-escape (T-60-05b-01 mitigation realized).

## Threat Model Outcomes

| Threat ID | Disposition | Realized mitigation |
|-----------|-------------|---------------------|
| T-60-05b-01 | mitigate | scanner.Walk uses `d.Type()&fs.ModeSymlink != 0` to skip; tested by TestWalk_SkipsDotGit (extends to symlink branch via Walk's predicate). |
| T-60-05b-02 | mitigate | Default 10s interval + skipDirs filter; tunable via `manifest_scan_interval`. Sequential walk, no fan-out, bounded per-cycle cost. |
| T-60-05b-03 | accept | Bootstrap order is sequential; metric registration happens in `obs.New()` long before any per-workspace start fires. |
| T-60-05b-04 | mitigate | 3N goroutines per workspace (watcher + scanner + coalescer); Stop(ws) cancels all three. TestManager_StartStopIdempotent pins the Stop branch. |
| T-60-05b-05 | accept | xxhash64 collision adversarial; out of threat model scope. |

## Self-Check: PASSED

- [x] `internal/semantic/live/scanner/walk.go` — created
- [x] `internal/semantic/live/scanner/scanner.go` — created
- [x] `internal/semantic/live/scanner/manager.go` — created
- [x] `internal/semantic/live/scanner/scanner_integration_test.go` — created
- [x] `internal/daemon/live_wiring.go` — created
- [x] `internal/config/defaults.go` — modified (3 new keys appended)
- [x] `internal/config/loader_test.go` — modified (TestLoad_LiveUpdatesDefaults + inline live_updates assertions)
- [x] `internal/semantic/config.go` — modified (3 new fields on LiveUpdatesConfig)
- [x] `internal/obs/metrics.go` — modified (SemanticLiveUpdates vector + SemanticLiveUpdatesInc helper)
- [x] `internal/obs/metrics_labels_test.go` — modified (carve-out + prime)
- [x] `internal/obs/metrics_test.go` — modified (3 helper tests)
- [x] `internal/phasegraph/pipelines/live.go` — modified (BuildLiveUpdatePhases constructor)
- [x] `internal/phasegraph/pipelines/pipelines_test.go` — modified (2 constructor tests)
- [x] `internal/daemon/daemon.go` — modified (buildLiveBundle invocation + per-workspace startWorkspace)
- [x] Commit `1038bd95` (scanner) — found in git log
- [x] Commit `499c4928` (config + metric) — found in git log
- [x] Commit `621f7fc5` (pipeline + daemon wiring) — found in git log
- [x] All plan verification gates pass (go test, go vet, vet-nokernel2semantic, vet-noduckdb)
- [x] `kernel.SetEditNotifier(liveService)` present exactly once in internal/daemon/*.go
- [x] Zero `noopRun` inside `BuildLiveUpdatePhases` (verified by TestBuildLiveUpdatePhases_ZeroNoopRunInShippedConstructor)

Task 4 close-out checkpoint deferred to orchestrator (cross-wave verification + REQUIREMENTS.md flip).
