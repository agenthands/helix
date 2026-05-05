---
phase: 60
plan: 05a
type: execute
wave: 3
depends_on: ["60-02", "60-03", "60-04"]
files_modified:
  - internal/semantic/live/watcher/watcher.go
  - internal/semantic/live/watcher/manager.go
  - internal/semantic/live/watcher/status.go
  - internal/semantic/live/watcher/enospc.go
  - internal/semantic/live/watcher/watcher_test.go
  - internal/semantic/live/watcher/enospc_test.go
  - internal/semantic/live/watcher/editor_fixtures_test.go
  - internal/semantic/live/testdata/editors/vim/save.sh
  - internal/semantic/live/testdata/editors/vim/README.md
  - internal/semantic/live/testdata/editors/jetbrains/save.go
  - internal/semantic/live/testdata/editors/jetbrains/README.md
  - internal/semantic/live/testdata/editors/vscode/save_atomic.go
  - internal/semantic/live/testdata/editors/vscode/save_truncate.go
  - internal/semantic/live/testdata/editors/vscode/README.md
autonomous: true
requirements: [LIVE-01, LIVE-02, LIVE-04]
must_haves:
  truths:
    - "Per-workspace fsnotify watcher catches Vim swap-rename, JetBrains ___jb_tmp___+rename, VS Code atomic-rename + truncate-write — verified by editor-fixture tests"
    - "Linux ENOSPC at fsnotify.Add triggers exactly one structured slog.Warn per workspace; Status() returns Active=false, Reason=inotify_enospc"
    - "Atomic-rename save patterns from Vim, JetBrains, and VS Code do not silently kill watching — verified by editor fixtures (LIVE-02)"
    - "JetBrains tempfile suffix filter (___jb_tmp___, ___jb_old___) drops noise at the watcher loop"
  artifacts:
    - path: internal/semantic/live/watcher/watcher.go
      provides: "Per-workspace fsnotify watcher loop with debounce, recursive add, JetBrains tempfile filter, atomic-rename re-attach"
      contains: "fsnotify.NewWatcher"
    - path: internal/semantic/live/watcher/enospc.go
      provides: "IsENOSPC helper, sync.Once-guarded slog.Warn"
      contains: "syscall.ENOSPC"
    - path: internal/semantic/live/watcher/status.go
      provides: "WatcherStatus + atomicStatus accessor (data accessor consumed by Phase 65 get_health)"
      contains: "type WatcherStatus"
    - path: internal/semantic/live/testdata/editors/jetbrains/save.go
      provides: "Go program reproducing the ___jb_tmp___ + ___jb_old___ rename sequence"
      contains: "___jb_tmp___"
    - path: internal/semantic/live/testdata/editors/vscode/save_atomic.go
      provides: "Go program reproducing VS Code atomic-rename save"
      contains: "os.Rename"
  key_links:
    - from: internal/semantic/live/watcher/watcher.go fsnotify.Errors
      to: enospc.go one-shot Warn
      via: "errors.Is(err, syscall.ENOSPC) + sync.Once"
      pattern: "syscall.ENOSPC"
    - from: editor_fixtures_test.go
      to: testdata/editors/{vim,jetbrains,vscode}/save*
      via: "os/exec under t.TempDir() against a live watcher"
      pattern: "exec\\.Command"
tags: [watcher, fsnotify, enospc, editor-fixtures, live-01, live-02, live-04]
---

<objective>
Ship the per-workspace fsnotify watcher (LIVE-01 producer side) with
ENOSPC fallback (LIVE-04) and the editor-fixture LIVE-02 test suite. This
is the watcher half of the original P05 — split out per checker WARNING-4
to keep the plan within sane context budget.

Sibling plan P05B ships the manifest scanner, config keys, bounded-label
metric, pipelines/live.go body fills, daemon bootstrap wiring, and the
phase close-out checkpoint. P05A and P05B share Wave 3 (both depend on
P02+P03+P04) and have ZERO files_modified overlap, so they may run
concurrently.

