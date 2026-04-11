//go:build integration || llm || llmjudge

package contract_test

import (
	"context"
	"strings"
	"testing"
	"time"
	"unicode"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// actionVerbs is the set of verbs that indicate a tool description communicates
// a clear action to an LLM agent (D-11).
var actionVerbs = []string{
	"activate", "analyze", "create", "delete", "edit", "find", "format",
	"get", "go", "insert", "list", "onboard", "prepare", "read", "rename",
	"replace", "retrieve", "return", "safe", "search", "switch", "verify", "write",
}

// hasActionVerb returns true if the description contains at least one action verb.
func hasActionVerb(desc string) bool {
	lower := strings.ToLower(desc)
	for _, v := range actionVerbs {
		if strings.Contains(lower, v) {
			return true
		}
	}
	return false
}

// listToolsWithMeta starts a runner and returns the full tool list with metadata.
func listToolsWithMeta(t *testing.T) []*mcp.Tool {
	t.Helper()
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools failed")
	require.NotEmpty(t, result.Tools, "expected at least one tool")
	_ = runner // kept alive by t.Cleanup
	return result.Tools
}

// wordSet splits a description into a set of lowercase words for similarity comparison.
func wordSet(desc string) map[string]struct{} {
	words := strings.FieldsFunc(strings.ToLower(desc), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	set := make(map[string]struct{}, len(words))
	for _, w := range words {
		set[w] = struct{}{}
	}
	return set
}

// TestSelectability_ActionVerbs asserts that every tool description contains
// at least one action verb for positive LLM selection (D-11).
func TestSelectability_ActionVerbs(t *testing.T) {
	tools := listToolsWithMeta(t)

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			assert.True(t, hasActionVerb(tool.Description),
				"description for %s should contain an action verb: %q", tool.Name, tool.Description)
		})
	}
}

// TestSelectability_UniqueDescriptions asserts that no two tools share the
// exact same description text.
func TestSelectability_UniqueDescriptions(t *testing.T) {
	tools := listToolsWithMeta(t)

	seen := map[string]string{} // description -> first tool name
	for _, tool := range tools {
		if prev, exists := seen[tool.Description]; exists {
			t.Errorf("duplicate description between %s and %s: %q",
				prev, tool.Name, tool.Description)
		}
		seen[tool.Description] = tool.Name
	}
}

// TestSelectability_MinimumLength asserts that every tool description is long
// enough to be meaningful for LLM selection (at least 20 characters).
func TestSelectability_MinimumLength(t *testing.T) {
	tools := listToolsWithMeta(t)

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			assert.Greater(t, len(tool.Description), 20,
				"description for %s is too short to be LLM-selectable: %q", tool.Name, tool.Description)
		})
	}
}

// TestSelectability_Disambiguation asserts that similar tool pairs have
// sufficiently different descriptions so an LLM can distinguish them (D-11).
func TestSelectability_Disambiguation(t *testing.T) {
	similarPairs := [][2]string{
		{"search_symbols", "find_references"},
		{"get_symbol_overview", "get_hover_info"},
		{"read_file", "read_memory"},
		{"write_file", "write_memory"},
		{"find_files", "search_in_files"},
		{"list_directory", "find_files"},
		{"insert_before_symbol", "insert_after_symbol"},
	}

	tools := listToolsWithMeta(t)
	descMap := make(map[string]string, len(tools))
	for _, tool := range tools {
		descMap[tool.Name] = tool.Description
	}

	for _, pair := range similarPairs {
		t.Run(pair[0]+"_vs_"+pair[1], func(t *testing.T) {
			descA, okA := descMap[pair[0]]
			descB, okB := descMap[pair[1]]
			if !okA || !okB {
				t.Skipf("tool %s or %s not found (may be filtered by profile)", pair[0], pair[1])
				return
			}

			setA := wordSet(descA)
			setB := wordSet(descB)

			// Calculate Jaccard similarity: intersection / union
			intersection := 0
			for w := range setA {
				if _, ok := setB[w]; ok {
					intersection++
				}
			}
			union := len(setA) + len(setB) - intersection
			jaccard := float64(intersection) / float64(union)

			assert.Less(t, jaccard, 0.75,
				"descriptions for %s and %s are too similar (Jaccard=%.2f):\n  %s: %q\n  %s: %q",
				pair[0], pair[1], jaccard, pair[0], descA, pair[1], descB)

			// Each description must have at least one distinguishing word
			hasUniqueA := false
			for w := range setA {
				if _, ok := setB[w]; !ok {
					hasUniqueA = true
					break
				}
			}
			hasUniqueB := false
			for w := range setB {
				if _, ok := setA[w]; !ok {
					hasUniqueB = true
					break
				}
			}
			assert.True(t, hasUniqueA,
				"description for %s has no distinguishing words from %s", pair[0], pair[1])
			assert.True(t, hasUniqueB,
				"description for %s has no distinguishing words from %s", pair[1], pair[0])
		})
	}
}

// TestSelectability_NegativeSelection asserts that tool descriptions do not
// contain misleading or placeholder terms that would confuse LLM selection.
func TestSelectability_NegativeSelection(t *testing.T) {
	tools := listToolsWithMeta(t)

	badTerms := []string{"todo", "placeholder", "not implemented", "deprecated"}

	for _, tool := range tools {
		t.Run(tool.Name, func(t *testing.T) {
			lower := strings.ToLower(tool.Description)
			for _, term := range badTerms {
				assert.NotContains(t, lower, term,
					"description for %s contains misleading term %q: %q",
					tool.Name, term, tool.Description)
			}
		})
	}
}
