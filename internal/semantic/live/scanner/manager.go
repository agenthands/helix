package scanner

import (
	"context"
	"log/slog"
	"sync"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/workspace"
)

// Manager owns per-workspace Scanner goroutines. The daemon (60-05B)
// constructs a single Manager during bootstrap and calls Start/Stop from
// SetActivateCallback. Mirrors the watcher/manager.go shape from 60-05A so
// the daemon wiring is symmetric.
//
// Manager is safe for concurrent Start/Stop calls.
type Manager struct {
	producer  Producer
	lookup    FileHashLookup
	cfg       Config
	logger    *slog.Logger
	repoIDFor func(workspace.WorkspaceKey) semantic.RepoID

	mu       sync.Mutex
	cancels  map[workspace.WorkspaceKey]context.CancelFunc
	scanners map[workspace.WorkspaceKey]*Scanner
}

// NewManager constructs a Manager. repoIDFor maps a workspace key to the
// semantic.RepoID used for store lookups; the daemon (60-05B) supplies a
// closure over its existing repo-id derivation logic.
func NewManager(p Producer, l FileHashLookup, repoIDFor func(workspace.WorkspaceKey) semantic.RepoID, cfg Config, logger *slog.Logger) *Manager {
	if logger == nil {
		logger = slog.Default()
	}
	return &Manager{
		producer:  p,
		lookup:    l,
		cfg:       cfg,
		logger:    logger,
		repoIDFor: repoIDFor,
		cancels:   make(map[workspace.WorkspaceKey]context.CancelFunc),
		scanners:  make(map[workspace.WorkspaceKey]*Scanner),
	}
}

// Start spawns a Scanner goroutine for ws. Idempotent: a second Start with
// the same ws returns nil without spawning a duplicate.
//
// The returned error is reserved for future validation (e.g., refusing
// non-absolute roots); current implementation always returns nil to keep
// the surface symmetric with watcher.Manager.Start.
func (m *Manager) Start(ctx context.Context, ws workspace.WorkspaceKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.scanners[ws]; ok {
		return nil
	}
	repoID := m.repoIDFor(ws)
	s := New(ws, repoID, m.producer, m.lookup, m.cfg, m.logger)
	cctx, cancel := context.WithCancel(ctx)
	m.scanners[ws] = s
	m.cancels[ws] = cancel
	go func() {
		_ = s.Run(cctx)
	}()
	return nil
}

// Stop cancels the Scanner goroutine for ws. Idempotent: a Stop on an
// unknown workspace is a no-op.
func (m *Manager) Stop(ws workspace.WorkspaceKey) {
	m.mu.Lock()
	cancel := m.cancels[ws]
	delete(m.cancels, ws)
	delete(m.scanners, ws)
	m.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}
