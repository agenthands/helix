//go:build integration

package integration_test

import (
	"context"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPSmoke_ToolCallRoundTrip(t *testing.T) {
	td := StartTestDaemon(t, Options{SkipLS: true})
	httpSession := td.NewHTTPSession(t)

	// Test 1: list_memories via HTTP -- proves full JSON serialization path.
	result, err := httpSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_memories",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "list_memories via HTTP failed: %s", textContent(result))

	// Test 2: write + read via HTTP -- proves bidirectional data integrity.
	writeResult, err := httpSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "write_memory",
		Arguments: map[string]any{"name": "http-test", "content": "via HTTP transport"},
	})
	require.NoError(t, err)
	require.False(t, writeResult.IsError, "write_memory via HTTP failed: %s", textContent(writeResult))

	readResult, err := httpSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "read_memory",
		Arguments: map[string]any{"name": "http-test"},
	})
	require.NoError(t, err)
	require.False(t, readResult.IsError, "read_memory via HTTP failed: %s", textContent(readResult))
	assert.Contains(t, textContent(readResult), "via HTTP transport")
}

func TestHTTPSmoke_WithLS(t *testing.T) {
	requireGopls(t)
	fixture := PrepareFixture(t, "go")
	// Activate workspace but skip LS wait -- we poll manually below
	// so we can skip (not fail) if gopls doesn't start.
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, SkipLS: true})

	// Poll for LS readiness with a generous timeout; skip if gopls
	// doesn't come up (pre-existing environment issue, not HTTP transport).
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ready := false
	for !ready {
		select {
		case <-ctx.Done():
			t.Skip("gopls did not become ready within timeout -- skipping HTTP+LS smoke test")
		default:
		}
		res, err := td.Session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "search_symbols",
			Arguments: map[string]any{"query": "main"},
		})
		if err == nil && !res.IsError && textContent(res) != "" && textContent(res) != "(no results)" {
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	httpSession := td.NewHTTPSession(t)

	// Symbol search via HTTP transport.
	result, err := httpSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search_symbols",
		Arguments: map[string]any{"query": "Helper", "scope": "workspace"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "search_symbols via HTTP failed: %s", textContent(result))
	assert.Contains(t, textContent(result), "Helper")
}
