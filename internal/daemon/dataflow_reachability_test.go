package daemon

import (
	"context"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/obs"
	semanticpkg "github.com/agenthands/helix/internal/semantic"
	"github.com/agenthands/helix/internal/semantic/integ"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/workspace"
)

// openDataFlowStore opens a real *Store in a temp workspace, hands the
// workspace's repoID to factsFn so the Facts are stamped consistently, commits
// them, and returns (store, repoID, cleanup). The SAME repoID must drive the
// later ReachableFrom query — factsFn threads it so facts/commit/query agree.
// (For driving the REAL adapter end-to-end — red-team B4.)
func openDataFlowStore(t *testing.T, factsFn func(repoID string) semanticstore.Facts) (*semanticstore.Store, string, func()) {
	t.Helper()
	wsDir := t.TempDir()
	t.Chdir(wsDir)
	ws := workspace.WorkspaceKey{RepoRoot: wsDir, Language: "go", Toolchain: "go1.22"}
	repoID := ws.Hash()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	provider := obs.Noop(logger.Handler())
	cfg := semanticpkg.Config{
		Enabled: true,
		Store: semanticpkg.StoreConfig{
			Kind: "duckdb", Path: filepath.Join(".helix", "semantic.duckdb"),
			MemoryLimit: "256MiB", Threads: 2,
		},
	}
	store, err := semanticstore.Open(context.Background(), cfg, logger, provider.Metrics())
	if err != nil {
		t.Fatalf("semanticstore.Open: %v", err)
	}
	if _, err := commitFixtureSnapshot(context.Background(), store, repoID, factsFn(repoID)); err != nil {
		_ = store.Close()
		t.Fatalf("commitFixtureSnapshot: %v", err)
	}
	return store, repoID, func() { _ = store.Close() }
}

// dataFlowFacts builds a Facts with one file, the named param symbols, and
// DATA_FLOWS edges over the given ordered pairs. EdgeIDs stamped dense 1-based
// (WriteSnapshotFacts inserts EdgeID verbatim with no allocator — v2.8 PK
// discipline). stableKey prefixes "sk:".
func dataFlowFacts(t *testing.T, repoID string, paramNames []string, flows [][2]string) semanticstore.Facts {
	t.Helper()
	files := []semanticstore.FileFact{{FileID: 1, RepoID: repoID, Path: "src/flow.go", Language: "go", ContentHash: "h", SizeBytes: 100, LineCount: 10}}
	nodeOf := map[string]uint64{}
	var symbols []semanticstore.SymbolFact
	for i, name := range paramNames {
		id := uint64(0x2000_0000_0000_0000 + uint64(i+1))
		id &= 0x7FFFFFFFFFFFFFFF
		nodeOf[name] = id
		symbols = append(symbols, semanticstore.SymbolFact{
			SymbolID: id, NodeID: id, FileID: 1, Language: "go",
			Kind: "parameter", Name: name, QualifiedName: name,
			StableKey: "sk:" + name, StartLine: i + 1, StartCol: 1, EndLine: i + 1, EndCol: 1,
			Visibility: "public", Confidence: 1.0,
		})
	}
	var edges []semanticstore.EdgeFact
	for i, f := range flows {
		edges = append(edges, semanticstore.EdgeFact{
			EdgeID: uint64(i + 1), SrcNodeID: nodeOf[f[0]], DstNodeID: nodeOf[f[1]],
			EdgeKind: "DATA_FLOWS", Source: "def_use", Confidence: 0.55,
		})
	}
	return semanticstore.Facts{Files: files, Symbols: symbols, Edges: edges}
}

// TestDataFlowReachability_MultiHop drives the REAL adapter: a src->mid->sink
// DATA_FLOWS chain, seeded at the src PARAM, reaches sink at hop 2.
func TestDataFlowReachability_MultiHop(t *testing.T) {
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		return dataFlowFacts(t, r, []string{"src", "mid", "sink"}, [][2]string{{"src", "mid"}, {"mid", "sink"}})
	})
	defer cleanup()
	a := NewP1DataFlowReachabilityAdapterForTest(store)

	got, err := a.ReachableFrom(context.Background(), repoID, integ.SymbolID("sk:src"), 5)
	if err != nil {
		t.Fatalf("ReachableFrom: %v", err)
	}
	hops := map[string]int{}
	for _, n := range got {
		hops[string(n.SymbolID)] = n.Hops
	}
	if hops["sk:src"] != 0 {
		t.Errorf("seed src hops = %d, want 0", hops["sk:src"])
	}
	if hops["sk:mid"] != 1 {
		t.Errorf("mid hops = %d, want 1", hops["sk:mid"])
	}
	if hops["sk:sink"] != 2 {
		t.Errorf("sink hops = %d, want 2 (src->mid->sink); reachable set: %+v", hops["sk:sink"], got)
	}
}

