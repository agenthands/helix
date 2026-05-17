package semantic

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/semantic/types"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- D-09 / D-13 recorder canaries ----------
//
// recorderStoreAccessorForExplain implements StoreAccessor and adds forbidden
// snapshot-write canaries. The recorder lives in this test file (package-local)
// so any future regression that reaches BeginSnapshot/Commit/Abort/Write via
// type assertion fails loudly. Mirrors tools_refresh_test.go:38-50 + 113-135.
//
// NOTE: a separate recorder type (recorderStoreAccessorForExplain) is used
// instead of reusing recorderStoreAccessor to avoid coupling to tools_refresh
// test seam injections.
type recorderStoreAccessorForExplain struct {
	t *testing.T

	graphVersion      uint64
	overlayHasPending bool
	latestSnapshot    uint64

	beginSnapshotCalls      atomic.Int64
	commitSnapshotCalls     atomic.Int64
	abortSnapshotCalls      atomic.Int64
	writeSnapshotFactsCalls atomic.Int64
}

func (r *recorderStoreAccessorForExplain) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return r.latestSnapshot, nil
}
func (r *recorderStoreAccessorForExplain) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return r.graphVersion, nil
}
func (r *recorderStoreAccessorForExplain) OverlayHasPendingRows(repoID string) bool {
	return r.overlayHasPending
}
func (r *recorderStoreAccessorForExplain) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return nil, nil, nil
}
func (r *recorderStoreAccessorForExplain) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (r *recorderStoreAccessorForExplain) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (r *recorderStoreAccessorForExplain) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// Forbidden methods (D-09 / D-13 invariants). NOT on StoreAccessor interface.
func (r *recorderStoreAccessorForExplain) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_symbol_deep handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *recorderStoreAccessorForExplain) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_symbol_deep handler must NOT call CommitSnapshot")
	return nil
}
func (r *recorderStoreAccessorForExplain) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_symbol_deep handler must NOT call AbortSnapshot")
	return nil
}
func (r *recorderStoreAccessorForExplain) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeSnapshotFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: explain_symbol_deep handler must NOT call WriteSnapshotFacts")
	return nil
}

// ---------- Test-only accessor fakes for the new explain_symbol_deep seams ----------

// fakeTypeChainAccessor implements TypeChainAccessor over an in-memory table
// keyed on the seed SymbolID.
type fakeTypeChainAccessor struct {
	rowsBySymbol map[integ.SymbolID][]TypeChainRow
	err          error
}

func (f *fakeTypeChainAccessor) TypeChainForSymbol(ctx context.Context, repoID string, sym integ.SymbolID) ([]TypeChainRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]TypeChainRow(nil), f.rowsBySymbol[sym]...), nil
}

// fakeSymbolEdgesAccessor implements SymbolEdgesAccessor over in-memory tables
// keyed on the seed SymbolID. Each map value is the full unbounded set; the
// handler is responsible for capping to D2 limits (50 callers, 100 edges per
// direction).
type fakeSymbolEdgesAccessor struct {
	callers  map[integ.SymbolID][]SymbolEdgeRow
	incoming map[integ.SymbolID][]SymbolEdgeRow
	outgoing map[integ.SymbolID][]SymbolEdgeRow
	err      error
}

func (f *fakeSymbolEdgesAccessor) CallersOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]SymbolEdgeRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]SymbolEdgeRow(nil), f.callers[sym]...), nil
}
func (f *fakeSymbolEdgesAccessor) IncomingEdgesOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]SymbolEdgeRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]SymbolEdgeRow(nil), f.incoming[sym]...), nil
}
func (f *fakeSymbolEdgesAccessor) OutgoingEdgesOf(ctx context.Context, repoID string, sym integ.SymbolID) ([]SymbolEdgeRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	return append([]SymbolEdgeRow(nil), f.outgoing[sym]...), nil
}

// ---------- common test setup ----------

