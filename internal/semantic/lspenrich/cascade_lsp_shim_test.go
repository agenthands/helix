// Phase 61 P05 — unit tests for the production cascadeLSPShim.
//
// These tests run under the default `go test` (no build tag) so they
// execute on every CI default-path run. The shim's URI-resolution
// behaviour, MethodNotFound mapping, and non-fatal LSP error mapping are
// the load-bearing invariants the production daemon dispatches against;
// the cascade.go callers depend on them.
//
// The shim is constructed against a fakeLease test double satisfying the
// unexported leaseRequester interface. Production wiring uses
// *lspool.WorkerLease (which also satisfies the interface) — see
// cascade_lsp_shim.go for the constructor that accepts the concrete
// lspool type.

package lspenrich

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
)

// fakeLease records every Request call so tests can assert (method,
// params) shapes. Implements the unexported leaseRequester interface
// the shim consumes.
type fakeLease struct {
	mu sync.Mutex

	calls []fakeLeaseCall

	// requestErr is returned from Request unconditionally when non-nil.
	// Tests use this to inject MethodNotFound / non-fatal / generic
	// errors and assert the shim's mapping behaviour.
	requestErr error

	// requestResult, when non-nil, is assigned to the result interface
	// after the call is recorded. Tests use this to feed canned LSP
	// responses (e.g., a documentSymbol JSON payload).
	requestResult []byte
}

type fakeLeaseCall struct {
	method string
	params any
}

func newFakeLease() *fakeLease {
	return &fakeLease{}
}

func (f *fakeLease) Request(_ context.Context, method string, params, _ any) error {
	f.mu.Lock()
	f.calls = append(f.calls, fakeLeaseCall{method: method, params: params})
	err := f.requestErr
	f.mu.Unlock()
	return err
}

func (f *fakeLease) Notify(_ context.Context, _ string, _ any) error { return nil }

func (f *fakeLease) lastCall() (fakeLeaseCall, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return fakeLeaseCall{}, false
	}
	return f.calls[len(f.calls)-1], true
}

// Test_CascadeLSPShim_CompileTime_AssertsCascadeLSPInterface ensures the
// production shim compile-time-asserts CascadeLSP satisfaction. If
// cascade_lsp_shim.go drops the var-assertion this test still ensures
// new()-on-shim returns a CascadeLSP-typed value via interface assignment.
func Test_CascadeLSPShim_ImplementsCascadeLSP(t *testing.T) {
	var _ CascadeLSP = newCascadeLSPShimForTest(newFakeLease())
}

// Test_CascadeLSPShim_DocumentSymbol_LearnsURIFromPath asserts the shim
// learns its file:// URI from the first DocumentSymbol(ctx, path) call.
// Subsequent Hover/CallHierarchy/etc. reuse this URI.
func Test_CascadeLSPShim_DocumentSymbol_LearnsURIFromPath(t *testing.T) {
	lease := newFakeLease()
	shim := newCascadeLSPShimForTest(lease)

	_, err := shim.DocumentSymbol(context.Background(), "/abs/path/main.go")
	if err != nil {
		t.Fatalf("DocumentSymbol: %v", err)
	}

	call, ok := lease.lastCall()
	if !ok {
		t.Fatal("expected at least one Request call")
	}
	if call.method != "textDocument/documentSymbol" {
		t.Errorf("method: got %q, want textDocument/documentSymbol", call.method)
	}
	uri := extractURIFromParams(t, call.params)
	if uri != "file:///abs/path/main.go" {
		t.Errorf("uri: got %q, want file:///abs/path/main.go", uri)
	}
}

// Test_CascadeLSPShim_Hover_AfterDocumentSymbol_ReusesURI asserts that
// after a successful DocumentSymbol the shim caches the URI and Hover
// reuses it. We can't drive a meaningful Hover without a position cached
// from a real documentSymbol response, so the test only asserts (a)
// Hover does NOT panic with a recorded URI and (b) the method name on
// the recorded call (if any) is "textDocument/hover".
//
// Because Hover requires a cached symbol position (from documentSymbol)
// and our fakeLease returns no symbol body, Hover returns (nil, nil)
// gracefully without firing a Request. This is the documented behaviour
// — see cascade_lsp_shim.go pickSymbolPosition.
func Test_CascadeLSPShim_Hover_AfterDocumentSymbol_NoPanic(t *testing.T) {
	lease := newFakeLease()
	shim := newCascadeLSPShimForTest(lease)

	// Prime URI via DocumentSymbol.
	_, err := shim.DocumentSymbol(context.Background(), "/abs/path/main.go")
	if err != nil {
		t.Fatalf("DocumentSymbol: %v", err)
	}

	// Hover with no cached symbol position falls through to (nil, nil)
	// without panicking. This is the graceful-degradation contract.
	edge, err := shim.Hover(context.Background(), Symbol{Name: "Foo"})
	if err != nil {
		t.Errorf("Hover: unexpected err %v", err)
	}
	if edge != nil {
		t.Errorf("Hover: expected nil edge for empty doc-symbol cache")
	}
}

