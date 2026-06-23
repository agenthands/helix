package repomapeval

import (
	"math"
	"testing"
)

// ndcgK is the cutoff for the headline metric (D-07): nDCG@10.
const ndcgK = 10

// corpusLanguages is the set of languages the hermetic golden scores. It mirrors
// the committed testdata/{gold,captured} files (py/go/rust — D-03).
var corpusLanguages = []string{"go", "python", "rust"}

// meanForwardRevRnd loads every exercise across all corpus languages and returns
// the mean nDCG@10 for the forward (repo_map) ranking, the reversed ranker, and
// the seeded-random ranker — the aggregate the discriminator margin bites on (so
// the margin holds on the CORPUS, not a single cherry-picked exercise).
func meanForwardRevRnd(t *testing.T) (fwd, rev, rnd float64, n int) {
	t.Helper()
	var sumF, sumR, sumD float64
	for _, lang := range corpusLanguages {
		gold, err := LoadGold(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadGold(%q): %v", lang, err)
		}
		captured, err := LoadCaptured(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadCaptured(%q): %v", lang, err)
		}
		for ex := range gold.Exercises {
			gset := gold.GoldSet(ex)
			ce, ok := captured.Exercises[ex]
			if !ok {
				t.Fatalf("%s/%s: no captured ranking", lang, ex)
			}
			forward := ce.RepoMap
			sumF += NDCGAtK(forward, gset, ndcgK)
			sumR += NDCGAtK(reversedRanker(forward), gset, ndcgK)
			sumD += NDCGAtK(seededRandomRanker(forward, discriminatorSeed, discriminatorSeed2), gset, ndcgK)
			n++
		}
	}
	if n == 0 {
		t.Fatal("corpus is empty — the discriminator would vacuously pass")
	}
	return sumF / float64(n), sumR / float64(n), sumD / float64(n), n
}

// TestDiscriminator (anti-vacuity, D-09): on the real authored gold corpus, the
// forward ranking's mean nDCG@10 beats max(reversed, seeded-random) by the
// committed DiscriminatorMargin, AND both adversarial rankers individually score
// STRICTLY BELOW forward (the discriminator BITES — a green-path-only gate is
// presumed broken, v1.12 CRITICAL).
func TestDiscriminator(t *testing.T) {
	fwd, rev, rnd, n := meanForwardRevRnd(t)
	t.Logf("corpus n=%d  mean nDCG@10: forward=%.4f reversed=%.4f random=%.4f (margin=%.2f)",
		n, fwd, rev, rnd, DiscriminatorMargin)

	worst := math.Max(rev, rnd)
	if fwd-worst < DiscriminatorMargin {
		t.Errorf("forward nDCG@10 (%.4f) - max(reversed=%.4f, random=%.4f) = %.4f < margin %.2f",
			fwd, rev, rnd, fwd-worst, DiscriminatorMargin)
	}
	// The discriminator must BITE: both adversarial rankers individually fail.
	if rev >= fwd {
		t.Errorf("reversed ranker (%.4f) did not score below forward (%.4f) — discriminator is vacuous", rev, fwd)
	}
	if rnd >= fwd {
		t.Errorf("seeded-random ranker (%.4f) did not score below forward (%.4f) — discriminator is vacuous", rnd, fwd)
	}
}

// TestScoreCorpus (hermetic golden): score the committed forward captured ranking
// vs committed gold across ALL py/go/rust exercises, producing recall@10 / MRR /
// nDCG@10 / budget-fit with NO binary and NO network. This is the authoritative
// go test ./bench/... proof. It asserts the corpus is non-degenerate: forward
// metrics are healthy (the front-loaded gold ⇒ high recall/nDCG), and every
// metric stays within [0,1].
func TestScoreCorpus(t *testing.T) {
	total := 0
	var sumRecall, sumMRR, sumNDCG, sumBudget float64
	for _, lang := range corpusLanguages {
		gold, err := LoadGold(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadGold(%q): %v", lang, err)
		}
		captured, err := LoadCaptured(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadCaptured(%q): %v", lang, err)
		}
		for ex := range gold.Exercises {
			gset := gold.GoldSet(ex)
			ce := captured.Exercises[ex]
			forward := ce.RepoMap

			recall := RecallAtK(forward, gset, ndcgK)
			mrr := MRR(forward, gset)
			ndcg := NDCGAtK(forward, gset, ndcgK)
			budget := BudgetFitRatio(forward, gset)

			for name, v := range map[string]float64{"recall": recall, "mrr": mrr, "ndcg": ndcg, "budget": budget} {
				if v < 0.0-eps || v > 1.0+eps {
					t.Errorf("%s/%s: %s = %v out of [0,1]", lang, ex, name, v)
				}
			}
			// Forward MRR must be 1.0: the top-ranked ID is always a gold symbol
			// in the committed forward rankings (gold is front-loaded).
			if math.Abs(mrr-1.0) > eps {
				t.Errorf("%s/%s: forward MRR = %v, want 1.0 (gold front-loaded)", lang, ex, mrr)
			}
			sumRecall += recall
			sumMRR += mrr
			sumNDCG += ndcg
			sumBudget += budget
			total++
		}
	}
	if total == 0 {
		t.Fatal("scored zero exercises")
	}
	t.Logf("hermetic golden (n=%d): recall@10=%.4f MRR=%.4f nDCG@10=%.4f budget-fit=%.4f",
		total, sumRecall/float64(total), sumMRR/float64(total),
		sumNDCG/float64(total), sumBudget/float64(total))

	// The committed corpus is healthy: mean forward nDCG@10 is high (front-loaded
	// gold). A regression that scrambled gold or captured would tank this.
	if meanNDCG := sumNDCG / float64(total); meanNDCG < 0.9 {
		t.Errorf("mean forward nDCG@10 = %.4f < 0.9 — committed corpus is degenerate", meanNDCG)
	}
}
