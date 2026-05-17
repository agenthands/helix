package semantic

import (
	"context"
	"encoding/json"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/graph"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- D-09 / D-13 recorder canaries ----------
//
// recorderStoreAccessorForValidateEdge implements StoreAccessor with
// snapshot-write canaries. Mirrors the pattern from tools_find_related_test.go
// and tools_explain_symbol_test.go.
type recorderStoreAccessorForValidateEdge struct {
	t *testing.T

	graphVersion      uint64
	overlayHasPending bool
	latestSnapshot    uint64

	beginSnapshotCalls      atomic.Int64
	commitSnapshotCalls     atomic.Int64
	abortSnapshotCalls      atomic.Int64
	writeSnapshotFactsCalls atomic.Int64
}

func (r *recorderStoreAccessorForValidateEdge) LatestCommittedSnapshot(ctx context.Context, repoID string) (uint64, error) {
	return r.latestSnapshot, nil
}
func (r *recorderStoreAccessorForValidateEdge) CurrentGraphVersion(ctx context.Context, repoID string) (uint64, error) {
	return r.graphVersion, nil
}
func (r *recorderStoreAccessorForValidateEdge) OverlayHasPendingRows(repoID string) bool {
	return r.overlayHasPending
}
func (r *recorderStoreAccessorForValidateEdge) QueryEffectiveAdjacency(ctx context.Context, repoID, projection string) (
	out, in map[graph.NodeID]map[graph.NodeID]float64, err error,
) {
	return nil, nil, nil
}
func (r *recorderStoreAccessorForValidateEdge) CurrentOverlayEpoch(ctx context.Context, repoID string) (uint64, error) {
	return 0, nil
}
func (r *recorderStoreAccessorForValidateEdge) OverlayChangedPathsSince(ctx context.Context, repoID string, baseEpoch uint64) ([]string, uint64, error) {
	return nil, 0, nil
}
func (r *recorderStoreAccessorForValidateEdge) LatestCommittedSnapshotBaseEpoch(ctx context.Context, repoID string) (uint64, bool, error) {
	return 0, false, nil
}

// Forbidden methods (D-09 / D-13). NOT on StoreAccessor interface.
func (r *recorderStoreAccessorForValidateEdge) BeginSnapshot(ctx context.Context, repoID string) (uint64, error) {
	r.beginSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: validate_graph_edge handler must NOT call BeginSnapshot")
	return 0, nil
}
func (r *recorderStoreAccessorForValidateEdge) CommitSnapshot(ctx context.Context, snapID uint64) error {
	r.commitSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: validate_graph_edge handler must NOT call CommitSnapshot")
	return nil
}
func (r *recorderStoreAccessorForValidateEdge) AbortSnapshot(ctx context.Context, snapID uint64) error {
	r.abortSnapshotCalls.Add(1)
	r.t.Fatalf("D-09 violation: validate_graph_edge handler must NOT call AbortSnapshot")
	return nil
}
func (r *recorderStoreAccessorForValidateEdge) WriteSnapshotFacts(ctx context.Context, snapID uint64, facts any) error {
	r.writeSnapshotFactsCalls.Add(1)
	r.t.Fatalf("D-09 violation: validate_graph_edge handler must NOT call WriteSnapshotFacts")
	return nil
}

// ---------- Fake EdgeEvidenceAccessor ----------

// fakeEdgeEvidenceAccessor returns canned per-edge evidence rows keyed on the
// (from, to, internal_kind) triple. Returning a nil slice signals "edge absent"
// (no matching row); returning a non-empty slice with all-zero ExistsInGraph
// would be a misuse, so the fake always sets ExistsInGraph=true on each row.
type fakeEdgeEvidenceAccessor struct {
	rows map[edgeKey][]EdgeEvidenceRow
	err  error
}

type edgeKey struct {
	from         integ.SymbolID
	to           integ.SymbolID
	internalKind string
}

func (f *fakeEdgeEvidenceAccessor) EvidenceForEdge(ctx context.Context, repoID string, from, to integ.SymbolID, internalKinds []string) ([]EdgeEvidenceRow, error) {
	if f.err != nil {
		return nil, f.err
	}
	var out []EdgeEvidenceRow
	for _, k := range internalKinds {
		out = append(out, f.rows[edgeKey{from, to, k}]...)
	}
	return out, nil
}

// ---------- common test setup ----------

func newSkillForValidateEdgeTest(
	t *testing.T,
	mode string,
	fx *PopulatedGraphFixture,
	ev *fakeEdgeEvidenceAccessor,
) (*SemanticSkill, *recorderStoreAccessorForValidateEdge) {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := &mcp.SessionInfo{
		SessionID: "test-validate-edge-session",
		Mode:      mode,
	}
	ws := workspace.WorkspaceKey{
		RepoRoot: "/tmp/repo-validate-edge-test", Language: "go", Toolchain: "go1.22",
	}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})

	store := &recorderStoreAccessorForValidateEdge{
		t:              t,
		graphVersion:   42,
		latestSnapshot: 7,
	}
	s.SetStore(store)
	s.SetSymbolByName(fx.SymbolByName)
	s.SetExtractorRun(fx.ExtractorRun)
	s.SetClusterMembership(fx.ClusterMembership)
	s.SetEdgeEvidence(ev)
	return s, store
}

