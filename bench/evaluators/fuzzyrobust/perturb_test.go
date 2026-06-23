package fuzzyrobust

import (
	"strings"
	"testing"
)

// stripWS removes all whitespace so two blocks can be compared for
// non-whitespace-content equality.
func stripWS(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case ' ', '\t', '\n', '\r':
		default:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// stripLeadWS removes leading whitespace from every line (the dimension the
// indentation tier normalizes), leaving internal+trailing whitespace and
// content intact.
func stripLeadWS(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		lines[i] = strings.TrimLeft(l, " \t")
	}
	return strings.Join(lines, "\n")
}

const sampleBlock = "func Answer(q string) int {\n\tw := fields(q)\n\treturn len(w)\n}"

// TestPerturbWhitespace: the whitespace transform changes ONLY leading/trailing
// whitespace (non-whitespace content identical), and is byte-reproducible.
func TestPerturbWhitespace(t *testing.T) {
	got := PerturbWhitespace(sampleBlock)
	if got == sampleBlock {
		t.Fatal("PerturbWhitespace did not change the block")
	}
	if stripWS(got) != stripWS(sampleBlock) {
		t.Fatalf("PerturbWhitespace altered non-whitespace content:\n got=%q\nwant=%q", stripWS(got), stripWS(sampleBlock))
	}
	if PerturbWhitespace(sampleBlock) != got {
		t.Fatal("PerturbWhitespace is not byte-reproducible")
	}
}

// TestPerturbIndent: the indent transform changes ONLY leading indentation
// (content after the indent + internal whitespace identical), and is
// byte-reproducible.
func TestPerturbIndent(t *testing.T) {
	got := PerturbIndent(sampleBlock)
	if got == sampleBlock {
		t.Fatal("PerturbIndent did not change the block")
	}
	if stripLeadWS(got) != stripLeadWS(sampleBlock) {
		t.Fatalf("PerturbIndent altered content past the indentation:\n got=%q\nwant=%q", stripLeadWS(got), stripLeadWS(sampleBlock))
	}
	if PerturbIndent(sampleBlock) != got {
		t.Fatal("PerturbIndent is not byte-reproducible")
	}
}

// TestPerturbEllipsis: the ellipsis transform replaces middle line(s) with a
// "..."-on-own-line placeholder, preserving head + tail anchors;
// byte-reproducible.
func TestPerturbEllipsis(t *testing.T) {
	got := PerturbEllipsis(sampleBlock)
	lines := strings.Split(got, "\n")
	if len(lines) != 3 {
		t.Fatalf("PerturbEllipsis on a 4-line block: got %d lines, want 3 (head/.../tail)", len(lines))
	}
	if lines[1] != "..." {
		t.Fatalf("PerturbEllipsis middle line = %q, want %q", lines[1], "...")
	}
	srcLines := strings.Split(sampleBlock, "\n")
	if lines[0] != srcLines[0] || lines[2] != srcLines[len(srcLines)-1] {
		t.Fatalf("PerturbEllipsis did not preserve head/tail anchors")
	}
	if PerturbEllipsis(sampleBlock) != got {
		t.Fatal("PerturbEllipsis is not byte-reproducible")
	}
}

// TestPerturbExact: the identity transform yields the unchanged block.
func TestPerturbExact(t *testing.T) {
	if PerturbExact(sampleBlock) != sampleBlock {
		t.Fatal("PerturbExact is not the identity")
	}
}
