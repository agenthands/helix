package lspool

import (
	"context"
	"sync"
)

// mutationMethods lists LSP methods that mutate document state (per D-11).
var mutationMethods = map[string]bool{
	"textDocument/didChange": true,
	"textDocument/didOpen":   true,
	"textDocument/didClose":  true,
	"textDocument/didSave":   true,
	"textDocument/rename":    true,
}

// WorkerLease binds a session to an LS worker.
// Per D-11: mutations are serialized via write lock, reads are parallel via read lock.
type WorkerLease struct {
	SessionID string
	Worker    *Worker
	Dirty     bool
	mu        sync.RWMutex // serializes mutations, parallel reads
}

// NewWorkerLease creates a new lease binding a session to a worker.
func NewWorkerLease(sessionID string, worker *Worker, dirty bool) *WorkerLease {
	return &WorkerLease{
		SessionID: sessionID,
		Worker:    worker,
		Dirty:     dirty,
	}
}

// Request sends a request through the lease with concurrency control.
// Mutations acquire a write lock; reads acquire a read lock.
func (l *WorkerLease) Request(ctx context.Context, method string, params, result interface{}) error {
	if IsMutation(method) {
		l.mu.Lock()
		defer l.mu.Unlock()
	} else {
		l.mu.RLock()
		defer l.mu.RUnlock()
	}
	return l.Worker.Request(ctx, method, params, result)
}

// Notify sends a notification through the lease.
func (l *WorkerLease) Notify(ctx context.Context, method string, params interface{}) error {
	return l.Worker.Notify(ctx, method, params)
}

// IsMutation returns true for LSP methods that mutate document state.
func IsMutation(method string) bool {
	return mutationMethods[method]
}
