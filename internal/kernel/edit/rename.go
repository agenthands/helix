package edit

import (
	"context"
	"encoding/json"
	"os"
	"sort"
	"time"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// RenameResult summarizes the outcome of a rename operation.
type RenameResult struct {
	FilesChanged int
	EditsApplied int
	Files        []string
	// Strategy tags which dispatch path produced this result (D-07).
	// Populated by RenameSymbol on every nil-error return.
	Strategy RenameStrategy
}

// tryNativeRenameFn is a package-level seam for the native LSP rename path.
// Production code assigns the real tryNativeRename below; tests swap it to
// exercise the dispatcher matrix without a live language server.
var tryNativeRenameFn = tryNativeRename

// overriderResolverFn is a package-level seam that resolves a RenameOverrider
// for a given lease. Production code inspects lease.Adapter() (constructing
// the RustAnalyzerRenameOverride wrapper inline on the rust path); tests swap
// it to return a fakeOverrider. Returns nil when no override is available.
var overriderResolverFn = defaultOverriderResolver

// defaultOverriderResolver is the production implementation of overriderResolverFn.
// It looks at the lease's quirk adapter and returns a RenameOverrider when the
// adapter is a *lspool.RustAnalyzerAdapter (wrapped in RustAnalyzerRenameOverride
// to honor the lspool -> edit import-cycle constraint). A future generic
// RenameOverrider adapter (implementing the interface directly) would be
// returned as-is.
func defaultOverriderResolver(lease *lspool.WorkerLease) RenameOverrider {
	if lease == nil {
		return nil
	}
	adapter := lease.Adapter()
	if adapter == nil {
		return nil
	}
	if o, ok := any(adapter).(RenameOverrider); ok {
		return o
	}
	if ra, ok := any(adapter).(*lspool.RustAnalyzerAdapter); ok {
		return &RustAnalyzerRenameOverride{Inner: ra}
	}
	return nil
}

// RenameSymbol dispatches a rename request. It first attempts the native
// LSP path (textDocument/prepareRename + textDocument/rename); on failure,
// if the underlying LS adapter exposes a RenameOverrider (directly or via
// the rust-analyzer wrapper), it delegates to the override. On success the
// result carries a Strategy tag. If both paths fail, returns serr.Unsupported
// with a message pointing agents at fuzzy_edit, replace_symbol_body, or
// search_in_files (D-06).
func RenameSymbol(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, newName string) (*RenameResult, error) {
	// D-01 native-first readiness gate: if the lease's adapter is a
	// *lspool.RustAnalyzerAdapter, wait for rust-analyzer quiescence before
	// dispatching textDocument/rename. Best-effort: ignore return value.
	//
	// Budget protection: the wait is capped at half of any remaining ctx
	// deadline so the client-side fallback retains enough budget to run
	// textDocument/references + apply edits when native rename fails and
	// the quiescent signal never fires (e.g. rust-analyzer 1.90 on temp
	// workspaces; see RCA §1 and BUG-02).
	if lease != nil {
		if ra, ok := any(lease.Adapter()).(*lspool.RustAnalyzerAdapter); ok {
			waitCtx := ctx
			if dl, ok := ctx.Deadline(); ok {
				remaining := time.Until(dl)
				if remaining > 0 {
					var cancel context.CancelFunc
					waitCtx, cancel = context.WithTimeout(ctx, remaining/2)
					defer cancel()
				}
			}
			_ = ra.WaitUntilRenameReady(waitCtx) // best-effort; D-01 native-first readiness gate
		}
	}

	nativeResult, nativeErr := tryNativeRenameFn(ctx, lease, uri, line, col, newName)
	if nativeErr == nil {
		nativeResult.Strategy = StrategyLSPNative
		return nativeResult, nil
	}

	overrider := overriderResolverFn(lease)
	if overrider == nil {
		// No override available — surface the native error verbatim (current
		// behavior preserved for non-rust LSes).
		return nil, nativeErr
	}

	overrideResult, overrideErr := overrider.RenameOverride(ctx, lease, uri, line, col, newName)
	if overrideErr != nil {
		return nil, serr.New(serr.Unsupported,
			"rename_symbol cannot proceed for this symbol (native LSP rename failed and the language-specific override also failed); use fuzzy_edit, replace_symbol_body, or search_in_files for a manual rename").
			WithTool("rename_symbol").
			WithDetail(overrideErr.Error())
	}
	overrideResult.Strategy = StrategyRustClientSide
	return overrideResult, nil
}

// tryNativeRename performs the native LSP rename path:
// textDocument/prepareRename (position refinement) followed by
// textDocument/rename and WorkspaceEdit application.
// EDT-04: LSP rename forwarding.
func tryNativeRename(ctx context.Context, lease *lspool.WorkerLease, uri string, line, col int, newName string) (*RenameResult, error) {
	pos := gen.Position{
		Line:      uint32(line),
		Character: uint32(col),
	}

	// Send prepareRename first — some LS implementations (rust-analyzer) require
	// this to resolve the symbol before textDocument/rename succeeds. If it returns
	// a range, use its start position for the rename (more precise than user input).
	prepareParams := gen.PrepareRenameParams{
		TextDocumentPositionParams: gen.TextDocumentPositionParams{
			TextDocument: gen.TextDocumentIdentifier{URI: uri},
			Position:     pos,
		},
	}
	var prepareResult json.RawMessage
	prepareErr := lease.Request(ctx, "textDocument/prepareRename", prepareParams, &prepareResult)
	if prepareErr == nil && len(prepareResult) > 0 {
		// Try to extract a range from the prepare result and use its start position.
		// A real prepareRename range has Start <= End and End > Start; when that
		// invariant holds we trust the refined Start even if it's (0, 0) (e.g.
		// a symbol at the very first byte of a file). See Phase 47 REVIEW MN-02.
		var rangeResult struct {
			Start gen.Position `json:"start"`
			End   gen.Position `json:"end"`
		}
		if err := json.Unmarshal(prepareResult, &rangeResult); err == nil {
			if rangeResult.End.Line > rangeResult.Start.Line ||
				(rangeResult.End.Line == rangeResult.Start.Line && rangeResult.End.Character > rangeResult.Start.Character) {
				pos = rangeResult.Start
			}
		}
	}

	params := gen.RenameParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
		Position:     pos,
		NewName:      newName,
	}

	var wsEdit gen.WorkspaceEdit
	if err := lease.Request(ctx, "textDocument/rename", params, &wsEdit); err != nil {
		return nil, serr.Wrap(serr.Internal, "rename", err).WithDetail(err.Error())
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
