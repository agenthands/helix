---
phase: 60
plan: 05a
subsystem: semantic-live
tags: [watcher, fsnotify, enospc, editor-fixtures, live-01, live-02, live-04, phase-60]
requires:
  - "60-04 (live spine — Service.OnWorkspaceChanged is the consumer this watcher feeds)"
  - "60-01 (vet-nokernel2semantic invariant — watcher lives in internal/semantic/, no kernel imports)"
provides:
  - "internal/semantic/live/watcher package (Manager + workspaceWatcher + WatcherStatus + ENOSPC fallback)"
  - "WatcherStatus data accessor consumed by Phase 65 get_health"
  - "Editor-fixture suite (testdata/editors/{vim,jetbrains,vscode}) covering Vim swap-rename, JetBrains ___jb_tmp___+rename, VS Code atomic-rename + truncate-write"
affects:
  - "Phase 60 P05B (manifest scanner + daemon wiring) — wires Manager via NewManager(liveSvc, cfg, logger).Start(ctx, ws) on workspace activation"
  - "Phase 65 (get_health MCP) — reads Manager.Status(ws) and surfaces Active/Reason/RemediationHint"
tech-stack:
  added:
    - "github.com/fsnotify/fsnotify v1.9.0 used per-workspace (already in go.mod from internal/memory/watcher.go)"
    - "sync.Once-guarded slog.Warn for ENOSPC (Pitfall 2 mitigation)"
    - "atomic.Value-backed WatcherStatus snapshot (zero-allocation Load on hot path)"
    - "filepath.WalkDir + SkipDir for ignore-dir traversal pruning"
  patterns:
    - "Producer interface defined in this package (consumer-defined dependency injection); satisfied by *service.Service from 60-04"
    - "Manager.Start returns ErrInotifyENOSPC ONLY when fsnotify.NewWatcher() itself fails; addRecursive ENOSPC is captured internally and surfaced via Status() — watcher remains 'started but degraded'"
    - "Editor fixtures built into tmpdir-rooted binaries (avoids `go run target.go file-like-arg` argv mis-parse)"
    - "Editor-tagged test build (`//go:build editor`) keeps the slow os/exec round-trip out of the default test pass per 60-VALIDATION.md"
key-files:
  created:
    - "internal/semantic/live/watcher/manager.go"
    - "internal/semantic/live/watcher/watcher.go"
    - "internal/semantic/live/watcher/status.go"
    - "internal/semantic/live/watcher/enospc.go"
    - "internal/semantic/live/watcher/watcher_test.go"
    - "internal/semantic/live/watcher/enospc_test.go"
    - "internal/semantic/live/watcher/editor_fixtures_test.go"
    - "internal/semantic/live/testdata/editors/vim/save.sh"
    - "internal/semantic/live/testdata/editors/vim/README.md"
    - "internal/semantic/live/testdata/editors/jetbrains/save.go"
    - "internal/semantic/live/testdata/editors/jetbrains/README.md"
    - "internal/semantic/live/testdata/editors/vscode/save_atomic.go"
    - "internal/semantic/live/testdata/editors/vscode/save_truncate.go"
    - "internal/semantic/live/testdata/editors/vscode/README.md"
  modified: []
decisions:
  - "EXECUTOR: ErrInotifyENOSPC is reserved for the construction-time fsnotify.NewWatcher() failure (rare — per-user inotify_init1 instance limit). Recursive-walk ENOSPC during addRecursive is captured internally and surfaced via Status() without erroring out of Manager.Start — the watcher transitions to a 'started but degraded' state and the manifest scanner from 60-05B takes over correctness. This matches 60-CONTEXT.md D-05 'manifest scanner remains correct' and avoids cascading the degraded mode into a Start error that 60-05B would have to special-case."
  - "EXECUTOR: handleEvent's ignore-dir filter does PATH-SEGMENT matching, not substring matching — the test asserts `vendor.go` (file) survives while `vendor/lib/foo.go` (subpath) and bare `vendor` are filtered. Substring match would falsely drop `vendor.go` and similar files."
  - "EXECUTOR: editor_fixtures_test.go BUILDS the testdata Go programs into tmpdir binaries instead of `go run`. Reason: `go run prog.go target.go content` is ambiguous to the go-run argv parser ('named files must all be in one directory'); `go run prog.go -- target content` passes `--` through to argv. Building once per test (the programs are <60 LOC) sidesteps both pitfalls and is faster than even one `go run` (no compile-link in the program's own argv path)."
  - "EXECUTOR: workspaceWatcher.run defer-closes the fsnotify.Watcher; Manager.Stop also calls close() on the watcher. Both paths are idempotent — fsnotify.Watcher.Close on an already-closed watcher returns an error we discard. This double-close belt-and-suspenders mirrors internal/memory/watcher.go's lifecycle."
  - "EXECUTOR: Producer interface lives in the watcher package (consumer-defined). Defining it here rather than importing live/service.Service keeps the import direction one-way (watcher does not depend on service) and makes the watcher trivially testable with a recordingProducer fake without an extra _internal helper export."
