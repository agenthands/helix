package fuzzy

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommonLeadingPrefix(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  string
	}{
		{"4 spaces", []string{"    a", "    b"}, "    "},
		{"1 tab", []string{"\tfoo", "\tbar"}, "\t"},
		{"empty line ignored", []string{"  a", "", "  b"}, "  "},
		{"no common prefix", []string{"foo", "bar"}, ""},
		{"mixed tab/space falls back to shortest shared", []string{"\t a", " \t a"}, ""},
		{"all empty", []string{"", ""}, ""},
		{"single non-empty", []string{"    x"}, "    "},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, commonLeadingPrefix(tt.lines))
		})
	}
}

func TestDedent(t *testing.T) {
	lines := []string{"    foo", "    bar", "", "    baz"}
	out := dedent(lines, "    ")
	assert.Equal(t, []string{"foo", "bar", "", "baz"}, out)
}

func TestReapplyPrefix_PreservesEmptyLines(t *testing.T) {
	lines := []string{"foo", "", "bar"}
	out := reapplyPrefix(lines, "\t\t")
	assert.Equal(t, []string{"\t\tfoo", "", "\t\tbar"}, out,
		"empty lines must stay empty — no trailing whitespace")
}

func TestReflow_AppliesSourcePrefix(t *testing.T) {
	// Search block indented with 4 spaces; replacement same indent.
	// Source region indented with 8 spaces. Expected: replacement gets 8-space indent.
	searchLines := []string{"    if x {", "        doThing()", "    }"}
	matchedRegion := []string{"        if x {", "            doThing()", "        }"}
	replacement := "    if x {\n        doOther()\n    }"
	out := reflow(searchLines, matchedRegion, replacement)
	assert.Contains(t, out, "        if x {")
	assert.Contains(t, out, "            doOther()")
	assert.Contains(t, out, "        }")
}

func TestReflow_TabsAndSpaces(t *testing.T) {
	// Search uses spaces; source uses tabs.
	searchLines := []string{"    foo", "    bar"}
	matchedRegion := []string{"\tfoo", "\tbar"}
	replacement := "    foo\n    baz"
	out := reflow(searchLines, matchedRegion, replacement)
	assert.Contains(t, out, "\tfoo")
	assert.Contains(t, out, "\tbaz")
	assert.NotContains(t, out, "    baz", "result should use source tabs, not search spaces")
}

func TestReflow_PreservesEmptyLines(t *testing.T) {
	searchLines := []string{"    a", "", "    b"}
	matchedRegion := []string{"        a", "", "        b"}
	replacement := "    a\n\n    c"
	out := reflow(searchLines, matchedRegion, replacement)
	lines := splitLines(out)
	// The blank line must remain exactly "" (zero bytes), not "        ".
	foundBlank := false
	for _, ln := range lines {
		if ln == "" {
			foundBlank = true
			break
		}
	}
	assert.True(t, foundBlank, "empty replacement line must survive reflow as \"\"")
}
