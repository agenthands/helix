package symbols

import (
	"context"
	"fmt"

	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// SymbolOutline represents a symbol in the document outline tree.
type SymbolOutline struct {
	Name     string
	Kind     string
	Range    gen.Range
	Children []SymbolOutline
}

// GetSymbolOverview sends textDocument/documentSymbol and returns a hierarchical outline.
// SYM-03: Get a structured outline of all symbols in a file, preserving hierarchy.
func GetSymbolOverview(ctx context.Context, lease *lspool.WorkerLease, uri string) ([]SymbolOutline, error) {
	params := gen.DocumentSymbolParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
	}
	var result []gen.DocumentSymbol
	if err := lease.Request(ctx, "textDocument/documentSymbol", params, &result); err != nil {
		return nil, fmt.Errorf("documentSymbol: %w", err)
	}
	return mapDocumentSymbols(result), nil
}

// mapDocumentSymbols recursively converts LSP DocumentSymbol[] to SymbolOutline[].
func mapDocumentSymbols(symbols []gen.DocumentSymbol) []SymbolOutline {
	out := make([]SymbolOutline, len(symbols))
	for i, sym := range symbols {
		out[i] = SymbolOutline{
			Name:     sym.Name,
			Kind:     SymbolKindName(sym.Kind),
			Range:    sym.Range,
			Children: mapDocumentSymbols(sym.Children),
		}
	}
	return out
}
