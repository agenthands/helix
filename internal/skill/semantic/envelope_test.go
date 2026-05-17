package semantic

import (
	"encoding/json"
	"strings"
	"testing"
)

// TestFreshnessEnum_ClosedSet asserts the four Freshness constants match the
// SPEC §26.2 closed enum strings.
func TestFreshnessEnum_ClosedSet(t *testing.T) {
	cases := []struct {
		name string
		val  Freshness
		want string
	}{
		{"fresh", FreshnessFresh, "fresh"},
		{"stale", FreshnessStale, "stale"},
		{"structurally_fresh_semantically_pending", FreshnessStructurallyFreshSemanticallyPending, "structurally_fresh_semantically_pending"},
		{"overlay_active", FreshnessOverlayActive, "overlay_active"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("Freshness mismatch: got %q, want %q", tc.val, tc.want)
			}
		})
	}
}

// TestIndexStatusEnum_ClosedSet asserts the three IndexStatus constants match
// the SPEC §23.1 closed enum strings.
func TestIndexStatusEnum_ClosedSet(t *testing.T) {
	cases := []struct {
		name string
		val  IndexStatus
		want string
	}{
		{"committed", IndexStatusCommitted, "committed"},
		{"building", IndexStatusBuilding, "building"},
		{"failed", IndexStatusFailed, "failed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("IndexStatus mismatch: got %q, want %q", tc.val, tc.want)
			}
		})
	}
}

// TestClusterStatus_DefaultUnknownShape asserts that the documented
// Phase 69-05 closed-enum shape (State="unknown", Reason="no-store")
// JSON-marshals to the documented field tags.
func TestClusterStatus_DefaultUnknownShape(t *testing.T) {
	cs := ClusterStatus{
		State:  "unknown",
		Reason: "no-store",
	}
	got, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"state":"unknown","reason":"no-store"}`
	if string(got) != want {
		t.Fatalf("ClusterStatus JSON mismatch:\n got  = %s\n want = %s", got, want)
	}

	// Reason is omitempty when empty: cs2.Reason == "" should not emit the
	// reason key (regression guard against a future tag change).
	cs2 := ClusterStatus{State: "current"}
	got2, err := json.Marshal(cs2)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want2 := `{"state":"current"}`
	if string(got2) != want2 {
		t.Fatalf("ClusterStatus omitempty mismatch:\n got  = %s\n want = %s", got2, want2)
	}
}

// TestClusterStatus_AdditiveFields_JSONShape asserts that the two new
// additive fields (ComputedAt unix-ms, MemberCount) marshal with the documented
// JSON keys and that both are omitempty (zero values must not emit keys —
// preserving backward compat with Phase 64 consumers that expect only
// state/reason).
func TestClusterStatus_AdditiveFields_JSONShape(t *testing.T) {
	cs := ClusterStatus{
		State:       "current",
		ComputedAt:  1715000000000,
		MemberCount: 5,
	}
	got, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"state":"current","computed_at":1715000000000,"member_count":5}`
	if string(got) != want {
		t.Fatalf("ClusterStatus additive JSON mismatch:\n got  = %s\n want = %s", got, want)
	}
	// reason key must be absent (omitempty empty string).
	if containsKey(t, got, "reason") {
		t.Fatalf("ClusterStatus must omit reason when empty; got = %s", got)
	}
}

// TestClusterStatus_BackwardCompat_PreFieldShape asserts the legacy two-field
// shape still round-trips unchanged (no computed_at / member_count keys when
// zero-valued).
func TestClusterStatus_BackwardCompat_PreFieldShape(t *testing.T) {
	cs := ClusterStatus{State: "unknown", Reason: "no-graph-version"}
	got, err := json.Marshal(cs)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"state":"unknown","reason":"no-graph-version"}`
	if string(got) != want {
		t.Fatalf("ClusterStatus backward-compat mismatch:\n got  = %s\n want = %s", got, want)
	}
}

// TestRetrievalStatus_JSONShape asserts the new RetrievalStatus struct's full
// JSON shape (all five fields, deterministic key order) and that empty Reason
// is omitted.
func TestRetrievalStatus_JSONShape(t *testing.T) {
	rs := RetrievalStatus{
		CorpusVersion:  7,
		IndexedFiles:   42,
		IndexedSymbols: 123,
		LastCompactAt:  1715000000000,
	}
	got, err := json.Marshal(rs)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"corpus_version":7,"indexed_files":42,"indexed_symbols":123,"last_compact_at":1715000000000}`
	if string(got) != want {
		t.Fatalf("RetrievalStatus JSON mismatch:\n got  = %s\n want = %s", got, want)
	}
	// Round-trip preserves all four numeric fields.
	var back RetrievalStatus
	if err := json.Unmarshal(got, &back); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if back != rs {
		t.Fatalf("RetrievalStatus round-trip mismatch:\n got  = %+v\n want = %+v", back, rs)
	}
}

