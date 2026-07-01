package semantic

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/semantic/integ"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// fakeDataFlowReachabilityAccessor is a test double for
// DataFlowReachabilityAccessor — returns canned reachable nodes (or an error).
type fakeDataFlowReachabilityAccessor struct {
	reachable []ReachableNode
	err       error
}

func (f *fakeDataFlowReachabilityAccessor) ReachableFrom(ctx context.Context, repoID string, seed integ.SymbolID, maxHops int) ([]ReachableNode, error) {
	return f.reachable, f.err
}

func traceWS() workspace.WorkspaceKey {
	return workspace.WorkspaceKey{RepoRoot: "/tmp/repo-trace", Language: "go", Toolchain: "go1.22"}
}

// newTraceDataFlowHarness wires a SemanticSkill for trace_data_flow handler
// tests. Mirrors newChangeImpactHarness but with DataFlowReachability.
func newTraceDataFlowHarness(t *testing.T, mode string, reachable []ReachableNode) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	_ = s.Init(skill.SkillDeps{})
	s.SetSessionAccessor(&mockSessionAccessor{
		ws:   traceWS(),
		sess: &mcp.SessionInfo{SessionID: "test-trace", Mode: mode},
	})
	s.SetStore(&changeImpactStoreRec{t: t, graphVersion: 42, latestSnapshot: 7})
	s.SetExtractorRun(&fixExtractorRunForImpact{})
	s.SetSymbolByName(&fixSymbolByNameForImpact{results: []integ.SymbolID{"sk:src"}})
	s.SetDataFlowReachability(&fakeDataFlowReachabilityAccessor{reachable: reachable})
	return s
}

// TestTraceDataFlow_HappyPath: reachable nodes are shaped into the response with
// their hop counts; NodesCount is the pre-cap total; no fallback on success.
func TestTraceDataFlow_HappyPath(t *testing.T) {
	reachable := []ReachableNode{
		{SymbolID: "sk:src", Hops: 0},
		{SymbolID: "sk:mid", Hops: 1},
		{SymbolID: "sk:sink", Hops: 2},
	}
	s := newTraceDataFlowHarness(t, "read", reachable)
	res := s.handleTraceDataFlow(context.Background(), TraceDataFlowArgs{
		Seed: SeedInput{SymbolID: "sk:src"},
	})
	if res.IsError {
		t.Fatalf("handler error: %v", res)
	}
	var body TraceDataFlowResult
	if err := json.Unmarshal([]byte(extractText(t, res)), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.FallbackReason != "" {
		t.Errorf("FallbackReason = %q, want empty on success", body.FallbackReason)
	}
	if body.NodesCount != 3 || len(body.Reachable) != 3 {
		t.Fatalf("NodesCount=%d len(Reachable)=%d, want 3/3", body.NodesCount, len(body.Reachable))
	}
	last := body.Reachable[len(body.Reachable)-1]
	if last.SymbolID != "sk:sink" || last.Hops != 2 {
		t.Errorf("last reachable = %+v, want {sk:sink, 2}", last)
	}
}

// TestTraceDataFlow_HonestEmptyNoFallback: a genuinely-empty reachable set is
// NOT a fallback (it honestly means "nothing flows from this param").
func TestTraceDataFlow_HonestEmptyNoFallback(t *testing.T) {
	s := newTraceDataFlowHarness(t, "read", nil) // no reachable
	res := s.handleTraceDataFlow(context.Background(), TraceDataFlowArgs{
		Seed: SeedInput{SymbolID: "sk:src"},
	})
	if res.IsError {
		t.Fatalf("handler error: %v", res)
	}
	var body TraceDataFlowResult
	if err := json.Unmarshal([]byte(extractText(t, res)), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.FallbackReason != "" {
		t.Errorf("empty reachable set must NOT be a fallback; got %q", body.FallbackReason)
	}
	if body.NodesCount != 0 || len(body.Reachable) != 0 {
		t.Errorf("want empty reachable; got %+v", body)
	}
}

// TestTraceDataFlow_NilAccessorFallback: an unwired accessor degrades with the
// data_flow_lookup_unavailable fallback (NOT an honest-empty).
func TestTraceDataFlow_NilAccessorFallback(t *testing.T) {
	s := &SemanticSkill{}
	_ = s.Init(skill.SkillDeps{})
	s.SetSessionAccessor(&mockSessionAccessor{ws: traceWS(), sess: &mcp.SessionInfo{SessionID: "test-trace", Mode: "read"}})
	s.SetStore(&changeImpactStoreRec{t: t, graphVersion: 42, latestSnapshot: 7})
	s.SetExtractorRun(&fixExtractorRunForImpact{})
	s.SetSymbolByName(&fixSymbolByNameForImpact{results: []integ.SymbolID{"sk:src"}})
	// DataFlowReachability deliberately NOT wired.
	res := s.handleTraceDataFlow(context.Background(), TraceDataFlowArgs{
		Seed: SeedInput{SymbolID: "sk:src"},
	})
	var body TraceDataFlowResult
	if err := json.Unmarshal([]byte(extractText(t, res)), &body); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if body.FallbackReason != "data_flow_lookup_unavailable" {
		t.Errorf("FallbackReason = %q, want data_flow_lookup_unavailable", body.FallbackReason)
	}
}
