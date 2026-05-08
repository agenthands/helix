// blast_radius_strangler_test.go — Phase 65 65-06 RED tests for the two-pass
// analyze_blast_radius orchestrator.
//
// Test matrix (mirrors the 65-06 PLAN <behavior> block):
//
//   1. TestAnalyzeBlastRadius_SemanticTwoPass — Pass 1 + Pass 2, validated +
//      refuted edges.
//   2. TestAnalyzeBlastRadius_CfgDisabled_TreeSitter — D-04 / Pitfall §3:
//      cfg-disabled emits source=tree_sitter (NOT fallback+index_disabled).
//   3. TestAnalyzeBlastRadius_LookupUnavailable_Fallback — defensive D-05:
//      cfg enabled but lookup unavailable.
//   4. TestAnalyzeBlastRadius_Pass1Error_FallsBackToLSP — ExpandFrom err
//      classifies via ClassifyLookupErr.
//   5. TestAnalyzeBlastRadius_Pass2Error_KeepsPass1 — ValidateCriticalEdges
//      err leaves Pass 1 confidences untouched.
//   6. TestFilterCritical_PublicAPIBoundary — unit: filterCritical helper.
//   7. TestApplyValidationVerdicts_NoMutation — unit: copy-before-mutate
//      contract (Pitfall §6).
//
// Tests use a fakeLookup hand-rolled SemanticLookup test double + fakeCfg
// ConfigGate test double; they exercise the orchestrator helpers
// (analyzeBlastRadiusViaLookup, filterCritical, applyValidationVerdicts,
// capConfidences) directly without spinning up a real MCP server.
package symbols

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/workspace"
	gen "github.com/agenthands/helix/protocol/gen"
)

// fakeLookup is a hand-rolled SemanticLookup test double mirroring the
// test-side double from internal/skill/repomap/strangler_test.go. It is
// M-readtier safe by construction: never touches the snapshot-write surface.
type fakeLookup struct {
	available    bool
	symbolID     integ.SymbolID
	symErr       error
	expandResult []integ.Impact
	expandErr    error
	validateRes  []integ.ValidatedEdge
	validateErr  error
	status       integ.SemanticStatus
	// locate is the optional LocateSymbol hook (Phase 65 65-12 Task 1).
	// nil → returns ("", 0, 0, false, nil); non-nil → invoked verbatim.
	locate func(integ.SymbolID) (string, uint32, uint32, bool, error)
}

func (f *fakeLookup) Available() bool { return f.available }
func (f *fakeLookup) SymbolID(_ context.Context, _ workspace.WorkspaceKey, _ string, _, _ uint32) (integ.SymbolID, error) {
	if f.symErr != nil {
		return integ.SymbolID(""), f.symErr
	}
	return f.symbolID, nil
}
func (f *fakeLookup) RankFiles(_ context.Context, _ workspace.WorkspaceKey) ([]integ.RankedFile, error) {
	return nil, integ.ErrNoSnapshot
}
func (f *fakeLookup) RankFromSeeds(_ context.Context, _ workspace.WorkspaceKey, _ []string) ([]integ.RankedFile, error) {
	return nil, integ.ErrNoSnapshot
}
func (f *fakeLookup) ExpandFrom(_ context.Context, _ workspace.WorkspaceKey, _ integ.SymbolID, _ int) ([]integ.Impact, error) {
	if f.expandErr != nil {
		return nil, f.expandErr
	}
	// Return a copy so the test can inspect the original after the orchestrator
	// completes (and verify Pitfall §6 copy-before-mutate).
	out := make([]integ.Impact, len(f.expandResult))
	copy(out, f.expandResult)
	return out, nil
}
func (f *fakeLookup) ValidateCriticalEdges(_ context.Context, _ workspace.WorkspaceKey, edges []integ.Edge) ([]integ.ValidatedEdge, error) {
	if f.validateErr != nil {
		return nil, f.validateErr
	}
	return f.validateRes, nil
}
func (f *fakeLookup) Status(_ context.Context, _ workspace.WorkspaceKey) (integ.SemanticStatus, error) {
	return f.status, nil
}
func (f *fakeLookup) LocateSymbol(_ context.Context, _ workspace.WorkspaceKey, sym integ.SymbolID) (string, uint32, uint32, bool, error) {
	// Phase 65 65-12 Task 2: existing orchestrator tests never call
	// LocateSymbol (they use lspProbeFn=nil). The new accumulator test
	// for lspProbeForEdges injects locateFn directly; this stub exists
	// purely to satisfy the interface. Returns a clean miss so the
	// kernel-side probe — if ever wired to this fake — drops every
	// edge as Refuted, which the new tests assert.
	if locFn := f.locate; locFn != nil {
		return locFn(sym)
	}
	return "", 0, 0, false, nil
}

// fakeCfg is the test ConfigGate double matching the production daemonCfgGate.
type fakeCfg struct{ enabled bool }