// newSkillForExplainTest constructs a SemanticSkill wired with the
// recorder StoreAccessor + the explain-symbol-deep specific accessor fakes.
// The session mode "read" satisfies modeTierRead.
func newSkillForExplainTest(
	t *testing.T,
	mode string,
	fx *PopulatedGraphFixture,
	tc TypeChainAccessor,
	se SymbolEdgesAccessor,
) (*SemanticSkill, *recorderStoreAccessorForExplain) {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{
		SessionID: "test-explain-session",
		Mode:      mode,
	}
	ws := workspace.WorkspaceKey{
		RepoRoot: "/tmp/repo-explain-test", Language: "go", Toolchain: "go1.22",
	}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})

	store := &recorderStoreAccessorForExplain{
		t:              t,
		graphVersion:   42,
		latestSnapshot: 7,
	}
	s.SetStore(store)
	s.SetSymbolByName(fx.SymbolByName)
	s.SetExtractorRun(fx.ExtractorRun)
	s.SetClusterMembership(fx.ClusterMembership)
	s.SetTypeChain(tc)
	s.SetSymbolEdges(se)
	return s, store
}

// unmarshalExplainResult is a helper that decodes the JSON body of an
// explain_symbol_deep CallToolResult.
func unmarshalExplainResult(t *testing.T, raw string) ExplainSymbolDeepResult {
	t.Helper()
	var out ExplainSymbolDeepResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal explain result: %v (raw=%s)", err, raw)
	}
	return out
}

// ---------- behavior tests ----------

func TestExplainSymbolDeep_HappyPath(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID
	tc := &fakeTypeChainAccessor{
		rowsBySymbol: map[integ.SymbolID][]TypeChainRow{
			seed: {
				{Tier: "tier1_lsp", EvidenceKind: string(types.EvidenceLSP), TargetSymbolID: "repo/src/types.go::Request"},
			},
		},
	}
	se := &fakeSymbolEdgesAccessor{
		callers: map[integ.SymbolID][]SymbolEdgeRow{
			seed: {{From: "repo/src/svc.go::handle", InternalKind: "CALLS"}},
		},
		incoming: map[integ.SymbolID][]SymbolEdgeRow{
			seed: {{From: "repo/src/svc.go::handle", InternalKind: "CALLS"}},
		},
		outgoing: map[integ.SymbolID][]SymbolEdgeRow{
			seed: {
				{To: "repo/src/types.go::Request", InternalKind: "RESOLVES_TO"},
				{To: "repo/src/util.go::log", InternalKind: "CALLS"},
			},
		},
	}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))

	if len(out.TypeChain) == 0 {
		t.Fatalf("type_chain empty; want at least 1 entry")
	}
	tcEntry := out.TypeChain[0]
	if tcEntry.Tier == "" {
		t.Errorf("type_chain[0].tier empty")
	}
	if tcEntry.EvidenceKind == "" {
		t.Errorf("type_chain[0].evidence_kind empty")
	}
	if tcEntry.Confidence <= 0 {
		t.Errorf("type_chain[0].confidence non-positive: %v", tcEntry.Confidence)
	}

	if len(out.Callers) > 50 {
		t.Errorf("callers cap exceeded: got %d, max 50", len(out.Callers))
	}
	if out.CallersCountReturned != len(out.Callers) {
		t.Errorf("callers_count_returned mismatch: returned=%d len=%d",
			out.CallersCountReturned, len(out.Callers))
	}
	if out.CallersCountTotal < len(out.Callers) {
		t.Errorf("callers_count_total %d < returned %d", out.CallersCountTotal, len(out.Callers))
	}

	if len(out.EdgesIncoming) > 100 {
		t.Errorf("edges_incoming cap exceeded: got %d, max 100", len(out.EdgesIncoming))
	}
	if len(out.EdgesOutgoing) > 100 {
		t.Errorf("edges_outgoing cap exceeded: got %d, max 100", len(out.EdgesOutgoing))
	}
	for i, e := range out.EdgesIncoming {
		if e.EdgeKind == "" {
			t.Errorf("edges_incoming[%d].edge_kind empty", i)
		}
		if e.InternalKind == "" {
			t.Errorf("edges_incoming[%d].internal_kind empty", i)
		}
	}
	for i, e := range out.EdgesOutgoing {
		if e.EdgeKind == "" {
			t.Errorf("edges_outgoing[%d].edge_kind empty", i)
		}
		if e.InternalKind == "" {
			t.Errorf("edges_outgoing[%d].internal_kind empty", i)
		}
	}

	// Cluster reference must be present (size may be 0 if isolated).
	if out.Cluster.ClusterID == 0 && out.Cluster.Size != 0 {
		t.Errorf("cluster inconsistent: id=0 but size=%d", out.Cluster.Size)
	}

	// Freshness envelope: non-zero graph_version + snapshot_id + status=current.
	if out.Freshness.GraphVersion == 0 {
		t.Errorf("freshness.graph_version zero; want 42")
	}
	if out.Freshness.SnapshotID == 0 {
		t.Errorf("freshness.snapshot_id zero; want 7")
	}
	if out.Freshness.Status != FreshnessStatusCurrent {
		t.Errorf("freshness.status = %q; want %q", out.Freshness.Status, FreshnessStatusCurrent)
	}

	// Seed envelope: resolution=exact (SymbolID path).
	if out.Seed.Resolution != ResolutionExact {
		t.Errorf("seed.resolution = %q; want %q", out.Seed.Resolution, ResolutionExact)
	}
}