// TestDataFlowReachability_HopCap: maxHops=1 reaches mid but NOT sink.
func TestDataFlowReachability_HopCap(t *testing.T) {
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		return dataFlowFacts(t, r, []string{"src", "mid", "sink"}, [][2]string{{"src", "mid"}, {"mid", "sink"}})
	})
	defer cleanup()
	a := NewP1DataFlowReachabilityAdapterForTest(store)

	got, _ := a.ReachableFrom(context.Background(), repoID, integ.SymbolID("sk:src"), 1)
	for _, n := range got {
		if string(n.SymbolID) == "sk:sink" {
			t.Errorf("sink reached at maxHops=1; want unreachable (it's hop 2): %+v", got)
		}
	}
}

// TestDataFlowReachability_BrokenChain: no mid->sink edge => sink unreachable.
func TestDataFlowReachability_BrokenChain(t *testing.T) {
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		return dataFlowFacts(t, r, []string{"src", "mid", "sink"}, [][2]string{{"src", "mid"}}) // no mid->sink
	})
	defer cleanup()
	a := NewP1DataFlowReachabilityAdapterForTest(store)

	got, _ := a.ReachableFrom(context.Background(), repoID, integ.SymbolID("sk:src"), 5)
	for _, n := range got {
		if string(n.SymbolID) == "sk:sink" {
			t.Errorf("sink reached despite broken mid->sink hop: %+v", got)
		}
	}
}

// TestDataFlowReachability_FunctionSeedReachesInBody: the corrected v2.13 truth.
// Pre-v2.13 a function seed reached only itself (DATA_FLOWS was purely
// param-anchored). Post-v2.13 DATA_FLOWS carries function->param
// (def_use_inbody) edges, so a FUNCTION seed now mechanically reaches its
// in-body target over the kind-agnostic node-ID BFS. This asserts the NEW truth
// (the M3 now-false emptiness invariant is corrected, not re-encoded).
func TestDataFlowReachability_FunctionSeedReachesInBody(t *testing.T) {
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		fnID := uint64(0x3000_0000_0000_0001) & 0x7FFFFFFFFFFFFFFF
		paramID := uint64(0x3000_0000_0000_0002) & 0x7FFFFFFFFFFFFFFF
		files := []semanticstore.FileFact{{FileID: 1, RepoID: r, Path: "src/flow.go", Language: "go", ContentHash: "h", SizeBytes: 100, LineCount: 10}}
		symbols := []semanticstore.SymbolFact{
			{SymbolID: fnID, NodeID: fnID, FileID: 1, Language: "go", Kind: "function", Name: "producer", QualifiedName: "producer", StableKey: "sk:producer", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 1, Visibility: "public", Confidence: 1.0},
			{SymbolID: paramID, NodeID: paramID, FileID: 1, Language: "go", Kind: "parameter", Name: "v", QualifiedName: "v", StableKey: "sk:v", StartLine: 2, StartCol: 1, EndLine: 2, EndCol: 1, Visibility: "public", Confidence: 1.0},
		}
		// A def_use_inbody edge crosses from the producer FUNCTION node into a
		// consumer PARAMETER node — the exact shape that makes a function seed
		// non-empty post-v2.13.
		edges := []semanticstore.EdgeFact{
			{EdgeID: 1, SrcNodeID: fnID, DstNodeID: paramID, SrcKind: "function", DstKind: "parameter", EdgeKind: "DATA_FLOWS", Source: "def_use_inbody", Confidence: 0.50, Weight: 0.5},
		}
		return semanticstore.Facts{Files: files, Symbols: symbols, Edges: edges}
	})
	defer cleanup()
	a := NewP1DataFlowReachabilityAdapterForTest(store)

	got, err := a.ReachableFrom(context.Background(), repoID, integ.SymbolID("sk:producer"), 5)
	if err != nil {
		t.Fatalf("ReachableFrom: %v", err)
	}
	hops := map[string]int{}
	for _, n := range got {
		hops[string(n.SymbolID)] = n.Hops
	}
	if hops["sk:producer"] != 0 {
		t.Errorf("seed producer hops = %d, want 0", hops["sk:producer"])
	}
	reached, ok := hops["sk:v"]
	if !ok {
		t.Fatalf("function seed did NOT reach in-body target sk:v — post-v2.13 a function->param def_use_inbody edge MUST be traversable; got %+v", got)
	}
	if reached != 1 {
		t.Errorf("in-body target sk:v hops = %d, want 1 (function->param in-body edge); got %+v", reached, got)
	}
}

