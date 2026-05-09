// lookup.go — SemanticLookup interface (Phase 65 D-03 + Open Question #3
// resolution: SymbolID translation is part of the interface contract).
//
// All methods are READ-ONLY. The production adapter
// (internal/daemon/semantic_wiring.go integSemanticLookup) is enforced by
// a grep canary in the daemon test layer; the doc here states the contract
// for any future implementer (mocks, additional adapters).
//
// Mitigation linkage:
//   - M-readtier: every method must avoid snapshot-write surfaces
//     (BeginSnapshot/CommitSnapshot/AbortSnapshot/WriteSnapshotFacts/OnFlush/
//     BumpGraphVersion). The grep canary at
//     internal/daemon/integ_lookup_test.go enforces this on the production
//     adapter.
//   - M-cold: Available() must NOT trigger background indexing on cold start
//     (D-06). It reflects "are we configured + is the store live", nothing
//     more.
//   - WR-NEW-01: errors flowing through the interface are classified by
//     ClassifyLookupErr (source.go) before reaching the envelope; raw error
//     text never appears in MCP output.
package integ

import (
	"context"

	"github.com/agenthands/helix/internal/workspace"
)

// SemanticLookup is the read-only seam between Phase 65 MCP-tool consumers
// (get_repo_map, get_context, analyze_blast_radius, get_health) and the
// daemon-resident semantic engine. Concrete implementations live behind the
// daemon-side wiring; consumers receive a SemanticLookup from Setter
// post-init wiring (Pattern 1).
//
// Method contracts:
//
//   - Available reports whether the lookup is configured AND the underlying
//     store handle is live. False when semantic_index is disabled or the
//     daemon has not yet wired the production adapter (NoopLookup default).
//
//   - SymbolID translates an LSP file:line:col location into a stable
//     Phase 59 EXTRACT-02 SymbolID. Returns ErrNoSnapshot when no snapshot
//     has been committed yet, ErrIndexBuilding when a build is in flight
//     and the symbol-row read would race with WriteSnapshotFacts, or
//     ErrIndexErrored on any other lookup failure. (Open Question #3
//     resolution: kernel-side analyze_blast_radius needs this translation
//     and cannot import the store.)
//
//   - RankFiles returns the workspace-wide ranked file list for the default
//     "call_graph" projection (Phase 62 D-07). Sort order is highest-first;
//     consumers may slice the result for budget fitting. Returns
//     ErrNoSnapshot before the first commit and ErrBleveRebuilding when
//     retrieval-side recovery is in progress.
//
//   - RankFromSeeds returns a ranked file list biased toward the supplied
//     seeds (workspace-relative paths or symbol-anchor strings). Implemented
//     in Phase 65 65-05 as a Personalized PageRank / RRF fuse over the
//     bleve retrieval scores. Same error sentinels as RankFiles.
//
//   - ExpandFrom returns the depth-bounded blast-radius frontier rooted at
//     sym. depth is enforced by the implementation (Phase 65 65-06 caps at
//     2 by default). Each Impact carries Phase 62 D-12 confidence and the
//     originating evidence.
//
//   - ValidateCriticalEdges runs Pass 2 LSP validation for the supplied
//     edges. Implementations may return the input edges unchanged with
//     LSPConfirmed=false when Pass 2 is unavailable (e.g., the language
//     server is unhealthy); the consumer caps confidence at the fallback
//     ceiling (≤ 0.6 per D-08).
//
//     Phase 65 65-12 architectural note: production
//     ValidateCriticalEdges implementations are PASSTHROUGHs (return the
//     input edges with LSPConfirmed=false, no LSP traffic). The
//     kernel-side analyze_blast_radius orchestrator now performs the
//     Pass-2 LSP probe directly via FindReferences using the lease it
//     already holds; the daemon-side adapter cannot perform LSP work
//     without a back-call breach. Test fakes (matrixLookup,
//     fakeLookup) may still drive verdicts directly for unit-test
//     scenarios. See internal/kernel/symbols/blast_radius_strangler.go
//     lspProbeFn for the kernel-side probe.
//
//   - LocateSymbol returns the (path, line, col) of the symbol's
//     declaration at the latest committed snapshot. Used by the
//     kernel-side analyze_blast_radius Pass-2 LSP probe to derive the
//     (line, col) input for FindReferences from a stable SymbolID. line
//     and col are 1-based. (ok=false, err=nil) on miss; (ok=false,
//     err=non-nil) on error. (Phase 65 65-12 Task 1; closes the
//     SymbolID-only seam previously exposed by SemanticLookup so the
//     kernel can issue the LSP probe without re-resolving locations
//     against the cursor coordinates.)
//
//   - Status returns a closed-shape snapshot of the engine state for
//     get_health and per-call freshness reporting. Cheap (a few atomic
//     reads + one snapshot-id read). v1.10 does not cache.
type SemanticLookup interface {
	Available() bool
	SymbolID(ctx context.Context, ws workspace.WorkspaceKey, path string, line, col uint32) (SymbolID, error)
	RankFiles(ctx context.Context, ws workspace.WorkspaceKey) ([]RankedFile, error)
	RankFromSeeds(ctx context.Context, ws workspace.WorkspaceKey, seeds []string) ([]RankedFile, error)
	ExpandFrom(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID, depth int) ([]Impact, error)
	ValidateCriticalEdges(ctx context.Context, ws workspace.WorkspaceKey, edges []Edge) ([]ValidatedEdge, error)
	LocateSymbol(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (path string, line, col uint32, ok bool, err error)
	Status(ctx context.Context, ws workspace.WorkspaceKey) (SemanticStatus, error)

	// Visibility returns the access-level classification of sym at the latest
	// committed snapshot. Returns (VisUnknown, ErrUnsupported) when the
	// semantic index is unavailable (D-19 conservative fallback applies).
	// See internal/semantic/integ/visibility.go for the closed-enum values.
	// Phase 66 Plan 02 — OI-02 resolution.
	Visibility(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (Visibility, error)

	// IsEntrypointReachable returns true when sym is reachable from one of
	// the workspace's externally-callable entry points (e.g., main, HTTP
	// handlers, exported constructors in a library). Returns (false,
	// ErrUnsupported) when the semantic index is unavailable.
	// Phase 66 Plan 02 — OI-03 resolution (conservative heuristic; full
	// entry-point graph is a Phase 67 deliverable).
	IsEntrypointReachable(ctx context.Context, ws workspace.WorkspaceKey, sym SymbolID) (bool, error)
}
