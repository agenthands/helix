package edit

import (
	"context"
	"fmt"
	"os"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/kernel/symbols"
)

// DeleteResult summarizes the outcome of a delete operation.
type DeleteResult struct {
	Deleted      bool
	References   int                      // number of remaining references (if not deleted)
	RefLocations []symbols.SymbolLocation // where the refs are
}

// SafeDelete deletes a symbol from a file after checking for references.
// EDT-05: Unless force=true, checks FindReferences before deleting.
func SafeDelete(ctx context.Context, lease *lspool.WorkerLease, uri string, symbolName string, force bool) (*DeleteResult, error) {
	// Find symbol via documentSymbol.
	plan, err := PlanEdit(ctx, lease, uri, symbolName, EditTypeDelete, "")
	if err != nil {
		return nil, fmt.Errorf("plan edit: %w", err)
	}

	// Unless force, check references.
	if !force {
		// Use the selection range start (identifier position) for reference lookup.
		refs, err := symbols.FindReferences(ctx, lease, uri,
			int(plan.Range.Start.Line), int(plan.Range.Start.Character), false)
		if err != nil {
			return nil, fmt.Errorf("find references: %w", err)
		}

		if len(refs) > 0 {
			return &DeleteResult{
				Deleted:      false,
				References:   len(refs),
				RefLocations: refs,
			}, nil
		}
	}

	// Safe to delete: remove the symbol from the file.
	filePath := uriToPath(uri)
	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	startByte, endByte := rangeToByteOffsets(source, plan.Range)

	// Remove from startByte to endByte, including trailing newline if present.
	if endByte < uint(len(source)) && source[endByte] == '\n' {
		endByte++
	}

	var result []byte
	result = append(result, source[:startByte]...)
	result = append(result, source[endByte:]...)

	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return nil, fmt.Errorf("write file %s: %w", filePath, err)
	}

	if err := notifyDidChange(ctx, lease, uri, string(result)); err != nil {
		return nil, fmt.Errorf("didChange notification: %w", err)
	}

	return &DeleteResult{Deleted: true}, nil
}
