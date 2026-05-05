---
phase: 60-live-update-pipeline
verified: 2026-05-05T18:00:00Z
status: passed
score: 33/33 must-haves verified
overrides_applied: 0
---

# Phase 60: Live Update Pipeline Verification Report

**Phase Goal:** Ship the live-update pipeline (LIVE-01..LIVE-07) end-to-end — fsnotify watcher with debounce + coalescing + ENOSPC fallback, editor-fixture coverage for Vim/JetBrains/VS Code save patterns, periodic manifest scanner, monotonic overlay_epoch contract under -race, and a kernel→semantic EditNotifier seam that emits ChangeHelixEdit after every successful in-place edit. The kernel⊥semantic boundary is enforced by a vet analyzer.

**Verified:** 2026-05-05
**Status:** passed
**Re-verification:** No — initial verification (post-review-fix)

## Goal Achievement

### ROADMAP Success Criteria (the contract)

| #   | Roadmap Success Criterion | Status | Evidence |
| --- | ------------------------- | ------ | -------- |
| SC1 | fsnotify watcher catches Vim, JetBrains (`___jb_tmp___`+rename), VS Code atomic-rename — verified by editor-fixture test | VERIFIED | `internal/semantic/live/watcher/editor_fixtures_test.go` lines 160 (VimSwapRename), 189 (JetBrainsSafeWrite), 222/248 (VSCodeAtomic + VSCodeTruncate). All four tests pass; jbTempSuffix filter at `watcher.go:26` |
| SC2 | Required (not best-effort) periodic content-hash scrub catches missed events; status surfaces via `get_semantic_graph_status` | VERIFIED (data path) | `scanner/scanner.go:106` `time.NewTicker(s.cfg.Interval)`; default=10s; `WatcherStatus` accessor at `watcher/status.go:39`. `get_semantic_graph_status` MCP tool itself is a Phase 64 deliverable (per ROADMAP) — Phase 60 only owes the data accessor, which exists |
| SC3 | Linux `inotify` `ENOSPC` falls back to manifest-poll mode per workspace with user-visible warning; semantic queries still answer with stale-allowed reads | VERIFIED | `watcher/enospc.go:25` `errors.Is(err, syscall.ENOSPC)`; sync.Once-guarded `slog.Warn`; `Status().Reason="inotify_enospc"` (`enospc_test.go:106`); manifest scanner default-on |
| SC4 | After every successful `replace_symbol_body`, `insert_before/after_symbol`, `rename_symbol`, `safe_delete_symbol`, `replace_in_file`, and `fuzzy_edit`, a `ChangeHelixEdit` event lands in the live queue via `postEditHook` (kernel does not import semantic) | VERIFIED | 5 edit-tool sites at `kernel/edit/tools.go:{383,452,521,588,670}` + 4 fileops sites at `kernel/fileops/tools.go:{258,440,456,510}` (covers create_file + replace_in_file exact + replace_in_file fuzzy fallback + fuzzy_edit). `vet-nokernel2semantic` reports zero kernel→semantic imports |
| SC5 | Overlay writes carry monotonic `overlay_epoch`; bulk-change events above `bulk_change_threshold` (default 200) collapse to single `bulk_update` event | VERIFIED | `overlay.go:119` `current_epoch = current_epoch + 1`; `TestOverlayEpochConcurrent` passes under `-race` (verified live: `go test -race ./internal/semantic/store/... -run TestOverlayEpochConcurrent` → ok 2.082s); `coalesce.go:42` `CoalesceEvents` collapses to `ChangeBulkUpdate` at threshold |

### Plan-Level Observable Truths (per PLAN frontmatter)

#### Plan 60-01 (vet-nokernel2semantic)

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 1   | `go vet -vettool=...vet-nokernel2semantic ./...` reports zero errors and fires on a kernel→semantic import | VERIFIED | Live run on full tree: clean. Analyzer at `internal/lint/nokernel2semantic/analyzer.go:38`; `analysistest.Run` in `analyzer_test.go` |
| 2   | Makefile vet target runs the new analyzer alongside vet-noduckdb | VERIFIED | `Makefile:21,31-32` declare `VETTOOL_NOKERNEL2SEMANTIC` and an install target |

