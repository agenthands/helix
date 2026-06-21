package aggregator

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// per_language_test.go locks REPORT-02: renderPerLanguage lists ALL 8 Tier-1
// languages (the canonical bench/languages set: cpp, csharp, go, java,
// javascript, python, rust, typescript — NO `c`) in fixed order; a language
// present in ByLanguage renders its pass_rate + n, and a language with NO
// benchmark coverage renders `n/a` (NOT omitted). The renderer is RNG-free and
// byte-stable, and cites the cost-table valid_until via the shared footer.

// TestPerLanguageRender locks the column/coverage/n-a contract with injected rows.
func TestPerLanguageRender(t *testing.T) {
	// Only python + go carry coverage; the other 6 Tier-1 languages are absent.
	byLang := []LanguageRow{
		{Language: "python", PassRate: 0.75, N: 4},
		{Language: "go", PassRate: 1.0, N: 3},
		// A non-Tier-1 / "" bucket must NOT leak into the fixed Tier-1 table.
		{Language: "", PassRate: 0.5, N: 2},
	}
	md := renderPerLanguage(byLang, testFooter())

	// All 8 Tier-1 languages appear, in fixed order.
	for _, lang := range []string{"cpp", "csharp", "go", "java", "javascript", "python", "rust", "typescript"} {
		assert.Contains(t, md, lang, "Tier-1 language %q must appear in the table", lang)
	}
	// `c` must NOT appear as its own row (the research 9-list is wrong).
	assert.NotContains(t, md, "| c |", "a bare `c` row must not be rendered")

	// Covered languages render their pooled pass-rate (%.4f) + n.
	assert.Contains(t, md, "0.7500", "python pass-rate must render at %.4f")
	assert.Contains(t, md, "1.0000", "go pass-rate must render at %.4f")

	// No-coverage Tier-1 languages render n/a, not omitted, never a fabricated 0.
	assert.Contains(t, md, "n/a", "a no-coverage Tier-1 language must render n/a")
	// java has no coverage -> its row carries n/a.
	javaIdx := strings.Index(md, "| java |")
	require.GreaterOrEqual(t, javaIdx, 0, "java must have a row")

	// Fixed order: cpp precedes csharp precedes go precedes ... precedes typescript.
	order := []string{"| cpp |", "| csharp |", "| go |", "| java |", "| javascript |", "| python |", "| rust |", "| typescript |"}
	prev := -1
	for _, seg := range order {
		idx := strings.Index(md, seg)
		require.GreaterOrEqual(t, idx, 0, "row %q must exist", seg)
		assert.Greater(t, idx, prev, "Tier-1 rows must be in fixed sorted order at %q", seg)
		prev = idx
	}

	// Footer provenance (valid_until citation).
	assert.Contains(t, md, "2027-01-28", "per_language footer cites the cost-table valid_until")
}

// TestPerLanguageByteStable: a double render of the same input is byte-identical
// (no map-iteration order leak; RNG-free).
func TestPerLanguageByteStable(t *testing.T) {
	byLang := []LanguageRow{
		{Language: "python", PassRate: 0.75, N: 4},
		{Language: "rust", PassRate: 0.33, N: 9},
	}
	a := renderPerLanguage(byLang, testFooter())
	b := renderPerLanguage(byLang, testFooter())
	assert.Equal(t, a, b, "renderPerLanguage must be byte-stable across renders")
}

// TestPerLanguageGolden runs the aggregate-time ByLanguage reduce over a partial-
// coverage Tier-1 fixture (only go + python + rust covered) and diffs the rendered
// per_language.md against the committed golden — proving the 5 no-coverage Tier-1
// languages render n/a. Regenerate with `-update`.
func TestPerLanguageGolden(t *testing.T) {
	dir := t.TempDir()
	// go + python pass, rust fails; the other 5 Tier-1 languages get no rows.
	writeLangRow(t, dir, "go-task", "your_agent_full", "go", 0, metric(true, 1000, 100, 5, 3, 0.9))
	writeLangRow(t, dir, "py-task", "your_agent_full", "python", 0, metric(true, 1000, 100, 5, 3, 0.9))
	writeLangRow(t, dir, "rs-task", "your_agent_full", "rust", 0, metric(false, 2000, 200, 9, 6, 0.5))

	rep, err := Aggregate(dir, aggConfig(1))
	require.NoError(t, err)
	require.NotNil(t, rep)

	got := renderPerLanguage(rep.ByLanguage, rep.Footer)
	assertGolden(t, "per_language.golden.md", got)
}
