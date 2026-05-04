// Package scheduler owns extraction lifecycle: initial-walk on workspace
// activation, incremental scheduling on file changes (Phase 60 fills body),
// state tracking, and the centralized RequireReady consumer-side gate.
//
// D-04 hard invariants:
//   - One active extraction job per workspace at a time.
//   - ScheduleInitialExtraction is idempotent — concurrent calls return same JobID.
//   - File changes during in-flight initial extraction are queued for incremental.
//   - Jobs are cancellable on workspace deactivation.
//   - RequireReady is the ONLY semantic readiness API for consumers — no time.Sleep polling.
//
// Allowed imports:
//   - internal/semantic/extract (consumes Registry, fact types)
//   - internal/repomap (consumes PageRank scores for initial-walk priority)
//   - internal/semantic/store (writes ExtractedFile facts via Phase 59 surface)
//
// Forbidden imports:
//   - internal/kernel (scheduler must not depend on kernel — daemon is the
//     wiring point that bridges them via callback)
package scheduler
