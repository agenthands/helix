package graph

import (
	"cmp"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"testing"
)

// digestScores produces a deterministic hex sha256 of the score vector.
// Serialization: lines of "<node>=<%.17g>\n" in sorted node order.
// %.17g preserves float64 round-trip bits so the digest is stable across
// runs as long as the engine itself is stable.
func digestScores[T cmp.Ordered](scores map[T]float64) string {
	keys := make([]T, 0, len(scores))
	for k := range scores {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	var b strings.Builder
	for _, k := range keys {
		fmt.Fprintf(&b, "%v=%.17g\n", k, scores[k])
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])
}

// readGolden loads a single-line golden hex digest, trimming whitespace.
func readGolden(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile("testdata/pagerank/" + name)
	if err != nil {
		t.Fatalf("read golden %s: %v", name, err)
	}
	return strings.TrimSpace(string(data))
}

// fixtureUniform is the canonical 5-node weighted graph used for
// determinism + uniform-teleport hex-digest tests.
//
// Topology:
//
//	a -> b (1.0), a -> c (0.5)
//	b -> c (1.0), b -> d (0.25)
//	c -> a (0.75), c -> d (0.5)
//	d -> e (1.0)
//	e -> a (0.5)
func fixtureUniform() (nodes []string, edges map[string]map[string]float64) {
	nodes = []string{"e", "b", "d", "a", "c"} // deliberately UNSORTED to prove engine sorts
	edges = map[string]map[string]float64{
		"a": {"b": 1.0, "c": 0.5},
		"b": {"c": 1.0, "d": 0.25},
		"c": {"a": 0.75, "d": 0.5},
		"d": {"e": 1.0},
		"e": {"a": 0.5},
	}
	return nodes, edges
}

func defaultOpts() Options {
	return Options{Damping: 0.85, Epsilon: 1e-6, MaxIter: 100}
}

// TestPageRank_Deterministic runs PageRank multiple times over a fixed
// graph and asserts the output map is identical every run. Combined with
// `-count=10` at the test runner level this is a 10×N determinism gate.
func TestPageRank_Deterministic(t *testing.T) {
	nodes, edges := fixtureUniform()
	first := PageRank(nodes, edges, defaultOpts())
	for i := 0; i < 10; i++ {
		got := PageRank(nodes, edges, defaultOpts())
		if len(got) != len(first) {
			t.Fatalf("run %d: len mismatch: got %d, want %d", i, len(got), len(first))
		}
		for k, v := range first {
			gv, ok := got[k]
			if !ok {
				t.Fatalf("run %d: missing key %v", i, k)
			}
			if gv != v {
				t.Fatalf("run %d: key %v: got %.17g want %.17g", i, k, gv, v)
			}
		}
	}
}

