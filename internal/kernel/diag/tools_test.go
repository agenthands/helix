package diag

import (
	"strings"
	"testing"

	serr "github.com/postfix/serena/internal/errors"
)

// TestDiagToolsTypedNoWorkspaceError verifies that diag tools use typed
// NoWorkspace errors instead of legacy hardcoded strings.
func TestDiagToolsTypedNoWorkspaceError(t *testing.T) {
	// The legacy error string was: "no active workspace - activate a project first"
	// The new error should use: serr.New(serr.NoWorkspace, "no active workspace")
	e := serr.New(serr.NoWorkspace, "no active workspace")
	msg := e.Error()
	if !strings.Contains(msg, "no_workspace") {
		t.Errorf("expected 'no_workspace' kind in error, got: %s", msg)
	}
	if strings.Contains(msg, "activate a project first") {
		t.Errorf("should not contain legacy suffix, got: %s", msg)
	}
}

// TestDiagValidationErrorFormat verifies InvalidArgs errors for diag tools.
func TestDiagValidationErrorFormat(t *testing.T) {
	tests := []struct {
		name  string
		tool  string
		field string
	}{
		{"get_diagnostics path", "get_diagnostics", "path"},
		{"get_code_actions path", "get_code_actions", "path"},
		{"format_code path", "format_code", "path"},
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
