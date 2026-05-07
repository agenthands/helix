// Package kernel: Phase 63 P63-02 Task 1 — active-edit-tx counter.
//
// edit_tx_count.go owns the per-workspace in-memory counter the Phase 63
// compaction gate consumes for BlockedEditTxActive. The 8 edit tools
// (replace_symbol_body, insert_before_symbol, insert_after_symbol,
// rename_symbol, safe_delete_symbol, replace_in_file, fuzzy_edit,
// write_file) wrap their Handle bodies with `defer
// k.BeginEditTx(ws)()` so the gate sees an in-flight count > 0 for the
// duration of any active edit.
//
// CONTEXT.md D-04 hard invariant: ActiveEditTxCount returns from
// in-memory atomic state — NO I/O. The counter is lazily installed
// per-workspace under editTxMu so workspaces that never see an edit do
// not allocate state.
//
// Lifecycle: BeginEditTx returns a release closure that decrements the
// counter on call. Callers MUST invoke the release exactly once
// (defer-style); skipping it leaks a phantom "+1" forever and would
// cause the gate to indefinitely report BlockedEditTxActive.

package kernel

import (
	"sync"
	"sync/atomic"

	"github.com/agenthands/helix/internal/workspace"
)

// editTxRegistry holds the per-workspace counters. Decoupled from
// *Kernel so it can be embedded without disturbing the existing struct
// shape; *Kernel exposes ActiveEditTxCount / BeginEditTx as thin
// forwarders. nil-safe under read access via the registry's nil-check
// pattern.
type editTxRegistry struct {
	mu       sync.Mutex
	counters map[string]*atomic.Int32
}

// editTx is a process-global registry. Per-Kernel attachment is via
// activeEditTxRegistry on the Kernel struct (see below); a global
// registry preserves the simple API even if a future refactor splits
// the kernel into multiple instances inside a single process.
var editTx = &editTxRegistry{}

// counterFor returns (and lazily installs) the atomic.Int32 counter for
// repoRoot.
func (r *editTxRegistry) counterFor(repoRoot string) *atomic.Int32 {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.counters == nil {
		r.counters = map[string]*atomic.Int32{}
	}
	c, ok := r.counters[repoRoot]
	if !ok {
		c = &atomic.Int32{}
		r.counters[repoRoot] = c
	}
	return c
}

// load returns the current counter value for repoRoot, or 0 if the
// counter has never been installed.
func (r *editTxRegistry) load(repoRoot string) int32 {
	r.mu.Lock()
	c, ok := r.counters[repoRoot]
	r.mu.Unlock()
	if !ok || c == nil {
		return 0
	}
	return c.Load()
}

// ActiveEditTxCount returns the number of currently-in-flight edit-tool
// Handle bodies for ws.RepoRoot. Phase 63 P63-02 Task 1: O(1) atomic
// read; CONTEXT.md D-04 hard invariant — NO I/O.
//
// nil-safe: a nil receiver returns 0.
func (k *Kernel) ActiveEditTxCount(ws workspace.WorkspaceKey) int {
	if k == nil {
		return 0
	}
	return int(editTx.load(ws.RepoRoot))
}

// BeginEditTx increments the per-workspace edit-tx counter and returns
// a release closure that decrements it. Callers MUST defer-invoke the
// release; skipping it leaks a phantom +1 and would cause the
// compaction gate to indefinitely report BlockedEditTxActive.
//
// Idempotent across the release closure (sync.Once-guarded) — calling
// release() twice does NOT double-decrement.
//
// Pattern (load-bearing for all 8 edit tools):
//
//	wsKey := wsKeyFn()
//	defer k.BeginEditTx(wsKey)()
func (k *Kernel) BeginEditTx(ws workspace.WorkspaceKey) func() {
	if k == nil {
		return func() {}
	}
	c := editTx.counterFor(ws.RepoRoot)
	c.Add(1)
	var once sync.Once
	return func() {
		once.Do(func() { c.Add(-1) })
	}
}
