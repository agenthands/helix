package lspenrich

import (
	"context"

	"github.com/agenthands/helix/internal/workspace"
)

// ReadinessProbe is the per-language readiness gate consulted by the
// enrichment worker BEFORE it acquires a lease. Phase 61 D-08:
//
//   - Java workspaces (jdtls) MUST observe ServiceReady + ProjectStatus=OK
//     before any cascade-level LSP call; calling textDocument/documentSymbol
//     on a not-yet-ready jdtls returns empty results that look indistin-
//     guishable from a no-symbols file. WaitUntilJavaReady is the existing
//     kernel-side wrapper (see internal/kernel/lspool/quirks.go:423).
//
//   - Rust workspaces (rust-analyzer) MUST observe
//     experimental/serverStatus.quiescent=true before the cascade fires; a
//     mid-indexing rust-analyzer returns partial results that mis-represent
//     symbol locations.
//
//   - Every other language is best-effort: the lease-acquire path itself
//     fails with serr.ErrCircuitOpen / ErrMaxWorkersReached when the LS
//     install is broken, so WaitForLanguageReady returns nil immediately
//     and the AcquireLease step is the gate (W4 — broken-LS-install
//     produces outcome=dropped, NOT outcome=applied).
//
// The interface is hosted in P02 because P02 is the sole caller; P03's
// Manager constructs the production implementation by adapting the kernel
// pool's per-(wsKey, lang) JdtlsAdapter / RustAnalyzerAdapter.
type ReadinessProbe interface {
	// JavaReady blocks until jdtls observes BOTH ServiceReady AND
	// ProjectStatus=OK for wsKey, OR until ctx expires (in which case
	// JavaReady returns ctx.Err() — propagated by WaitForLanguageReady).
	JavaReady(ctx context.Context, wsKey workspace.WorkspaceKey) error

	// RustQuiescent blocks until rust-analyzer reports
	// experimental/serverStatus.quiescent=true for wsKey, OR until ctx
	// expires.
	RustQuiescent(ctx context.Context, wsKey workspace.WorkspaceKey) error
}

// WaitForLanguageReady is the dispatch entry-point invoked by the worker
// BEFORE its first per-file LSP call. Returns nil for unknown languages
// (best-effort fall-through — AcquireLease is the gate).
//
// Phase 61 acceptance #10: Java jobs MUST go through JavaReady; Rust jobs
// MUST go through RustQuiescent; everything else returns nil immediately.
//
// W4: this function does NOT mask AcquireLease errors. For a broken-LS-
// install scenario on a best-effort language (e.g. perl with no perl LS),
// WaitForLanguageReady returns nil and the worker observes the
// AcquireLease error — that worker classifies the result as
// outcome=dropped (NOT applied). See worker.go processOne switch.
func WaitForLanguageReady(ctx context.Context, p ReadinessProbe, lang string, wsKey workspace.WorkspaceKey) error {
	if p == nil {
		return nil
	}
	switch lang {
	case "java":
		return p.JavaReady(ctx, wsKey)
	case "rust":
		return p.RustQuiescent(ctx, wsKey)
	default:
		return nil
	}
}
