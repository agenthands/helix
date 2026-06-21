package cli

import (
	"reflect"
	"testing"
)

// TestParseLocusLine_FileURIWithCol parses the bare daemon locus form
// "file://<abs>:L:C" (go_to_definition golden) into a workspace-relative tuple
// WITHOUT re-converting coordinates (the daemon already emits 1-based, see
// internal/kernel/symbols/tools.go:189-190).
func TestParseLocusLine_FileURIWithCol(t *testing.T) {
	got, ok := parseLocusLine("file:///ws/main.go:5:6", "/ws", false)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true")
	}
	want := locus{relpath: "main.go", line: 5, col: 6, payload: ""}
	if got != want {
		t.Fatalf("parseLocusLine = %+v, want %+v", got, want)
	}
}

// TestParseLocusLine_FileURIWithPayload parses the "file://…:L:C — payload"
// form (formatLocations with a Name/Preview) and keeps the payload verbatim.
func TestParseLocusLine_FileURIWithPayload(t *testing.T) {
	got, ok := parseLocusLine("file:///ws/main.go:5:6 — Helper [Function]", "/ws", false)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true")
	}
	want := locus{relpath: "main.go", line: 5, col: 6, payload: "Helper [Function]"}
	if got != want {
		t.Fatalf("parseLocusLine = %+v, want %+v", got, want)
	}
}

// TestParseLocusLine_SearchForm parses the fileops "relpath:L: text" form
// (search_in_files, NO column) and defaults col to 1.
func TestParseLocusLine_SearchForm(t *testing.T) {
	got, ok := parseLocusLine("main.go:5: func main() {", "/ws", false)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true")
	}
	want := locus{relpath: "main.go", line: 5, col: 1, payload: "func main() {"}
	if got != want {
		t.Fatalf("parseLocusLine = %+v, want %+v", got, want)
	}
}

// TestParseLocusLine_SearchFormNumericText confirms WR-03: a search match whose
// matched source line is itself purely numeric (daemon form "relpath:L: 42") is
// still parsed as grammar (b) with the number as the payload, NOT misclassified
// as grammar (a) and dropped to passthrough. The two grammars are distinguished
// structurally (colon-space vs adjacent ":C"), not by whether the text is numeric.
func TestParseLocusLine_SearchFormNumericText(t *testing.T) {
	got, ok := parseLocusLine("a.go:10: 42", "", false)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true (all-digits search text)")
	}
	want := locus{relpath: "a.go", line: 10, col: 1, payload: "42"}
	if got != want {
		t.Fatalf("parseLocusLine = %+v, want %+v", got, want)
	}
}

// TestParseLocusLine_ColonLCStillGrammarA confirms the structural discriminator
// still routes a true "<path>:<L>:<C>" line (adjacent ":C", no space) to
// grammar (a), not the search form.
func TestParseLocusLine_ColonLCStillGrammarA(t *testing.T) {
	got, ok := parseLocusLine("a.go:10:6", "", false)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true")
	}
	want := locus{relpath: "a.go", line: 10, col: 6, payload: ""}
	if got != want {
		t.Fatalf("parseLocusLine = %+v, want %+v (grammar a)", got, want)
	}
}

// TestParseLocusLine_Passthrough confirms non-matching lines return ok=false
// (passthrough sentinel — never a panic; T-92-01 DoS mitigation).
func TestParseLocusLine_Passthrough(t *testing.T) {
	cases := []string{
		"(no results)",
		"```go",
		"",
		"just some prose with no locus",
		"file://", // truncated
		"main.go", // no line
	}
	for _, in := range cases {
		if _, ok := parseLocusLine(in, "/ws", false); ok {
			t.Errorf("parseLocusLine(%q) ok=true, want false (passthrough)", in)
		}
	}
}

// TestParseLocusLine_AbsKeepsAbsolute confirms abs=true keeps the absolute path
// (no filepath.Rel applied).
func TestParseLocusLine_AbsKeepsAbsolute(t *testing.T) {
	got, ok := parseLocusLine("file:///ws/sub/main.go:5:6", "/ws", true)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true")
	}
	if got.relpath != "/ws/sub/main.go" {
		t.Fatalf("abs=true relpath = %q, want %q", got.relpath, "/ws/sub/main.go")
	}
}

// TestParseLocusLine_ToSlash confirms path normalization uses filepath.ToSlash
// so a backslash-style absolute path under root yields a forward-slash relpath
// (golden stability across OSes).
func TestParseLocusLine_ToSlash(t *testing.T) {
	// Synthetic backslash input: a relative search-form path with backslashes
	// must normalize to forward slashes for golden stability.
	got, ok := parseLocusLine(`sub\pkg\file.go:3: x := 1`, "/ws", false)
	if !ok {
		t.Fatalf("parseLocusLine returned ok=false, want true")
	}
	if got.relpath != "sub/pkg/file.go" {
		t.Fatalf("ToSlash relpath = %q, want %q", got.relpath, "sub/pkg/file.go")
	}
}

// TestSortDedupLoci_OrderIndependent confirms sortDedupLoci sorts by
// (relpath, line, col) and removes exact duplicates so the same multiset in any
// order yields byte-identical output (OUT-02 determinism).
func TestSortDedupLoci_OrderIndependent(t *testing.T) {
	a := []locus{
		{relpath: "b.go", line: 2, col: 1},
		{relpath: "a.go", line: 10, col: 3},
		{relpath: "a.go", line: 2, col: 5},
		{relpath: "a.go", line: 2, col: 1},
	}
	b := []locus{
		{relpath: "a.go", line: 2, col: 1},
		{relpath: "a.go", line: 2, col: 5},
		{relpath: "a.go", line: 10, col: 3},
		{relpath: "b.go", line: 2, col: 1},
	}
	gotA := sortDedupLoci(a)
	gotB := sortDedupLoci(b)
	if !reflect.DeepEqual(gotA, gotB) {
		t.Fatalf("sortDedupLoci not order-independent:\n a=%+v\n b=%+v", gotA, gotB)
	}
	want := []locus{
		{relpath: "a.go", line: 2, col: 1},
		{relpath: "a.go", line: 2, col: 5},
		{relpath: "a.go", line: 10, col: 3},
		{relpath: "b.go", line: 2, col: 1},
	}
	if !reflect.DeepEqual(gotA, want) {
		t.Fatalf("sortDedupLoci = %+v, want %+v", gotA, want)
	}
}

// TestSortDedupLoci_AllIdentical confirms N identical loci collapse to 1.
func TestSortDedupLoci_AllIdentical(t *testing.T) {
	in := []locus{
		{relpath: "a.go", line: 1, col: 1, payload: "x"},
		{relpath: "a.go", line: 1, col: 1, payload: "x"},
		{relpath: "a.go", line: 1, col: 1, payload: "x"},
	}
	got := sortDedupLoci(in)
	if len(got) != 1 {
		t.Fatalf("sortDedupLoci of 3 identical = %d entries, want 1", len(got))
	}
}
