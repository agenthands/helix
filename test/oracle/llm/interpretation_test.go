//go:build llm

package llm

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestInterpretation verifies that an LLM can correctly interpret MCP tool
// outputs — distinguishing success from failure and not hallucinating
// unsupported conclusions (LLM-03/D-04).
// Sequential execution with inter-call delays (D-19). No t.Parallel.
func TestInterpretation(t *testing.T) {
	SkipWithoutAPIKey(t)

	client := NewClient()
	model := SubjectModel()
	_, selfJudged := JudgeModel()
	system := InterpretationSystemPrompt()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	// Load golden files for success and error cases.
	successCases := LoadGoldenFiles(t)
	errorCases := LoadErrorGoldenFiles(t)

	require.NotEmpty(t, successCases, "expected at least one success golden file")

	// If more than 30 success cases, sample every Nth to keep under ~40 API calls.
	if len(successCases) > 30 {
		n := (len(successCases) + 29) / 30 // ceiling division to get step size
		var sampled []GoldenCase
		for i := 0; i < len(successCases); i += n {
			sampled = append(sampled, successCases[i])
		}
		t.Logf("Sampled %d of %d success cases (every %d)", len(sampled), len(successCases), n)
		successCases = sampled
	}

	callCount := 0

	// Success cases: expect STATUS: SUCCESS.
	for _, gc := range successCases {
		if callCount > 0 {
			InterCallDelay()
		}

		gc := gc // capture
		t.Run("success/"+gc.ToolName+"/"+gc.Scenario, func(t *testing.T) {
			user := InterpretationUserPrompt(gc.ToolName, gc.Content)

			response, stopReason, err := AskSingleTurn(ctx, client, model, system, user)
			require.NoError(t, err, "AskSingleTurn failed for %s/%s", gc.ToolName, gc.Scenario)

			_ = stopReason

			// Assert STATUS: SUCCESS (case-insensitive check).
			upper := strings.ToUpper(response)
			require.Contains(t, upper, "STATUS: SUCCESS",
				"expected STATUS: SUCCESS for success golden %s/%s, got response:\n%s",
				gc.ToolName, gc.Scenario, response)

			// Assert LIMITATIONS section present (LLM acknowledges what it cannot conclude).
			require.Contains(t, upper, "LIMITATIONS:",
				"expected LIMITATIONS section in response for %s/%s, got:\n%s",
				gc.ToolName, gc.Scenario, response)

			// Write transcript for judge consumption.
			WriteTranscript(t, &Transcript{
				ScenarioID: "interpret-" + gc.ToolName + "-" + gc.Scenario + "-success",
				Category:   "interpretation",
				Model:      model,
				System:     system,
				UserPrompt: user,
				Response:   response,
				StopReason: stopReason,
				SelfJudged: selfJudged,
			})
		})
		callCount++
	}

	// Error cases: expect STATUS: FAILURE or STATUS: UNCLEAR (not SUCCESS).
	for _, gc := range errorCases {
		if callCount > 0 {
			InterCallDelay()
		}

		gc := gc // capture
		t.Run("error/"+gc.ToolName+"/"+gc.Scenario, func(t *testing.T) {
			user := InterpretationUserPrompt(gc.ToolName, gc.Content)

			response, stopReason, err := AskSingleTurn(ctx, client, model, system, user)
			require.NoError(t, err, "AskSingleTurn failed for error case %s/%s", gc.ToolName, gc.Scenario)

			_ = stopReason

			// Assert NOT STATUS: SUCCESS — should be FAILURE or UNCLEAR.
			upper := strings.ToUpper(response)
			require.NotContains(t, upper, "STATUS: SUCCESS",
				"expected STATUS: FAILURE or UNCLEAR for error golden %s/%s, got response:\n%s",
				gc.ToolName, gc.Scenario, response)

			// Should contain either FAILURE or UNCLEAR.
			hasFailure := strings.Contains(upper, "STATUS: FAILURE")
			hasUnclear := strings.Contains(upper, "STATUS: UNCLEAR")
			require.True(t, hasFailure || hasUnclear,
				"expected STATUS: FAILURE or STATUS: UNCLEAR for error golden %s/%s, got:\n%s",
				gc.ToolName, gc.Scenario, response)

			// Write transcript for judge consumption.
			WriteTranscript(t, &Transcript{
				ScenarioID: "interpret-" + gc.ToolName + "-" + gc.Scenario + "-error",
				Category:   "interpretation",
				Model:      model,
				System:     system,
				UserPrompt: user,
				Response:   response,
				StopReason: stopReason,
				SelfJudged: selfJudged,
			})
		})
		callCount++
	}

	t.Logf("Total interpretation API calls: %d (success: %d, error: %d)",
		callCount, len(successCases), len(errorCases))
}