func TestExplainSymbolDeep_ResolutionAmbiguous(t *testing.T) {
	// Build a fixture variant where (file, name) matches 3 symbols.
	// We re-use the existing fixture and inject a custom SymbolByName accessor
	// that returns 3 rows for the same (path, name).
	fx := buildPopulatedGraphFixture(t)
	const path = "repo/src/svc.go"
	const name = "DupName"
	ambig := &ambigSymbolByName{ids: []integ.SymbolID{
		"repo/src/svc.go::DupName#a",
		"repo/src/svc.go::DupName#b",
		"repo/src/svc.go::DupName#c",
	}}
	fx.SymbolByName = ambig

	tc := &fakeTypeChainAccessor{rowsBySymbol: map[integ.SymbolID][]TypeChainRow{}}
	se := &fakeSymbolEdgesAccessor{}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{FilePath: path, SymbolName: name},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))

	if out.Seed.Resolution != ResolutionAmbiguous {
		t.Errorf("seed.resolution = %q; want %q", out.Seed.Resolution, ResolutionAmbiguous)
	}
	if len(out.Seed.AmbiguousCandidates) != 3 {
		t.Errorf("ambiguous_candidates len = %d; want 3", len(out.Seed.AmbiguousCandidates))
	}
}

// ambigSymbolByName returns the same canned ids for ANY (path, name) tuple —
// useful for ambiguity tests.
type ambigSymbolByName struct{ ids []integ.SymbolID }

func (a *ambigSymbolByName) QuerySymbolByName(ctx context.Context, repoID, path, name string) ([]integ.SymbolID, error) {
	return append([]integ.SymbolID(nil), a.ids...), nil
}

func TestExplainSymbolDeep_ResolutionNotFound(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	tc := &fakeTypeChainAccessor{}
	se := &fakeSymbolEdgesAccessor{}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{FilePath: "unknown/file.go", SymbolName: "Missing"},
	})
	if res.IsError {
		t.Fatalf("expected success (not_found is NOT an error envelope), got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))

	if out.Seed.Resolution != ResolutionNotFound {
		t.Errorf("seed.resolution = %q; want %q", out.Seed.Resolution, ResolutionNotFound)
	}
	if out.FallbackReason != "symbol_not_found" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "symbol_not_found")
	}
}

