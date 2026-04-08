package memory

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher monitors memory directories for file changes and auto-updates the
// SQLite FTS5 index. It debounces events to avoid redundant rebuilds.
type Watcher struct {
	index      *Index
	projectDir string
	globalDir  string
	watcher    *fsnotify.Watcher
	logger     *slog.Logger
	debounce   time.Duration // default 300ms per Pitfall 2
}

// NewWatcher creates a Watcher that monitors projectDir and globalDir for
// markdown file changes. The default debounce window is 300ms.
func NewWatcher(index *Index, projectDir, globalDir string, logger *slog.Logger) (*Watcher, error) {
	if logger == nil {
		logger = slog.Default()
	}

	fw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		index:      index,
		projectDir: projectDir,
		globalDir:  globalDir,
		watcher:    fw,
		logger:     logger,
		debounce:   300 * time.Millisecond,
	}

	// Add directories to watcher (including subdirectories).
	for _, dir := range []string{projectDir, globalDir} {
		if dir != "" {
			if err := w.addRecursive(dir); err != nil {
				fw.Close()
				return nil, err
			}
		}
	}

	return w, nil
}

// Run blocks and processes file events until the context is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	var (
		timer   *time.Timer
		pending = make(map[string]fsnotify.Op)
		mu      sync.Mutex
	)

	flush := func() {
		mu.Lock()
		batch := pending
		pending = make(map[string]fsnotify.Op)
		mu.Unlock()

		for path, op := range batch {
			w.processEvent(path, op)
		}
	}

	for {
		select {
		case <-ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return ctx.Err()

		case event, ok := <-w.watcher.Events:
			if !ok {
				return nil
			}
			if !strings.HasSuffix(event.Name, ".md") {
				// Watch for new directories to add them recursively.
				if event.Has(fsnotify.Create) {
					if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
						w.addRecursive(event.Name)
					}
				}
				continue
			}

			mu.Lock()
			pending[event.Name] = event.Op
			mu.Unlock()

			// Reset debounce timer.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(w.debounce, flush)

		case err, ok := <-w.watcher.Errors:
			if !ok {
				return nil
			}
			w.logger.Warn("watcher error", "error", err)
		}
	}
}

// Close stops the underlying fsnotify watcher.
func (w *Watcher) Close() error {
	return w.watcher.Close()
}

// processEvent handles a single debounced file event.
func (w *Watcher) processEvent(path string, op fsnotify.Op) {
	name, scope := w.nameFromPath(path)
	if name == "" {
		return
	}

	if op.Has(fsnotify.Remove) || op.Has(fsnotify.Rename) {
		if err := w.index.Remove(name); err != nil {
			w.logger.Warn("watcher: failed to remove from index", "name", name, "error", err)
		} else {
			w.logger.Info("watcher: removed from index", "name", name)
		}
		return
	}

	// Create or Write: read file and upsert.
	data, err := os.ReadFile(path)
	if err != nil {
		w.logger.Warn("watcher: failed to read file", "path", path, "error", err)
		return
	}

	if err := w.index.Upsert(name, path, scope, string(data)); err != nil {
		w.logger.Warn("watcher: failed to upsert index", "name", name, "error", err)
	} else {
		w.logger.Info("watcher: indexed", "name", name)
	}
}

// nameFromPath converts an absolute file path to a memory name and scope.
func (w *Watcher) nameFromPath(path string) (name, scope string) {
	// Try global dir first.
	if w.globalDir != "" {
		if rel, err := filepath.Rel(w.globalDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			name = strings.TrimSuffix(rel, ".md")
			name = filepath.ToSlash(name)
			return "global/" + name, "global"
		}
	}
	// Try project dir.
	if w.projectDir != "" {
		if rel, err := filepath.Rel(w.projectDir, path); err == nil && !strings.HasPrefix(rel, "..") {
			name = strings.TrimSuffix(rel, ".md")
			name = filepath.ToSlash(name)
			return name, "project"
		}
	}
	return "", ""
}

// addRecursive adds a directory and all its subdirectories to the fsnotify watcher.
func (w *Watcher) addRecursive(dir string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() {
			if err := w.watcher.Add(path); err != nil {
				w.logger.Warn("watcher: failed to watch dir", "path", path, "error", err)
			}
		}
		return nil
	})
}