func unmarshalValidateEdgeResult(t *testing.T, raw string) ValidateGraphEdgeResult {
	t.Helper()
	var out ValidateGraphEdgeResult
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("unmarshal validate edge result: %v (raw=%s)", err, raw)
	}
	return out
}

// ---------- Closed-enum tests ----------

func TestEvidenceSource_ClosedSet(t *testing.T) {
	want := map[string]struct{}{
		"lsp":           {},
		"ast":           {},
		"type_resolver": {},
	}
	got := map[string]struct{}{
		string(EvidenceSourceLSP):          {},
		string(EvidenceSourceAST):          {},
		string(EvidenceSourceTypeResolver): {},
	}
	if len(got) != len(want) {
		t.Fatalf("EvidenceSource constant count mismatch: got %d distinct, want %d", len(got), len(want))
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("EvidenceSource missing expected value %q", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Fatalf("EvidenceSource has unexpected value %q (closed-enum drift)", k)
		}
	}
}

func TestEvidenceStatus_ClosedSet(t *testing.T) {
	want := map[string]struct{}{
		"complete": {},
		"partial":  {},
		"none":     {},
	}
	got := map[string]struct{}{
		string(EvidenceStatusComplete): {},
		string(EvidenceStatusPartial):  {},
		string(EvidenceStatusNone):     {},
	}
	if len(got) != len(want) {
		t.Fatalf("EvidenceStatus constant count mismatch: got %d distinct, want %d", len(got), len(want))
	}
	for k := range want {
		if _, ok := got[k]; !ok {
			t.Fatalf("EvidenceStatus missing expected value %q", k)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Fatalf("EvidenceStatus has unexpected value %q (closed-enum drift)", k)
		}
	}
}

// ---------- Behavior tests ----------

func TestValidateGraphEdge_HappyPath(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	ev := &fakeEdgeEvidenceAccessor{
		rows: map[edgeKey][]EdgeEvidenceRow{
			{from, to, "CALLS"}: {
				{
					InternalKind:   "CALLS",
					Source:         "lsp.go.text_document_references",
					TreeSitterKind: "call_expression",
					File:           "repo/src/svc.go",
					Range:          &EvidenceRange{StartLine: 10, StartCol: 4, EndLine: 10, EndCol: 12},
					Tier:           "tier1_lsp",
					EvidenceKind:   "lsp",
				},
			},
		},
	}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))

	// Three sources of evidence: LSP (0.4) + AST (0.3) + type_resolver(tier1=0.3) = 1.0.
	if out.Confidence < 0.99 || out.Confidence > 1.0001 {
		t.Errorf("confidence = %v; want ~1.0", out.Confidence)
	}
	if len(out.Evidence) != 3 {
		t.Errorf("evidence count = %d; want 3 (lsp + ast + type_resolver)", len(out.Evidence))
	}
	// Sorted desc by confidence_contribution.
	for i := 1; i < len(out.Evidence); i++ {
		if out.Evidence[i].ConfidenceContribution > out.Evidence[i-1].ConfidenceContribution {
			t.Errorf("evidence not sorted desc at i=%d: %v > %v",
				i, out.Evidence[i].ConfidenceContribution, out.Evidence[i-1].ConfidenceContribution)
		}
	}
	if out.EvidenceStatus != EvidenceStatusComplete {
		t.Errorf("evidence_status = %q; want %q", out.EvidenceStatus, EvidenceStatusComplete)
	}
	if out.FallbackReason != "" {
		t.Errorf("fallback_reason = %q; want empty", out.FallbackReason)
	}
}

func TestValidateGraphEdge_LenientEmptyEvidence(t *testing.T) {
	// Edge present (some internal-kind row exists) but no citation metadata
	// (no Source / no TreeSitterKind / no tier). The handler MUST still return
	// non-zero confidence capped at ≤ 0.6 (TYPES-04) with evidence_status=none
	// and fallback_reason populated.
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	ev := &fakeEdgeEvidenceAccessor{
		rows: map[edgeKey][]EdgeEvidenceRow{
			{from, to, "CALLS"}: {
				// Edge exists but carries no source/tier metadata — purely "edge present".
				{InternalKind: "CALLS"},
			},
		},
	}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("expected success (D4 lenient), got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	if out.Confidence <= 0 {
		t.Errorf("confidence = %v; want > 0 (D4 lenient: edge exists)", out.Confidence)
	}
	if out.Confidence > 0.6 {
		t.Errorf("confidence = %v; want ≤ 0.6 (TYPES-04 cap when evidence_status != complete)", out.Confidence)
	}
	if out.FallbackReason == "" {
		t.Errorf("fallback_reason empty; want a closed-enum reason (evidence_lookup_lagging)")
	}
}

