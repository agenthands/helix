//go:build integration

package integration_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestSymbols_TypeScriptFixture exercises symbol retrieval tools against the TypeScript fixture
// with typescript-language-server.
func TestSymbols_TypeScriptFixture(t *testing.T) {
	t.Skip("typescript-language-server requires per-file didOpen for textDocument/* operations; needs multi-file open support in quirks adapter")
	requireLS(t, "typescript-language-server")

	fixture := PrepareFixture(t, "typescript")
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
			"path": "main.ts",
		})
		text := textContent(result)
		t.Logf("get_symbol_overview: %s", text)
		for _, sym := range []string{"helper", "DemoClass", "unusedFunc", "usingHelper"} {
			assert.Contains(t, text, sym, "symbol overview should contain %q", sym)
		}
	})

	t.Run("find_references", func(t *testing.T) {
		// helper() defined at line 2 (0-indexed), col 16 in main.ts
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "main.ts",
			"line":                2,
			"column":              16,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references: %s", text)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		assert.GreaterOrEqual(t, len(lines), 2, "expected at least 2 reference locations (def + call in usingHelper)")
	})

	t.Run("get_hover_info", func(t *testing.T) {
		result := callTool(t, td.Session, "get_hover_info", map[string]any{
			"path":   "main.ts",
			"line":   2,
			"column": 16,
		})
		text := textContent(result)
		t.Logf("get_hover_info: %s", text)
		assert.Contains(t, text, "helper")
	})

	t.Run("find_references_cross_file", func(t *testing.T) {
		// Greeter class defined at line 4 (0-indexed), col 13 in greeter.ts
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "greeter.ts",
			"line":                4,
			"column":              13,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references_cross_file: %s", text)
		assert.Contains(t, text, "main.ts", "cross-file reference should include main.ts")
	})

	t.Run("find_implementations", func(t *testing.T) {
		// IGreeter interface at line 0 (0-indexed), col 17 in greeter.ts
		result := callTool(t, td.Session, "find_implementations", map[string]any{
			"path":   "greeter.ts",
			"line":   0,
			"column": 17,
		})
		text := textContent(result)
		t.Logf("find_implementations: %s", text)
		assert.True(t,
			strings.Contains(text, "Greeter") || strings.Contains(text, "greeter.ts"),
			"expected implementations to mention Greeter or greeter.ts, got: %s", text)
	})
}

// TestEdit_TypeScriptFixture exercises representative edit operations against the TypeScript fixture.
func TestEdit_TypeScriptFixture(t *testing.T) {
	t.Skip("typescript-language-server requires per-file didOpen for textDocument/* operations; needs multi-file open support in quirks adapter")
	requireLS(t, "typescript-language-server")

	t.Run("replace_body", func(t *testing.T) {
		fixture := PrepareFixture(t, "typescript")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 90 * time.Second, LSQuery: "helper"})

		// Replace helper() body
		callTool(t, td.Session, "replace_symbol_body", map[string]any{
			"path":        "main.ts",
			"symbol_name": "helper",
			"new_body":    "    return \"replaced\";",
		})

		// Verify via read_file
		result := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.ts",
		})
		text := textContent(result)
		t.Logf("read_file after replace: %s", text)
		assert.Contains(t, text, "replaced")
	})

	t.Run("rename", func(t *testing.T) {
		fixture := PrepareFixture(t, "typescript")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture, LSTimeout: 90 * time.Second, LSQuery: "helper"})

		// Rename helper to renamedHelper (line 2, col 16 -- 0-indexed def position)
		// rename_symbol uses 1-indexed line/column
		callTool(t, td.Session, "rename_symbol", map[string]any{
			"path":     "main.ts",
			"line":     3,
			"column":   17,
			"new_name": "renamedHelper",
		})

		// Verify rename propagated
		result := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.ts",
		})
		text := textContent(result)
		t.Logf("read_file after rename: %s", text)
		assert.Contains(t, text, "renamedHelper")
		assert.Contains(t, text, "renamedHelper()", "usingHelper should reference renamedHelper")
	})
}
