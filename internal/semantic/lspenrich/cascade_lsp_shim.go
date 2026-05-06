// Phase 61 P05 — production cascadeLSPShim.
//
// cascadeLSPShim adapts a *lspool.WorkerLease into a lspenrich.CascadeLSP.
// It is the load-bearing seam between Manager.Run-dispatched enrichment
// jobs and the kernel's per-language LS pool.
//
// Lifetime: a fresh shim is constructed per cascade invocation by the
// CascadeLSPFactory closure that lives on Worker.NewCascadeLSP. The shim
// stores the supplied lease and a lazily-learned file:// URI; the URI is
// captured from the first DocumentSymbol(ctx, path) call (cascade.go
// §14.4 step 1 fires DocumentSymbol before any per-symbol LSP calls).
// Subsequent Hover/CallHierarchy/TypeHierarchy/Implementation calls reuse
// the cached URI; if Hover or peers are invoked before DocumentSymbol
// the shim returns (nil, nil) gracefully — no panic.
//
// Lease lifecycle: cascadeLSPShim NEVER releases the underlying lease.
// The Manager (P03) owns the lease lifecycle (B2 invariant per
// CONTEXT.md lines 492-494). The shim is a pure call-dispatch adapter.
//
// MethodNotFound contract: any -32601 LSP error is mapped to
// ErrMethodNotFound, the typed sentinel that cascade.go's IsMethodNotFound
// recognises and that records (lang, method) into the CapabilityCache.
// Soft "not applicable to this symbol" errors (e.g., gopls's "not a type
// name" on prepareTypeHierarchy for non-type symbols) are mapped to
// (nil, nil) so the cascade silently continues to the next step.

package lspenrich

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/agenthands/helix/internal/kernel/lspool"
	gen "github.com/agenthands/helix/protocol/gen"
)

// leaseRequester is the unexported test seam that *lspool.WorkerLease
// satisfies via its Request method. The cascade_lsp_shim_test.go
// fakeLease satisfies it without spinning a real LS subprocess.
//
// Production constructor NewCascadeLSPShim takes the concrete
// *lspool.WorkerLease (the public API surface) and the CascadeLSPFactory
// signature (lspenrich/worker.go) is unchanged — internally the shim
// just stores the concrete lease as a leaseRequester interface.
//
// Notify is intentionally NOT on this seam — the shim never sends LSP
// notifications (didOpen/didChange/etc. are owned by the foreground
// kernel sessions, not the enrichment shim).  Tests that need to drive
// didOpen against a real lease (integration_dispatch_test.go) call
// *lspool.WorkerLease.Notify directly on the concrete lease.
type leaseRequester interface {
	Request(ctx context.Context, method string, params, result any) error
}

// shimDocSym is the minimal cached projection of an LSP DocumentSymbol
// the shim retains for pickSymbolPosition. Only Name + selection-range
// start position are needed — the LSP types in protocol/gen carry far
// more fields the shim doesn't use.
type shimDocSym struct {
	name string
	pos  gen.Position
}

// cascadeLSPShim implements CascadeLSP via a *lspool.WorkerLease (or any
// leaseRequester). The shim is single-use per cascade and holds no state
// beyond the lease, the lazily-learned URI, and the documentSymbol cache.
type cascadeLSPShim struct {
	lease leaseRequester

	mu      sync.Mutex
	uri     string       // lazily learned from the first DocumentSymbol call
	docSyms []shimDocSym // populated from the documentSymbol response
}

// Compile-time assertion: cascadeLSPShim satisfies CascadeLSP.
var _ CascadeLSP = (*cascadeLSPShim)(nil)

// NewCascadeLSPShim is the production constructor. The CascadeLSPFactory
// signature `func(*lspool.WorkerLease) CascadeLSP` (worker.go) is
// preserved — internally we store the concrete lease as a leaseRequester
// interface so unit tests can substitute a fakeLease.
func NewCascadeLSPShim(lease *lspool.WorkerLease) CascadeLSP {
	return &cascadeLSPShim{lease: lease}
}

