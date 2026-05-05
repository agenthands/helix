package watcher

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/workspace"
)

// JetBrains "safe write" save sequence drops these two suffixes onto
// the watch directory before the actual rename onto the target file.
// 60-RESEARCH.md "Editor Save-Pattern Dossier" verifies the sequence
// verbatim. Filtering at the watcher loop avoids feeding noise paths
// into the classifier (D-01 paths-only invariant) and keeps the
// downstream coalescer's pending map small.
const (
	jbTempSuffix = "___jb_tmp___"
	jbOldSuffix  = "___jb_old___"
)

// workspaceWatcher owns one fsnotify.Watcher and one event-loop
// goroutine per workspace. The struct is constructed by Manager.Start
// and never escapes the Manager — its lifecycle is bound to the ctx
// passed into run().
type workspaceWatcher struct {
	ws       workspace.WorkspaceKey
	cfg      Config
	producer Producer
	logger   *slog.Logger
	fw       *fsnotify.Watcher

	enospc sync.Once    // guards markENOSPC slog.Warn (Pitfall 2)
	status atomicStatus // accessor for Phase 65 get_health

	mu      sync.Mutex
	timer   *time.Timer
	pending map[string]struct{} // path-set; D-01 paths-only
}

// newWorkspaceWatcher constructs the per-workspace watcher and seeds
// the recursive-add walk. ENOSPC at fsnotify.NewWatcher() is surfaced
// as ErrInotifyENOSPC; ENOSPC during the recursive walk is captured
// internally and the watcher is returned in a "degraded but started"
// state (Status().Active == false, Reason == "inotify_enospc"). The
// degraded mode is intentional — the manifest scanner (60-05B) takes
// over correctness, and the Manager's caller (60-05B daemon wiring)
// can still issue Stop() to clean up the partial fsnotify handle.
func newWorkspaceWatcher(
	ws workspace.WorkspaceKey,
	cfg Config,
	producer Producer,
	logger *slog.Logger,
) (*workspaceWatcher, error) {
	fw, err := fsnotify.NewWatcher()
	if err != nil {
		// ENOSPC may surface here too on Linux when inotify_init1
		// hits the per-user instance limit, distinct from the
		// per-instance watch limit hit by Add().
		if isENOSPC(err) {
			return nil, ErrInotifyENOSPC
		}
		return nil, err
	}
	ww := &workspaceWatcher{
		ws:       ws,
		cfg:      cfg,
		producer: producer,
		logger:   logger,
		fw:       fw,
		pending:  make(map[string]struct{}),
	}
	ww.status.Store(WatcherStatus{Active: true, Reason: "running"})

	if err := ww.addRecursive(ws.RepoRoot); err != nil {
		// Partial-add: ENOSPC during recursive walk is the common
		// Linux failure mode (Pitfall 2). The watcher is left in
		// place — degraded — and Status() reflects the fallback so
		// Phase 65 get_health can surface it. Non-ENOSPC walk errors
		// (permission, missing root) are logged once and the
		// watcher continues with whatever subset of dirs it managed
		// to register.
		if isENOSPC(err) {
			ww.markENOSPC()
		} else {
			ww.logger.Warn("watcher: addRecursive partial",
				"workspace", ws, "err", err)
		}
	}
	return ww, nil
}

