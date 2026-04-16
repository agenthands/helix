package repomap

import (
	"context"
	"fmt"

	serr "github.com/postfix/serena/internal/errors"
	gen "github.com/postfix/serena/protocol/gen"
)

// SymbolRequester abstracts the LSP documentSymbol request for testability.
// WorkerLease from lspool satisfies this interface via its Request method.
type SymbolRequester interface {
	Request(ctx context.Context, method string, params, result interface{}) error
}

// FallbackExtractor produces def-only tags from LSP documentSymbol responses
// for languages without tree-sitter grammars. Per D-07, all symbols are mapped
// as TagDef; no references are extracted via this path.
type FallbackExtractor struct{}

// NewFallbackExtractor creates a new FallbackExtractor.
func NewFallbackExtractor() *FallbackExtractor {
	return &FallbackExtractor{}
}

// Extract calls textDocument/documentSymbol via the given requester and returns
// def-only tags. Nested DocumentSymbol children produce qualified names
// (e.g. "ClassName.methodName") per D-03.
func (f *FallbackExtractor) Extract(ctx context.Context, requester SymbolRequester, filePath string, uri string) ([]Tag, error) {
	params := gen.DocumentSymbolParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
	}
	var result []gen.DocumentSymbol
	if err := requester.Request(ctx, "textDocument/documentSymbol", params, &result); err != nil {
		return nil, serr.Wrap(serr.Internal, "fallback documentSymbol", err)
	}
	var tags []Tag
	flattenSymbols(result, filePath, "", &tags)
	return tags, nil
}

// flattenSymbols recursively walks the DocumentSymbol tree and appends def-only
// tags. parentName is used to produce qualified names for nested symbols.
func flattenSymbols(symbols []gen.DocumentSymbol, filePath string, parentName string, tags *[]Tag) {
	for _, sym := range symbols {
		name := sym.Name
		if parentName != "" {
			name = fmt.Sprintf("%s.%s", parentName, sym.Name)
		}
		*tags = append(*tags, Tag{
			Name:      name,
			Kind:      TagDef,
			File:      filePath,
			Line:      int(sym.SelectionRange.Start.Line),
			Column:    int(sym.SelectionRange.Start.Character),
			StartByte: 0,
			EndByte:   0,
		})
		if len(sym.Children) > 0 {
			flattenSymbols(sym.Children, filePath, name, tags)
		}
	}
}
