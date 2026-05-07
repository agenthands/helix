// Package semantic implements the semantic skill, contributing 4 MCP tools
// for indexing, refreshing, inspecting, and querying the live semantic graph
// (Phase 64).
//
// This file (skill.go) is FINAL at end-of-W0 (Phase 64-03). Wave-1 (64-04)
// and Wave-2 (64-05/06/07) plans NEVER re-edit this file except to delete a
// single help-text stub line each from the stub-declaration block at the
// bottom of the file. Each stub deletion has zero conflict surface across
// plans because each plan removes a distinct constant.
package semantic

import (
	"context"
	"log/slog"
	"sync"

	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/internal/workspace"
)

// SemanticSkill implements skill.ToolProvider, exposing four MCP tools backed
// by the Phase 60-63 semantic engine: index_semantic_graph,
// refresh_semantic_graph, get_semantic_graph_status, get_semantic_context.
type SemanticSkill struct {
	mu        sync.Mutex
	logger    *slog.Logger
	store     StoreAccessor
	scheduler SchedulerAccessor
	queue     QueueAccessor
	live      LiveAccessor
	runner    RunnerAccessor
	retrieval RetrievalAccessor
	compactor CompactorAccessor
	session   SessionAccessor // injected: ctx -> *mcp.SessionInfo (closes checker W2)
}

func init() { skill.Register(&SemanticSkill{}) }

// Name returns the skill identifier.
func (s *SemanticSkill) Name() string { return "semantic" }

// Description returns a human-readable description.
func (s *SemanticSkill) Description() string {
	return "Semantic graph indexing and retrieval (4 tools)"
}

// Init initializes the skill with shared dependencies.
func (s *SemanticSkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}
	return nil
}

// ----- Post-init setters (mirror RepoMapSkill.SetEnrichFn). FINAL set; wave-1/wave-2 plans never add more. -----

// SetStore wires the StoreAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetStore(a StoreAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.store = a
}

// SetScheduler wires the SchedulerAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetScheduler(a SchedulerAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.scheduler = a
}

// SetQueue wires the QueueAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetQueue(a QueueAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.queue = a
}

// SetLive wires the LiveAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetLive(a LiveAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.live = a
}

// SetRunner wires the RunnerAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetRunner(a RunnerAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runner = a
}

// SetRetrieval wires the RetrievalAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetRetrieval(a RetrievalAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.retrieval = a
}

// SetCompactor wires the CompactorAccessor adapter (P64-08 daemon wiring).
func (s *SemanticSkill) SetCompactor(a CompactorAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.compactor = a
}

// SetSessionAccessor wires the per-request session lookup. Daemon (P64-08)
// passes the same closure used by InstallMiddleware (since getSession is an
// injected closure, NOT an exported package symbol). Closes checker W2.
func (s *SemanticSkill) SetSessionAccessor(a SessionAccessor) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.session = a
}

// sessionSnapshot returns the SessionSnapshot for the current request, or a
// zero-value snapshot when the daemon has not wired SessionAccessor (test
// path). This is the seam through which checkMode reads session.Mode.
func (s *SemanticSkill) sessionSnapshot(ctx context.Context) mcp.SessionSnapshot {
	s.mu.Lock()
	a := s.session
	s.mu.Unlock()
	if a == nil {
		return mcp.SessionSnapshot{}
	}
	sess := a.Session(ctx)
	if sess == nil {
		return mcp.SessionSnapshot{}
	}
	return sess.Snapshot()
}

// workspaceKey returns the WorkspaceKey for the current request. Returns the
// zero-value workspace.WorkspaceKey{} when SessionAccessor has not been wired
// (test path) or the session has no workspace bound.
//
// Implementation reads via SessionAccessor.Workspace(ctx) (not via the
// SessionSnapshot, which only carries scalar fields like Mode/Profile/Language
// plus a hashed WorkspaceKey string — not the canonical workspace.WorkspaceKey
// struct). The daemon's SessionAccessor adapter (P64-08) owns the translation
// from *mcp.SessionInfo to workspace.WorkspaceKey.
//
// Authored in this plan (W1 closure) so wave-1/wave-2 handler plans
// (64-04/05/06/07) consume `s.workspaceKey(ctx)` without re-editing skill.go.
func (s *SemanticSkill) workspaceKey(ctx context.Context) workspace.WorkspaceKey {
	s.mu.Lock()
	a := s.session
	s.mu.Unlock()
	if a == nil {
		return workspace.WorkspaceKey{}
	}
	return a.Workspace(ctx)
}

// Tools returns the 4 MCP tool definitions for semantic graph operations.
//
// HelpText constants (indexHelp, refreshHelp, statusHelp, contextHelp) are
// declared in tool-specific files added by Wave 1/2 plans:
//   - tools_index.go    (W1 / 64-04) owns the real `const indexHelp`.
//   - tools_refresh.go  (W2 / 64-05) owns the real `const refreshHelp`.
//   - tools_status.go   (W2 / 64-06) owns the real `const statusHelp`.
//   - tools_context.go  (W2 / 64-07) owns the real `const contextHelp`.
//
// Until those tool files land, the four var declarations at the bottom of
// this file (the "help-text stub block") provide compile-time placeholders.
// Each W1/W2 tool plan deletes its stub line AND adds the real `const`
// declaration in its tool file.
func (s *SemanticSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{
			Name:             "index_semantic_graph",
			Description:      "Build or refresh a committed semantic snapshot.",
			BriefDescription: "Index the semantic graph",
			HelpText:         indexHelp,
		},
		{
			Name:             "refresh_semantic_graph",
			Description:      "Apply pending live source changes (read+).",
			BriefDescription: "Refresh live overlay",
			HelpText:         refreshHelp,
		},
		{
			Name:             "get_semantic_graph_status",
			Description:      "Return semantic graph status (read+).",
			BriefDescription: "Semantic index status",
			HelpText:         statusHelp,
		},
		{
			Name:             "get_semantic_context",
			Description:      "Ranked, evidence-backed semantic context (read+).",
			BriefDescription: "Semantic context retrieval",
			HelpText:         contextHelp,
		},
	}
}

// GetSemanticSkill returns the registered SemanticSkill instance for daemon
// post-init wiring (P64-08). Returns nil if the skill has not been registered.
func GetSemanticSkill() *SemanticSkill {
	s, ok := skill.Get("semantic")
	if !ok {
		return nil
	}
	ss, ok := s.(*SemanticSkill)
	if !ok {
		return nil
	}
	return ss
}

// ----- Help-text stubs (W0 placeholder block). -----
//
// Each W1/W2 tool plan REPLACES its stub:
//   - 64-04 (W1) deletes `indexHelp` line below + introduces
//     `const indexHelp = "..."` in tools_index.go.
//   - 64-05 (W2) deletes `refreshHelp` line below + introduces
//     `const refreshHelp = "..."` in tools_refresh.go.
//   - 64-06 (W2) deletes `statusHelp` line below + introduces
//     `const statusHelp = "..."` in tools_status.go.
//   - 64-07 (W2) deletes `contextHelp` line below + introduces
//     `const contextHelp = "..."` in tools_context.go.
//
// Each deletion is a SINGLE LINE — zero overlap across plans, so wave-2
// plans can land in parallel. The vars (rather than consts) accommodate
// the const declaration that W1/W2 plans bring in their tool files.
var (
	contextHelp = "get_semantic_context: stub help (replaced by P64-07)"
)
