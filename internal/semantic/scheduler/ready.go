package scheduler

import (
	"context"
	"time"

	"github.com/agenthands/helix/internal/semantic"
)

// ReadyPolicy controls how RequireReady waits for the workspace's semantic
// index to reach a usable state. The zero value is NOT the documented default
// — callers that want the documented behavior call DefaultReadyPolicy().
//
// D-04 documented defaults (from CONTEXT.md):
//
//	Timeout=30s, AllowPartial=true, MinState=SemanticPartial, TriggerIfCold=true.
type ReadyPolicy struct {
	// Timeout caps the maximum wait. Default 30s
	// (cfg.SemanticIndex.Extraction.ExtractionReadyTimeout). On timeout the
	// caller receives the latest known status with Ready=true if any files
	// were indexed (best-partial) and Ready=false otherwise.
	Timeout time.Duration

	// AllowPartial: if true, SemanticPartial counts as ready. Default true.
	AllowPartial bool

	// MinState is the minimum index state that satisfies the wait. Default
	// SemanticPartial. Implementations rank states via stateRank().
	MinState SemanticIndexState

	// TriggerIfCold: if true and the workspace is in SemanticNotStarted,
	// RequireReady kicks ScheduleInitialExtraction before waiting. Default true.
	TriggerIfCold bool
}

// ReadyResult is the consumer-facing return of RequireReady. Ready==true means
// the wait was satisfied (state met MinState or partial-with-AllowPartial).
// On timeout, Ready==true iff at least one file was indexed (best-partial).
type ReadyResult struct {
	State      SemanticIndexState
	Partial    bool
	Ready      bool
	IndexedAt  time.Time
	FilesTotal int
	FilesDone  int
	Errors     []IndexError
}

// DefaultReadyPolicy returns the D-04 documented defaults. Callers that want
// tighter policy override fields explicitly; do not mutate the returned struct.
func DefaultReadyPolicy() ReadyPolicy {
	return ReadyPolicy{
		Timeout:       30 * time.Second,
		AllowPartial:  true,
		MinState:      SemanticPartial,
		TriggerIfCold: true,
	}
}

// toResult converts a SemanticStatus into a ReadyResult with the given
// readiness flag. Internal helper kept on a value receiver so RequireReady
// can adapt either the live status or a subscriber-delivered transition.
func (s SemanticStatus) toResult(ready bool) ReadyResult {
	return ReadyResult{
		State:      s.State,
		Partial:    s.Partial,
		Ready:      ready,
		IndexedAt:  s.IndexedAt,
		FilesTotal: s.FilesTotal,
		FilesDone:  s.FilesDone,
		Errors:     s.Errors,
	}
}

// RequireReady is the SOLE semantic readiness API. Consumers MUST use this;
// any time.Sleep-based polling violates D-04 acceptance #9.
//
// Callers MUST be on a goroutine that does not hold any scheduler-internal
// lock — RequireReady can wait up to ReadyPolicy.Timeout.
//
// Behavior:
//   - State == ready  → return immediately (Ready=true).
//   - State == partial && AllowPartial → return immediately (Ready=true,
//     Partial=true).
//   - State == failed → return ErrSemanticFailed (Ready=false).
//   - State == not_started && TriggerIfCold → kick scheduler then wait.
//   - State == indexing|stale → wait until ready/partial/failed or Timeout.
//   - Timeout → return best-partial (Ready=true if FilesDone > 0, else false,
//     no error — partial readiness is not an error per D-04).
//   - ctx.Done() → return ctx.Err() with the latest known status.
//
// T-59-03-01: Timer + ctx.Done() escape paths bound the wait; the default 30s
// ceiling caps DoS exposure even if the caller forgets to set Timeout.
func (s *Scheduler) RequireReady(ctx context.Context, ws semantic.WorkspaceID, policy ReadyPolicy) (ReadyResult, error) {
	st := s.Status(ws)

	switch st.State {
	case SemanticReady:
		return st.toResult(true), nil
	case SemanticPartial:
		if policy.AllowPartial {
			return st.toResult(true), nil
		}
		// fall through to wait for SemanticReady
	case SemanticFailed:
		return st.toResult(false), ErrSemanticFailed
	case SemanticNotStarted:
		if policy.TriggerIfCold {
			s.ScheduleInitialExtraction(ws, InitialExtraction{
				Reason: "require_ready_cold",
				Mode:   ModeIncrementalIfPossible,
			})
		}
		// fall through to wait
	case SemanticIndexing, SemanticStale:
		// fall through to wait
	}

	// Subscribe AFTER the initial fast-path returns so we don't allocate a
	// channel for ready/partial-with-AllowPartial/failed callers. The
	// scheduler's lifetime equals the daemon's, so unbounded subscriber
	// growth is acceptable for Wave 2; P05 may add explicit Unsubscribe when
	// wiring deactivation.
	sub := s.Subscribe(ws)

	timer := time.NewTimer(policy.Timeout)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return s.Status(ws).toResult(false), ctx.Err()
		case <-timer.C:
			cur := s.Status(ws)
			return cur.toResult(cur.FilesDone > 0), nil
		case got := <-sub:
			if got.State == SemanticFailed {
				return got.toResult(false), ErrSemanticFailed
			}
			if got.State == SemanticPartial && !policy.AllowPartial {
				continue
			}
			if stateRank(got.State) >= stateRank(policy.MinState) {
				return got.toResult(true), nil
			}
		}
	}
}

// stateRank orders states by readiness. Used by RequireReady to compare an
// incoming subscriber event against ReadyPolicy.MinState. Partial < Ready
// (rank 2 vs 3) so MinState=SemanticPartial accepts both partial and ready
// transitions; Indexing/Stale rank below partial so they never satisfy the
// wait directly.
func stateRank(s SemanticIndexState) int {
	switch s {
	case SemanticReady:
		return 3
	case SemanticPartial:
		return 2
	case SemanticIndexing, SemanticStale:
		return 1
	default:
		return 0
	}
}
