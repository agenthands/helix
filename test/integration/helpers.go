//go:build integration

package integration_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// requireLS skips the test if the given language server binary is not in PATH.
func requireLS(t *testing.T, binary string) {
	t.Helper()
	if _, err := exec.LookPath(binary); err != nil {
		t.Skipf("%s not installed, skipping integration test", binary)
	}
}

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

// listSessionTools returns the names of all tools visible to the current MCP session.
// Goes through the MCP client's tools/list so ProfileFilterMiddleware is exercised
// end-to-end — this is the oracle used by profile/mode contract tests (D-02, D-03).
func listSessionTools(t *testing.T, session *mcp.ClientSession) []string {
	t.Helper()
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(t, err, "tools/list")
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
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
