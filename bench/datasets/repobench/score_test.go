package repobench

import (
	"context"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/bench/evaluators/editsim"
	"github.com/agenthands/helix/bench/evaluators/exactmatch"
)

// TestAccAtK_HitMissBoundary proves the RepoBench-R retrieval metric: AccAtK is
// true iff the gold snippet index appears within the first k of the ranked
// retrieval ordering. It covers hit-at-k, miss-beyond-k, and the k>=len(ranked)
// boundary.
func TestAccAtK_HitMissBoundary(t *testing.T) {
	ranked := []int{3, 1, 0, 2} // retrieval order: candidate 3 ranked first, etc.

	// gold=1 is at rank position 2 (1-indexed): hit at k>=2, miss at k=1.
	if AccAtK(ranked, 1, 1) {
		t.Error("AccAtK(ranked, gold=1, k=1) = true, want false (gold is the 2nd ranked)")
	}
	if !AccAtK(ranked, 1, 2) {
		t.Error("AccAtK(ranked, gold=1, k=2) = false, want true (gold within top-2)")
	}
	if !AccAtK(ranked, 1, 3) {
		t.Error("AccAtK(ranked, gold=1, k=3) = false, want true")
	}

	// gold=2 is at rank position 4 (last): miss for k<4, hit at k>=4.
	if AccAtK(ranked, 2, 3) {
		t.Error("AccAtK(ranked, gold=2, k=3) = true, want false (gold is the 4th ranked)")
	}
	if !AccAtK(ranked, 2, 4) {
		t.Error("AccAtK(ranked, gold=2, k=4) = false, want true")
	}

	// k >= len(ranked) boundary: clamps to len(ranked); any present gold is a hit.
	if !AccAtK(ranked, 2, 99) {
		t.Error("AccAtK(ranked, gold=2, k=99) = false, want true (k clamps to len)")
	}
	// gold absent from ranked: never a hit, even at large k.
	if AccAtK(ranked, 7, 99) {
		t.Error("AccAtK(ranked, gold=7, k=99) = true, want false (gold not in ranked)")
	}
	// k <= 0 is a vacuous top-0: never a hit.
	if AccAtK(ranked, 3, 0) {
		t.Error("AccAtK(ranked, gold=3, k=0) = true, want false (empty top-k)")
	}
}

// TestCompletionScore_DelegatesToPlan01 proves -C/-P scoring reuses the Plan 01
// exactmatch + editsim scorers: an exact completion is EM-true with ES==1.0; a
// wrong completion is EM-false with ES matching editsim.ES exactly (no local
// re-implementation of edit distance).
func TestCompletionScore_DelegatesToPlan01(t *testing.T) {
	gold := "return json.load(f)"

	em, es := CompletionScore(gold, gold)
	if !em {
		t.Error("CompletionScore(gold, gold) em = false, want true")
	}
	if es != 1.0 {
		t.Errorf("CompletionScore(gold, gold) es = %v, want 1.0", es)
	}

	pred := "return json.loads(f)" // off by two chars
	em, es = CompletionScore(pred, gold)
	if em {
		t.Error("CompletionScore(near-miss, gold) em = true, want false")
	}
	// The scorer MUST delegate to editsim.ES — assert byte-for-byte equality with
	// the Plan 01 scorer (guards against any local Levenshtein re-implementation).
	if want := editsim.ES(pred, gold); es != want {
		t.Errorf("CompletionScore es = %v, want editsim.ES = %v (must delegate to Plan 01)", es, want)
	}
	if want := exactmatch.EM(pred, gold); em != want {
		t.Errorf("CompletionScore em = %v, want exactmatch.EM = %v (must delegate to Plan 01)", em, want)
	}
}

// TestCompletionScore_ReferenceValue asserts the EM/ES on a known RepoBench-C
// reference pair equals the published EM/ES metric the Plan 01 scorers compute.
// RepoBench-C reports EM (exact next_line match) and ES (normalized edit
// similarity, the same CM-ES family as CrossCodeEval; RepoBench paper Liu et al.
// ICLR 2024). For a perfect prediction EM=true and ES=1.0; for a one-token
// substitution the ES is 1 - editdistance/maxlen over runes. This is the
// hermetic reference; the live "metrics match published reference on a sampled
// subset" run is the HELIX_BENCH_NETWORK-gated complement, never the sole proof.
func TestCompletionScore_ReferenceValue(t *testing.T) {
	// Drive the reference pair off the committed python completion fixture so the
	// value is paper/dataset-card sourced (provenance lives in the fixture).
	raw, err := os.ReadFile(filepath.Join("testdata", "sample.parquet"))
	if err != nil {
		t.Fatalf("read sample.parquet: %v", err)
	}
	tasks, err := LoadParquetBytes(context.Background(), raw, "python")
	if err != nil {
		t.Fatalf("LoadParquetBytes(python): %v", err)
	}
	comp := findTask(tasks, TaskCompletion)
	if comp == nil {
		t.Fatal("no python completion task decoded")
	}
	gold := comp.NextLine // "return json.load(f)"

	// Perfect prediction: published RepoBench-C reference for a correct line is
	// EM=true, ES=1.0.
	if em, es := CompletionScore(gold, gold); !em || es != 1.0 {
		t.Errorf("CompletionScore(gold, gold) = (%v, %v), want (true, 1.0)", em, es)
	}

	// A single-character deletion (drop the trailing ')'): EM=false; ES is the
	// normalized edit similarity. gold has 19 runes; one deletion -> distance 1 ->
	// ES = 1 - 1/19.
	pred := "return json.load(f"
	em, es := CompletionScore(pred, gold)
	if em {
		t.Error("CompletionScore(one-char-short, gold) em = true, want false")
	}
	want := 1.0 - 1.0/float64(len([]rune(gold)))
	if math.Abs(es-want) > 1e-12 {
		t.Errorf("CompletionScore es = %v, want %v (1 - 1/%d)", es, want, len([]rune(gold)))
	}
}
