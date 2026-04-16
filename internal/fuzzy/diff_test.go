package fuzzy

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFormatAmbiguity(t *testing.T) {
	tests := []struct {
		name string
		hits []int
		want string
	}{
		{
			name: "2 hits",
			hits: []int{10, 20},
			want: "ambiguous match: search block matches 2 locations (lines 10, 20)",
		},
		{
			name: "exactly 5 hits (no overflow suffix)",
			hits: []int{1, 2, 3, 4, 5},
			want: "ambiguous match: search block matches 5 locations (lines 1, 2, 3, 4, 5)",
		},
		{
			name: "6 hits (and 1 more)",
			hits: []int{1, 2, 3, 4, 5, 6},
			want: "ambiguous match: search block matches 6 locations (lines 1, 2, 3, 4, 5, ...and 1 more)",
		},
		{
			name: "10 hits (and 5 more)",
			hits: []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10},
			want: "ambiguous match: search block matches 10 locations (lines 1, 2, 3, 4, 5, ...and 5 more)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, formatAmbiguity(tt.hits))
		})
	}
}

func TestFormatFailureDiff(t *testing.T) {
	out := formatFailureDiff("a\nb", "x\ny")
	assert.Contains(t, out, "--- search")
	assert.Contains(t, out, "+++ nearest source region")
	assert.Contains(t, out, "@@ -1,2 +1,2 @@")
	assert.Contains(t, out, "-a")
	assert.Contains(t, out, "-b")
	assert.Contains(t, out, "+x")
	assert.Contains(t, out, "+y")
}

func TestFormatFailureDiff_SingleLine(t *testing.T) {
	out := formatFailureDiff("foo", "bar")
	assert.Contains(t, out, "@@ -1,1 +1,1 @@")
}

func TestFindNearestWindow_PicksBestMatchCount(t *testing.T) {
	source := []string{"unrelated", "a", "b", "zzz", "a", "b"}
	search := []string{"a", "b"}
	// Two perfect windows at indices 1 and 4; we expect the first.
	window := findNearestWindow(source, search)
	assert.Equal(t, []string{"a", "b"}, window)
}

func TestFindNearestWindow_NoMatchReturnsFirstWindow(t *testing.T) {
	source := []string{"x", "y", "z", "w"}
	search := []string{"a", "b"}
	window := findNearestWindow(source, search)
	assert.Len(t, window, 2, "nearest window should be same length as search")
}

func TestFindNearestWindow_SourceShorterThanSearch(t *testing.T) {
	source := []string{"x"}
	search := []string{"a", "b", "c"}
	window := findNearestWindow(source, search)
	assert.Equal(t, source, window, "when source shorter than search, return entire source")
}

func TestFormatFailureDiff_EmptyNearest(t *testing.T) {
	// Defensive: empty nearest window still produces a well-formed payload.
	out := formatFailureDiff("foo", "")
	assert.True(t, strings.HasPrefix(out, "--- search"))
}
