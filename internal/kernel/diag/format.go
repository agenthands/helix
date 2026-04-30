package diag

import (
	"context"
	"os"
	"sort"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel/lspool"
	gen "github.com/agenthands/helix/protocol/gen"
)

// FormatOptions controls formatting behaviour.
type FormatOptions struct {
	TabSize      int
	InsertSpaces bool
}

// TextEditResult is a simplified representation of a text edit from the LS.
type TextEditResult struct {
	StartLine int
	StartCol  int
	EndLine   int
	EndCol    int
	NewText   string
}

// FormatDocument sends textDocument/formatting and returns the resulting edits.
func FormatDocument(ctx context.Context, lease *lspool.WorkerLease, uri string, opts FormatOptions) ([]TextEditResult, error) {
	tabSize := opts.TabSize
	if tabSize <= 0 {
		tabSize = 4
	}

	params := gen.DocumentFormattingParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
		Options: gen.FormattingOptions{
			TabSize:      uint32(tabSize),
			InsertSpaces: opts.InsertSpaces,
		},
	}

	var edits []gen.TextEdit
	if err := lease.Request(ctx, "textDocument/formatting", params, &edits); err != nil {
		return nil, serr.Wrap(serr.Internal, "formatting request", err)
	}

	results := make([]TextEditResult, 0, len(edits))
	for _, e := range edits {
		results = append(results, TextEditResult{
			StartLine: int(e.Range.Start.Line),
			StartCol:  int(e.Range.Start.Character),
			EndLine:   int(e.Range.End.Line),
			EndCol:    int(e.Range.End.Character),
			NewText:   e.NewText,
		})
	}
	return results, nil
}

// ApplyFormatEdits applies text edits to a file on disk. Edits are applied in
// reverse document order (bottom-up) to preserve earlier line numbers.
// The write is atomic: content is written to a temp file then renamed.
func ApplyFormatEdits(path string, edits []TextEditResult) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return serr.Wrap(serr.Internal, "reading file", err)
	}

	lines := strings.Split(string(data), "\n")

	// Sort edits bottom-up so earlier offsets remain stable.
	sorted := make([]TextEditResult, len(edits))
	copy(sorted, edits)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].StartLine != sorted[j].StartLine {
			return sorted[i].StartLine > sorted[j].StartLine
		}
		return sorted[i].StartCol > sorted[j].StartCol
	})

	for _, e := range sorted {
		lines = applyEdit(lines, e)
	}

	content := strings.Join(lines, "\n")

	// Atomic write via temp file.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0o644); err != nil {
		return serr.Wrap(serr.Internal, "writing temp file", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return serr.Wrap(serr.Internal, "renaming temp file", err)
	}
	return nil
}

// applyEdit replaces the text in lines[startLine:endLine] at the given columns.
func applyEdit(lines []string, e TextEditResult) []string {
	// Clamp to valid range.
	if e.StartLine < 0 {
		e.StartLine = 0
	}
	if e.EndLine >= len(lines) {
		e.EndLine = len(lines) - 1
	}
	if e.StartLine >= len(lines) {
		// Append at end.
		return append(lines, strings.Split(e.NewText, "\n")...)
	}

	// Build the prefix (before start col on start line) and suffix (after end col on end line).
	startLineText := lines[e.StartLine]
	endLineText := lines[e.EndLine]

	startCol := e.StartCol
	if startCol > len(startLineText) {
		startCol = len(startLineText)
	}

	endCol := e.EndCol
	if endCol > len(endLineText) {
		endCol = len(endLineText)
	}

	prefix := startLineText[:startCol]
	suffix := endLineText[endCol:]

	replacement := prefix + e.NewText + suffix
	newLines := strings.Split(replacement, "\n")

	// Replace lines[startLine..endLine] with newLines.
	result := make([]string, 0, len(lines)-((e.EndLine-e.StartLine)+1)+len(newLines))
	result = append(result, lines[:e.StartLine]...)
	result = append(result, newLines...)
	if e.EndLine+1 < len(lines) {
		result = append(result, lines[e.EndLine+1:]...)
	}
	return result
}
