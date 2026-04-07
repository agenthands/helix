package edit

import (
	"context"
	"fmt"
	"strings"

	"github.com/postfix/serena/internal/kernel/lspool"
	"github.com/postfix/serena/internal/kernel/symbols"
	gen "github.com/postfix/serena/protocol/gen"
)

// EditType enumerates the kinds of edits the planner can produce.
const (
	EditTypeReplaceBody  = "replace_body"
	EditTypeInsertBefore = "insert_before"
	EditTypeInsertAfter  = "insert_after"
	EditTypeRename       = "rename"
	EditTypeDelete       = "delete"
)

// EditPlan describes a resolved edit ready for execution.
type EditPlan struct {
	URI        string
	SymbolName string
	EditType   string    // one of EditType* constants
	Range      gen.Range // computed edit range
	NewContent string    // content to insert/replace
}

// PlanEdit resolves a symbol via documentSymbol and produces an EditPlan.
func PlanEdit(ctx context.Context, lease *lspool.WorkerLease, uri string, symbolName string, editType string, content string) (*EditPlan, error) {
	outlines, err := symbols.GetSymbolOverview(ctx, lease, uri)
	if err != nil {
		return nil, fmt.Errorf("get symbol overview: %w", err)
	}

	outline := findOutlineByName(outlines, symbolName)
	if outline == nil {
		return nil, fmt.Errorf("symbol %q not found in %s", symbolName, uri)
	}

	return &EditPlan{
		URI:        uri,
		SymbolName: symbolName,
		EditType:   editType,
		Range:      outline.Range,
		NewContent: content,
	}, nil
}

// findOutlineByName searches recursively for a symbol by name in the outline tree.
func findOutlineByName(outlines []symbols.SymbolOutline, name string) *symbols.SymbolOutline {
	for i := range outlines {
		if outlines[i].Name == name {
			return &outlines[i]
		}
		// Also check with dot-qualified names for methods.
		if strings.HasSuffix(outlines[i].Name, "."+name) || strings.HasPrefix(name, outlines[i].Name+".") {
			return &outlines[i]
		}
		if found := findOutlineByName(outlines[i].Children, name); found != nil {
			return found
		}
	}
	return nil
}
