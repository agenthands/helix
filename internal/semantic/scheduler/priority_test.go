package scheduler_test

import (
	"testing"

	"github.com/agenthands/helix/internal/semantic/scheduler"
)

// TestBuildPriorityQueue_FourTierOrdering tests the 4-tier priority assignment
// by popping every file and asserting tier order. Two files are promoted to
// tier 1 (inflight) so we exercise the deterministic tie-break too.
func TestBuildPriorityQueue_FourTierOrdering(t *testing.T) {
	files := []string{
		"a/lib.go",       // first-class go (tier 3)
		"b/util.ts",      // first-class typescript (tier 3) — promoted to inflight
		"c/important.go", // first-class go (tier 3) — promoted to repomap
		"d/README.md",    // non-first-class (tier 4)
		"e/inflight.py",  // first-class python (tier 3) — promoted to inflight
	}
	inflight := map[string]struct{}{
		"b/util.ts":     {},
		"e/inflight.py": {},
	}
	repomap := map[string]struct{}{
		"c/important.go": {},
	}

	pq := scheduler.BuildPriorityQueue(files, inflight, repomap)
	popped := drainQueue(pq)

	if len(popped) != len(files) {
		t.Fatalf("expected %d items, got %d", len(files), len(popped))
	}

	// Tier 1 (inflight) — sorted-path tie-break: b/util.ts < e/inflight.py.
	if popped[0].Path != "b/util.ts" {
		t.Fatalf("tier 1 first should be b/util.ts, got %q", popped[0].Path)
	}
	if popped[0].Priority != scheduler.PriorityInflightToolReferenced {
		t.Fatalf("expected priority Inflight, got %d", popped[0].Priority)
	}
	if popped[1].Path != "e/inflight.py" {
		t.Fatalf("tier 1 second should be e/inflight.py, got %q", popped[1].Path)
	}
	// Tier 2 (repomap): c/important.go.
	if popped[2].Path != "c/important.go" {
		t.Fatalf("tier 2 (repomap) should be 3rd, got %q", popped[2].Path)
	}
	if popped[2].Priority != scheduler.PriorityRepomapImportant {
		t.Fatalf("expected priority RepomapImportant, got %d", popped[2].Priority)
	}
	// Tier 3 (first-class remaining): a/lib.go.
	if popped[3].Path != "a/lib.go" {
		t.Fatalf("tier 3 (first-class) should be 4th, got %q", popped[3].Path)
	}
	if popped[3].Priority != scheduler.PriorityFirstClassSource {
		t.Fatalf("expected priority FirstClassSource, got %d", popped[3].Priority)
	}
	// Tier 4 (other): d/README.md.
	if popped[4].Path != "d/README.md" {
		t.Fatalf("tier 4 (other) should be last, got %q", popped[4].Path)
	}
	if popped[4].Priority != scheduler.PriorityNonFirstClass {
		t.Fatalf("expected priority NonFirstClass, got %d", popped[4].Priority)
	}
}

// TestBuildPriorityQueue_DeterministicTieBreak verifies same-priority entries
// pop in sorted-path order. All files are first-class go => all tier 3 =>
// pop order is purely lexical.
func TestBuildPriorityQueue_DeterministicTieBreak(t *testing.T) {
	files := []string{"z.go", "a.go", "m.go"}
	pq := scheduler.BuildPriorityQueue(files, nil, nil)
	popped := drainQueue(pq)
	want := []string{"a.go", "m.go", "z.go"}
	for i, p := range popped {
		if p.Path != want[i] {
			t.Fatalf("tie-break order: position %d expected %q, got %q", i, want[i], p.Path)
		}
	}
}

// TestBuildPriorityQueue_EmptyInputs verifies safe handling of nil sets and
// empty file slice.
func TestBuildPriorityQueue_EmptyInputs(t *testing.T) {
	pq := scheduler.BuildPriorityQueue(nil, nil, nil)
	if pq.Len() != 0 {
		t.Fatalf("empty input should yield Len()=0, got %d", pq.Len())
	}
	pq2 := scheduler.BuildPriorityQueue([]string{"a.go"}, nil, nil)
	if pq2.Len() != 1 {
		t.Fatalf("single-input should yield Len()=1, got %d", pq2.Len())
	}
}

// TestClassifyByExtension_BoundedLabels enforces the closed enum: only
// {"go", "typescript", "python", "other"} are returned (matches the bounded
// allowlist registered in internal/obs/metrics.go for the extraction metric).
func TestClassifyByExtension_BoundedLabels(t *testing.T) {
	cases := map[string]string{
		"main.go":    "go",
		"app.ts":     "typescript",
		"app.tsx":    "typescript",
		"app.js":     "typescript",
		"app.jsx":    "typescript",
		"app.mjs":    "typescript",
		"app.cjs":    "typescript",
		"main.py":    "python",
		"README.md":  "other",
		"":           "other",
		"Makefile":   "other",
		"weird.GO":   "go",         // case-insensitive
		"upper.PY":   "python",     // case-insensitive
		"shouty.TSX": "typescript", // case-insensitive
	}
	allowed := map[string]struct{}{"go": {}, "typescript": {}, "python": {}, "other": {}}
	for path, want := range cases {
		got := scheduler.ClassifyByExtension(path)
		if got != want {
			t.Errorf("ClassifyByExtension(%q) = %q, want %q", path, got, want)
		}
		if _, ok := allowed[got]; !ok {
			t.Errorf("ClassifyByExtension(%q) returned out-of-allowlist label %q", path, got)
		}
	}
}

// drainQueue pops every entry from the priority queue in priority order.
func drainQueue(pq *scheduler.PriorityQueue) []scheduler.PoppedFile {
	var out []scheduler.PoppedFile
	for pq.Len() > 0 {
		out = append(out, pq.PopFile())
	}
	return out
}