// TestPageRank_HexDigest pins the byte-equality of the score vector against
// a checked-in golden digest. This is the GRAPH-01 hard gate.
func TestPageRank_HexDigest(t *testing.T) {
	nodes, edges := fixtureUniform()
	scores := PageRank(nodes, edges, defaultOpts())
	got := digestScores(scores)
	want := readGolden(t, "golden_uniform.txt")
	if want == "PLACEHOLDER" {
		t.Logf("PLACEHOLDER digest detected; computed digest is %s", got)
		t.Fatalf("golden_uniform.txt is a PLACEHOLDER; replace with real digest")
	}
	if got != want {
		t.Fatalf("uniform digest mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

// TestPageRank_Personalized verifies Personalize boosts seeded nodes and
// that the resulting digest matches a pinned golden.
func TestPageRank_Personalized(t *testing.T) {
	nodes, edges := fixtureUniform()
	personal := map[any]float64{"a": 1.0, "c": 1.0}
	scores := PageRank(nodes, edges, Options{
		Damping: 0.85, Epsilon: 1e-6, MaxIter: 100,
		Personalize: personal,
	})
	uniform := PageRank(nodes, edges, defaultOpts())

	// Personalized scores at "a" and "c" must beat their uniform-teleport baselines.
	if !(scores["a"] > uniform["a"]) {
		t.Errorf("personalize: scores[a]=%.6g should exceed uniform[a]=%.6g", scores["a"], uniform["a"])
	}
	if !(scores["c"] > uniform["c"]) {
		t.Errorf("personalize: scores[c]=%.6g should exceed uniform[c]=%.6g", scores["c"], uniform["c"])
	}

	// RankNodes must produce a sorted-desc result.
	ranked := RankNodes(nodes, edges, Options{
		Damping: 0.85, Epsilon: 1e-6, MaxIter: 100,
		Personalize: personal,
	})
	if len(ranked) != len(nodes) {
		t.Fatalf("ranked len: got %d want %d", len(ranked), len(nodes))
	}
	for i := 1; i < len(ranked); i++ {
		if ranked[i-1].Score < ranked[i].Score {
			t.Errorf("ranked not desc at %d: %v=%.6g < %v=%.6g",
				i, ranked[i-1].Node, ranked[i-1].Score, ranked[i].Node, ranked[i].Score)
		}
	}

	got := digestScores(scores)
	want := readGolden(t, "golden_personalized.txt")
	if want == "PLACEHOLDER" {
		t.Logf("PLACEHOLDER digest detected; computed digest is %s", got)
		t.Fatalf("golden_personalized.txt is a PLACEHOLDER; replace with real digest")
	}
	if got != want {
		t.Fatalf("personalized digest mismatch:\n  got:  %s\n  want: %s", got, want)
	}
}

// fixtureRing is a fully symmetric ring graph (4 nodes, equal weights).
// All scores converge to 1/n; RankNodes must order by node-id ascending
// (= GRAPH-03 stable-key tiebreak).
func fixtureRing() (nodes []string, edges map[string]map[string]float64) {
	nodes = []string{"d", "b", "a", "c"} // unsorted on purpose
	edges = map[string]map[string]float64{
		"a": {"b": 1.0},
		"b": {"c": 1.0},
		"c": {"d": 1.0},
		"d": {"a": 1.0},
	}
	return nodes, edges
}

func TestPageRank_TieBreak(t *testing.T) {
	nodes, edges := fixtureRing()
	scores := PageRank(nodes, edges, defaultOpts())

	// All four scores should be equal (or within float wobble).
	want := 1.0 / 4.0
	for _, k := range []string{"a", "b", "c", "d"} {
		if math.Abs(scores[k]-want) > 1e-9 {
			t.Errorf("ring node %s: got %.6g want ~%.6g", k, scores[k], want)
		}
	}

	ranked := RankNodes(nodes, edges, defaultOpts())
	wantOrder := []string{"a", "b", "c", "d"}
	for i, r := range ranked {
		if r.Node != wantOrder[i] {
			t.Errorf("tiebreak position %d: got %s want %s (full=%v)", i, r.Node, wantOrder[i], ranked)
		}
	}

	got := digestScores(scores)
	wantDigest := readGolden(t, "golden_tiebreak.txt")
	if wantDigest == "PLACEHOLDER" {
		t.Logf("PLACEHOLDER digest detected; computed digest is %s", got)
		t.Fatalf("golden_tiebreak.txt is a PLACEHOLDER; replace with real digest")
	}
	if got != wantDigest {
		t.Fatalf("tiebreak digest mismatch:\n  got:  %s\n  want: %s", got, wantDigest)
	}
}

func TestPageRank_Empty(t *testing.T) {
	got := PageRank[string](nil, nil, defaultOpts())
	if got != nil {
		t.Errorf("nil nodes: got %v, want nil", got)
	}

	got = PageRank([]string{}, nil, defaultOpts())
	if got != nil {
		t.Errorf("empty nodes: got %v, want nil", got)
	}

	single := PageRank([]string{"only"}, nil, defaultOpts())
	if len(single) != 1 {
		t.Fatalf("single-node len: got %d want 1", len(single))
	}
	if math.Abs(single["only"]-1.0) > 1e-9 {
		t.Errorf("single-node score: got %.6g want 1.0", single["only"])
	}
}

func TestPageRank_Dangling(t *testing.T) {
	// "x" is dangling (no outgoing edges). a -> b is the only edge.
	nodes := []string{"a", "b", "x"}
	edges := map[string]map[string]float64{
		"a": {"b": 1.0},
	}
	scores := PageRank(nodes, edges, defaultOpts())
	if len(scores) != 3 {
		t.Fatalf("dangling len: got %d want 3", len(scores))
	}
	total := 0.0
	for _, s := range scores {
		if math.IsNaN(s) || math.IsInf(s, 0) {
			t.Fatalf("dangling: NaN/Inf in scores: %v", scores)
		}
		if s <= 0 {
			t.Errorf("dangling: non-positive score: %v", scores)
		}
		total += s
	}
	if math.Abs(total-1.0) > 1e-3 {
		t.Errorf("dangling: scores sum %.6g should be ~1.0", total)
	}
}
