package help

import (
	"strings"
	"testing"
)

func TestExtractParamDocs(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{
				"type":        "string",
				"description": "The symbol name to search for",
			},
			"direction": map[string]any{
				"type":        "string",
				"description": "Search direction",
				"enum":        []any{"incoming", "outgoing"},
			},
			"verbose": map[string]any{
				"type":        "boolean",
				"description": "Enable verbose output",
			},
		},
		"required": []any{"name"},
	}

	docs := ExtractParamDocs(schema)
	if len(docs) != 3 {
		t.Fatalf("expected 3 params, got %d", len(docs))
	}

	// Sorted alphabetically: direction, name, verbose
	if docs[0].Name != "direction" {
		t.Errorf("expected first param 'direction', got %q", docs[0].Name)
	}
	if docs[0].Type != "string" {
		t.Errorf("expected type 'string', got %q", docs[0].Type)
	}
	if len(docs[0].EnumValues) != 2 {
		t.Errorf("expected 2 enum values, got %d", len(docs[0].EnumValues))
	}
	if docs[0].Required {
		t.Error("direction should not be required")
	}

	if docs[1].Name != "name" {
		t.Errorf("expected second param 'name', got %q", docs[1].Name)
	}
	if !docs[1].Required {
		t.Error("name should be required")
	}

	if docs[2].Name != "verbose" {
		t.Errorf("expected third param 'verbose', got %q", docs[2].Name)
	}
	if docs[2].Type != "boolean" {
		t.Errorf("expected type 'boolean', got %q", docs[2].Type)
	}
}

func TestExtractParamDocsEmpty(t *testing.T) {
	// Nil input
	if docs := ExtractParamDocs(nil); docs != nil {
		t.Errorf("expected nil for nil input, got %v", docs)
	}

	// Empty map
	if docs := ExtractParamDocs(map[string]any{}); docs != nil {
		t.Errorf("expected nil for empty schema, got %v", docs)
	}

	// Schema without properties
	if docs := ExtractParamDocs(map[string]any{"type": "object"}); docs != nil {
		t.Errorf("expected nil for schema without properties, got %v", docs)
	}
}

func TestFormatHelp(t *testing.T) {
	params := []ParamDoc{
		{Name: "name", Type: "string", Description: "The symbol name", Required: true},
		{Name: "verbose", Type: "boolean", Description: "Verbose mode", Required: false},
		{Name: "direction", Type: "string", Description: "Direction", Required: false, EnumValues: []string{"in", "out"}},
	}

	output := FormatHelp("find_symbol", "Find a symbol in the codebase", params, "## Examples\n\nCall with {\"name\": \"MyClass\"}")

	if !strings.Contains(output, "# find_symbol") {
		t.Error("expected tool name header")
	}
	if !strings.Contains(output, "Find a symbol in the codebase") {
		t.Error("expected full description")
	}
	if !strings.Contains(output, "## Parameters") {
		t.Error("expected parameters section")
	}
	if !strings.Contains(output, "**name** (string, required): The symbol name") {
		t.Error("expected name parameter with required")
	}
	if !strings.Contains(output, "**verbose** (boolean, optional): Verbose mode") {
		t.Error("expected verbose parameter with optional")
	}
	if !strings.Contains(output, "[values: in, out]") {
		t.Error("expected enum values for direction")
	}
	if !strings.Contains(output, "## Examples") {
		t.Error("expected help text section")
	}
}

func TestFormatHelpNoParams(t *testing.T) {
	output := FormatHelp("ping", "Echo a message back", nil, "")
	if !strings.Contains(output, "# ping") {
		t.Error("expected tool name header")
	}
	if strings.Contains(output, "## Parameters") {
		t.Error("expected no parameters section for nil params")
	}
}
