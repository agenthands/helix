// p1_adapter_export.go — Phase 74-06: test-only constructors that wrap
// the production semP1SymbolEdgesAdapter and semP1ClusterMembershipAdapter so
// that black-box tests in `package semantic_test` can construct them without
// importing unexported daemon types.
//
// Cross-package visibility note: Go's test-binary visibility rule prevents
// _test.go symbols from being seen across package boundaries (the same
// constraint that caused integ_lookup_export.go to lose its _test.go suffix
// during Phase 65-12 BL-1). This file therefore lives in the regular build
// set without the _test.go suffix. The "ForTest" suffix in the constructor
// names makes their test-fixture intent unambiguous; nothing in production
// code (any non-test daemon source) calls them.
//
// Cross-package consumers:
//   - internal/skill/semantic/p1_production_wiring_e2e_test.go (Plan 74-06, D-03a)
//     uses NewP1SymbolEdgesAdapterForTest + NewP1ClusterMembershipAdapterForTest to
//     wire the two FOLD accessors with production adapters backed by a real *Store.

package daemon

import (
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/skill/semantic"
)

// NewP1SymbolEdgesAdapterForTest constructs a production semP1SymbolEdgesAdapter
// wrapped around the caller-supplied *Store and returns it as the
// semantic.SymbolEdgesAccessor interface. Used by Plan 74-06 D-03a E2E tests to
// wire the production SymbolEdges accessor without relying on test-only inline
// fakes.
//
// Test-fixture only — no production code should call this function.
func NewP1SymbolEdgesAdapterForTest(store *semanticstore.Store) semantic.SymbolEdgesAccessor {
	return &semP1SymbolEdgesAdapter{store: store}
}

// NewP1DataFlowReachabilityAdapterForTest constructs a production
// semP1DataFlowReachabilityAdapter wrapped around the caller-supplied *Store
// and returns it as the semantic.DataFlowReachabilityAccessor interface. Used
// by v2.10 trace_data_flow tests to drive the REAL reachability BFS over a
// populated store without test-only inline fakes.
//
// Test-fixture only — no production code should call this function.
func NewP1DataFlowReachabilityAdapterForTest(store *semanticstore.Store) semantic.DataFlowReachabilityAccessor {
	return &semP1DataFlowReachabilityAdapter{store: store}
}

// NewP1ClusterMembershipAdapterForTest constructs a production
// semP1ClusterMembershipAdapter wrapped around the caller-supplied *Store and
// returns it as the semantic.ClusterMembershipAccessor interface. Used by Plan
// 74-06 D-03a E2E tests to wire the production ClusterMembership accessor
// without relying on test-only inline fakes.
//
// Test-fixture only — no production code should call this function.
func NewP1ClusterMembershipAdapterForTest(store *semanticstore.Store) semantic.ClusterMembershipAccessor {
	return &semP1ClusterMembershipAdapter{store: store}
}