// DocumentSymbol fires textDocument/documentSymbol against the lease,
// captures the URI from the supplied path on first call, and caches the
// returned symbol shape so subsequent Hover/CallHierarchy/etc. can pick
// positions out of it.
func (s *cascadeLSPShim) DocumentSymbol(ctx context.Context, path string) ([]Symbol, error) {
	s.mu.Lock()
	if s.uri == "" && path != "" {
		s.uri = "file://" + path
	}
	uri := s.uri
	s.mu.Unlock()

	if uri == "" {
		return nil, nil
	}

	params := map[string]any{
		"textDocument": map[string]any{"uri": uri},
	}
	var raw json.RawMessage
	if err := s.lease.Request(ctx, "textDocument/documentSymbol", params, &raw); err != nil {
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// gopls returns []DocumentSymbol; some LSPs return []SymbolInformation —
	// try DocumentSymbol first.
	var docSyms []gen.DocumentSymbol
	if err := json.Unmarshal(raw, &docSyms); err == nil && len(docSyms) > 0 {
		s.cacheDocSyms(docSyms)
		return symbolsFromDocumentSymbols(docSyms, uri), nil
	}
	// Fall back to SymbolInformation.
	var syms []gen.SymbolInformation
	if err := json.Unmarshal(raw, &syms); err == nil {
		out := make([]Symbol, 0, len(syms))
		for _, si := range syms {
			out = append(out, Symbol{
				Name: si.Name,
				Path: uriToPath(si.Location.URI),
				Kind: kindLabel(si.Kind),
			})
		}
		return out, nil
	}
	return nil, nil
}

// cacheDocSyms walks the LSP DocumentSymbol tree and stores a flat
// (name, position) projection that pickSymbolPosition consults.
func (s *cascadeLSPShim) cacheDocSyms(docs []gen.DocumentSymbol) {
	flat := make([]shimDocSym, 0, len(docs))
	var walk func(syms []gen.DocumentSymbol)
	walk = func(syms []gen.DocumentSymbol) {
		for _, d := range syms {
			flat = append(flat, shimDocSym{name: d.Name, pos: d.SelectionRange.Start})
			if len(d.Children) > 0 {
				walk(d.Children)
			}
		}
	}
	walk(docs)

	s.mu.Lock()
	s.docSyms = flat
	s.mu.Unlock()
}

// DrainDiagnostics returns nil.  Phase 61 v1 does not expose a
// diagnostics buffer accessor on *lspool.WorkerLease; the production
// hook will be wired when the publishDiagnostics handler ships
// (Phase 62+).
func (s *cascadeLSPShim) DrainDiagnostics(_ string) []Diagnostic { return nil }

// pickSymbolPosition returns the first cached selection-range start
// matching sym.Name; falls back to the first cached symbol when nothing
// matches.  Returns ok=false when the documentSymbol cache is empty —
// callers short-circuit to (nil, nil) in that case.
func (s *cascadeLSPShim) pickSymbolPosition(sym Symbol) (gen.Position, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, d := range s.docSyms {
		if d.name == sym.Name {
			return d.pos, true
		}
	}
	if len(s.docSyms) > 0 {
		return s.docSyms[0].pos, true
	}
	return gen.Position{}, false
}

// uriOrEmpty returns the cached URI under the mu lock.  Used by the
// per-symbol calls to short-circuit when DocumentSymbol has not been
// invoked yet.
func (s *cascadeLSPShim) uriOrEmpty() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.uri
}

