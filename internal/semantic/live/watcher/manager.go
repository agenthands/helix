package watcher

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/workspace"
)

// Producer is the consumer-defined interface satisfied by
// internal/semantic/live/service.Service.OnWorkspaceChanged. The
// watcher manager holds a Producer reference and feeds it
// fire-and-forget WorkspaceChangeSignal values; classification, per-
// workspace coalescing, and overlay dispatch all live downstream
// (60-04 spine).
//
// Defining the interface in this package (rather than importing the
// concrete *service.Service) keeps the import direction one-way and
// makes the watcher trivially testable with a recordingProducer fake
// (see watcher_test.go and editor_fixtures_test.go).
type Producer interface {
	OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error
}

// Config holds the per-Manager behavioural knobs. A zero-valued Config
// is illegal at run-time; constructors apply DefaultConfig() over any
// unset fields.
type Config struct {
	// DebounceMs is the time.Duration the per-workspace watcher waits
	// after the last fsnotify event before flushing the pending path-
	// set to the Producer. Mirrors live_updates.debounce_ms in
	// internal/config/defaults.go (60-05B owns the koanf binding).
	DebounceMs time.Duration

	// IgnoreDirs is the set of directory basenames the recursive
	// walker SkipDirs and the event loop drops events under. Closed
	// list to keep the producer side cheap; LIVE-03's manifest scanner
	// re-validates with the same set.
	IgnoreDirs []string

	// MaxFileSizeBytes is reserved for future use — the watcher does
	// not stat sizes today, but the Config carries the field so 60-05B
	// can wire the binding without API churn.
	MaxFileSizeBytes int64
}

// DefaultConfig returns the Manager defaults applied when Config
// fields are zero. Kept exported so tests and 60-05B's daemon wiring
// can compose against the same source-of-truth.
//
//   - DebounceMs: 250ms (CONTEXT D-02 nominal; see SPEC §16.2).
//   - IgnoreDirs: .git, node_modules, vendor, dist, build, target,
//     coverage — the same closed list the manifest scanner reuses.
func DefaultConfig() Config {
	return Config{
		DebounceMs: 250 * time.Millisecond,
		IgnoreDirs: []string{
			".git", "node_modules", "vendor",
			"dist", "build", "target", "coverage",
		},
	}
}

// applyDefaults fills in any unset fields on cfg from DefaultConfig().
// Returns the patched config rather than mutating the receiver.
func applyDefaults(cfg Config) Config {
	def := DefaultConfig()
	if cfg.DebounceMs <= 0 {
		cfg.DebounceMs = def.DebounceMs
	}
	if len(cfg.IgnoreDirs) == 0 {
		cfg.IgnoreDirs = def.IgnoreDirs
	}
	return cfg
}

// Manager owns the per-workspace registry of fsnotify goroutines.
// Manager is safe for concurrent Start/Stop/Status calls — the
// internal map is guarded by a sync.Mutex; long-running event-loop
// work happens off-lock inside the per-workspace goroutine.
//
// The Manager intentionally does not hold an ambient context — each
// Start call provides a context that scopes the goroutine's lifetime,
// matching the pattern used by *service.Service in 60-04 (one ctx per
// Start; Stop cancels it deterministically).
type Manager struct {
	cfg      Config
	producer Producer
	logger   *slog.Logger

	mu     sync.Mutex
	active map[workspace.WorkspaceKey]*workspaceWatcher
}

// NewManager constructs a Manager wired to producer. cfg may be the
// zero value; DefaultConfig fields are applied per-field. logger MUST
// be non-nil (the caller — typically 60-05B's daemon — owns the
// shared *slog.Logger; failing fast here keeps the producer path
// branchless).
func NewManager(producer Producer, cfg Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		cfg:      applyDefaults(cfg),
		producer: producer,
		logger:   logger,
		active:   make(map[workspace.WorkspaceKey]*workspaceWatcher),
	}
}

// Start spins up a per-workspace fsnotify goroutine for ws. Idempotent:
// a second Start with the same key is a no-op (matches Service.Start
// shape from 60-04 service.go for symmetry).
//
// Returns ErrInotifyENOSPC if fsnotify.NewWatcher() itself fails with
// ENOSPC at construction; addRecursive ENOSPC during the initial walk
// is handled internally and reflected in Status() rather than
// surfaced as an error (the watcher is considered "started but
// degraded" — the manifest scanner from 60-05B remains correct).
func (m *Manager) Start(ctx context.Context, ws workspace.WorkspaceKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.active[ws]; ok {
		return nil
	}
	ww, err := newWorkspaceWatcher(ws, m.cfg, m.producer, m.logger)
	if err != nil {
		return err
	}
	m.active[ws] = ww
	go ww.run(ctx)
	return nil
}

// Stop cancels the per-workspace goroutine and closes the underlying
// fsnotify.Watcher. Idempotent: a second Stop is a no-op. Stop does
// NOT block on the goroutine drain — the run loop returns when its
// context is cancelled, which is normally bound to the daemon's root
// context.
func (m *Manager) Stop(ws workspace.WorkspaceKey) {
	m.mu.Lock()
	ww := m.active[ws]
	delete(m.active, ws)
	m.mu.Unlock()
	if ww != nil {
		ww.close()
	}
}

// Status returns the latest WatcherStatus snapshot for ws. Returns
// WatcherStatus{Active: false, Reason: "not_started"} if no Start has
// been called for the key. This is the data accessor consumed by
// Phase 65's get_health MCP tool.
func (m *Manager) Status(ws workspace.WorkspaceKey) WatcherStatus {
	m.mu.Lock()
	ww := m.active[ws]
	m.mu.Unlock()
	if ww == nil {
		return WatcherStatus{Active: false, Reason: "not_started"}
	}
	return ww.Status()
}