// TestRetrievalStatus_ReasonClosedEnum table-tests the five closed-enum Reason
// values (priority order documented on the struct):
//
//	bleve-unavailable > corpus_version-uninitialized > corpus_version-lag >
//	compactor-never-ran > "" (lowest — no degradation).
func TestRetrievalStatus_ReasonClosedEnum(t *testing.T) {
	cases := []struct {
		name   string
		reason string
		want   string // expected JSON of the full struct
	}{
		{
			name:   "bleve-unavailable",
			reason: "bleve-unavailable",
			want:   `{"corpus_version":0,"indexed_files":0,"indexed_symbols":0,"last_compact_at":0,"reason":"bleve-unavailable"}`,
		},
		{
			name:   "corpus_version-uninitialized",
			reason: "corpus_version-uninitialized",
			want:   `{"corpus_version":0,"indexed_files":0,"indexed_symbols":0,"last_compact_at":0,"reason":"corpus_version-uninitialized"}`,
		},
		{
			name:   "corpus_version-lag",
			reason: "corpus_version-lag",
			want:   `{"corpus_version":0,"indexed_files":0,"indexed_symbols":0,"last_compact_at":0,"reason":"corpus_version-lag"}`,
		},
		{
			name:   "compactor-never-ran",
			reason: "compactor-never-ran",
			want:   `{"corpus_version":0,"indexed_files":0,"indexed_symbols":0,"last_compact_at":0,"reason":"compactor-never-ran"}`,
		},
		{
			name:   "empty-omitempty",
			reason: "",
			want:   `{"corpus_version":0,"indexed_files":0,"indexed_symbols":0,"last_compact_at":0}`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rs := RetrievalStatus{Reason: tc.reason}
			got, err := json.Marshal(rs)
			if err != nil {
				t.Fatalf("json.Marshal returned error: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("RetrievalStatus reason=%q JSON mismatch:\n got  = %s\n want = %s", tc.reason, got, tc.want)
			}
			// Round-trip: Reason field survives Unmarshal.
			var back RetrievalStatus
			if err := json.Unmarshal(got, &back); err != nil {
				t.Fatalf("json.Unmarshal returned error: %v", err)
			}
			if back.Reason != tc.reason {
				t.Fatalf("RetrievalStatus.Reason round-trip mismatch: got %q, want %q", back.Reason, tc.reason)
			}
		})
	}
}

