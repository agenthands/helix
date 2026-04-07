package edit

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// ReplaceBody replaces a symbol's body using tree-sitter for precise body extraction.
// Per D-14: tree-sitter-first for body surgery, LSP-assisted for discovery.
// Per D-15: falls back to full symbol range replacement when tree-sitter unavailable.
func ReplaceBody(ctx context.Context, lease *lspool.WorkerLease, extractor *BodyExtractor, uri string, symbolName string, newBody string, lang string) error {
	filePath := uriToPath(uri)

	source, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file %s: %w", filePath, err)
	}

	// Get DocumentSymbol via LSP to find symbol range.
	plan, err := PlanEdit(ctx, lease, uri, symbolName, EditTypeReplaceBody, newBody)
	if err != nil {
		return fmt.Errorf("plan edit: %w", err)
	}

	var startByte, endByte uint

	// Try tree-sitter body extraction first (D-14).
	if extractor != nil && extractor.SupportsLanguage(lang) {
		startByte, endByte, err = extractor.ExtractBody(source, lang, symbolName, plan.Range)
		if err != nil {
			// Fall back to full symbol range (D-15).
			startByte, endByte = rangeToByteOffsets(source, plan.Range)
		}
	} else {
		// No tree-sitter grammar: fall back to full symbol range (D-15).
		startByte, endByte = rangeToByteOffsets(source, plan.Range)
	}

	if startByte > uint(len(source)) || endByte > uint(len(source)) || startByte > endByte {
		return fmt.Errorf("invalid byte range [%d:%d] for file of length %d", startByte, endByte, len(source))
	}

	// Replace bytes from startByte to endByte with newBody.
	var result []byte
	result = append(result, source[:startByte]...)
	result = append(result, []byte(newBody)...)
	result = append(result, source[endByte:]...)

	// Atomic write.
	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return fmt.Errorf("write file %s: %w", filePath, err)
	}

	// Notify language server of change.
	if err := notifyDidChange(ctx, lease, uri, string(result)); err != nil {
		return fmt.Errorf("didChange notification: %w", err)
	}

	return nil
}

// rangeToByteOffsets converts an LSP Range to byte offsets in source.
func rangeToByteOffsets(source []byte, r gen.Range) (uint, uint) {
	lines := strings.SplitAfter(string(source), "\n")

	var startByte, endByte uint
	var currentByte uint

	for lineIdx, line := range lines {
		lineNum := uint32(lineIdx)
		lineLen := uint(len(line))

		if lineNum == r.Start.Line {
			startByte = currentByte + uint(r.Start.Character)
		}
		if lineNum == r.End.Line {
			endByte = currentByte + uint(r.End.Character)
			break
		}
		currentByte += lineLen
	}

	// Clamp to source bounds.
	srcLen := uint(len(source))
	if startByte > srcLen {
		startByte = srcLen
	}
	if endByte > srcLen {
		endByte = srcLen
	}

	return startByte, endByte
}

// notifyDidChange sends a textDocument/didChange notification to the LS worker.
func notifyDidChange(ctx context.Context, lease *lspool.WorkerLease, uri string, fullText string) error {
	params := gen.DidChangeTextDocumentParams{
		TextDocument: gen.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: gen.TextDocumentIdentifier{URI: uri},
			Version:                0, // version managed by workspace
		},
		ContentChanges: []gen.TextDocumentContentChangeEvent{
			{
				Value: map[string]interface{}{
					"text": fullText,
				},
			},
		},
	}
	return lease.Notify(ctx, "textDocument/didChange", params)
}

// uriToPath converts a file:// URI to a filesystem path.
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return uri[len("file://"):]
	}
	return uri
}
