// Package kernel notifier.go declares the EditNotifier interface and the
// Kernel-side setter/accessor pair that lets external packages (Phase 60
// semantic/live) receive a fire-and-forget signal whenever a kernel edit
// tool successfully writes to a workspace file.
//
// The interface lives inside internal/kernel/ so kernel handlers can call
// it WITHOUT importing internal/semantic/* (LIVE-07 invariant #1; enforced
// by cmd/vet-nokernel2semantic from Phase 60 P01).
//
// 60-CONTEXT.md D-03 contract:
//   - OnEdit MUST return immediately after enqueueing into the per-
//     workspace coalescer; no extraction, no I/O, no blocking.
//   - OnEdit errors are non-fatal; edit-tool callers swallow them.
package kernel

import (
	"context"

	"github.com/agenthands/helix/internal/workspace"
)

// EditNotifier is the interface a downstream consumer (Phase 60
// semantic/live service) implements to receive notifications that a kernel
// edit tool has successfully written to one or more workspace files.
//
// OnEdit MUST return in O(microseconds) — implementations enqueue into
// a non-blocking coalescer and return immediately. The watcher and
// manifest scanner provide the correctness story; OnEdit is purely
// a latency optimization.
type EditNotifier interface {
	OnEdit(ctx context.Context, workspaceID workspace.WorkspaceKey, paths []string) error
}

// SetEditNotifier installs n as the active notifier. Subsequent calls
// replace the value (last write wins). Safe for concurrent readers via
// the underlying atomic.Value.
//
// The daemon calls this once at bootstrap after constructing the
// Phase 60 live service (mirrors RepoMapSkill.SetEnrichFn).
func (k *Kernel) SetEditNotifier(n EditNotifier) {
	// Wrap in a typed holder so the atomic.Value type is consistent
	// across both nil and non-nil writes (atomic.Value rejects mixed
	// concrete types; the holder normalizes the stored type).
	k.editNotifier.Store(editNotifierHolder{n: n})
}

// EditNotifier returns the currently-installed notifier, or nil if none
// has been set. Callers MUST nil-check before invoking OnEdit:
//
//	if n := k.EditNotifier(); n != nil {
//	    _ = n.OnEdit(ctx, wsKey, paths)
//	}
func (k *Kernel) EditNotifier() EditNotifier {
	v := k.editNotifier.Load()
	if v == nil {
		return nil
	}
	h, ok := v.(editNotifierHolder)
	if !ok {
		return nil
	}
	return h.n
}

// editNotifierHolder boxes an EditNotifier so atomic.Value sees a
// consistent concrete type even when the stored value is the typed-nil
// interface (otherwise atomic.Value rejects mixed concrete types).
type editNotifierHolder struct {
	n EditNotifier
}
