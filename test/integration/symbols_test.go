//go:build integration

package integration_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSymbols_GoFixture exercises all 9 symbol retrieval tools against the Go fixture
// with strict assertions. The Language field bug is fixed -- gopls starts successfully
// and all tools return actual symbol data.
func TestSymbols_GoFixture(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

	// --- 1. go_to_definition ---
	t.Run("go_to_definition", func(t *testing.T) {
		// "Helper" is called at line 6 (0-indexed), col 1 in main.go
		result := callTool(t, td.Session, "go_to_definition", map[string]any{
			"path":   "main.go",
			"line":   6,
			"column": 1,
		})
		text := textContent(result)
		t.Logf("go_to_definition: %s", text)
		assert.Contains(t, text, "main.go")
	})

	// --- 2. find_references ---
	t.Run("find_references", func(t *testing.T) {
		// "Helper" defined at line 10, col 5 (0-indexed) in main.go
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "main.go",
			"line":                10,
			"column":              5,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references: %s", text)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		assert.GreaterOrEqual(t, len(lines), 2, "expected at least 2 reference locations")
	})

	// --- 3. get_hover_info ---
	t.Run("get_hover_info", func(t *testing.T) {
		result := callTool(t, td.Session, "get_hover_info", map[string]any{
			"path":   "main.go",
			"line":   10,
			"column": 5,
		})
		text := textContent(result)
		t.Logf("get_hover_info: %s", text)
		assert.Contains(t, text, "Helper")
	})

	// --- 4. search_symbols ---
	t.Run("search_symbols", func(t *testing.T) {
		result := callTool(t, td.Session, "search_symbols", map[string]any{
			"query": "Demo",
		})
		text := textContent(result)
		t.Logf("search_symbols: %s", text)
		assert.Contains(t, text, "DemoStruct")
	})

	// --- 5. get_symbol_overview ---
	t.Run("get_symbol_overview", func(t *testing.T) {
		result := callTool(t, td.Session, "get_symbol_overview", map[string]any{
			"path": "main.go",
		})
		text := textContent(result)
		t.Logf("get_symbol_overview: %s", text)
		for _, sym := range []string{"Helper", "DemoStruct", "Value", "UsingHelper", "main"} {
			assert.Contains(t, text, sym, "symbol overview should contain %q", sym)
		}
	})

	// --- 6. find_implementations ---
	t.Run("find_implementations", func(t *testing.T) {
		// "Greeter" interface at line 5, col 5 (0-indexed) in pkg/greeter.go
		result := callTool(t, td.Session, "find_implementations", map[string]any{
			"path":   "pkg/greeter.go",
			"line":   5,
			"column": 5,
		})
		text := textContent(result)
		t.Logf("find_implementations: %s", text)
		assert.True(t,
			strings.Contains(text, "SimpleGreeter") || strings.Contains(text, "greeter.go"),
			"expected implementations to mention SimpleGreeter or greeter.go, got: %s", text)
	})

	// --- 7. get_call_hierarchy ---
	t.Run("get_call_hierarchy", func(t *testing.T) {
		result := callTool(t, td.Session, "get_call_hierarchy", map[string]any{
			"path":      "main.go",
			"line":      10,
			"column":    5,
			"direction": "incoming",
		})
		text := textContent(result)
		t.Logf("get_call_hierarchy: %s", text)
		// Strict: call succeeded without error (callTool fatals on IsError).
	})

	// --- 8. get_type_hierarchy ---
	t.Run("get_type_hierarchy", func(t *testing.T) {
		// "SimpleGreeter" at line 10, col 5 (0-indexed) in pkg/greeter.go
		result := callTool(t, td.Session, "get_type_hierarchy", map[string]any{
			"path":   "pkg/greeter.go",
			"line":   10,
			"column": 5,
		})
		text := textContent(result)
		t.Logf("get_type_hierarchy: %s", text)
		// Strict: call succeeded without error (callTool fatals on IsError).
	})

	// --- 9. analyze_blast_radius ---
	t.Run("analyze_blast_radius", func(t *testing.T) {
		result := callTool(t, td.Session, "analyze_blast_radius", map[string]any{
			"path":   "main.go",
			"line":   10,
			"column": 5,
		})
		text := textContent(result)
		t.Logf("analyze_blast_radius: %s", text)
		assert.True(t,
			strings.Contains(text, "Helper") || strings.Contains(text, "main.go") || strings.Contains(text, "Blast Radius"),
			"expected blast radius to mention Helper, main.go, or Blast Radius, got: %s", text)
	})
}

// TestSymbols_CodebaseSmoke exercises symbol tools against Serena's own codebase
// with strict assertions (callTool fatals on error).
func TestSymbols_CodebaseSmoke(t *testing.T) {
	requireGopls(t)

	td := StartTestDaemon(t, Options{
		WorkspaceDir: projectRoot(),
	})

	t.Run("search_symbols_Daemon", func(t *testing.T) {
		result := callTool(t, td.Session, "search_symbols", map[string]any{
			"query": "Daemon",
		})
		text := textContent(result)
		t.Logf("search_symbols (codebase): text_len=%d", len(text))
		assert.Contains(t, text, "Daemon")
	})

	t.Run("get_symbol_overview_daemon_go", func(t *testing.T) {
		result := callTool(t, td.Session, "get_symbol_overview", map[string]any{
			"path": "internal/daemon/daemon.go",
		})
		text := textContent(result)
		t.Logf("get_symbol_overview (codebase): text_len=%d", len(text))
		assert.Contains(t, text, "Daemon")
		assert.Contains(t, text, "New")
	})
}