#### Plan 60-02 (schema v3 + overlay)

| #   | Truth | Status | Evidence |
| --- | ----- | ------ | -------- |
| 3   | Schema v2→v3 migration adds current_epoch + 4× write_epoch + 4 indexes; CurrentSchemaVersion=3 | VERIFIED | `migrations_types.go:17` `CurrentSchemaVersion = 3`; `migrations_registry.go:30` `{From: 2, To: 3, Apply: applyMigration003}`; `migrations.go:470` `applyMigration003` |
| 4   | `BeginOverlayTx` atomically increments per-workspace current_epoch and stamps every fact row with write_epoch | VERIFIED | `overlay.go:79` `BeginOverlayTx`; `overlay.go:119` SQL `current_epoch = current_epoch + 1`; UpsertOverlayFile stamps tx.epoch |
| 5   | Concurrent `BeginOverlayTx` on SAME workspace serialize and each receives unique monotone write_epoch under -race with N=64 | VERIFIED | `TestOverlayEpochConcurrent` passes under `-race` (live re-run ok 2.082s) |
| 6   | Concurrent `BeginOverlayTx` on DIFFERENT workspaces do not serialize | VERIFIED | Per-workspace mutex registry in `overlay.go:62-146` |
| 7   | Tx Rollback does NOT rewind current_epoch (D-04 invariant) | VERIFIED | Bump uses `s.db` not `tx`; covered by overlay_test.go |
| 8   | OverlayTx surfaces UpsertOverlayFile + MarkFileDeleted/MarkSymbolsDeleted/MarkReferencesDeleted/MarkEdgesDeleted | VERIFIED | All 5 methods declared in `overlay.go` (grep confirms) |
| 9   | Migration succeeds clean v0→v3, v1→v3, v2→v3 | VERIFIED | `migrations_test.go` covers all three paths |

#### Plan 60-03 (EditNotifier seam)

| #    | Truth | Status | Evidence |
| ---- | ----- | ------ | -------- |
| 10   | `internal/kernel/notifier.go` declares public EditNotifier interface `(ctx, workspace.WorkspaceKey, []string) error` | VERIFIED | `notifier.go:30` |
| 11   | Kernel exposes SetEditNotifier + EditNotifier accessor (atomic.Value, nil-safe) | VERIFIED | `notifier.go:40,53`; `kernel.go:36` field |
| 12   | `internal/kernel/*` does NOT import `internal/semantic/*` (vet-nokernel2semantic green) | VERIFIED | Live vet run clean |
| 13   | 8 kernel edit/fileops tools call `k.EditNotifier().OnEdit(...)` on success | VERIFIED | 9 OnEdit call sites total (5 edit + 4 fileops; replace_in_file has exact-path + fuzzy-fallback paths to cover both branches) |
| 14   | OnEdit returns in O(microseconds) — TestOnEditNonBlocking | VERIFIED | `kernel/edit/notifier_integration_test.go` contains TestOnEditNonBlocking |
| 15   | OnEdit error swallowed at every call site | VERIFIED | Every site uses `_ = n.OnEdit(...)` discard form |

#### Plan 60-04 (live spine)

| #    | Truth | Status | Evidence |
| ---- | ----- | ------ | -------- |
| 16   | WorkspaceChangeSignal is paths-only; ChangeSource closed-enum {helix_edit, fsnotify, manifest_scan} | VERIFIED | `signal.go:30,38,41,45,65` |
| 17   | ClassifyPathChange is the SINGLE site deciding SourceChangeKind | VERIFIED | `classifier.go:51` |
| 18   | CoalesceEvents pure function; covers 6 SPEC §16.2 merge rules + threshold collapse | VERIFIED | `coalesce.go:42` + tests in `coalesce_test.go` and `bulk_test.go` |
| 19   | Coalescer is one goroutine per workspace; debounce+max-batch-delay; sequential dispatch; per-event errors do not abort batch | VERIFIED | `coalescer.go:80,225-237` |
| 20   | Empty coalesced batches do NOT advance overlay_epoch (no BeginOverlayTx call) | VERIFIED | Empty short-circuit in `coalescer.go:202-204` |
| 21   | liveService.OnEdit implements kernel.EditNotifier; non-blocking enqueue | VERIFIED | `service/service.go:121` |
| 22   | ScheduleIncremental fills Phase 59 stub; dispatches per FileChange.Kind | VERIFIED | `scheduler.go:124`; `scheduler_incremental_test.go` covers per-kind dispatch |
| 23   | LSPQueue typed buffered channel; producer-only in P60 | VERIFIED | `lspqueue/queue.go:24,30,42`; consumer left for Phase 61 |

