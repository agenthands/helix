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

// TestGRPCSmoke_ToolCallRoundTrip drives tool calls through a second MCP session
// over the retained gRPC/in-memory transport (the wire the CLI dials), proving
// the full serialization path end-to-end against the warm daemon.
//
// Phase 94 RETIRE-02: this replaces the former TestHTTPSmoke_ToolCallRoundTrip —
// the Streamable-HTTP /mcp head was deleted. The CLI-subprocess unix-socket
// round-trip is additionally covered by TestCLI_E2E_OneShot.
func TestGRPCSmoke_ToolCallRoundTrip(t *testing.T) {
	td := StartTestDaemon(t, Options{SkipLS: true})
	grpcSession := td.NewGRPCSession(t)

	// Test 1: list_memories -- proves full JSON serialization path.
	result, err := grpcSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "list_memories",
		Arguments: map[string]any{},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "list_memories via gRPC failed: %s", textContent(result))

	// Test 2: write + read -- proves bidirectional data integrity.
	writeResult, err := grpcSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "write_memory",
		Arguments: map[string]any{"name": "grpc-test", "content": "via gRPC transport"},
	})
	require.NoError(t, err)
	require.False(t, writeResult.IsError, "write_memory via gRPC failed: %s", textContent(writeResult))

	readResult, err := grpcSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "read_memory",
		Arguments: map[string]any{"name": "grpc-test"},
	})
	require.NoError(t, err)
	require.False(t, readResult.IsError, "read_memory via gRPC failed: %s", textContent(readResult))
	assert.Contains(t, textContent(readResult), "via gRPC transport")
}

func TestGRPCSmoke_WithLS(t *testing.T) {
	requireGopls(t)
	fixture := PrepareFixture(t, "go")
	// Activate workspace but skip LS wait -- we poll manually below
	// so we can skip (not fail) if gopls doesn't start.
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, SkipLS: true})

	// Poll for LS readiness with a generous timeout; skip if gopls
	// doesn't come up (pre-existing environment issue, not the transport).
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ready := false
	for !ready {
		select {
		case <-ctx.Done():
			t.Skip("gopls did not become ready within timeout -- skipping gRPC+LS smoke test")
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

	grpcSession := td.NewGRPCSession(t)

	// Symbol search via the retained gRPC/in-memory transport.
	result, err := grpcSession.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "search_symbols",
		Arguments: map[string]any{"query": "Helper"},
	})
	require.NoError(t, err)
	require.False(t, result.IsError, "search_symbols via gRPC failed: %s", textContent(result))
	assert.Contains(t, textContent(result), "Helper")
}
