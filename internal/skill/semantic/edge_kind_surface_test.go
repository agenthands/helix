package semantic

import (
	"strings"
	"testing"
)

// TestEdgeKindSurface_RoundTrip asserts every documented internal kind string
// maps to the expected surface enum value. Pitfall 2: RESOLVES_TO → has_type
// (NOT uses_type).
func TestEdgeKindSurface_RoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		internal string
		want     EdgeKindSurface
		wantStr  string
	}{
		{"CALLS", "CALLS", EdgeKindCalls, "calls"},
		{"REFERENCES", "REFERENCES", EdgeKindReferences, "references"},
		{"IMPLEMENTS", "IMPLEMENTS", EdgeKindImplements, "implements"},
		{"EXTENDS", "EXTENDS", EdgeKindExtends, "extends"},
		{"RESOLVES_TO", "RESOLVES_TO", EdgeKindHasType, "has_type"}, // Pitfall 2
		{"USES_TYPE", "USES_TYPE", EdgeKindUsesType, "uses_type"},
		{"CONTAINS", "CONTAINS", EdgeKindContains, "contains"},
		{"DEFINED_IN", "DEFINED_IN", EdgeKindContains, "contains"}, // synonym (RESEARCH lines 346-347)
		{"IMPORTS", "IMPORTS", EdgeKindImports, "imports"},
		{"DEFINES", "DEFINES", EdgeKindDefines, "defines"},
		{"DATA_FLOWS", "DATA_FLOWS", EdgeKindDataFlows, "data_flows"},
		{"STRUCTURAL_TWIN", "STRUCTURAL_TWIN", EdgeKindStructuralTwin, "structural_twin"},
		{"HTTP_CALLS", "HTTP_CALLS", EdgeKindHTTPCalls, "http_calls"},
		{"ASYNC_CALLS", "ASYNC_CALLS", EdgeKindAsyncCalls, "async_calls"},
		{"EMITS", "EMITS", EdgeKindEmits, "emits"},
		{"LISTENS_ON", "LISTENS_ON", EdgeKindListensOn, "listens_on"},
		{"SIMILAR_TO", "SIMILAR_TO", EdgeKindSimilarTo, "similar_to"},
		{"SEMANTICALLY_RELATED", "SEMANTICALLY_RELATED", EdgeKindSemanticallyRelated, "semantically_related"},
		{"HANDLES", "HANDLES", EdgeKindHandles, "handles"},
		{"CONFIGURES", "CONFIGURES", EdgeKindConfigures, "configures"},
		{"WRITES", "WRITES", EdgeKindWrites, "writes"},
		{"MEMBER_OF", "MEMBER_OF", EdgeKindMemberOf, "member_of"},
		{"TESTS", "TESTS", EdgeKindTests, "tests"},
		{"FILE_CHANGES_WITH", "FILE_CHANGES_WITH", EdgeKindFileChangesWith, "file_changes_with"},
		{"CROSS_IMPORTS", "CROSS_IMPORTS", EdgeKindCrossImports, "cross_imports"},
		{"CROSS_CALLS", "CROSS_CALLS", EdgeKindCrossCalls, "cross_calls"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapInternalKind(tc.internal)
			if got != tc.want {
				t.Fatalf("MapInternalKind(%q) = %q, want %q", tc.internal, got, tc.want)
			}
			if string(got) != tc.wantStr {
				t.Fatalf("MapInternalKind(%q) string value = %q, want %q", tc.internal, string(got), tc.wantStr)
			}
		})
	}
}

// TestEdgeKindSurface_Unmapped asserts unmapped internal kinds collapse to
// EdgeKindOther without panicking.
func TestEdgeKindSurface_Unmapped(t *testing.T) {
	cases := []struct {
		name     string
		internal string
	}{
		{"WHATEVER_NEW_KIND", "WHATEVER_NEW_KIND"},
		{"empty", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := MapInternalKind(tc.internal)
			if got != EdgeKindOther {
				t.Fatalf("MapInternalKind(%q) = %q, want EdgeKindOther (%q)", tc.internal, got, EdgeKindOther)
			}
			if string(got) != "other" {
				t.Fatalf("EdgeKindOther string value = %q, want %q", string(got), "other")
			}
		})
	}
}

