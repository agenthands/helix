package relatedidx

import (
	"math/rand"
	"reflect"
	"testing"
)

// TestDeterminism_NShuffles strengthens the order-independence proof per the
// red-team B2 finding: a same-input re-run can pass by luck on an order-
// dependent implementation, so instead build the vector from N independent
// random permutations of the same token multiset and assert ALL are byte-
// identical. This falsifies any Go-map-iteration-order / float-sum-non-
// associativity hazard. (Our accumulator is int32, so the guarantee is
// structural, but the test must be able to catch a regression to map/float.)
func TestDeterminism_NShuffles(t *testing.T) {
	base := []string{"ledger", "account", "balance", "transfer", "audit", "posting", "journal", "credit", "debit", "reconcile"}
	want := ComputeContextVector(base)
	rng := rand.New(rand.NewSource(1))
	for n := 0; n < 50; n++ {
		shuf := append([]string(nil), base...)
		rng.Shuffle(len(shuf), func(i, j int) { shuf[i], shuf[j] = shuf[j], shuf[i] })
		if got := ComputeContextVector(shuf); !reflect.DeepEqual(got, want) {
			t.Fatalf("shuffle %d produced a different vector — accumulation is order-dependent", n)
		}
	}
}

// TestDeterminism_OrderIndependent proves the load-bearing determinism
// guarantee: the same token multiset in ANY order yields a byte-identical
// vector. This is also the engine-level proof that RI is STRUCTURE-INDEPENDENT
// — token order is the only "structure" a flat token stream carries, and RI
// discards it entirely. (MinHash, by contrast, fingerprints order via
// trigrams.) Orthogonality to the SIMILAR_TO signal starts here.
func TestDeterminism_OrderIndependent(t *testing.T) {
	a := []string{"user", "name", "get", "id", "fetch", "record", "validate"}
	b := []string{"validate", "id", "get", "record", "name", "fetch", "user"}
	va := ComputeContextVector(a)
	vb := ComputeContextVector(b)
	if !reflect.DeepEqual(va, vb) {
		t.Fatalf("permuted token sets produced different vectors:\n a=%v\n b=%v", va, vb)
	}
	// And a literal re-run is identical (no hidden RNG / map-iteration leak).
	if !reflect.DeepEqual(va, ComputeContextVector(a)) {
		t.Fatal("re-running ComputeContextVector on identical input differed")
	}
}

func TestCosine_Self(t *testing.T) {
	v := ComputeContextVector([]string{"parse", "http", "request", "header", "body", "method"})
	if got := Cosine(&v, &v); got < 0.9999 {
		t.Fatalf("Cosine(v,v) = %f, want ~1.0", got)
	}
}

// TestSharedVocabBeatsDisjoint is the core discrimination property: bodies that
// share vocabulary score strictly higher than bodies with disjoint vocabulary,
// and disjoint vocabulary scores LOW (near zero) — proving the ±1 sign is
// unbiased (no DC offset inflating unrelated pairs).
func TestSharedVocabBeatsDisjoint(t *testing.T) {
	base := []string{"user", "account", "balance", "deposit", "withdraw", "ledger"}
	shared := []string{"user", "account", "balance", "transfer", "ledger", "audit"}    // 4/6 overlap
	disjoint := []string{"pixel", "render", "shader", "vertex", "texture", "viewport"} // 0 overlap

	vBase := ComputeContextVector(base)
	vShared := ComputeContextVector(shared)
	vDisjoint := ComputeContextVector(disjoint)

	simShared := Cosine(&vBase, &vShared)
	simDisjoint := Cosine(&vBase, &vDisjoint)

	if simShared <= simDisjoint {
		t.Fatalf("shared-vocab cosine %.3f not > disjoint-vocab cosine %.3f", simShared, simDisjoint)
	}
	if simShared < 0.4 {
		t.Errorf("shared-vocab cosine %.3f unexpectedly low (4/6 overlap)", simShared)
	}
	if simDisjoint > 0.25 {
		t.Errorf("disjoint-vocab cosine %.3f too high — sign may be biased (DC offset)", simDisjoint)
	}
}

