// skill_adapter.go — Phase 65 65-06 ToolProvider mirror of
// internal/kernel/health/skill_adapter.go. Exposes the strangler-fig-aware
// analyze_blast_radius tool through the skill interface for daemon discovery.
//
// The pre-existing SymbolRetrievalSkill (skill.go) already enumerates all
// nine symbol-retrieval tools as a catalog-only entry; SymbolsSkill is a
// kernel-resident sibling that ships the strangler-fig wiring sites for the
// blast-radius tool specifically. The daemon's catalog registration step is
// last-writer-wins on tool name, so listing analyze_blast_radius here does
// not cause MCP-SDK double-registration (the SDK AddTool call lives in
// registerAnalyzeBlastRadius).
package symbols

import (
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
)

// SymbolsSkill is a ToolProvider adapter for the strangler-fig-aware
// analyze_blast_radius tool. Mirrors the kernel/health and kernel/help
// skill_adapter.go pattern — the daemon owns MCP-SDK registration via
// RegisterTools, and this adapter contributes only the catalog (tools/list)
// entry.
type SymbolsSkill struct{}

func init() {
	skill.Register(&SymbolsSkill{})
}

// Name returns the unique skill identifier.
func (s *SymbolsSkill) Name() string { return "symbols" }

// Description returns a human-readable description of this skill.
func (s *SymbolsSkill) Description() string {
	return "Symbol analysis tools (analyze_blast_radius)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *SymbolsSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the ToolDefs for the strangler-fig blast-radius surface.
// RegisterFn is nil because the daemon owns MCP-SDK registration (D-01).
func (s *SymbolsSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{
			Name:             "analyze_blast_radius",
			Description:      "Analyze the blast radius (impact) of changing a symbol",
			BriefDescription: "Analyze the impact of changing a symbol",
		},
	}
}
