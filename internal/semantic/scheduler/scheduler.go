package scheduler

import (
	"context"
	"errors"
	"sync"

	"github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/extract"
)

// ExtractionScheduler is the consumer-facing surface owned by P03. The
// daemon (P05) constructs a *Scheduler and exposes it through this
// interface to keep callers free of the concrete type's wiring deps.
//
// D-04 invariants:
//   - ScheduleInitialExtraction is idempotent per workspace: concurrent
//     calls return the same in-flight JobID.
//   - ScheduleIncremental is a Phase 60 stub here (returns a sentinel JobID).
//   - Status reads the latest published SemanticStatus.
//   - Subscribe returns a buffered channel (cap 8) that receives every
//     transition; slow consumers are dropped non-blockingly (T-59-03-02).
//   - RequireReady is the ONLY semantic readiness API — no time.Sleep polling
//     anywhere in this package (D-04 acceptance #9).
type ExtractionScheduler interface {
	ScheduleInitialExtraction(workspaceID semantic.WorkspaceID, req InitialExtraction) JobID
	ScheduleIncremental(workspaceID semantic.WorkspaceID, changes []FileChange) JobID
	Status(workspaceID semantic.WorkspaceID) SemanticStatus
	Subscribe(workspaceID semantic.WorkspaceID) <-chan SemanticStatus
	// RequireReady is the SOLE semantic readiness API. Implementation lives in
	// ready.go; consumers MUST use this method instead of polling Status().
	RequireReady(ctx context.Context, ws semantic.WorkspaceID, policy ReadyPolicy) (ReadyResult, error)
}

// Scheduler is the concrete in-memory implementation of ExtractionScheduler.
// Phase 59 P03 ships the orchestration surface (state, subscribers, idempotent
// job admission). Phase 59 P05 wires the actual extraction loop (provider
// dispatch, store writes) onto this scaffold; the public API does not change.
type Scheduler struct {
	mu       sync.Mutex
	registry *extract.Registry
	// store, repomap, config — wired by NewScheduler in P05 (daemon bootstrap).
	// For Wave 2 we ship the interface and a minimal in-memory implementation.
	jobs        map[semantic.WorkspaceID]JobID // in-flight job per workspace
	states      map[semantic.WorkspaceID]SemanticStatus
	subscribers map[semantic.WorkspaceID][]chan SemanticStatus
	incHandler  IncrementalHandler // 60-04 D-07: nil-safe; daemon wires post-construction
}

// IncrementalHandler is the consumer side of ScheduleIncremental
// dispatch. The semantic/live/handler.Handler implements it (the handler
// receiver methods UpdateChangedFile + HandleFileDeleted match this
// signature pair).
//
// Declaring the interface in the SCHEDULER package — rather than the
// live package — keeps the import direction one-way (scheduler ←
// live/handler) and avoids a cycle: live/handler imports scheduler
// (for FileChange + JobID), scheduler must NOT import live/handler.
type IncrementalHandler interface {
	UpdateChangedFile(ctx context.Context, repoID semantic.RepoID, path string) error
	HandleFileDeleted(ctx context.Context, repoID semantic.RepoID, path string) error
}

// NewScheduler constructs a Scheduler with the given extract.Registry. The
// registry is consumed read-only and may be nil in tests that do not exercise
// the per-language dispatch path; production daemon wiring (P05) always
// passes a non-nil registry.
func NewScheduler(registry *extract.Registry) *Scheduler {
	return &Scheduler{
		registry:    registry,
		jobs:        make(map[semantic.WorkspaceID]JobID),
		states:      make(map[semantic.WorkspaceID]SemanticStatus),
		subscribers: make(map[semantic.WorkspaceID][]chan SemanticStatus),
	}
}

// ScheduleInitialExtraction kicks an initial-walk extraction for ws. Idempotent
// per workspace: if a job is already in-flight, the existing JobID is
// returned and req is ignored (the first call's reason wins). The state
// transitions to SemanticIndexing on the first admission and Subscribers
// receive the new status.
func (s *Scheduler) ScheduleInitialExtraction(ws semantic.WorkspaceID, req InitialExtraction) JobID {
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.jobs[ws]; ok {
		return existing // idempotent: return the in-flight job ID
	}
	job := JobID(generateJobID(ws, req.Reason))
	s.jobs[ws] = job
	s.transitionUnlocked(ws, SemanticIndexing)
	// Body of the actual walk lands when daemon wires the store + extractor (P05);
	// for Wave 2 we ship the orchestration surface.
	return job
}