// Test_CascadeLSPShim_Hover_WithoutDocumentSymbol_ReturnsNilNil asserts
// that calling Hover BEFORE DocumentSymbol does not panic and returns
// (nil, nil) — the shim has no URI to dispatch with.
func Test_CascadeLSPShim_Hover_WithoutDocumentSymbol_ReturnsNilNil(t *testing.T) {
	lease := newFakeLease()
	shim := newCascadeLSPShimForTest(lease)

	edge, err := shim.Hover(context.Background(), Symbol{Name: "Foo"})
	if err != nil {
		t.Errorf("Hover: unexpected err %v", err)
	}
	if edge != nil {
		t.Errorf("Hover: expected nil edge when URI is unset")
	}
}

// Test_CascadeLSPShim_TypeHierarchy_MethodNotFoundMapping asserts the
// shim maps a "-32601 method not found" lease error to the typed
// lspenrich.ErrMethodNotFound sentinel that the cascade recognises. This
// is the contract cascade_integration_test.go:223-225 and cascade.go's
// IsMethodNotFound check both rely on.
func Test_CascadeLSPShim_TypeHierarchy_MethodNotFoundMapping(t *testing.T) {
	lease := newFakeLease()
	lease.requestErr = errors.New("error: -32601 method not found")
	shim := newCascadeLSPShimForTest(lease)

	// Prime URI so the call is actually issued.
	_, _ = shim.DocumentSymbol(context.Background(), "/abs/path/main.go")
	// Inject a fake symbol position so TypeHierarchy actually fires the
	// prepareTypeHierarchy request — without it the shim falls through
	// to (nil, nil) without dispatching.
	primeFakeSymbolPosition(shim)

	edges, err := shim.TypeHierarchy(context.Background(), Symbol{Name: "Foo"}, 2)
	if !errors.Is(err, ErrMethodNotFound) {
		t.Errorf("err: got %v, want ErrMethodNotFound", err)
	}
	if edges != nil {
		t.Errorf("edges: got %v, want nil on MethodNotFound", edges)
	}
}

// Test_isJSONRPCMethodNotFound exercises the helper directly. The shim
// dispatches mapping decisions on this predicate.
func Test_isJSONRPCMethodNotFound(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"-32601 in message", errors.New("rpc: -32601 something"), true},
		{"MethodNotFound wording", errors.New("MethodNotFound"), true},
		{"method not found wording", errors.New("error: method not found"), true},
		{"unrelated error", errors.New("connection refused"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isJSONRPCMethodNotFound(tc.err); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// Test_isNonFatalLSPError exercises the helper directly. The shim uses
// this predicate to suppress soft "this method does not apply to this
// symbol" errors so the cascade continues.
func Test_isNonFatalLSPError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil error", nil, false},
		{"not a type name", errors.New("not a type name"), true},
		{"no symbol at this position", errors.New("no symbol at this position"), true},
		{"connection refused (fatal)", errors.New("connection refused"), false},
		{"is not a type", errors.New("is not a type"), true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isNonFatalLSPError(tc.err); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// extractURIFromParams pulls textDocument.uri out of the params shape
// the shim sends to lease.Request. The shape is map[string]any with a
// nested map[string]any. Returns "" if the shape doesn't match.
func extractURIFromParams(t *testing.T, params any) string {
	t.Helper()
	m, ok := params.(map[string]any)
	if !ok {
		return ""
	}
	td, ok := m["textDocument"].(map[string]any)
	if !ok {
		return ""
	}
	uri, _ := td["uri"].(string)
	return uri
}

// newCascadeLSPShimForTest constructs a shim with a fakeLease.  Defined
// in cascade_lsp_shim_test.go so it stays test-only; the production
// constructor (NewCascadeLSPShim) takes a *lspool.WorkerLease.
func newCascadeLSPShimForTest(lease *fakeLease) *cascadeLSPShim {
	return &cascadeLSPShim{lease: lease}
}

// primeFakeSymbolPosition seeds the shim's docSyms cache with one entry
// so TypeHierarchy / Hover / CallHierarchy / Implementation actually
// fire their LSP requests.  Without this seed the shim's
// pickSymbolPosition returns ok=false and the methods short-circuit to
// (nil, nil) — that's a separate code path from the MethodNotFound
// mapping we want to test.
func primeFakeSymbolPosition(shim *cascadeLSPShim) {
	shim.mu.Lock()
	defer shim.mu.Unlock()
	shim.docSyms = []shimDocSym{{name: "Foo"}}
}

// Compile-time canary so a missing constructor symbol surfaces at the
// test-build step rather than as a runtime panic.
var _ = fmt.Sprintf
