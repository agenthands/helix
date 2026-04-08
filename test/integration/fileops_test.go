//go:build integration

package integration_test

import (
	"strings"
	"testing"
)

// TestFileOps_GoFixture exercises all 6 file operation tools against the Go fixture.
// File ops do NOT need a language server -- they work against the filesystem directly.
func TestFileOps_GoFixture(t *testing.T) {
	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, SkipLS: true})

	// --- 1. read_file ---
	t.Run("read_file", func(t *testing.T) {
		result := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.go",
		})
		text := textContent(result)
		if !strings.Contains(text, "func Helper()") {
			t.Errorf("expected read_file to contain 'func Helper()', got: %s", text)
		}
		if !strings.Contains(text, "package main") {
			t.Errorf("expected read_file to contain 'package main', got: %s", text)
		}
	})

	// --- 2. create_file ---
	t.Run("create_file", func(t *testing.T) {
		result := callTool(t, td.Session, "create_file", map[string]any{
			"path":    "newfile.go",
			"content": "package main\n\nfunc NewFunc() {}\n",
		})
		text := textContent(result)
		if !strings.Contains(text, "created") {
			t.Errorf("expected create_file confirmation, got: %s", text)
		}

		// Verify by reading back.
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "newfile.go",
		})
		readText := textContent(readResult)
		if !strings.Contains(readText, "func NewFunc()") {
			t.Errorf("expected created file to contain 'func NewFunc()', got: %s", readText)
		}
	})

	// --- 3. list_directory ---
	t.Run("list_directory", func(t *testing.T) {
		result := callTool(t, td.Session, "list_directory", map[string]any{
			"path": ".",
		})
		text := textContent(result)
		if !strings.Contains(text, "main.go") {
			t.Errorf("expected list_directory to contain 'main.go', got: %s", text)
		}
		if !strings.Contains(text, "pkg") {
			t.Errorf("expected list_directory to contain 'pkg', got: %s", text)
		}
	})

	// --- 4. find_files ---
	t.Run("find_files", func(t *testing.T) {
		result := callTool(t, td.Session, "find_files", map[string]any{
			"pattern": "**/*greeter*",
		})
		text := textContent(result)
		if !strings.Contains(text, "greeter.go") {
			t.Errorf("expected find_files to find 'greeter.go', got: %s", text)
		}
	})

	// --- 5. search_in_files ---
	t.Run("search_in_files", func(t *testing.T) {
		result := callTool(t, td.Session, "search_in_files", map[string]any{
			"pattern": "func Helper",
		})
		text := textContent(result)
		if !strings.Contains(text, "main.go") {
			t.Errorf("expected search_in_files to find match in main.go, got: %s", text)
		}
		if !strings.Contains(text, "Helper") {
			t.Errorf("expected search_in_files to contain 'Helper', got: %s", text)
		}
	})

	// --- 6. replace_in_file ---
	t.Run("replace_in_file", func(t *testing.T) {
		// Replace in the copied fixture (t.TempDir copy, not source).
		result := callTool(t, td.Session, "replace_in_file", map[string]any{
			"path":        "main.go",
			"pattern":     "Helper function called",
			"replacement": "Helper was invoked",
		})
		text := textContent(result)
		if !strings.Contains(text, "replacement") {
			t.Logf("replace_in_file result: %s", text)
		}

		// Verify replacement by reading back.
		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.go",
		})
		readText := textContent(readResult)
		if !strings.Contains(readText, "Helper was invoked") {
			t.Errorf("expected replaced text 'Helper was invoked', got: %s", readText)
		}
		if strings.Contains(readText, "Helper function called") {
			t.Errorf("old text 'Helper function called' should have been replaced, got: %s", readText)
		}
	})
}