func (f *fakeCfg) SemanticIndexEnabled() bool { return f.enabled }

// fakeLSP is the LSP-fallback test double. The real registerAnalyzeBlastRadius
// closes over a *lspool.WorkerLease + AnalyzeBlastRadius LSP call; in tests we
// inject a synthetic *BlastRadius via a function pointer the orchestrator
// honours.
type fakeLSP struct {
	br  *BlastRadius
	err error
}

func (f *fakeLSP) Analyze() (*BlastRadius, error) {
	return f.br, f.err
}

// envelopePayload captures the on-wire envelope shape parsed from the
// orchestrator's MarshalEnvelope output.
type envelopePayload struct {
	Source         string                   `json:"source"`
	FallbackReason string                   `json:"fallback_reason,omitempty"`
	GraphVersion   uint64                   `json:"graph_version,omitempty"`
	PerNode        []map[string]interface{} `json:"per_node"`
}

// helper: build a BlastRadius with PerNode entries set to the supplied
// confidences (NodeImpact.Confidence). Used by the LSP-fallback tests so we
// can assert the cap is applied uniformly.
func brWithConfidences(confs ...float64) *BlastRadius {
	br := &BlastRadius{}
	br.PerNode = make([]NodeImpact, 0, len(confs))
	for i, c := range confs {
		br.PerNode = append(br.PerNode, NodeImpact{
			SymbolID:   string(rune('A' + i)),
			Confidence: c,
			Evidence:   integ.Evidence{},
		})
	}
	return br
}

// ---------------------------------------------------------------------------
// 1. Semantic two-pass success path.
// ---------------------------------------------------------------------------

func TestAnalyzeBlastRadius_SemanticTwoPass(t *testing.T) {
	ctx := context.Background()
	ws := workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go"}

	// Pass 1: 5 impacts at varying confidences (mix of critical and not).
	// Edges A and B are "critical" (confidence < 0.80) — they will get
	// validated in Pass 2. Edges C, D, E remain non-critical (≥ 0.80) and
	// must retain their Pass 1 confidence values byte-for-byte.
	edgeA := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	edgeB := integ.Edge{From: "src", To: "tgt-B", Kind: "calls", Confidence: 0.70}
	edgeC := integ.Edge{From: "src", To: "tgt-C", Kind: "calls", Confidence: 0.80}
	edgeD := integ.Edge{From: "src", To: "tgt-D", Kind: "calls", Confidence: 0.90}
	edgeE := integ.Edge{From: "src", To: "tgt-E", Kind: "calls", Confidence: 0.95}
	pass1 := []integ.Impact{
		{SymbolID: "tgt-A", Confidence: 0.45, Evidence: integ.Evidence{Edges: []integ.Edge{edgeA}}},
		{SymbolID: "tgt-B", Confidence: 0.70, Evidence: integ.Evidence{Edges: []integ.Edge{edgeB}}},
		{SymbolID: "tgt-C", Confidence: 0.80, Evidence: integ.Evidence{Edges: []integ.Edge{edgeC}}},
		{SymbolID: "tgt-D", Confidence: 0.90, Evidence: integ.Evidence{Edges: []integ.Edge{edgeD}}},
		{SymbolID: "tgt-E", Confidence: 0.95, Evidence: integ.Evidence{Edges: []integ.Edge{edgeE}}},
	}

	// Pass 2: A is confirmed (→ confidence 1.00), B is refuted (→ 0.20 +
	// Refuted=true).
	pass2 := []integ.ValidatedEdge{
		{Edge: edgeA, LSPConfirmed: true},
		{Edge: edgeB, LSPConfirmed: false},
	}

	lookup := &fakeLookup{
		available:    true,
		symbolID:     integ.SymbolID("src"),
		expandResult: pass1,
		validateRes:  pass2,
	}

	// Phase 65 65-12 Task 2 BL-3 migration: pass nil lspProbeFn so the
// orchestrator falls back to lookup.ValidateCriticalEdges — preserves
// the legacy semantic exercised by the matrix test fakes (matrixLookup,
// fakeLookup) which still drive synthetic verdicts.
impacts, src, reason, _, err := analyzeBlastRadiusViaLookup(ctx, lookup, ws, integ.SymbolID("src"), nil)
	require.NoError(t, err)
	assert.Equal(t, integ.SourceSemantic, src)
	assert.Empty(t, string(reason))
	require.Len(t, impacts, 5)

	// Confirmed: A flipped to 1.00.
	assert.Equal(t, 1.00, impacts[0].Confidence, "confirmed edge A must flip to 1.00")
	assert.False(t, impacts[0].Refuted, "confirmed edge A must not be marked refuted")

	// Refuted: B flipped to 0.20 with Refuted=true.
	assert.InDelta(t, 0.20, impacts[1].Confidence, 1e-9, "refuted edge B must flip to 0.20")
	assert.True(t, impacts[1].Refuted, "refuted edge B must set Refuted=true")

	// Non-critical: C, D, E retain Pass 1 confidence byte-identical.
	assert.Equal(t, 0.80, impacts[2].Confidence, "non-critical edge C must retain Pass 1 confidence")
	assert.Equal(t, 0.90, impacts[3].Confidence, "non-critical edge D must retain Pass 1 confidence")
	assert.Equal(t, 0.95, impacts[4].Confidence, "non-critical edge E must retain Pass 1 confidence")
}

