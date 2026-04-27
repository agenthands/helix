package symbols

import (
	"context"
	"fmt"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// SymbolLocation represents a code location with a preview line.
type SymbolLocation struct {
	URI     string
	Range   gen.Range
	Preview string // the line of code at this location (best-effort)
	Name    string // optional symbol name (for workspace search results)
	Kind    string // optional symbol kind name
}

// HoverResult holds formatted hover/type information.
type HoverResult struct {
	Content  string // markdown content
	Language string // language ID for code blocks (best-effort)
}

// GoToDefinition sends textDocument/definition and returns location(s).
// SYM-01: Navigate to the definition of a symbol at line:col in the given file URI.
func GoToDefinition(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int) ([]SymbolLocation, error) {
	params := gen.DefinitionParams{
		TextDocumentPositionParams: makePositionParams(uri, line, col),
	}
	var result []gen.Location
	if err := lease.Request(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, serr.Wrap(serr.Internal, "definition", err)
	}
	return locationsToSymbolLocations(result), nil
}

// FindReferences sends textDocument/references and returns all reference locations.
// SYM-02: Find all references to the symbol at the given position.
func FindReferences(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, includeDecl bool) ([]SymbolLocation, error) {
	params := gen.ReferenceParams{
		TextDocumentPositionParams: makePositionParams(uri, line, col),
		Context: gen.ReferenceContext{
			IncludeDeclaration: includeDecl,
		},
	}
	var result []gen.Location
	if err := lease.Request(ctx, "textDocument/references", params, &result); err != nil {
		return nil, serr.Wrap(serr.Internal, "references", err)
	}
	return locationsToSymbolLocations(result), nil
}

// GetHover sends textDocument/hover and returns formatted hover content.
// SYM-05: Get type/documentation information for the symbol at the given position.
func GetHover(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int) (*HoverResult, error) {
	params := gen.HoverParams{
		TextDocumentPositionParams: makePositionParams(uri, line, col),
	}
	var result gen.Hover
	if err := lease.Request(ctx, "textDocument/hover", params, &result); err != nil {
		return nil, serr.Wrap(serr.Internal, "hover", err)
	}
	content := extractHoverContent(result)
	if content == "" {
		return nil, nil // no hover info available
	}
	return &HoverResult{Content: content}, nil
}

// FindImplementations sends textDocument/implementation and returns locations.
// SYM-06: Find all implementations of an interface or abstract method.
func FindImplementations(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int) ([]SymbolLocation, error) {
	params := gen.ImplementationParams{
		TextDocumentPositionParams: makePositionParams(uri, line, col),
	}
	var result []gen.Location
	if err := lease.Request(ctx, "textDocument/implementation", params, &result); err != nil {
		return nil, serr.Wrap(serr.Internal, "implementation", err)
	}
	return locationsToSymbolLocations(result), nil
}

// --- helpers ---

// makePositionParams converts a 1-indexed (line, col) — the public API
// convention used in tool schemas and matched by formatLocations' display
// output — into the 0-indexed Position the LSP expects. Inputs less than 1
// clamp to 0 so we never emit negative coordinates if a caller mistakenly
// passes 0.
func makePositionParams(uri string, line, col int) gen.TextDocumentPositionParams {
	return gen.TextDocumentPositionParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
		Position: gen.Position{
			Line:      ToLSPCoord(line),
			Character: ToLSPCoord(col),
		},
	}
}

// ToLSPCoord converts a 1-indexed coordinate to the 0-indexed value LSP
// expects. Values ≤ 0 clamp to 0.
func ToLSPCoord(v int) uint32 {
	if v <= 1 {
		return 0
	}
	return uint32(v - 1)
}

func locationsToSymbolLocations(locs []gen.Location) []SymbolLocation {
	out := make([]SymbolLocation, len(locs))
	for i, loc := range locs {
		out[i] = SymbolLocation{
			URI:   loc.URI,
			Range: loc.Range,
		}
	}
	return out
}

// extractHoverContent extracts the text content from the Hover result.
// The contents field is a union type; we handle MarkupContent, string, and []MarkedString.
func extractHoverContent(hover gen.Hover) string {
	if hover.Contents.Value == nil {
		return ""
	}
	switch v := hover.Contents.Value.(type) {
	case gen.MarkupContent:
		return v.Value
	case string:
		return v
	case map[string]interface{}:
		// Could be a MarkupContent as a raw map from JSON
		if val, ok := v["value"]; ok {
			if s, ok := val.(string); ok {
				return s
			}
		}
	case []interface{}:
		// Array of MarkedString
		var parts []string
		for _, item := range v {
			switch it := item.(type) {
			case string:
				parts = append(parts, it)
			case map[string]interface{}:
				if val, ok := it["value"]; ok {
					if s, ok := val.(string); ok {
						parts = append(parts, s)
					}
				}
			}
		}
		return strings.Join(parts, "\n\n")
	case gen.MarkedString:
		return extractMarkedStringContent(v)
	case []gen.MarkedString:
		var parts []string
		for _, m := range v {
			if s := extractMarkedStringContent(m); s != "" {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, "\n\n")
	}
	return ""
}

// extractMarkedStringContent unwraps MarkedString = Or_Literal_String, which
// holds either a string or a {language, value} struct.
func extractMarkedStringContent(m gen.MarkedString) string {
	if m.Value == nil {
		return ""
	}
	switch v := m.Value.(type) {
	case string:
		return v
	case map[string]interface{}:
		if val, ok := v["value"].(string); ok {
			return val
		}
	}
	return ""
}

// SymbolKindName converts a SymbolKind enum to a human-readable string.
func SymbolKindName(kind gen.SymbolKind) string {
	switch kind {
	case gen.SymbolKindFile:
		return "File"
	case gen.SymbolKindModule:
		return "Module"
	case gen.SymbolKindNamespace:
		return "Namespace"
	case gen.SymbolKindPackage:
		return "Package"
	case gen.SymbolKindClass:
		return "Class"
	case gen.SymbolKindMethod:
		return "Method"
	case gen.SymbolKindProperty:
		return "Property"
	case gen.SymbolKindField:
		return "Field"
	case gen.SymbolKindConstructor:
		return "Constructor"
	case gen.SymbolKindEnum:
		return "Enum"
	case gen.SymbolKindInterface:
		return "Interface"
	case gen.SymbolKindFunction:
		return "Function"
	case gen.SymbolKindVariable:
		return "Variable"
	case gen.SymbolKindConstant:
		return "Constant"
	case gen.SymbolKindString:
		return "String"
	case gen.SymbolKindNumber:
		return "Number"
	case gen.SymbolKindBoolean:
		return "Boolean"
	case gen.SymbolKindArray:
		return "Array"
	case gen.SymbolKindObject:
		return "Object"
	case gen.SymbolKindKey:
		return "Key"
	case gen.SymbolKindNull:
		return "Null"
	case gen.SymbolKindEnumMember:
		return "EnumMember"
	case gen.SymbolKindStruct:
		return "Struct"
	case gen.SymbolKindEvent:
		return "Event"
	case gen.SymbolKindOperator:
		return "Operator"
	case gen.SymbolKindTypeParameter:
		return "TypeParameter"
	default:
		return fmt.Sprintf("SymbolKind(%d)", kind)
	}
}
