// Package service ships the per-daemon glue that bridges kernel edits
// (via the kernel.EditNotifier interface from 60-03) into the per-
// workspace coalescer pipeline.  Kept in a sub-package of internal/
// semantic/live to break what would otherwise be an import cycle:
//
//	live           ← types (signal.go, classifier.go)
//	  ↑
//	live/coalescer ← consumes live types
//	  ↑
//	live/service   ← composes live + live/coalescer + kernel.EditNotifier
//
// The Service type is the load-bearing seam that Phase 60-05B's daemon
// wiring installs via Kernel.SetEditNotifier.
package service

import (
	"context"
	"sync"
	"time"

	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/live"
	"github.com/agenthands/helix/internal/semantic/live/coalescer"
	"github.com/agenthands/helix/internal/workspace"
)

// Service is the per-daemon glue: implements kernel.EditNotifier, holds
// the per-workspace coalescer registry, owns the classifier, delegates
// to a shared handler.Handler.
//
// The semantic→kernel import direction is permitted by 60-01's
// vet-nokernel2semantic analyzer (which only forbids kernel→semantic).
// The Service depends on kernel.EditNotifier purely for the interface
// satisfaction, so the var-_ assertion at the bottom of this file
// statically pins the relationship without dragging the whole kernel
// surface into the live package.
type Service struct {
	cfg        coalescer.Config
	handler    coalescer.EventHandler
	classifier ClassifierFunc
	repoIDFor  func(workspace.WorkspaceKey) semantic.RepoID
	logger     coalescer.Logger
	now        func() time.Time

	mu         sync.RWMutex
	coalescers map[workspace.WorkspaceKey]*coalescer.Coalescer
	cancels    map[workspace.WorkspaceKey]context.CancelFunc

	// Phase 63 P63-02 Task 3: optional post-flush hook fan-out. When
	// set, every coalescer started by Start gets its own
	// SetOnFlush(func() { onFlushHook(ws) }) closure so the
	// per-workspace flush-completion signal reaches the daemon's
	// compactBundle.OnCoalescerFlush. Nil-safe.
	onFlushMu   sync.Mutex
	onFlushHook func(workspace.WorkspaceKey)
}

// SetOnFlushHook registers a process-global post-flush callback. Phase
// 63 P63-02 Task 3: the daemon wires this to
// compactBundle.OnCoalescerFlush so the compaction timer is reset on
// every flush.
//
// MUST be called before Start to take effect on the first coalescer
// (the hook is captured at Start time). Nil clears the hook.
func (s *Service) SetOnFlushHook(fn func(workspace.WorkspaceKey)) {
	if s == nil {
		return
	}
	s.onFlushMu.Lock()
	s.onFlushHook = fn
	s.onFlushMu.Unlock()
}

// LastFlushAt returns the per-workspace coalescer's most-recent flush
// timestamp, or the zero value when no coalescer is registered for ws.
// Phase 63 P63-02 Task 3: consumed by the compaction gate via the
// daemon's coalescerAccessor adapter. nil-safe.
func (s *Service) LastFlushAt(ws workspace.WorkspaceKey) time.Time {
	if s == nil {
		return time.Time{}
	}
	s.mu.RLock()
	c, ok := s.coalescers[ws]
	s.mu.RUnlock()
	if !ok || c == nil {
		return time.Time{}
	}
	return c.LastFlushAt()
}

// ClassifierFunc is the signature the daemon-wired classifier closure
// matches.  Production wiring is a closure over live.ClassifyPathChange
// with a concrete FileHashLookup + FileHasher.
type ClassifierFunc func(
	ctx context.Context,
	repoID semantic.RepoID,
	path string,
	source live.ChangeSource,
) (live.SourceChangeKind, bool, error)

// New constructs a Service.  All four function-typed parameters must
// be non-nil; callers (60-05B) wire them from concrete deps.
func New(
	cfg coalescer.Config,
	handler coalescer.EventHandler,
	classifier ClassifierFunc,
	repoIDFor func(workspace.WorkspaceKey) semantic.RepoID,
	logger coalescer.Logger,
) *Service {
	return &Service{
		cfg:        cfg,
		handler:    handler,
		classifier: classifier,
		repoIDFor:  repoIDFor,
		logger:     logger,
		now:        time.Now,
		coalescers: make(map[workspace.WorkspaceKey]*coalescer.Coalescer),
		cancels:    make(map[workspace.WorkspaceKey]context.CancelFunc),
	}
}

