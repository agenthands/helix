// Package skill defines the plugin/skill interface and registration system
// for extending Serena's tool set without touching core (WFL-03).
package skill

import (
	"log/slog"

	"github.com/postfix/serena/internal/mcp"
)

// SkillDeps provides dependencies that skills need during initialization.
type SkillDeps struct {
	// ProjectDir is the .serena/ directory for project-scoped data.
	ProjectDir string
	// GlobalDir is the ~/.serena/ directory for global data.
	GlobalDir string
	// Logger is the structured logger for skill output.
	Logger *slog.Logger
}

// Skill is a reusable capability package (D-13).
// Skills register via init() and are discoverable at runtime.
type Skill interface {
	// Name returns the unique identifier for this skill.
	Name() string
	// Description returns a human-readable description of this skill.
	Description() string
	// Init initializes the skill with shared dependencies.
	Init(deps SkillDeps) error
}

// ToolProvider is a Skill that contributes MCP tools (D-14 tool skill).
type ToolProvider interface {
	Skill
	// Tools returns the MCP tool definitions this skill provides.
	Tools() []*mcp.ToolDef
}

// WorkflowProvider is a Skill that contributes prompts/context (D-14 workflow skill).
type WorkflowProvider interface {
	Skill
	// Prompts returns a map of prompt name to prompt template content.
	Prompts() map[string]string
}
