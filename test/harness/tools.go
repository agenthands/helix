//go:build integration || llm || llmjudge

package harness

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// CallTool invokes an MCP tool and asserts success.
func CallTool(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
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

// CallToolExpectError invokes an MCP tool and asserts it returns an error result.
func CallToolExpectError(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
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
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(tb, err, "tools/list")
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}