// ---------------------------------------------------------------------------
// 2. Cfg-disabled — D-04 / Pitfall §3 contract: source=tree_sitter, NOT
// fallback+index_disabled.
// ---------------------------------------------------------------------------

func TestAnalyzeBlastRadius_CfgDisabled_TreeSitter(t *testing.T) {
	cfg := &fakeCfg{enabled: false}
	// Lookup.Available() == true, but cfg-disabled wins per priority ladder.
	lookup := &fakeLookup{available: true}

	src, reason := integ.ChooseSource(cfg, lookup, nil)
	require.Equal(t, integ.SourceTreeSitter, src,
		"cfg-disabled MUST emit SourceTreeSitter (D-04 + Pitfall §3) — not SourceFallback")
	require.Empty(t, string(reason),
		"cfg-disabled SourceTreeSitter MUST have empty FallbackReason (NOT 'index_disabled')")

	// LSP primitive returns 4 PerNode results with confidences 0.8/0.9/1.0/0.7.
	br := brWithConfidences(0.8, 0.9, 1.0, 0.7)
	capConfidences(br, 0.6)

	for i, n := range br.PerNode {
		assert.LessOrEqual(t, n.Confidence, 0.6,
			"PerNode[%d] confidence must be hard-capped at 0.6 (D-08; M-confcap; ROADMAP SC #2)", i)
	}

	// Envelope render — wire shape sanity.
	bytesOut, err := formatBlastRadiusEnvelope(br, src, reason)
	require.NoError(t, err)
	var env envelopePayload
	require.NoError(t, json.Unmarshal(bytesOut, &env))
	assert.Equal(t, "tree_sitter", env.Source,
		"envelope.source MUST be 'tree_sitter' on cfg-disabled (D-04)")
	assert.Empty(t, env.FallbackReason,
		"envelope.fallback_reason MUST be empty on cfg-disabled (NOT 'index_disabled')")
}

// ---------------------------------------------------------------------------
// 3. Defensive D-05 — cfg enabled but lookup unavailable.
// ---------------------------------------------------------------------------

func TestAnalyzeBlastRadius_LookupUnavailable_Fallback(t *testing.T) {
	cfg := &fakeCfg{enabled: true}
	lookup := &fakeLookup{available: false}

	src, reason := integ.ChooseSource(cfg, lookup, nil)
	require.Equal(t, integ.SourceFallback, src,
		"cfg.Enabled=true && !lookup.Available() MUST emit SourceFallback (defensive D-05)")
	require.Equal(t, integ.FallbackReasonIndexDisabled, reason,
		"defensive D-05 MUST stamp FallbackReasonIndexDisabled per source_select.go ladder")

	br := brWithConfidences(0.8, 0.9, 1.0, 0.7)
	capConfidences(br, 0.6)
	for i, n := range br.PerNode {
		assert.LessOrEqual(t, n.Confidence, 0.6,
			"PerNode[%d] confidence must be hard-capped on fallback path too", i)
	}

	bytesOut, err := formatBlastRadiusEnvelope(br, src, reason)
	require.NoError(t, err)
	var env envelopePayload
	require.NoError(t, json.Unmarshal(bytesOut, &env))
	assert.Equal(t, "fallback", env.Source)
	assert.Equal(t, "index_disabled", env.FallbackReason)
}

// ---------------------------------------------------------------------------
// 4. Pass 1 error — ExpandFrom returns ErrIndexBuilding → fallback with
// classified reason.
// ---------------------------------------------------------------------------

