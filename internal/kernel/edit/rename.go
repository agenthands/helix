package edit

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
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
		return nil, serr.Wrap(serr.Internal, "rename", err)
	}

	result := &RenameResult{}

	// Apply changes from the WorkspaceEdit.Changes map.
	if wsEdit.Changes != nil {
		for fileURI, edits := range wsEdit.Changes {
			if err := applyTextEdits(fileURI, edits); err != nil {
				return nil, serr.Wrap(serr.Internal, "apply rename edits", err).WithDetail(fileURI)
			}
			result.FilesChanged++
			result.EditsApplied += len(edits)
			result.Files = append(result.Files, fileURI)
		}
	}

	// Also handle DocumentChanges (gopls and many LSP servers prefer this format).
	if len(wsEdit.DocumentChanges) > 0 && result.FilesChanged == 0 {
		for _, dc := range wsEdit.DocumentChanges {
			// DocumentChanges entries are union types; try to extract TextDocumentEdit.
			raw, err := json.Marshal(dc.Value)
			if err != nil {
				continue
			}
			var tde gen.TextDocumentEdit
			if err := json.Unmarshal(raw, &tde); err != nil || tde.TextDocument.URI == "" {
				continue
			}
			// Extract TextEdits from the union-typed edits.
			var edits []gen.TextEdit
			for _, e := range tde.Edits {
				eRaw, err := json.Marshal(e.Value)
				if err != nil {
					continue
				}
				var te gen.TextEdit
				if err := json.Unmarshal(eRaw, &te); err == nil {
					edits = append(edits, te)
				}
			}
			if len(edits) > 0 {
				if err := applyTextEdits(tde.TextDocument.URI, edits); err != nil {
					return nil, serr.Wrap(serr.Internal, "apply rename edits", err).WithDetail(tde.TextDocument.URI)
				}
				result.FilesChanged++
				result.EditsApplied += len(edits)
				result.Files = append(result.Files, tde.TextDocument.URI)
			}
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
		return serr.Wrap(serr.Internal, "read file", err).WithDetail(filePath)
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
		return serr.Wrap(serr.Internal, "write file", err).WithDetail(filePath)
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