// addRecursive walks root and registers every directory with the
// fsnotify backend. ENOSPC is bubbled up via the WalkDir return so
// the caller (newWorkspaceWatcher) can flip status to degraded;
// other per-path errors are logged at Warn but do not abort the walk
// (matches internal/memory/watcher.go:177-189).
func (w *workspaceWatcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			// Skip inaccessible entries — matches the memory watcher's
			// permissive "skip and continue" policy. A workspace under
			// a partially-readable mount should still produce events
			// for the readable subset.
			return nil
		}
		// Reject symlinks (T-60-04-03 / T-60-05b-01 invariant #6: both
		// watcher and scanner skip symlinks). A symlinked directory
		// reported by WalkDir carries fs.ModeSymlink even though
		// d.IsDir() returns true for the resolved target — without
		// this check fw.Add would register watches on out-of-workspace
		// targets via the linked path.
		if d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		if !d.IsDir() {
			return nil
		}
		if w.shouldSkipDir(d.Name()) {
			return filepath.SkipDir
		}
		if addErr := w.fw.Add(path); addErr != nil {
			if isENOSPC(addErr) {
				// Bubble up — caller flips ENOSPC status and
				// remaining dirs in this walk are intentionally
				// skipped (the inotify table is full anyway).
				return addErr
			}
			w.logger.Warn("watcher: add", "path", path, "err", addErr)
		}
		return nil
	})
}

// shouldSkipDir tests whether name is in the configured ignore list.
// Cheap O(N) scan over a small N (~7 entries); converting to a map
// would cost more in allocation than it saves.
func (w *workspaceWatcher) shouldSkipDir(name string) bool {
	for _, ig := range w.cfg.IgnoreDirs {
		if name == ig {
			return true
		}
	}
	return false
}

// run drives the event loop. Returns when ctx is cancelled OR when
// the fsnotify Events/Errors channels close (which indicates fw.Close
// was invoked from another goroutine, typically Manager.Stop).
//
// The deferred fw.Close() is idempotent — a second Close after Stop
// returns ErrClosed which we discard.
func (w *workspaceWatcher) run(ctx context.Context) {
	defer func() {
		_ = w.fw.Close()
	}()

	flush := w.makeFlush(ctx)

	for {
		select {
		case <-ctx.Done():
			w.mu.Lock()
			if w.timer != nil {
				w.timer.Stop()
			}
			w.mu.Unlock()
			w.status.Store(WatcherStatus{Active: false, Reason: "closed"})
			return

		case ev, ok := <-w.fw.Events:
			if !ok {
				w.status.Store(WatcherStatus{Active: false, Reason: "closed"})
				return
			}
			w.handleEvent(ev, flush)

		case err, ok := <-w.fw.Errors:
			if !ok {
				w.status.Store(WatcherStatus{Active: false, Reason: "closed"})
				return
			}
			if isENOSPC(err) {
				w.markENOSPC()
				continue
			}
			w.logger.Warn("fsnotify error",
				"workspace", w.ws, "err", err)
		}
	}
}

// handleEvent applies the JetBrains-tempfile filter, the ignore-dir
// path filter, then enqueues the path into the pending set and resets
// the debounce timer. A Create on a subdirectory is also re-Added so
// nested writes are not silently missed (matches
// internal/memory/watcher.go:93-98).
func (w *workspaceWatcher) handleEvent(ev fsnotify.Event, flush func()) {
	// JetBrains "safe write" filter — the rename onto the destination
	// path will fire a separate Create/Write that the loop will pick
	// up on the next iteration. Dropping the suffix paths here keeps
	// the pending map sized to "real" file paths only.
	if strings.HasSuffix(ev.Name, jbTempSuffix) ||
		strings.HasSuffix(ev.Name, jbOldSuffix) {
		return
	}

	// Drop events that fall under one of the ignore dirs (e.g.,
	// .git/refs/heads/main being mutated by `git checkout`). The check
	// is path-segment-based to avoid false positives on filenames that
	// happen to contain ".git" as a substring.
	if w.eventInIgnoredDir(ev.Name) {
		return
	}

	w.mu.Lock()
	w.pending[ev.Name] = struct{}{}
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.cfg.DebounceMs, flush)
	w.mu.Unlock()

	// Re-Add directory if a Create on a subdir was observed. The
	// fsnotify backend does NOT recurse into newly-created subdirs by
	// default; without this hook a `mkdir -p sub/dir` followed by a
	// write inside sub/dir would be silently missed. Use Lstat (not
	// Stat) and reject symlinks so a Create on a symlinked directory
	// does NOT add an out-of-workspace watch (invariant #6 mirror of
	// addRecursive's symlink check).
	if ev.Has(fsnotify.Create) {
		if info, err := os.Lstat(ev.Name); err == nil &&
			info.IsDir() && info.Mode()&os.ModeSymlink == 0 {
			_ = w.fw.Add(ev.Name)
		}
	}
}