func TestAnalyzeBlastRadius_Pass1Error_FallsBackToLSP(t *testing.T) {
	ctx := context.Background()
	ws := workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go"}

	lookup := &fakeLookup{
		available: true,
		symbolID:  integ.SymbolID("src"),
		expandErr: integ.ErrIndexBuilding,
	}

	// Phase 65 65-12 Task 2 BL-3 migration: pass nil lspProbeFn so the
// orchestrator falls back to lookup.ValidateCriticalEdges — preserves
// the legacy semantic exercised by the matrix test fakes (matrixLookup,
// fakeLookup) which still drive synthetic verdicts.
impacts, src, reason, _, err := analyzeBlastRadiusViaLookup(ctx, lookup, ws, integ.SymbolID("src"), nil)
	require.Error(t, err, "Pass 1 ExpandFrom error MUST surface as the function error so caller can drop to LSP")
	assert.Equal(t, integ.SourceFallback, src)
	assert.Equal(t, integ.FallbackReasonIndexBuilding, reason,
		"ClassifyLookupErr(ErrIndexBuilding) → FallbackReasonIndexBuilding")
	assert.Nil(t, impacts, "Pass 1 error returns no impacts; caller falls through to LSP path")

	// Caller path: capConfidences applied to LSP fallback BlastRadius.
	br := brWithConfidences(0.8, 0.9, 1.0, 0.7)
	capConfidences(br, 0.6)
	for i, n := range br.PerNode {
		assert.LessOrEqual(t, n.Confidence, 0.6,
			"PerNode[%d] confidence must be hard-capped on Pass 1 error fallback path", i)
	}

	bytesOut, encErr := formatBlastRadiusEnvelope(br, src, reason)
	require.NoError(t, encErr)
	var env envelopePayload
	require.NoError(t, json.Unmarshal(bytesOut, &env))
	assert.Equal(t, "fallback", env.Source)
	assert.Equal(t, "index_building", env.FallbackReason)
}

// ---------------------------------------------------------------------------
// 5. Pass 2 error — ValidateCriticalEdges errors; orchestrator keeps Pass 1
// confidences untouched and reports source=semantic.
// ---------------------------------------------------------------------------

func TestAnalyzeBlastRadius_Pass2Error_KeepsPass1(t *testing.T) {
	ctx := context.Background()
	ws := workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go"}

	// 3 impacts: A (0.45) and B (0.70) are critical (confidence < 0.80); C
	// (0.90) is not. The Pass 2 call will be issued for A+B and will error.
	edgeA := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	edgeB := integ.Edge{From: "src", To: "tgt-B", Kind: "calls", Confidence: 0.70}
	edgeC := integ.Edge{From: "src", To: "tgt-C", Kind: "calls", Confidence: 0.90}
	pass1 := []integ.Impact{
		{SymbolID: "tgt-A", Confidence: 0.45, Evidence: integ.Evidence{Edges: []integ.Edge{edgeA}}},
		{SymbolID: "tgt-B", Confidence: 0.70, Evidence: integ.Evidence{Edges: []integ.Edge{edgeB}}},
		{SymbolID: "tgt-C", Confidence: 0.90, Evidence: integ.Evidence{Edges: []integ.Edge{edgeC}}},
	}

	lookup := &fakeLookup{
		available:    true,
		symbolID:     integ.SymbolID("src"),
		expandResult: pass1,
		validateErr:  errStub("network error"),
	}

	// Phase 65 65-12 Task 2 BL-3 migration: pass nil lspProbeFn so the
// orchestrator falls back to lookup.ValidateCriticalEdges — preserves
// the legacy semantic exercised by the matrix test fakes (matrixLookup,
// fakeLookup) which still drive synthetic verdicts.
impacts, src, reason, _, err := analyzeBlastRadiusViaLookup(ctx, lookup, ws, integ.SymbolID("src"), nil)
	require.NoError(t, err, "Pass 2 LSP error is non-fatal — Pass 1 still trusted")
	assert.Equal(t, integ.SourceSemantic, src,
		"Pass 2 error MUST keep source=semantic — Pass 1 result is the canonical answer")
	assert.Empty(t, string(reason))
	require.Len(t, impacts, 3)

	// All 3 impacts retain their Pass 1 confidences — no flip to 1.00 / 0.20.
	assert.Equal(t, 0.45, impacts[0].Confidence, "Pass 2 error MUST NOT flip A to 1.00 or 0.20")
	assert.Equal(t, 0.70, impacts[1].Confidence, "Pass 2 error MUST NOT flip B to 1.00 or 0.20")
	assert.Equal(t, 0.90, impacts[2].Confidence, "non-critical C MUST retain 0.90")
	for i, im := range impacts {
		assert.False(t, im.Refuted, "Pass 2 error MUST NOT mark impacts[%d] as Refuted", i)
	}
}

// errStub is a plain error type for the Pass 2 error test; not a sentinel
// from integ — confirms the orchestrator does NOT propagate non-classified
// errors through the function-error return on the Pass 2 path.
type errStub string

func (e errStub) Error() string { return string(e) }

// ---------------------------------------------------------------------------
// 6. filterCritical helper — public-API boundary + low-confidence union.
// ---------------------------------------------------------------------------

