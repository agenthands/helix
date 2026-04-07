package mcp

import (
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/postfix/serena/internal/workspace"
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

func TestErrorDetail_JSON(t *testing.T) {
	detail := ErrorDetail{
		Code:       "WORKSPACE_NOT_READY",
		Cause:      "workspace is still initializing",
		Suggestion: "wait and retry in a few seconds",
	}
	data, err := detail.MarshalJSON()
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"code":"WORKSPACE_NOT_READY"`)
	assert.Contains(t, string(data), `"suggestion"`)
}

func TestErrorDetail_JSON_NoSuggestion(t *testing.T) {
	detail := ErrorDetail{
		Code:  "SESSION_EXPIRED",
		Cause: "session timed out",
	}
	data, err := json.Marshal(detail)
	assert.NoError(t, err)
	assert.Contains(t, string(data), `"code":"SESSION_EXPIRED"`)
	assert.NotContains(t, string(data), `"suggestion"`)
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

func TestDomainErrors(t *testing.T) {
	assert.EqualError(t, ErrSessionExpired, "session expired")
	assert.EqualError(t, ErrWorkspaceNotReady, "workspace not ready")
	assert.EqualError(t, ErrToolNotAvailable, "tool not available in current mode")
	assert.EqualError(t, ErrProjectNotFound, "project not found at specified path")
	assert.EqualError(t, ErrLSCrashed, "language server crashed")
}

func TestNewSerenaMCPServer(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	reg := workspace.NewRegistry()
	srv := NewSerenaMCPServer(reg, logger)

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
	srv := NewSerenaMCPServer(reg, logger)

	initialCount := srv.Registry().Count()

	// Test registry add/remove which is what AddTool/RemoveTool wraps
	srv.Registry().Register(&ToolDef{Name: "custom_tool", Description: "a custom tool"})
	assert.Equal(t, initialCount+1, srv.Registry().Count())
	assert.Contains(t, srv.Registry().Names(), "custom_tool")

	srv.Registry().Unregister("custom_tool")
	assert.Equal(t, initialCount, srv.Registry().Count())
	assert.NotContains(t, srv.Registry().Names(), "custom_tool")
}