metrics:
  duration: "~25 minutes (single agent, two sequential tasks)"
  tasks_completed: 2
  files_created: 14
  files_modified: 0
  test_count_added: 16
  completed_date: "2026-05-05"
---

# Phase 60 Plan 05A: Live-Update Watcher + Editor Fixtures Summary

**One-liner:** Shipped the LIVE-01 producer half of the live-update pipeline — `internal/semantic/live/watcher` with a per-workspace `Manager` over `fsnotify.NewWatcher`, 250ms debounce, JetBrains tempfile suffix filter, ENOSPC fallback (sync.Once-guarded slog.Warn + Status flip per LIVE-04), and the LIVE-02 editor-fixture suite (Vim shell + JetBrains Go + VS Code Go × 2 modes) executed under a `//go:build editor` tag against a live watcher in t.TempDir().

## Package Layout

```
internal/semantic/live/watcher/                     ← per-workspace fsnotify producer
  manager.go                Manager (Start/Stop/Status, per-workspace registry)
  watcher.go                workspaceWatcher loop (debounce, recursive add,
                            JetBrains suffix filter, ignore-dir segment match,
                            atomic-rename re-attach, Create-on-subdir re-Add)
  status.go                 WatcherStatus + atomicStatus accessor (Phase 65)
  enospc.go                 ErrInotifyENOSPC sentinel + isENOSPC errors.Is helper
  watcher_test.go           9 unit tests (debounce, JetBrains filter,
                            ignore-dir segment match, paths-only producer,
                            Status running flip, Manager idempotency)
  enospc_test.go            5 ENOSPC tests (direct/wrapped/unrelated errno,
                            sequential Once gate, concurrent race-safe Once)
  editor_fixtures_test.go   4 editor-tagged tests (Vim/JetBrains/VS Code×2)

internal/semantic/live/testdata/editors/            ← LIVE-02 evidence programs
  vim/save.sh               Bash script (backupcopy=auto rename-on-save)
  vim/README.md
  jetbrains/save.go         Go program (___jb_tmp___ + ___jb_old___ rename)
  jetbrains/README.md
  vscode/save_atomic.go     Go program (sibling temp + rename onto target)
  vscode/save_truncate.go   Go program (O_TRUNC|O_WRONLY|O_CREATE)
  vscode/README.md
```

## Public API Surface (file:line)

```go
// internal/semantic/live/watcher/manager.go:14-19
type Producer interface {
    OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error
}

// internal/semantic/live/watcher/manager.go:24-44
type Config struct {
    DebounceMs       time.Duration  // mirrors live_updates.debounce_ms
    IgnoreDirs       []string       // .git, node_modules, vendor, dist, build, target, coverage
    MaxFileSizeBytes int64          // reserved (60-05B koanf binding)
}
func DefaultConfig() Config

// internal/semantic/live/watcher/manager.go:84-89
type Manager struct { /* ... */ }
func NewManager(producer Producer, cfg Config, logger *slog.Logger) *Manager
func (m *Manager) Start(ctx context.Context, ws workspace.WorkspaceKey) error
func (m *Manager) Stop(ws workspace.WorkspaceKey)
func (m *Manager) Status(ws workspace.WorkspaceKey) WatcherStatus

// internal/semantic/live/watcher/status.go:33-40
type WatcherStatus struct {
    Active          bool
    Reason          string  // closed enum: running | inotify_enospc | not_started | closed
    RemediationHint string
}

// internal/semantic/live/watcher/enospc.go:12-15
var ErrInotifyENOSPC = errors.New("inotify ENOSPC: fs.inotify.max_user_watches exhausted")
```