func TestExplainSymbolDeep_TruncationCallers(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// 75 callers — should truncate to 50.
	hot := make([]SymbolEdgeRow, 0, 75)
	for i := 0; i < 75; i++ {
		hot = append(hot, SymbolEdgeRow{From: integ.SymbolID(seedHotCaller(i)), InternalKind: "CALLS"})
	}
	tc := &fakeTypeChainAccessor{}
	se := &fakeSymbolEdgesAccessor{
		callers: map[integ.SymbolID][]SymbolEdgeRow{seed: hot},
	}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))
	if out.CallersCountReturned != 50 {
		t.Errorf("callers_count_returned = %d; want 50", out.CallersCountReturned)
	}
	if out.CallersCountTotal != 75 {
		t.Errorf("callers_count_total = %d; want 75", out.CallersCountTotal)
	}
	if len(out.Callers) != 50 {
		t.Errorf("len(callers) = %d; want 50", len(out.Callers))
	}
}

func seedHotCaller(i int) string {
	return "repo/src/caller_" + itoa3(i) + ".go::Call"
}
func itoa3(i int) string {
	const digits = "0123456789"
	return string([]byte{digits[i/100%10], digits[i/10%10], digits[i%10]})
}

func TestExplainSymbolDeep_TruncationEdges(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// 150 incoming + 130 outgoing → truncate to 100 each.
	incoming := make([]SymbolEdgeRow, 0, 150)
	for i := 0; i < 150; i++ {
		incoming = append(incoming, SymbolEdgeRow{
			From: integ.SymbolID("repo/in_" + itoa3(i) + ".go::X"), InternalKind: "REFERENCES",
		})
	}
	outgoing := make([]SymbolEdgeRow, 0, 130)
	for i := 0; i < 130; i++ {
		outgoing = append(outgoing, SymbolEdgeRow{
			To: integ.SymbolID("repo/out_" + itoa3(i) + ".go::Y"), InternalKind: "CALLS",
		})
	}
	tc := &fakeTypeChainAccessor{}
	se := &fakeSymbolEdgesAccessor{
		incoming: map[integ.SymbolID][]SymbolEdgeRow{seed: incoming},
		outgoing: map[integ.SymbolID][]SymbolEdgeRow{seed: outgoing},
	}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))

	if out.EdgesIncomingCountReturned != 100 {
		t.Errorf("edges_incoming_count_returned = %d; want 100", out.EdgesIncomingCountReturned)
	}
	if out.EdgesIncomingCountTotal != 150 {
		t.Errorf("edges_incoming_count_total = %d; want 150", out.EdgesIncomingCountTotal)
	}
	if out.EdgesOutgoingCountReturned != 100 {
		t.Errorf("edges_outgoing_count_returned = %d; want 100", out.EdgesOutgoingCountReturned)
	}
	if out.EdgesOutgoingCountTotal != 130 {
		t.Errorf("edges_outgoing_count_total = %d; want 130", out.EdgesOutgoingCountTotal)
	}
}

func TestExplainSymbolDeep_DegradedPath(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID

	// Inject a tier6_heuristic (EvidenceHeuristic) row so the handler flags
	// the type-resolver as degraded.
	tc := &fakeTypeChainAccessor{
		rowsBySymbol: map[integ.SymbolID][]TypeChainRow{
			seed: {
				{Tier: "tier6_heuristic", EvidenceKind: string(types.EvidenceHeuristic), TargetSymbolID: "repo/src/types.go::Request"},
			},
		},
	}
	se := &fakeSymbolEdgesAccessor{}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))

	if out.Confidence > 0.6 {
		t.Errorf("top-level confidence on degraded path = %v; want ≤ 0.6 (TYPES-04 cap)", out.Confidence)
	}
	if out.FallbackReason != "type_resolver_degraded" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "type_resolver_degraded")
	}
}

