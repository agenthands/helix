// Multi-language populated-graph fixture builder for Phase 71 handler plans.
//
// DO NOT inline-copy this fixture in per-tool test files. Each Phase 71 wave-2
// plan (71-03 explain_symbol_deep, 71-04 find_related_symbols, 71-05
// validate_graph_edge) must invoke buildPopulatedGraphFixture(t) to receive a
// fresh, race-clean fixture with the canonical Go / TypeScript / Java symbols
// and CALLS / RESOLVES_TO / USES_TYPE edges this phase's success criterion
// (multi-language coverage) requires.

package semantic

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/integ"
)

// PopulatedGraphFixture is a self-contained, in-memory multi-language graph
// fixture exposing the three Phase 71 accessor seams. The backing data is
// table-driven (no real *Store dependency) so the fixture remains race-
// clean under -race and compiles independently of the duckdb platform
// constraint.
//
// Layout (one symbol per language plus cross-language edges):
//
//	Go:         repo/src/svc.go::ServeHTTP        (CALLS) → repo/src/svc.go::handle
//	            repo/src/svc.go::handle           (USES_TYPE) → repo/src/types.go::Request
//	TypeScript: repo/web/api.ts::fetchUser        (RESOLVES_TO) → repo/web/types.ts::User
//	Java:       repo/api/Foo.java::Foo.bar       (USES_TYPE) → repo/api/Bar.java::Bar
//
// Every symbol's stable_key serves as its SymbolID.
type PopulatedGraphFixture struct {
	// Seed symbol IDs, exported for cross-test reuse.
	GoSeedSymbolID   integ.SymbolID
	TSSeedSymbolID   integ.SymbolID
	JavaSeedSymbolID integ.SymbolID

	// RepoID is the workspace.WorkspaceKey.Hash() the fixture expects.
	RepoID string

	// Accessors satisfy the three Phase 71 seam interfaces. Each accessor
	// is keyed on the (path, name) tuple stored at fixture-build time.
	SymbolByName      SymbolByNameAccessor
	ExtractorRun      ExtractorRunAccessor
	ClusterMembership ClusterMembershipAccessor
}

// fixtureSymbol is one entry in the in-memory table.
type fixtureSymbol struct {
	path     string
	name     string
	stableID integ.SymbolID
	language string
}

// fixtureSymbolByName implements SymbolByNameAccessor over a static table.
// Lookup is keyed on (path, name) and returns all matching stable IDs.
type fixtureSymbolByName struct {
	rows []fixtureSymbol
}

func (f *fixtureSymbolByName) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
	var out []integ.SymbolID
	for _, r := range f.rows {
		if r.path == path && r.name == name {
			out = append(out, r.stableID)
		}
	}
	return out, nil
}

// fixtureExtractorRun returns a static, monotonic id.
type fixtureExtractorRun struct {
	runID string
}

func (f *fixtureExtractorRun) LatestExtractorRunID(ctx context.Context, repoID string) (string, error) {
	return f.runID, nil
}

// fixtureClusterMembership returns a per-symbol cluster_id keyed on the
// fixture's deterministic map. Symbols not in the map fall back to (0, 0, nil)
// — the same shape the production "cluster_boost_unavailable" path uses.
type fixtureClusterMembership struct {
	clusters map[integ.SymbolID]struct {
		id   uint64
		size int
	}
}

func (f *fixtureClusterMembership) ClusterIDOf(ctx context.Context, repoID string, symbolID integ.SymbolID) (uint64, int, error) {
	if c, ok := f.clusters[symbolID]; ok {
		return c.id, c.size, nil
	}
	return 0, 0, nil
}