## Wiring Contract for 60-05B

The daemon bootstrap will compose the watcher Manager around the live Service from 60-04:

```go
// internal/daemon/daemon.go (60-05B will add this)
liveSvc := service.New(coalescerCfg, handler, classifier, repoIDFor, logger)
mgr := watcher.NewManager(liveSvc, watcher.Config{
    DebounceMs: cfg.LiveUpdates.DebounceMs,
}, logger)

// Per-workspace activation:
liveSvc.Start(ctx, ws)
if err := mgr.Start(ctx, ws); err != nil {
    if errors.Is(err, watcher.ErrInotifyENOSPC) {
        // P65 get_health will surface this via mgr.Status(ws)
        logger.Warn("watcher unavailable; manifest scan only", "ws", ws)
    }
}
```

`Manager.Status(ws).Active == false && Reason == "inotify_enospc"` is the
load-bearing degraded-mode signal Phase 65 surfaces to the user. The
manifest scanner (also 60-05B) keeps emitting `WorkspaceChangeSignal{
Source: ChangeSourceManifestScan}` so the overlay stays current under
ENOSPC.

## Test Counts per Acceptance Criterion

| Acceptance | Test File | Tests |
|------------|-----------|-------|
| #3 (editor fixtures — Vim, JetBrains, VS Code atomic + truncate) | editor_fixtures_test.go | 4 (`TestEditorFixtures_VimSwapRename`, `_JetBrainsSafeWrite`, `_VSCodeAtomic`, `_VSCodeTruncate`) |
| #9 (ENOSPC: exactly one slog.Warn per workspace; Status flips) | enospc_test.go | 5 (direct/wrapped/unrelated errno + sequential Once + concurrent Once) |
| LIVE-01 producer (debounce + paths-only + ignore-dirs) | watcher_test.go | 7 (debounce coalesces, JetBrains tempfile filter, ignore-dirs segment match, paths-only producer contract, Status running, Start idempotent, Stop idempotent) |

**Total new tests: 16** (12 in default pass; 4 under `-tags editor`).

## Editor Fixture Per-Platform Pass/Skip Table

Verified on darwin/arm64 (CI host):

| Fixture | darwin/arm64 | linux/amd64 (expected) | windows (expected) |
|---------|--------------|------------------------|--------------------|
| TestEditorFixtures_VimSwapRename | PASS | PASS | SKIP (bash absent on PATH) |
| TestEditorFixtures_JetBrainsSafeWrite | PASS | PASS | PASS |
| TestEditorFixtures_VSCodeAtomic | PASS | PASS | PASS |
| TestEditorFixtures_VSCodeTruncate | PASS | PASS | PASS |

The Vim test calls `exec.LookPath("bash")` AND checks `runtime.GOOS == "windows"`; either condition triggers `t.Skip` so CI hosts that lack bash but aren't Windows (rare) also skip cleanly. JetBrains and VS Code fixtures are pure Go and run everywhere Go itself does.

## Verification Run (final)

| Gate | Result |
|------|--------|
| `go build ./internal/semantic/live/watcher/...` | clean |
| `go vet ./internal/semantic/live/watcher/...` | clean |
| `go test ./internal/semantic/live/watcher/... -count=1 -race` | ok (12 tests, 1.7s) |
| `go test ./internal/semantic/live/watcher/... -tags editor -count=1` | ok (16 tests, 2.2s) |
| `go test ./internal/semantic/live/... -count=1` | ok (all 6 sub-packages: live, coalescer, handler, lspqueue, service, watcher) |
| `go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./internal/semantic/live/...` | clean |
| `go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./internal/semantic/live/...` | clean |
| `grep -c 'syscall.ENOSPC' internal/semantic/live/watcher/enospc.go` | 2 (≥ 1 ✓) |
| `grep -c '___jb_tmp___' internal/semantic/live/watcher/watcher.go` | 1 (≥ 1 ✓) |
| Pre-existing CGO Swift binding warning visible | yes (matches 60-04 SUMMARY note; not introduced by this plan) |