Dependencies:
  - P02 (Wave 1) — overlay tx + epoch contract (consumed via P04 service)
  - P03 (Wave 1) — kernel.EditNotifier (orthogonal; watcher feeds liveService.OnWorkspaceChanged, not OnEdit)
  - P04 (Wave 2) — liveService is the Producer this watcher feeds

Purpose: Make the Linux/macOS/Windows fsnotify producer + the LIVE-02
editor-fixture suite shippable in isolation without coupling to the
scanner / config / daemon-wiring scope.
Output: One sub-package (`internal/semantic/live/watcher/`), three editor
fixture programs (Vim shell, JetBrains Go, VS Code Go × 2 modes), and the
ENOSPC fallback contract verified by tests.
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
@internal/memory/watcher.go

<interfaces>
<!-- Live service public API from P04 (the consumer side this plan feeds) -->

```go
// internal/semantic/live/service.go
type Service struct { /* ... */ }
func (s *Service) OnWorkspaceChanged(ctx context.Context, sig WorkspaceChangeSignal) error
```

<!-- Memory watcher reference (verbatim shape, internal/memory/watcher.go:60-118) -->

```go
// Already in tree — Phase 60 mirrors structure but per-workspace, paths-only
// output, ENOSPC handling, JetBrains tempfile filter.
func (w *Watcher) Run(ctx context.Context) error {
    var (
        timer   *time.Timer
        pending = make(map[string]fsnotify.Op)
        mu      sync.Mutex
    )
    flush := func() { /* ... */ }
    for {
        select {
        case <-ctx.Done(): /* ... */ return ctx.Err()
        case event, ok := <-w.watcher.Events: /* ... */
        case err, ok := <-w.watcher.Errors: /* ... */
        }
    }
}

func (w *Watcher) addRecursive(dir string) error { /* filepath.Walk + Add */ }
```
</interfaces>
</context>

<tasks>

