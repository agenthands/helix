// Package lspenrich orchestrates async LSP enrichment of tree-sitter facts.
//
// Phase 61 introduces this package as the seam between the live-update
// pipeline (internal/semantic/live) and the kernel's LS worker pool
// (internal/kernel/lspool). The enrichment worker, the 2-lane priority
// queue, the LeaseAcquirer interface, and the closed-enum Outcome /
// MetricsSink / OverlayStore contracts shared by P02 (cascade) and P03
// (manager + status) all live here.
//
// ENRICH-01 invariant — IMPORT BOUNDARY:
//
// This package and its sub-packages MUST NOT import
// `github.com/agenthands/helix/internal/kernel`. Only the kernel's worker-
// pool seam (`github.com/agenthands/helix/internal/kernel/lspool`) and the
// workspace key type (`github.com/agenthands/helix/internal/workspace`) are
// permitted. Mechanically enforced by `internal/lint/nosemantic2kernel` —
// see `make vet`.
//
// Trace: 61-CONTEXT.md acceptance criterion #1; 61-01-PLAN.md must_haves
// truth #1.
package lspenrich