func TestMinTokenGate(t *testing.T) {
	tiny := []string{"x", "y"} // below MinTokens
	if _, ok := VectorFromTokens(tiny); ok {
		t.Errorf("VectorFromTokens(%d tokens) ok=true, want false (below MinTokens=%d)", len(tiny), MinTokens)
	}
	enough := make([]string, MinTokens)
	for i := range enough {
		enough[i] = string(rune('a' + i))
	}
	if _, ok := VectorFromTokens(enough); !ok {
		t.Errorf("VectorFromTokens(%d tokens) ok=false, want true (== MinTokens)", len(enough))
	}
}

func TestComputeVector_Nil(t *testing.T) {
	if v, ok := ComputeVector(nil, nil); ok || v != nil {
		t.Errorf("ComputeVector(nil,nil) = (%v,%v), want (nil,false)", v, ok)
	}
}

// TestAppendSubtokens proves vocabulary extraction keeps identifier TEXT and
// splits it into meaningful subtokens — the inverse of MinHash's
// normalizeKind, which would erase all of this to a single "I".
func TestAppendSubtokens(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"getUserName", []string{"get", "user", "name"}},
		{"HTTPServer", []string{"http", "server"}},
		{"user_id", []string{"user", "id"}},
		{"parseJSON", []string{"parse", "json"}},
		{"snake_case_thing", []string{"snake", "case", "thing"}},
		{"already", []string{"already"}},
	}
	for _, c := range cases {
		var got []string
		appendSubtokens(c.in, &got)
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("appendSubtokens(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestAppendWords(t *testing.T) {
	var got []string
	appendWords("// Parse the HTTP request-body (v2).", &got)
	want := []string{"parse", "the", "http", "request", "body", "v2"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("appendWords = %v, want %v", got, want)
	}
}

// TestVocabularyDrivesSimilarity_NotLength guards against a degenerate engine
// where similarity tracks token COUNT rather than token IDENTITY. Two vectors
// of equal length but disjoint vocabulary must NOT look similar.
func TestVocabularyDrivesSimilarity_NotLength(t *testing.T) {
	a := ComputeContextVector([]string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta"})
	b := ComputeContextVector([]string{"one", "two", "three", "four", "five", "six"})
	if sim := Cosine(&a, &b); sim > 0.25 {
		t.Fatalf("equal-length disjoint-vocab cosine %.3f too high — similarity tracks length not vocabulary", sim)
	}
}

// TestSimHash_Deterministic asserts the SimHash signature is stable across
// calls (hyperplanes are computed once, deterministically).
func TestSimHash_Deterministic(t *testing.T) {
	v := ComputeContextVector([]string{"ledger", "account", "balance", "audit", "posting", "journal"})
	if SimHash(&v) != SimHash(&v) {
		t.Fatal("SimHash not deterministic across calls")
	}
}

// TestSimLSH_RetrievesIdenticalVectors proves the LSH index returns a node
// whose vector is identical (cosine 1.0 → same signature → shares all bands).
// This is the retrieval the emission pass relies on.
func TestSimLSH_RetrievesIdenticalVectors(t *testing.T) {
	v := ComputeContextVector([]string{"ledger", "account", "balance", "audit", "posting", "journal", "credit", "debit"})
	sig := SimHash(&v)
	idx := NewSimLSH()
	idx.Insert(1, sig)
	idx.Insert(2, sig)                // identical signature
	idx.Insert(3, SimHash(&Vector{})) // all-zero, different bucket
	cands := idx.Candidates(1, sig)
	found2 := false
	for _, c := range cands {
		if c == 2 {
			found2 = true
		}
		if c == 1 {
			t.Error("Candidates returned self")
		}
	}
	if !found2 {
		t.Errorf("LSH did not retrieve the identical-signature node; got %v", cands)
	}
}

// TestKeepToken proves the stop-list drops boilerplate/keywords/short/numeric
// tokens but keeps domain vocabulary — the anti-saturation guarantee.
func TestKeepToken(t *testing.T) {
	drop := []string{"err", "ctx", "nil", "return", "if", "for", "func", "value", "result", "i", "x", "42", "a"}
	for _, tok := range drop {
		if keepToken(tok) {
			t.Errorf("keepToken(%q) = true, want false (boilerplate/keyword/trivial)", tok)
		}
	}
	keep := []string{"ledger", "payroll", "withholding", "shader", "vertex", "reconcile"}
	for _, tok := range keep {
		if !keepToken(tok) {
			t.Errorf("keepToken(%q) = false, want true (domain vocabulary)", tok)
		}
	}
}
