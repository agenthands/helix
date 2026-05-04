package scheduler

import (
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
	// RequireReady is the SOLE semantic readiness API. Method declaration is
	// extended onto this interface when ready.go lands in Task 2.
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

// ScheduleIncremental is a Phase 60 stub — returns a sentinel JobID and does
// not mutate state. Wave 2 ships the interface so consumers compile today.
func (s *Scheduler) ScheduleIncremental(ws semantic.WorkspaceID, changes []FileChange) JobID {
	return JobID("phase60-incremental-stub")
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
