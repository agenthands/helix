//go:build integration

package integration_test

import (
	"context"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// textContent extracts the text from the first TextContent element in a CallToolResult.
// Returns empty string if no TextContent is found.
func textContent(r *mcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// callTool invokes an MCP tool and asserts success.
func callTool(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(t, err, "tool %s call error", name)
	require.False(t, result.IsError, "tool %s failed: %s", name, textContent(result))
	return result
}

// callToolExpectError invokes an MCP tool and asserts it returns an error result.
func callToolExpectError(t *testing.T, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(t, err, "tool %s call error (protocol level)", name)
	require.True(t, result.IsError, "tool %s should have returned error but succeeded: %s", name, textContent(result))
	return result
}
