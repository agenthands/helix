//go:build llm

package llm

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/harness"
)

// disambiguationPairs contains all similar tool pairs to test.
// Includes the 7 pairs from selectability_test.go plus 4 additional
// LLM-confusable pairs (D-03).
var disambiguationPairs = [][2]string{
	// From selectability_test.go
	{"search_symbols", "find_references"},
	{"get_symbol_overview", "get_hover_info"},
	{"read_file", "read_memory"},
	{"write_file", "write_memory"},
	{"find_files", "search_in_files"},
	{"list_directory", "find_files"},
	{"insert_before_symbol", "insert_after_symbol"},
	// Additional LLM-confusable pairs
	{"edit_symbol_body", "replace_symbol"},
	{"get_call_hierarchy", "get_type_hierarchy"},
	{"search_symbols", "search_in_files"},
	{"onboard_project", "list_directory"},
}

// TestDisambiguation verifies that an LLM can distinguish between similar
// tool pairs based on descriptions alone (LLM-02). Tests each pair
// bidirectionally. Sequential execution with inter-call delays (D-19).
func TestDisambiguation(t *testing.T) {
	SkipWithoutAPIKey(t)

	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools failed")

	// Build description map from actual tool list.
	descMap := make(map[string]string, len(result.Tools))
	for _, tool := range result.Tools {
		descMap[tool.Name] = tool.Description
	}

	client := NewClient()
	model := SubjectModel()
	callCount := 0

	for _, pair := range disambiguationPairs {
		toolA, toolB := pair[0], pair[1]

		// Skip if either tool not found (may be filtered by profile).
		descA, okA := descMap[toolA]
		descB, okB := descMap[toolB]
		if !okA || !okB {
			t.Logf("skipping pair %s/%s: tool not found in session", toolA, toolB)
			continue
		}

		// Test both directions: select toolA, then select toolB.
		for _, target := range []string{toolA, toolB} {
			subtestName := fmt.Sprintf("%s_vs_%s_select_%s", toolA, toolB, target)

			if callCount > 0 {
				InterCallDelay()
			}
			callCount++

			t.Run(subtestName, func(t *testing.T) {
				system := DisambiguationSystemPrompt(toolA, descA, toolB, descB)
				taskDesc := DisambiguationTask(toolA, toolB, target)
				user := DisambiguationUserPrompt(taskDesc)

				response, stopReason, err := AskSingleTurn(ctx, client, model, system, user)
				require.NoError(t, err, "AskSingleTurn failed for %s", subtestName)

				require.True(t,
					strings.Contains(strings.ToLower(strings.TrimSpace(response)), strings.ToLower(target)),
					"expected response to contain target tool %q, got %q", target, response)

				WriteTranscript(t, &Transcript{
					ScenarioID: fmt.Sprintf("disambig-%s-vs-%s-select-%s", toolA, toolB, target),
					Category:   "disambiguation",
					Model:      model,
					System:     system,
					UserPrompt: user,
					Response:   response,
					StopReason: stopReason,
				})
			})
		}
	}
}
