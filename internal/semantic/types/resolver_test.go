package types

import (
	"context"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/agenthands/helix/internal/semantic/graph"
)

// fakeResolver is a per-test recording fake. It is shared across the
// chain/fixpoint/emit/dispatcher tests in this package.
type fakeResolver struct {
	// resolveChain returns the response for a given hop (i-th call). If the
	// slice is shorter than the call count, the LAST element is reused.
	resolveChain []ChainResponse
	calls        atomic.Int32
	lastReq      ChainRequest
	// err is returned from every call when non-nil.
	err error
}

func (f *fakeResolver) ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error) {
	idx := int(f.calls.Add(1)) - 1
	f.lastReq = req
	if f.err != nil {
		return ChainResponse{}, f.err
	}
	if len(f.resolveChain) == 0 {
		return ChainResponse{Resolved: false, Confidence: ConfidenceUnknown,
			EvidenceKind: EvidenceUnknown, ValidationState: "unresolved"}, nil
	}
	if idx >= len(f.resolveChain) {
		idx = len(f.resolveChain) - 1
	}
	return f.resolveChain[idx], nil
}

func (f *fakeResolver) ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error) {
	return SymbolResponse{}, nil
}

// TestDispatcher_KnownLanguage routes "go" requests to the registered Go
// resolver fake.
func TestDispatcher_KnownLanguage(t *testing.T) {
	goFake := &fakeResolver{resolveChain: []ChainResponse{{
		Resolved: true, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP,
		ValidationState: "validated",
	}}}
	d := NewDispatcher(map[string]Resolver{"go": goFake})
	resp, err := d.ResolveChain(context.Background(), ChainRequest{
		RepoID: "r", Language: "go", ChainTokens: []string{"x"},
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if goFake.calls.Load() != 1 {
		t.Fatalf("Go resolver calls = %d, want 1", goFake.calls.Load())
	}
	if !resp.Resolved || resp.Confidence != ConfidenceLSP {
		t.Fatalf("resp = %+v, want resolved+LSP", resp)
	}
}

// TestDispatcher_UnknownLanguage returns an unresolved 0.20 response when no
// resolver is registered for the request language (D-12).
func TestDispatcher_UnknownLanguage(t *testing.T) {
	d := NewDispatcher(map[string]Resolver{})
	resp, err := d.ResolveChain(context.Background(), ChainRequest{
		RepoID: "r", Language: "cobol", ChainTokens: []string{"x"},
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if resp.Resolved {
		t.Fatalf("resp.Resolved = true, want false for unknown language")
	}
	if resp.Confidence != ConfidenceUnknown {
		t.Fatalf("resp.Confidence = %v, want %v", resp.Confidence, ConfidenceUnknown)
	}
	if resp.ValidationState != "unresolved" {
		t.Fatalf("resp.ValidationState = %q, want unresolved", resp.ValidationState)
	}
	if !strings.Contains(resp.Reason, "no resolver") {
		t.Fatalf("resp.Reason = %q, want substring 'no resolver'", resp.Reason)
	}
}

// TestDispatcher_JavascriptSharesTypescript: javascript is aliased to the
// typescript-registered resolver (D-11).
func TestDispatcher_JavascriptSharesTypescript(t *testing.T) {
	tsFake := &fakeResolver{resolveChain: []ChainResponse{{
		Resolved: true, Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP,
		ValidationState: "validated",
	}}}
	d := NewDispatcher(map[string]Resolver{"typescript": tsFake})
	_, err := d.ResolveChain(context.Background(), ChainRequest{
		RepoID: "r", Language: "javascript", ChainTokens: []string{"x"},
	})
	if err != nil {
		t.Fatalf("ResolveChain err: %v", err)
	}
	if tsFake.calls.Load() != 1 {
		t.Fatalf("typescript resolver calls = %d, want 1 (javascript should alias)", tsFake.calls.Load())
	}
}

// TestDispatcher_ResolveSymbol_Unknown mirrors the unknown-language path for
// symbol requests.
func TestDispatcher_ResolveSymbol_Unknown(t *testing.T) {
	d := NewDispatcher(map[string]Resolver{})
	resp, err := d.ResolveSymbol(context.Background(), SymbolRequest{
		RepoID: "r", Language: "cobol", SymbolNodeID: graph.NodeID(1),
	})
	if err != nil {
		t.Fatalf("ResolveSymbol err: %v", err)
	}
	if resp.Resolved || resp.Confidence != ConfidenceUnknown {
		t.Fatalf("resp = %+v, want unresolved+0.20", resp)
	}
}
