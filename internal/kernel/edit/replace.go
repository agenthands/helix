package edit

import (
	"context"
	"fmt"
	"os"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/fuzzy"
	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// FuzzyMatchInfo carries fuzzy match metadata back to the tool handler
// for response formatting (D-07).
type FuzzyMatchInfo struct {
	Strategy fuzzy.Strategy
	Score    float64
}

// ReplaceBody replaces a symbol's body using tree-sitter for precise body extraction.
// Per D-14: tree-sitter-first for body surgery, LSP-assisted for discovery.
// Per D-15: falls back to full symbol range replacement when tree-sitter unavailable.
func ReplaceBody(ctx context.Context, lease *lspool.WorkerLease, extractor *BodyExtractor, uri string, symbolName string, newBody string, lang string) error {
	plan, err := PlanEdit(ctx, lease, uri, symbolName, EditTypeReplaceBody, newBody)
	if err != nil {
		return serr.Wrap(serr.Internal, "plan edit", err)
	}
	_, err = ReplaceBodyWithPlan(ctx, lease, extractor, plan, lang, "")
	return err
}

// ReplaceBodyWithPlan executes a body replacement using a pre-computed plan.
// When searchBody is non-empty, the replacement is scoped to the fuzzy-matched
// sub-region within the tree-sitter/range-extracted body (FUZZ-05).
// Returns FuzzyMatchInfo when fuzzy matching was used, nil otherwise.
func ReplaceBodyWithPlan(ctx context.Context, lease *lspool.WorkerLease, extractor *BodyExtractor, plan *EditPlan, lang string, searchBody string) (*FuzzyMatchInfo, error) {
	filePath := uriToPath(plan.URI)

	source, err := os.ReadFile(filePath)
	if err != nil {
		return nil, serr.Wrap(serr.Internal, "read file", err).WithDetail(filePath)
	}

	var startByte, endByte uint

	// Try tree-sitter body extraction first (D-14).
	if extractor != nil && extractor.SupportsLanguage(lang) {
		startByte, endByte, err = extractor.ExtractBody(source, lang, plan.SymbolName, plan.Range)
		if err != nil {
			// Fall back to full symbol range (D-15).
			startByte, endByte = rangeToByteOffsets(source, plan.Range)
		}
	} else {
		// No tree-sitter grammar: fall back to full symbol range (D-15).
		startByte, endByte = rangeToByteOffsets(source, plan.Range)
	}

	if startByte > uint(len(source)) || endByte > uint(len(source)) || startByte > endByte {
		return nil, serr.New(serr.Internal, fmt.Sprintf("invalid byte range [%d:%d] for file of length %d", startByte, endByte, len(source)))
	}

	if searchBody != "" {
		// D-02: fuzzy-match searchBody within the extracted body region.
		bodyText := string(source[startByte:endByte])
		fResult, fErr := fuzzy.Match(bodyText, searchBody, fuzzy.Options{
			Replacement:   plan.NewContent,
			AllowEllipsis: false, // replace_symbol_body is symbol-oriented, no ellipsis
		})
		if fErr != nil {
			return nil, fErr
		}
		// Translate body-relative offsets to absolute file offsets (Pitfall 2).
		absStart := int(startByte) + fResult.StartByte
		absEnd := int(startByte) + fResult.EndByte

		// Replace using absolute offsets and reflowed replacement text.
		var result []byte
		result = append(result, source[:absStart]...)
		result = append(result, []byte(fResult.ReplacementText)...)
		result = append(result, source[absEnd:]...)

		if err := os.WriteFile(filePath, result, 0644); err != nil {
			return nil, serr.Wrap(serr.Internal, "write file", err).WithDetail(filePath)
		}
		if lease != nil {
			if err := notifyDidChange(ctx, lease, plan.URI, string(result)); err != nil {
				return nil, serr.Wrap(serr.Internal, "didChange notification", err)
			}
		}

		return &FuzzyMatchInfo{Strategy: fResult.Strategy, Score: fResult.Score}, nil
	}

	// Existing behavior: full body replace (no searchBody).
	var result []byte
	result = append(result, source[:startByte]...)
	result = append(result, []byte(plan.NewContent)...)
	result = append(result, source[endByte:]...)

	// Atomic write.
	if err := os.WriteFile(filePath, result, 0644); err != nil {
		return nil, serr.Wrap(serr.Internal, "write file", err).WithDetail(filePath)
	}

	// Notify language server of change.
	if lease != nil {
		if err := notifyDidChange(ctx, lease, plan.URI, string(result)); err != nil {
			return nil, serr.Wrap(serr.Internal, "didChange notification", err)
		}
	}

	return nil, nil
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
