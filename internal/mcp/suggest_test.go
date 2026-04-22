package mcp_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/internal/mcp"
)

func newCallToolReqWithArgs(name string, argsJSON string) *mcpsdk.CallToolRequest {
	return &mcpsdk.CallToolRequest{
		Params: &mcpsdk.CallToolParamsRaw{
			Name:      name,
			Arguments: json.RawMessage(argsJSON),
		},
	}
}

func suggestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

// --- Levenshtein distance tests ---

func TestLevenshteinDistance(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"", "", 0},
		{"abc", "abc", 0},
		{"kitten", "sitting", 3},
		{"file_path", "file_pth", 1},
		{"relative_path", "reltive_path", 1},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s_%s", tt.a, tt.b), func(t *testing.T) {
			got := mcp.LevenshteinDistanceForTest(tt.a, tt.b)
			if got != tt.want {
				t.Errorf("levenshteinDistance(%q, %q) = %d, want %d", tt.a, tt.b, got, tt.want)
			}
		})
	}

	// Negative case: distance should be > 2 for unrelated strings.
	d := mcp.LevenshteinDistanceForTest("path", "relative_path")
	if d <= 2 {
		t.Errorf("expected distance > 2 for (path, relative_path), got %d", d)
	}
}

// --- ExtractBadParams tests ---

func TestExtractBadParams(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "single param",
			input: `validating "arguments": unexpected additional properties ["file_path"]`,
			want:  []string{"file_path"},
		},
		{
			name:  "multiple params",
			input: `validating "arguments": unexpected additional properties ["file_path" "other"]`,
			want:  []string{"file_path", "other"},
		},
		{
			name:  "unrelated error",
			input: "some other error",
			want:  nil,
		},
		{
			name:  "empty string",
			input: "",
			want:  nil,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mcp.ExtractBadParamsForTest(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("ExtractBadParams(%q) = %v, want %v", tt.input, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ExtractBadParams(%q)[%d] = %q, want %q", tt.input, i, got[i], tt.want[i])
				}
			}
		})
	}
}

// --- BestParamSuggestion tests ---

func TestBestParamSuggestion(t *testing.T) {
	tests := []struct {
		name     string
		unknown  string
		valid    []string
		maxDist  int
		wantName string
		wantDist int
	}{
		{
			name:     "close typo",
			unknown:  "file_pth",
			valid:    []string{"file_path", "relative_path", "line"},
			maxDist:  2,
			wantName: "file_path",
			wantDist: 1,
		},
		{
			name:     "substring match",
			unknown:  "path",
			valid:    []string{"relative_path"},
			maxDist:  2,
			wantName: "relative_path",
		},
		{
			name:     "no match",
			unknown:  "xyz",
			valid:    []string{"file_path", "relative_path"},
			maxDist:  2,
			wantName: "",
			wantDist: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, dist := mcp.BestParamSuggestionForTest(tt.unknown, tt.valid, tt.maxDist)
			if name != tt.wantName {
				t.Errorf("BestParamSuggestion(%q, %v, %d) name = %q, want %q", tt.unknown, tt.valid, tt.maxDist, name, tt.wantName)
			}
			if tt.wantDist > 0 && dist != tt.wantDist {
				t.Errorf("BestParamSuggestion(%q, %v, %d) dist = %d, want %d", tt.unknown, tt.valid, tt.maxDist, dist, tt.wantDist)
			}
		})
	}
}

// --- BestValueSuggestion tests ---

func TestBestValueSuggestion(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		valid    []string
		maxDist  int
		wantName string
		wantDist int
	}{
		{
			name:     "close enum typo",
			value:    "incming",
			valid:    []string{"incoming", "outgoing"},
			maxDist:  2,
			wantName: "incoming",
			wantDist: 1,
		},
		{
			name:     "no match",
			value:    "xyz",
			valid:    []string{"incoming", "outgoing"},
			maxDist:  2,
			wantName: "",
			wantDist: 3,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name, dist := mcp.BestValueSuggestionForTest(tt.value, tt.valid, tt.maxDist)
			if name != tt.wantName {
				t.Errorf("BestValueSuggestion(%q, %v, %d) name = %q, want %q", tt.value, tt.valid, tt.maxDist, name, tt.wantName)
			}
			if dist != tt.wantDist {
				t.Errorf("BestValueSuggestion(%q, %v, %d) dist = %d, want %d", tt.value, tt.valid, tt.maxDist, dist, tt.wantDist)
			}
		})
	}
}

