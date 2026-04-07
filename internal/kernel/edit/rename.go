package edit

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// RenameResult summarizes the outcome of a rename operation.
type RenameResult struct {
	FilesChanged int
	EditsApplied int
	Files        []string
}

// RenameSymbol sends textDocument/rename and applies the resulting WorkspaceEdit.
// EDT-04: LSP rename forwarding.
func RenameSymbol(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, newName string) (*RenameResult, error) {
	params := gen.RenameParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
		Position: gen.Position{
			Line:      uint32(line),
			Character: uint32(col),
		},
		NewName: newName,
	}

	var wsEdit gen.WorkspaceEdit
	if err := lease.Request(ctx, "textDocument/rename", params, &wsEdit); err != nil {
		return nil, fmt.Errorf("rename: %w", err)
	}

	result := &RenameResult{}

	// Apply changes from the WorkspaceEdit.Changes map.
	if wsEdit.Changes != nil {
		for fileURI, edits := range wsEdit.Changes {
			if err := applyTextEdits(fileURI, edits); err != nil {
				return nil, fmt.Errorf("apply edits to %s: %w", fileURI, err)
			}
			result.FilesChanged++
			result.EditsApplied += len(edits)
			result.Files = append(result.Files, fileURI)
		}
	}

	sort.Strings(result.Files)
	return result, nil
}

// applyTextEdits applies a set of TextEdits to a file.
// Edits are applied in reverse order (from end to start) to preserve offsets.
func applyTextEdits(uri string, edits []gen.TextEdit) error {
	filePath := uriToPath(uri)

	source, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read %s: %w", filePath, err)
	}

	// Sort edits in reverse document order (later positions first).
	sortedEdits := make([]gen.TextEdit, len(edits))
	copy(sortedEdits, edits)
	sort.Slice(sortedEdits, func(i, j int) bool {
		ri, rj := sortedEdits[i].Range, sortedEdits[j].Range
		if ri.Start.Line != rj.Start.Line {
			return ri.Start.Line > rj.Start.Line
		}
		return ri.Start.Character > rj.Start.Character
	})

	result := source
	for _, edit := range sortedEdits {
		startByte, endByte := rangeToByteOffsets(result, edit.Range)
		var buf []byte
		buf = append(buf, result[:startByte]...)
		buf = append(buf, []byte(edit.NewText)...)
		buf = append(buf, result[endByte:]...)
		result = buf
	}

	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return fmt.Errorf("write %s: %w", filePath, err)
	}

	return nil
}

// pathToURI converts a filesystem path to a file:// URI.
func pathToURI(path string) string {
	if strings.HasPrefix(path, "file://") {
		return path
	}
	return "file://" + path
}
