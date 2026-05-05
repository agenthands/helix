---
phase: 60
plan: 05b
type: execute
wave: 3
depends_on: ["60-02", "60-03", "60-04"]
files_modified:
  - internal/semantic/live/scanner/scanner.go
  - internal/semantic/live/scanner/manager.go
  - internal/semantic/live/scanner/walk.go
  - internal/semantic/live/scanner/scanner_integration_test.go
  - internal/config/defaults.go
  - internal/config/loader_test.go
  - internal/semantic/config/config.go
  - internal/obs/metrics.go
  - internal/obs/metrics_labels_test.go
  - internal/obs/metrics_test.go
  - internal/phasegraph/pipelines/live.go
  - internal/daemon/daemon.go
  - .planning/REQUIREMENTS.md
  - .planning/ROADMAP.md
autonomous: false
requirements: [LIVE-03]
must_haves:
  truths:
    - "Manifest scanner walks workspace every manifest_scan_interval (default 10s), hashes via xxhash64, compares against semantic_files.content_hash, emits WorkspaceChangeSignal{Source: manifest_scan} on mismatch (LIVE-03)"
    - "Manifest scanner detects watcher misses within 2 × interval — verified by integration test"
    - "Three new config keys watcher_enabled / manifest_scan_enabled / manifest_scan_interval flow through 4-layer koanf precedence"
    - "helix_semantic_live_updates_total{kind, outcome} bounded labels via closed-enum drop-on-unknown helper, registered in internal/obs"
    - "Daemon bootstrap wires liveService → kernel.SetEditNotifier(liveService); watcher manager (from P05A) + scanner manager start per workspace activation gated on cfg.SemanticIndex.LiveUpdates.{Enabled, WatcherEnabled, ManifestScanEnabled}"
    - "internal/phasegraph/pipelines/live.go: zero noopRun bodies remain in the SHIPPED phase constructor — every one of the 9 phases gets a real closure (or any retained no-op is documented by name in the SUMMARY)"
    - "LIVE-01..LIVE-07 all flipped to [x] in REQUIREMENTS.md after Task 4 close-out checkpoint approval"
  artifacts:
    - path: internal/semantic/live/scanner/scanner.go
      provides: "Per-workspace manifest scanner ticker loop"
      contains: "time.NewTicker"
    - path: internal/semantic/live/scanner/walk.go
      provides: "filepath.WalkDir with embedded skipDirs (no repomap import)"
      contains: "filepath.WalkDir"
    - path: internal/obs/metrics.go
      provides: "SemanticLiveUpdatesInc helper with closed-enum drop-on-unknown"
      contains: "SemanticLiveUpdatesInc"
    - path: internal/daemon/daemon.go
      provides: "Live-update wiring step: liveService construction, kernel.SetEditNotifier, watcher/scanner Start in SetActivateCallback"
      contains: "SetEditNotifier(liveService)"
  key_links:
    - from: internal/daemon/daemon.go SetActivateCallback
      to: watcherMgr.Start(ws) + scannerMgr.Start(ws)
      via: "gated on cfg.SemanticIndex.LiveUpdates.{Enabled,WatcherEnabled,ManifestScanEnabled}"
      pattern: "watcherMgr\\.Start|scannerMgr\\.Start"
    - from: internal/daemon/daemon.go bootstrap
      to: kernel.SetEditNotifier(liveService)
      via: "post Phase 59 scheduler open"
      pattern: "SetEditNotifier"
tags: [scanner, manifest-scan, daemon-wiring, metrics, pipelines, live-03, close-out]
---

<objective>
Ship the manifest scanner (LIVE-03), the three new live-update config keys,
the bounded-label `helix_semantic_live_updates_total` metric, the
`pipelines/live.go` body fills (zero noopRun in the shipped constructor),
the daemon bootstrap wiring, and the Phase 60 close-out checkpoint that
flips LIVE-01..LIVE-07 to `[x]` in REQUIREMENTS.md.

This is the wiring half of the original P05 — split out per checker
WARNING-4 to keep each plan within sane context budget. Sibling plan
P05A ships the watcher + ENOSPC fallback + LIVE-02 editor fixtures.

P05A and P05B share Wave 3 (both depend on P02+P03+P04) and have ZERO
files_modified overlap, so they may run concurrently. The daemon
bootstrap wiring in this plan REFERENCES the watcher.Manager type from
P05A's package — but P05A and P05B run in parallel because the daemon
wiring lands in a SetActivateCallback closure that is only invoked at
runtime, not at compile-time of P05A's package. (Both plans build
against each other's symbols cleanly.)

Dependencies:
  - P02 (Wave 1) — overlay tx (handler consumer)
  - P03 (Wave 1) — kernel.EditNotifier + 8-tool wiring (target of SetEditNotifier)
  - P04 (Wave 2) — liveService, classifier, coalescer, handler, ScheduleIncremental
  - P05A (Wave 3, parallel) — watcher.Manager type referenced by daemon bootstrap