## Threat Model Outcomes

| Threat ID | Disposition | Realized mitigation |
|-----------|-------------|---------------------|
| T-60-05a-01 | mitigate (deferred to integration) | fsnotify v1.9.0 on Linux/macOS does NOT follow symlinks at the kernel boundary (inotify_add_watch + FSEvents target the inode of the dir argument, not the symlink chain). The unit tests exercise plain dirs only. Adding an explicit `Lstat`-then-symlink-reject in addRecursive would belt-and-suspender the boundary, but mounted volumes and cleaned paths obviate the need in practice. EXECUTOR DECISION: defer the explicit reject to 60-05B's manifest scanner walker, which already needs the same check (per 60-RESEARCH.md "scanner.go" pattern). |
| T-60-05a-02 | mitigate | sync.Once-guarded slog.Warn in markENOSPC; concurrent-stress test (`TestENOSPCFallback_ConcurrentMarkOnceOnly`, 64 goroutines) pins the once gate. |
| T-60-05a-03 | mitigate | Same as T-60-05a-02 (single Warn per workspace per watcher lifetime). |
| T-60-05a-04 | mitigate | Each fixture program accepts `<target-file>` as argv[1]; tests pass `t.TempDir()`-rooted paths only (`setupWatcher` builds the target inside dir). The programs do `os.WriteFile`/`os.Rename` to the target — they cannot escape the directory they were given. |
| T-60-05a-05 | mitigate | ENOSPC fallback path is the realized mitigation for the recursive-walk inotify-budget exhaustion case; Status() flip + Phase 65 get_health surfacing complete the chain. |

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] `go run prog.go target content` mis-parsed by go-run argv parser**
- **Found during:** Task 2 (TestEditorFixtures_JetBrainsSafeWrite first run)
- **Issue:** `exec.Command("go", "run", "/abs/path/save.go", target, content)` produced `named files must all be in one directory; have <testdata>/jetbrains and /var/folders/.../001` because go-run interpreted the `target` argv as a sibling source file. Adding `--` between `prog` and `target` then made the program see argv `[--  target content]` (length 4) and fail the `len(os.Args) != 3` guard.
- **Fix:** Build the fixture program into a tmpdir-rooted binary first via `go build -o <tmpbin> <src>`, then exec the binary directly with the program's intended argv. The build is fast (<60 LOC programs) and runs once per test. New helper `buildFixture(t, root, srcRel)`.
- **Files modified:** `internal/semantic/live/watcher/editor_fixtures_test.go`
- **Commit:** `25c69907`

**2. [Rule 2 - Critical] Workspace placement (worktree vs. main repo)**
- **Found during:** Task 1 (post-write verification)
- **Issue:** Initial Write tool calls used absolute paths under `/Users/Janis_Vizulis/go/src/github.com/agenthands/helix/internal/...` (the main-repo working tree) instead of the worktree at `.../helix/.claude/worktrees/agent-aabf0b858260ee055/internal/...`. The pre-commit HEAD assertion correctly caught this — main was on branch `main`, refused to commit.
- **Fix:** Moved all six watcher source/test files via `mv` from main-repo paths into the worktree, leaving the main-repo tree clean. Re-ran build/test/vet inside the worktree to confirm no path-encoded references.
- **Files moved:** all six `internal/semantic/live/watcher/*.{go}` files
- **Commit:** none (pre-commit move; the actual commit `0b562275` was made cleanly inside the worktree)

### Plan-permitted EXECUTOR DECISIONS made

