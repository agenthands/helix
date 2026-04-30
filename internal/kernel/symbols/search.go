package symbols

import (
	"context"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	gen "github.com/agenthands/helix/protocol/gen"
)

// SearchSymbols sends workspace/symbol and returns matching symbol locations.
// SYM-04: Search for symbols across the workspace by name pattern.
func SearchSymbols(ctx context.Context, lease *lspool.WorkerLease, query string) ([]SymbolLocation, error) {
	params := gen.WorkspaceSymbolParams{
		Query: query,
	}
	var result []gen.SymbolInformation
	if err := lease.Request(ctx, "workspace/symbol", params, &result); err != nil {
		return nil, serr.Wrap(serr.Internal, "workspace symbol", err)
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