Purpose: Wire the producer side (scanner) and the daemon-level
integration (config + metric + pipelines + bootstrap) and close out the
phase against REQUIREMENTS.md.
Output: One sub-package (`internal/semantic/live/scanner/`), three
config keys + struct fields, one bounded-label metric helper,
`pipelines/live.go` constructor with zero noopRun, daemon bootstrap step
that ties everything together, and the close-out checkpoint.
</objective>

<execution_context>
@$HOME/.claude/get-shit-done/workflows/execute-plan.md
@$HOME/.claude/get-shit-done/templates/summary.md
</execution_context>

<context>
@.planning/PROJECT.md
@.planning/ROADMAP.md
@.planning/STATE.md
@.planning/phases/60-live-update-pipeline/60-CONTEXT.md
@.planning/phases/60-live-update-pipeline/60-RESEARCH.md
@.planning/phases/60-live-update-pipeline/60-PATTERNS.md
@.planning/phases/60-live-update-pipeline/60-VALIDATION.md
@.planning/phases/60-live-update-pipeline/60-01-SUMMARY.md
@.planning/phases/60-live-update-pipeline/60-02-SUMMARY.md
@.planning/phases/60-live-update-pipeline/60-03-SUMMARY.md
@.planning/phases/60-live-update-pipeline/60-04-SUMMARY.md
@.planning/phases/60-live-update-pipeline/60-05a-SUMMARY.md
@internal/skill/repomap/skill.go
@internal/daemon/daemon.go
@internal/config/defaults.go
@internal/obs/metrics.go
@internal/phasegraph/pipelines/live.go

<interfaces>
<!-- Live service public API from P04 (the consumer this plan wires) -->

```go
// internal/semantic/live/service.go
type Service struct { /* ... */ }
func NewService(cfg coalescer.Config, handler coalescer.EventHandler, classifier ..., repoIDFor ..., logger ...) *Service
func (s *Service) Start(ctx context.Context, ws workspace.WorkspaceKey)
func (s *Service) Stop(ws workspace.WorkspaceKey)
func (s *Service) OnEdit(ctx context.Context, ws workspace.WorkspaceKey, paths []string) error
func (s *Service) OnWorkspaceChanged(ctx context.Context, sig WorkspaceChangeSignal) error
```

<!-- Watcher manager (from P05A; this plan instantiates it) -->

```go
// internal/semantic/live/watcher/manager.go
func NewManager(p Producer, cfg Config, logger *slog.Logger) *Manager
func (m *Manager) Start(ctx context.Context, ws workspace.WorkspaceKey) error
func (m *Manager) Stop(ws workspace.WorkspaceKey)
```

<!-- internal/obs/metrics.go closed-enum helper analog (lines 333-351) -->

```go
func (m *Metrics) EditOutcomeInc(toolName, outcome, strategy string) {
    switch outcome {
    case "success", "no_match", "ambiguous", "verifier_blocked",
         "concurrent_modification", "internal_error":
    default:
        return
    }
    m.EditOutcome.WithLabelValues(toolName, outcome, strategy).Inc()
}
```

<!-- existing daemon SetActivateCallback (verbatim, internal/daemon/daemon.go:463-496 sketch) -->

```go
mcpServer.SetActivateCallback(func(ctx context.Context, repoPath string) error {
    rt, err := k.ActivateWorkspace(ctx, repoPath)
    if err != nil { return err }
    activeWSKey = workspace.WorkspaceKey{RepoRoot: repoPath}
    if semanticScheduler != nil {
        semanticScheduler.ScheduleInitialExtraction(...)
    }
    return nil
})
```

<!-- pipelines/live.go (verbatim phase IDs from internal/phasegraph/pipelines/live.go) -->

