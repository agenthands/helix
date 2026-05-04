package scheduler

import "time"

// SemanticIndexState is the lifecycle state of a workspace's semantic index.
// Closed enum — values are written into log fields, surfaced as bounded-label
// metric values, and consumed by RequireReady's ordering rank.
type SemanticIndexState string

const (
	SemanticNotStarted SemanticIndexState = "not_started"
	SemanticIndexing   SemanticIndexState = "indexing"
	SemanticReady      SemanticIndexState = "ready"
	SemanticPartial    SemanticIndexState = "partial"
	SemanticFailed     SemanticIndexState = "failed"
	SemanticStale      SemanticIndexState = "stale"
)

// SemanticStatus is the consumer-facing snapshot of a workspace's extraction
// progress. Returned by Scheduler.Status and pushed to Subscribe channels on
// every state transition.
type SemanticStatus struct {
	State      SemanticIndexState
	Partial    bool
	FilesTotal int
	FilesDone  int
	IndexedAt  time.Time
	Errors     []IndexError
}

// IndexError describes a per-file extraction failure surfaced through
// SemanticStatus.Errors. Reason should match one of the extract.PartialReason
// values when the failure originates from the extractor; free-form otherwise.
type IndexError struct {
	File    string
	Reason  string // matches extract.PartialReason values
	Message string
}

// JobID is the opaque handle returned by ScheduleInitialExtraction /
// ScheduleIncremental. Idempotent: concurrent ScheduleInitialExtraction calls
// for the same workspace return the same JobID until the in-flight job
// completes (D-04 invariant).
type JobID string

// InitialExtractionMode controls whether ScheduleInitialExtraction prefers an
// incremental rebuild (when a prior snapshot exists) or always walks the full
// tree. Phase 59 ships the enum; the daemon-wired implementation in P05 picks
// the actual strategy.
type InitialExtractionMode int

const (
	ModeAuto InitialExtractionMode = iota
	ModeIncrementalIfPossible
	ModeFullRebuild
)

// InitialExtraction is the request payload for ScheduleInitialExtraction.
// Reason is recorded for telemetry (e.g., "workspace_activation",
// "require_ready_cold"); Mode selects the extraction strategy.
type InitialExtraction struct {
	Reason string
	Mode   InitialExtractionMode
}

// FileChange describes a single workspace file mutation passed to
// ScheduleIncremental. Phase 60 fills the body; Phase 59 ships the type so
// consumers can compile against the interface today.
type FileChange struct {
	Path string
	Kind string // "modified" | "created" | "deleted"
}
