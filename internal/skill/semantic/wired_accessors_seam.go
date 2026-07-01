package semantic

// wired_accessors_seam.go — cross-package test seam for D-03 runtime bootstrap
// test (Plan 74-05). WiredAccessorsBoolMap and WiredAccessorsForTest live in a
// non-test file so that internal/daemon tests can import and call them.
//
// Background: export_test.go files (package foo, _test.go suffix) are only
// compiled into the test binary for package foo itself; they are NOT accessible
// from tests in a different package such as internal/daemon. To allow
// daemon-package bootstrap tests to assert P1 accessor wiring after daemon.New,
// the seam must reside in a regular compilable file.
//
// The seam is intentionally minimal: a pure read under the skill mutex, with
// no side effects. Negligible binary impact.

// WiredAccessorsBoolMap is a snapshot of which P1 accessor fields are non-nil
// on a SemanticSkill. Used by the D-03 runtime bootstrap test in
// internal/daemon/p1_accessor_bootstrap_test.go to assert production wiring
// without reflection.
type WiredAccessorsBoolMap struct {
	SymbolByName      bool
	ExtractorRun      bool
	ClusterMembership bool
	TypeChain         bool
	SymbolEdges       bool
	EdgeEvidence      bool
	ClusterMap        bool
	ClusterMember     bool
	ClusterPageRank   bool
	ImpactLookup         bool
	DataFlowReachability bool
}

// WiredAccessorsForTest returns a WiredAccessorsBoolMap snapshot of which P1
// accessor fields are non-nil on s. Called by daemon-package bootstrap tests
// (Plan 74-05) as semantic.WiredAccessorsForTest(skill).
//
// Thread-safe: acquires s.mu before reading unexported fields.
func WiredAccessorsForTest(s *SemanticSkill) WiredAccessorsBoolMap {
	s.mu.Lock()
	defer s.mu.Unlock()
	return WiredAccessorsBoolMap{
		SymbolByName:      s.symbolByName != nil,
		ExtractorRun:      s.extractorRun != nil,
		ClusterMembership: s.clusterMembership != nil,
		TypeChain:         s.typeChain != nil,
		SymbolEdges:       s.symbolEdges != nil,
		EdgeEvidence:      s.edgeEvidence != nil,
		ClusterMap:        s.clusterMap != nil,
		ClusterMember:     s.clusterMember != nil,
		ClusterPageRank:   s.clusterPageRank != nil,
		ImpactLookup:         s.impactLookup != nil,
		DataFlowReachability: s.dataFlowReachability != nil,
	}
}
