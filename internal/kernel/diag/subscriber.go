// Package diag implements diagnostics tools: publishDiagnostics subscription,
// code action forwarding, and code formatting via LSP.
package diag

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	gen "github.com/agenthands/helix/protocol/gen"
)

// DiagnosticStore collects and serves publishDiagnostics notifications from
// language servers. It is safe for concurrent use.
type DiagnosticStore struct {
	mu    sync.RWMutex
	byURI map[string][]gen.Diagnostic

	// waiters tracks channels waiting for diagnostics keyed by URI.
	waitMu  sync.Mutex
	waiters map[string][]chan struct{}
}

// NewDiagnosticStore creates a new empty diagnostic store.
func NewDiagnosticStore() *DiagnosticStore {
	return &DiagnosticStore{
		byURI:   make(map[string][]gen.Diagnostic),
		waiters: make(map[string][]chan struct{}),
	}
}

// HandlePublishDiagnostics processes a textDocument/publishDiagnostics notification.
// It replaces all stored diagnostics for the URI and wakes any waiters.
func (d *DiagnosticStore) HandlePublishDiagnostics(params gen.PublishDiagnosticsParams) {
	uri := params.URI

	d.mu.Lock()
	d.byURI[uri] = params.Diagnostics
	d.mu.Unlock()

	// Wake waiters for this URI.
	d.waitMu.Lock()
	waiters := d.waiters[uri]
	d.waiters[uri] = nil
	d.waitMu.Unlock()

	for _, ch := range waiters {
		close(ch)
	}
}

// HandleNotification is a JSON-RPC notification callback that dispatches
// textDocument/publishDiagnostics to HandlePublishDiagnostics.
func (d *DiagnosticStore) HandleNotification(method string, params json.RawMessage) {
	if method != "textDocument/publishDiagnostics" {
		return
	}
	var p gen.PublishDiagnosticsParams
	if err := json.Unmarshal(params, &p); err != nil {
		return
	}
	d.HandlePublishDiagnostics(p)
}

// GetDiagnostics returns the current diagnostics for a file URI.
func (d *DiagnosticStore) GetDiagnostics(uri string) []gen.Diagnostic {
	d.mu.RLock()
	defer d.mu.RUnlock()

	diags := d.byURI[uri]
	if diags == nil {
		return nil
	}
	// Return a copy to avoid data races.
	out := make([]gen.Diagnostic, len(diags))
	copy(out, diags)
	return out
}

// GetAllDiagnostics returns a snapshot of all stored diagnostics keyed by URI.
func (d *DiagnosticStore) GetAllDiagnostics() map[string][]gen.Diagnostic {
	d.mu.RLock()
	defer d.mu.RUnlock()

	out := make(map[string][]gen.Diagnostic, len(d.byURI))
	for uri, diags := range d.byURI {
		cp := make([]gen.Diagnostic, len(diags))
		copy(cp, diags)
		out[uri] = cp
	}
	return out
}

// WaitForDiagnostics blocks until diagnostics arrive for the given URI or
// the timeout expires. If the context is cancelled, it returns the context error.
func (d *DiagnosticStore) WaitForDiagnostics(ctx context.Context, uri string, timeout time.Duration) ([]gen.Diagnostic, error) {
	// Check if diagnostics already exist.
	if diags := d.GetDiagnostics(uri); len(diags) > 0 {
		return diags, nil
	}

	// Register a waiter channel.
	ch := make(chan struct{})
	d.waitMu.Lock()
	d.waiters[uri] = append(d.waiters[uri], ch)
	d.waitMu.Unlock()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-ch:
		return d.GetDiagnostics(uri), nil
	case <-timer.C:
		return nil, context.DeadlineExceeded
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Clear removes stored diagnostics for a URI.
func (d *DiagnosticStore) Clear(uri string) {
	d.mu.Lock()
	delete(d.byURI, uri)
	d.mu.Unlock()
}
