// integ_lookup_export.go — Phase 65 65-09 / WR-6 / BL-A test-fixture
// constructors + exported FixtureSymbolMeta type.
//
// Phase 65 65-12 deviation note (Rule 3): the file was originally named
// integ_lookup_export_for_test.go (with the `_test.go` suffix to
// guarantee exclusion from production binaries). 65-12 BL-1 surfaced
// that Go's test-binary visibility rule prevents cross-package
// consumption of `_test.go` symbols (the test binary for package
// internal/kernel/symbols cannot see daemon's `*_test.go` symbols).
// To preserve the BL-A cross-package access seam contract, the file
// was renamed (no `_test.go` suffix) and now lives in the regular
// build set. The constructor names (NewIntegSemanticLookupForTest,
// NewE2EIntegLookupForTest, FixtureSymbolMeta) make their
// test-fixture intent unambiguous; nothing in production code (any
// non-test daemon source) calls them.
//
// Cross-package consumers (now actually working post-65-12):
//   - internal/skill/semantic/integration_test.go
//     (TestE2E_StranglerFig_ProductionAdapter_SourceSemantic, 65-12 Task 4)
//     uses NewIntegSemanticLookupForTest with its own caller-built store.
//   - internal/kernel/symbols/bl1_blast_radius_e2e_test.go
//     (TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder, 65-12 Task 2
//     BL-1) uses NewE2EIntegLookupForTest with the fully-populated
//     fixture so every argument to the kernel-side BL-1 test sources
//     from the canonical syms map.
//
// Plan-vs-tree drift note: the plan referred to FixtureSymbolMeta.EdgeID
// as type integ.EdgeID, but no such type exists in the integ package
// today. To avoid blocking 65-09 on a separate type-introduction wave,
// EdgeID is declared as uint64 here — the value-shape is identical
// (overlay edge IDs are uint64 hashes via overlay.edgeIDForTriple) and
// downstream consumers in 65-12 read it as a numeric edge identifier.
package daemon

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// FixtureSymbolMeta describes either a fixture symbol (Path/Line/Col/
// SymbolID/StableKey set, EdgeID zero) or a fixture edge (EdgeID/StableKey
// set, Path/Line/Col/SymbolID zero). Cross-package callers (BL-A:
// 65-12 BL-1 ConfidenceLadder test) read entries by canonical key
// ("confirmed_edge_from", "confirmed_edge_to", "refuted_edge_from",
// "refuted_edge_to" for symbols; "confirmed_edge", "refuted_edge" for
// EdgeIDs) without re-grepping the fixture.
type FixtureSymbolMeta struct {
	Path      string
	Line      uint32
	Col       uint32
	SymbolID  integ.SymbolID
	EdgeID    uint64 // zero for symbol-shaped entries
	StableKey string
}

// NewIntegSemanticLookupForTest constructs an *integSemanticLookup that
// satisfies integ.SemanticLookup against a CALLER-BUILT *Store. Used by
// tests that already build their own store (e.g.,
// internal/skill/semantic/integration_test.go's newE2EHarness) and only
// need the production adapter wrapped around it.
//
// Test-only — see file header for the build-exclusion rationale.
func NewIntegSemanticLookupForTest(
	s *semanticstore.Store, ws workspace.WorkspaceKey,
) integ.SemanticLookup {
	return &integSemanticLookup{
		store:     s,
		enabledFn: func() bool { return true },
		wsKeyFn:   func() workspace.WorkspaceKey { return ws },
	}
}

// NewE2EIntegLookupForTest is the BL-A populated-harness builder. It
// wraps newE2EIntegLookup (the daemon-package-private fixture builder
// used by integ_lookup_e2e_test.go) and re-exposes its return tuple to
// external test packages. The return type uses the EXPORTED
// FixtureSymbolMeta so the kernel-side
// TestRegisterAnalyzeBlastRadius_E2E_ConfidenceLadder test (65-12 Task 2
// BL-1) can name the type and read canonical syms entries by string key.
//
// The fixture-build logic is NOT duplicated — newE2EIntegLookup is the
// single source of truth; this function exists purely as a
// cross-package access seam.
//
// Test-only — see file header for the build-exclusion rationale.
func NewE2EIntegLookupForTest(t *testing.T) (
	lookup *integSemanticLookup,
	ws workspace.WorkspaceKey,
	store *semanticstore.Store,
	syms map[string]FixtureSymbolMeta,
	cleanup func(),
) {
	t.Helper()
	return newE2EIntegLookup(t)
}
