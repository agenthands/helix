package fuzzy

import (
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEllipsis_TwoSegments(t *testing.T) {
	segments, err := segmentSearch("a\n...\nb")
	require.NoError(t, err)
	require.Len(t, segments, 2)
	assert.Contains(t, segments[0], "a")
	assert.Contains(t, segments[1], "b")
}

func TestEllipsis_MultipleSegments(t *testing.T) {
	segments, err := segmentSearch("first\n...\nsecond\n...\nthird")
	require.NoError(t, err)
	assert.Len(t, segments, 3)
}

func TestEllipsis_LeadingWhitespace(t *testing.T) {
	segments, err := segmentSearch("a\n   ...\nb")
	require.NoError(t, err)
	assert.Len(t, segments, 2)
}

func TestEllipsis_InlineLiteral(t *testing.T) {
	segments, err := segmentSearch("func f(args ...int) {\n}")
	require.NoError(t, err)
	assert.Nil(t, segments, "inline ... must be treated as literal, not a marker")
}

func TestEllipsis_EmptySegmentRejected(t *testing.T) {
	cases := []struct {
		name   string
		search string
	}{
		{"empty leading", "...\nb"},
		{"empty trailing", "a\n..."},
		{"empty middle", "a\n...\n...\nb"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			_, err := segmentSearch(tt.search)
			require.Error(t, err)
			assert.ErrorIs(t, err, serr.ErrInvalidArgs)
		})
	}
}

func TestEllipsis_ReplacementSegmentMismatch(t *testing.T) {
	err := validateSegmentCounts(2, 3)
	require.Error(t, err)
	assert.ErrorIs(t, err, serr.ErrInvalidArgs)
}

func TestEllipsis_NoMarkerReturnsNil(t *testing.T) {
	segments, err := segmentSearch("plain block with no marker")
	require.NoError(t, err)
	assert.Nil(t, segments)
}
