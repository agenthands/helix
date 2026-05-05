package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/semantic/live/lspqueue"
)

// Lane is the closed-enum lane identifier on the 2-lane priority queue.
// Phase 61 D-01: producer-owned lane choice; consumer drains LaneHigh strictly
// before LaneBackground (no aging, no fairness budget).
type Lane string

const (
	// LaneHigh carries helix_edit + agent-requested jobs. Drained first.
	LaneHigh Lane = "high"
	// LaneBackground carries watcher / manifest-scan jobs. Drained only when
	// LaneHigh is empty.
	LaneBackground Lane = "background"
)

// LaneQueue wraps two lspqueue.Queue instances behind a strict-priority
// drain. Phase 61 D-01.
//
// Both lanes are bounded buffered channels; a full lane drops the new job
// (Enqueue / EnqueueLane returns false). The strict-priority Drain reads
// LaneHigh non-blockingly first; if empty, it blocks on either lane or ctx.
type LaneQueue struct {
	high       *lspqueue.Queue
	background *lspqueue.Queue
}

// NewLaneQueue constructs a 2-lane queue with the supplied per-lane buffer
// capacities. Non-positive values default to 1024 (per lspqueue.New).
func NewLaneQueue(highBuf, bgBuf int) *LaneQueue {
	return &LaneQueue{
		high:       lspqueue.New(highBuf),
		background: lspqueue.New(bgBuf),
	}
}

// EnqueueLane is the lane-aware non-blocking producer side. Returns true if
// the job was queued, false if the target lane buffer was full or the lane
// argument is unknown (closed-enum guard).
func (q *LaneQueue) EnqueueLane(lane Lane, job lspqueue.RevalidateFileJob) bool {
	switch lane {
	case LaneHigh:
		return q.high.Enqueue(job)
	case LaneBackground:
		return q.background.Enqueue(job)
	default:
		return false
	}
}

// Enqueue is the legacy single-method shape preserved for backward-compat
// (B5 belt-and-braces). It treats every job as LaneHigh — new callers MUST
// use EnqueueLane.
//
// Deprecated: use EnqueueLane(LaneHigh, job).
func (q *LaneQueue) Enqueue(job lspqueue.RevalidateFileJob) bool {
	return q.EnqueueLane(LaneHigh, job)
}

// Depth returns the current buffered count for the given lane. Returns 0 for
// unknown lanes (defensive).
func (q *LaneQueue) Depth(lane Lane) int {
	switch lane {
	case LaneHigh:
		return q.high.Len()
	case LaneBackground:
		return q.background.Len()
	default:
		return 0
	}
}

// Drain reads one job, blocking until either lane has a job or ctx is done.
// Implements strict priority (Phase 61 D-01): LaneHigh is checked
// non-blockingly first; only if LaneHigh is empty does the blocking select
// consider LaneBackground.
//
// Returns ctx.Err() with an empty Lane label if ctx is cancelled before any
// job arrives.
func (q *LaneQueue) Drain(ctx context.Context) (lspqueue.RevalidateFileJob, Lane, error) {
	// Non-blocking high check first — if a high job is available we MUST
	// take it before considering background, even if a background job is
	// also pending. This is the load-bearing "strict priority" property.
	select {
	case job := <-q.high.Channel():
		return job, LaneHigh, nil
	default:
	}

	// Both lanes empty (or only background pending) — block on whichever
	// arrives first, plus ctx.Done.
	select {
	case job := <-q.high.Channel():
		return job, LaneHigh, nil
	case job := <-q.background.Channel():
		return job, LaneBackground, nil
	case <-ctx.Done():
		return lspqueue.RevalidateFileJob{}, "", ctx.Err()
	}
}
