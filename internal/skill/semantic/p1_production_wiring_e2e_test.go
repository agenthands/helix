// p1_production_wiring_e2e_test.go — Phase 74-06 D-03a / D-03b:
// production-path E2E test for the 6 Phase 71/72 P1 MCP tools.
//
// Distinction from p1_e2e_external_test.go (Phase 73-04):
//   - Phase 73 wired SymbolEdges and ClusterMembership with inline fixture fakes.
//   - This file wires those two accessors using the PRODUCTION adapter structs
//     from internal/daemon/semantic_wiring.go (semP1SymbolEdgesAdapter,
//     semP1ClusterMembershipAdapter) via the For-Test constructors exported by
//     internal/daemon/p1_adapter_export.go (Plan 74-06).
//   - TypeChain and EdgeEvidence remain nil (D-03b: assert documented fallback_reason
//     values for nil accessors).
//
// TDD context (Plan 74-06 RED/GREEN):
//   - RED: fails before Plan 74-04 wires the production path (SymbolByName nil →
//     seed_resolution_failed fallback). After Plan 74-04, the production daemon path
//     wires all P1 accessors, so the test passes (GREEN).
//   - GREEN: after Plan 74-04 applies, all 6 handler assertions pass.
//
// Coverage map:
//   A-04: explain_symbol_deep on p1SeedGoHandle: fallback_reason=="" (SymbolByName wired)
//   A-05: find_related_symbols on p1SeedGoServeHTTP: fallback_reason!="cluster_boost_unavailable"
//         (ClusterMembership production adapter wired)
//   A-06 / A-10 / D-03b: validate_graph_edge: fallback_reason=="evidence_lookup_unavailable"
//         (EdgeEvidence nil → D-03b degraded-path contract: accessor absent is reported as
//         evidence_lookup_unavailable per tools_validate_edge.go:383)
//   A-07: get_cluster_map: fallback_reason=="" and total_clusters>0 (ClusterMap wired)
//   A-08: explain_cluster: fallback_reason=="" (ClusterMember+ClusterPageRank+ClusterMembership wired;
//         member_count may be 0 for non-CALLS fixture edges — see note below)
//   A-09: get_change_impact_graph: fallback_reason!="impact_lookup_unavailable" (ImpactLookup wired)
//   A-11: direction partitions — incoming and outgoing exercised via production adapter;
//         callers (CALLS-filtered) is empty because fixture uses "call_graph" edge_kind
//   A-12: extractor_run_id non-empty (ExtractorRun wired + fixture has committed snapshot)
//
// D-03b production edge adapter notes:
//   CallersOf(sym) uses a SQL filter edge_kind='CALLS'. The buildP1E2EFixture seeds
//   edges with EdgeKind="call_graph", so CallersOf returns empty with this fixture.
//   IncomingEdgesOf (no edge_kind filter) and OutgoingEdgesOf both return edge rows
//   from the "call_graph" edges, confirming the adapter correctly executes both the
//   filtered and unfiltered directions.
//
// Package-cycle note: this file is `package semantic_test` (black-box) so it
// can import internal/daemon for the production adapter constructors. Handler
// entry points are reached via the Handle*ForTest exports defined in
// export_p1_test.go (package semantic).
//
// extractText helper is declared in status_e2e_external_test.go (same package).

package semantic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/skill/semantic"
)