func TestValidateGraphEdge_TYPES04Cap(t *testing.T) {
	// Only type_resolver tier6_heuristic evidence — raw additive could exceed
	// 0.6 in theory; top-level Confidence MUST clamp to ≤ 0.6 via
	// types.CapCommentConfidence. Per-source confidence_contribution in the
	// evidence array is NOT clamped (D4 asymmetry).
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/types.go::Request")

	ev := &fakeEdgeEvidenceAccessor{
		rows: map[edgeKey][]EdgeEvidenceRow{
			{from, to, "RESOLVES_TO"}: {
				{
					InternalKind: "RESOLVES_TO",
					Tier:         "tier6_heuristic",
					EvidenceKind: "heuristic",
				},
			},
		},
	}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "has_type",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	if out.Confidence > 0.6 {
		t.Errorf("top-level confidence = %v; must be ≤ 0.6 (TYPES-04 cap on tier6)", out.Confidence)
	}
	// Per-source contribution NOT clamped — heuristic scales to 0.1, but the
	// asymmetry is documented: per-source values may be < 0.6 already; this
	// case asserts the per-source field exists and is non-zero.
	if len(out.Evidence) == 0 {
		t.Fatalf("evidence array empty; want type_resolver citation")
	}
	if out.Evidence[0].ConfidenceContribution <= 0 {
		t.Errorf("per-source contribution non-positive: %v", out.Evidence[0].ConfidenceContribution)
	}
}

func TestValidateGraphEdge_EdgeAbsent(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	// No rows for any (from, to, kind) tuple.
	ev := &fakeEdgeEvidenceAccessor{rows: map[edgeKey][]EdgeEvidenceRow{}}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("edge-absent must NOT be an error envelope; got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	if out.Confidence != 0 {
		t.Errorf("confidence = %v; want 0 (edge absent)", out.Confidence)
	}
	if len(out.Evidence) != 0 {
		t.Errorf("len(evidence) = %d; want 0", len(out.Evidence))
	}
	if out.EvidenceStatus != EvidenceStatusNone {
		t.Errorf("evidence_status = %q; want %q", out.EvidenceStatus, EvidenceStatusNone)
	}
	if out.FallbackReason != "edge_not_found" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "edge_not_found")
	}
}

func TestValidateGraphEdge_EdgeKindClosedEnum(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	ev := &fakeEdgeEvidenceAccessor{}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "FREEFORM_KIND",
	})
	if !res.IsError {
		t.Fatalf("expected error envelope for freeform edge_kind; got success")
	}
}

func TestValidateGraphEdge_EvidenceCapAt10(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	// Construct 12 LSP citations on a single CALLS edge.
	rows := make([]EdgeEvidenceRow, 0, 12)
	for i := 0; i < 12; i++ {
		rows = append(rows, EdgeEvidenceRow{
			InternalKind: "CALLS",
			Source:       "lsp.go.text_document_references",
			File:         "repo/src/svc.go",
		})
	}
	ev := &fakeEdgeEvidenceAccessor{
		rows: map[edgeKey][]EdgeEvidenceRow{
			{from, to, "CALLS"}: rows,
		},
	}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	if len(out.Evidence) != 10 {
		t.Errorf("len(evidence) = %d; want 10 (cap)", len(out.Evidence))
	}
	if out.EvidenceCountTotal < 12 {
		t.Errorf("evidence_count_total = %d; want ≥ 12", out.EvidenceCountTotal)
	}
	if out.EvidenceCountReturned != 10 {
		t.Errorf("evidence_count_returned = %d; want 10", out.EvidenceCountReturned)
	}
}

func TestValidateGraphEdge_BothSeedsResolution(t *testing.T) {
	// from=symbol_id (exact); to=(file,name) where the accessor returns >1
	// candidate so resolution=ambiguous.
	fx := buildPopulatedGraphFixture(t)
	const ambigPath = "repo/src/svc.go"
	const ambigName = "DupName"
	ambig := &ambigSymbolByName{ids: []integ.SymbolID{
		"repo/src/svc.go::DupName#a",
		"repo/src/svc.go::DupName#b",
	}}
	fx.SymbolByName = ambig

	ev := &fakeEdgeEvidenceAccessor{}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(fx.GoSeedSymbolID)},
		To:       SeedInput{FilePath: ambigPath, SymbolName: ambigName},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	if out.FromResolution != ResolutionExact {
		t.Errorf("from_resolution = %q; want %q", out.FromResolution, ResolutionExact)
	}
	if out.ToResolution != ResolutionAmbiguous {
		t.Errorf("to_resolution = %q; want %q", out.ToResolution, ResolutionAmbiguous)
	}
}

