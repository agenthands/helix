package guardrails

import (
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
)

// GuardrailsSkill is a no-tool skill that exists for Caddy-style init()
// registration. The actual enforcement path lives in
// internal/mcp/guardrail_middleware.go, wired by daemon.go step 14b.5.
// Blank-importing this package from internal/daemon/imports.go fires init()
// so that any future per-init side effects (e.g., additional sink wiring) are
// guaranteed to execute before daemon.go's bootstrap completes.
type GuardrailsSkill struct{}

func init() {
	skill.Register(&GuardrailsSkill{})
}

// Name returns the unique skill identifier.
func (s *GuardrailsSkill) Name() string { return "guardrails" }

// Description returns a human-readable description of this skill.
func (s *GuardrailsSkill) Description() string {
	return "Server-side guardrails for destructive MCP tools (Phase 66)"
}

// Init is a no-op; all guardrail wiring is performed by daemon.go step 14b.5.
func (s *GuardrailsSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns nil — this skill contributes no MCP tools.
// The middleware is wired directly by daemon.go, not through the ToolProvider surface.
func (s *GuardrailsSkill) Tools() []*mcp.ToolDef { return nil }