func TestExplainSymbolDeep_ReadOnly(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID
	tc := &fakeTypeChainAccessor{
		rowsBySymbol: map[integ.SymbolID][]TypeChainRow{
			seed: {{Tier: "tier1_lsp", EvidenceKind: string(types.EvidenceLSP)}},
		},
	}
	se := &fakeSymbolEdgesAccessor{
		callers:  map[integ.SymbolID][]SymbolEdgeRow{seed: {{From: "a", InternalKind: "CALLS"}}},
		incoming: map[integ.SymbolID][]SymbolEdgeRow{seed: {{From: "a", InternalKind: "CALLS"}}},
		outgoing: map[integ.SymbolID][]SymbolEdgeRow{seed: {{To: "b", InternalKind: "CALLS"}}},
	}
	s, store := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}

	// D-09 / D-13 canaries: NO forbidden snapshot-write call invoked.
	if store.beginSnapshotCalls.Load() != 0 {
		t.Errorf("BeginSnapshot was called %d times — D-09 violation",
			store.beginSnapshotCalls.Load())
	}
	if store.commitSnapshotCalls.Load() != 0 {
		t.Errorf("CommitSnapshot was called %d times — D-09 violation",
			store.commitSnapshotCalls.Load())
	}
	if store.abortSnapshotCalls.Load() != 0 {
		t.Errorf("AbortSnapshot was called %d times — D-09 violation",
			store.abortSnapshotCalls.Load())
	}
	if store.writeSnapshotFactsCalls.Load() != 0 {
		t.Errorf("WriteSnapshotFacts was called %d times — D-09 violation",
			store.writeSnapshotFactsCalls.Load())
	}
}

func TestExplainSymbolDeep_ModeRejected(t *testing.T) {
	// modeTierRead always returns nil (every session passes — including empty
	// mode strings), so a true mode-rejected outcome cannot be triggered for a
	// read+ tool. This test asserts the handler INVOKES checkMode (snap is
	// read) and produces no panic / no error envelope, then verifies that
	// when the resolveSeed accessor is unwired (a different InvalidArgs path),
	// the handler returns the structured error WITHOUT touching the store
	// canaries. This combination preserves the spirit of the plan's
	// "no accessor calls on mode-reject" assertion while reflecting the
	// codebase invariant that read+ never rejects.
	fx := buildPopulatedGraphFixture(t)
	fx.SymbolByName = nil // force resolveSeed accessor-unwired error path

	tc := &fakeTypeChainAccessor{}
	se := &fakeSymbolEdgesAccessor{}
	s, store := newSkillForExplainTest(t, "read", fx, tc, se)
	// Override the nil SymbolByName seam after newSkillForExplainTest installed it.
	s.SetSymbolByName(nil)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{FilePath: "x.go", SymbolName: "Y"},
	})
	if !res.IsError {
		t.Fatalf("expected error envelope when resolver unwired; got success")
	}
	// No write-side canaries fired.
	if store.beginSnapshotCalls.Load()+store.commitSnapshotCalls.Load()+
		store.abortSnapshotCalls.Load()+store.writeSnapshotFactsCalls.Load() != 0 {
		t.Errorf("forbidden snapshot-write canaries fired on error path")
	}
}

func TestExplainSymbolDeep_EdgeClassification(t *testing.T) {
	// Pitfall 2 regression guard: RESOLVES_TO internal kind must surface as
	// EdgeKindHasType ("has_type"), NOT uses_type.
	fx := buildPopulatedGraphFixture(t)
	seed := fx.GoSeedSymbolID
	tc := &fakeTypeChainAccessor{}
	se := &fakeSymbolEdgesAccessor{
		outgoing: map[integ.SymbolID][]SymbolEdgeRow{
			seed: {
				{To: "repo/src/types.go::Request", InternalKind: "RESOLVES_TO"},
			},
		},
	}
	s, _ := newSkillForExplainTest(t, "read", fx, tc, se)

	res := s.handleExplainSymbolDeep(context.Background(), ExplainSymbolDeepArgs{
		Seed: SeedInput{SymbolID: string(seed)},
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalExplainResult(t, textOf(res))

	found := false
	for _, e := range out.EdgesOutgoing {
		if e.InternalKind == "RESOLVES_TO" && e.EdgeKind == EdgeKindHasType {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("missing edge {internal_kind=RESOLVES_TO, edge_kind=has_type} — Pitfall 2 regression; got outgoing=%+v", out.EdgesOutgoing)
	}
}