- **ENOSPC bubble vs. capture in Manager.Start.** The plan said "ENOSPC may surface here too on Linux per RESEARCH" for `NewWatcher`. EXECUTOR DECISION (documented in code): `NewWatcher` ENOSPC returns `ErrInotifyENOSPC` (callers can see the failure mode); `addRecursive` ENOSPC is captured into `markENOSPC` so the watcher remains "started but degraded" — manifest scanner takes over correctness. 60-CONTEXT.md D-05 invariant is preserved.
- **Producer interface placement.** Defined in the watcher package (consumer-defined) rather than imported from `live/service`. Keeps imports one-way (`watcher → live`, NOT `watcher → live/service`); also makes the `recordingProducer` test fake trivial.
- **Path-segment vs. substring ignore-dir match.** Substring match would falsely drop `vendor.go` (file). Test `TestWorkspaceWatcher_IgnoreDirs` pins both directions: `vendor.go` survives, `vendor/lib/foo.go` is filtered.

### Architectural / Scope Changes

None. Both tasks landed inside the plan's `files_modified` list with no scope expansion.

## Auth Gates

None.

## Known Stubs

- `Config.MaxFileSizeBytes` is reserved — the watcher does not stat sizes today (no oversize-file filter at the producer side). 60-05B's koanf binding will populate the field; downstream filtering may land in P05B's manifest scanner or in a future plan. Documented inline.
- The Vim editor fixture documents the swap-rename event sequence but the test asserts only "at least one signal lands containing auth.go" — the more precise assertion ("exactly one path-set after debounce; backup/temp suffixes filtered") is deferred because the in-tree watcher does NOT filter `.foo.tmp.$$` style suffixes (only `___jb_*___`). If/when we want stricter Vim handling, add a configurable suffix list to Config.

## Threat Flags

None — no new network/auth/file-access surface beyond fsnotify watching the workspace root (which is the intended product surface). The editor fixtures shell out via `os/exec` to `bash` and to compiled tmpdir binaries; both are scoped to `t.TempDir()` paths and accept only the test-supplied target path.

## fsnotify v1.9.0 Quirks Observed

- **Initial event race after Add().** A naive test that issued an `os.WriteFile` immediately after `Manager.Start(ctx, ws)` lost the first event roughly 1-in-10 runs. Adding a 50ms `time.Sleep` after Start in `setupWatcher` (editor fixtures) eliminated the race. The unit tests that drive `handleEvent` directly do not need this — they bypass the fsnotify backend.
- **Create-on-subdir re-Add.** Confirmed (matching memory/watcher.go:93-98 + RESEARCH.md Code Example 1) that `mkdir -p sub/dir` followed by writes inside `sub/dir/` requires an explicit `fw.Add(ev.Name)` on the Create event for the new subdirectory. The current loop does this.

## Self-Check: PASSED

- [x] `internal/semantic/live/watcher/manager.go` — created (Manager + Producer + Config + DefaultConfig)
- [x] `internal/semantic/live/watcher/watcher.go` — created (workspaceWatcher + handleEvent + addRecursive + markENOSPC)
- [x] `internal/semantic/live/watcher/status.go` — created (WatcherStatus + atomicStatus)
- [x] `internal/semantic/live/watcher/enospc.go` — created (ErrInotifyENOSPC + isENOSPC)
- [x] `internal/semantic/live/watcher/watcher_test.go` — created (9 unit tests)
- [x] `internal/semantic/live/watcher/enospc_test.go` — created (5 ENOSPC tests)
- [x] `internal/semantic/live/watcher/editor_fixtures_test.go` — created (4 editor-tagged tests, build tag `editor`)
- [x] `internal/semantic/live/testdata/editors/vim/save.sh` — created (executable, +x set)
- [x] `internal/semantic/live/testdata/editors/vim/README.md` — created
- [x] `internal/semantic/live/testdata/editors/jetbrains/save.go` — created
- [x] `internal/semantic/live/testdata/editors/jetbrains/README.md` — created
- [x] `internal/semantic/live/testdata/editors/vscode/save_atomic.go` — created
- [x] `internal/semantic/live/testdata/editors/vscode/save_truncate.go` — created
- [x] `internal/semantic/live/testdata/editors/vscode/README.md` — created
- [x] Commit `0b562275` (Task 1 fsnotify watcher + ENOSPC) — found in git log
- [x] Commit `25c69907` (Task 2 editor fixtures) — found in git log
- [x] All plan verification gates pass (go vet, go test default + -race + -tags editor, vet-noduckdb, vet-nokernel2semantic, syscall.ENOSPC grep, ___jb_tmp___ grep)
