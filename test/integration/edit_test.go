//go:build integration

package integration_test

import (
	"strings"
	"testing"
)

// TestEdit_ReplaceBody exercises the replace_symbol_body tool (EDIT-01).
// Round-trip: read symbol body, replace it, re-read, confirm new body.
func TestEdit_ReplaceBody(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

	// Verify Helper exists in overview.
	overview := callTool(t, td.Session, "get_symbol_overview", map[string]any{
		"path": "main.go",
	})
	text := textContent(overview)
	if !strings.Contains(text, "Helper") {
		t.Fatalf("expected symbol overview to contain Helper, got: %s", text)
	}

	// Replace Helper's body.
	callTool(t, td.Session, "replace_symbol_body", map[string]any{
		"path":        "main.go",
		"symbol_name": "Helper",
		"new_body":    "{\n\tfmt.Println(\"Modified Helper\")\n}",
	})

	// Re-read and verify.
	readResult := callTool(t, td.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	content := textContent(readResult)
	if !strings.Contains(content, "Modified Helper") {
		t.Errorf("expected file to contain 'Modified Helper' after replace, got:\n%s", content)
	}

	// Symbol name should still exist (only body changed).
	overview2 := callTool(t, td.Session, "get_symbol_overview", map[string]any{
		"path": "main.go",
	})
	if !strings.Contains(textContent(overview2), "Helper") {
		t.Error("expected Helper symbol to still exist after body replacement")
	}
}

// TestEdit_InsertBeforeAfter exercises insert_before_symbol and insert_after_symbol (EDIT-02).
// Each insert uses a fresh fixture to avoid stale LS symbol positions after file mutation.
func TestEdit_InsertBeforeAfter(t *testing.T) {
	requireGopls(t)

	t.Run("insert_before", func(t *testing.T) {
		fixture := PrepareFixture(t, "go")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

		callTool(t, td.Session, "insert_before_symbol", map[string]any{
			"path":        "main.go",
			"symbol_name": "Helper",
			"content":     "// BEFORE_MARKER\n",
		})

		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.go",
		})
		content := textContent(readResult)
		if !strings.Contains(content, "BEFORE_MARKER") {
			t.Fatalf("expected BEFORE_MARKER after insert_before_symbol, got:\n%s", content)
		}

		// BEFORE_MARKER should appear before "func Helper".
		beforeIdx := strings.Index(content, "BEFORE_MARKER")
		helperIdx := strings.Index(content, "func Helper")
		if beforeIdx >= helperIdx {
			t.Errorf("BEFORE_MARKER (pos %d) should appear before func Helper (pos %d)", beforeIdx, helperIdx)
		}
	})

	t.Run("insert_after", func(t *testing.T) {
		fixture := PrepareFixture(t, "go")
		td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

		// Warm up: ensure LS has indexed symbols before mutation.
		callTool(t, td.Session, "get_symbol_overview", map[string]any{
			"path": "main.go",
		})

		callTool(t, td.Session, "insert_after_symbol", map[string]any{
			"path":        "main.go",
			"symbol_name": "Helper",
			"content":     "// AFTER_MARKER",
		})

		readResult := callTool(t, td.Session, "read_file", map[string]any{
			"path": "main.go",
		})
		content := textContent(readResult)
		t.Logf("File after insert_after:\n%s", content)
		if !strings.Contains(content, "AFTER_MARKER") {
			t.Fatalf("expected AFTER_MARKER after insert_after_symbol, got:\n%s", content)
		}

		// AFTER_MARKER should appear after the Helper function closing brace.
		helperIdx := strings.Index(content, "func Helper")
		afterIdx := strings.Index(content, "AFTER_MARKER")
		if afterIdx <= helperIdx {
			t.Errorf("AFTER_MARKER (pos %d) should appear after func Helper (pos %d)", afterIdx, helperIdx)
		}
	})
}

// TestEdit_RenameCrossFile exercises rename_symbol across files (EDIT-03).
func TestEdit_RenameCrossFile(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

	// Rename Helper to RenamedHelper.
	// Helper is defined at line 11 (1-indexed), column 6 (1-indexed: "func H..." -> H is col 6).
	callTool(t, td.Session, "rename_symbol", map[string]any{
		"path":     "main.go",
		"line":     11,
		"column":   6,
		"new_name": "RenamedHelper",
	})

	// Verify main.go contains RenamedHelper and not standalone "func Helper()".
	readResult := callTool(t, td.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	content := textContent(readResult)
	if !strings.Contains(content, "RenamedHelper") {
		t.Errorf("expected RenamedHelper in main.go after rename, got:\n%s", content)
	}
	if strings.Contains(content, "func Helper()") {
		t.Errorf("expected 'func Helper()' to be renamed, but it still exists in:\n%s", content)
	}

	// Verify UsingHelper's body now calls RenamedHelper (cross-reference updated).
	if !strings.Contains(content, "RenamedHelper()") {
		t.Errorf("expected UsingHelper body to call RenamedHelper(), got:\n%s", content)
	}
}

// TestEdit_SafeDelete exercises safe_delete_symbol for both blocked and success paths (EDIT-04).
func TestEdit_SafeDelete(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

	// Block path: Helper is referenced by main() and UsingHelper, so delete should be blocked.
	// safe_delete_symbol returns a normal (non-error) result with reference info when blocked.
	blockResult := callTool(t, td.Session, "safe_delete_symbol", map[string]any{
		"path":        "main.go",
		"symbol_name": "Helper",
	})
	blockText := textContent(blockResult)
	if !strings.Contains(strings.ToLower(blockText), "reference") {
		t.Errorf("expected blocked delete to mention references, got: %s", blockText)
	}

	// Verify Helper still exists after blocked delete.
	readResult := callTool(t, td.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	if !strings.Contains(textContent(readResult), "func Helper()") {
		t.Error("Helper should still exist after blocked delete")
	}

	// Succeed path: UnusedFunc has no references, should delete successfully.
	deleteResult := callTool(t, td.Session, "safe_delete_symbol", map[string]any{
		"path":        "main.go",
		"symbol_name": "UnusedFunc",
	})
	t.Logf("safe_delete UnusedFunc result: %s", textContent(deleteResult))

	// Verify UnusedFunc is gone.
	readResult2 := callTool(t, td.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	content2 := textContent(readResult2)
	if strings.Contains(content2, "func UnusedFunc()") {
		t.Error("expected UnusedFunc function to be deleted, but it still exists")
	}
}

// TestEdit_VerifyEdit exercises the verify_edit tool after a valid replacement.
func TestEdit_VerifyEdit(t *testing.T) {
	requireGopls(t)

	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture})

	// Replace Helper's body with valid Go code.
	callTool(t, td.Session, "replace_symbol_body", map[string]any{
		"path":        "main.go",
		"symbol_name": "Helper",
		"new_body":    "{\n\tfmt.Println(\"Verified Helper\")\n}",
	})

	// Run verify_edit -- should report no errors for valid edit.
	verifyResult := callTool(t, td.Session, "verify_edit", map[string]any{
		"path": "main.go",
	})
	verifyText := textContent(verifyResult)
	t.Logf("verify_edit result: %s", verifyText)

	// A valid edit should not produce error diagnostics.
	if strings.Contains(strings.ToLower(verifyText), "error") &&
		!strings.Contains(strings.ToLower(verifyText), "no error") {
		t.Errorf("expected clean verify_edit after valid replacement, got: %s", verifyText)
	}
}