func TestFilterCritical_PublicAPIBoundary(t *testing.T) {
	// Edge confidences and crossesPublicAPI table:
	//
	//   imp[0]: low confidence, in-package      → critical (low conf)
	//   imp[1]: high confidence, cross-pkg+exported → critical (public API)
	//   imp[2]: high confidence, in-package     → NOT critical
	//   imp[3]: high confidence, cross-pkg+unexported → NOT critical
	//   imp[4]: low confidence, cross-pkg+exported  → critical (both)
	imps := []integ.Impact{
		{SymbolID: "a", Confidence: 0.45, Evidence: integ.Evidence{Edges: []integ.Edge{
			{From: integ.SymbolID("pkg/foo:Bar"), To: integ.SymbolID("pkg/foo:Baz")},
		}}},
		{SymbolID: "b", Confidence: 0.95, Evidence: integ.Evidence{Edges: []integ.Edge{
			{From: integ.SymbolID("pkg/foo:Bar"), To: integ.SymbolID("pkg/other:Qux")},
		}}},
		{SymbolID: "c", Confidence: 0.95, Evidence: integ.Evidence{Edges: []integ.Edge{
			{From: integ.SymbolID("pkg/foo:Bar"), To: integ.SymbolID("pkg/foo:Internal")},
		}}},
		{SymbolID: "d", Confidence: 0.95, Evidence: integ.Evidence{Edges: []integ.Edge{
			{From: integ.SymbolID("pkg/foo:Bar"), To: integ.SymbolID("pkg/other:internal")},
		}}},
		{SymbolID: "e", Confidence: 0.40, Evidence: integ.Evidence{Edges: []integ.Edge{
			{From: integ.SymbolID("pkg/foo:Bar"), To: integ.SymbolID("pkg/other:Pub")},
		}}},
	}

	out := filterCritical(imps)
	require.Len(t, out, 3, "filterCritical: 3 entries (low conf | crosses public-API)")
	assert.Equal(t, integ.SymbolID("a"), out[0].SymbolID, "low-confidence in-pkg is critical")
	assert.Equal(t, integ.SymbolID("b"), out[1].SymbolID, "cross-pkg + exported target is critical")
	assert.Equal(t, integ.SymbolID("e"), out[2].SymbolID, "low conf + cross-pkg + exported is critical")
}

// ---------------------------------------------------------------------------
// 7a. applyValidationVerdicts — WR-05 accumulator semantics (Phase 65 65-11).
// ---------------------------------------------------------------------------

// TestApplyValidationVerdicts_RefutationTaintsRegardlessOfConfirmation pins
// WR-05: when an impact has multiple evidence edges and at least ONE edge is
// refuted, the impact MUST be marked Refuted regardless of any confirmations
// on sibling edges. The pre-WR-05 implementation used a `break` after the
// first matching verdict, so a confirmation on edge[0] would mask a refutation
// on edge[1].
func TestApplyValidationVerdicts_RefutationTaintsRegardlessOfConfirmation(t *testing.T) {
	edgeConfirmed := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	edgeRefuted := integ.Edge{From: "src", To: "tgt-B", Kind: "calls", Confidence: 0.45}

	impacts := []integ.Impact{
		{
			SymbolID:   "tgt-multi",
			Confidence: 0.45,
			Evidence: integ.Evidence{Edges: []integ.Edge{
				edgeConfirmed, // first edge: confirmed
				edgeRefuted,   // second edge: refuted — MUST taint the impact
			}},
		},
	}

	verdicts := []integ.ValidatedEdge{
		{Edge: edgeConfirmed, LSPConfirmed: true},
		{Edge: edgeRefuted, LSPConfirmed: false},
	}

	applyValidationVerdicts(impacts, verdicts)

	assert.InDelta(t, 0.20, impacts[0].Confidence, 1e-9,
		"WR-05: any single refutation MUST drop confidence to 0.20 even when sibling edges were confirmed")
	assert.True(t, impacts[0].Refuted,
		"WR-05: any single refutation MUST set Refuted=true regardless of sibling confirmations")
}

// TestApplyValidationVerdicts_AllConfirmedFlipsToOne preserves the existing
// confirmed-only path: when every matched verdict is LSPConfirmed=true and no
// edges are refuted, Confidence flips to 1.00 and Refuted stays false.
func TestApplyValidationVerdicts_AllConfirmedFlipsToOne(t *testing.T) {
	edgeA := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	edgeB := integ.Edge{From: "src", To: "tgt-B", Kind: "calls", Confidence: 0.45}

	impacts := []integ.Impact{
		{
			SymbolID:   "tgt-multi",
			Confidence: 0.45,
			Evidence:   integ.Evidence{Edges: []integ.Edge{edgeA, edgeB}},
		},
	}

	verdicts := []integ.ValidatedEdge{
		{Edge: edgeA, LSPConfirmed: true},
		{Edge: edgeB, LSPConfirmed: true},
	}

	applyValidationVerdicts(impacts, verdicts)
	assert.Equal(t, 1.00, impacts[0].Confidence,
		"WR-05: every-confirmed impact MUST flip Confidence to 1.00")
	assert.False(t, impacts[0].Refuted,
		"WR-05: every-confirmed impact MUST NOT be Refuted")
}

// ---------------------------------------------------------------------------
// 7b. formatBlastRadiusEnvelopeFromImpacts — WR-03 graph_version stamp.
// ---------------------------------------------------------------------------

