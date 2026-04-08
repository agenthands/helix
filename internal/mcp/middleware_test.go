package mcp_test

import (
	"context"
	"log/slog"
	"os"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/internal/mcp"
)

// mockProfileResolver is a test-only ProfileResolver that avoids importing the profile package.
type mockProfileResolver struct {
	overrides map[string]map[string]string // profileName -> toolName -> description
}

func (r *mockProfileResolver) ToolDescriptionOverrides(profileName string) map[string]string {
	if r.overrides == nil {
		return nil
	}
	return r.overrides[profileName]
}

func TestProfileFilterMiddleware_FiltersToolsList(t *testing.T) {
	resolver := buildTestResolver(t)

	// Session with an AllowedTools whitelist
	session := &mcp.SessionInfo{
		SessionID:    "test-1",
		Profile:      "claude-code",
		Mode:         "edit",
		AllowedTools: []string{"find_symbol", "activate_project"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mw := mcp.ProfileFilterMiddleware(resolver, func(ctx context.Context) *mcp.SessionInfo {
		return session
	}, logger)

	// Simulate a tools/list response with 3 tools (one should be filtered)
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.ListToolsResult{
			Tools: []*mcpsdk.Tool{
				{Name: "find_symbol", Description: "original description"},
				{Name: "activate_project", Description: "original activate"},
				{Name: "some_excluded_tool", Description: "should be filtered"},
			},
		}, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listResult, ok := result.(*mcpsdk.ListToolsResult)
	if !ok {
		t.Fatalf("expected *ListToolsResult, got %T", result)
	}

	// Only the 2 allowed tools should remain
	if len(listResult.Tools) != 2 {
		t.Errorf("expected 2 tools after filtering, got %d", len(listResult.Tools))
	}
	for _, tool := range listResult.Tools {
		if tool.Name == "some_excluded_tool" {
			t.Error("expected some_excluded_tool to be filtered out")
		}
	}
}

func TestProfileFilterMiddleware_AppliesDescriptionOverrides(t *testing.T) {
	resolver := buildTestResolver(t)

	session := &mcp.SessionInfo{
		SessionID:    "test-2",
		Profile:      "test-override",
		Mode:         "edit",
		AllowedTools: []string{"find_symbol", "activate_project"},
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mw := mcp.ProfileFilterMiddleware(resolver, func(ctx context.Context) *mcp.SessionInfo {
		return session
	}, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.ListToolsResult{
			Tools: []*mcpsdk.Tool{
				{Name: "find_symbol", Description: "original description"},
				{Name: "activate_project", Description: "original activate"},
			},
		}, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listResult := result.(*mcpsdk.ListToolsResult)

	// Check that the description override was applied
	for _, tool := range listResult.Tools {
		if tool.Name == "find_symbol" {
			if tool.Description != "Find a symbol with overridden description" {
				t.Errorf("expected overridden description, got %q", tool.Description)
			}
		}
	}
}

func TestProfileFilterMiddleware_PassesThroughNonToolsMethods(t *testing.T) {
	resolver := buildTestResolver(t)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mw := mcp.ProfileFilterMiddleware(resolver, func(ctx context.Context) *mcp.SessionInfo {
		return nil
	}, logger)

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.ListToolsResult{}, nil
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "resources/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler to be called for non-tools method")
	}
}

func TestProfileFilterMiddleware_NilSession(t *testing.T) {
	resolver := buildTestResolver(t)

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mw := mcp.ProfileFilterMiddleware(resolver, func(ctx context.Context) *mcp.SessionInfo {
		return nil
	}, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.ListToolsResult{
			Tools: []*mcpsdk.Tool{
				{Name: "ping", Description: "test"},
			},
		}, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// With nil session, tools should pass through unfiltered
	listResult := result.(*mcpsdk.ListToolsResult)
	if len(listResult.Tools) != 1 {
		t.Errorf("expected 1 tool (unfiltered), got %d", len(listResult.Tools))
	}
}

func TestProfileFilterMiddleware_NilAllowedToolsPassesAll(t *testing.T) {
	resolver := buildTestResolver(t)

	// Session with nil AllowedTools = all tools allowed
	session := &mcp.SessionInfo{
		SessionID:    "test-all",
		Profile:      "full",
		Mode:         "edit",
		AllowedTools: nil,
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	mw := mcp.ProfileFilterMiddleware(resolver, func(ctx context.Context) *mcp.SessionInfo {
		return session
	}, logger)

	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.ListToolsResult{
			Tools: []*mcpsdk.Tool{
				{Name: "ping", Description: "test"},
				{Name: "echo", Description: "test2"},
			},
		}, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/list", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	listResult := result.(*mcpsdk.ListToolsResult)
	if len(listResult.Tools) != 2 {
		t.Errorf("expected 2 tools (nil = all allowed), got %d", len(listResult.Tools))
	}
}

// buildTestResolver creates a mock ProfileResolver for testing without importing the profile package.
func buildTestResolver(t *testing.T) mcp.ProfileResolver {
	t.Helper()
	return &mockProfileResolver{
		overrides: map[string]map[string]string{
			"test-override": {
				"find_symbol": "Find a symbol with overridden description",
			},
		},
	}
}