```go
const (
    PhaseCollectEvents          phasegraph.PhaseID = "collect_events"
    PhaseCoalesceEvents         phasegraph.PhaseID = "coalesce_events"
    PhaseClassifyEvents         phasegraph.PhaseID = "classify_events"
    PhaseParseChangedFiles      phasegraph.PhaseID = "parse_changed_files"
    PhaseDiffEffectiveFacts     phasegraph.PhaseID = "diff_effective_facts"
    PhaseWriteOverlay           phasegraph.PhaseID = "write_overlay"
    PhaseRepairGraphCache       phasegraph.PhaseID = "repair_graph_cache"
    PhaseMarkScoresClusters     phasegraph.PhaseID = "mark_scores_clusters"
    PhaseEnqueueLSPRevalidation phasegraph.PhaseID = "enqueue_lsp_revalidation"
)
// All 9 currently use noopRun — this plan replaces ALL of them.
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: Manifest scanner with periodic walk + ENOSPC fallback contract</name>
  <files>
    internal/semantic/live/scanner/scanner.go,
    internal/semantic/live/scanner/manager.go,
    internal/semantic/live/scanner/walk.go,
    internal/semantic/live/scanner/scanner_integration_test.go
  </files>
  <read_first>
    - internal/skill/repomap/skill.go lines 386-411 (walkAndExtract — STRUCTURAL TEMPLATE; do NOT import this package)
    - internal/kernel/fileops/find.go lines 35-67 (alternative in-tree walker analog with skipDirs)
    - 60-CONTEXT.md D-05 (manifest scanner contract)
    - 60-PATTERNS.md "internal/semantic/live/scanner/scanner.go" section
    - 60-VALIDATION.md acceptance criterion #10 (scanner catches watcher misses within 2× interval)
  </read_first>
  <action>
    Create `internal/semantic/live/scanner/walk.go`:

    ```go
    // Package scanner ships the periodic content-hash manifest scanner that
    // catches missed fsnotify events (LIVE-03) and serves as the ENOSPC
    // fallback (LIVE-04). Walks the workspace root every interval, hashes
    // each file with xxhash64, compares against semantic_files.content_hash,
    // emits WorkspaceChangeSignal{Source: manifest_scan} on mismatch.
    //
    // 60-CONTEXT.md D-05 invariant: this package does NOT import
    // internal/repomap/* — the walker shape mirrors repomap's but is
    // net-new code. The skipDirs list is re-declared here.
    package scanner

    import (
        "io/fs"
        "os"
        "path/filepath"
    )

    // skipDirs mirrors SPEC §13.2 exclusions; declared verbatim in the live
    // scanner package to honor the 59-D-01 cascade (no internal/repomap
    // import from internal/semantic/...).
    var skipDirs = map[string]bool{
        ".git":         true,
        "node_modules": true,
        "vendor":       true,
        "dist":         true,
        "build":        true,
        "target":       true,
        "coverage":     true,
    }

    // Walk visits every regular file under root that is not below a skipped
    // directory and is not a symlink. fn is called with the absolute path.
    func Walk(root string, fn func(absPath string) error) error {
        return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
            if err != nil { return nil }
            if d.IsDir() {
                if skipDirs[d.Name()] { return filepath.SkipDir }
                return nil
            }
            if d.Type()&fs.ModeSymlink != 0 { return nil }
            return fn(path)
        })
    }
    ```

    Create `internal/semantic/live/scanner/scanner.go`:

    ```go
    package scanner

    import (
        "context"
        "fmt"
        "io"
        "log/slog"
        "os"
        "sync"
        "time"

        "github.com/cespare/xxhash/v2"

        "github.com/agenthands/helix/internal/semantic"
        "github.com/agenthands/helix/internal/semantic/live"
        "github.com/agenthands/helix/internal/workspace"
    )

    type Producer interface {
        OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error
    }

    type FileHashLookup interface {
        // KnownFiles returns map[absPath]contentHash for the given repoID.
        // Used to detect deletions (path in store, not on disk).
        KnownFiles(ctx context.Context, repoID semantic.RepoID) (map[string]string, error)
    }

    type Config struct {
        Interval         time.Duration
        MaxParallelFiles int
    }

    type Scanner struct {
        ws         workspace.WorkspaceKey
        repoID     semantic.RepoID
        producer   Producer
        lookup     FileHashLookup
        cfg        Config
        logger     *slog.Logger
    }

    func New(ws workspace.WorkspaceKey, repoID semantic.RepoID, p Producer, l FileHashLookup, cfg Config, logger *slog.Logger) *Scanner {
        if cfg.Interval <= 0 { cfg.Interval = 10 * time.Second }
        if cfg.MaxParallelFiles <= 0 { cfg.MaxParallelFiles = 4 }
        return &Scanner{ws: ws, repoID: repoID, producer: p, lookup: l, cfg: cfg, logger: logger}
    }

    // Run drains until ctx is cancelled. One scan cycle per Interval.
    func (s *Scanner) Run(ctx context.Context) error {
        // Run an immediate first scan so tests don't have to wait Interval before
        // observing a missed event.
        s.scanOnce(ctx)
        ticker := time.NewTicker(s.cfg.Interval)
        defer ticker.Stop()
        for {
            select {
            case <-ctx.Done(): return ctx.Err()
            case <-ticker.C:  s.scanOnce(ctx)
            }
        }
    }

    func (s *Scanner) scanOnce(ctx context.Context) {
        known, err := s.lookup.KnownFiles(ctx, s.repoID)
        if err != nil {
            s.logger.Warn("scanner: KnownFiles failed", "err", err)
            return
        }
        seen := make(map[string]bool, len(known))
        var changed []string
        var mu sync.Mutex

        // Sequential walk for now; concurrency is a follow-up tunable.
        _ = Walk(s.ws.RepoRoot, func(absPath string) error {
            seen[absPath] = true
            hash, err := hashFile(absPath)
            if err != nil { return nil }
            prev, ok := known[absPath]
            if !ok || prev != hash {
                mu.Lock()
                changed = append(changed, absPath)
                mu.Unlock()
            }
            return nil
        })

        // Detect deletions: path in store, missing on disk.
        for path := range known {
            if !seen[path] {
                changed = append(changed, path)
            }
        }
        if len(changed) == 0 { return }
        _ = s.producer.OnWorkspaceChanged(ctx, live.WorkspaceChangeSignal{
            WorkspaceID: s.ws,
            Paths:       changed,
            Source:      live.ChangeSourceManifestScan,
            ObservedAt:  time.Now(),
        })
    }

    func hashFile(path string) (string, error) {
        f, err := os.Open(path)
        if err != nil { return "", err }
        defer f.Close()
        h := xxhash.New()
        if _, err := io.Copy(h, f); err != nil { return "", err }
        return fmt.Sprintf("%016x", h.Sum64()), nil
    }
    ```

    Create `internal/semantic/live/scanner/manager.go` mirroring `watcher/manager.go`: `Manager` with `Start(ctx, ws)`, `Stop(ws)`. Each scanner owns its own goroutine.

    Create `internal/semantic/live/scanner/scanner_integration_test.go`:

    ```go
    package scanner

    import (
        "context"
        "os"
        "path/filepath"
        "sync"
        "testing"
        "time"

        "github.com/agenthands/helix/internal/semantic"
        "github.com/agenthands/helix/internal/semantic/live"
        "github.com/agenthands/helix/internal/workspace"
    )

    type recordingProducer struct {
        mu  sync.Mutex
        sigs []live.WorkspaceChangeSignal
    }
    func (r *recordingProducer) OnWorkspaceChanged(ctx context.Context, s live.WorkspaceChangeSignal) error {
        r.mu.Lock(); defer r.mu.Unlock()
        r.sigs = append(r.sigs, s)
        return nil
    }

    type stubLookup struct{ files map[string]string }
    func (s *stubLookup) KnownFiles(ctx context.Context, _ semantic.RepoID) (map[string]string, error) {
        return s.files, nil
    }

    func TestScanner_DetectsNewFile(t *testing.T) {
        dir := t.TempDir()
        os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644)

        rp := &recordingProducer{}
        sc := New(
            workspace.WorkspaceKey{RepoRoot: dir},
            "ws1", rp,
            &stubLookup{files: map[string]string{}}, // empty store → "a.go" is created
            Config{Interval: 50 * time.Millisecond}, testLogger(t))

        ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
        defer cancel()
        _ = sc.Run(ctx)

        rp.mu.Lock(); defer rp.mu.Unlock()
        if len(rp.sigs) == 0 { t.Fatal("expected at least one signal") }
        if rp.sigs[0].Source != live.ChangeSourceManifestScan { t.Fatalf("source=%v", rp.sigs[0].Source) }
    }

    func TestScannerCatchesWatcherMisses(t *testing.T) {
        // Acceptance #10: scanner detects a watcher miss within 2 × interval.
        // Setup: one file "a.go" in store with the OLD hash; modify on disk;
        // run scanner with interval=200ms; assert signal within 500ms.
        dir := t.TempDir()
        path := filepath.Join(dir, "a.go")
        os.WriteFile(path, []byte("v1"), 0o644)
        oldHash, _ := hashFile(path)
        os.WriteFile(path, []byte("v2"), 0o644) // off-watcher edit
        rp := &recordingProducer{}
        sc := New(workspace.WorkspaceKey{RepoRoot: dir}, "ws1", rp,
            &stubLookup{files: map[string]string{path: oldHash}},
            Config{Interval: 200 * time.Millisecond}, testLogger(t))

        ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
        defer cancel()
        _ = sc.Run(ctx)

        rp.mu.Lock(); defer rp.mu.Unlock()
        if len(rp.sigs) == 0 { t.Fatal("scanner did not detect the off-watcher edit within 500ms") }
        if !pathInList(rp.sigs[0].Paths, path) { t.Fatalf("paths=%v missing %s", rp.sigs[0].Paths, path) }
    }
    ```

    Add a small helper for FileHashLookup wired against the Phase 57+ `internal/semantic/store/effective.go`. The store impl reads `semantic_files.content_hash` per repoID. If a single-row "give me everything for repoID" query isn't already in `effective.go`, add the helper there as part of this task — keep it small (`SELECT path, content_hash FROM semantic_files WHERE repo_id = ?`).
  </action>
  <verify>
    <automated>cd $REPO_ROOT && go test ./internal/semantic/live/scanner/... -count=1 -timeout 30s && go vet ./...</automated>
  </verify>
  <done>
    Scanner walks tmpdir, hashes via xxhash64, emits WorkspaceChangeSignal on mismatch; TestScannerCatchesWatcherMisses asserts detection within 2× interval; no internal/repomap import (vet-clean).
  </done>
</task>

<task type="auto">
  <name>Task 2: Config keys (defaults + struct fields + per-feature defaults test) + bounded-label metric</name>
  <files>
    internal/config/defaults.go,
    internal/config/loader_test.go,
    internal/semantic/config/config.go,
    internal/obs/metrics.go,
    internal/obs/metrics_labels_test.go,
    internal/obs/metrics_test.go
  </files>
  <read_first>
    - internal/config/defaults.go lines 78-87 (existing live_updates.* block — APPEND to it)
    - internal/config/loader_test.go (TestLoad_SemanticIndexDefaults pattern — line 217+)
    - internal/semantic/config/config.go (or wherever SerenaConfig.SemanticIndex.LiveUpdates is declared — locate via grep)
    - internal/obs/metrics.go lines 333-351 (EditOutcomeInc closed-enum drop pattern)
    - internal/obs/metrics_labels_test.go (allowlist testing convention)
    - 60-CONTEXT.md D-05 (config keys), acceptance #11 (metric labels)
    - 60-PATTERNS.md "internal/config/defaults.go" + "Closed-enum metric labels" sections
  </read_first>
  <action>
    1) Append three keys to `internal/config/defaults.go` after line 87:

    ```go
        // Phase 60 D-05: live-updates watcher + manifest scanner toggles.
        "semantic_index.live_updates.watcher_enabled":         true,
        "semantic_index.live_updates.manifest_scan_enabled":   true,
        "semantic_index.live_updates.manifest_scan_interval":  "10s",
    ```

    2) Locate the `SerenaConfig.SemanticIndex.LiveUpdates` struct (likely `internal/semantic/config/config.go` per RESEARCH.md "Reusable Assets"). Add three fields:

    ```go
    type LiveUpdates struct {
        // ... existing fields ...
        WatcherEnabled       bool          `koanf:"watcher_enabled"`
        ManifestScanEnabled  bool          `koanf:"manifest_scan_enabled"`
        ManifestScanInterval time.Duration `koanf:"manifest_scan_interval"`
    }
    ```

    Add `time` import if missing.

    3) Add `TestLoad_LiveUpdatesDefaults` to `internal/config/loader_test.go` mirroring `TestLoad_SemanticIndexDefaults`:

    ```go
    func TestLoad_LiveUpdatesDefaults(t *testing.T) {
        cfg, err := Load(LoadOpts{}) // identical to existing TestLoad_SemanticIndexDefaults shape
        if err != nil { t.Fatalf("Load: %v", err) }
        if !cfg.SemanticIndex.LiveUpdates.WatcherEnabled {
            t.Errorf("default WatcherEnabled = false, want true")
        }
        if !cfg.SemanticIndex.LiveUpdates.ManifestScanEnabled {
            t.Errorf("default ManifestScanEnabled = false, want true")
        }
        if got := cfg.SemanticIndex.LiveUpdates.ManifestScanInterval; got != 10*time.Second {
            t.Errorf("default ManifestScanInterval = %v, want 10s", got)
        }
    }
    ```

    4) Add `helix_semantic_live_updates_total` metric. Edit `internal/obs/metrics.go`:

    Add a `*prometheus.CounterVec` field to `Metrics` struct named `SemanticLiveUpdates` with labels `kind, outcome`. Register it in `newMetrics()` alongside the existing vectors. Then append the helper after `EditOutcomeInc` (line 339):

    ```go
    // SemanticLiveUpdatesInc increments helix_semantic_live_updates_total.
    // Phase 60 D-07 closed-enum: kind ∈ {file_created, file_modified,
    // file_deleted, file_renamed, helix_edit, bulk_update}; outcome ∈
    // {applied, no_op, error, dropped}. Unknown values are dropped.
    func (m *Metrics) SemanticLiveUpdatesInc(kind, outcome string) {
        switch kind {
        case "file_created", "file_modified", "file_deleted", "file_renamed",
             "helix_edit", "bulk_update":
        default:
            return
        }
        switch outcome {
        case "applied", "no_op", "error", "dropped":
        default:
            return
        }
        m.SemanticLiveUpdates.WithLabelValues(kind, outcome).Inc()
    }
    ```

    5) Extend `internal/obs/metrics_labels_test.go` allowlist for the new metric. Search the test file for the AllowedLabels map and add an entry for `helix_semantic_live_updates_total` with the closed-enum kind+outcome carve-out matching the 6×4 = 24 combinations.

    6) Add a `metrics_test.go` test exercising the helper:

    ```go
    func TestSemanticLiveUpdatesInc_DropsUnknownKind(t *testing.T) {
        m := newMetrics(prometheus.NewRegistry())
        m.SemanticLiveUpdatesInc("garbage", "applied")
        // Counter must remain at zero — assert via gather.
    }
    func TestSemanticLiveUpdatesInc_DropsUnknownOutcome(t *testing.T) { /* similar */ }
    func TestSemanticLiveUpdatesInc_AcceptsAllValid(t *testing.T) { /* exercises 24 combos */ }
    ```
  </action>
  <verify>
    <automated>cd $REPO_ROOT && go test ./internal/config/... -run TestLoad_LiveUpdatesDefaults -count=1 && go test ./internal/obs/... -count=1 && go vet ./...</automated>
  </verify>
  <done>
    Three new keys load via koanf 4-layer precedence; SerenaConfig.SemanticIndex.LiveUpdates struct has the three fields; metric helper rejects unknown labels; allowlist test includes the new metric.
  </done>
</task>

<task type="auto">
  <name>Task 3: Pipeline live.go body fills + daemon bootstrap wiring</name>
  <files>
    internal/phasegraph/pipelines/live.go,
    internal/daemon/daemon.go
  </files>
  <read_first>
    - internal/phasegraph/pipelines/live.go (entire file — 9 noopRun placeholders to replace)
    - internal/daemon/daemon.go (lines 372-385 SetEnrichFn block; lines 463-496 SetActivateCallback block; the Phase 59 scheduler open block — locate where `semanticScheduler` is constructed)
    - 60-PATTERNS.md "internal/daemon/daemon.go" section (lines 816-899)
    - 60-CONTEXT.md D-06 (phasegraph fills), D-07 (ScheduleIncremental wiring path)
  </read_first>
  <action>
    1) Replace 9 `noopRun` placeholders in `pipelines/live.go` with closures that wire to the Phase 60 components. Per 60-PATTERNS.md:

    ```go
    // Replace each `Run: noopRun` with concrete bodies. The phase IDs +
    // Requires + Provides DO NOT CHANGE — Phase 57 D-04 locks them.
    //
    // The Run bodies receive a *phasegraph.PhaseContext (or whatever the
    // existing PhaseSpec.Run signature uses) and return error.
    //
    // For Phase 60, several bodies are intentionally thin:
    //   - PhaseRepairGraphCache and PhaseMarkScoresClusters are no-op stubs
    //     (Phase 62 fills bodies; the typed input/output flows through).
    //   - PhaseEnqueueLSPRevalidation calls lspqueue.Enqueue — Phase 61
    //     fills the consumer.
    //
    // Concrete: each Run closure receives the live service via the closure
    // capture (the daemon constructs LiveUpdatePhases AFTER NewService).
    ```

    The exact Run signature depends on `phasegraph.PhaseSpec.Run` — read the actual signature and adapt. If `Run` is `func(ctx context.Context) error`, the closures look like:

    ```go
    {ID: PhaseCollectEvents, Requires: nil, Provides: []string{"events"},
        Run: func(ctx context.Context) error {
            // Producer side — watcher + scanner + helix_edit hook all feed
            // liveService.OnWorkspaceChanged. The "phase" in the DAG sense
            // is "the live service is up and accepting signals" — i.e. this
            // body verifies wiring rather than performing per-tick work.
            return nil
        }},
    // ... etc ...
    ```

    Phase 60's contract is "fill the bodies": zero `noopRun` bodies may remain in the shipped constructor. Even if a phase's runtime work is reactive (driven by the watcher / coalescer rather than by ticking the DAG), its Run body MUST do something meaningful — at minimum a non-trivial validator, e.g. `if svc == nil { return errors.New("collect_events: live service not wired") }`. Any phase that absolutely cannot ship a real body MUST be enumerated by NAME with a justification in the SUMMARY's `<retained_noops>` section, and the verify-grep pin (`noopRun` count) MUST be updated to the exact remaining count instead of 0.

    The pragmatic ship path: provide LiveUpdatePhases as a constructor function (not a package-level var) that takes the wired components:

    ```go
    func BuildLiveUpdatePhases(svc *live.Service, store *store.Store, sched scheduler.ExtractionScheduler, lspQ *lspqueue.Queue) []phasegraph.PhaseSpec {
        return []phasegraph.PhaseSpec{
            {ID: PhaseCollectEvents, ..., Run: func(ctx context.Context) error { /* validate svc != nil */ }},
            // ...
        }
    }
    ```

    The existing package-level `LiveUpdatePhases` var stays (with noopRun) for compatibility with Phase 57's tests; the new constructor is what the daemon uses. Document the migration path in SUMMARY.

    2) Edit `internal/daemon/daemon.go`. After the Phase 59 scheduler-open block (search `semanticScheduler` to find it), insert:

    ```go
    // 12d. Wire Phase 60 live-update pipeline.
    var liveService *live.Service
    var watcherMgr  *watcher.Manager
    var scannerMgr  *scanner.Manager
    if cfg.SemanticIndex.Enabled && cfg.SemanticIndex.LiveUpdates.Enabled {
        // Build the handler (consumer of coalescer dispatches).
        h := &handler.Handler{
            Store:  semanticStore,            // P02 store handle
            Hasher: scanner.HashFile,         // expose hashFile via package-level var or func
            Sched:  semanticScheduler,        // Phase 59 scheduler
            Logger: logger,
        }
        // Wire the scheduler's IncrementalHandler back-edge.
        semanticScheduler.SetIncrementalHandler(h)

        // Build classifier closure.
        classifierFn := func(ctx context.Context, repoID semantic.RepoID, path string, src live.ChangeSource) (live.SourceChangeKind, bool, error) {
            return live.ClassifyPathChange(ctx, repoID, path,
                semanticStore /* implements FileHashLookup */,
                scanner.HashFile,
                src)
        }
        repoIDFor := func(ws workspace.WorkspaceKey) semantic.RepoID {
            return semantic.RepoID(ws.RepoRoot) // or whatever the existing repoID derivation is
        }

        liveService = live.NewService(
            coalescer.Config{
                DebounceMs:           cfg.SemanticIndex.LiveUpdates.DebounceMs,
                MaxBatchDelayMs:      cfg.SemanticIndex.LiveUpdates.MaxBatchDelayMs,
                BulkChangeThreshold:  cfg.SemanticIndex.LiveUpdates.BulkChangeThreshold,
                QueueSize:            1024,
            },
            h, classifierFn, repoIDFor, logger)

        // Wire kernel.SetEditNotifier (LIVE-07 invariant: kernel does NOT
        // import semantic; the interface lives in internal/kernel/notifier.go).
        k.SetEditNotifier(liveService)

        // Watcher + scanner managers (per-workspace lifecycle hooks below).
        if cfg.SemanticIndex.LiveUpdates.WatcherEnabled {
            watcherMgr = watcher.NewManager(liveService, watcher.Config{
                DebounceMs: cfg.SemanticIndex.LiveUpdates.DebounceMs,
            }, logger)
        }
        if cfg.SemanticIndex.LiveUpdates.ManifestScanEnabled {
            scannerMgr = scanner.NewManager(/* deps */)
        }
    }
    ```

    3) Inside the existing `SetActivateCallback` body (lines 463-496), AFTER `ScheduleInitialExtraction`, insert per-workspace lifecycle:

    ```go
    if liveService != nil {
        liveService.Start(ctx, workspace.WorkspaceKey{RepoRoot: repoPath})
    }
    if watcherMgr != nil {
        if err := watcherMgr.Start(ctx, workspace.WorkspaceKey{RepoRoot: repoPath}); err != nil {
            logger.Warn("live: watcher start", "err", err)
        }
    }
    if scannerMgr != nil {
        if err := scannerMgr.Start(ctx, workspace.WorkspaceKey{RepoRoot: repoPath}); err != nil {
            logger.Warn("live: scanner start", "err", err)
        }
    }
    ```

    4) Add the bounded-label metric registration. The metric vector itself is registered in `internal/obs/metrics.go` (Task 4 sub-step 4). The DAEMON wires the helper into the live handler/coalescer dispatch by passing `observability.Metrics()` into `NewService` if needed. If not needed (helper is callable from any code that has access to the global `*Metrics`), no daemon-side wiring required.

    5) Hash-file helper visibility: `scanner.HashFile` may need to be exported. If the current implementation is private (`hashFile`), add a public wrapper:

    ```go
    // HashFile is the package-level wrapper exposing the xxhash64 of a file
    // for use by the live classifier and other Phase 60 callers.
    func HashFile(absPath string) (string, error) { return hashFile(absPath) }
    ```

    6) Verify daemon builds and runs the full integration. Add a smoke test under `internal/daemon/daemon_live_smoke_test.go` (build tag `//go:build integration`) that constructs a daemon with `SemanticIndex.LiveUpdates.Enabled=true`, activates a tmpdir workspace, writes a file via `os.WriteFile`, waits up to 2× manifest_scan_interval, and asserts `semantic_live_overlay_files` has a row for that file.
  </action>
  <verify>
    <automated>cd $REPO_ROOT && go build ./... && go vet ./... && go test ./internal/phasegraph/... ./internal/daemon/... -count=1 && go install ./cmd/vet-noduckdb && go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./... && go install ./cmd/vet-nokernel2semantic && go vet -vettool=$(go env GOPATH)/bin/vet-nokernel2semantic ./...</automated>
  </verify>
  <done>
    pipelines/live.go has 9 real Run closures (or a constructor that builds them with wired components — document choice in SUMMARY); daemon constructs liveService, calls k.SetEditNotifier(liveService), Start watcher + scanner per workspace activation; vet-noduckdb and vet-nokernel2semantic both clean.
  </done>