// TestFormatBlastRadiusEnvelopeFromImpacts_StampsGraphVersion pins WR-03: the
// semantic-arm envelope MUST carry the graph_version threaded from
// lookup.Status. The pre-WR-03 implementation hardcoded GraphVersion=0 (the
// integ.Envelope zero-value), so the envelope omitted the key entirely.
func TestFormatBlastRadiusEnvelopeFromImpacts_StampsGraphVersion(t *testing.T) {
	impacts := []integ.Impact{
		{SymbolID: "X", Confidence: 0.95, Evidence: integ.Evidence{}},
	}
	const wantGV uint64 = 4242

	out, err := formatBlastRadiusEnvelopeFromImpacts(impacts, integ.SourceSemantic, "", wantGV)
	require.NoError(t, err)

	var env envelopePayload
	require.NoError(t, json.Unmarshal(out, &env))
	assert.Equal(t, "semantic", env.Source)
	assert.Equal(t, wantGV, env.GraphVersion,
		"WR-03: envelope.graph_version MUST be the value threaded from lookup.Status")
}

// ---------------------------------------------------------------------------
// 7. applyValidationVerdicts — copy-before-mutate (Pitfall §6).
// ---------------------------------------------------------------------------

func TestApplyValidationVerdicts_NoMutation(t *testing.T) {
	edge := integ.Edge{From: "from", To: "to", Kind: "calls", Confidence: 0.45}
	pass1 := []integ.Impact{
		{SymbolID: "to", Confidence: 0.45, Evidence: integ.Evidence{Edges: []integ.Edge{edge}}},
	}
	// Snapshot Pass 1 confidence before any mutation.
	originalConf := pass1[0].Confidence

	// Copy Pass 1 (orchestrator contract), then apply verdicts to the COPY.
	copyOf := make([]integ.Impact, len(pass1))
	copy(copyOf, pass1)
	verdicts := []integ.ValidatedEdge{{Edge: edge, LSPConfirmed: true}}
	applyValidationVerdicts(copyOf, verdicts)

	// The copy reflects the verdict (1.00).
	assert.Equal(t, 1.00, copyOf[0].Confidence,
		"verdicts MUST flip the copy's confidence to 1.00")

	// The ORIGINAL Pass 1 slice retains the pre-mutation confidence.
	assert.Equal(t, originalConf, pass1[0].Confidence,
		"original Pass 1 slice MUST be untouched (Pitfall §6 copy-before-mutate)")
}

// ---------------------------------------------------------------------------
// 8. lspProbeForEdges — Phase 65 65-12 Task 2 kernel-side LSP probe.
// ---------------------------------------------------------------------------

// lspProbeFixture wires a synthetic locator + probeFn for the unit tests:
// the locator is a static map sym → (path, line, col); probeFn is a static
// map (uri, line, col) → []SymbolLocation OR an injected error.
type lspProbeFixture struct {
	t          *testing.T
	locations  map[integ.SymbolID]struct{ Path string; Line, Col uint32 }
	probeFn    func(ctx context.Context, uri string, line, col int) ([]SymbolLocation, error)
}

func (f *lspProbeFixture) locator(sym integ.SymbolID) (string, uint32, uint32, bool) {
	loc, ok := f.locations[sym]
	if !ok {
		return "", 0, 0, false
	}
	return loc.Path, loc.Line, loc.Col, true
}

// TestLspProbeForEdges_ConfirmsRealEdge — happy path: probeFn returns a
// SymbolLocation overlapping edge.To's coordinates → LSPConfirmed=true.
func TestLspProbeForEdges_ConfirmsRealEdge(t *testing.T) {
	repoRoot := "/repo"
	edge := integ.Edge{From: "from-sym", To: "to-sym", Kind: "calls"}
	fix := &lspProbeFixture{
		t: t,
		locations: map[integ.SymbolID]struct{ Path string; Line, Col uint32 }{
			"from-sym": {Path: "src/from.go", Line: 5, Col: 1},
			"to-sym":   {Path: "src/to.go", Line: 10, Col: 2},
		},
		probeFn: func(_ context.Context, uri string, line, col int) ([]SymbolLocation, error) {
			// Returns a single location that overlaps edge.To (line 10 col 2 → LSP 9, 1).
			return []SymbolLocation{{
				URI: pathToURI(repoRoot, "src/to.go"),
				Range: gen.Range{
					Start: gen.Position{Line: 9, Character: 1},
					End:   gen.Position{Line: 9, Character: 5},
				},
			}}, nil
		},
	}
	out := lspProbeForEdges(context.Background(), []integ.Edge{edge}, fix.locator, fix.probeFn, repoRoot)
	require.Len(t, out, 1)
	assert.True(t, out[0].LSPConfirmed, "edge whose probe returns an overlapping location MUST be confirmed")
	assert.Equal(t, edge, out[0].Edge)
}