// SetIncrementalHandler installs h as the consumer for
// ScheduleIncremental dispatch. Mirrors the post-init setter pattern
// used by RepoMapSkill.SetEnrichFn (daemon bootstrap calls this in
// the wiring step after both scheduler and live handler are
// constructed). Safe for concurrent readers via the same mutex that
// guards in-flight job state.
func (s *Scheduler) SetIncrementalHandler(h IncrementalHandler) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.incHandler = h
}

// ScheduleIncremental dispatches per-FileChange to the registered
// IncrementalHandler. Returns a deterministic non-sentinel JobID and
// transitions the workspace state to SemanticIndexing.
//
// Per-change dispatch:
//
//	Kind="modified"|"created" → handler.UpdateChangedFile
//	Kind="deleted"            → handler.HandleFileDeleted
//	other                     → silently skipped (forward-compat)
//
// Nil-safe: if no handler is registered (scheduler constructed before
// live wiring), the JobID is still allocated and the state transition
// still fires — the dispatch loop is a no-op.
//
// 60-04 D-07: this is the in-process API the live dispatcher (D-02)
// and Phase 64's refresh_semantic_graph MCP tool both reach.
func (s *Scheduler) ScheduleIncremental(ws semantic.WorkspaceID, changes []FileChange) JobID {
	s.mu.Lock()
	h := s.incHandler
	job := JobID(generateJobID(ws, "incremental"))
	s.transitionUnlocked(ws, SemanticIndexing)
	s.mu.Unlock()

	if h == nil {
		// Pre-wiring: state transition + JobID still fire so the
		// caller can subscribe and observe progression once the
		// handler is installed (in production the handler is wired
		// at daemon bootstrap, before any tool can call Schedule*).
		return job
	}

	go func() {
		ctx := context.Background()
		for _, ch := range changes {
			switch ch.Kind {
			case "modified", "created":
				_ = h.UpdateChangedFile(ctx, semantic.RepoID(ws), ch.Path)
			case "deleted":
				_ = h.HandleFileDeleted(ctx, semantic.RepoID(ws), ch.Path)
			}
		}
	}()
	return job
}

// Status returns the latest published SemanticStatus for ws. Workspaces that
// were never scheduled return a zero-value SemanticStatus with State =
// SemanticNotStarted.
func (s *Scheduler) Status(ws semantic.WorkspaceID) SemanticStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	if st, ok := s.states[ws]; ok {
		return st
	}
	return SemanticStatus{State: SemanticNotStarted}
}

// Subscribe returns a buffered channel (cap 8) that receives every state
// transition for ws. The channel is owned by the scheduler — callers MUST
// NOT close it. P05 may add an explicit Unsubscribe when wiring deactivation;
// Wave 2 ships scheduler-lifetime-bound subscriptions.
func (s *Scheduler) Subscribe(ws semantic.WorkspaceID) <-chan SemanticStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch := make(chan SemanticStatus, 8) // buffered to avoid blocking publisher
	s.subscribers[ws] = append(s.subscribers[ws], ch)
	return ch
}

// transitionUnlocked publishes a state change for ws to subscribers. Caller
// MUST hold s.mu. Subscribers receive via best-effort non-blocking send so a
// single slow consumer cannot stall the publisher (T-59-03-02 mitigation).
func (s *Scheduler) transitionUnlocked(ws semantic.WorkspaceID, state SemanticIndexState) {
	st := s.states[ws]
	st.State = state
	s.states[ws] = st
	for _, sub := range s.subscribers[ws] {
		select {
		case sub <- st:
		default: // drop on full subscriber — best-effort fanout
		}
	}
}

// generateJobID is deterministic by design: the in-flight JobID is recovered
// without consulting wallclock so the idempotent "concurrent calls return the
// same JobID" invariant is testable (D-04 invariant).
func generateJobID(ws semantic.WorkspaceID, reason string) string {
	return string(ws) + "-" + reason
}

// ErrSemanticFailed is returned by RequireReady when the workspace's
// semantic index is in the SemanticFailed state.
var ErrSemanticFailed = errors.New("semantic index failed")
