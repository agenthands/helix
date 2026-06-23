package fuzzyrobust

import (
	"testing"
)

// corpusLanguages mirrors the committed testdata/drift/*.json (D-03: py/go/rust).
var corpusLanguages = []string{"go", "python", "rust"}

// testdataDir is the committed corpus root relative to the package dir.
const testdataDir = "testdata"

// perStrategyFloor is the documented per-strategy-per-language minimum (D-04:
// ~4 cases per strategy per language).
const perStrategyFloor = 4

// singleSiteTiers are the four cascade tiers a single-site drift case can carry.
var singleSiteTiers = []string{
	StrategyExact, StrategyWhitespace, StrategyIndentationFlex, StrategyEllipsis,
}

// TestCorpusFloor: each of go/python/rust meets the documented per-strategy
// floor (~4 cases/strategy/lang) AND the corpus carries >=1 duplicate-block
// ambiguous case. It FAILS (RED) if any language is below floor or the ambiguous
// case is absent.
func TestCorpusFloor(t *testing.T) {
	totalAmbiguous := 0
	for _, lang := range corpusLanguages {
		corpus, err := LoadDrift(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadDrift(%q): %v", lang, err)
		}
		perStrategy := map[string]int{}
		ambiguous := 0
		for _, c := range corpus.Cases {
			if c.Ambiguous {
				ambiguous++
				totalAmbiguous++
				if c.ExpectedStrategy != OutcomeAmbiguous {
					t.Errorf("%s case %q is flagged Ambiguous but ExpectedStrategy=%q, want %q",
						lang, c.ID, c.ExpectedStrategy, OutcomeAmbiguous)
				}
				continue
			}
			perStrategy[c.ExpectedStrategy]++
		}
		for _, tier := range singleSiteTiers {
			if perStrategy[tier] < perStrategyFloor {
				t.Errorf("%s: strategy %q has %d cases, want >= %d (floor)",
					lang, tier, perStrategy[tier], perStrategyFloor)
			}
		}
		if ambiguous < 1 {
			t.Errorf("%s: no duplicate-block ambiguous case (want >= 1)", lang)
		}
	}
	if totalAmbiguous < 1 {
		t.Fatal("corpus has zero ambiguous cases across all languages (anti-vacuity floor unmet)")
	}
}

// TestLoadRejectsTraversal: LoadDrift / LoadCaptured reject a malicious language
// path segment BEFORE filepath.Join (T-102-05, V5 control).
func TestLoadRejectsTraversal(t *testing.T) {
	bad := []string{"..", "../secrets", "a/b", `a\b`, "", ".hidden"}
	for _, lang := range bad {
		if _, err := LoadDrift(testdataDir, lang); err == nil {
			t.Errorf("LoadDrift accepted malicious language %q (want error)", lang)
		}
		if _, err := LoadCaptured(testdataDir, lang); err == nil {
			t.Errorf("LoadCaptured accepted malicious language %q (want error)", lang)
		}
	}
}
