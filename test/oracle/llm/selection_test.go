//go:build llm

package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestSelection verifies that an LLM can select the correct tool for every
// exposed Serena tool given a natural-language task description (LLM-01).
// One subtest per tool. Sequential execution with inter-call delays (D-19).
func TestSelection(t *testing.T) {
	SkipWithoutAPIKey(t)

	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools failed")
	require.NotEmpty(t, result.Tools, "expected at least one tool")

	client := NewClient()
	model := SubjectModel()
	toolListStr := FormatToolList(result.Tools)

	for i, tool := range result.Tools {
		if i > 0 {
			InterCallDelay()
		}

		t.Run(tool.Name, func(t *testing.T) {
			system := SelectionSystemPrompt(toolListStr)
			taskDesc := ToolTaskDescription(tool.Name)
			user := SelectionUserPrompt(taskDesc)

			response, stopReason, err := AskSingleTurn(ctx, client, model, system, user)
			require.NoError(t, err, "AskSingleTurn failed for %s", tool.Name)

			require.True(t,
				strings.Contains(strings.ToLower(strings.TrimSpace(response)), strings.ToLower(tool.Name)),
				"expected response to contain tool name %q, got %q", tool.Name, response)

			WriteTranscript(t, &Transcript{
				ScenarioID: "select-" + tool.Name,
				Category:   "selection",
				Model:      model,
				System:     system,
				UserPrompt: user,
				Response:   response,
				StopReason: stopReason,
			})
		})
	}
}
