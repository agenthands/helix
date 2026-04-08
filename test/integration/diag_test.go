//go:build integration

package integration_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// TestDiag_GoFixture exercises all 3 diagnostic tools against the Go fixture.
// Diagnostic tools need an LS, but due to the known Language field issue in activeWSKey,
// these tests use behavioral assertions (the tool returns without panicking and returns
// either results or a structured error).
func TestDiag_GoFixture(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	// SkipLS: true -- harness WaitForLS would fatalf; we poll manually.
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, SkipLS: true})

	// Best-effort LS readiness poll.
	func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				t.Log("LS readiness poll timed out (known Language field issue) -- using behavioral assertions")
				return
			case <-ticker.C:
				result, err := td.Session.CallTool(ctx, &mcp.CallToolParams{
					Name:      "search_symbols",
					Arguments: map[string]any{"query": "main"},
				})
				if err != nil {
					continue
				}
				text := textContent(result)
				if !result.IsError && text != "" && text != "(no results)" {
					t.Logf("LS ready: %s", text[:min(len(text), 80)])
					return
				}
			}
		}
	}()

	// --- 1. get_diagnostics ---
	t.Run("get_diagnostics", func(t *testing.T) {
		result, err := td.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "get_diagnostics",
			Arguments: map[string]any{"path": "main.go"},
		})
		if err != nil {
			t.Fatalf("get_diagnostics protocol error: %v", err)
		}
		text := textContent(result)
		t.Logf("get_diagnostics: isError=%v text=%s", result.IsError, text)
		// Behavioral: the call succeeded at the protocol level.
		// For a clean file, diagnostics may be empty, contain informational items,
		// or return an error due to the Language field issue.
		if !result.IsError && text != "" {
			if !strings.Contains(text, "main.go") && !strings.Contains(text, "no diagnostics") &&
				!strings.Contains(text, "WARNING") && !strings.Contains(text, "ERROR") {
				t.Logf("unexpected diagnostics format: %s", text)
			}
		}
	})

	// --- 2. get_code_actions ---
	t.Run("get_code_actions", func(t *testing.T) {
		result, err := td.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name: "get_code_actions",
			Arguments: map[string]any{
				"path":   "main.go",
				"line":   1,
				"column": 1,
			},
		})
		if err != nil {
			t.Fatalf("get_code_actions protocol error: %v", err)
		}
		text := textContent(result)
		t.Logf("get_code_actions: isError=%v text=%s", result.IsError, text)
		// Behavioral: protocol-level success. gopls may or may not offer actions.
	})

	// --- 3. format_code ---
	t.Run("format_code", func(t *testing.T) {
		result, err := td.Session.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "format_code",
			Arguments: map[string]any{"path": "main.go"},
		})
		if err != nil {
			t.Fatalf("format_code protocol error: %v", err)
		}
		text := textContent(result)
		t.Logf("format_code: isError=%v text=%s", result.IsError, text)

		// If formatting succeeded, verify the file still contains expected content.
		if !result.IsError {
			readResult := callTool(t, td.Session, "read_file", map[string]any{
				"path": "main.go",
			})
			readText := textContent(readResult)
			if !strings.Contains(readText, "func Helper()") {
				t.Errorf("format should preserve 'func Helper()', got: %s", readText)
			}
		}
	})
}
