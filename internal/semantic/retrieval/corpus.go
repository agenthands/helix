package retrieval

import (
	"github.com/agenthands/helix/internal/semantic/store"
)

// corpus.go — STUB (RED gate). Real implementation lands in Task 1 GREEN.
//
// MapSymbolToDoc transforms a snapshot SymbolRow + (optional) source bytes
// into the SymbolDoc indexed by bleve. Per CONTEXT.md D-06 the indexed text
// fields are: tokenized symbol name (camelCase / snake_case split), full
// docstring, tokenized file path components, and a ~5-line comment window
// above + below the declaration line.
//
// fileSource may be nil — in that case the comment-window field is left
// empty. The recovery rebuild path passes nil because IterateCommittedSymbols
// returns SymbolRow without source bytes (see recovery.go for rationale).
func MapSymbolToDoc(row store.SymbolRow, fileSource []byte) SymbolDoc {
	panic("retrieval.MapSymbolToDoc: not implemented (RED gate — Task 1 GREEN fills this in)")
}