// TestStatusResult_NestedRetrievalStatus_TopLevelRetrievalPending asserts the
// envelope structure required by Phase 69 STATUS-01 (closes regression-guard
// against accidental nesting of retrieval_pending under retrieval_status —
// Phase 64 consumers depend on top-level retrieval_pending).
func TestStatusResult_NestedRetrievalStatus_TopLevelRetrievalPending(t *testing.T) {
	sr := StatusResult{
		CommonEnvelope: CommonEnvelope{
			Freshness:    FreshnessFresh,
			GraphVersion: 9,
		},
		LatestSnapshotID: 4,
		ScoreStatus:      map[string]string{"calls": "current"},
		ClusterStatus: ClusterStatus{
			State:       "current",
			ComputedAt:  1715000000000,
			MemberCount: 3,
		},
		LastLiveUpdateMs: 1715000001000,
		PendingLSPFiles:  0,
		RetrievalPending: true,
		RetrievalStatus: RetrievalStatus{
			CorpusVersion:  9,
			IndexedFiles:   10,
			IndexedSymbols: 200,
			LastCompactAt:  1715000000500,
		},
	}
	got, err := json.Marshal(sr)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	// retrieval_status is a nested object …
	var generic map[string]json.RawMessage
	if err := json.Unmarshal(got, &generic); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	rs, ok := generic["retrieval_status"]
	if !ok {
		t.Fatalf("StatusResult missing nested retrieval_status: %s", got)
	}
	// … and it parses as a RetrievalStatus.
	var nested RetrievalStatus
	if err := json.Unmarshal(rs, &nested); err != nil {
		t.Fatalf("retrieval_status not a RetrievalStatus object: %v\nraw = %s", err, rs)
	}
	if nested.CorpusVersion != 9 || nested.IndexedFiles != 10 {
		t.Fatalf("retrieval_status round-trip mismatch: got %+v", nested)
	}
	// retrieval_pending stays at top level (Phase 64 contract).
	rp, ok := generic["retrieval_pending"]
	if !ok {
		t.Fatalf("StatusResult missing top-level retrieval_pending: %s", got)
	}
	if string(rp) != "true" {
		t.Fatalf("retrieval_pending top-level mismatch: got %s", rp)
	}
	// Regression guard: retrieval_pending MUST NOT also live inside
	// retrieval_status.
	var nestedRaw map[string]json.RawMessage
	if err := json.Unmarshal(rs, &nestedRaw); err != nil {
		t.Fatalf("json.Unmarshal nested returned error: %v", err)
	}
	if _, dup := nestedRaw["retrieval_pending"]; dup {
		t.Fatalf("retrieval_pending must NOT be nested inside retrieval_status (Phase 64 contract): %s", rs)
	}
}

// Compile-time interface-satisfaction guard: existing in-package mocks
// (mockRetrievalAccessor in tools_context_test.go, statusMockRetrieval in
// tools_status_test.go, e2eRetrievalAcc in integration_test.go) must each
// implement the post-Plan 69-04 RetrievalAccessor surface — including the
// new RetrievalStatus(ws) method. If any implementer drifts, the package
// will fail to compile (TDD RED trigger).
//
// The assertions below force a compile error if someone removes
// RetrievalStatus from the interface.
var (
	_ = func() RetrievalAccessor { return (*mockRetrievalAccessor)(nil) }
	_ = func() RetrievalAccessor { return (*statusMockRetrieval)(nil) }
)

// ----------------------------------------------------------------------------
// Phase 71-02 — FreshnessV2 envelope + FreshnessStatus / FreshnessSource closed enums.
// All additive to the existing Phase 64 envelope contract (D5).
// ----------------------------------------------------------------------------

// TestFreshnessStatus_ClosedEnum asserts the new three-value closed enum
// (current / stale / unknown — Phase 69 D1 precedent) declares exactly the
// documented constants with lowercase string values.
func TestFreshnessStatus_ClosedEnum(t *testing.T) {
	cases := []struct {
		name string
		val  FreshnessStatus
		want string
	}{
		{"current", FreshnessStatusCurrent, "current"},
		{"stale", FreshnessStatusStale, "stale"},
		{"unknown", FreshnessStatusUnknown, "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("FreshnessStatus mismatch: got %q, want %q", tc.val, tc.want)
			}
			if tc.want != strings.ToLower(tc.want) {
				t.Fatalf("FreshnessStatus value %q is not all-lowercase", tc.want)
			}
		})
	}
	seen := map[string]struct{}{
		string(FreshnessStatusCurrent): {},
		string(FreshnessStatusStale):   {},
		string(FreshnessStatusUnknown): {},
	}
	if len(seen) != 3 {
		t.Fatalf("FreshnessStatus closed-enum drift: got %d distinct values, want 3", len(seen))
	}
}

