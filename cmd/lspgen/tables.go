package main

// nameOverrides maps metamodel type names to Go-friendly names where needed.
var nameOverrides = map[string]string{
	"URI":         "URI",
	"DocumentUri": "DocumentURI",
	"integer":     "int32",
	"uinteger":    "uint32",
	"decimal":     "float64",
	"LSPAny":      "interface{}",
	"LSPObject":   "map[string]interface{}",
	"LSPArray":    "[]interface{}",
	"null":        "interface{}",
}

// fieldNameOverrides maps JSON field names to Go-friendly exported names.
var fieldNameOverrides = map[string]string{
	"uri":         "URI",
	"id":          "ID",
	"documentUri": "DocumentURI",
	"jsonrpc":     "JSONRPC",
	"utf8":        "UTF8",
	"utf16":       "UTF16",
	"utf32":       "UTF32",
}

// baseTypeMap maps LSP base type names to Go types.
var baseTypeMap = map[string]string{
	"string":      "string",
	"integer":     "int32",
	"uinteger":    "uint32",
	"decimal":     "float64",
	"boolean":     "bool",
	"null":        "interface{}",
	"URI":         "string",
	"DocumentUri": "string",
	"RegExp":      "string",
}
