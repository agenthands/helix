package diag

import (
	"context"
	"fmt"

	"github.com/postfix/serena/internal/kernel/lspool"
	gen "github.com/postfix/serena/protocol/gen"
)

// CodeActionResult is a simplified view of a CodeAction returned by the LS.
type CodeActionResult struct {
	Title       string
	Kind        string
	IsPreferred bool
	Edit        *gen.WorkspaceEdit // nil if action requires a command
	Diagnostics []gen.Diagnostic
}

// GetCodeActions sends textDocument/codeAction and returns the available actions
// for the given position or range in the file identified by uri.
// Line and column values are 0-indexed (LSP convention).
func GetCodeActions(ctx context.Context, lease *lspool.WorkerLease, uri string, startLine, startCol, endLine, endCol int) ([]CodeActionResult, error) {
	params := gen.CodeActionParams{
		TextDocument: gen.TextDocumentIdentifier{URI: uri},
		Range: gen.Range{
			Start: gen.Position{Line: uint32(startLine), Character: uint32(startCol)},
			End:   gen.Position{Line: uint32(endLine), Character: uint32(endCol)},
		},
		Context: gen.CodeActionContext{
			Diagnostics: []gen.Diagnostic{},
		},
	}

	var raw []gen.CodeAction
	if err := lease.Request(ctx, "textDocument/codeAction", params, &raw); err != nil {
		return nil, fmt.Errorf("codeAction request: %w", err)
	}

	results := make([]CodeActionResult, 0, len(raw))
	for _, ca := range raw {
		r := CodeActionResult{
			Title:       ca.Title,
			Edit:        ca.Edit,
			Diagnostics: ca.Diagnostics,
		}
		if ca.Kind != nil {
			r.Kind = string(*ca.Kind)
		}
		if ca.IsPreferred != nil {
			r.IsPreferred = *ca.IsPreferred
		}
		results = append(results, r)
	}
	return results, nil
}

// ApplyCodeAction applies a code action's workspace edit by writing file changes.
// If the action has no edit, it returns an error.
func ApplyCodeAction(ctx context.Context, lease *lspool.WorkerLease, action CodeActionResult) error {
	if action.Edit == nil {
		return fmt.Errorf("code action %q has no workspace edit (may require a command)", action.Title)
	}

	// WorkspaceEdit.Changes maps document URI -> []TextEdit.
	// Apply each edit via applyEdit or by notifying the client.
	// For now we use workspace/applyEdit to let the LS client apply it.
	applyParams := struct {
		Label string          `json:"label,omitempty"`
		Edit  gen.WorkspaceEdit `json:"edit"`
	}{
		Label: action.Title,
		Edit:  *action.Edit,
	}

	var applied struct {
		Applied bool `json:"applied"`
	}
	if err := lease.Request(ctx, "workspace/applyEdit", applyParams, &applied); err != nil {
		return fmt.Errorf("applying workspace edit: %w", err)
	}
	if !applied.Applied {
		return fmt.Errorf("workspace edit was not applied by the server")
	}
	return nil
}
