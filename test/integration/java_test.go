package integration_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/test/integration/jdtlscache"
)

// TestSymbols_JavaFixture exercises symbol retrieval tools against the Java fixture
// with jdtls as the language server.
func TestSymbols_JavaFixture(t *testing.T) {
	requireLS(t, "jdtls")

	jdtlsPath, err := exec.LookPath("jdtls")
	require.NoError(t, err)
	fixtureSrc := filepath.Join(projectRoot(), "testdata", "fixtures", "java")
	warmDir, err := jdtlscache.ResolveDataDir("java", fixtureSrc, jdtlsPath)
	require.NoError(t, err)

	fixture := PrepareFixture(t, "java")
	td := StartTestDaemon(t, Options{
		WorkspaceDir: fixture,
		JdtlsDataDir: warmDir,
		LSTimeout:    120 * time.Second,
	})

	t.Run("search_symbols", func(t *testing.T) {
		result := callTool(t, td.Session, "search_symbols", map[string]any{
			"query": "Main",
		})
		text := textContent(result)
		t.Logf("search_symbols: %s", text)
		assert.Contains(t, text, "Main")
	})

	t.Run("get_symbol_overview", func(t *testing.T) {
		result := callTool(t, td.Session, "get_symbol_overview", map[string]any{
			"path": "Main.java",
		})
		text := textContent(result)
		t.Logf("get_symbol_overview: %s", text)
		for _, sym := range []string{"helper", "unusedMethod", "usingHelper", "main"} {
			assert.Contains(t, text, sym, "symbol overview should contain %q", sym)
		}
	})

	t.Run("find_references", func(t *testing.T) {
		// helper() defined at line 2 in Main.java (1-indexed)
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "Main.java",
			"line":                2,
			"column":              26,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references: %s", text)
		lines := strings.Split(strings.TrimSpace(text), "\n")
		assert.GreaterOrEqual(t, len(lines), 2, "expected at least 2 reference locations for helper")
	})

	t.Run("get_hover_info", func(t *testing.T) {
		result := callTool(t, td.Session, "get_hover_info", map[string]any{
			"path":   "Main.java",
			"line":   2,
			"column": 26,
		})
		text := textContent(result)
		t.Logf("get_hover_info: %s", text)
		assert.Contains(t, text, "helper")
	})

	t.Run("find_references_cross_file", func(t *testing.T) {
		// Greeter interface at line 1 in Greeter.java
		result := callTool(t, td.Session, "find_references", map[string]any{
			"path":                "Greeter.java",
			"line":                1,
			"column":              11,
			"include_declaration": true,
		})
		text := textContent(result)
		t.Logf("find_references_cross_file: %s", text)
		assert.Contains(t, text, "Main.java", "cross-file reference to Greeter should include Main.java")
	})

	t.Run("find_implementations", func(t *testing.T) {
		// Greeter interface at line 1, col 11 in Greeter.java
		result := callTool(t, td.Session, "find_implementations", map[string]any{
			"path":   "Greeter.java",
			"line":   1,
			"column": 11,
		})
		text := textContent(result)
		t.Logf("find_implementations: %s", text)
		assert.True(t,
			strings.Contains(text, "SimpleGreeter") || strings.Contains(text, "Greeter.java"),
			"expected implementations to mention SimpleGreeter, got: %s", text)
	})
}

// TestEdit_JavaFixture exercises representative edit tools against the Java fixture.
func TestEdit_JavaFixture(t *testing.T) {
	requireLS(t, "jdtls")

	jdtlsPath, err := exec.LookPath("jdtls")
	require.NoError(t, err)
	fixtureSrc := filepath.Join(projectRoot(), "testdata", "fixtures", "java")
	warmDir, err := jdtlscache.ResolveDataDir("java", fixtureSrc, jdtlsPath)
	require.NoError(t, err)

	t.Run("replace_body", func(t *testing.T) {
		fixture := PrepareFixture(t, "java")
		td := StartTestDaemon(t, Options{
			WorkspaceDir: fixture,
			JdtlsDataDir: warmDir,
			LSTimeout:    120 * time.Second,
		})

		result := callTool(t, td.Session, "replace_symbol_body", map[string]any{
			"path":        "Main.java",
			"symbol_name": "helper",
			"new_body":    `return "replaced";`,
		})
		text := textContent(result)
		t.Logf("replace_body: %s", text)
		assert.Contains(t, text, "Replaced")

		// Verify the replacement via read_file
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "Main.java",
		})
		readText := textContent(readResult)
		assert.Contains(t, readText, `"replaced"`, "file should contain new body")
	})

	t.Run("rename", func(t *testing.T) {
		fixture := PrepareFixture(t, "java")
		td := StartTestDaemon(t, Options{
			WorkspaceDir: fixture,
			JdtlsDataDir: warmDir,
			LSTimeout:    120 * time.Second,
		})

		// Rename helper to renamedHelper (line 2, col 26 in Main.java, 1-indexed)
		result := callTool(t, td.Session, "rename_symbol", map[string]any{
			"path":     "Main.java",
			"line":     2,
			"column":   26,
			"new_name": "renamedHelper",
		})
		text := textContent(result)
		t.Logf("rename: %s", text)

		// Verify cross-reference updated in usingHelper
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "Main.java",
		})
		readText := textContent(readResult)
		assert.Contains(t, readText, "renamedHelper", "usingHelper should reference renamedHelper after rename")
	})
}