<task type="auto">
  <name>Task 1: fsnotify watcher manager + ENOSPC fallback + atomic-rename re-attach</name>
  <files>
    internal/semantic/live/watcher/watcher.go,
    internal/semantic/live/watcher/manager.go,
    internal/semantic/live/watcher/status.go,
    internal/semantic/live/watcher/enospc.go,
    internal/semantic/live/watcher/watcher_test.go,
    internal/semantic/live/watcher/enospc_test.go
  </files>
  <read_first>
    - internal/memory/watcher.go (entire file — exact debounce + recursive-add reference)
    - 60-RESEARCH.md "Code Example 1" (fsnotify directory watch with atomic-rename re-attach), Pitfalls 1, 2, 5
    - 60-PATTERNS.md "internal/semantic/live/watcher/watcher.go" + "watcher/enospc.go" sections
    - 60-CONTEXT.md D-05 (ENOSPC), acceptance criteria #9 (ENOSPC), #3 (editor fixtures)
    - go.mod (confirm fsnotify v1.9.0 already present at line 10)
  </read_first>
  <action>
    Create `internal/semantic/live/watcher/manager.go`:

    ```go
    // Package watcher ships the per-workspace fsnotify watcher manager. One
    // watcher goroutine per workspace; cross-workspace independence per
    // 60-CONTEXT.md D-05.
    package watcher

    import (
        "context"
        "log/slog"
        "sync"

        "github.com/agenthands/helix/internal/semantic/live"
        "github.com/agenthands/helix/internal/workspace"
    )

    type Producer interface {
        OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error
    }

    type Config struct {
        DebounceMs       time.Duration  // mirrors live_updates.debounce_ms
        IgnoreDirs       []string       // .git, node_modules, vendor, dist, build, target, coverage
        MaxFileSizeBytes int64
    }

    type Manager struct {
        cfg      Config
        producer Producer
        logger   *slog.Logger

        mu       sync.Mutex
        active   map[workspace.WorkspaceKey]*workspaceWatcher
    }

    func NewManager(p Producer, cfg Config, logger *slog.Logger) *Manager {
        if cfg.DebounceMs <= 0 { cfg.DebounceMs = 250 * time.Millisecond }
        if len(cfg.IgnoreDirs) == 0 {
            cfg.IgnoreDirs = []string{".git", "node_modules", "vendor", "dist", "build", "target", "coverage"}
        }
        return &Manager{cfg: cfg, producer: p, logger: logger,
            active: make(map[workspace.WorkspaceKey]*workspaceWatcher)}
    }

    func (m *Manager) Start(ctx context.Context, ws workspace.WorkspaceKey) error {
        m.mu.Lock()
        defer m.mu.Unlock()
        if _, ok := m.active[ws]; ok { return nil }
        ww, err := newWorkspaceWatcher(ws, m.cfg, m.producer, m.logger)
        if err != nil {
            return err
        }
        m.active[ws] = ww
        go ww.run(ctx)
        return nil
    }

    func (m *Manager) Stop(ws workspace.WorkspaceKey) {
        m.mu.Lock()
        ww := m.active[ws]
        delete(m.active, ws)
        m.mu.Unlock()
        if ww != nil { ww.close() }
    }

    func (m *Manager) Status(ws workspace.WorkspaceKey) WatcherStatus {
        m.mu.Lock()
        ww := m.active[ws]
        m.mu.Unlock()
        if ww == nil {
            return WatcherStatus{Active: false, Reason: "not_started"}
        }
        return ww.Status()
    }
    ```

    Create `internal/semantic/live/watcher/watcher.go` mirroring `internal/memory/watcher.go:60-118` shape; full body per 60-RESEARCH.md "Code Example 1":

    ```go
    package watcher

    import (
        "context"
        "errors"
        "log/slog"
        "os"
        "path/filepath"
        "strings"
        "sync"
        "syscall"
        "time"

        "github.com/fsnotify/fsnotify"

        "github.com/agenthands/helix/internal/semantic/live"
        "github.com/agenthands/helix/internal/workspace"
    )

    type workspaceWatcher struct {
        ws       workspace.WorkspaceKey
        cfg      Config
        producer Producer
        logger   *slog.Logger
        fw       *fsnotify.Watcher
        enospc   sync.Once
        status   atomicStatus

        mu       sync.Mutex
        timer    *time.Timer
        pending  map[string]struct{}
    }

    func newWorkspaceWatcher(ws workspace.WorkspaceKey, cfg Config, p Producer, logger *slog.Logger) (*workspaceWatcher, error) {
        fw, err := fsnotify.NewWatcher()
        if err != nil {
            // ENOSPC may surface here too on Linux per RESEARCH.
            if isENOSPC(err) {
                return nil, ErrInotifyENOSPC
            }
            return nil, err
        }
        ww := &workspaceWatcher{
            ws: ws, cfg: cfg, producer: p, logger: logger, fw: fw,
            pending: make(map[string]struct{}),
        }
        ww.status.Store(WatcherStatus{Active: true, Reason: "running"})
        if err := ww.addRecursive(ws.RepoRoot); err != nil {
            // partial-add: ENOSPC during recursive walk is the common Linux case.
            // Return success but stamp status; manifest scanner takes over.
            if isENOSPC(err) {
                ww.markENOSPC()
            } else {
                ww.logger.Warn("watcher: addRecursive partial", "err", err)
            }
        }
        return ww, nil
    }

    func (w *workspaceWatcher) addRecursive(root string) error {
        return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
            if err != nil { return nil }
            if d.IsDir() {
                if w.shouldSkipDir(d.Name()) { return filepath.SkipDir }
                if addErr := w.fw.Add(path); addErr != nil {
                    if isENOSPC(addErr) {
                        return addErr // bubble up so caller marks ENOSPC
                    }
                    w.logger.Warn("watcher: add", "path", path, "err", addErr)
                }
            }
            return nil
        })
    }

    func (w *workspaceWatcher) shouldSkipDir(name string) bool {
        for _, ig := range w.cfg.IgnoreDirs { if name == ig { return true } }
        return false
    }

    func (w *workspaceWatcher) run(ctx context.Context) {
        defer w.fw.Close()
        flush := w.makeFlush(ctx)
        for {
            select {
            case <-ctx.Done():
                w.mu.Lock()
                if w.timer != nil { w.timer.Stop() }
                w.mu.Unlock()
                return
            case ev, ok := <-w.fw.Events:
                if !ok { return }
                w.handleEvent(ev, flush)
            case err, ok := <-w.fw.Errors:
                if !ok { return }
                if isENOSPC(err) {
                    w.markENOSPC()
                } else {
                    w.logger.Warn("fsnotify error", "workspace", w.ws, "err", err)
                }
            }
        }
    }

    func (w *workspaceWatcher) handleEvent(ev fsnotify.Event, flush func()) {
        // Filter JetBrains tempfile suffixes — the rename will fire a
        // Create on the destination path next.
        if strings.HasSuffix(ev.Name, "___jb_tmp___") ||
            strings.HasSuffix(ev.Name, "___jb_old___") {
            return
        }
        // Skip events under ignore dirs (e.g. .git/refs/heads/x).
        for _, ig := range w.cfg.IgnoreDirs {
            if strings.Contains(ev.Name, string(filepath.Separator)+ig+string(filepath.Separator)) {
                return
            }
        }

        w.mu.Lock()
        w.pending[ev.Name] = struct{}{}
        if w.timer != nil { w.timer.Stop() }
        w.timer = time.AfterFunc(w.cfg.DebounceMs, flush)
        w.mu.Unlock()

        // Re-add directory if a Create on a subdir was observed.
        if ev.Has(fsnotify.Create) {
            if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
                _ = w.fw.Add(ev.Name)
            }
        }
    }

    func (w *workspaceWatcher) makeFlush(ctx context.Context) func() {
        return func() {
            w.mu.Lock()
            if len(w.pending) == 0 { w.mu.Unlock(); return }
            paths := make([]string, 0, len(w.pending))
            for p := range w.pending { paths = append(paths, p) }
            w.pending = make(map[string]struct{})
            w.mu.Unlock()

            sig := live.WorkspaceChangeSignal{
                WorkspaceID: w.ws,
                Paths:       paths,
                Source:      live.ChangeSourceFsnotify,
                ObservedAt:  time.Now(),
            }
            _ = w.producer.OnWorkspaceChanged(ctx, sig)
        }
    }

    func (w *workspaceWatcher) markENOSPC() {
        w.enospc.Do(func() {
            w.logger.Warn("fsnotify ENOSPC; falling back to manifest scan",
                "workspace", w.ws,
                "remediation", "echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf")
            w.status.Store(WatcherStatus{
                Active: false,
                Reason: "inotify_enospc",
                RemediationHint: "echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf",
            })
        })
    }

    func (w *workspaceWatcher) Status() WatcherStatus { return w.status.Load() }
    func (w *workspaceWatcher) close() { _ = w.fw.Close() }
    ```

    Create `internal/semantic/live/watcher/status.go`:

    ```go
    package watcher

    import "sync/atomic"

    // WatcherStatus is the data accessor consumed by Phase 65 get_health.
    type WatcherStatus struct {
        Active          bool
        Reason          string
        RemediationHint string
    }

    type atomicStatus struct{ v atomic.Value }
    func (a *atomicStatus) Store(s WatcherStatus) { a.v.Store(s) }
    func (a *atomicStatus) Load() WatcherStatus {
        v := a.v.Load()
        if v == nil { return WatcherStatus{} }
        return v.(WatcherStatus)
    }
    ```

    Create `internal/semantic/live/watcher/enospc.go`:

    ```go
    package watcher

    import (
        "errors"
        "syscall"
    )

    // ErrInotifyENOSPC is returned when fsnotify.NewWatcher() fails with
    // ENOSPC (Linux inotify watch-limit exhaustion).
    var ErrInotifyENOSPC = errors.New("inotify ENOSPC: fs.inotify.max_user_watches exhausted")

    // isENOSPC matches the unwrapped syscall error fsnotify v1.9.0 returns
    // from inotify_add_watch (see 60-RESEARCH.md Pitfall 2).
    func isENOSPC(err error) bool {
        return errors.Is(err, syscall.ENOSPC)
    }
    ```

    Create `internal/semantic/live/watcher/enospc_test.go`:

    ```go
    package watcher

    import (
        "errors"
        "syscall"
        "testing"
    )

    func TestIsENOSPC_DirectErrno(t *testing.T) {
        if !isENOSPC(syscall.ENOSPC) { t.Fatal("direct ENOSPC not matched") }
    }

    func TestIsENOSPC_WrappedErrno(t *testing.T) {
        wrapped := &someError{cause: syscall.ENOSPC}
        if !isENOSPC(wrapped) { t.Fatal("wrapped ENOSPC not matched via errors.Is") }
    }

    type someError struct{ cause error }
    func (e *someError) Error() string { return "outer: " + e.cause.Error() }
    func (e *someError) Unwrap() error { return e.cause }

    func TestENOSPCFallback_StatusFlips(t *testing.T) {
        // Simulate the markENOSPC path by constructing a workspaceWatcher
        // with a fake fsnotify.Watcher (or skip fsnotify entirely and
        // exercise markENOSPC directly).
        ww := &workspaceWatcher{logger: testLogger(t)}
        ww.status.Store(WatcherStatus{Active: true, Reason: "running"})
        ww.markENOSPC()
        ww.markENOSPC() // second call must NOT re-log (sync.Once)
        s := ww.Status()
        if s.Active { t.Fatal("expected Active=false after ENOSPC") }
        if s.Reason != "inotify_enospc" { t.Fatalf("Reason = %q, want inotify_enospc", s.Reason) }
        // Asserting "exactly one slog.Warn" requires capturing slog output;
        // use slog.NewTextHandler with a bytes.Buffer and grep for the
        // remediation string. Per 60-CONTEXT.md acceptance #9.
    }
    ```

    Create `internal/semantic/live/watcher/watcher_test.go` covering:
      - TestWorkspaceWatcher_DebounceCoalesces (3 events in 50ms → 1 OnWorkspaceChanged after debounce)
      - TestWorkspaceWatcher_JetBrainsTempfileFiltered (Create on `___jb_tmp___` does NOT enter pending)
      - TestWorkspaceWatcher_IgnoreDirs (Write under .git/ does NOT enter pending)
      - TestWorkspaceWatcher_ProducerReceivesPathsOnly (signal has Source=fsnotify, Paths populated, no Kind)
  </action>
  <verify>
    <automated>cd $REPO_ROOT && go build ./internal/semantic/live/watcher/... && go test ./internal/semantic/live/watcher/... -count=1 && go vet ./... && go install ./cmd/vet-noduckdb && go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./internal/semantic/live/...</automated>
  </verify>
  <done>
    Watcher manager + per-workspace watcher loop compile; ENOSPC test green; debounce + JetBrains-suffix-filter + ignore-dirs covered; vet-noduckdb still clean.
  </done>
