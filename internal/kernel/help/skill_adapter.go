package help

import (
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
)

// HelpSkill is a ToolProvider adapter that exposes the get_tool_help
// tool through the skill interface for daemon discovery.
type HelpSkill struct{}

func init() {
	skill.Register(&HelpSkill{})
}

// Name returns the unique skill identifier.
func (s *HelpSkill) Name() string { return "help" }

// Description returns a human-readable description of this skill.
func (s *HelpSkill) Description() string {
	return "Tool documentation (get_tool_help)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *HelpSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the ToolDefs for help. RegisterFn is nil because
// the daemon owns MCP registration (D-01).
func (s *HelpSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "get_tool_help", Description: "Get comprehensive documentation for any Helix tool including parameters, types, and usage examples", BriefDescription: "Get detailed usage documentation for a Helix tool"},
	}
}
