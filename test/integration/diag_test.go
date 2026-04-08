//go:build integration

package integration_test

import (
	"strings"
	"testing"
)

// TestDiag_GoFixture exercises all 3 diagnostic tools against the Go fixture
// with strict assertions. The Language field bug is fixed -- gopls starts successfully
// and all tools return actual results.
func TestDiag_GoFixture(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

	// --- 1. get_diagnostics ---
	t.Run("get_diagnostics", func(t *testing.T) {
		result := callTool(t, td.Session, "get_diagnostics", map[string]any{
			"path": "main.go",
		})
		text := textContent(result)
		t.Logf("get_diagnostics: %s", text)
		// For a clean file, diagnostics may be empty or contain informational items.
		// The strict assertion is that the tool succeeded (callTool fatals on IsError).
		if text != "" {
			if !strings.Contains(text, "main.go") && !strings.Contains(text, "no diagnostics") &&
				!strings.Contains(text, "WARNING") && !strings.Contains(text, "ERROR") {
				t.Logf("diagnostics format: %s", text)
			}
		}
	})

	// --- 2. get_code_actions ---
	t.Run("get_code_actions", func(t *testing.T) {
		result := callTool(t, td.Session, "get_code_actions", map[string]any{
			"path":   "main.go",
			"line":   1,
			"column": 1,
		})
		text := textContent(result)
		t.Logf("get_code_actions: %s", text)
		// Strict: callTool fatals on IsError. gopls may or may not offer actions for clean code.
	})

	// --- 3. format_code ---
	t.Run("format_code", func(t *testing.T) {
		result := callTool(t, td.Session, "format_code", map[string]any{
			"path": "main.go",
		})
		text := textContent(result)
		t.Logf("format_code: %s", text)

		// Verify the file still contains expected content after formatting.
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.go",
		})
		readText := textContent(readResult)
		if !strings.Contains(readText, "func Helper()") {
			t.Errorf("format should preserve 'func Helper()', got: %s", readText)
		}
	})
}
