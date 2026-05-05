package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/workspace"
)

// LeaseAcquirer is the kernel/semantic seam used by the LSP enrichment
// worker (Phase 61 P02). The interface is intentionally TWO methods only —
// one for acquiring a worker lease (mirrors *lspool.Pool.AcquireLease so
// the worker can call through without importing internal/kernel), and one
// for the foreground-busy preemption signal (Phase 61 D-04).
//
// 61-CONTEXT.md D-04 contract:
//
//   - AcquireLease delegates to the kernel pool with the same semantics
//     as the foreground tool path. Enrichment-side callers MUST set
//     sessionID with the prefix "lsp-enrichment:" so the pool can
//     distinguish enrichment leases from foreground tool calls (the
//     prefix is what gates the lastForegroundLease stamp).
//   - ForegroundBusy reports whether a non-enrichment lease was acquired
//     for wsKey within the configured yield_check_window
//     (default 200ms; daemon overrides via *lspool.Pool.SetYieldCheckWindow).
//     The worker invokes it between cascade steps; on true it abandons the
//     remainder of the per-file cascade and stamps
//     partial_reason="preempted".
//
// The interface is satisfied by *lspool.Pool directly — there is no
// adapter shim in P01. P03 daemon wiring may introduce a thin adapter for
// observability but the canonical implementation is *lspool.Pool itself.
//
// ENRICH-01 boundary: this file imports internal/kernel/lspool +
// internal/workspace ONLY. Adding a third internal/kernel/* import would
// fail the nosemantic2kernel vet analyzer.
type LeaseAcquirer interface {
	AcquireLease(ctx context.Context, sessionID string, wsKey workspace.WorkspaceKey, dirty bool) (*lspool.WorkerLease, error)
	ForegroundBusy(wsKey workspace.WorkspaceKey) bool
}