</task>

<task type="auto">
  <name>Task 2: Editor-fixture LIVE-02 test suite (Vim, JetBrains, VS Code)</name>
  <files>
    internal/semantic/live/watcher/editor_fixtures_test.go,
    internal/semantic/live/testdata/editors/vim/save.sh,
    internal/semantic/live/testdata/editors/vim/README.md,
    internal/semantic/live/testdata/editors/jetbrains/save.go,
    internal/semantic/live/testdata/editors/jetbrains/README.md,
    internal/semantic/live/testdata/editors/vscode/save_atomic.go,
    internal/semantic/live/testdata/editors/vscode/save_truncate.go,
    internal/semantic/live/testdata/editors/vscode/README.md
  </files>
  <read_first>
    - 60-RESEARCH.md "Editor Save-Pattern Dossier" (lines 626-700) — verbatim event sequences for Vim, JetBrains, VS Code
    - 60-VALIDATION.md acceptance criterion #3 (editor fixtures)
    - 60-CONTEXT.md acceptance #3
    - the watcher loop you wrote in Task 1
  </read_first>
  <action>
    Create the three fixture programs. Each is small (~30-50 lines) and self-contained.

    `testdata/editors/vim/save.sh` — bash script that simulates Vim's swap-rename via shell `mv` operations (skipped on Windows). Per RESEARCH.md, Vim's default `backupcopy=auto` produces RENAME(orig) + CREATE(newcontent) sequence:

    ```bash
    #!/usr/bin/env bash
    # Simulates Vim's default 'backupcopy=auto' rename-on-save sequence.
    # Usage: save.sh <target-file> <new-content>
    set -euo pipefail
    target="$1"
    content="$2"
    dir="$(dirname "$target")"
    base="$(basename "$target")"
    tmp="$dir/.${base}.tmp.$$"
    backup="$dir/${base}~"
    printf '%s' "$content" > "$tmp"
    [ -f "$target" ] && mv "$target" "$backup" || true
    mv "$tmp" "$target"
    rm -f "$backup"
    ```

    `testdata/editors/jetbrains/save.go` — Go program. Reproduces the four-step sequence from RESEARCH.md (CREATE __jb_tmp__, RENAME orig → __jb_old__, RENAME __jb_tmp__ → orig, REMOVE __jb_old__):

    ```go
    // Command save reproduces JetBrains IDE's "safe write" save sequence
    // verbatim: write to ___jb_tmp___, rename original to ___jb_old___,
    // rename tmp to original, delete old.
    //
    // Usage: go run save.go <target-file> <new-content>
    package main

    import (
        "fmt"
        "os"
    )

    func main() {
        if len(os.Args) != 3 { fmt.Fprintln(os.Stderr, "usage: save <target> <content>"); os.Exit(2) }
        target, content := os.Args[1], os.Args[2]
        tmp, old := target+"___jb_tmp___", target+"___jb_old___"
        if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil { panic(err) }
        if _, err := os.Stat(target); err == nil {
            if err := os.Rename(target, old); err != nil { panic(err) }
        }
        if err := os.Rename(tmp, target); err != nil { panic(err) }
        _ = os.Remove(old)
    }
    ```

    `testdata/editors/vscode/save_atomic.go` — Go program reproducing VS Code's atomic-rename save (when `files.atomicSave: true`):

    ```go
    // Command save_atomic reproduces VS Code's atomic-rename save: write
    // content to a sibling temp file, then rename onto the target.
    //
    // Usage: go run save_atomic.go <target-file> <new-content>
    package main

    import (
        "fmt"
        "os"
        "path/filepath"
    )

    func main() {
        if len(os.Args) != 3 { fmt.Fprintln(os.Stderr, "usage: save_atomic <target> <content>"); os.Exit(2) }
        target, content := os.Args[1], os.Args[2]
        tmp, err := os.CreateTemp(filepath.Dir(target), ".vsctmp~*")
        if err != nil { panic(err) }
        if _, err := tmp.WriteString(content); err != nil { panic(err) }
        if err := tmp.Close(); err != nil { panic(err) }
        if err := os.Rename(tmp.Name(), target); err != nil { panic(err) }
    }
    ```

    `testdata/editors/vscode/save_truncate.go` — Go program reproducing the default truncate-then-write path:

    ```go
    // Command save_truncate reproduces VS Code's default O_TRUNC save:
    // open the target with O_TRUNC|O_WRONLY|O_CREATE, write, close.
    //
    // Usage: go run save_truncate.go <target-file> <new-content>
    package main

    import (
        "fmt"
        "os"
    )

    func main() {
        if len(os.Args) != 3 { fmt.Fprintln(os.Stderr, "usage: save_truncate <target> <content>"); os.Exit(2) }
        target, content := os.Args[1], os.Args[2]
        f, err := os.OpenFile(target, os.O_TRUNC|os.O_WRONLY|os.O_CREATE, 0o644)
        if err != nil { panic(err) }
        if _, err := f.WriteString(content); err != nil { panic(err) }
        if err := f.Close(); err != nil { panic(err) }
    }
    ```

    Each `README.md` documents what the program reproduces (with citation back to 60-RESEARCH.md "Editor Save-Pattern Dossier").

    Create `internal/semantic/live/watcher/editor_fixtures_test.go` (build tag `//go:build editor` to keep it out of the default test pass per 60-VALIDATION.md):

    ```go
    //go:build editor
    // +build editor

    package watcher_test

    import (
        "context"
        "os"
        "os/exec"
        "path/filepath"
        "runtime"
        "testing"
        "time"

        "github.com/agenthands/helix/internal/semantic/live"
        "github.com/agenthands/helix/internal/semantic/live/watcher"
        "github.com/agenthands/helix/internal/workspace"
    )

    type recordingProducer struct {
        ch chan live.WorkspaceChangeSignal
    }
    func (r *recordingProducer) OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error {
        select { case r.ch <- sig: default: }
        return nil
    }

    func setupWatcher(t *testing.T) (string, *recordingProducer, func()) {
        t.Helper()
        dir := t.TempDir()
        target := filepath.Join(dir, "auth.go")
        os.WriteFile(target, []byte("package main\n"), 0o644)

        rp := &recordingProducer{ch: make(chan live.WorkspaceChangeSignal, 8)}
        mgr := watcher.NewManager(rp, watcher.Config{DebounceMs: 100 * time.Millisecond}, testLogger(t))
        ws := workspace.WorkspaceKey{RepoRoot: dir}
        ctx, cancel := context.WithCancel(context.Background())
        if err := mgr.Start(ctx, ws); err != nil { t.Fatalf("Start: %v", err) }
        return target, rp, func() { cancel(); mgr.Stop(ws) }
    }

    func TestEditorFixtures_VimSwapRename(t *testing.T) {
        if _, err := exec.LookPath("bash"); err != nil {
            t.Skip("bash not on PATH (Windows CI)")
        }
        target, rp, teardown := setupWatcher(t)
        defer teardown()
        cmd := exec.Command("bash", "internal/semantic/live/testdata/editors/vim/save.sh", target, "package main\n// modified\n")
        if err := cmd.Run(); err != nil { t.Fatalf("vim save: %v", err) }
        select {
        case sig := <-rp.ch:
            if sig.Source != live.ChangeSourceFsnotify { t.Fatalf("source=%v", sig.Source) }
            if !pathInList(sig.Paths, target) { t.Fatalf("paths=%v missing %s", sig.Paths, target) }
        case <-time.After(2 * time.Second):
            t.Fatal("watcher did not emit signal within 2s")
        }
    }

    // TestEditorFixtures_JetBrainsSafeWrite (uses jetbrains/save.go via go run)
    // TestEditorFixtures_VSCodeAtomic       (uses vscode/save_atomic.go)
    // TestEditorFixtures_VSCodeTruncate     (uses vscode/save_truncate.go)
    // — same shape; replace the cmd construction.
    ```

    Provide the testLogger helper (one-line slog.Default returner) and pathInList helper. The Vim test requires `bash` on PATH; on Windows CI it skips. The JetBrains and VS Code tests are pure Go and run on every platform.
  </action>
  <verify>
    <automated>cd $REPO_ROOT && go test ./internal/semantic/live/watcher/... -tags editor -count=1 -timeout 30s</automated>
  </verify>
  <done>
    Three editor fixture programs exist; editor_fixtures_test.go runs each program against a real fsnotify watcher in a tmpdir; signals reach the producer with Source=fsnotify and Paths containing the target.
  </done>