// TestDataFlowReachability_FunctionAnchoredNoError (FLOW-05f smoke): the
// param-seeded BFS traverses a snapshot mixing param->param, function->param
// (def_use_inbody) and param->function (def_use_return) edges WITHOUT error —
// function-node endpoints must not crash the walk. Chain:
// srcFn -(in-body)-> tParam -(return-bridge)-> tFn -(in-body)-> sinkParam.
func TestDataFlowReachability_FunctionAnchoredNoError(t *testing.T) {
	store, repoID, cleanup := openDataFlowStore(t, func(r string) semanticstore.Facts {
		srcFn := uint64(0x4000_0000_0000_0001) & 0x7FFFFFFFFFFFFFFF
		tParam := uint64(0x4000_0000_0000_0002) & 0x7FFFFFFFFFFFFFFF
		tFn := uint64(0x4000_0000_0000_0003) & 0x7FFFFFFFFFFFFFFF
		sinkParam := uint64(0x4000_0000_0000_0004) & 0x7FFFFFFFFFFFFFFF
		files := []semanticstore.FileFact{{FileID: 1, RepoID: r, Path: "src/flow.go", Language: "go", ContentHash: "h", SizeBytes: 100, LineCount: 10}}
		symbols := []semanticstore.SymbolFact{
			{SymbolID: srcFn, NodeID: srcFn, FileID: 1, Language: "go", Kind: "function", Name: "src", QualifiedName: "src", StableKey: "sk:src", StartLine: 1, StartCol: 1, EndLine: 1, EndCol: 1, Visibility: "public", Confidence: 1.0},
			{SymbolID: tParam, NodeID: tParam, FileID: 1, Language: "go", Kind: "parameter", Name: "x", QualifiedName: "x", StableKey: "sk:x", StartLine: 2, StartCol: 1, EndLine: 2, EndCol: 1, Visibility: "public", Confidence: 1.0},
			{SymbolID: tFn, NodeID: tFn, FileID: 1, Language: "go", Kind: "function", Name: "transform", QualifiedName: "transform", StableKey: "sk:transform", StartLine: 3, StartCol: 1, EndLine: 3, EndCol: 1, Visibility: "public", Confidence: 1.0},
			{SymbolID: sinkParam, NodeID: sinkParam, FileID: 1, Language: "go", Kind: "parameter", Name: "v", QualifiedName: "v", StableKey: "sk:v", StartLine: 4, StartCol: 1, EndLine: 4, EndCol: 1, Visibility: "public", Confidence: 1.0},
		}
		edges := []semanticstore.EdgeFact{
			{EdgeID: 1, SrcNodeID: srcFn, DstNodeID: tParam, SrcKind: "function", DstKind: "parameter", EdgeKind: "DATA_FLOWS", Source: "def_use_inbody", Confidence: 0.50, Weight: 0.5},
			{EdgeID: 2, SrcNodeID: tParam, DstNodeID: tFn, SrcKind: "parameter", DstKind: "function", EdgeKind: "DATA_FLOWS", Source: "def_use_return", Confidence: 0.55, Weight: 0.5},
			{EdgeID: 3, SrcNodeID: tFn, DstNodeID: sinkParam, SrcKind: "function", DstKind: "parameter", EdgeKind: "DATA_FLOWS", Source: "def_use_inbody", Confidence: 0.50, Weight: 0.5},
		}
		return semanticstore.Facts{Files: files, Symbols: symbols, Edges: edges}
	})
	defer cleanup()
	a := NewP1DataFlowReachabilityAdapterForTest(store)

	got, err := a.ReachableFrom(context.Background(), repoID, integ.SymbolID("sk:src"), 5)
	if err != nil {
		t.Fatalf("ReachableFrom over function-anchored edges errored: %v", err)
	}
	hops := map[string]int{}
	for _, n := range got {
		hops[string(n.SymbolID)] = n.Hops
	}
	if _, ok := hops["sk:v"]; !ok {
		t.Errorf("multi-hop over function-anchored edges did not reach sk:v; got %+v", got)
	}
}