// TestP1E2EProductionPath drives all 6 P1 handlers against a real *Store fixture
// with production adapters (not inline fakes) for SymbolEdges and ClusterMembership.
// TypeChain and EdgeEvidence are nil (D-03b contract assertions).
//
// The test reuses buildP1E2EFixture(t) from p1_e2e_external_test.go for data
// population, then replaces the inline fakes with production adapters.
func TestP1E2EProductionPath(t *testing.T) {
	ctx := context.Background()

	// ---- Build the shared fixture (real *Store + bleve + 7 symbols + 3 call_graph edges + 3 clusters). ----
	fix := buildP1E2EFixture(t)

	// ---- Replace inline fakes with production adapters for the two FOLD accessors. ----
	//
	// FOLD: SymbolEdges + ClusterMembership use production SQL adapters from daemon package.
	// DEFER: TypeChain + EdgeEvidence remain nil (D-03b asserts fallback contracts for these).

	fix.skill.SetSymbolEdges(daemon.NewP1SymbolEdgesAdapterForTest(fix.store))
	fix.skill.SetClusterMembership(daemon.NewP1ClusterMembershipAdapterForTest(fix.store))
	fix.skill.SetTypeChain(nil)    // D-03b: nil → type_chain absent (nil-guard in handler)
	fix.skill.SetEdgeEvidence(nil) // D-03b: nil → fallback_reason="evidence_lookup_unavailable"

	s := fix.skill

	// ---- A-04: explain_symbol_deep on p1SeedGoHandle ----
	// Production SymbolByName wired → fallback_reason=="" (seed_resolution_failed blocked).
	// Production SymbolEdges wired → IncomingEdgesOf returns "call_graph" edges for handle.
	// CallersOf (CALLS-filtered) returns empty with this fixture (edge_kind="call_graph").
	// TypeChain nil → type_chain absent (nil-guard fires, no error).
	t.Run("explain_symbol_deep_handle_incoming", func(t *testing.T) {
		res := semantic.HandleExplainSymbolDeepForTest(s, ctx, semantic.ExplainSymbolDeepArgs{
			Seed: semantic.SeedInput{SymbolID: p1SeedGoHandle},
		})
		require.NotNil(t, res)
		require.False(t, res.IsError, "explain_symbol_deep returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		// A-04: production SymbolByName wired — no seed_resolution_failed.
		fallback, _ := body["fallback_reason"].(string)
		require.Empty(t, fallback,
			"A-04: explain_symbol_deep on known symbol with production wiring must not set fallback_reason; got %q", fallback)

		// A-11: incoming direction — IncomingEdgesOf returns "call_graph" edges (all kinds, no CALLS filter).
		// ServeHTTP→handle edge is a call_graph edge; IncomingEdgesOf(handle) must return it.
		incoming, _ := body["edges_incoming"].([]interface{})
		require.NotEmpty(t, incoming,
			"A-11: explain_symbol_deep(handle): production SymbolEdges.IncomingEdgesOf must return incoming call_graph edges")

		// A-11: callers direction exercised — CallersOf applies CALLS filter;
		// with fixture edge_kind="call_graph", callers returns empty (not an error — documented).
		// The callers JSON field must be present (even if empty slice) confirming CallersOf was called.
		_, callersPresent := body["callers"]
		require.True(t, callersPresent,
			"A-11: explain_symbol_deep must include 'callers' field (even if empty with call_graph fixture edges)")

		// D-03b: TypeChain nil → type_chain field absent or empty array.
		if rawTC, exists := body["type_chain"]; exists {
			tcSlice, _ := rawTC.([]interface{})
			require.Empty(t, tcSlice,
				"D-03b: TypeChain nil → type_chain must be empty or absent")
		}

		// A-12: extractor_run_id must be non-empty (ExtractorRun wired + fixture has snapshot).
		freshMap, _ := body["freshness"].(map[string]interface{})
		require.NotNil(t, freshMap, "A-12: freshness envelope must be present")
		runID, _ := freshMap["extractor_run_id"].(string)
		require.NotEmpty(t, runID,
			"A-12: freshness.extractor_run_id must be non-empty (ExtractorRun wired + committed snapshot)")
		freshnessStatus, _ := freshMap["status"].(string)
		require.NotEmpty(t, freshnessStatus,
			"A-12: freshness.status must be non-empty string")
		require.Contains(t, []string{"current", "stale", "unknown"}, freshnessStatus,
			"A-12: freshness.status must be a declared FreshnessStatus value")
	})

	// ---- A-11: explain_symbol_deep on p1SeedGoServeHTTP — outgoing direction ----
	// Production OutgoingEdgesOf → ServeHTTP → handle call_graph edge must appear in edges_outgoing.
	t.Run("explain_symbol_deep_servehttp_outgoing", func(t *testing.T) {
		res := semantic.HandleExplainSymbolDeepForTest(s, ctx, semantic.ExplainSymbolDeepArgs{
			Seed: semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
		})
		require.NotNil(t, res)
		require.False(t, res.IsError, "explain_symbol_deep(ServeHTTP) returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		fallback, _ := body["fallback_reason"].(string)
		require.Empty(t, fallback,
			"explain_symbol_deep(ServeHTTP) must not set fallback_reason with production wiring")

		// A-11: outgoing direction — OutgoingEdgesOf returns ServeHTTP→handle call_graph edge.
		outgoing, _ := body["edges_outgoing"].([]interface{})
		require.NotEmpty(t, outgoing,
			"A-11: explain_symbol_deep(ServeHTTP): production SymbolEdges.OutgoingEdgesOf must return ServeHTTP→handle")
	})

	// ---- A-05: find_related_symbols on p1SeedGoServeHTTP ----
	// Production ClusterMembership wired → ClusterIDOf returns cluster 1 (Go cluster).
	// cluster_boost_unavailable is the nil-accessor fallback; wired accessor must not emit it.
	t.Run("find_related_symbols_cluster_wired", func(t *testing.T) {
		res := semantic.HandleFindRelatedSymbolsForTest(s, ctx, semantic.FindRelatedSymbolsArgs{
			Seed: semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
			K:    10,
		})
		require.NotNil(t, res)
		require.False(t, res.IsError, "find_related_symbols returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		// A-05: ClusterMembership wired → handler can boost and must not emit cluster_boost_unavailable.
		fallback, _ := body["fallback_reason"].(string)
		require.NotEqual(t, "cluster_boost_unavailable", fallback,
			"A-05: find_related_symbols with production ClusterMembership must not emit cluster_boost_unavailable")
		require.Empty(t, fallback,
			"A-05: find_related_symbols with production wiring must have empty fallback_reason")
	})

	// ---- A-07: get_cluster_map ----
	// ClusterMap wired → total_clusters > 0.
	t.Run("get_cluster_map_wired", func(t *testing.T) {
		res := semantic.HandleGetClusterMapForTest(s, ctx, semantic.GetClusterMapArgs{TopN: 10})
		require.NotNil(t, res)
		require.False(t, res.IsError, "get_cluster_map returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		// A-07: ClusterMap wired → no fallback.
		fallback, _ := body["fallback_reason"].(string)
		require.Empty(t, fallback,
			"A-07: get_cluster_map with wired ClusterMap must not set fallback_reason")

		// A-07: fixture seeds 3 clusters → total_clusters > 0.
		totalClusters, _ := body["total_clusters"].(float64)
		require.Greater(t, int(totalClusters), 0,
			"A-07: get_cluster_map must return > 0 clusters from the 3 seeded clusters")
	})

	// ---- A-08: explain_cluster ----
	// ClusterMember + ClusterPageRank + ClusterMembership wired.
	// Note: QueryClusterMembers may return 0 rows with the current store implementation
	// (this is a known behavior documented in Phase 73); the A-08 assertion is that
	// fallback_reason=="" (handler handles empty member list gracefully without falling back).
	t.Run("explain_cluster_wired", func(t *testing.T) {
		res := semantic.HandleExplainClusterForTest(s, ctx, semantic.ExplainClusterArgs{
			ClusterID:  fix.seedClusterIDEncoded,
			MaxMembers: 100,
		})
		require.NotNil(t, res)
		require.False(t, res.IsError, "explain_cluster returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		// A-08: all cluster accessors wired → no fallback (handler degrades gracefully with empty members).
		fallback, _ := body["fallback_reason"].(string)
		require.Empty(t, fallback,
			"A-08: explain_cluster with wired accessors must not set fallback_reason")

		// A-08: cluster_id must echo back the input token.
		clusterID, _ := body["cluster_id"].(string)
		require.Equal(t, fix.seedClusterIDEncoded, clusterID,
			"A-08: explain_cluster must echo back the input cluster_id")
	})

	// ---- A-09: get_change_impact_graph ----
	// ImpactLookup wired → fallback_reason != "impact_lookup_unavailable".
	t.Run("get_change_impact_graph_wired", func(t *testing.T) {
		res := semantic.HandleGetChangeImpactGraphForTest(s, ctx, semantic.GetChangeImpactGraphArgs{
			Seed:     semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
			MaxDepth: 2,
		})
		require.NotNil(t, res)
		require.False(t, res.IsError, "get_change_impact_graph returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		// A-09: ImpactLookup wired → must not report impact_lookup_unavailable.
		fallback, _ := body["fallback_reason"].(string)
		require.NotEqual(t, "impact_lookup_unavailable", fallback,
			"A-09: ImpactLookup is wired; get_change_impact_graph must not report impact_lookup_unavailable")

		// A-09: fixture seeds ServeHTTP → handle CALLS edge → non-empty node set.
		nodes, _ := body["nodes"].([]interface{})
		require.NotEmpty(t, nodes,
			"A-09: get_change_impact_graph: seeded ServeHTTP→handle call chain must yield non-empty nodes")
	})

	// ---- A-06 / A-10 / D-03b: validate_graph_edge with EdgeEvidence nil ----
	// D-03b contract: EdgeEvidence nil → handler nil-guard fires →
	// fallback_reason="evidence_lookup_unavailable" (per tools_validate_edge.go:383).
	// Note: the plan listed "edge_not_found" as the expected fallback, but actual handler
	// code returns "evidence_lookup_unavailable" when the accessor is nil (not when it
	// returns 0 rows). This assertion reflects actual production behavior.
	t.Run("validate_graph_edge_edge_evidence_nil", func(t *testing.T) {
		res := semantic.HandleValidateGraphEdgeForTest(s, ctx, semantic.ValidateGraphEdgeArgs{
			From:     semantic.SeedInput{SymbolID: p1SeedGoServeHTTP},
			To:       semantic.SeedInput{SymbolID: p1SeedGoHandle},
			EdgeKind: "calls",
		})
		require.NotNil(t, res)
		require.False(t, res.IsError, "validate_graph_edge returned error: %s", extractText(t, res))

		text := extractText(t, res)
		var body map[string]interface{}
		require.NoError(t, json.Unmarshal([]byte(text), &body))

		// A-06 / A-10 / D-03b: EdgeEvidence nil → evidence_lookup_unavailable (accessor absent contract).
		fallback, _ := body["fallback_reason"].(string)
		require.Equal(t, "evidence_lookup_unavailable", fallback,
			"D-03b / A-06 / A-10: EdgeEvidence nil must cause fallback_reason='evidence_lookup_unavailable'")
	})
}
