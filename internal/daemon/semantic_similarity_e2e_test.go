package daemon

import (
	"context"
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	semanticstore "github.com/agenthands/helix/internal/semantic/store"
	"github.com/agenthands/helix/internal/treesitter"
)

// goExtract parses src as a Go file at path through the real tree-sitter
// provider, returning the ExtractedFile (with MinHash / ASTProfile
// fingerprints stamped on function/method symbols).
func goExtract(t *testing.T, p extract.Provider, path, src string) *extract.ExtractedFile {
	t.Helper()
	ef, err := p.Extract(context.Background(), []byte(src), extract.SourceFile{Path: path, Language: "go"})
	if err != nil {
		t.Fatalf("Extract(%s): %v", path, err)
	}
	return ef
}

// TestFactsFromExtracted_E2E_SimilarAndStructuralTwin drives the real Go
// provider end-to-end: two files each holding a structurally-identical function
// body (≥ minhash.MinNodes leaf tokens) must produce a SIMILAR_TO edge
// (identical MinHash) AND a STRUCTURAL_TWIN edge (identical ASTProfile) between
// the two function symbols, with real resolved NodeIDs. (STRUCTURAL_TWIN is the
// renamed structural-shape edge — formerly the misnamed "DATA_FLOWS"; the
// DATA_FLOWS kind is now reserved for true interprocedural flow.)
func TestFactsFromExtracted_E2E_SimilarAndStructuralTwin(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)

	// Two distinct function NAMES with byte-identical BODIES. MinHash
	// normalizes identifiers to "I", so the differing names do not change
	// the signature — the bodies fingerprint identically. The body is large
	// enough to clear minhash.MinNodes (30 leaf tokens).
	body := `{
	total := 0
	for i := 0; i < 10; i++ {
		total = total + i*2
		if total > 5 {
			total = total - 1
		}
	}
	result := total * 3
	return result
}`
	srcA := "package a\n\nfunc Compute() int " + body + "\n"
	srcB := "package b\n\nfunc ComputeClone() int " + body + "\n"

	efA := goExtract(t, p, "a/compute.go", srcA)
	efB := goExtract(t, p, "b/compute_clone.go", srcB)

	// Sanity: the provider must have fingerprinted the two functions.
	if !hasFingerprintedFunc(efA) {
		t.Fatal("provider did not fingerprint Compute (MinHash/Profile nil) — body too small or wiring broken")
	}
	if !hasFingerprintedFunc(efB) {
		t.Fatal("provider did not fingerprint ComputeClone")
	}

	got := factsFromExtracted([]*extract.ExtractedFile{efA, efB}, "r", "", nil, nil)

	sim := firstEdge(got.Edges, "SIMILAR_TO")
	if sim == nil {
		t.Fatal("no SIMILAR_TO edge — identical clones must link")
	}
	if sim.SrcNodeID == 0 || sim.DstNodeID == 0 || sim.SrcNodeID == sim.DstNodeID {
		t.Errorf("SIMILAR_TO endpoints invalid: %d→%d", sim.SrcNodeID, sim.DstNodeID)
	}

	st := firstEdge(got.Edges, "STRUCTURAL_TWIN")
	if st == nil {
		t.Fatal("no STRUCTURAL_TWIN edge — identical structural profiles must link")
	}
	if st.SrcNodeID == 0 || st.DstNodeID == 0 || st.SrcNodeID == st.DstNodeID {
		t.Errorf("STRUCTURAL_TWIN endpoints invalid: %d→%d", st.SrcNodeID, st.DstNodeID)
	}
}

