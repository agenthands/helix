package daemon

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/classifier"
	"github.com/agenthands/helix/internal/semantic/extract"
	"github.com/agenthands/helix/internal/semantic/minhash"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
)

func TestLastNameSegment(t *testing.T) {
	cases := []struct{ in, want string }{
		{"io.Reader", "Reader"},
		{"pkg::Base", "Base"},
		{`App\Models\User`, "User"},
		{"a/b/c", "c"},
		{"Plain", "Plain"},
		{"", ""},
		{"trailing.", "trailing."}, // trailing separator → no segment, return whole
	}
	for _, c := range cases {
		if got := lastNameSegment(c.in); got != c.want {
			t.Errorf("lastNameSegment(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// resolvePendingDst must fill DstNodeID + confidence when a candidate name
// resolves, leave it untouched when none resolve, and never produce a
// self-edge.
func TestResolvePendingDst(t *testing.T) {
	idx := map[string]uint64{"Base": 10, "Helper": 20}

	edges := []semanticstore.EdgeFact{
		{SrcNodeID: 1, DstNodeID: 0, EdgeKind: "EXTENDS", Confidence: 0.20, Weight: 0.5},    // resolves to Base
		{SrcNodeID: 2, DstNodeID: 0, EdgeKind: "IMPLEMENTS", Confidence: 0.20, Weight: 0.5}, // external, stays 0
		{SrcNodeID: 10, DstNodeID: 0, EdgeKind: "EXTENDS", Confidence: 0.20, Weight: 0.5},   // self-edge rejected
	}
	pending := []pendingDstResolve{
		{edgeIdx: 0, candidates: []string{"pkg.Base", "Base"}, srcNodeID: 1, confidence: 0.70, weight: 0.6},
		{edgeIdx: 1, candidates: []string{"io.Reader", "Reader"}, srcNodeID: 2, confidence: 0.70, weight: 0.6},
		{edgeIdx: 2, candidates: []string{"Base"}, srcNodeID: 10, confidence: 0.70, weight: 0.6},
	}
	resolvePendingDst(edges, pending, idx)

	if edges[0].DstNodeID != 10 || edges[0].Confidence != 0.70 || edges[0].Weight != 0.6 {
		t.Errorf("edge0 = dst %d conf %.2f w %.2f, want dst 10 conf 0.70 w 0.60",
			edges[0].DstNodeID, edges[0].Confidence, edges[0].Weight)
	}
	if edges[1].DstNodeID != 0 || edges[1].Confidence != 0.20 {
		t.Errorf("edge1 (external) = dst %d conf %.2f, want dst 0 conf 0.20 (unresolved)",
			edges[1].DstNodeID, edges[1].Confidence)
	}
	if edges[2].DstNodeID != 0 {
		t.Errorf("edge2 = dst %d, want 0 (self-edge must be rejected)", edges[2].DstNodeID)
	}
}

// identicalSig fills all K slots with the same value v.
func identicalSig(v uint64) *minhash.Signature {
	s := &minhash.Signature{}
	for i := range s.Values {
		s.Values[i] = v
	}
	return s
}

// similarToEdges must link two identical signatures and not link a disjoint
// one. Edge is emitted once with lower NodeID as source.
func TestSimilarToEdges_LinksClones(t *testing.T) {
	a := identicalSig(0xAAAA)
	bClone := identicalSig(0xAAAA) // Jaccard(a,b)=1.0 ≥ threshold
	cDiff := identicalSig(0xBBBB)  // Jaccard(a,c)=0.0

	nodes := []fingerprintedNode{
		{nodeID: 100, kind: "function", language: "go", sig: a},
		{nodeID: 200, kind: "function", language: "go", sig: bClone},
		{nodeID: 300, kind: "function", language: "go", sig: cDiff},
	}
	edges := similarToEdges(nodes)

	var simCount int
	var got *semanticstore.EdgeFact
	for i := range edges {
		if edges[i].EdgeKind == "SIMILAR_TO" {
			simCount++
			got = &edges[i]
		}
	}
	if simCount != 1 {
		t.Fatalf("got %d SIMILAR_TO edges, want exactly 1 (100↔200 clone pair)", simCount)
	}
	if got.SrcNodeID != 100 || got.DstNodeID != 200 {
		t.Errorf("SIMILAR_TO %d→%d, want 100→200 (lower id = src)", got.SrcNodeID, got.DstNodeID)
	}
	if got.Source != "minhash" {
		t.Errorf("SIMILAR_TO source = %q, want minhash", got.Source)
	}
}

// similarToEdges must skip nodes without a signature and not crash on <2 sigs.
func TestSimilarToEdges_NilAndSingleton(t *testing.T) {
	if got := similarToEdges(nil); got != nil {
		t.Errorf("nil input → %d edges, want 0", len(got))
	}
	one := []fingerprintedNode{{nodeID: 1, sig: identicalSig(1)}, {nodeID: 2, sig: nil}}
	if got := similarToEdges(one); got != nil {
		t.Errorf("one-signature input → %d edges, want 0", len(got))
	}
}

// structuralTwinEdges must link two identical structural profiles and not link a
// structurally different one.
func TestStructuralTwinEdges_LinksSameShape(t *testing.T) {
	// Two near-identical profiles, one clearly different.
	pA := &classifier.ASTProfile{}
	pA[0], pA[8], pA[10], pA[24] = 2, 12, 4, 30 // if-depth, stmt, call, length
	pB := &classifier.ASTProfile{}
	pB[0], pB[8], pB[10], pB[24] = 2, 12, 4, 30 // identical → cosine 1.0
	pC := &classifier.ASTProfile{}
	pC[1], pC[15] = 9, 40 // disjoint dims → cosine 0 vs A/B

	nodes := []fingerprintedNode{
		{nodeID: 100, kind: "function", language: "go", profile: pA},
		{nodeID: 200, kind: "function", language: "go", profile: pB},
		{nodeID: 300, kind: "function", language: "go", profile: pC},
	}
	edges := structuralTwinEdges(nodes)

	var stCount int
	var got *semanticstore.EdgeFact
	for i := range edges {
		if edges[i].EdgeKind == "STRUCTURAL_TWIN" {
			stCount++
			got = &edges[i]
		}
	}
	if stCount != 1 {
		t.Fatalf("got %d STRUCTURAL_TWIN edges, want exactly 1 (100↔200 same-shape pair)", stCount)
	}
	if got.SrcNodeID != 100 || got.DstNodeID != 200 {
		t.Errorf("STRUCTURAL_TWIN %d→%d, want 100→200 (lower id = src)", got.SrcNodeID, got.DstNodeID)
	}
	if got.Source != "ast_profile" {
		t.Errorf("STRUCTURAL_TWIN source = %q, want ast_profile", got.Source)
	}
}

// structuralTwinEdges must not cross language boundaries even with identical
// profiles (the bucket key is language-scoped).
func TestStructuralTwinEdges_LanguageScoped(t *testing.T) {
	p := &classifier.ASTProfile{}
	p[0], p[8], p[10], p[24] = 1, 5, 2, 20

	nodes := []fingerprintedNode{
		{nodeID: 1, kind: "function", language: "go", profile: p},
		{nodeID: 2, kind: "function", language: "python", profile: p},
	}
	if got := structuralTwinEdges(nodes); len(got) != 0 {
		t.Errorf("cross-language identical profiles → %d STRUCTURAL_TWIN edges, want 0", len(got))
	}
}

// structuralTwinEdges must skip all-zero profiles (no structural signal).
func TestStructuralTwinEdges_SkipsZeroProfile(t *testing.T) {
	zero := &classifier.ASTProfile{}
	nodes := []fingerprintedNode{
		{nodeID: 1, kind: "function", language: "go", profile: zero},
		{nodeID: 2, kind: "function", language: "go", profile: zero},
	}
	if got := structuralTwinEdges(nodes); len(got) != 0 {
		t.Errorf("all-zero profiles → %d STRUCTURAL_TWIN edges, want 0", len(got))
	}
}

// importTargetCandidates: named imports yield candidates (+ last segment);
// bare module imports yield nil so the IMPORTS edge stays module-level.
func TestImportTargetCandidates(t *testing.T) {
	bare := extract.ImportFact{Language: "go", Source: "fmt"}
	if got := importTargetCandidates(bare); got != nil {
		t.Errorf("bare module import → %v, want nil", got)
	}
	named := extract.ImportFact{Language: "php", Source: `App\Models`, Symbols: []string{`App\Models\User`}}
	got := importTargetCandidates(named)
	want := []string{`App\Models\User`, "User"}
	if len(got) != len(want) {
		t.Fatalf("named import candidates = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("candidate[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestProfileCosineAndNorm(t *testing.T) {
	p := &classifier.ASTProfile{}
	p[0], p[1] = 3, 4 // norm = 5
	if n := profileNorm(p); n != 5 {
		t.Errorf("profileNorm = %v, want 5", n)
	}
	// cosine with itself is 1.
	if c := profileCosine(p, p, 5, 5); c < 0.9999 || c > 1.0001 {
		t.Errorf("profileCosine(p,p) = %v, want ~1.0", c)
	}
}
