package lspool

import (
	"context"

	gen "github.com/agenthands/helix/protocol/gen"
)

// LSAdapter provides typed LSP methods on top of a Worker.
// Each method delegates to worker.Request and handles capability checking.
type LSAdapter struct {
	worker *Worker
}

// NewLSAdapter creates a new LSP adapter wrapping the given worker.
func NewLSAdapter(worker *Worker) *LSAdapter {
	return &LSAdapter{worker: worker}
}

// Initialize sends the LSP initialize request. Typically called by Worker.Start() internally.
func (a *LSAdapter) Initialize(ctx context.Context, params gen.InitializeParams) (*gen.InitializeResult, error) {
	var result gen.InitializeResult
	if err := a.worker.Request(ctx, "initialize", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Definition returns the definition location(s) for the symbol at the given position.
func (a *LSAdapter) Definition(ctx context.Context, params gen.DefinitionParams) ([]gen.Location, error) {
	caps := a.worker.Capabilities()
	if caps.DefinitionProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.Location
	if err := a.worker.Request(ctx, "textDocument/definition", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// References returns all reference locations for the symbol at the given position.
func (a *LSAdapter) References(ctx context.Context, params gen.ReferenceParams) ([]gen.Location, error) {
	caps := a.worker.Capabilities()
	if caps.ReferencesProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.Location
	if err := a.worker.Request(ctx, "textDocument/references", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// DocumentSymbol returns the document symbols (outline) for the given document.
func (a *LSAdapter) DocumentSymbol(ctx context.Context, params gen.DocumentSymbolParams) ([]gen.DocumentSymbol, error) {
	caps := a.worker.Capabilities()
	if caps.DocumentSymbolProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.DocumentSymbol
	if err := a.worker.Request(ctx, "textDocument/documentSymbol", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Hover returns hover information for the symbol at the given position.
func (a *LSAdapter) Hover(ctx context.Context, params gen.HoverParams) (*gen.Hover, error) {
	caps := a.worker.Capabilities()
	if caps.HoverProvider == nil {
		return nil, ErrNotSupported
	}
	var result gen.Hover
	if err := a.worker.Request(ctx, "textDocument/hover", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

// Implementation returns implementation locations for the symbol at the given position.
func (a *LSAdapter) Implementation(ctx context.Context, params gen.ImplementationParams) ([]gen.Location, error) {
	caps := a.worker.Capabilities()
	if caps.ImplementationProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.Location
	if err := a.worker.Request(ctx, "textDocument/implementation", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// TypeDefinition returns the type definition location(s) for the symbol at the given position.
func (a *LSAdapter) TypeDefinition(ctx context.Context, params gen.TypeDefinitionParams) ([]gen.Location, error) {
	caps := a.worker.Capabilities()
	if caps.TypeDefinitionProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.Location
	if err := a.worker.Request(ctx, "textDocument/typeDefinition", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// WorkspaceSymbol searches for symbols across the workspace.
func (a *LSAdapter) WorkspaceSymbol(ctx context.Context, params gen.WorkspaceSymbolParams) ([]gen.SymbolInformation, error) {
	caps := a.worker.Capabilities()
	if caps.WorkspaceSymbolProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.SymbolInformation
	if err := a.worker.Request(ctx, "workspace/symbol", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Formatting formats the entire document.
func (a *LSAdapter) Formatting(ctx context.Context, params gen.DocumentFormattingParams) ([]gen.TextEdit, error) {
	caps := a.worker.Capabilities()
	if caps.DocumentFormattingProvider == nil {
		return nil, ErrNotSupported
	}
	var result []gen.TextEdit
	if err := a.worker.Request(ctx, "textDocument/formatting", params, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Rename performs a rename refactoring across the workspace.
func (a *LSAdapter) Rename(ctx context.Context, params gen.RenameParams) (*gen.WorkspaceEdit, error) {
	caps := a.worker.Capabilities()
	if caps.RenameProvider == nil {
		return nil, ErrNotSupported
	}
	var result gen.WorkspaceEdit
	if err := a.worker.Request(ctx, "textDocument/rename", params, &result); err != nil {
		return nil, err
	}
	return &result, nil
}