// Start spins up a coalescer goroutine for ws.  Idempotent: a second
// Start with the same ws is a no-op.  The daemon (60-05B) calls Start
// when a workspace is activated and Stop on deactivation.
func (s *Service) Start(ctx context.Context, ws workspace.WorkspaceKey) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.coalescers[ws]; ok {
		return
	}
	c := coalescer.New(ws, s.cfg, s.handler, s.logger)
	// Phase 63 P63-02 Task 3: forward post-flush signals into the
	// daemon-installed hook. Closure captures ws so the daemon's
	// OnCoalescerFlush(ws) receives the right key.
	s.onFlushMu.Lock()
	hook := s.onFlushHook
	s.onFlushMu.Unlock()
	if hook != nil {
		wsCapture := ws
		c.SetOnFlush(func() { hook(wsCapture) })
	}
	cctx, cancel := context.WithCancel(ctx)
	s.coalescers[ws] = c
	s.cancels[ws] = cancel
	go func() { _ = c.Run(cctx) }()
}

// Stop cancels a per-workspace coalescer goroutine.  Idempotent.
func (s *Service) Stop(ws workspace.WorkspaceKey) {
	s.mu.Lock()
	cancel := s.cancels[ws]
	delete(s.coalescers, ws)
	delete(s.cancels, ws)
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// OnEdit implements kernel.EditNotifier.  Fire-and-forget:
//
//   - Classifies each path (helix_edit short-circuits without I/O —
//     classifier returns ChangeHelixEdit immediately for that source).
//   - Non-blocking enqueues into the per-workspace coalescer (the
//     Coalescer.Enqueue method uses select-default-drop internally).
//
// Returns nil on the happy path; logs and drops if the workspace has
// no running coalescer (Start was never called).  The kernel-side
// caller at 60-03 swallows the error, so Service.OnEdit is effectively
// fire-and-forget end-to-end.
func (s *Service) OnEdit(ctx context.Context, ws workspace.WorkspaceKey, paths []string) error {
	return s.signal(ctx, live.WorkspaceChangeSignal{
		WorkspaceID: ws,
		Paths:       paths,
		Source:      live.ChangeSourceHelixEdit,
		ObservedAt:  s.now(),
	})
}

// OnWorkspaceChanged is the public entry that the watcher (60-05A) and
// the manifest scanner (60-05B) call.  Same fire-and-forget semantics
// as OnEdit.
func (s *Service) OnWorkspaceChanged(ctx context.Context, sig live.WorkspaceChangeSignal) error {
	return s.signal(ctx, sig)
}

// signal is the shared classify-and-enqueue body for OnEdit and
// OnWorkspaceChanged.
func (s *Service) signal(ctx context.Context, sig live.WorkspaceChangeSignal) error {
	s.mu.RLock()
	c := s.coalescers[sig.WorkspaceID]
	s.mu.RUnlock()
	if c == nil {
		// Workspace not registered with the live service.  60-05B's
		// daemon wiring calls Start before any kernel hook can fire
		// against this workspace; reaching this branch in production
		// would be an ordering bug.
		s.logger.Warn("live: signal for unstarted workspace; dropping",
			"workspace", sig.WorkspaceID)
		return nil
	}
	repoID := s.repoIDFor(sig.WorkspaceID)
	for _, path := range sig.Paths {
		kind, ok, err := s.classifier(ctx, repoID, path, sig.Source)
		if err != nil {
			s.logger.Warn("live: classifier error",
				"path", path, "err", err)
			continue
		}
		if !ok {
			continue
		}
		c.Enqueue(live.SourceChangeEvent{
			RepoID:     repoID,
			Kind:       kind,
			Path:       path,
			Source:     sig.Source,
			ObservedAt: sig.ObservedAt,
		})
	}
	return nil
}

// Compile-time assertion: Service satisfies kernel.EditNotifier.
//
// This is the load-bearing seam between the kernel edit/fileops tools
// (60-03) and the live-update pipeline.  If this line fails to compile,
// the Service is no longer a valid notifier and the daemon bootstrap
// (60-05B) cannot wire it via Kernel.SetEditNotifier.
var _ kernel.EditNotifier = (*Service)(nil)