</task>

</tasks>

<threat_model>
## Trust Boundaries

| Boundary | Description |
|----------|-------------|
| Filesystem ↔ watcher | fsnotify reports kernel events for any path under watched dirs; ENOSPC + atomic-rename are the dominant adversarial cases. |
| Editor fixtures ↔ test workspace | `os/exec` runs Go programs (jetbrains/vscode) and bash scripts (vim) within a `t.TempDir()`; no privileged operations. |

## STRIDE Threat Register

| Threat ID | Category | Component | Disposition | Mitigation Plan |
|-----------|----------|-----------|-------------|-----------------|
| T-60-05a-01 | Information Disclosure | Watcher follows symlink out of workspace and emits external paths | mitigate | watcher relies on fsnotify's per-OS behavior — fsnotify on Linux does NOT follow symlinks by default. EXECUTOR DECISION: confirm via test that `Add(workspaceRoot)` does not resolve symlinks; if it does, add explicit symlink rejection in addRecursive. |
| T-60-05a-02 | Denial of Service | Watcher floods (silent missed events under ENOSPC) | mitigate | sync.Once-guarded slog.Warn (Pitfall 2 mitigation); manifest scanner (P05B) takes over correctness. Tested by enospc_test.go — exactly one Warn even under repeated ENOSPC errors. |
| T-60-05a-03 | Information Disclosure | Log-spam DoS via repeated ENOSPC slog.Warn | mitigate | Same as T-60-05a-02. Single sync.Once per workspace per watcher lifetime. |
| T-60-05a-04 | Tampering | Editor fixture program creates files outside tmpdir | mitigate | Fixture programs accept `<target-file>` as argv[1]; tests pass `t.TempDir()`-rooted paths only. The programs do `os.Rename` to the target — they cannot escape the directory they were given. |
| T-60-05a-05 | DoS | The fsnotify Add() on a recursive walk eats the entire inotify budget on a workspace with millions of dirs | mitigate | ENOSPC fallback path (Pitfall 2) catches this; manifest scanner (P05B) remains correct. Phase 65 get_health surfaces the degraded state. |
</threat_model>

