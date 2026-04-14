//go:build integration

package integration_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSymbols_RustFixture exercises symbol retrieval tools against the Rust fixture
// with rust-analyzer as the language server.
func TestSymbols_RustFixture(t *testing.T) {
	requireLS(t, "rust-analyzer")

	fixture := PrepareFixture(t, "rust")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 45 * time.Second, LSQuery: "helper"})

	t.Run("search_symbols", func(t *testing.T) {
		result := callTool(t, td.Session, "search_symbols", map[string]any{
			"query": "Demo",
		})
		text := textContent(result)
		t.Logf("search_symbols: %s", text)
		assert.Contains(t, text, "DemoStruct")
	})

	t.Run("get_symbol_overview", func(t *testing.T) {
		result := callTool(t, td.Session, "get_symbol_overview", map[string]any{
			"path": "src/main.rs",
		})
		text := textContent(result)
		t.Logf("get_symbol_overview: %s", text)
		for _, sym := range []string{"helper", "DemoStruct", "unused_func", "using_helper"} {
			assert.Contains(t, text, sym, "symbol overview should contain %q", sym)
		}
	})

	t.Run("find_references", func(t *testing.T) {
		// helper() defined at line 3, col 3 in src/main.rs (0-indexed)
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "src/main.rs",
			"line":                3,
			"column":              3,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references: %s", text)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		assert.GreaterOrEqual(t, len(lines), 2, "expected at least 2 reference locations for helper")
	})

	t.Run("get_hover_info", func(t *testing.T) {
		result := callTool(t, td.Session, "get_hover_info", map[string]any{
			"path":   "src/main.rs",
			"line":   3,
			"column": 3,
		})
		text := textContent(result)
		t.Logf("get_hover_info: %s", text)
		assert.Contains(t, text, "helper")
	})

	t.Run("find_references_cross_file", func(t *testing.T) {
		// Greeter trait at line 0, col 10 in src/greeter.rs (0-indexed)
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "src/greeter.rs",
			"line":                0,
			"column":              10,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references_cross_file: %s", text)
		assert.Contains(t, text, "main.rs", "cross-file reference to Greeter should include main.rs")
	})

	t.Run("find_implementations", func(t *testing.T) {
		// Greeter trait at line 0, col 10 in src/greeter.rs (0-indexed)
		result := callTool(t, td.Session, "find_implementations", map[string]any{
			"path":   "src/greeter.rs",
			"line":   0,
			"column": 10,
		})
		text := textContent(result)
		t.Logf("find_implementations: %s", text)
		assert.True(t,
			strings.Contains(text, "SimpleGreeter") || strings.Contains(text, "greeter.rs"),
			"expected implementations to mention SimpleGreeter, got: %s", text)
	})
}

// TestEdit_RustFixture exercises representative edit tools against the Rust fixture.
func TestEdit_RustFixture(t *testing.T) {
	requireLS(t, "rust-analyzer")

	t.Run("replace_body", func(t *testing.T) {
		fixture := PrepareFixture(t, "rust")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 45 * time.Second, LSQuery: "helper"})

		result := callTool(t, td.Session, "replace_symbol_body", map[string]any{
			"path":        "src/main.rs",
			"symbol_name": "helper",
			"new_body":    `"replaced".to_string()`,
		})
		text := textContent(result)
		t.Logf("replace_body: %s", text)
		assert.Contains(t, text, "Replaced")

		// Verify the replacement via read_file
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "src/main.rs",
		})
		readText := textContent(readResult)
		assert.Contains(t, readText, `"replaced"`, "file should contain new body")
	})

	t.Run("rename", func(t *testing.T) {
		t.Skip("rust-analyzer returns 'file not found' for textDocument/rename in temp workspaces; needs prepareRename support investigation")

		fixture := PrepareFixture(t, "rust")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 45 * time.Second, LSQuery: "helper"})

		// Rename helper to renamed_helper (line 4, col 4 in src/main.rs, 1-indexed)
		result := callTool(t, td.Session, "rename_symbol", map[string]any{
			"path":     "src/main.rs",
			"line":     4,
			"column":   4,
			"new_name": "renamed_helper",
		})
		text := textContent(result)
		t.Logf("rename: %s", text)

		// Verify cross-reference updated
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "src/main.rs",
		})
		readText := textContent(readResult)
		assert.Contains(t, readText, "renamed_helper", "using_helper should reference renamed_helper after rename")
	})
}
