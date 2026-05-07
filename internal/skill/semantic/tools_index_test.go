package semantic

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// ---------- mocks ----------

// mockSessionAccessor implements SessionAccessor for handler tests. The
// handler reads via s.sessionSnapshot(ctx) which calls Session(ctx).Snapshot();
// to inject a specific Mode we construct a real *mcp.SessionInfo and call
// RecordModeTransition (the only supported writer).
type mockSessionAccessor struct {
	ws   workspace.WorkspaceKey
	sess *mcp.SessionInfo
}

func (m *mockSessionAccessor) Session(ctx context.Context) *mcp.SessionInfo {
	return m.sess
}

func (m *mockSessionAccessor) Workspace(ctx context.Context) workspace.WorkspaceKey {
	return m.ws
}

// newSessionInfoWithMode builds a *mcp.SessionInfo with the given mode. The
// SessionInfo struct literal is permitted in tests per session.go's note:
// "Direct field access is retained only for single-writer initialization in
// the daemon bootstrap before the session is exposed to MCP handlers." Tests
// run before any concurrent reader, so direct init is safe.
func newSessionInfoWithMode(t *testing.T, mode string) *mcp.SessionInfo {
	t.Helper()
	return &mcp.SessionInfo{
		SessionID: "test-session",
		Mode:      mode,
	}
}

// recordedRunCall captures one runner.Run invocation so the
// AutoModeResolvesBeforeRun test can assert that Run was NEVER called with
// mode="auto".
type recordedRunCall struct {
	mode  string
	maxMs int
}

// mockRunnerAccessor implements RunnerAccessor with recording + injection.
type mockRunnerAccessor struct {
	resolveAutoReturn string
	runResult         IndexResult
	runErr            error
	resolveCalls      atomic.Int64
	runCallsMu        sync.Mutex
	runCalls          []recordedRunCall
}

func (m *mockRunnerAccessor) Run(ctx context.Context, ws workspace.WorkspaceKey, mode string, maxMs int) (IndexResult, error) {
	m.runCallsMu.Lock()
	m.runCalls = append(m.runCalls, recordedRunCall{mode: mode, maxMs: maxMs})
	m.runCallsMu.Unlock()
	return m.runResult, m.runErr
}

func (m *mockRunnerAccessor) ResolveAuto(ctx context.Context, ws workspace.WorkspaceKey) string {
	m.resolveCalls.Add(1)
	if m.resolveAutoReturn == "" {
		return "full"
	}
	return m.resolveAutoReturn
}

func (m *mockRunnerAccessor) runCallCount() int {
	m.runCallsMu.Lock()
	defer m.runCallsMu.Unlock()
	return len(m.runCalls)
}

func (m *mockRunnerAccessor) firstRunMode() string {
	m.runCallsMu.Lock()
	defer m.runCallsMu.Unlock()
	if len(m.runCalls) == 0 {
		return ""
	}
	return m.runCalls[0].mode
}

// newSkillForHandlerTest constructs a SemanticSkill with the given session
// mode + runner mock wired up. The workspace is a fixed test value.
func newSkillForHandlerTest(t *testing.T, mode string, runner *mockRunnerAccessor) *SemanticSkill {
	t.Helper()
	s := &SemanticSkill{}
	if err := s.Init(skill.SkillDeps{}); err != nil {
		t.Fatalf("Init: %v", err)
	}
	sess := newSessionInfoWithMode(t, mode)
	ws := workspace.WorkspaceKey{RepoRoot: "/tmp/repo-handler-test", Language: "go", Toolchain: "go1.22"}
	s.SetSessionAccessor(&mockSessionAccessor{ws: ws, sess: sess})
	s.SetRunner(runner)
	return s
}

// ---------- happy path ----------

