package types

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

// stallResolver always returns unresolved. Used to test bounded iteration +
// TYPES-04 invariant.
type stallResolver struct {
	calls atomic.Int32
}

func (s *stallResolver) ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error) {
	s.calls.Add(1)
	return ChainResponse{
		Resolved: false, Confidence: ConfidenceUnknown, EvidenceKind: EvidenceUnknown,
		ValidationState: "",
	}, nil
}

func (s *stallResolver) ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error) {
	return SymbolResponse{}, nil
}

// TestFixpoint_NonConverged: 2 unresolved refs, fake never resolves them →
// every response carries ValidationState="unresolved" (TYPES-04 invariant).
func TestFixpoint_NonConverged(t *testing.T) {
	r := &stallResolver{}
	refs := []ChainRequest{
		{RepoID: "r", Language: "go", ChainTokens: []string{"a"}},
		{RepoID: "r", Language: "go", ChainTokens: []string{"b"}},
	}
	resps, err := FixpointResolve(context.Background(), r, refs, 8)
	if err != nil {
		t.Fatalf("FixpointResolve err: %v", err)
	}
	if len(resps) != 2 {
		t.Fatalf("len(resps) = %d, want 2", len(resps))
	}
	for i, resp := range resps {
		if resp.Resolved {
			t.Fatalf("resps[%d].Resolved = true, want false", i)
		}
		if resp.ValidationState != "unresolved" {
			t.Fatalf("resps[%d].ValidationState = %q, want unresolved (TYPES-04)", i, resp.ValidationState)
		}
	}
}

// TestFixpoint_EarlyExit: resolver returns Resolved=true on first call. Pass 1
// resolves; pass 2 has nothing to do (already-resolved skipped); state hash
// unchanged → exit. Resolver invoked exactly once.
func TestFixpoint_EarlyExit(t *testing.T) {
	calls := atomic.Int32{}
	r := &funcResolver{fn: func(ctx context.Context, req ChainRequest) (ChainResponse, error) {
		calls.Add(1)
		return ChainResponse{
			Resolved: true, Target: 1,
			Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP,
			ValidationState: "validated",
		}, nil
	}}
	refs := []ChainRequest{{RepoID: "r", Language: "go", ChainTokens: []string{"a"}}}
	resps, err := FixpointResolve(context.Background(), r, refs, 8)
	if err != nil {
		t.Fatalf("FixpointResolve err: %v", err)
	}
	if !resps[0].Resolved {
		t.Fatalf("expected resolved, got %+v", resps[0])
	}
	if calls.Load() != 1 {
		t.Fatalf("resolver calls = %d, want 1 (early-exit on no-progress)", calls.Load())
	}
}

// TestFixpoint_ProgressThenStall: 3 refs; pass 0 resolves "a"; pass 1 resolves
// "b"; pass 2 resolves nothing → state.Hash unchanged → exit. Final: 2
// resolved (a, b), 1 unresolved (c, ValidationState="unresolved").
func TestFixpoint_ProgressThenStall(t *testing.T) {
	r := newProgressResolver(map[string]int{
		"a": 0, // resolves at pass 0
		"b": 1, // resolves at pass 1
		// "c" never resolves
	})
	refs := []ChainRequest{
		{ChainTokens: []string{"a"}},
		{ChainTokens: []string{"b"}},
		{ChainTokens: []string{"c"}},
	}
	resps, err := FixpointResolve(context.Background(), r, refs, 8)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	count := 0
	for _, x := range resps {
		if x.Resolved {
			count++
		}
	}
	if count != 2 {
		t.Fatalf("resolved count = %d, want 2 (a+b; c stalls)", count)
	}
	if resps[2].Resolved {
		t.Fatalf("c should be unresolved")
	}
	if resps[2].ValidationState != "unresolved" {
		t.Fatalf("resps[2].ValidationState = %q, want unresolved (TYPES-04)", resps[2].ValidationState)
	}
}

// TestFixpoint_BoundedAtMaxIter: 10 refs all unresolved; resolver flips one
// new ref per pass. After maxIter=8 passes → 8 resolved, 2 unresolved. The
// fixpoint MUST stop at iter 8 even though progress was still happening.
func TestFixpoint_BoundedAtMaxIter(t *testing.T) {
	resolveAtPass := map[string]int{}
	for i := 0; i < 10; i++ {
		resolveAtPass["r"+strconv.Itoa(i)] = i
	}
	r := newProgressResolver(resolveAtPass)
	refs := make([]ChainRequest, 10)
	for i := range refs {
		refs[i] = ChainRequest{ChainTokens: []string{"r" + strconv.Itoa(i)}}
	}
	resps, err := FixpointResolve(context.Background(), r, refs, 8)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	count := 0
	for _, x := range resps {
		if x.Resolved {
			count++
		}
	}
	if count != 8 {
		t.Fatalf("resolved count = %d, want 8 (capped at maxIter=8)", count)
	}
	// Remaining 2 are unresolved → must carry ValidationState=unresolved.
	for i := range resps {
		if !resps[i].Resolved && resps[i].ValidationState != "unresolved" {
			t.Fatalf("resps[%d].ValidationState = %q, want unresolved", i, resps[i].ValidationState)
		}
	}
}

// progressResolver tracks a "current pass" (advances when we observe a fresh
// scan over previously-unresolved tokens). Each token has a "resolves at
// pass P" entry; we report Resolved=true once the current pass ≥ P.
type progressResolver struct {
	mu            sync.Mutex
	pass          int
	seenInPass    map[string]bool
	resolveAtPass map[string]int
}

func newProgressResolver(resolveAtPass map[string]int) *progressResolver {
	return &progressResolver{
		seenInPass:    make(map[string]bool),
		resolveAtPass: resolveAtPass,
	}
}

func (p *progressResolver) ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	tok := req.ChainTokens[0]
	if p.seenInPass[tok] {
		// A second scan within the same pass means a new pass started.
		p.pass++
		p.seenInPass = make(map[string]bool)
	}
	p.seenInPass[tok] = true
	want, ok := p.resolveAtPass[tok]
	if ok && p.pass >= want {
		return ChainResponse{
			Resolved: true, Target: 1,
			Confidence: ConfidenceLSP, EvidenceKind: EvidenceLSP,
			ValidationState: "validated",
		}, nil
	}
	return ChainResponse{
		Resolved: false, Confidence: ConfidenceUnknown, EvidenceKind: EvidenceUnknown,
	}, nil
}

func (p *progressResolver) ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error) {
	return SymbolResponse{}, nil
}

// funcResolver wraps a function as a Resolver. Used by TestFixpoint_EarlyExit.
type funcResolver struct {
	fn func(ctx context.Context, req ChainRequest) (ChainResponse, error)
}

func (f *funcResolver) ResolveChain(ctx context.Context, req ChainRequest) (ChainResponse, error) {
	return f.fn(ctx, req)
}

func (f *funcResolver) ResolveSymbol(ctx context.Context, req SymbolRequest) (SymbolResponse, error) {
	return SymbolResponse{}, nil
}