// TestFreshnessSource_ClosedEnum asserts the new three-value source enum
// (graph / type_resolver_ladder / ast_fallback — D5) declares exactly the
// documented constants with lowercase string values, and that the Source
// field round-trips through JSON marshal/unmarshal.
func TestFreshnessSource_ClosedEnum(t *testing.T) {
	cases := []struct {
		name string
		val  FreshnessSource
		want string
	}{
		{"graph", FreshnessSourceGraph, "graph"},
		{"type_resolver_ladder", FreshnessSourceTypeResolverLadder, "type_resolver_ladder"},
		{"ast_fallback", FreshnessSourceASTFallback, "ast_fallback"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("FreshnessSource mismatch: got %q, want %q", tc.val, tc.want)
			}
			if tc.want != strings.ToLower(tc.want) {
				t.Fatalf("FreshnessSource value %q is not all-lowercase", tc.want)
			}
		})
	}
	// Round-trip: Source field survives JSON marshal/unmarshal.
	f := FreshnessV2{Source: FreshnessSourceTypeResolverLadder}
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	var back FreshnessV2
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	if back.Source != FreshnessSourceTypeResolverLadder {
		t.Fatalf("FreshnessSource round-trip mismatch: got %q, want %q", back.Source, FreshnessSourceTypeResolverLadder)
	}
	seen := map[string]struct{}{
		string(FreshnessSourceGraph):              {},
		string(FreshnessSourceTypeResolverLadder): {},
		string(FreshnessSourceASTFallback):        {},
	}
	if len(seen) != 3 {
		t.Fatalf("FreshnessSource closed-enum drift: got %d distinct values, want 3", len(seen))
	}
}

// TestFreshnessV2_ShapeMarshal asserts a fully-populated FreshnessV2 marshals
// to exactly the documented keys/values (D5).
func TestFreshnessV2_ShapeMarshal(t *testing.T) {
	f := FreshnessV2{
		GraphVersion:   7,
		SnapshotID:     42,
		ExtractorRunID: "snap-42",
		AsOfUnixMs:     1715760000000,
		Status:         FreshnessStatusCurrent,
	}
	got, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{"graph_version":7,"snapshot_id":42,"extractor_run_id":"snap-42","as_of_unix_ms":1715760000000,"status":"current"}`
	if string(got) != want {
		t.Fatalf("FreshnessV2 JSON mismatch:\n got  = %s\n want = %s", got, want)
	}
}

// TestFreshnessV2_OmitEmpty asserts an empty FreshnessV2{} marshals to an
// empty JSON object — every field carries omitempty so degraded paths emitting
// a partial envelope do not surface zero-value keys.
func TestFreshnessV2_OmitEmpty(t *testing.T) {
	f := FreshnessV2{}
	got, err := json.Marshal(f)
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	want := `{}`
	if string(got) != want {
		t.Fatalf("FreshnessV2 omit-empty mismatch:\n got  = %s\n want = %s", got, want)
	}
}

// TestFreshnessV2_AdditiveToExisting asserts the existing Phase 64 Freshness
// const block is byte-identical to the pre-plan state — guards against an
// accidental rename / value drift of the 4-state Freshness type that Phase 64
// tools depend on.
func TestFreshnessV2_AdditiveToExisting(t *testing.T) {
	cases := []struct {
		name string
		val  Freshness
		want string
	}{
		{"fresh", FreshnessFresh, "fresh"},
		{"stale", FreshnessStale, "stale"},
		{"structurally_fresh_semantically_pending", FreshnessStructurallyFreshSemanticallyPending, "structurally_fresh_semantically_pending"},
		{"overlay_active", FreshnessOverlayActive, "overlay_active"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.val) != tc.want {
				t.Fatalf("existing Freshness drift: %q != %q (Phase 64 contract regression)", tc.val, tc.want)
			}
		})
	}
}

// TestFreshnessStatus_Independent asserts FreshnessStatus is a distinct named
// type from Phase 64's Freshness — they coexist with potentially overlapping
// string values (e.g., "stale") because Go's type system treats them as
// different named types. This test is a runtime guard; the compile-time guard
// is the separate const block referencing only the new type in envelope.go.
func TestFreshnessStatus_Independent(t *testing.T) {
	var f Freshness = FreshnessStale
	var s FreshnessStatus = FreshnessStatusStale
	if string(f) != "stale" {
		t.Fatalf("Freshness sanity: got %q, want %q", f, "stale")
	}
	if string(s) != "stale" {
		t.Fatalf("FreshnessStatus sanity: got %q, want %q", s, "stale")
	}
	var typedNew FreshnessStatus = FreshnessStatusCurrent
	if string(typedNew) != "current" {
		t.Fatalf("FreshnessStatusCurrent: got %q, want %q", typedNew, "current")
	}
}

// containsKey reports whether the given JSON byte slice contains a top-level
// key. Used by the additive-fields test to assert omitempty behavior.
func containsKey(t *testing.T, raw []byte, key string) bool {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}
	_, ok := m[key]
	return ok
}
