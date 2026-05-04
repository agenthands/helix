package scheduler

import "github.com/agenthands/helix/internal/semantic"

// TestSetStatus is a test-only helper that publishes a SemanticStatus for ws
// without going through ScheduleInitialExtraction. Used by ready_test.go to
// stage state machine transitions deterministically. Production callers MUST
// NOT mutate scheduler state directly — they use ScheduleInitialExtraction or
// the (Phase 60) extraction-completion callback (T-59-03-03 mitigation:
// available only via _test.go consumers).
func (s *Scheduler) TestSetStatus(ws semantic.WorkspaceID, st SemanticStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[ws] = st
	for _, sub := range s.subscribers[ws] {
		select {
		case sub <- st:
		default:
		}
	}
}

// HasInflightJob reports whether ws currently has a job recorded in the
// in-flight map. Test-only — exercises the idempotency invariant.
func (s *Scheduler) HasInflightJob(ws semantic.WorkspaceID) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.jobs[ws]
	return ok
}
