// p1_multilang_test.go — Multi-language coverage for all 6 P1 graph tools
// across all 11 first-class languages.
package semantic_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	semantic "github.com/agenthands/helix/internal/skill/semantic"
)

var allLangSeeds = []struct {
	lang string
	seed string
}{
	{"go", p1SeedGoServeHTTP},
	{"typescript", p1SeedTSFetch},
	{"java", p1SeedJavaFooBar},
	{"python", p1SeedPyHello},
	{"c_sharp", p1SeedCsCalc},
	{"rust", p1SeedRsMain},
	{"c", p1SeedCHello},
	{"cpp", p1SeedCppRun},
	{"kotlin", p1SeedKtGreet},
	{"php", p1SeedPhpIndex},
	{"ruby", p1SeedRbInit},
}

var allLangEdges = []struct {
	lang     string
	from, to string
}{
	{"go", p1SeedGoServeHTTP, p1SeedGoHandle},
	{"typescript", p1SeedTSFetch, p1SeedTSUser},
	{"java", p1SeedJavaFooBar, p1SeedJavaBar},
	{"python", p1SeedPyHello, p1SeedPyWorld},
	{"c_sharp", p1SeedCsCalc, p1SeedCsResult},
	{"rust", p1SeedRsMain, p1SeedRsConfig},
	{"c", p1SeedCHello, p1SeedCTypes},
	{"cpp", p1SeedCppRun, p1SeedCppEngine},
	{"kotlin", p1SeedKtGreet, p1SeedKtPerson},
	{"php", p1SeedPhpIndex, p1SeedPhpHelper},
	{"ruby", p1SeedRbInit, p1SeedRbBase},
}

func TestMultiLang_ExplainSymbolDeep(t *testing.T) {
	for _, tc := range allLangSeeds {
		t.Run(tc.lang, func(t *testing.T) {
			ctx := context.Background()
			fix := buildP1E2EFixture(t)
			res := semantic.HandleExplainSymbolDeepForTest(fix.skill, ctx, semantic.ExplainSymbolDeepArgs{
				Seed: semantic.SeedInput{SymbolID: tc.seed},
			})
			require.NotNil(t, res)
			require.False(t, res.IsError, "explain_symbol_deep[%s] error: %s", tc.lang, extractText(t, res))
			text := extractText(t, res)
			assertP1Envelope(t, "explain_symbol_deep", text)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &body))
			fallback, _ := body["fallback_reason"].(string)
			require.Empty(t, fallback, "explain_symbol_deep[%s] must not fallback", tc.lang)
		})
	}
}

func TestMultiLang_FindRelatedSymbols(t *testing.T) {
	for _, tc := range allLangSeeds {
		t.Run(tc.lang, func(t *testing.T) {
			ctx := context.Background()
			fix := buildP1E2EFixture(t)
			res := semantic.HandleFindRelatedSymbolsForTest(fix.skill, ctx, semantic.FindRelatedSymbolsArgs{
				Seed: semantic.SeedInput{SymbolID: tc.seed}, K: 10,
			})
			require.NotNil(t, res)
			require.False(t, res.IsError, "find_related_symbols[%s] error: %s", tc.lang, extractText(t, res))
			text := extractText(t, res)
			assertP1Envelope(t, "find_related_symbols", text)
		})
	}
}

func TestMultiLang_ValidateGraphEdge(t *testing.T) {
	for _, tc := range allLangEdges {
		t.Run(tc.lang, func(t *testing.T) {
			ctx := context.Background()
			fix := buildP1E2EFixture(t)
			res := semantic.HandleValidateGraphEdgeForTest(fix.skill, ctx, semantic.ValidateGraphEdgeArgs{
				From:     semantic.SeedInput{SymbolID: tc.from},
				To:       semantic.SeedInput{SymbolID: tc.to},
				EdgeKind: "calls",
			})
			require.NotNil(t, res)
			require.False(t, res.IsError, "validate_graph_edge[%s] error: %s", tc.lang, extractText(t, res))
			text := extractText(t, res)
			assertP1Envelope(t, "validate_graph_edge", text)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &body))
			conf, ok := body["confidence"].(float64)
			require.True(t, ok, "validate_graph_edge[%s]: must have confidence", tc.lang)
			require.GreaterOrEqual(t, conf, 0.0)
		})
	}
}

func TestMultiLang_GetChangeImpactGraph(t *testing.T) {
	for _, tc := range allLangSeeds {
		t.Run(tc.lang, func(t *testing.T) {
			ctx := context.Background()
			fix := buildP1E2EFixture(t)
			res := semantic.HandleGetChangeImpactGraphForTest(fix.skill, ctx, semantic.GetChangeImpactGraphArgs{
				Seed: semantic.SeedInput{SymbolID: tc.seed}, MaxDepth: 2,
			})
			require.NotNil(t, res)
			require.False(t, res.IsError, "get_change_impact_graph[%s] error: %s", tc.lang, extractText(t, res))
			text := extractText(t, res)
			assertP1Envelope(t, "get_change_impact_graph", text)
			var body map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(text), &body))
			nodes, _ := body["nodes"].([]interface{})
			require.NotEmpty(t, nodes, "get_change_impact_graph[%s]: must have nodes", tc.lang)
		})
	}
}

func TestMultiLang_GetClusterMap(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)
	res := semantic.HandleGetClusterMapForTest(fix.skill, ctx, semantic.GetClusterMapArgs{TopN: 50})
	require.NotNil(t, res)
	require.False(t, res.IsError, "get_cluster_map error: %s", extractText(t, res))
	text := extractText(t, res)
	assertP1Envelope(t, "get_cluster_map", text)
}

func TestMultiLang_ExplainCluster(t *testing.T) {
	ctx := context.Background()
	fix := buildP1E2EFixture(t)
	res := semantic.HandleExplainClusterForTest(fix.skill, ctx, semantic.ExplainClusterArgs{
		ClusterID: fix.seedClusterIDEncoded, MaxMembers: 100,
	})
	require.NotNil(t, res)
	require.False(t, res.IsError, "explain_cluster error: %s", extractText(t, res))
	text := extractText(t, res)
	assertP1Envelope(t, "explain_cluster", text)
}