// TestLspProbeForEdges_RefutesMissingEdge — probeFn returns no locations →
// LSPConfirmed=false.
func TestLspProbeForEdges_RefutesMissingEdge(t *testing.T) {
	repoRoot := "/repo"
	edge := integ.Edge{From: "from-sym", To: "to-sym", Kind: "calls"}
	fix := &lspProbeFixture{
		t: t,
		locations: map[integ.SymbolID]struct{ Path string; Line, Col uint32 }{
			"from-sym": {Path: "src/from.go", Line: 5, Col: 1},
			"to-sym":   {Path: "src/to.go", Line: 10, Col: 2},
		},
		probeFn: func(_ context.Context, _ string, _, _ int) ([]SymbolLocation, error) {
			return nil, nil
		},
	}
	out := lspProbeForEdges(context.Background(), []integ.Edge{edge}, fix.locator, fix.probeFn, repoRoot)
	require.Len(t, out, 1)
	assert.False(t, out[0].LSPConfirmed, "edge whose probe returns no locations MUST be refuted")
}

// TestLspProbeForEdges_LocatorMissReturnsRefuted — locator misses on either
// endpoint → edge is refuted without invoking probeFn.
func TestLspProbeForEdges_LocatorMissReturnsRefuted(t *testing.T) {
	repoRoot := "/repo"
	edge := integ.Edge{From: "missing-sym", To: "to-sym", Kind: "calls"}
	called := false
	fix := &lspProbeFixture{
		t: t,
		locations: map[integ.SymbolID]struct{ Path string; Line, Col uint32 }{
			"to-sym": {Path: "src/to.go", Line: 10, Col: 2},
		},
		probeFn: func(_ context.Context, _ string, _, _ int) ([]SymbolLocation, error) {
			called = true
			return nil, nil
		},
	}
	out := lspProbeForEdges(context.Background(), []integ.Edge{edge}, fix.locator, fix.probeFn, repoRoot)
	require.Len(t, out, 1)
	assert.False(t, out[0].LSPConfirmed, "edge whose endpoint locator misses MUST be refuted")
	assert.False(t, called, "probeFn MUST NOT be invoked when the locator misses")
}

// TestLspProbeForEdges_FindReferencesErrorReturnsRefuted — probeFn errors →
// LSPConfirmed=false, no fatal return.
func TestLspProbeForEdges_FindReferencesErrorReturnsRefuted(t *testing.T) {
	repoRoot := "/repo"
	edge := integ.Edge{From: "from-sym", To: "to-sym", Kind: "calls"}
	fix := &lspProbeFixture{
		t: t,
		locations: map[integ.SymbolID]struct{ Path string; Line, Col uint32 }{
			"from-sym": {Path: "src/from.go", Line: 5, Col: 1},
			"to-sym":   {Path: "src/to.go", Line: 10, Col: 2},
		},
		probeFn: func(_ context.Context, _ string, _, _ int) ([]SymbolLocation, error) {
			return nil, errors.New("probe error")
		},
	}
	out := lspProbeForEdges(context.Background(), []integ.Edge{edge}, fix.locator, fix.probeFn, repoRoot)
	require.Len(t, out, 1)
	assert.False(t, out[0].LSPConfirmed, "edge whose probe errored MUST be refuted (non-fatal)")
}

// TestLspProbeForEdges_AccumulatorSemantics — Phase 65 65-12 Task 2 BL-3
// regression: ensures the WR-05 accumulator semantics from 65-11
// (a single refutation taints the impact regardless of confirmations on
// sibling edges) ALSO fire when verdicts come from lspProbeFn (the
// kernel-side path) — not just from lookup.ValidateCriticalEdges. The
// existing TestApplyValidationVerdicts_RefutationTaintsRegardlessOfConfirmation
// guards the verdict applier; this test guards the kernel-side probe →
// applier path.
func TestLspProbeForEdges_AccumulatorSemantics(t *testing.T) {
	repoRoot := "/repo"

	edgeConfirmed := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	edgeRefuted := integ.Edge{From: "src", To: "tgt-B", Kind: "calls", Confidence: 0.45}

	fix := &lspProbeFixture{
		t: t,
		locations: map[integ.SymbolID]struct{ Path string; Line, Col uint32 }{
			"src":   {Path: "src/from.go", Line: 5, Col: 1},
			"tgt-A": {Path: "src/a.go", Line: 10, Col: 2},
			"tgt-B": {Path: "src/b.go", Line: 20, Col: 4},
		},
		probeFn: func(_ context.Context, uri string, _, _ int) ([]SymbolLocation, error) {
			// Always return a hit overlapping tgt-A but NEVER overlapping tgt-B.
			// The probe is fired once per edge with edge.From's coords; the
			// fixture's locations map reads back tgt-A from src/a.go.
			//
			// Since lspProbeForEdges issues the probe with edge.From's URI
			// (which is the same src/from.go for both edges), we return
			// locations covering tgt-A's coordinates — confirming the first
			// edge — but NOT tgt-B's, so the second is refuted.
			_ = uri // both edges share the same From URI
			return []SymbolLocation{{
				URI: pathToURI(repoRoot, "src/a.go"),
				Range: gen.Range{
					Start: gen.Position{Line: 9, Character: 1},
					End:   gen.Position{Line: 9, Character: 5},
				},
			}}, nil
		},
	}
	verdicts := lspProbeForEdges(context.Background(),
		[]integ.Edge{edgeConfirmed, edgeRefuted}, fix.locator, fix.probeFn, repoRoot)
	require.Len(t, verdicts, 2)
	require.True(t, verdicts[0].LSPConfirmed, "edge[0] (tgt-A) must be confirmed by the synthetic probe")
	require.False(t, verdicts[1].LSPConfirmed, "edge[1] (tgt-B) must be refuted (no overlap)")

	impacts := []integ.Impact{
		{
			SymbolID:   "tgt-multi",
			Confidence: 0.45,
			Evidence:   integ.Evidence{Edges: []integ.Edge{edgeConfirmed, edgeRefuted}},
		},
	}
	applyValidationVerdicts(impacts, verdicts)
	assert.InDelta(t, 0.20, impacts[0].Confidence, 1e-9,
		"BL-3: a single lspProbeFn refutation MUST drop confidence to 0.20 even when sibling was confirmed")
	assert.True(t, impacts[0].Refuted,
		"BL-3: a single lspProbeFn refutation MUST set Refuted=true regardless of sibling confirmations")
}