// TestEdgeKindSurface_AllConstantsLowercase asserts every declared surface enum
// constant is all-lowercase, matching the closed-enum convention in
// envelope.go.
func TestEdgeKindSurface_AllConstantsLowercase(t *testing.T) {
	all := []EdgeKindSurface{
		EdgeKindCalls,
		EdgeKindReferences,
		EdgeKindImplements,
		EdgeKindExtends,
		EdgeKindHasType,
		EdgeKindUsesType,
		EdgeKindContains,
		EdgeKindDefines,
		EdgeKindImports,
		EdgeKindDataFlows,
		EdgeKindStructuralTwin,
		EdgeKindHTTPCalls,
		EdgeKindAsyncCalls,
		EdgeKindEmits,
		EdgeKindListensOn,
		EdgeKindSimilarTo,
		EdgeKindSemanticallyRelated,
		EdgeKindHandles,
		EdgeKindConfigures,
		EdgeKindWrites,
		EdgeKindMemberOf,
		EdgeKindTests,
		EdgeKindFileChangesWith,
		EdgeKindCrossImports,
		EdgeKindCrossCalls,
		EdgeKindOther,
	}
	for _, v := range all {
		s := string(v)
		if s != strings.ToLower(s) {
			t.Fatalf("EdgeKindSurface constant %q is not all-lowercase", s)
		}
	}
}

// TestEdgeKindSurface_ClosedSet asserts the set of declared constants exactly
// matches the documented 8-value closed enum. Catches accidental
// additions/removals.
func TestEdgeKindSurface_ClosedSet(t *testing.T) {
	want := map[string]struct{}{
		"calls":                {},
		"references":           {},
		"implements":           {},
		"extends":              {},
		"has_type":             {},
		"uses_type":            {},
		"contains":             {},
		"defines":              {},
		"imports":              {},
		"data_flows":           {},
		"structural_twin":      {},
		"http_calls":           {},
		"async_calls":          {},
		"emits":                {},
		"listens_on":           {},
		"similar_to":           {},
		"semantically_related": {},
		"handles":              {},
		"configures":           {},
		"writes":               {},
		"member_of":            {},
		"tests":                {},
		"file_changes_with":    {},
		"cross_imports":        {},
		"cross_calls":          {},
		"other":                {},
	}
	got := map[string]struct{}{
		string(EdgeKindCalls):               {},
		string(EdgeKindReferences):          {},
		string(EdgeKindImplements):          {},
		string(EdgeKindExtends):             {},
		string(EdgeKindHasType):             {},
		string(EdgeKindUsesType):            {},
		string(EdgeKindContains):            {},
		string(EdgeKindDefines):             {},
		string(EdgeKindImports):             {},
		string(EdgeKindDataFlows):           {},
		string(EdgeKindStructuralTwin):      {},
		string(EdgeKindHTTPCalls):           {},
		string(EdgeKindAsyncCalls):          {},
		string(EdgeKindEmits):               {},
		string(EdgeKindListensOn):           {},
		string(EdgeKindSimilarTo):           {},
		string(EdgeKindSemanticallyRelated): {},
		string(EdgeKindHandles):             {},
		string(EdgeKindConfigures):          {},
		string(EdgeKindWrites):              {},
		string(EdgeKindMemberOf):            {},
		string(EdgeKindTests):               {},
		string(EdgeKindFileChangesWith):     {},
		string(EdgeKindCrossImports):        {},
		string(EdgeKindCrossCalls):          {},
		string(EdgeKindOther):               {},
	}
	if len(got) != len(want) {
		t.Fatalf("EdgeKindSurface constant count mismatch: got %d distinct values, want %d", len(got), len(want))
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("EdgeKindSurface missing expected value %q", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Fatalf("EdgeKindSurface has unexpected value %q (closed-enum drift)", k)
		}
	}
}
