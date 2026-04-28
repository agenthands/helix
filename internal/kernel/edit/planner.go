package edit

import (
	"context"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
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
	URI            string
	SymbolName     string
	EditType       string    // one of EditType* constants
	Range          gen.Range // computed edit range (full symbol extent)
	SelectionRange gen.Range // identifier range (for cursor positioning)
	NewContent     string    // content to insert/replace
}

// PlanEdit resolves a symbol via documentSymbol and produces an EditPlan.
// Should be called on a warm (indexed) lease for accurate symbol ranges.
func PlanEdit(ctx context.Context, lease *lspool.WorkerLease, uri string, symbolName string, editType string, content string) (*EditPlan, error) {
	outlines, err := symbols.GetSymbolOverview(ctx, lease, uri)
	if err != nil {
		return nil, serr.Wrap(serr.Internal, "get symbol overview", err)
	}

	outline := findOutlineByName(outlines, symbolName)
	if outline == nil {
		return nil, serr.New(serr.NotFound, "symbol not found").WithDetail(symbolName)
	}

	return &EditPlan{
		URI:            uri,
		SymbolName:     symbolName,
		EditType:       editType,
		Range:          outline.Range,
		SelectionRange: outline.SelectionRange,
		NewContent:     content,
	}, nil
}

// findOutlineByName searches recursively for a symbol by name in the outline tree.
//
// jdtls (and some other LSPs) return method names with their argument list
// appended — e.g. "helper()" or "format(String,Object[])" — so a literal
// equality check on a bare method name like "helper" misses. Strip the
// trailing parenthesised argument list before comparing in addition to the
// existing exact and dot-qualified matches.
func findOutlineByName(outlines []symbols.SymbolOutline, name string) *symbols.SymbolOutline {
	for i := range outlines {
		candidate := outlines[i].Name
		bareCandidate := stripSymbolSignature(candidate)
		if candidate == name || bareCandidate == name {
			return &outlines[i]
		}
		// Also check with dot-qualified names for methods.
		if strings.HasSuffix(candidate, "."+name) || strings.HasSuffix(bareCandidate, "."+name) ||
			strings.HasPrefix(name, candidate+".") || strings.HasPrefix(name, bareCandidate+".") {
			return &outlines[i]
		}
		if found := findOutlineByName(outlines[i].Children, name); found != nil {
			return found
		}
	}
	return nil
}

// stripSymbolSignature removes a trailing "(...)" argument list from a symbol
// name, leaving the bare identifier. Returns the input unchanged if no
// parenthesised tail is present.
func stripSymbolSignature(name string) string {
	if idx := strings.IndexByte(name, '('); idx >= 0 {
		return name[:idx]
	}
	return name
}
