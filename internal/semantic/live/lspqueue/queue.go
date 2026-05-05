// Package lspqueue ships the typed buffered handoff for LSP enrichment
// jobs.  Phase 60 P04 declares the queue and exposes Enqueue
// (producer-only); Phase 61 wires a worker goroutine to drain via
// Channel().
//
// The queue is intentionally minimal — a typed channel wrapper with a
// non-blocking Enqueue.  The producer-vs-consumer split lets P60 ship
// the type without forcing P61 to have already landed.
package lspqueue

import (
	"github.com/agenthands/helix/internal/semantic"
)

// RevalidateFileJob is the unit of work the LSP queue carries: a
// (repo, path) pair the Phase 61 worker re-validates against the
// language server.
type RevalidateFileJob struct {
	RepoID semantic.RepoID
	Path   string
}

// Queue is a typed buffered channel of RevalidateFileJob.
type Queue struct {
	ch chan RevalidateFileJob
}

// New constructs a Queue with the given buffer capacity.  Non-positive
// values default to 1024.
func New(buffer int) *Queue {
	if buffer <= 0 {
		buffer = 1024
	}
	return &Queue{ch: make(chan RevalidateFileJob, buffer)}
}

// Enqueue is non-blocking.  Returns true if the job was queued, false
// if dropped due to buffer pressure.  Phase 60 callers tolerate drops
// (the watcher and scanner provide the correctness story; the LSP
// enrichment is purely a freshness optimization); Phase 61 will tighten
// the contract once the consumer is wired.
func (q *Queue) Enqueue(job RevalidateFileJob) bool {
	select {
	case q.ch <- job:
		return true
	default:
		return false
	}
}

// Channel returns the read-only side.  Phase 61's worker reads from
// this; Phase 60 has no consumer.
func (q *Queue) Channel() <-chan RevalidateFileJob { return q.ch }

// Len returns the current number of buffered jobs.  Useful for metrics
// (helix_semantic_live_lspqueue_depth) and for tests.
func (q *Queue) Len() int { return len(q.ch) }