// --- BuildToolSchemaMap tests ---

func TestBuildToolSchemaMap(t *testing.T) {
	tools := []*mcpsdk.Tool{
		{
			Name: "test_tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"symbol_name": map[string]any{"type": "string"},
					"direction": map[string]any{
						"type": "string",
						"enum": []any{"incoming", "outgoing"},
					},
				},
			},
		},
		{
			Name:        "nil_schema_tool",
			InputSchema: nil,
		},
	}

	schemaMap := mcp.BuildToolSchemaMap(tools)

	// Test that we can access the schema map through the middleware.
	// Since tools field is unexported, we verify via middleware behavior.
	// But we can also verify by building a middleware and testing it.

	// Verify via middleware: a typo for test_tool should get a suggestion.
	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, fmt.Errorf(`validating "arguments": unexpected additional properties ["symbol_nam"]`)
	}
	handler := mw(inner)
	result, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("test_tool", `{}`))

	// Should have been converted to a tool error with suggestion.
	if err != nil {
		t.Fatalf("expected nil error after enrichment, got: %v", err)
	}
	ctr, ok := result.(*mcpsdk.CallToolResult)
	if !ok || ctr == nil {
		t.Fatal("expected *CallToolResult")
	}
	if !ctr.IsError {
		t.Error("expected IsError=true")
	}
	tc, ok := ctr.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if !strings.Contains(tc.Text, "symbol_name") {
		t.Errorf("expected suggestion containing 'symbol_name', got: %s", tc.Text)
	}

	// Verify nil schema tool doesn't cause errors: call middleware for it.
	inner2 := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, fmt.Errorf(`validating "arguments": unexpected additional properties ["foo"]`)
	}
	handler2 := mw(inner2)
	_, err2 := handler2(context.Background(), "tools/call", newCallToolReqWithArgs("nil_schema_tool", `{}`))
	// Should return the original error unchanged (no valid params to suggest).
	if err2 == nil {
		t.Error("expected original error to be preserved for nil_schema_tool")
	}
}

// --- SuggestionMiddleware protocol error tests ---

func TestSuggestMiddleware_ParamTypo(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap([]*mcpsdk.Tool{
		{
			Name: "find_symbol",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"symbol_name":   map[string]any{"type": "string"},
					"relative_path": map[string]any{"type": "string"},
					"line":          map[string]any{"type": "integer"},
				},
			},
		},
	})

	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		// "relativ_path" is distance 1 from "relative_path" -- realistic typo.
		return nil, fmt.Errorf(`validating "arguments": unexpected additional properties ["relativ_path"]`)
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("find_symbol", `{}`))

	if err != nil {
		t.Fatalf("expected nil error after enrichment, got: %v", err)
	}
	ctr, ok := result.(*mcpsdk.CallToolResult)
	if !ok || ctr == nil {
		t.Fatal("expected *CallToolResult")
	}
	if !ctr.IsError {
		t.Error("expected IsError=true")
	}
	tc, ok := ctr.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	// Must contain original error and suggestion.
	if !strings.Contains(tc.Text, `unexpected additional properties`) {
		t.Errorf("expected original error text preserved, got: %s", tc.Text)
	}
	if !strings.Contains(tc.Text, "Did you mean: relative_path (instead of relativ_path)?") {
		t.Errorf("expected suggestion for relative_path, got: %s", tc.Text)
	}
}

func TestSuggestMiddleware_NoMatch(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap([]*mcpsdk.Tool{
		{
			Name: "find_symbol",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"symbol_name":   map[string]any{"type": "string"},
					"relative_path": map[string]any{"type": "string"},
				},
			},
		},
	})

	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, fmt.Errorf(`validating "arguments": unexpected additional properties ["xyz_unknown"]`)
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("find_symbol", `{}`))

	// Original error should be returned unchanged.
	if err == nil {
		t.Fatal("expected original error to be preserved")
	}
	if !strings.Contains(err.Error(), "xyz_unknown") {
		t.Errorf("expected original error text, got: %v", err)
	}
}

// --- SuggestionMiddleware tool error tests ---

