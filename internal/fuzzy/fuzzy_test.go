package fuzzy

import (
	"strings"
	"testing"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMatch_ExactStrategy(t *testing.T) {
	source := "line 0\nline 1\nline 2\n"
	search := "line 1"
	res, err := Match(source, search, Options{Replacement: "replaced"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, StrategyExact, res.Strategy)
	assert.Equal(t, 1.0, res.Score)
	assert.Equal(t, "line 1", res.MatchedText)
	assert.Equal(t, source[res.StartByte:res.EndByte], res.MatchedText)
}

func TestMatch_WhitespaceStrategy(t *testing.T) {
	source := "foo  \nbar  \n"
	search := "foo\nbar"
	res, err := Match(source, search, Options{Replacement: "x\ny"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, StrategyWhitespace, res.Strategy)
	assert.Equal(t, 0.95, res.Score)
}

func TestMatch_IndentationStrategy(t *testing.T) {
	source := "\tfoo\n\tbar\n"
	search := "    foo\n    bar"
	res, err := Match(source, search, Options{Replacement: "    foo\n    baz"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, StrategyIndentationFlex, res.Strategy)
	assert.Equal(t, 0.85, res.Score)
	assert.Contains(t, res.ReplacementText, "\tfoo")
	assert.Contains(t, res.ReplacementText, "\tbaz")
}

func TestMatch_FailWithDiff(t *testing.T) {
	source := "alpha\nbeta\n"
	search := "completely\nunrelated"
	_, err := Match(source, search, Options{Replacement: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, serr.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "--- search")
	assert.Contains(t, err.Error(), "+++ nearest source region")
}

func TestCascade_StopsOnAmbiguity(t *testing.T) {
	source := "foo\nbar\nfoo\nbar\n"
	search := "foo\nbar"
	_, err := Match(source, search, Options{Replacement: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, serr.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "ambiguous match")
}

func TestMatch_StrategyReporting(t *testing.T) {
	tests := []struct {
		name   string
		source string
		search string
		want   Strategy
	}{
		{"exact", "a\nb\nc\n", "b", StrategyExact},
		{"whitespace", "a  \nb  \n", "a\nb", StrategyWhitespace},
		{"indent-flex", "\tfoo", "    foo", StrategyIndentationFlex},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res, err := Match(tt.source, tt.search, Options{Replacement: "x"})
			require.NoError(t, err)
			assert.Equal(t, tt.want, res.Strategy)
		})
	}
}

func TestMatch_ScoreTiers(t *testing.T) {
	cases := map[Strategy]float64{
		StrategyExact:           1.0,
		StrategyWhitespace:      0.95,
		StrategyIndentationFlex: 0.85,
	}
	inputs := []struct {
		source string
		search string
		want   Strategy
	}{
		{"a\nb\nc\n", "b", StrategyExact},
		{"foo  \n", "foo", StrategyWhitespace},
		{"\tfoo\n", "    foo", StrategyIndentationFlex},
	}
	for _, in := range inputs {
		res, err := Match(in.source, in.search, Options{Replacement: "x"})
		require.NoError(t, err)
		assert.Equal(t, cases[in.want], res.Score)
	}
}

func TestAmbiguity_Exact(t *testing.T) {
	source := "dup\nother\ndup\n"
	_, err := Match(source, "dup", Options{Replacement: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, serr.ErrInvalidArgs)
}

func TestAmbiguity_LineNumberCap(t *testing.T) {
	// 10 copies of "dup" -> "lines 1, 2, 3, 4, 5, ...and 5 more"
	source := ""
	for i := 0; i < 10; i++ {
		source += "dup\n"
	}
	_, err := Match(source, "dup", Options{Replacement: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lines 1, 2, 3, 4, 5, ...and 5 more")
}

func TestAmbiguity_OverflowSuffix(t *testing.T) {
	// Exactly 5 copies -> NO overflow suffix.
	source := "dup\ndup\ndup\ndup\ndup\n"
	_, err := Match(source, "dup", Options{Replacement: "x"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "lines 1, 2, 3, 4, 5)")
	assert.NotContains(t, err.Error(), "...and")
}

func TestAmbiguity_DoesNotCascade(t *testing.T) {
	// Source has 2 exact copies of "foo" -- exact strategy yields 2 hits -> ambiguity.
	// Whitespace strategy would also yield 2 hits, but we must NEVER get that far.
	// We assert the error message mentions strategy=exact via the detail.
	source := "foo\nbar\nfoo\n"
	_, err := Match(source, "foo", Options{Replacement: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, serr.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "strategy=exact")
}

func TestEllipsis_InOrderMatching(t *testing.T) {
	source := "start\nalpha\nmiddle\nbeta\nend\n"
	search := "alpha\n...\nbeta"
	res, err := Match(source, search, Options{Replacement: "alpha\n...\nbeta", AllowEllipsis: true})
	require.NoError(t, err)
	require.NotNil(t, res)
	// Matched region should span from the first segment's start to the last segment's end.
	assert.Contains(t, res.MatchedText, "alpha")
	assert.Contains(t, res.MatchedText, "beta")
}

func TestEllipsis_MixedTierAggregationReportsWeakest(t *testing.T) {
	// Segment 1 ("alpha") matches exactly.
	// Segment 2 ("    beta") only matches under indent-flex against "\tbeta".
	// The aggregated Result must report StrategyIndentationFlex / 0.85 -- NOT exact.
	source := "start\nalpha\nmiddle\n\tbeta\nend\n"
	search := "alpha\n...\n    beta"
	replacement := "alpha\n...\n    beta"
	res, err := Match(source, search, Options{Replacement: replacement, AllowEllipsis: true})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Equal(t, StrategyIndentationFlex, res.Strategy,
		"segmented match must report WEAKEST tier across segments (FUZZ-02)")
	assert.Equal(t, 0.85, res.Score)
}

func TestEllipsis_ByteOffsetsWithTrailingNewline(t *testing.T) {
	// Source ends with a trailing '\n'. The splitLines+Join round-trip
	// used to drift by one byte here; line-index arithmetic must not.
	source := "start\nalpha\nmiddle\nbeta\nend\n"
	search := "alpha\n...\nbeta"
	replacement := "alpha\n...\nbeta"
	res, err := Match(source, search, Options{Replacement: replacement, AllowEllipsis: true})
	require.NoError(t, err)
	require.NotNil(t, res)
	// The matched span in the ORIGINAL source string must start at "alpha"
	// and end at "beta" -- exact byte offsets, no off-by-one.
	assert.Equal(t, strings.Index(source, "alpha"), res.StartByte)
	assert.Equal(t, strings.Index(source, "beta")+len("beta"), res.EndByte,
		"EndByte must land exactly after \"beta\" even with a trailing source newline")
}

func TestMatch_EmptySearch(t *testing.T) {
	_, err := Match("abc", "", Options{Replacement: "x"})
	require.Error(t, err)
	assert.ErrorIs(t, err, serr.ErrInvalidArgs)
	assert.Contains(t, err.Error(), "empty search block")
}

func TestMatch_ReplacementText_Reflow(t *testing.T) {
	source := "    hello\n    world\n"
	search := "    hello\n    world"
	res, err := Match(source, search, Options{Replacement: "    goodbye\n    world"})
	require.NoError(t, err)
	require.NotNil(t, res)
	assert.Contains(t, res.ReplacementText, "    goodbye")
	assert.Contains(t, res.ReplacementText, "    world")
}

func TestMatch_ByteOffsets(t *testing.T) {
	source := "zero\none\ntwo\n"
	res, err := Match(source, "one", Options{Replacement: "x"})
	require.NoError(t, err)
	assert.Equal(t, "one", source[res.StartByte:res.EndByte])
}