func TestValidateGraphEdge_FromNotFound(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	ev := &fakeEdgeEvidenceAccessor{}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{FilePath: "no/such/file.go", SymbolName: "Missing"},
		To:       SeedInput{SymbolID: string(fx.GoSeedSymbolID)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("not_found must NOT be an error envelope; got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	if out.Confidence != 0 {
		t.Errorf("confidence = %v; want 0", out.Confidence)
	}
	if len(out.Evidence) != 0 {
		t.Errorf("len(evidence) = %d; want 0", len(out.Evidence))
	}
	if out.EvidenceStatus != EvidenceStatusNone {
		t.Errorf("evidence_status = %q; want %q", out.EvidenceStatus, EvidenceStatusNone)
	}
	if out.FallbackReason != "symbol_not_found" {
		t.Errorf("fallback_reason = %q; want %q", out.FallbackReason, "symbol_not_found")
	}
}

func TestValidateGraphEdge_ASTCitationFallback(t *testing.T) {
	// Open Q3 resolution: when an AST contribution is graph-attested but the
	// extractor did not stamp tree_sitter_kind/range, the citation emits
	// source=ast / tree_sitter_kind="" and the envelope evidence_status drops
	// to partial.
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	ev := &fakeEdgeEvidenceAccessor{
		rows: map[edgeKey][]EdgeEvidenceRow{
			{from, to, "CALLS"}: {
				// AST contribution attested by graph but no extractor metadata.
				{InternalKind: "CALLS", ASTAttested: true},
			},
		},
	}
	s, _ := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	out := unmarshalValidateEdgeResult(t, textOf(res))
	foundASTPartial := false
	for _, c := range out.Evidence {
		if c.Source == EvidenceSourceAST && c.TreeSitterKind == "" {
			foundASTPartial = true
		}
	}
	if !foundASTPartial {
		t.Errorf("expected AST citation with empty tree_sitter_kind; got %+v", out.Evidence)
	}
	if out.EvidenceStatus != EvidenceStatusPartial {
		t.Errorf("evidence_status = %q; want %q", out.EvidenceStatus, EvidenceStatusPartial)
	}
}

func TestValidateGraphEdge_ReadOnly(t *testing.T) {
	fx := buildPopulatedGraphFixture(t)
	from := fx.GoSeedSymbolID
	to := integ.SymbolID("repo/src/svc.go::handle")

	ev := &fakeEdgeEvidenceAccessor{
		rows: map[edgeKey][]EdgeEvidenceRow{
			{from, to, "CALLS"}: {
				{InternalKind: "CALLS", Source: "lsp.go.text_document_references"},
			},
		},
	}
	s, store := newSkillForValidateEdgeTest(t, "read", fx, ev)

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{SymbolID: string(from)},
		To:       SeedInput{SymbolID: string(to)},
		EdgeKind: "calls",
	})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if store.beginSnapshotCalls.Load()+store.commitSnapshotCalls.Load()+
		store.abortSnapshotCalls.Load()+store.writeSnapshotFactsCalls.Load() != 0 {
		t.Errorf("D-09 violation: snapshot-write canary fired")
	}
}

func TestValidateGraphEdge_ModeRejected(t *testing.T) {
	// modeTierRead is the lowest tier — every session passes. We exercise the
	// alternate rejection path: SymbolByName unwired drives an error envelope
	// before any accessor is touched. Mirrors 71-03/71-04's stance.
	fx := buildPopulatedGraphFixture(t)
	ev := &fakeEdgeEvidenceAccessor{}
	s, store := newSkillForValidateEdgeTest(t, "read", fx, ev)
	s.SetSymbolByName(nil) // force resolver-unwired error

	res := s.handleValidateGraphEdge(context.Background(), ValidateGraphEdgeArgs{
		From:     SeedInput{FilePath: "x.go", SymbolName: "Y"},
		To:       SeedInput{FilePath: "z.go", SymbolName: "Q"},
		EdgeKind: "calls",
	})
	if !res.IsError {
		t.Fatalf("expected error envelope when resolver unwired; got success")
	}
	if store.beginSnapshotCalls.Load()+store.commitSnapshotCalls.Load()+
		store.abortSnapshotCalls.Load()+store.writeSnapshotFactsCalls.Load() != 0 {
		t.Errorf("snapshot-write canary fired on error path")
	}
}
