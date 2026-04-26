package obs

import (
	"testing"

	dto "github.com/prometheus/client_model/go"
)

// TestMetrics_CardinalityBounds asserts the per-family series caps from
// .planning/phases/53-obs-metrics-gaps/53-CONTEXT.md D-03 and analogous
// bounds for the new families. Worst-case priming uses a synthetic
// 52-language list (matching the LS catalog upper bound). Helper-side
// closed-enum drops mean any unknown enum value is silently rejected, so
// this test simultaneously documents the bound AND proves the helper guards
// hold when fed every legal enum combination.
func TestMetrics_CardinalityBounds(t *testing.T) {
	m := newMetrics()

	// Build 52 distinct synthetic language identifiers (e.g. "langaa".."langza").
	languages := make([]string, 0, 52)
	for i := 0; i < 52; i++ {
		languages = append(languages,
			"lang"+string(rune('a'+(i%26)))+string(rune('a'+(i/26))))
	}

	for _, lang := range languages {
		for _, result := range []string{"hit", "miss"} {
			for _, scope := range []string{"clean", "dirty", "crashed"} {
				m.LSPoolCacheInc(lang, result, scope)
			}
			m.RepoMapCacheInc(lang, result)
		}
		m.RepoMapExtractObserve(lang, 0.001)
		for _, ph := range []string{"activate", "deactivate", "timeout", "shutdown"} {
			m.SessionLifecycleInc(lang, ph)
		}
	}
	for _, tool := range []string{
		"replace_symbol_body", "insert_before_symbol", "insert_after_symbol",
		"rename_symbol", "safe_delete_symbol",
		"replace_in_file", "fuzzy_edit", "create_file",
	} {
		for _, oc := range []string{"success", "fuzzy_applied", "refused_ambiguous", "failed"} {
			m.EditOutcomeInc(tool, oc)
		}
	}

	caps := map[string]int{
		"serena_lspool_cache_total":               52 * 2 * 3, // 312
		"serena_repomap_cache_total":              52 * 2,     // 104
		"serena_repomap_extract_duration_seconds": 52,
		"serena_session_lifecycle_total":          52 * 4, // 208
		"serena_edit_outcome_total":               8 * 4,  // 32
	}

	mfs, err := m.Registry().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
	seen := map[string]int{}
	for _, mf := range mfs {
		seen[mf.GetName()] = len(mf.GetMetric())
	}
	for name, capN := range caps {
		got, ok := seen[name]
		if !ok {
			t.Errorf("family %q not registered", name)
			continue
		}
		if got > capN {
			t.Errorf("family %q has %d series; cap is %d (D-03 bound)", name, got, capN)
		}
	}
	_ = (*dto.MetricFamily)(nil) // ensure dto import retained even if unused above
}
