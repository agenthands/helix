package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/workspace"
)

// PoolAcquirer is the production adapter that satisfies LeaseAcquirer +
// LeaseReleaser by forwarding directly to *lspool.Pool.  Phase 61 P03 wires
// this in the daemon bootstrap (live_wiring.go).
//
// The compile-time assertion below ensures the adapter stays in sync with
// the LeaseAcquirer interface — adding a method to LeaseAcquirer that
// PoolAcquirer doesn't satisfy will fail the build at this site rather than
// at the daemon-wiring callsite.
//
// **Carry-over from 61-02 (61-02-SUMMARY.md "Next Phase Readiness")**: the
// production CASCADE adapter (the CascadeLSPFactory wrapping *WorkerLease.
// Request) MUST tolerate gopls/jdtls soft errors as nil-result, not as
// LS-unavailable.  That tolerance lives in the cascade-LSP shim adapter
// (separate file — Phase 64 daemon wiring will own the production shim);
// PoolAcquirer ITSELF only routes lease lifecycle and never inspects LSP
// payload errors, so the soft-error semantics do not apply at this layer.
type PoolAcquirer struct {
	Pool *lspool.Pool
}

// NewPoolAcquirer constructs a PoolAcquirer wrapping the given *Pool.  Nil
// pool is permitted (degraded-mode bootstrap); any AcquireLease call on a
// nil pool will panic deterministically — caller responsibility.
func NewPoolAcquirer(p *lspool.Pool) *PoolAcquirer {
	return &PoolAcquirer{Pool: p}
}

// AcquireLease forwards to *lspool.Pool.AcquireLease verbatim.
func (a *PoolAcquirer) AcquireLease(
	ctx context.Context,
	sessionID string,
	wsKey workspace.WorkspaceKey,
	dirty bool,
) (*lspool.WorkerLease, error) {
	return a.Pool.AcquireLease(ctx, sessionID, wsKey, dirty)
}

// ForegroundBusy forwards to *lspool.Pool.ForegroundBusy verbatim.
func (a *PoolAcquirer) ForegroundBusy(wsKey workspace.WorkspaceKey) bool {
	return a.Pool.ForegroundBusy(wsKey)
}

// ReleaseLease forwards to *lspool.Pool.ReleaseLease.  Manager calls this
// via the LeaseReleaser optional interface during OnWorkspaceDeactivate +
// Stop to honor the B2 lease lifecycle invariant.
func (a *PoolAcquirer) ReleaseLease(sessionID string) {
	a.Pool.ReleaseLease(sessionID)
}

// Compile-time assertion: PoolAcquirer satisfies LeaseAcquirer (Phase 61
// kernel-semantic seam) AND LeaseReleaser (B2 lease lifecycle).
var (
	_ LeaseAcquirer = (*PoolAcquirer)(nil)
	_ LeaseReleaser = (*PoolAcquirer)(nil)
)