#### Plan 60-05A (watcher + ENOSPC + editor fixtures)

| #    | Truth | Status | Evidence |
| ---- | ----- | ------ | -------- |
| 24   | Per-workspace fsnotify watcher catches Vim swap-rename, JetBrains tempfile+rename, VS Code atomic+truncate | VERIFIED | 4 editor-fixture tests (vim, jetbrains, vscode atomic, vscode truncate) all pass |
| 25   | Linux ENOSPC at fsnotify.Add triggers exactly one structured slog.Warn; Status returns Active=false, Reason=inotify_enospc | VERIFIED | `enospc_test.go:106-107` asserts Reason="inotify_enospc"; sync.Once gate in enospc.go |
| 26   | Atomic-rename save patterns from Vim, JetBrains, and VS Code do not silently kill watching (LIVE-02) | VERIFIED | Editor fixtures pass; jbTempSuffix filter at `watcher.go:26`; CR-02 fix added symlink rejection (`watcher.go:121,240`) |
| 27   | JetBrains tempfile suffix filter (`___jb_tmp___`, `___jb_old___`) drops noise at watcher loop | VERIFIED | `watcher.go:26` `jbTempSuffix = "___jb_tmp___"`; `editor_fixtures_test.go:206-213` regression-asserts no leak |

#### Plan 60-05B (scanner + config + metric + pipelines + daemon wiring + close-out)

| #    | Truth | Status | Evidence |
| ---- | ----- | ------ | -------- |
| 28   | Manifest scanner walks workspace every manifest_scan_interval (10s default), hashes via xxhash64, emits WorkspaceChangeSignal{manifest_scan} on mismatch (LIVE-03) | VERIFIED | `scanner/scanner.go:106` ticker; `scanner/walk.go:51` WalkDir; `scanner_integration_test.go` passes |
| 29   | Manifest scanner detects watcher misses within 2× interval | VERIFIED | `scanner_integration_test.go` contains the integration assertion (test passes) |
| 30   | Three new config keys (watcher_enabled / manifest_scan_enabled / manifest_scan_interval) flow through 4-layer koanf | VERIFIED | `defaults.go:94-96`; `semantic/config.go:151,154,158`; `loader_test.go` covers them |
| 31   | helix_semantic_live_updates_total{kind, outcome} bounded labels via closed-enum drop-on-unknown helper | VERIFIED | `obs/metrics.go:438,453` `SemanticLiveUpdatesInc`; **CR-01 fix wired**: emitted at coalescer drop (`coalescer.go:138`), apply (`coalescer.go:236`), error (`coalescer.go:231`) |
| 32   | Daemon bootstrap: kernel.SetEditNotifier(liveService); watcher+scanner Start per workspace activation | VERIFIED | `live_wiring.go:194` `k.SetEditNotifier(liveService)`; `live_wiring.go:60-69` SetActivateCallback Start gate |
| 33   | pipelines/live.go: zero noopRun bodies in shipped constructor (every Run is a real closure) | VERIFIED | `BuildLiveUpdatePhases` uses `mkValidator` for all 9 phases (`live.go:128-181`); not noopRun |

### Required Artifacts