// TestFactsFromExtracted_E2E_HeritageResolves drives the real Go provider:
// an embedded struct field (Go heritage) whose target type is defined in
// another file of the same batch must resolve the IMPLEMENTS edge's
// DstNodeID to that type's node (not stay 0), at the raised confidence.
func TestFactsFromExtracted_E2E_HeritageResolves(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)

	base := goExtract(t, p, "base.go", "package m\n\ntype Base struct {\n\tX int\n}\n")
	derived := goExtract(t, p, "derived.go", "package m\n\ntype Derived struct {\n\tBase\n\tY int\n}\n")

	// Provider must have captured the embed heritage on Derived.
	if len(derived.Heritage) == 0 {
		t.Fatal("provider captured no heritage for embedded Base — query/wiring broken")
	}

	got := factsFromExtracted([]*extract.ExtractedFile{base, derived}, "r", "", nil, nil)

	// Find the IMPLEMENTS edge whose target resolved to Base's node.
	var baseNode uint64
	for i := range got.Symbols {
		if got.Symbols[i].Name == "Base" {
			baseNode = got.Symbols[i].NodeID
		}
	}
	if baseNode == 0 {
		t.Fatal("Base symbol missing a NodeID in the batch")
	}

	impl := firstEdge(got.Edges, "IMPLEMENTS")
	if impl == nil {
		t.Fatal("no IMPLEMENTS edge for embedded Base")
	}
	if impl.DstNodeID != baseNode {
		t.Errorf("IMPLEMENTS DstNodeID = %d, want Base node %d (resolved via batch name index)", impl.DstNodeID, baseNode)
	}
	if impl.Confidence != 0.70 {
		t.Errorf("resolved IMPLEMENTS confidence = %.2f, want 0.70 (raised on resolution)", impl.Confidence)
	}
}

func hasFingerprintedFunc(ef *extract.ExtractedFile) bool {
	for _, s := range ef.Symbols {
		if extract.IsFingerprintableKind(s.Kind) && (s.MinHash != nil || s.Profile != nil) {
			return true
		}
	}
	return false
}

func firstEdge(edges []semanticstore.EdgeFact, kind string) *semanticstore.EdgeFact {
	for i := range edges {
		if edges[i].EdgeKind == kind {
			return &edges[i]
		}
	}
	return nil
}

// TestFactsFromExtracted_E2E_SemanticallyRelated drives the real Go provider
// end-to-end: two functions sharing a rich domain vocabulary must produce a
// SEMANTICALLY_RELATED edge between their symbols, with real resolved NodeIDs.
// Bodies are vocabulary-identical (same kept token multiset) so the RI context
// vectors are identical (cosine 1.0) and LSH retrieval is guaranteed —
// determinism here is structural, not probabilistic. (The distinctness of
// SEMANTICALLY_RELATED from SIMILAR_TO is proven separately in
// TestRelatedness_DistinctAndSelective on diff-structure bodies.)
func TestFactsFromExtracted_E2E_SemanticallyRelated(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)

	body := `{
	taxableIncome := grossSalary - exemption
	bracketRate := taxBracket * multiplier
	withholding := taxableIncome * bracketRate
	netPayroll := grossSalary - withholding
	reconciledLedger := netPayroll + accrualAdjustment
	return reconciledLedger
}`
	srcA := "package a\n\nfunc ComputePayroll(grossSalary, taxBracket, exemption, multiplier, accrualAdjustment int) int " + body + "\n"
	srcB := "package b\n\nfunc ComputeWages(grossSalary, taxBracket, exemption, multiplier, accrualAdjustment int) int " + body + "\n"

	efA := goExtract(t, p, "a/payroll.go", srcA)
	efB := goExtract(t, p, "b/wages.go", srcB)

	if efA.Symbols[0].ContextVec == nil || efB.Symbols[0].ContextVec == nil {
		t.Fatal("provider did not compute ContextVec — body vocabulary too small or wiring broken")
	}

	got := factsFromExtracted([]*extract.ExtractedFile{efA, efB}, "r", "", nil, nil)

	rel := firstEdge(got.Edges, "SEMANTICALLY_RELATED")
	if rel == nil {
		t.Fatal("no SEMANTICALLY_RELATED edge — shared-vocabulary functions must link")
	}
	if rel.SrcNodeID == 0 || rel.DstNodeID == 0 || rel.SrcNodeID == rel.DstNodeID {
		t.Errorf("SEMANTICALLY_RELATED endpoints invalid: %d→%d", rel.SrcNodeID, rel.DstNodeID)
	}
	if rel.Source != "random_index" {
		t.Errorf("SEMANTICALLY_RELATED Source = %q, want random_index", rel.Source)
	}
}
