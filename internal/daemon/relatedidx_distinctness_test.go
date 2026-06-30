package daemon

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/extract"
	goextract "github.com/agenthands/helix/internal/semantic/extract/golang"
	"github.com/agenthands/helix/internal/semantic/minhash"
	"github.com/agenthands/helix/internal/semantic/relatedidx"
	"github.com/agenthands/helix/internal/treesitter"
)

// firstFingerprinted returns the (MinHash, ContextVec) of the first
// fingerprinted function/method symbol in ef.
func firstFingerprinted(ef *extract.ExtractedFile) (*minhash.Signature, *relatedidx.Vector) {
	for _, sf := range ef.Symbols {
		if sf.MinHash != nil || sf.ContextVec != nil {
			return sf.MinHash, sf.ContextVec
		}
	}
	return nil, nil
}

// TestRelatedness_DistinctAndSelective is the milestone's anti-vacuity guard.
// It proves on REAL parsed Go bodies that the vocabulary signal (relatedidx
// cosine, → SEMANTICALLY_RELATED) and the structural signal (MinHash Jaccard, →
// SIMILAR_TO) are ORTHOGONAL, and that the vocabulary edge is SELECTIVE (does
// not fire on unrelated bodies). Without this, SEMANTICALLY_RELATED could
// silently collapse into either a duplicate of SIMILAR_TO or a near-complete
// graph dominated by boilerplate vocabulary — both vacuous.
//
// Mutation check (manual): the guard bites because it asserts a 2x2. If the RI
// engine regressed to fingerprinting STRUCTURE (i.e. became MinHash), PAIR2
// (different structure, shared vocab) would drop below the cosine threshold and
// this test would FAIL. If the stop-list were removed (boilerplate dominates),
// PAIR3 (unrelated) cosine would rise toward the threshold and the selectivity
// assertion would FAIL. Verified RED both ways during development.
func TestRelatedness_DistinctAndSelective(t *testing.T) {
	grammars := treesitter.NewGrammarRegistry()
	p := goextract.NewProvider(grammars)

	mk := func(name, src string) (*minhash.Signature, *relatedidx.Vector) {
		ef, err := p.Extract(t.Context(), []byte(src), extract.SourceFile{Path: name, Language: "go"})
		if err != nil {
			t.Fatalf("Extract(%s): %v", name, err)
		}
		s, v := firstFingerprinted(ef)
		if v == nil {
			t.Fatalf("%s: no ContextVec — body too small or wiring broken", name)
		}
		return s, v
	}

	// PAIR 1: byte-identical STRUCTURE, fully DISJOINT vocabulary.
	s1a, v1a := mk("payroll.go", `package a
func ComputePayrollTax(grossSalary int, taxBracket int) int {
	taxableIncome := grossSalary - 12000
	bracketRate := taxBracket * 2
	withholding := taxableIncome * bracketRate
	netPayroll := grossSalary - withholding
	return netPayroll
}`)
	s1b, v1b := mk("shader.go", `package b
func RenderSceneShader(vertexBuffer int, shaderProgram int) int {
	clipSpace := vertexBuffer - 12000
	fragmentDepth := shaderProgram * 2
	rasterPixels := clipSpace * fragmentDepth
	framebuffer := vertexBuffer - rasterPixels
	return framebuffer
}`)

	// PAIR 2: DIFFERENT structure (loop+branch vs straight-line), SHARED
	// domain vocabulary (payroll / salary / withholding / bracket).
	s2a, v2a := mk("withhold.go", `package c
func ApplyPayrollWithholding(grossSalary int, taxBracket int) int {
	taxableIncome := grossSalary - 12000
	withholding := taxableIncome * taxBracket
	return grossSalary - withholding
}`)
	s2b, v2b := mk("schedule.go", `package d
func ComputePayrollSchedule(grossSalary int, taxBracket int, periods int) int {
	total := 0
	for period := 0; period < periods; period++ {
		taxableIncome := grossSalary - 12000
		if taxBracket > 0 {
			withholding := taxableIncome * taxBracket
			total = total + (grossSalary - withholding)
		}
	}
	return total
}`)

	// PAIR 3: clearly UNRELATED, but BOTH SATURATED WITH BOILERPLATE
	// (ctx/err/nil/return/result/value, identical control flow). This is the
	// fixture that makes the selectivity assertion non-vacuous: with the
	// stop-list these score ~0.14 (no edge); WITHOUT it the shared plumbing
	// vocabulary dominates and they score ~0.69 (> 0.55, a spurious edge).
	// Measured + mutation-confirmed 2026-06-30. A domain-rich PAIR3 would NOT
	// catch a removed stop-list — the boilerplate density is the whole point.
	_, v3a := mk("loadinvoice.go", `package e
func LoadInvoice(ctx context.Context, invoiceID string) error {
	result, err := fetchInvoice(ctx, invoiceID)
	if err != nil { return err }
	if result == nil { return nil }
	value := result.amount
	if value < 0 { return err }
	return nil
}`)
	_, v3b := mk("rendertexture.go", `package f
func RenderTexture(ctx context.Context, textureID string) error {
	result, err := loadTexture(ctx, textureID)
	if err != nil { return err }
	if result == nil { return nil }
	value := result.pixels
	if value < 0 { return err }
	return nil
}`)

	jac1 := minhash.Jaccard(s1a, s1b)
	cos1 := relatedidx.Cosine(v1a, v1b)
	jac2 := minhash.Jaccard(s2a, s2b)
	cos2 := relatedidx.Cosine(v2a, v2b)
	cos3 := relatedidx.Cosine(v3a, v3b)
	const thr = 0.55 // == semanticRelatedThreshold

	t.Logf("PAIR1 same-struct/disjoint-vocab: Jaccard=%.3f Cosine=%.3f", jac1, cos1)
	t.Logf("PAIR2 diff-struct/shared-vocab : Jaccard=%.3f Cosine=%.3f", jac2, cos2)
	t.Logf("PAIR3 unrelated                : Cosine=%.3f", cos3)

	// --- Distinctness (the 2x2) ---
	// PAIR1: structurally similar (would be SIMILAR_TO) but NOT vocabulary-related.
	if jac1 < minhash.JaccardThreshold {
		t.Errorf("PAIR1 Jaccard %.3f < %.3f: expected structural near-clone (SIMILAR_TO)", jac1, minhash.JaccardThreshold)
	}
	if cos1 >= thr {
		t.Errorf("PAIR1 Cosine %.3f >= %.3f: disjoint vocabulary must NOT be SEMANTICALLY_RELATED (engine regressed toward structure?)", cos1, thr)
	}
	// PAIR2: vocabulary-related (SEMANTICALLY_RELATED) but NOT structurally similar.
	if cos2 < thr {
		t.Errorf("PAIR2 Cosine %.3f < %.3f: shared-domain vocabulary must be SEMANTICALLY_RELATED", cos2, thr)
	}
	if jac2 >= minhash.JaccardThreshold {
		t.Errorf("PAIR2 Jaccard %.3f >= %.3f: different structure must NOT be SIMILAR_TO", jac2, minhash.JaccardThreshold)
	}

	// --- Selectivity (anti-saturation) ---
	if cos3 >= thr {
		t.Errorf("PAIR3 Cosine %.3f >= %.3f: unrelated bodies linked — boilerplate saturation (stop-list ineffective)", cos3, thr)
	}
}
