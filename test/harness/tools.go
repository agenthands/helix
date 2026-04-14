//go:build integration || llm || llmjudge

package harness

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// toolTimeout is the default timeout for MCP tool calls in tests.
// Prevents indefinite hangs in CI if a tool or LS deadlocks.
const toolTimeout = 30 * time.Second

// CallTool invokes an MCP tool and asserts success.
func CallTool(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(tb, err, "tool %s call error", name)
	require.False(tb, result.IsError, "tool %s failed: %s", name, TextContent(result))
	return result
}

// TextContent extracts the text from the first TextContent element in a CallToolResult.
// Returns empty string if no TextContent is found.
func TextContent(r *mcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// WaitForSearchResults polls search_symbols until it returns non-empty results.
// Use this for LS that need time to index after initialization (clangd, zls, sourcekit-lsp).
func WaitForSearchResults(tb testing.TB, session *mcp.ClientSession, query string, timeout time.Duration) string {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			tb.Fatalf("search_symbols(%q) returned no results after %v", query, timeout)
			return ""
		case <-ticker.C:
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "search_symbols",
				Arguments: map[string]any{"query": query},
			})
			if err != nil || result.IsError {
				continue
			}
			text := TextContent(result)
			if text != "" && text != "(no results)" {
				return text
			}
		}
	}
}

// CallToolExpectError invokes an MCP tool and asserts it returns an error result.
func CallToolExpectError(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	result, err := session.CallTool(ctx, &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(tb, err, "tool %s call error (protocol level)", name)
	require.True(tb, result.IsError, "tool %s should have returned error but succeeded: %s", name, TextContent(result))
	return result
}

// ListSessionTools returns the names of all tools visible to the current MCP session.
// Goes through the MCP client's tools/list so ProfileFilterMiddleware is exercised end-to-end.
func ListSessionTools(tb testing.TB, session *mcp.ClientSession) []string {
	tb.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), toolTimeout)
	defer cancel()
	result, err := session.ListTools(ctx, &mcp.ListToolsParams{})
	require.NoError(tb, err, "tools/list")
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}
