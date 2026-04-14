//go:build integration

package integration_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSymbols_PythonFixture exercises symbol retrieval tools against the Python fixture
// with pyright-langserver.
func TestSymbols_PythonFixture(t *testing.T) {
	requireLS(t, "pyright-langserver")

	fixture := PrepareFixture(t, "python")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 90 * time.Second, LSQuery: "helper"})

	t.Run("search_symbols", func(t *testing.T) {
		result := callTool(t, td.Session, "search_symbols", map[string]any{
			"query": "Demo",
		})
		text := textContent(result)
		t.Logf("search_symbols: %s", text)
		assert.Contains(t, text, "DemoClass")
	})

	t.Run("get_symbol_overview", func(t *testing.T) {
		result := callTool(t, td.Session, "get_symbol_overview", map[string]any{
			"path": "main.py",
		})
		text := textContent(result)
		t.Logf("get_symbol_overview: %s", text)
		for _, sym := range []string{"helper", "DemoClass", "unused_func", "using_helper"} {
			assert.Contains(t, text, sym, "symbol overview should contain %q", sym)
		}
	})

	t.Run("find_references", func(t *testing.T) {
		// helper() defined at line 3 (0-indexed), col 4 in main.py
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "main.py",
			"line":                3,
			"column":              4,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references: %s", text)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		assert.GreaterOrEqual(t, len(lines), 2, "expected at least 2 reference locations (def + call in using_helper)")
	})

	t.Run("get_hover_info", func(t *testing.T) {
		result := callTool(t, td.Session, "get_hover_info", map[string]any{
			"path":   "main.py",
			"line":   3,
			"column": 4,
		})
		text := textContent(result)
		t.Logf("get_hover_info: %s", text)
		assert.Contains(t, text, "helper")
	})

	t.Run("find_references_cross_file", func(t *testing.T) {
		// greet() defined at line 0 (0-indexed), col 4 in utils.py
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "utils.py",
			"line":                0,
			"column":              4,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references_cross_file: %s", text)
		assert.Contains(t, text, "main.py", "cross-file reference should include main.py")
	})
}

// TestEdit_PythonFixture exercises representative edit operations against the Python fixture.
func TestEdit_PythonFixture(t *testing.T) {
	requireLS(t, "pyright-langserver")

	t.Run("replace_body", func(t *testing.T) {
		fixture := PrepareFixture(t, "python")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 90 * time.Second, LSQuery: "helper"})

		// Replace helper() body
		callTool(t, td.Session, "replace_symbol_body", map[string]any{
			"path":        "main.py",
			"symbol_name": "helper",
			"new_body":    "    return \"replaced\"",
		})

		// Verify via read_file
		result := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.py",
		})
		text := textContent(result)
		t.Logf("read_file after replace: %s", text)
		assert.Contains(t, text, "replaced")
	})

	t.Run("rename", func(t *testing.T) {
		fixture := PrepareFixture(t, "python")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 90 * time.Second, LSQuery: "helper"})

		// Rename helper to renamed_helper (line 4, col 5 -- 1-indexed for rename_symbol)
		callTool(t, td.Session, "rename_symbol", map[string]any{
			"path":     "main.py",
			"line":     4,
			"column":   5,
			"new_name": "renamed_helper",
		})

		// Verify rename propagated
		result := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.py",
		})
		text := textContent(result)
		t.Logf("read_file after rename: %s", text)
		assert.Contains(t, text, "renamed_helper")
		assert.Contains(t, text, "renamed_helper()", "using_helper should reference renamed_helper")
	})
}
