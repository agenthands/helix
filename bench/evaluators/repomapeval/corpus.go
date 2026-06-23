package repomapeval

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// GoldCorpus is the committed, ground-truth symbol-level gold for one language.
// It maps an exercise name (e.g. "wordy") to the set of gold "file:symbol" IDs
// that the solution defines (authored ONCE from each exercise's .meta/example.*
// reference solution + .meta/config.json files.solution — D-02, NEVER derived
// from get-repo-map output). The map representation makes a gold lookup O(1) for
// the metric scorers; iteration order is never relied on (the metrics sort or
// index, never range-and-emit).
type GoldCorpus struct {
	// Language is the per-track language the corpus was loaded for.
	Language string
	// Exercises maps exercise name -> ordered gold file:symbol IDs (source-order
	// for readability; the metrics treat them as a set via GoldSet).
	Exercises map[string][]string
}

// GoldSet returns the gold IDs for one exercise as a membership set, the shape
// the recall@k / MRR / nDCG@k scorers consume. A missing exercise yields an
// empty (non-nil) set so callers never nil-deref.
func (g GoldCorpus) GoldSet(exercise string) map[string]bool {
	set := make(map[string]bool, len(g.Exercises[exercise]))
	for _, id := range g.Exercises[exercise] {
		set[id] = true
	}
	return set
}

// CapturedRanking is the committed, pre-parsed ordered ranking for one language.
// It maps an exercise name to BOTH the uniform get-repo-map ordering (RepoMap)
// and the seeded get-context ordering (Context), each an ordered slice of
// "file:symbol" IDs. The captured artifact is regenerable HELIX_BIN-gated by the
// //go:build ignore regenerator (bench/runtime/repomap_eval_capture_regen.go);
// the leaf only ever reads the committed JSON (committed-vs-committed scoring).
type CapturedRanking struct {
	// Language is the per-track language the ranking was loaded for.
	Language string
	// Exercises maps exercise name -> its captured rankings.
	Exercises map[string]CapturedExercise
}

// CapturedExercise carries the two ordered rankings captured for one exercise.
type CapturedExercise struct {
	// RepoMap is the uniform-PageRank get-repo-map ordering.
	RepoMap []string `json:"repo_map"`
	// Context is the personalized-PageRank get-context ordering (seeded from the
	// exercise's files.solution stub path).
	Context []string `json:"context"`
}

// validatePathSegment rejects a name that could escape a join root once it
// becomes a path segment. It is a clone of
// bench/datasets/aider-polyglot/loader.go:81 (validatePathSegment) kept LOCAL to
// this stdlib-only leaf so the leaf carries NO bench/datasets import (the
// stdlib-only boundary TestLeafImports enforces). It rejects "", anything
// filepath.Clean rewrites ("..", "a//b"), any embedded separator, and a leading
// dot. MUST run BEFORE any filepath.Join (T-102-01, V5 control).
func validatePathSegment(name, kind string) error {
	if name == "" {
		return fmt.Errorf("repomapeval: %s is empty", kind)
	}
	if name != filepath.Clean(name) || strings.ContainsAny(name, `/\`) || strings.HasPrefix(name, ".") {
		return fmt.Errorf("repomapeval: %s %q contains path separators, parent refs, or a leading dot", kind, name)
	}
	return nil
}

// LoadGold reads dir/gold/<language>.json into a GoldCorpus. The language is
// validated as a path segment BEFORE any filepath.Join (T-102-01) so a traversal
// segment is rejected up front. No network, no kernel import: stdlib only.
func LoadGold(dir, language string) (GoldCorpus, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return GoldCorpus{}, err
	}
	path := filepath.Join(dir, "gold", language+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return GoldCorpus{}, fmt.Errorf("repomapeval: read gold %s: %w", path, err)
	}
	var exercises map[string][]string
	if err := json.Unmarshal(raw, &exercises); err != nil {
		return GoldCorpus{}, fmt.Errorf("repomapeval: parse gold %s: %w", path, err)
	}
	if len(exercises) == 0 {
		return GoldCorpus{}, fmt.Errorf("repomapeval: gold %s is empty (fail-closed)", path)
	}
	return GoldCorpus{Language: language, Exercises: exercises}, nil
}

// LoadCaptured reads dir/captured/<language>.json into a CapturedRanking. The
// language is validated as a path segment BEFORE any filepath.Join (T-102-01).
// No network, no kernel import: stdlib only.
func LoadCaptured(dir, language string) (CapturedRanking, error) {
	if err := validatePathSegment(language, "language"); err != nil {
		return CapturedRanking{}, err
	}
	path := filepath.Join(dir, "captured", language+".json")
	raw, err := os.ReadFile(path)
	if err != nil {
		return CapturedRanking{}, fmt.Errorf("repomapeval: read captured %s: %w", path, err)
	}
	var exercises map[string]CapturedExercise
	if err := json.Unmarshal(raw, &exercises); err != nil {
		return CapturedRanking{}, fmt.Errorf("repomapeval: parse captured %s: %w", path, err)
	}
	if len(exercises) == 0 {
		return CapturedRanking{}, fmt.Errorf("repomapeval: captured %s is empty (fail-closed)", path)
	}
	return CapturedRanking{Language: language, Exercises: exercises}, nil
}

// SplitID splits a "relpath:symbol" gold/ranking ID into its relpath and symbol
// parts. The split is on the LAST colon so a relpath containing a colon (rare,
// but possible on some path shapes) keeps the symbol as the trailing segment.
// ok is false when the ID is not well-formed (no colon, empty relpath, or empty
// symbol). It is total on every input and never panics.
func SplitID(id string) (relpath, symbol string, ok bool) {
	i := strings.LastIndex(id, ":")
	if i <= 0 || i == len(id)-1 {
		return "", "", false
	}
	return id[:i], id[i+1:], true
}
