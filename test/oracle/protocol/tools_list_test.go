//go:build integration || llm || llmjudge

package protocol_test

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/test/harness"
)

// TestToolsList_UniqueNames verifies that tools/list returns tools with unique
// names and a non-zero count (PROTO-02).
func TestToolsList_UniqueNames(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools error")
	require.NotEmpty(t, result.Tools, "tools list must not be empty")

	seen := make(map[string]bool, len(result.Tools))
	for _, tool := range result.Tools {
		t.Run(tool.Name, func(t *testing.T) {
			require.False(t, seen[tool.Name], "duplicate tool name: %s", tool.Name)
			seen[tool.Name] = true
		})
	}
}

// TestToolsList_NonEmptyDescriptions verifies that every tool has a meaningful
// description (at least 10 characters) (PROTO-02).
func TestToolsList_NonEmptyDescriptions(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools error")

	for _, tool := range result.Tools {
		t.Run(tool.Name, func(t *testing.T) {
			require.NotEmpty(t, tool.Description, "tool %s has empty description", tool.Name)
			require.GreaterOrEqual(t, len(tool.Description), 10,
				"tool %s description too short (%d chars): %q", tool.Name, len(tool.Description), tool.Description)
		})
	}
}

// TestToolsList_InputSchemaPresent verifies that every tool declares an
// inputSchema with type "object" (PROTO-02).
func TestToolsList_InputSchemaPresent(t *testing.T) {
	runner := harness.StartRunner(t, harness.RunnerOptions{SkipLS: true})
	defer runner.Stop()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := runner.Session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(t, err, "ListTools error")

	for _, tool := range result.Tools {
		t.Run(tool.Name, func(t *testing.T) {
			require.NotNil(t, tool.InputSchema, "tool %s has nil InputSchema", tool.Name)

			schemaMap, ok := tool.InputSchema.(map[string]any)
			require.True(t, ok, "tool %s InputSchema is not map[string]any, got %T", tool.Name, tool.InputSchema)

			typeVal, hasType := schemaMap["type"]
			require.True(t, hasType, "tool %s InputSchema missing 'type' key", tool.Name)
			require.Equal(t, "object", typeVal, "tool %s InputSchema type must be 'object'", tool.Name)
		})
	}
}
