package health

import (
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
)

// HealthSkill is a ToolProvider adapter that exposes the get_health
// tool through the skill interface for daemon discovery.
type HealthSkill struct{}

func init() {
	skill.Register(&HealthSkill{})
}

// Name returns the unique skill identifier.
func (s *HealthSkill) Name() string { return "health" }

// Description returns a human-readable description of this skill.
func (s *HealthSkill) Description() string {
	return "Health monitoring tool (get_health)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *HealthSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the ToolDefs for health. RegisterFn is nil because
// the daemon owns MCP registration (D-01).
func (s *HealthSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "get_health", Description: "Get workspace health status and language server states", BriefDescription: "Check workspace health and language server status"},
	}
}
