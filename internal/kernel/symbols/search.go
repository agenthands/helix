package symbols

import (
	"context"
	"fmt"

	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// SearchSymbols sends workspace/symbol and returns matching symbol locations.
// SYM-04: Search for symbols across the workspace by name pattern.
func SearchSymbols(ctx context.Context, lease *lspool.WorkerLease, query string) ([]SymbolLocation, error) {
	params := gen.WorkspaceSymbolParams{
		Query: query,
	}
	var result []gen.SymbolInformation
	if err := lease.Request(ctx, "workspace/symbol", params, &result); err != nil {
		return nil, fmt.Errorf("workspace/symbol: %w", err)
	}
	out := make([]SymbolLocation, len(result))
	for i, sym := range result {
		out[i] = SymbolLocation{
			URI:   sym.Location.URI,
			Range: sym.Location.Range,
			Name:  sym.Name,
			Kind:  SymbolKindName(sym.Kind),
		}
	}
	return out, nil
}