// buildPopulatedGraphFixture returns a fresh multi-language graph fixture
// suitable for Phase 71 handler tests. Subsequent plans MUST reuse this
// helper rather than constructing equivalent fixtures inline.
func buildPopulatedGraphFixture(t *testing.T) *PopulatedGraphFixture {
	t.Helper()

	goSvcServe := fixtureSymbol{
		path: "repo/src/svc.go", name: "ServeHTTP",
		stableID: "repo/src/svc.go::ServeHTTP", language: "go",
	}
	goSvcHandle := fixtureSymbol{
		path: "repo/src/svc.go", name: "handle",
		stableID: "repo/src/svc.go::handle", language: "go",
	}
	goTypeRequest := fixtureSymbol{
		path: "repo/src/types.go", name: "Request",
		stableID: "repo/src/types.go::Request", language: "go",
	}
	tsFetchUser := fixtureSymbol{
		path: "repo/web/api.ts", name: "fetchUser",
		stableID: "repo/web/api.ts::fetchUser", language: "typescript",
	}
	tsUser := fixtureSymbol{
		path: "repo/web/types.ts", name: "User",
		stableID: "repo/web/types.ts::User", language: "typescript",
	}
	javaFooBar := fixtureSymbol{
		path: "repo/api/Foo.java", name: "Foo.bar",
		stableID: "repo/api/Foo.java::Foo.bar", language: "java",
	}
	javaBar := fixtureSymbol{
		path: "repo/api/Bar.java", name: "Bar",
		stableID: "repo/api/Bar.java::Bar", language: "java",
	}

	rows := []fixtureSymbol{
		goSvcServe, goSvcHandle, goTypeRequest,
		tsFetchUser, tsUser,
		javaFooBar, javaBar,
	}

	return &PopulatedGraphFixture{
		GoSeedSymbolID:   goSvcServe.stableID,
		TSSeedSymbolID:   tsFetchUser.stableID,
		JavaSeedSymbolID: javaFooBar.stableID,
		RepoID:           "fixture-repo",
		SymbolByName:     &fixtureSymbolByName{rows: rows},
		ExtractorRun:     &fixtureExtractorRun{runID: "snap-1"},
		ClusterMembership: &fixtureClusterMembership{
			clusters: map[integ.SymbolID]struct {
				id   uint64
				size int
			}{
				goSvcServe.stableID:  {id: 1, size: 2},
				goSvcHandle.stableID: {id: 1, size: 2},
				tsFetchUser.stableID: {id: 2, size: 1},
				javaFooBar.stableID:  {id: 3, size: 1},
			},
		},
	}
}

// TestPopulatedGraphFixture_MultiLanguage asserts the fixture exposes at
// least one Go, one TypeScript, and one Java symbol resolvable through the
// SymbolByName accessor.
func TestPopulatedGraphFixture_MultiLanguage(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	ctx := context.Background()

	goRows, err := fx.SymbolByName.QuerySymbolByName(ctx, fx.RepoID, "repo/src/svc.go", "ServeHTTP")
	if err != nil {
		t.Fatalf("Go QuerySymbolByName: %v", err)
	}
	if len(goRows) != 1 || goRows[0] != fx.GoSeedSymbolID {
		t.Errorf("Go: got %v, want [%v]", goRows, fx.GoSeedSymbolID)
	}

	tsRows, err := fx.SymbolByName.QuerySymbolByName(ctx, fx.RepoID, "repo/web/api.ts", "fetchUser")
	if err != nil {
		t.Fatalf("TS QuerySymbolByName: %v", err)
	}
	if len(tsRows) != 1 || tsRows[0] != fx.TSSeedSymbolID {
		t.Errorf("TS: got %v, want [%v]", tsRows, fx.TSSeedSymbolID)
	}

	javaRows, err := fx.SymbolByName.QuerySymbolByName(ctx, fx.RepoID, "repo/api/Foo.java", "Foo.bar")
	if err != nil {
		t.Fatalf("Java QuerySymbolByName: %v", err)
	}
	if len(javaRows) != 1 || javaRows[0] != fx.JavaSeedSymbolID {
		t.Errorf("Java: got %v, want [%v]", javaRows, fx.JavaSeedSymbolID)
	}

	// ExtractorRun returns a non-empty id.
	runID, err := fx.ExtractorRun.LatestExtractorRunID(ctx, fx.RepoID)
	if err != nil {
		t.Fatalf("LatestExtractorRunID: %v", err)
	}
	if runID == "" {
		t.Errorf("got empty runID, want non-empty")
	}

	// ClusterMembership returns a real cluster_id for a known seed and
	// (0, 0, nil) for an unknown symbol (the cluster-boost-disabled path).
	clusterID, size, err := fx.ClusterMembership.ClusterIDOf(ctx, fx.RepoID, fx.GoSeedSymbolID)
	if err != nil {
		t.Fatalf("ClusterIDOf(Go): %v", err)
	}
	if clusterID == 0 || size == 0 {
		t.Errorf("Go seed: got (cluster=%d,size=%d), want non-zero", clusterID, size)
	}
	zeroID, zeroSize, err := fx.ClusterMembership.ClusterIDOf(ctx, fx.RepoID, "unknown_symbol")
	if err != nil {
		t.Fatalf("ClusterIDOf(unknown): %v", err)
	}
	if zeroID != 0 || zeroSize != 0 {
		t.Errorf("unknown symbol: got (cluster=%d,size=%d), want (0,0)", zeroID, zeroSize)
	}
}
