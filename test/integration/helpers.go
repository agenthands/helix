package integration_test

import (
	"context"
	"os/exec"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/require"
)

// requireLS skips the test if the given language server binary is not in PATH.
func requireLS(tb testing.TB, binary string) {
	tb.Helper()
	if _, err := exec.LookPath(binary); err != nil {
		tb.Skipf("%s not installed, skipping integration test", binary)
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
func callTool(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(tb, err, "tool %s call error", name)
	require.False(tb, result.IsError, "tool %s failed: %s", name, textContent(result))
	return result
}

// listSessionTools returns the names of all tools visible to the current MCP session.
// Goes through the MCP client's tools/list so ProfileFilterMiddleware is exercised
// end-to-end — this is the oracle used by profile/mode contract tests (D-02, D-03).
func listSessionTools(tb testing.TB, session *mcp.ClientSession) []string {
	tb.Helper()
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	require.NoError(tb, err, "tools/list")
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// callToolExpectProtocolError invokes an MCP tool and asserts the call is refused
// at the protocol level (a non-nil error from CallTool — e.g. the Phase 91 SEC-01
// ProfileEnforcementMiddleware returning a typed PermissionDenied), rather than
// returning a result with IsError. The handler is never reached. Returns the
// error for further assertions (e.g. message contains "permission_denied").
func callToolExpectProtocolError(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) error {
	tb.Helper()
	_, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.Error(tb, err, "tool %s should be refused at the protocol level", name)
	return err
}

// callToolExpectError invokes an MCP tool and asserts it returns an error result.
func callToolExpectError(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	require.NoError(tb, err, "tool %s call error (protocol level)", name)
	require.True(tb, result.IsError, "tool %s should have returned error but succeeded: %s", name, textContent(result))
	return result
}