func TestIndexHandler_HappyPath_ReturnsCommitted(t *testing.T) {
	runner := &mockRunnerAccessor{
		runResult: IndexResult{
			SnapshotID: 42,
			Status:     IndexStatusCommitted,
		},
	}
	s := newSkillForHandlerTest(t, "review", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	var out IndexResult
	if err := json.Unmarshal([]byte(textOf(res)), &out); err != nil {
		t.Fatalf("unmarshal result: %v (raw=%s)", err, textOf(res))
	}
	if out.SnapshotID != 42 || out.Status != IndexStatusCommitted {
		t.Fatalf("unexpected envelope: %+v", out)
	}
}

// ---------- mode violations ----------

func TestIndexHandler_ModeViolationFromRead(t *testing.T) {
	runner := &mockRunnerAccessor{}
	s := newSkillForHandlerTest(t, "read", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
	if !res.IsError {
		t.Fatalf("expected mode-violation error, got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "current mode is") {
		t.Fatalf("error envelope must contain 'current mode is', got: %s", textOf(res))
	}
	if runner.runCallCount() != 0 {
		t.Fatalf("runner.Run must NOT be called on mode violation; got %d calls", runner.runCallCount())
	}
}

func TestIndexHandler_ModeViolationFromEdit(t *testing.T) {
	runner := &mockRunnerAccessor{}
	s := newSkillForHandlerTest(t, "edit", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
	if !res.IsError {
		t.Fatalf("expected mode-violation error, got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "current mode is") {
		t.Fatalf("error envelope must contain 'current mode is', got: %s", textOf(res))
	}
	if runner.runCallCount() != 0 {
		t.Fatalf("runner.Run must NOT be called on mode violation; got %d calls", runner.runCallCount())
	}
}

func TestIndexHandler_AdminAllowed(t *testing.T) {
	runner := &mockRunnerAccessor{
		runResult: IndexResult{SnapshotID: 1, Status: IndexStatusCommitted},
	}
	s := newSkillForHandlerTest(t, "admin", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "full"})
	if res.IsError {
		t.Fatalf("admin mode should be allowed; got error: %s", textOf(res))
	}
	if runner.runCallCount() != 1 {
		t.Fatalf("runner.Run should be called once; got %d", runner.runCallCount())
	}
}

// ---------- path validation ----------

func TestIndexHandler_PathTraversalRejected(t *testing.T) {
	runner := &mockRunnerAccessor{}
	s := newSkillForHandlerTest(t, "review", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{
		Mode:  "full",
		Paths: []string{"../../../etc/passwd"},
	})
	if !res.IsError {
		t.Fatalf("expected path-traversal error, got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "path traversal") {
		t.Fatalf("error must mention 'path traversal', got: %s", textOf(res))
	}
	if runner.runCallCount() != 0 {
		t.Fatalf("runner.Run must NOT be called when path validation fails")
	}
}

func TestIndexHandler_AbsolutePathOutsideRoot(t *testing.T) {
	runner := &mockRunnerAccessor{}
	s := newSkillForHandlerTest(t, "review", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{
		Mode:  "full",
		Paths: []string{"/etc/passwd"},
	})
	if !res.IsError {
		t.Fatalf("expected absolute-path error, got success: %s", textOf(res))
	}
	if !strings.Contains(textOf(res), "outside workspace root") {
		t.Fatalf("error must mention 'outside workspace root', got: %s", textOf(res))
	}
	if runner.runCallCount() != 0 {
		t.Fatalf("runner.Run must NOT be called when path validation fails")
	}
}

// ---------- B5 closure: handler resolves auto BEFORE Run ----------

func TestIndexHandler_AutoModeResolvesBeforeRun(t *testing.T) {
	runner := &mockRunnerAccessor{
		resolveAutoReturn: "full",
		runResult:         IndexResult{SnapshotID: 7, Status: IndexStatusCommitted},
	}
	s := newSkillForHandlerTest(t, "review", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: "auto"})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if runner.resolveCalls.Load() != 1 {
		t.Fatalf("ResolveAuto must be called exactly once for mode=auto; got %d", runner.resolveCalls.Load())
	}
	if runner.runCallCount() != 1 {
		t.Fatalf("Run must be called exactly once; got %d", runner.runCallCount())
	}
	gotMode := runner.firstRunMode()
	if gotMode == "auto" || gotMode == "" {
		t.Fatalf("Run must NEVER be called with mode='auto' or empty (B5); got %q", gotMode)
	}
	if gotMode != "full" {
		t.Fatalf("Run mode arg: got %q, want %q (resolved value)", gotMode, "full")
	}
}

func TestIndexHandler_EmptyModeResolvesBeforeRun(t *testing.T) {
	runner := &mockRunnerAccessor{
		resolveAutoReturn: "incremental",
		runResult:         IndexResult{SnapshotID: 8, Status: IndexStatusCommitted},
	}
	s := newSkillForHandlerTest(t, "review", runner)

	res := s.handleIndexSemanticGraph(context.Background(), IndexSemanticGraphArgs{Mode: ""})
	if res.IsError {
		t.Fatalf("expected success, got error: %s", textOf(res))
	}
	if runner.resolveCalls.Load() != 1 {
		t.Fatalf("ResolveAuto must be called for empty mode; got %d", runner.resolveCalls.Load())
	}
	gotMode := runner.firstRunMode()
	if gotMode != "incremental" {
		t.Fatalf("Run mode arg: got %q, want %q (resolved value)", gotMode, "incremental")
	}
}

// ---------- helpers ----------

func textOf(res *mcpsdk.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	if tc, ok := res.Content[0].(*mcpsdk.TextContent); ok {
		return tc.Text
	}
	return ""
}