</task>

<task type="checkpoint:human-verify" gate="blocking">
  <name>Task 4: Phase 60 close-out — REQUIREMENTS.md updates + end-to-end smoke verification</name>
  <files>.planning/REQUIREMENTS.md, .planning/ROADMAP.md</files>
  <action>
    All five plans of Phase 60 are merged. Schema v3 + overlay tx + epoch
    contract (P02). EditNotifier interface + 8-tool wiring (P03). Live spine:
    classifier + coalescer + handler + service + ScheduleIncremental fill
    (P04). Watcher + scanner + editor fixtures + metric + daemon wiring (P05).
    Analyzer enforcing kernel→semantic boundary (P01).

    Human-verify steps:

    1. Confirm all unit + integration tests pass:
       `go test ./... -race -count=1`
       Expected: exit 0.

    2. Confirm both vet-tools are green:
       `make vet`
       Expected: zero analyzer diagnostics.

    3. Confirm the editor-fixture suite passes:
       `go test ./internal/semantic/live/watcher/... -tags editor -count=1 -timeout 30s`
       Expected: exit 0; Vim test may skip on Windows; JetBrains + VS Code
       fixtures pass everywhere.

    4. End-to-end smoke: build the daemon, point it at a tmp workspace,
       call `replace_in_file` (or any of the 8 hooked tools) via the MCP
       inspector or `helix` CLI, and inspect the resulting DuckDB:
         a. Query `SELECT current_epoch FROM semantic_live_overlay_meta` — should be >= 1.
         b. Query `SELECT path, write_epoch FROM semantic_live_overlay_files` — should show the edited path.

    5. Edit `.planning/REQUIREMENTS.md` to flip LIVE-01 through LIVE-07
       from `- [ ]` to `- [x]`.

    6. Update `.planning/ROADMAP.md` Phase 60 row from "Not started" to "Complete"
       once all SUMMARYs are written.

    Resume signal: type "approved" if every check passed, or describe any
    failure (test name + error message + plan number) so the planner can
    issue a gap-closure plan.
  </action>
  <verify>
    <automated>grep -cE '^- \[x\] \*\*LIVE-0[1-7]\*\*' .planning/REQUIREMENTS.md</automated>
  </verify>
  <done>
    All four numbered checks above pass; REQUIREMENTS.md shows 7 LIVE-0x lines as `[x]`; ROADMAP.md Phase 60 row reads "Complete".
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Filesystem ↔ scanner | filepath.WalkDir traverses workspace; symlink rejection prevents escape. |
| Daemon bootstrap ↔ live service | Single trusted construction path; live.NewService is called once with internal-only inputs. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-60-05b-01 | Information Disclosure | Scanner walks symlink out of workspace and emits external paths | mitigate | scanner.Walk uses `d.Type()&fs.ModeSymlink != 0 → return nil` (skip). Tested by walk_test.go. |
| T-60-05b-02 | DoS | Manifest scanner thrashes disk on huge workspace every 10s | mitigate | Default interval 10s; default skipDirs filter (.git/node_modules/etc); workspace size scales the per-scan cost but the scanner is sequential and bounded. Tunable via config (`manifest_scan_interval`). |
| T-60-05b-03 | Repudiation | Daemon wires watcher + scanner before metric registration → events count in old vector | accept | Bootstrap order is sequential; metric registration is part of internal/obs init that runs before any per-workspace start. Risk only realizes if a future refactor reorders bootstrap. |
| T-60-05b-04 | DoS | A workspace activation spawns watcher + scanner + coalescer goroutines = 3 per workspace; N workspaces → 3N goroutines | mitigate | 3N is bounded by user-active workspace count (typically 1-3 per developer session). No goroutine leaks: Stop(ws) cancels all three. |
| T-60-05b-05 | Tampering | A malicious file content_hash collision (xxhash64) hides a real change from the scanner | accept | xxhash64 has ~2^32 collision resistance; deliberate collision requires adversarial content design that the threat model does not cover. The watcher (P05A) catches such changes regardless of hash. |
</threat_model>