| Artifact                                                       | Expected                                              | Status     | Details |
| -------------------------------------------------------------- | ----------------------------------------------------- | ---------- | ------- |
| `internal/lint/nokernel2semantic/analyzer.go`                  | go/analysis Analyzer                                  | VERIFIED   | `var Analyzer` declared, tests pass |
| `cmd/vet-nokernel2semantic/main.go`                            | singlechecker entry point                             | VERIFIED   | `singlechecker.Main(...)` |
| `internal/semantic/store/migrations.go`                        | applyMigration003 + schema3Statements                 | VERIFIED   | `applyMigration003` at line 470 |
| `internal/semantic/store/migrations_registry.go`               | v2→v3 entry                                           | VERIFIED   | `{From: 2, To: 3, ...}` |
| `internal/semantic/store/migrations_types.go`                  | CurrentSchemaVersion=3                                | VERIFIED   | line 17 |
| `internal/semantic/store/overlay.go`                           | BeginOverlayTx + tombstone helpers + UpsertOverlayFile | VERIFIED  | All present |
| `internal/semantic/store/overlay_concurrent_test.go`           | TestOverlayEpochConcurrent                            | VERIFIED   | Passes under -race |
| `internal/kernel/notifier.go`                                  | EditNotifier interface + Kernel methods               | VERIFIED   | line 30 |
| `internal/kernel/notifier_test.go`                             | Kernel set/get notifier roundtrip                     | VERIFIED   | exists |
| `internal/kernel/edit/notifier_integration_test.go`            | TestOnEditCalledOnSuccess + NonBlocking               | VERIFIED   | exists |
| `internal/kernel/fileops/notifier_integration_test.go`         | OnEditCalledOnSuccess for 3 fileops tools             | VERIFIED   | exists |
| `internal/semantic/live/signal.go`                             | WorkspaceChangeSignal + ChangeSource enum + SourceChangeEvent | VERIFIED | All types declared |
| `internal/semantic/live/classifier.go`                         | ClassifyPathChange                                    | VERIFIED   | line 51 |
| `internal/semantic/live/coalescer/coalesce.go`                 | CoalesceEvents (pure) + MergeChange                   | VERIFIED   | lines 42, 88 |
| `internal/semantic/live/coalescer/coalescer.go`                | Coalescer struct, per-workspace single-goroutine     | VERIFIED   | line 80 |
| `internal/semantic/live/handler/handler.go`                    | UpdateChangedFile, HandleFileDeleted, HandleFileRenamed, HandleBulkUpdate | VERIFIED | lines 128, 155, 170, 185 |
| `internal/semantic/live/service/service.go`                    | liveService.OnEdit implementing kernel.EditNotifier   | VERIFIED   | line 121 |
| `internal/semantic/live/watcher/watcher.go`                    | Per-workspace fsnotify watcher                        | VERIFIED   | `fsnotify.NewWatcher` |
| `internal/semantic/live/watcher/enospc.go`                     | IsENOSPC + sync.Once Warn                             | VERIFIED   | line 25 |
| `internal/semantic/live/watcher/status.go`                     | WatcherStatus + atomicStatus                          | VERIFIED   | line 39 |
| `internal/semantic/live/testdata/editors/{vim,jetbrains,vscode}/*` | Editor fixtures                                   | VERIFIED   | save.sh, save.go, save_atomic.go, save_truncate.go all present |
| `internal/semantic/live/scanner/scanner.go`                    | Per-workspace manifest scanner ticker                 | VERIFIED   | `time.NewTicker` |
| `internal/semantic/live/scanner/walk.go`                       | filepath.WalkDir with embedded skipDirs               | VERIFIED   | line 51 |
| `internal/obs/metrics.go`                                      | SemanticLiveUpdatesInc helper                         | VERIFIED   | line 438; **wired in coalescer post-CR-01** |
| `internal/phasegraph/pipelines/live.go`                        | BuildLiveUpdatePhases — zero noopRun                  | VERIFIED   | All 9 phases use `mkValidator` |
| `internal/daemon/daemon.go` + `live_wiring.go`                 | Live-update wiring + SetEditNotifier + Start gates    | VERIFIED   | `live_wiring.go:194` SetEditNotifier; `live_wiring.go:237` BuildLiveUpdatePhases invocation (CR-03 fix) |
| `internal/daemon/live_e2e_test.go`                             | TestLiveUpdate_E2E_OverlayEpochAdvancesOnEdit         | VERIFIED   | Passes; asserts current_epoch advance + write_epoch stamp + lspqueue.Len()==1 |

### Key Link Verification