func (s *cascadeLSPShim) Hover(ctx context.Context, sym Symbol) (*Edge, error) {
	if s.uriOrEmpty() == "" {
		return nil, nil
	}
	pos, ok := s.pickSymbolPosition(sym)
	if !ok {
		return nil, nil
	}
	params := map[string]any{
		"textDocument": map[string]any{"uri": s.uriOrEmpty()},
		"position":     map[string]any{"line": pos.Line, "character": pos.Character},
	}
	var raw json.RawMessage
	if err := s.lease.Request(ctx, "textDocument/hover", params, &raw); err != nil {
		if isJSONRPCMethodNotFound(err) {
			return nil, ErrMethodNotFound
		}
		if isNonFatalLSPError(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	// Hover with a non-null body counts as a TYPE_OF edge with
	// confidence=1.0 + source="lsp.hover".
	return &Edge{
		Kind:            "TYPE_OF",
		Source:          "lsp.hover",
		Confidence:      1.0,
		ValidationState: "validated",
	}, nil
}

func (s *cascadeLSPShim) CallHierarchy(ctx context.Context, sym Symbol, _ int) ([]Edge, error) {
	if s.uriOrEmpty() == "" {
		return nil, nil
	}
	pos, ok := s.pickSymbolPosition(sym)
	if !ok {
		return nil, nil
	}
	prepareParams := map[string]any{
		"textDocument": map[string]any{"uri": s.uriOrEmpty()},
		"position":     map[string]any{"line": pos.Line, "character": pos.Character},
	}
	var items []gen.CallHierarchyItem
	if err := s.lease.Request(ctx, "textDocument/prepareCallHierarchy", prepareParams, &items); err != nil {
		if isJSONRPCMethodNotFound(err) {
			return nil, ErrMethodNotFound
		}
		if isNonFatalLSPError(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(items) == 0 {
		return nil, nil
	}
	// Outgoing calls from items[0] — these are the callees of the symbol,
	// each producing one CALLS edge.
	outParams := map[string]any{"item": items[0]}
	var outgoing []map[string]any
	if err := s.lease.Request(ctx, "callHierarchy/outgoingCalls", outParams, &outgoing); err != nil {
		if isJSONRPCMethodNotFound(err) {
			return nil, ErrMethodNotFound
		}
		if isNonFatalLSPError(err) {
			return nil, nil
		}
		return nil, err
	}
	edges := make([]Edge, 0, len(outgoing))
	for range outgoing {
		edges = append(edges, Edge{
			Kind:            "CALLS",
			Source:          "lsp.callHierarchy",
			Confidence:      1.0,
			ValidationState: "validated",
		})
	}
	return edges, nil
}

func (s *cascadeLSPShim) TypeHierarchy(ctx context.Context, sym Symbol, _ int) ([]Edge, error) {
	if s.uriOrEmpty() == "" {
		return nil, nil
	}
	pos, ok := s.pickSymbolPosition(sym)
	if !ok {
		return nil, nil
	}
	params := map[string]any{
		"textDocument": map[string]any{"uri": s.uriOrEmpty()},
		"position":     map[string]any{"line": pos.Line, "character": pos.Character},
	}
	var items []gen.TypeHierarchyItem
	if err := s.lease.Request(ctx, "textDocument/prepareTypeHierarchy", params, &items); err != nil {
		// gopls returns MethodNotFound for prepareTypeHierarchy on
		// non-type symbols; map to ErrMethodNotFound so the cascade
		// records the capability and continues.
		if isJSONRPCMethodNotFound(err) {
			return nil, ErrMethodNotFound
		}
		// gopls (and other LSes) often return non-fatal errors like
		// "not a type name" / "no symbol at this position" when the
		// position is on a non-type symbol.  These are not
		// LS-unavailable conditions — they're "this capability doesn't
		// apply to this symbol".  Map them to a silent no-op so the
		// cascade continues.
		if isNonFatalLSPError(err) {
			return nil, nil
		}
		return nil, err
	}
	edges := make([]Edge, 0, len(items))
	for range items {
		edges = append(edges, Edge{
			Kind:            "EXTENDS",
			Source:          "lsp.typeHierarchy",
			Confidence:      1.0,
			ValidationState: "validated",
		})
	}
	return edges, nil
}

func (s *cascadeLSPShim) Implementation(ctx context.Context, sym Symbol) ([]Edge, error) {
	if s.uriOrEmpty() == "" {
		return nil, nil
	}
	pos, ok := s.pickSymbolPosition(sym)
	if !ok {
		return nil, nil
	}
	params := map[string]any{
		"textDocument": map[string]any{"uri": s.uriOrEmpty()},
		"position":     map[string]any{"line": pos.Line, "character": pos.Character},
	}
	var raw json.RawMessage
	if err := s.lease.Request(ctx, "textDocument/implementation", params, &raw); err != nil {
		if isJSONRPCMethodNotFound(err) {
			return nil, ErrMethodNotFound
		}
		if isNonFatalLSPError(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var locs []gen.Location
	if err := json.Unmarshal(raw, &locs); err != nil {
		return nil, nil
	}
	edges := make([]Edge, 0, len(locs))
	for range locs {
		edges = append(edges, Edge{
			Kind:            "IMPLEMENTS",
			Source:          "lsp.implementation",
			Confidence:      1.0,
			ValidationState: "validated",
		})
	}
	return edges, nil
}

// Definition is a stub — Phase 61 v1's cascade ReferencesForSymbol
// returns nil so this method is never invoked from the production
// dispatch path.  Kept for interface completeness and future-phase
// (Phase 62+) reference resolution.
func (s *cascadeLSPShim) Definition(_ context.Context, _ Reference) (*Edge, error) {
	return nil, nil
}

// ReferencesForSymbol returns nil — Phase 61 v1 does not derive
// per-symbol references in the production shim.  Phase 62+ wires this
// to textDocument/references.
func (s *cascadeLSPShim) ReferencesForSymbol(_ Symbol) []Reference { return nil }

// =============================================================================
// Helpers — moved from cascade_integration_test.go.  Now package-internal
// (unexported) so the production shim, the integration test, and the
// upcoming integration_dispatch_test all share one implementation.
// =============================================================================

// symbolsFromDocumentSymbols flattens an LSP DocumentSymbol tree into
// the cascade-internal Symbol shape, walking children depth-first.
func symbolsFromDocumentSymbols(docs []gen.DocumentSymbol, uri string) []Symbol {
	out := make([]Symbol, 0, len(docs))
	var walk func(syms []gen.DocumentSymbol)
	walk = func(syms []gen.DocumentSymbol) {
		for _, s := range syms {
			out = append(out, Symbol{
				Name: s.Name,
				Path: uriToPath(uri),
				Kind: kindLabel(s.Kind),
			})
			if len(s.Children) > 0 {
				walk(s.Children)
			}
		}
	}
	walk(docs)
	return out
}

// isJSONRPCMethodNotFound reports whether err's textual form contains
// the canonical JSON-RPC -32601 marker.  The kernel's jsonrpc layer
// wraps these errors but the wrapping is internal to that package; the
// shim does string-matching to avoid importing internal/kernel/jsonrpc
// (the nosemantic2kernel vet analyzer forbids semantic→kernel/* imports
// other than internal/kernel/lspool — verified against
// internal/lint/nosemantic2kernel/analyzer.go's lspoolPkgPath carve-out).
//
// The matcher is case-insensitive and folds out whitespace variants so
// "Method Not Found", "MethodNotFound", and "method not found" all
// match.  In practice, kernel/jsonrpc.ResponseError.Error() returns
// only e.Message (no Code prefix), so the "-32601" branch never fires
// against errors produced by that path; it's retained because some
// LSP wrappers (and tests) embed the numeric code in the message text.
//
// TODO(phase-62+): when the kernel exposes a typed
// IsMethodNotFound(error) helper through *lspool.WorkerLease (or a
// third package outside kernel/), switch this matcher to the typed
// path so we match on Code == -32601 directly.
func isJSONRPCMethodNotFound(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "-32601") ||
		strings.Contains(s, "method not found") ||
		strings.Contains(s, "methodnotfound")
}

// isNonFatalLSPError reports whether err looks like a soft "this
// capability does not apply to this symbol" response from a real LSP —
// distinct from a hard LS-unavailable failure.  gopls returns errors
// like "not a type name" / "no symbol at this position" for
// prepareTypeHierarchy / prepareCallHierarchy when the cursor is on a
// non-type / non-callable symbol; these are NOT MethodNotFound (the
// method exists, the input is just inapplicable) but they're also not
// LS-unavailable.  The cascade should treat them as "no result, continue".
func isNonFatalLSPError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "not a type name") ||
		strings.Contains(s, "no symbol at this position") ||
		strings.Contains(s, "no identifier found") ||
		strings.Contains(s, "no type info") ||
		strings.Contains(s, "no object found") ||
		strings.Contains(s, "is a function, not a method") ||
		strings.Contains(s, "no implementation found") ||
		strings.Contains(s, "not a method") ||
		strings.Contains(s, "no implementations found") ||
		strings.Contains(s, "is not a type") ||
		strings.Contains(s, "no type hierarchy")
}

// uriToPath strips the file:// scheme prefix.
func uriToPath(uri string) string {
	if strings.HasPrefix(uri, "file://") {
		return strings.TrimPrefix(uri, "file://")
	}
	return uri
}

// kindLabel maps an LSP SymbolKind to a short human-readable label.
// The case constants are pulled from protocol/gen so the switch tracks
// any future metaModel regen automatically; the LSP 3.17 wire values are
// SymbolKindClass=5, SymbolKindMethod=6, SymbolKindFunction=12.
func kindLabel(k gen.SymbolKind) string {
	switch k {
	case gen.SymbolKindClass:
		return "Class"
	case gen.SymbolKindFunction:
		return "Function"
	case gen.SymbolKindMethod:
		return "Method"
	default:
		return fmt.Sprintf("Kind(%d)", k)
	}
}
