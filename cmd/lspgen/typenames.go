package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"unicode"
)

// goType converts a metamodel Type to a Go type string.
func (g *Generator) goType(t Type) string {
	switch t.Kind {
	case "base":
		return g.goBaseType(t)
	case "reference":
		if override, ok := nameOverrides[t.Name]; ok {
			return override
		}
		return exportName(t.Name)
	case "array":
		if t.Element != nil {
			return "[]" + g.goType(*t.Element)
		}
		return "[]interface{}"
	case "map":
		keyType := "string"
		if t.Key != nil {
			keyType = g.goType(*t.Key)
		}
		valType := "interface{}"
		if t.Value != nil {
			if vt, ok := t.Value.(map[string]interface{}); ok {
				var valT Type
				b, _ := json.Marshal(vt)
				_ = json.Unmarshal(b, &valT)
				valType = g.goType(valT)
			}
		}
		return fmt.Sprintf("map[%s]%s", keyType, valType)
	case "or":
		return g.orTypeName(t)
	case "and":
		// For "and" types, use the first referenced type as the base.
		if len(t.Items) > 0 {
			return g.goType(t.Items[0])
		}
		return "interface{}"
	case "literal":
		// Literal types with properties become inline anonymous types.
		// For simplicity, represent as map[string]interface{}.
		return "map[string]interface{}"
	case "stringLiteral":
		return "string"
	case "integerLiteral":
		return "int32"
	case "booleanLiteral":
		return "bool"
	case "tuple":
		return "[]interface{}"
	default:
		return "interface{}"
	}
}

// goBaseType maps a base type to its Go equivalent.
func (g *Generator) goBaseType(t Type) string {
	if t.Name != "" {
		if goType, ok := baseTypeMap[t.Name]; ok {
			return goType
		}
	}
	return "interface{}"
}

// orTypeName generates a name for an Or union type.
func (g *Generator) orTypeName(t Type) string {
	if len(t.Items) == 0 {
		return "interface{}"
	}

	// Special case: X | null -> *X (pointer)
	nonNull := filterNonNull(t.Items)
	if len(nonNull) == 1 && len(t.Items) == 2 {
		return g.goType(nonNull[0])
	}

	parts := make([]string, len(t.Items))
	for i, item := range t.Items {
		parts[i] = typeShortName(item)
	}
	sort.Strings(parts)
	return "Or_" + strings.Join(parts, "_")
}

// typeShortName returns a short name for a type suitable for use in Or_ type names.
// All returned names must be valid Go identifier fragments (no brackets, braces, etc.).
func typeShortName(t Type) string {
	switch t.Kind {
	case "base":
		name := t.Name
		switch name {
		case "integer":
			return "Int32"
		case "uinteger":
			return "Uint32"
		case "decimal":
			return "Float64"
		case "boolean":
			return "Boolean"
		case "string":
			return "String"
		case "null":
			return "Null"
		case "URI", "DocumentUri", "RegExp":
			return name
		default:
			return exportName(name)
		}
	case "reference":
		name := t.Name
		if _, ok := nameOverrides[name]; ok {
			// Use the metamodel name for Or_ names, not the Go override
			return name
		}
		return exportName(name)
	case "array":
		if t.Element != nil {
			return typeShortName(*t.Element) + "Array"
		}
		return "Array"
	case "stringLiteral":
		return "String"
	case "integerLiteral":
		return "Int32"
	case "booleanLiteral":
		return "Boolean"
	case "literal":
		return "Literal"
	case "map":
		return "Map"
	case "tuple":
		return "Tuple"
	default:
		return "Any"
	}
}

// filterNonNull returns types that are not the null type.
func filterNonNull(types []Type) []Type {
	var result []Type
	for _, t := range types {
		if t.Kind != "base" || t.Name != "null" {
			result = append(result, t)
		}
	}
	return result
}

// exportName converts a string to an exported Go identifier.
func exportName(s string) string {
	if s == "" {
		return ""
	}
	// Handle special cases
	if override, ok := nameOverrides[s]; ok {
		return override
	}
	runes := []rune(s)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// goFieldName converts a JSON field name to a Go exported field name.
func goFieldName(name string) string {
	if override, ok := fieldNameOverrides[name]; ok {
		return override
	}

	// CamelCase conversion
	runes := []rune(name)
	if len(runes) == 0 {
		return name
	}
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

// ptrType wraps a Go type in a pointer (*T).
// For slices, maps, and interface{}, no pointer is needed (they are already nilable).
func ptrType(t string) string {
	if strings.HasPrefix(t, "[]") || strings.HasPrefix(t, "map[") || t == "interface{}" {
		return t
	}
	return "*" + t
}

// methodToGoName converts an LSP method name like "textDocument/completion" to "TextDocumentCompletion".
func methodToGoName(method string) string {
	parts := strings.Split(method, "/")
	var result string
	for _, part := range parts {
		if part == "$" || part == "" {
			continue
		}
		// Remove $ prefix from parts like $/cancelRequest
		part = strings.TrimPrefix(part, "$")
		result += exportName(part)
	}
	return result
}

// isPrimitive returns true if the Go type is a primitive (not a struct).
func isPrimitive(goType string) bool {
	switch goType {
	case "string", "int32", "uint32", "float64", "bool", "interface{}":
		return true
	}
	return strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[")
}

// cleanVarName makes a Go type name safe for use as a variable name.
func cleanVarName(goType string) string {
	goType = strings.ReplaceAll(goType, "[]", "Slice")
	goType = strings.ReplaceAll(goType, "map[", "Map")
	goType = strings.ReplaceAll(goType, "]", "")
	goType = strings.ReplaceAll(goType, "*", "Ptr")
	goType = strings.ReplaceAll(goType, "{}", "")
	goType = strings.ReplaceAll(goType, "interface", "Any")
	goType = strings.ReplaceAll(goType, "string", "String")
	return goType
}

// deduplicateStrings removes duplicate strings while preserving order.
func deduplicateStrings(ss []string) []string {
	seen := make(map[string]bool)
	var result []string
	for _, s := range ss {
		if !seen[s] {
			seen[s] = true
			result = append(result, s)
		}
	}
	return result
}

// isValidAliasTarget checks if a Go type string is valid as the right-hand side
// of a type alias (must be an identifier, possibly qualified).
func isValidAliasTarget(goType string) bool {
	if goType == "" || goType == "interface{}" {
		return false
	}
	if strings.HasPrefix(goType, "[]") || strings.HasPrefix(goType, "map[") || strings.HasPrefix(goType, "*") {
		return false
	}
	return true
}

// accessorName returns the short name used in As<Name>() and Set<Name>() accessors.
func accessorName(goType string) string {
	name := cleanVarName(goType)
	if len(name) > 0 {
		runes := []rune(name)
		runes[0] = unicode.ToUpper(runes[0])
		return string(runes)
	}
	return "Value"
}