| From                                         | To                                                  | Via                                  | Status   | Details |
| -------------------------------------------- | --------------------------------------------------- | ------------------------------------ | -------- | ------- |
| Makefile vet target                          | cmd/vet-nokernel2semantic                           | `go vet -vettool=...`                | WIRED    | Makefile:21,31-32 + live vet run clean |
| `overlay.go:BeginOverlayTx`                  | `semantic_live_overlay_meta.current_epoch`          | `current_epoch = current_epoch + 1` SQL | WIRED | `overlay.go:119` |
| `overlay.go:OverlayTx.UpsertOverlay*`        | `semantic_live_overlay_*.write_epoch`               | INSERT/UPDATE stamps tx.epoch        | WIRED    | UpsertOverlayFile body |
| `kernel/edit/*.go` and `kernel/fileops/*.go` success paths | `k.EditNotifier().OnEdit`                | fire-and-forget after edit returns nil | WIRED  | 9 call sites (covers 8 tools, replace_in_file has 2 paths) |
| `service/service.go:OnEdit`                  | per-workspace coalescer                             | non-blocking channel send (select+default) | WIRED | `service.go:121-141` |
| `coalescer.go:flush()`                       | handler dispatch                                    | switch on SourceChangeKind           | WIRED    | `coalescer.go:225-237` |
| `scheduler.go:ScheduleIncremental`           | handler.UpdateChangedFile / handler.HandleFileDeleted | per-FileChange dispatch            | WIRED    | `scheduler.go:124` |
| `daemon.go:SetActivateCallback`              | watcherMgr.Start + scannerMgr.Start                 | gated on cfg.SemanticIndex.LiveUpdates.{Enabled,WatcherEnabled,ManifestScanEnabled} | WIRED | `live_wiring.go:60-69, 208-218` |
| `daemon/live_wiring.go` bootstrap            | `kernel.SetEditNotifier(liveService)`               | post Phase 59 scheduler open         | WIRED    | `live_wiring.go:194` |
| `watcher.go:fsnotify.Errors`                 | enospc.go one-shot Warn                             | errors.Is + sync.Once                | WIRED    | `enospc.go:25`; enospc_test.go pins behavior |
| `editor_fixtures_test.go`                    | testdata/editors/{vim,jetbrains,vscode}/save*       | os/exec under t.TempDir against live watcher | WIRED | tests pass |
| **CR-01:** coalescer drop/apply/error sites  | `metrics.SemanticLiveUpdatesInc`                    | MetricsSink interface                | WIRED    | `coalescer.go:138,231,236` + 3 regression tests in `metrics_emission_test.go` |
| **CR-02:** watcher addRecursive + handleEvent | symlink rejection (`fs.ModeSymlink` + `os.Lstat`)  | scanner-mirroring guard              | WIRED    | `watcher.go:121,240` + `symlink_test.go` regression test |
| **CR-03:** `live_wiring.go` post-bundle      | `BuildLiveUpdatePhases` → ValidatePhaseGraph + RunPhaseGraph | runtime invocation              | WIRED    | `live_wiring.go:237-245` |
| **CR-04:** handler.UpdateChangedFile success | `LSPQueue.Enqueue(RevalidateFileJob)`               | producer side of Phase 60 P04        | WIRED    | `handler.go:144-145` + `live_e2e_test.go:176` Len==1 assertion |

### Data-Flow Trace (Level 4)

| Artifact | Data Variable | Source | Produces Real Data | Status |
| -------- | ------------- | ------ | ------------------ | ------ |
| `live_e2e_test.go` end-to-end | `current_epoch` row | `BeginOverlayTx` SQL bump | YES (test asserts post>pre) | FLOWING |
| `live_e2e_test.go` end-to-end | `semantic_live_overlay_files.write_epoch` | UpsertOverlayFile in tx | YES (1 ≤ write_epoch ≤ current_epoch) | FLOWING |
| `live_e2e_test.go` end-to-end | `bundle.lspQueue` | handler.LSPQueue.Enqueue post-commit | YES (Len()==1 after one OnEdit) | FLOWING |
| Coalescer metric emission | `helix_semantic_live_updates_total{outcome=applied}` | `c.metrics.SemanticLiveUpdatesInc` post-Dispatch | YES (regression test pins increment) | FLOWING |
| Watcher Status | `WatcherStatus.Reason` | `atomicStatus` Store on ENOSPC | YES (`enospc_test.go:106` asserts) | FLOWING |
| `storeFileHashLookup.EffectiveContentHash` (live_wiring.go:88) | Always returns ("", false, nil) | TODO(60-D-05) — Phase 62 fills | NO (intentional Phase 62 stub, documented in 60-05b-SUMMARY) | STATIC (deferred) |
| `scannerStoreLookup.KnownFiles` (live_wiring.go:104) | Always returns empty map | TODO(60-D-05) — Phase 62 fills | NO (intentional Phase 62 stub, documented in 60-05b-SUMMARY) | STATIC (deferred) |

