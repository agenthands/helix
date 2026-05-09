package retrieval

import (
	"strings"
	"unicode"

	"github.com/agenthands/helix/internal/semantic/store"
)

// MapSymbolToDoc transforms a snapshot SymbolRow + (optional) source bytes
// into the SymbolDoc indexed by bleve. Per CONTEXT.md D-06 the indexed text
// fields are: tokenized symbol name (camelCase / snake_case split), full
// docstring, tokenized file path components, and a ~5-line comment window
// (CommentWindowLines half-width above + below the declaration line).
//
// fileSource may be nil — in that case the comment-window field is left
// empty. The recovery rebuild path passes nil because IterateCommittedSymbols
// returns SymbolRow without source bytes (see recovery.go for rationale).
//
// The function takes `store.SymbolRow` from internal/semantic/store (declared
// by 64-02) — NOT a locally-redefined type — to keep the corpus mapper a
// pure adapter between snapshot rows and bleve documents.
func MapSymbolToDoc(row store.SymbolRow, fileSource []byte) SymbolDoc {
	return SymbolDoc{
		ID:            row.SymbolID,
		Name:          tokenizeIdent(row.Name),
		Path:          tokenizePath(row.Path),
		Doc:           row.Docstring,
		CommentWindow: extractCommentWindow(fileSource, row.LineStart, CommentWindowLines),
	}
}

// tokenizeIdent splits a camelCase / snake_case / kebab-case identifier into
// space-separated tokens so the bleve text analyzer indexes each component
// individually. Output also retains the original identifier as a leading
// token so exact-name queries still match.
//
//	"GetUserByID"   -> "GetUserByID Get User By ID"
//	"snake_case_id" -> "snake_case_id snake case id"
//	"kebab-case-id" -> "kebab-case-id kebab case id"
func tokenizeIdent(s string) string {
	if s == "" {
		return ""
	}
	parts := []string{s}
	parts = append(parts, splitCamelSnakeKebab(s)...)
	return strings.Join(parts, " ")
}

// splitCamelSnakeKebab decomposes s into individual word tokens using camel-
// case boundaries, plus '_' and '-' separators. Empty tokens are dropped.
func splitCamelSnakeKebab(s string) []string {
	if s == "" {
		return nil
	}
	// Pass 1: split on '_' and '-' (snake / kebab).
	chunks := strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == '-'
	})
	out := make([]string, 0, len(chunks)*2)
	// Pass 2: within each chunk, split on lower→upper transitions.
	for _, c := range chunks {
		out = append(out, splitCamel(c)...)
	}
	return out
}

// splitCamel splits an identifier chunk on lower→upper boundaries while
// keeping consecutive uppercase runs together (so "HTTPServer" -> "HTTP Server"
// rather than "H T T P Server").
func splitCamel(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	runes := []rune(s)
	start := 0
	for i := 1; i < len(runes); i++ {
		prev := runes[i-1]
		cur := runes[i]
		// Lower→upper boundary: split.
		if unicode.IsLower(prev) && unicode.IsUpper(cur) {
			out = append(out, string(runes[start:i]))
			start = i
			continue
		}
		// Upper→upper followed by upper→lower (e.g., "HTTPServer" at the
		// 'S'->'e' boundary): the split point is the trailing uppercase.
		if unicode.IsUpper(prev) && unicode.IsUpper(cur) && i+1 < len(runes) && unicode.IsLower(runes[i+1]) {
			out = append(out, string(runes[start:i]))
			start = i
			continue
		}
	}
	out = append(out, string(runes[start:]))
	return out
}

// tokenizePath splits a file path into space-separated components on '/' and
// '-' so bleve can index e.g. "src/auth-service/handler.go" as
// "src auth service handler go" plus the original path.
func tokenizePath(p string) string {
	if p == "" {
		return ""
	}
	parts := []string{p}
	chunks := strings.FieldsFunc(p, func(r rune) bool {
		return r == '/' || r == '-' || r == '\\' || r == '.'
	})
	parts = append(parts, chunks...)
	return strings.Join(parts, " ")
}

// extractCommentWindow returns the source-line text within `windowLines` of
// `anchorLine` (1-based) above and below — clamped to file bounds. Returns
// "" when src is nil/empty or anchorLine is out of range. The window is
// joined with newlines so the bleve analyzer sees a coherent block.
func extractCommentWindow(src []byte, anchorLine, windowLines int) string {
	if len(src) == 0 || anchorLine <= 0 || windowLines <= 0 {
		return ""
	}
	lines := strings.Split(string(src), "\n")
	if anchorLine > len(lines) {
		return ""
	}
	// 1-based -> 0-based.
	idx := anchorLine - 1
	start := idx - windowLines
	if start < 0 {
		start = 0
	}
	end := idx + windowLines + 1
	if end > len(lines) {
		end = len(lines)
	}
	return strings.Join(lines[start:end], "\n")
}