func TestSuggestMiddleware_EnumValue(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap([]*mcpsdk.Tool{
		{
			Name: "get_call_hierarchy",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"direction": map[string]any{
						"type": "string",
						"enum": []any{"incoming", "outgoing"},
					},
					"symbol_name": map[string]any{"type": "string"},
				},
			},
		},
	})

	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: `invalid value "incming" for field "direction"`},
			},
			IsError: true,
		}, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("get_call_hierarchy", `{}`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ctr, ok := result.(*mcpsdk.CallToolResult)
	if !ok || ctr == nil {
		t.Fatal("expected *CallToolResult")
	}
	tc, ok := ctr.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	if !strings.Contains(tc.Text, "Did you mean: incoming (instead of incming)?") {
		t.Errorf("expected enum value suggestion, got: %s", tc.Text)
	}
}

// --- SuggestionMiddleware scope tests ---

func TestSuggestMiddleware_SameToolOnly(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap([]*mcpsdk.Tool{
		{
			Name: "tool_a",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"alpha": map[string]any{"type": "string"},
				},
			},
		},
		{
			Name: "tool_b",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"beta": map[string]any{"type": "string"},
				},
			},
		},
	})

	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		// "bta" is close to "beta" (tool_b), but request is for tool_a.
		return nil, fmt.Errorf(`validating "arguments": unexpected additional properties ["bta"]`)
	}

	handler := mw(inner)
	_, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("tool_a", `{}`))

	// "bta" is not close to "alpha" (tool_a's only param), so no suggestion.
	if err == nil {
		t.Fatal("expected original error to be preserved (no cross-tool suggestion)")
	}
	if strings.Contains(err.Error(), "Did you mean") {
		t.Error("expected no suggestion (beta belongs to tool_b, not tool_a)")
	}
}

func TestSuggestMiddleware_AppendOnly(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap([]*mcpsdk.Tool{
		{
			Name: "test_tool",
			InputSchema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"file_path": map[string]any{"type": "string"},
				},
			},
		},
	})

	originalText := "original error message about file_pth"
	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return nil, fmt.Errorf(`validating "arguments": unexpected additional properties ["file_pth"]`)
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("test_tool", `{}`))

	if err != nil {
		t.Fatalf("expected nil error after enrichment, got: %v", err)
	}
	ctr, ok := result.(*mcpsdk.CallToolResult)
	if !ok || ctr == nil {
		t.Fatal("expected *CallToolResult")
	}
	tc, ok := ctr.Content[0].(*mcpsdk.TextContent)
	if !ok {
		t.Fatal("expected TextContent")
	}
	// Original error text must be preserved before the suggestion line.
	_ = originalText // We check the original SDK error is preserved.
	if !strings.HasPrefix(tc.Text, `validating "arguments": unexpected additional properties ["file_pth"]`) {
		t.Errorf("original error text not preserved at start, got: %s", tc.Text)
	}
	if !strings.Contains(tc.Text, "\nDid you mean: file_path (instead of file_pth)?") {
		t.Errorf("expected suggestion appended with newline, got: %s", tc.Text)
	}
}

// --- Pass-through tests ---

func TestSuggestMiddleware_SuccessPassthrough(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap([]*mcpsdk.Tool{
		{
			Name: "test_tool",
			InputSchema: map[string]any{
				"type":       "object",
				"properties": map[string]any{"param": map[string]any{"type": "string"}},
			},
		},
	})

	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())
	successResult := &mcpsdk.CallToolResult{
		Content: []mcpsdk.Content{
			&mcpsdk.TextContent{Text: "success result"},
		},
	}
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		return successResult, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/call", newCallToolReqWithArgs("test_tool", `{}`))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != successResult {
		t.Error("expected success result to pass through unchanged")
	}
}

func TestSuggestMiddleware_NonToolsCall(t *testing.T) {
	schemaMap := mcp.BuildToolSchemaMap(nil)

	mw := mcp.SuggestionMiddleware(schemaMap, suggestLogger())

	called := false
	inner := func(ctx context.Context, method string, req mcpsdk.Request) (mcpsdk.Result, error) {
		called = true
		return &mcpsdk.ListToolsResult{}, nil
	}

	handler := mw(inner)
	result, err := handler(context.Background(), "tools/list", nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("expected inner handler to be called")
	}
	if _, ok := result.(*mcpsdk.ListToolsResult); !ok {
		t.Error("expected ListToolsResult to pass through unchanged")
	}
}