<verification>
- `go test ./internal/semantic/live/watcher/... -count=1 -race` — exit 0
- `go test ./internal/semantic/live/watcher/... -tags editor -count=1 -timeout 30s` — exit 0 (Vim test may skip on Windows / hosts without bash)
- `go build ./...` — exit 0
- `go install ./cmd/vet-noduckdb && go vet -vettool=$(go env GOPATH)/bin/vet-noduckdb ./internal/semantic/live/watcher/...` — exit 0
- `grep -c 'syscall.ENOSPC' internal/semantic/live/watcher/enospc.go` — ≥ 1
- `grep -c '___jb_tmp___' internal/semantic/live/watcher/watcher.go` — ≥ 1 (JetBrains tempfile filter present)
</verification>

<success_criteria>
- [ ] Per-workspace fsnotify watcher with debounce, recursive add, JetBrains tempfile filter
- [ ] ENOSPC handling: one slog.Warn per workspace, Status() returns Active=false, Reason="inotify_enospc"
- [ ] Three editor-fixture programs (Vim shell, JetBrains Go, VS Code Go × 2 modes); editor-tagged test passes
- [ ] WatcherStatus data accessor exposed for Phase 65 get_health consumption
- [ ] No `internal/semantic/store` import in `internal/semantic/live/watcher/`
- [ ] vet-noduckdb clean
</success_criteria>

<output>
After completion, create `.planning/phases/60-live-update-pipeline/60-05a-SUMMARY.md` documenting: watcher package layout, ENOSPC fallback contract (one-shot Warn + Status flip), JetBrains/VS Code/Vim fixture-suite outcomes (per-platform pass/skip table), any fsnotify v1.9.0 quirks observed, and the test counts per acceptance.
</output>
