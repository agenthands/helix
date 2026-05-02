package mcp

import (
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/agenthands/helix/internal/workspace"
)

func TestNewToolRegistry(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	r := NewToolRegistry(logger)
	assert.Equal(t, 0, r.Count())

	r.Register(&ToolDef{Name: "ping", Description: "test tool"})
	assert.Equal(t, 1, r.Count())
	assert.Contains(t, r.Names(), "ping")

	r.Unregister("ping")
	assert.Equal(t, 0, r.Count())
}

func TestSessionInfo_AllowedTools(t *testing.T) {
	s := &SessionInfo{
		SessionID:    "test-session",
		AllowedTools: []string{"ping", "echo"},
	}
	assert.Equal(t, 2, len(s.AllowedTools))
}

func TestSessionInfo_NilAllowedTools(t *testing.T) {
	s := &SessionInfo{
		SessionID: "test-session",
	}
	// nil AllowedTools means all tools are available
	assert.Nil(t, s.AllowedTools)
}

func TestNewSerenaMCPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reg := workspace.NewRegistry()
	srv := NewSerenaMCPServer(reg, logger, nil)

	assert.NotNil(t, srv)
	assert.NotNil(t, srv.SDK())
	assert.NotNil(t, srv.Registry())

	// Verify dummy tools are registered in our registry
	names := srv.Registry().Names()
	assert.Contains(t, names, "ping")
	assert.Contains(t, names, "echo")
	assert.Contains(t, names, "activate_project")
	assert.Equal(t, 3, srv.Registry().Count())
}

func TestSerenaMCPServer_DynamicToolAddRemove(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reg := workspace.NewRegistry()
	srv := NewSerenaMCPServer(reg, logger, nil)

	initialCount := srv.Registry().Count()

	// Test registry add/remove which is what AddTool/RemoveTool wraps
	srv.Registry().Register(&ToolDef{Name: "custom_tool", Description: "a custom tool"})
	assert.Equal(t, initialCount+1, srv.Registry().Count())
	assert.Contains(t, srv.Registry().Names(), "custom_tool")

	srv.Registry().Unregister("custom_tool")
	assert.Equal(t, initialCount, srv.Registry().Count())
	assert.NotContains(t, srv.Registry().Names(), "custom_tool")
}
