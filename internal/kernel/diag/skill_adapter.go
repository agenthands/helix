package diag

import (
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/skill"
)

// DiagnosticsSkill is a ToolProvider adapter that exposes the 3 diagnostic
// tools through the skill interface for daemon discovery.
type DiagnosticsSkill struct{}

func init() {
	skill.Register(&DiagnosticsSkill{})
}

// Name returns the unique skill identifier.
func (s *DiagnosticsSkill) Name() string { return "diagnostics" }

// Description returns a human-readable description of this skill.
func (s *DiagnosticsSkill) Description() string {
	return "Diagnostic tools (get_diagnostics, get_code_actions, format_code)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *DiagnosticsSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the 3 ToolDefs for diagnostics. RegisterFn is nil because
// the daemon owns MCP registration (D-01).
func (s *DiagnosticsSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "get_diagnostics", Description: "Returns current diagnostics (errors, warnings) for a file"},
		{Name: "get_code_actions", Description: "Returns available code actions/quick fixes for a position or range"},
		{Name: "format_code", Description: "Formats a file via the language server and writes the result"},
	}
}