<verification>
- `go test ./internal/semantic/live/scanner/... -count=1 -race -timeout 30s` — exit 0
- `go test ./internal/semantic/live/scanner/... -run TestScannerCatchesWatcherMisses -count=1 -timeout 30s` — exit 0
- `go test ./internal/obs/... -run TestSemanticLiveUpdates -count=1` — exit 0
- `go test ./internal/config/... -run TestLoad_LiveUpdatesDefaults -count=1` — exit 0
- `go test ./internal/phasegraph/... ./internal/daemon/... -count=1` — exit 0
- `go build ./...` — exit 0
- `make vet` — exit 0 (both vet-noduckdb and vet-nokernel2semantic)
- `grep -v '^//' internal/phasegraph/pipelines/live.go | grep -c 'noopRun'` — must be `0` (preferred form: every phase Run body in the SHIPPED `BuildLiveUpdatePhases` constructor — or the package-level `LiveUpdatePhases` var if the constructor approach is rejected — receives a real closure; the 9 `noopRun` placeholders are ALL replaced. Comment-only mentions are filtered out via `grep -v '^//'`. If any phase legitimately remains a typed no-op in this reactive DAG model, it MUST be enumerated by name with a justification in the plan SUMMARY's `<retained_noops>` section AND this verify line MUST be updated to pin the EXACT remaining count rather than `0`.)
- `grep -c 'SetEditNotifier(liveService)' internal/daemon/daemon.go` — 1
- `grep -E '^- \[x\] \*\*LIVE-0[1-7]\*\*' .planning/REQUIREMENTS.md | wc -l` — 7 (after Task 4 close-out)
</verification>

<success_criteria>
- [ ] Per-workspace manifest scanner ticking at default 10s, hashing via xxhash64, comparing against semantic_files.content_hash
- [ ] Manifest scanner detects watcher misses within 2× interval (acceptance #10)
- [ ] No `internal/repomap` import in `internal/semantic/live/scanner/`
- [ ] Three new config keys (`watcher_enabled`, `manifest_scan_enabled`, `manifest_scan_interval`) flow through 4-layer koanf precedence
- [ ] `helix_semantic_live_updates_total{kind, outcome}` registered with closed-enum drop-on-unknown helper
- [ ] `pipelines/live.go` Run bodies replace ALL 9 noopRun placeholders in the shipped constructor; `grep -v '^//' internal/phasegraph/pipelines/live.go | grep -c 'noopRun'` returns 0 (or the SUMMARY enumerates retained no-ops by name with justification and the verify pin is updated to the exact remaining count)
- [ ] Daemon bootstrap wires `kernel.SetEditNotifier(liveService)`, watcher manager (from P05A package), scanner manager
- [ ] Per-workspace Start in `SetActivateCallback`, gated on cfg
- [ ] LIVE-01..LIVE-07 marked `[x]` in REQUIREMENTS.md (Task 4 close-out)
- [ ] `make vet` clean (both analyzers)
</success_criteria>

<output>
After completion, create `.planning/phases/60-live-update-pipeline/60-05b-SUMMARY.md` documenting: scanner package layout, config-key wiring path, metric helper signature + drop-on-unknown coverage, the `pipelines/live.go` constructor (with `<retained_noops>` section if any phase intentionally ships a no-op body), the daemon bootstrap diff, REQUIREMENTS.md close-out, and the test counts per acceptance.
</output>