The two STATIC entries are intentional Phase 62 stubs that were explicitly carved out by the plan's domain decisions and documented in 60-05b-SUMMARY.md and the 60-REVIEW.md IN-05/IN-06 info items. They are not Phase 60 gaps — they are forward-deferred classifier inputs that downgrade the live-update producer surface (every fsnotify event will surface as `file_created` rather than `file_modified` until Phase 62 ships the typed effective-hash query). Phase 60's contract is the pipeline shape and the epoch monotonicity, both of which are met.

### Behavioral Spot-Checks

| Behavior | Command | Result | Status |
| -------- | ------- | ------ | ------ |
| nokernel2semantic analyzer reports zero on full tree | `go install ./cmd/vet-nokernel2semantic && go vet -vettool=...` | clean | PASS |
| Lint analyzer + store + kernel tests all pass | `go test ./internal/lint/... ./internal/semantic/store/... ./internal/kernel/...` | all ok | PASS |
| Live pipeline tests all pass | `go test ./internal/semantic/live/...` | 7 packages ok | PASS |
| Daemon E2E asserts epoch advance + write_epoch stamp + lspqueue Len==1 | `go test ./internal/daemon/... -run TestLive` | ok 1.216s | PASS |
| Race-tagged epoch contract under -race | `go test -race ./internal/semantic/store/... -run TestOverlayEpochConcurrent` | ok 2.082s | PASS |
| Phasegraph + obs tests pass | `go test ./internal/phasegraph/... ./internal/obs/...` | all ok | PASS |
| Full build clean | `go build ./internal/... ./cmd/...` | clean (only benign C #define warning in vendored swift binding) | PASS |

### Requirements Coverage

| Requirement | Source Plan       | Description                                                                 | Status     | Evidence |
| ----------- | ----------------- | --------------------------------------------------------------------------- | ---------- | -------- |
| LIVE-01     | 60-04, 60-05a     | Directory-level fsnotify watcher, debounce 250ms, coalescer per SPEC §16.2, structured upserts/tombstones | SATISFIED | watcher + coalescer tests pass; debounce_ms default 250 in defaults.go |
| LIVE-02     | 60-05a            | Atomic-rename save patterns (Vim, JetBrains, VS Code) do not kill watching, verified by editor-fixture test | SATISFIED | 4 editor fixture tests pass + jbTempSuffix filter |
| LIVE-03     | 60-05b            | Required (not best-effort) periodic content-hash scrub catches missed events; status via get_semantic_graph_status | SATISFIED (Phase 60 portion) | scanner ticker + WatcherStatus accessor; MCP tool itself = Phase 64 |
| LIVE-04     | 60-04, 60-05a     | Linux inotify ENOSPC falls back to manifest-poll mode with user-visible warning; queries answer with stale reads | SATISFIED | enospc.go + sync.Once Warn + Status flip; manifest scanner default-on |
| LIVE-05     | 60-02, 60-04      | Bulk-change events above bulk_change_threshold (default 200) collapse to single bulk_update event | SATISFIED | CoalesceEvents threshold collapse + bulk_test.go |
| LIVE-06     | 60-02             | Overlay write tx carries monotonic overlay_epoch read by COMPACT-01 under CAS | SATISFIED | TestOverlayEpochConcurrent under -race |
| LIVE-07     | 60-01, 60-03, 60-04 | After every successful in-place edit, ChangeHelixEdit lands in live queue via postEditHook (kernel does NOT import semantic) | SATISFIED | 9 OnEdit call sites + vet-nokernel2semantic green + live_e2e_test E2E |

All 7 LIVE-* requirements account for verified Phase 60 evidence. REQUIREMENTS.md was correctly flipped to `[x]` / `Complete` for LIVE-01..LIVE-07.

### Anti-Patterns Found

| File | Line | Pattern | Severity | Impact |
| ---- | ---- | ------- | -------- | ------ |
| `internal/daemon/live_wiring.go` | 88-90 | `EffectiveContentHash` returns `("", false, nil)` with TODO(60-D-05) | Info | Documented Phase 62 deferral (in 60-05b-SUMMARY); collapses ChangeFileModified→ChangeFileCreated in production until Phase 62 |
| `internal/daemon/live_wiring.go` | 104-106 | `KnownFiles` returns empty map with TODO(60-D-05) | Info | Same Phase 62 deferral; scanner emits full-walk "changed" set every cycle until typed hash lookup lands |

Both items are explicit, planned Phase 62 work — not unfinished Phase 60 work. They appear in the prompt's `<known_post_review_fixes>` block as "intentional Phase 62 stubs documented in 60-05b-SUMMARY.md (NOT gaps for Phase 60)". WR-01..WR-07 from 60-REVIEW.md are warnings only, not blockers.

### Human Verification Required

None. Phase 60 ships an internal pipeline with no user-facing UI. All behaviors that could plausibly require human verification (Vim/JetBrains/VSCode save patterns, ENOSPC fallback, watcher restart on rename) are exercised by the editor-fixture and enospc_test integration suites. The end-to-end smoke (`TestLiveUpdate_E2E_OverlayEpochAdvancesOnEdit`) verifies the kernel→semantic→overlay→lspqueue chain in-process.

The MCP-tool surface for live status (`get_semantic_graph_status`) is a Phase 64 deliverable per the ROADMAP — not Phase 60.

### Gaps Summary

No gaps. All four post-review BLOCKERs (CR-01..CR-04) were closed inline in commit 7f775b17:

- **CR-01:** `helix_semantic_live_updates_total{kind, outcome}` is now emitted at every drop/apply/error site in `coalescer.go:138,231,236` via the new `MetricsSink` interface; 3 regression tests in `metrics_emission_test.go` pin the behavior. The previous `_ = metrics` discard at `live_wiring.go:209` is replaced by real `Metrics` plumbing.
- **CR-02:** Watcher rejects symlinks in both `addRecursive` (`watcher.go:121` `d.Type()&fs.ModeSymlink != 0`) and `handleEvent` (`watcher.go:240` `os.Lstat` + `info.Mode()&os.ModeSymlink == 0`); regression test in `symlink_test.go`.
- **CR-03:** `BuildLiveUpdatePhases` is now invoked from `buildLiveBundle` via `phasegraph.ValidatePhaseGraph` + `RunPhaseGraph` at `live_wiring.go:237-245`; failure logs `slog.Warn` (does not abort the daemon — a deliberate D-07 policy choice documented inline) so a future regression dropping `SetEditNotifier` is observable in logs.
- **CR-04:** `handler.LSPQueue` field added (`handler.go:85`); `UpdateChangedFile` now `Enqueue`s a `RevalidateFileJob` after `tx.Commit()` (`handler.go:144-145`); the daemon E2E test extended to assert `bundle.lspQueue.Len() == 1` after one EditNotifier.OnEdit (`live_e2e_test.go:176`).

The 9 OnEdit call sites in kernel correctly cover all 8 LIVE-07 tools. The 7 warning items (WR-01..WR-07) and 6 info items from 60-REVIEW.md remain open as documented backlog (not phase-blocking). The 2 intentional Phase 62 stubs (`EffectiveContentHash`, `KnownFiles`) are explicitly out-of-scope per 60-05b-SUMMARY.

All gates green:
- `go build ./internal/... ./cmd/...` clean
- `go vet ./...` clean
- `go vet -vettool=...vet-nokernel2semantic ./internal/...` clean (kernel⊥semantic boundary intact)
- `go test ./internal/lint/... ./internal/semantic/store/... ./internal/kernel/... ./internal/semantic/live/... ./internal/daemon/... ./internal/phasegraph/... ./internal/obs/...` all packages pass
- `go test -race ./internal/semantic/store/... -run TestOverlayEpochConcurrent` ok 2.082s

---

_Verified: 2026-05-05T18:00:00Z_
_Verifier: Claude (gsd-verifier)_
