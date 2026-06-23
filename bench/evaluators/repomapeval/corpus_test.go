package repomapeval

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// testdataDir is the committed corpus root for the hermetic golden tests.
const testdataDir = "testdata"

// fixturesRoot is the vendored aider-polyglot fixture tree, relative to this
// package dir. TestGoldParseable cross-checks every gold ID's relpath against
// the exercise's .meta/config.json files.solution (the ground-truth source the
// gold was authored from — D-02).
const fixturesRoot = "../../datasets/aider-polyglot/fixtures"

// languageFloors is the documented per-language size floor (CORPUS.md): the
// count of vendored exercises Phase 99 shipped. A corpus below floor fails.
var languageFloors = map[string]int{
	"go":     9,
	"python": 9,
	"rust":   10,
}

// TestCorpusFloor: every language's gold corpus meets its documented floor.
func TestCorpusFloor(t *testing.T) {
	for lang, floor := range languageFloors {
		gold, err := LoadGold(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadGold(%q): %v", lang, err)
		}
		if len(gold.Exercises) < floor {
			t.Errorf("gold corpus %q has %d exercises, want >= %d (floor)",
				lang, len(gold.Exercises), floor)
		}
		// A floor corpus with empty gold sets would saturate every metric — guard it.
		for ex, ids := range gold.Exercises {
			if len(ids) == 0 {
				t.Errorf("gold corpus %q exercise %q has zero gold IDs", lang, ex)
			}
		}
	}
}

// solutionRelpaths reads dir/.meta/config.json and returns the files.solution
// relpaths for the exercise (the ground-truth keys gold IDs must match).
func solutionRelpaths(t *testing.T, exerciseDir string) map[string]bool {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(exerciseDir, ".meta", "config.json"))
	if err != nil {
		t.Fatalf("read config.json %s: %v", exerciseDir, err)
	}
	var cfg struct {
		Files struct {
			Solution []string `json:"solution"`
		} `json:"files"`
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("parse config.json %s: %v", exerciseDir, err)
	}
	set := make(map[string]bool, len(cfg.Files.Solution))
	for _, rel := range cfg.Files.Solution {
		set[rel] = true
	}
	return set
}

// TestGoldParseable: every gold ID is relpath:symbol-well-formed AND its relpath
// matches a files.solution entry of that exercise (Pitfall 3 guard).
func TestGoldParseable(t *testing.T) {
	for lang := range languageFloors {
		gold, err := LoadGold(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadGold(%q): %v", lang, err)
		}
		for ex, ids := range gold.Exercises {
			exerciseDir := filepath.Join(fixturesRoot, lang, "exercises", "practice", ex)
			sols := solutionRelpaths(t, exerciseDir)
			for _, id := range ids {
				relpath, symbol, ok := SplitID(id)
				if !ok {
					t.Errorf("%s/%s: gold ID %q is not relpath:symbol-well-formed", lang, ex, id)
					continue
				}
				if relpath == "" || symbol == "" {
					t.Errorf("%s/%s: gold ID %q has empty relpath or symbol", lang, ex, id)
				}
				if !sols[relpath] {
					t.Errorf("%s/%s: gold ID %q relpath %q is not a files.solution entry %v",
						lang, ex, id, relpath, sols)
				}
			}
		}
	}
}

// TestCapturedAligned: every gold exercise has a captured entry with a non-empty
// repo_map AND context ranking (a missing/empty captured entry fails).
func TestCapturedAligned(t *testing.T) {
	for lang := range languageFloors {
		gold, err := LoadGold(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadGold(%q): %v", lang, err)
		}
		captured, err := LoadCaptured(testdataDir, lang)
		if err != nil {
			t.Fatalf("LoadCaptured(%q): %v", lang, err)
		}
		for ex := range gold.Exercises {
			ce, ok := captured.Exercises[ex]
			if !ok {
				t.Errorf("%s: gold exercise %q has no captured ranking", lang, ex)
				continue
			}
			if len(ce.RepoMap) == 0 {
				t.Errorf("%s/%s: captured repo_map ranking is empty", lang, ex)
			}
			if len(ce.Context) == 0 {
				t.Errorf("%s/%s: captured context ranking is empty", lang, ex)
			}
		}
	}
}

// TestLoadRejectsTraversal: LoadGold/LoadCaptured reject a malicious language
// path segment BEFORE any filepath.Join (T-102-01, V5 control).
func TestLoadRejectsTraversal(t *testing.T) {
	bad := []string{"..", "../python", "go/..", `a\b`, "", ".hidden"}
	for _, lang := range bad {
		if _, err := LoadGold(testdataDir, lang); err == nil {
			t.Errorf("LoadGold(%q) accepted a malicious path segment", lang)
		}
		if _, err := LoadCaptured(testdataDir, lang); err == nil {
			t.Errorf("LoadCaptured(%q) accepted a malicious path segment", lang)
		}
	}
}
