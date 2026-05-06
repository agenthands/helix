package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/workspace"
)

// PoolReadinessProbe is the production ReadinessProbe that wraps
// *lspool.Pool's per-(wsKey, lang) JdtlsAdapter / RustAnalyzerAdapter
// accessors (added in P03-T4 — W8).  Both methods short-circuit to nil when
// the workspace has no Java / Rust worker, so the AcquireLease step becomes
// the gate for languages outside the Java/Rust readiness machinery.
type PoolReadinessProbe struct {
	Pool *lspool.Pool
}

// NewPoolReadinessProbe constructs a PoolReadinessProbe wrapping the given
// *Pool.  Nil pool is treated as a no-op probe (both methods return nil).
func NewPoolReadinessProbe(p *lspool.Pool) *PoolReadinessProbe {
	return &PoolReadinessProbe{Pool: p}
}

// JavaReady blocks until jdtls observes BOTH ServiceReady AND
// ProjectStatus=OK for wsKey, OR until ctx expires.  Returns nil when Java
// is not active in the workspace (the AcquireLease step is the gate).
//
// W8: uses the real *lspool.Pool.JdtlsAdapter accessor (no pseudo-comments).
func (p *PoolReadinessProbe) JavaReady(ctx context.Context, wsKey workspace.WorkspaceKey) error {
	if p == nil || p.Pool == nil {
		return nil
	}
	adapter := p.Pool.JdtlsAdapter(wsKey)
	if adapter == nil {
		return nil
	}
	return adapter.WaitUntilJavaReady(ctx)
}

// RustQuiescent blocks until rust-analyzer reports quiescent=true via
// experimental/serverStatus, OR until ctx expires.  Returns nil when Rust is
// not active in the workspace.
//
// W8: uses the real *lspool.Pool.RustAnalyzerAdapter accessor + the new
// QuiescentChan() method (no pseudo-comments).
func (p *PoolReadinessProbe) RustQuiescent(ctx context.Context, wsKey workspace.WorkspaceKey) error {
	if p == nil || p.Pool == nil {
		return nil
	}
	adapter := p.Pool.RustAnalyzerAdapter(wsKey)
	if adapter == nil {
		return nil
	}
	select {
	case <-adapter.QuiescentChan():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Compile-time assertion: PoolReadinessProbe satisfies ReadinessProbe.
var _ ReadinessProbe = (*PoolReadinessProbe)(nil)