// eventInIgnoredDir returns true if name lies under one of the
// configured ignore dirs. Path-segment matching avoids false positives
// on substrings (e.g., "vendor.go" should NOT match "vendor"). The
// scan also matches "/.git" at the start of a relative path, since
// filepath.Separator+ig+filepath.Separator on its own would miss that
// case on POSIX systems where the workspace root has no trailing
// separator.
func (w *workspaceWatcher) eventInIgnoredDir(name string) bool {
	sep := string(filepath.Separator)
	for _, ig := range w.cfg.IgnoreDirs {
		needle := sep + ig + sep
		if strings.Contains(name, needle) {
			return true
		}
		// Catch the path-prefix case (".git/HEAD" relative to root).
		if strings.HasSuffix(name, sep+ig) || name == ig {
			return true
		}
	}
	return false
}

// makeFlush builds the closure handed to time.AfterFunc. Captures
// ctx so the producer call has a parent context to chain off of —
// the producer itself is fire-and-forget, but a cancelled context
// lets the downstream classifier short-circuit lookups.
func (w *workspaceWatcher) makeFlush(ctx context.Context) func() {
	return func() {
		w.mu.Lock()
		if len(w.pending) == 0 {
			w.mu.Unlock()
			return
		}
		paths := make([]string, 0, len(w.pending))
		for p := range w.pending {
			paths = append(paths, p)
		}
		w.pending = make(map[string]struct{})
		w.mu.Unlock()

		sig := live.WorkspaceChangeSignal{
			WorkspaceID: w.ws,
			Paths:       paths,
			Source:      live.ChangeSourceFsnotify,
			ObservedAt:  time.Now(),
		}
		// Producer is fire-and-forget — Service.OnWorkspaceChanged
		// returns nil even on classifier errors (logs internally).
		// Discarding the error here matches the kernel-side OnEdit
		// callsite in 60-03 edit/fileops tools.
		_ = w.producer.OnWorkspaceChanged(ctx, sig)
	}
}

// markENOSPC is the sync.Once-guarded slog.Warn + status flip for the
// inotify ENOSPC fallback (LIVE-04, Pitfall 2). The Once ensures
// repeated ENOSPC errors during a recursive add (every dir's Add
// returns the same errno) emit exactly one log line per workspace
// per watcher lifetime.
func (w *workspaceWatcher) markENOSPC() {
	w.enospc.Do(func() {
		w.logger.Warn("fsnotify ENOSPC; falling back to manifest scan",
			"workspace", w.ws,
			"remediation", "echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf")
		w.status.Store(WatcherStatus{
			Active:          false,
			Reason:          "inotify_enospc",
			RemediationHint: "echo fs.inotify.max_user_watches=524288 | sudo tee -a /etc/sysctl.conf",
		})
	})
}

// Status returns the latest WatcherStatus snapshot. Cheap atomic
// load; safe to call from any goroutine.
func (w *workspaceWatcher) Status() WatcherStatus {
	return w.status.Load()
}

// close stops the timer and closes the fsnotify backend. Called from
// Manager.Stop; the run loop will then observe the Events/Errors
// channel close and return. Errors from fw.Close() are discarded
// because there is nothing meaningful for the caller to do — the
// watcher is being torn down.
func (w *workspaceWatcher) close() {
	w.mu.Lock()
	if w.timer != nil {
		w.timer.Stop()
		w.timer = nil
	}
	w.mu.Unlock()
	_ = w.fw.Close()
}
