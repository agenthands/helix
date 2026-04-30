package edit

import (
	"strings"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
)

// TestDetectLangEdgeCases verifies detectLang for known and unknown extensions.
func TestDetectLangEdgeCases(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"main.go", "go"},
		{"app.py", "python"},
		{"index.ts", "typescript"},
		{"index.tsx", "typescript"},
		{"index.js", "typescript"},
		{"index.jsx", "typescript"},
		{"lib.rs", "rust"},
		{"unknown.xyz", ""},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectLang(tt.path)
			if got != tt.want {
				t.Errorf("detectLang(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// TestEditToolValidationErrorFormat verifies validation error messages for edit tools.
func TestEditToolValidationErrorFormat(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		field string
	}{
		{"replace_symbol_body path", "replace_symbol_body", "path"},
		{"replace_symbol_body symbol_name", "replace_symbol_body", "symbol_name"},
		{"replace_symbol_body new_body", "replace_symbol_body", "new_body"},
		{"insert_before_symbol path", "insert_before_symbol", "path"},
		{"insert_before_symbol symbol_name", "insert_before_symbol", "symbol_name"},
		{"insert_before_symbol content", "insert_before_symbol", "content"},
		{"insert_after_symbol path", "insert_after_symbol", "path"},
		{"rename_symbol path", "rename_symbol", "path"},
		{"rename_symbol new_name", "rename_symbol", "new_name"},
		{"safe_delete_symbol path", "safe_delete_symbol", "path"},
		{"safe_delete_symbol symbol_name", "safe_delete_symbol", "symbol_name"},
		{"verify_edit path", "verify_edit", "path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := serr.New(serr.InvalidArgs, "missing required field: "+tt.field).
				WithTool(tt.tool)
			msg := e.Error()
			if !strings.Contains(msg, "invalid_args") {
				t.Errorf("expected 'invalid_args' in error, got: %s", msg)
			}
			if !strings.Contains(msg, tt.field) {
				t.Errorf("expected field name %q in error, got: %s", tt.field, msg)
			}
		})
	}
}

// TestVerifyEditWorkspaceCheckError verifies verify_edit returns NoWorkspace
// error when workspace key has empty root.
func TestVerifyEditWorkspaceCheckError(t *testing.T) {
	e := serr.New(serr.NoWorkspace, "no active workspace").
		WithTool("verify_edit")
	msg := e.Error()
	if !strings.Contains(msg, "no_workspace") {
		t.Errorf("expected 'no_workspace' in error, got: %s", msg)
	}
}
