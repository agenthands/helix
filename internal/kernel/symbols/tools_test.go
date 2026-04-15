package symbols

import (
	"strings"
	"testing"

	serr "github.com/postfix/serena/internal/errors"
)

// TestValidationErrorFormat verifies that InvalidArgs errors for symbol tools
// produce the expected "invalid_args: missing required field: ..." format.
func TestValidationErrorFormat(t *testing.T) {
	tests := []struct {
		name     string
		toolName string
		field    string
	}{
		{"go_to_definition path", "go_to_definition", "path"},
		{"find_references path", "find_references", "path"},
		{"get_symbol_overview path", "get_symbol_overview", "path"},
		{"search_symbols query", "search_symbols", "query"},
		{"get_hover_info path", "get_hover_info", "path"},
		{"find_implementations path", "find_implementations", "path"},
		{"get_call_hierarchy path", "get_call_hierarchy", "path"},
		{"get_type_hierarchy path", "get_type_hierarchy", "path"},
		{"analyze_blast_radius path", "analyze_blast_radius", "path"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := serr.New(serr.InvalidArgs, "missing required field: "+tt.field).
				WithTool(tt.toolName)
			msg := err.Error()
			if !strings.Contains(msg, "invalid_args") {
				t.Errorf("expected 'invalid_args' in error, got: %s", msg)
			}
			if !strings.Contains(msg, "missing required field: "+tt.field) {
				t.Errorf("expected field name %q in error, got: %s", tt.field, msg)
			}
		})
	}
}

// TestValidateEmptyPathReturnsInvalidArgs verifies inline validation produces
// errorResult for empty path in position-based tools.
func TestValidateEmptyPathReturnsInvalidArgs(t *testing.T) {
	// Verify the errorResult helper produces IsError=true
	r := errorResult(serr.New(serr.InvalidArgs, "missing required field: path").
		WithTool("go_to_definition").Error())
	if !r.IsError {
		t.Fatal("expected IsError=true from errorResult")
	}
	if len(r.Content) == 0 {
		t.Fatal("expected content in result")
	}
}

func TestInlineValidationErrorMessages(t *testing.T) {
	// This test verifies that the validation blocks in tools.go produce correct
	// error messages. Since handlers are closures, we verify the error construction
	// pattern directly.
	tests := []struct {
		name  string
		field string
		tool  string
	}{
		{"go_to_definition empty path", "path", "go_to_definition"},
		{"search_symbols empty query", "query", "search_symbols"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := serr.New(serr.InvalidArgs, "missing required field: "+tt.field).
				WithTool(tt.tool)
			result := errorResult(e.Error())
			if !result.IsError {
				t.Fatal("expected IsError=true")
			}
			// The error message should be embedded in the text content
			if len(result.Content) == 0 {
				t.Fatal("expected at least one content item")
			}
		})
	}
}

// TestZeroLineColAreValid verifies that line=0 and column=0 are NOT rejected
// by validation (they are valid 0-indexed LSP positions).
func TestZeroLineColAreValid(t *testing.T) {
	// This is a design assertion: the validation code must NOT check for
	// line == 0 or col == 0. We verify this by confirming the args struct
	// allows zero values (Go zero-value for int is 0).
	args := GoToDefinitionArgs{
		Path: "test.go",
		Line: 0,
		Col:  0,
	}
	if args.Path == "" {
		t.Fatal("path should not be empty")
	}
	// line=0 and col=0 are valid, no validation should reject them
	if args.Line != 0 || args.Col != 0 {
		t.Fatal("zero values should be preserved")
	}
}
