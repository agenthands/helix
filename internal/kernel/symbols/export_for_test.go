// export_for_test.go — Phase 65 65-12 Task 2 BL-1 cross-package access
// seam. The bl1_blast_radius_e2e_test.go test (in the EXTERNAL test
// package `symbols_test`) consumes the production analyze_blast_radius
// orchestrator + Pass-2 LSP probe through the re-exports declared here.
//
// Naming: this file uses the `_test.go` suffix in `package symbols` (NOT
// `package symbols_test`). Per the standard Go build convention, files
// named `*_test.go` are compiled into test binaries only — never into
// production binaries. No build tag is needed; no production leakage
// occurs (mirroring internal/daemon/integ_lookup_export_for_test.go from
// 65-09).
//
// The re-exports forward verbatim to the unexported originals:
//   - AnalyzeBlastRadiusViaLookupForTest → analyzeBlastRadiusViaLookup
//   - LspProbeForEdgesForTest → lspProbeForEdges
//   - PathToURIForTest → pathToURI
//
// The test binary therefore exercises the production code paths with
// zero internal-API leakage.
package symbols

import (
	"context"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
)

// AnalyzeBlastRadiusViaLookupForTest is the test-only re-export of
// analyzeBlastRadiusViaLookup. Identical signature; identical behavior.
func AnalyzeBlastRadiusViaLookupForTest(
	ctx context.Context,
	lookup integ.SemanticLookup,
	ws workspace.WorkspaceKey,
	sym integ.SymbolID,
	lspProbeFn func(context.Context, []integ.Edge) []integ.ValidatedEdge,
) ([]integ.Impact, integ.Source, integ.FallbackReason, uint64, error) {
	return analyzeBlastRadiusViaLookup(ctx, lookup, ws, sym, lspProbeFn)
}

// LspProbeForEdgesForTest is the test-only re-export of lspProbeForEdges.
func LspProbeForEdgesForTest(
	ctx context.Context,
	edges []integ.Edge,
	locator func(integ.SymbolID) (path string, line, col uint32, ok bool),
	probeFn func(ctx context.Context, uri string, line, col int) ([]SymbolLocation, error),
	repoRoot string,
) []integ.ValidatedEdge {
	return lspProbeForEdges(ctx, edges, locator, probeFn, repoRoot)
}

// PathToURIForTest is the test-only re-export of pathToURI used to
// construct synthetic SymbolLocation URIs in BL-1 fixtures.
func PathToURIForTest(root, path string) string {
	return pathToURI(root, path)
}