// TestAnalyzeBlastRadiusViaLookup_AppliesLspProbeWhenProvided pins the
// new orchestrator contract: when lspProbeFn != nil it is consulted
// INSTEAD of lookup.ValidateCriticalEdges.
func TestAnalyzeBlastRadiusViaLookup_AppliesLspProbeWhenProvided(t *testing.T) {
	ctx := context.Background()
	ws := workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go"}

	edgeA := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	pass1 := []integ.Impact{
		{SymbolID: "tgt-A", Confidence: 0.45, Evidence: integ.Evidence{Edges: []integ.Edge{edgeA}}},
	}
	// validateRes returns LSPConfirmed=false — if it were consulted,
	// confidence would NOT flip to 1.00. The lspProbeFn returns
	// LSPConfirmed=true and MUST win.
	lookup := &fakeLookup{
		available:    true,
		expandResult: pass1,
		validateRes: []integ.ValidatedEdge{
			{Edge: edgeA, LSPConfirmed: false},
		},
	}
	probeCalled := false
	lspProbeFn := func(_ context.Context, edges []integ.Edge) []integ.ValidatedEdge {
		probeCalled = true
		out := make([]integ.ValidatedEdge, len(edges))
		for i, e := range edges {
			out[i] = integ.ValidatedEdge{Edge: e, LSPConfirmed: true}
		}
		return out
	}
	impacts, src, _, _, err := analyzeBlastRadiusViaLookup(ctx, lookup, ws, integ.SymbolID("src"), lspProbeFn)
	require.NoError(t, err)
	assert.Equal(t, integ.SourceSemantic, src)
	require.True(t, probeCalled, "lspProbeFn MUST be invoked when non-nil")
	require.Len(t, impacts, 1)
	assert.Equal(t, 1.00, impacts[0].Confidence,
		"lspProbeFn confirmed verdict MUST flip confidence to 1.00 (NOT 0.20 from lookup.ValidateCriticalEdges)")
}

// TestAnalyzeBlastRadiusViaLookup_FallsBackToLookupValidateWhenLspProbeNil
// pins the legacy semantic: when lspProbeFn==nil the orchestrator still
// consults lookup.ValidateCriticalEdges (preserves matrix-fake semantic).
func TestAnalyzeBlastRadiusViaLookup_FallsBackToLookupValidateWhenLspProbeNil(t *testing.T) {
	ctx := context.Background()
	ws := workspace.WorkspaceKey{RepoRoot: "/repo", Language: "go"}

	edgeA := integ.Edge{From: "src", To: "tgt-A", Kind: "calls", Confidence: 0.45}
	pass1 := []integ.Impact{
		{SymbolID: "tgt-A", Confidence: 0.45, Evidence: integ.Evidence{Edges: []integ.Edge{edgeA}}},
	}
	lookup := &fakeLookup{
		available:    true,
		expandResult: pass1,
		validateRes: []integ.ValidatedEdge{
			{Edge: edgeA, LSPConfirmed: true},
		},
	}
	impacts, src, _, _, err := analyzeBlastRadiusViaLookup(ctx, lookup, ws, integ.SymbolID("src"), nil)
	require.NoError(t, err)
	assert.Equal(t, integ.SourceSemantic, src)
	require.Len(t, impacts, 1)
	assert.Equal(t, 1.00, impacts[0].Confidence,
		"nil lspProbeFn MUST fall back to lookup.ValidateCriticalEdges (legacy semantic)")
}
