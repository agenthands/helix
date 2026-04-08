//go:build integration

package integration_test

import (
	"strings"
	"testing"
)

func TestHarness_StartAndCallTool(t *testing.T) {
	td := StartTestDaemon(t, Options{SkipLS: true})
	result := callTool(t, td.Session, "list_memories", map[string]any{})
	if result == nil {
		t.Fatal("expected non-nil result from list_memories")
	}
}

func TestHarness_FixtureActivateAndReadFile(t *testing.T) {
	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, SkipLS: true})

	// Verify workspace activation worked by reading a fixture file via MCP.
	result := callTool(t, td.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	text := textContent(result)
	if !strings.Contains(text, "Helper") {
		t.Fatalf("expected read_file result to contain 'Helper', got: %s", text)
	}
	if !strings.Contains(text, "DemoStruct") {
		t.Fatalf("expected read_file result to contain 'DemoStruct', got: %s", text)
	}
}
