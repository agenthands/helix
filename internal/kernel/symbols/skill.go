package symbols

import (
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/skill"
)

// SymbolRetrievalSkill is a ToolProvider adapter that exposes the 9 symbol
// retrieval tools through the skill interface for daemon discovery.
type SymbolRetrievalSkill struct{}

func init() {
	skill.Register(&SymbolRetrievalSkill{})
}

// Name returns the unique skill identifier.
func (s *SymbolRetrievalSkill) Name() string { return "symbol-retrieval" }

// Description returns a human-readable description of this skill.
func (s *SymbolRetrievalSkill) Description() string {
	return "Symbol retrieval tools (go_to_definition, find_references, etc.)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *SymbolRetrievalSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the 9 ToolDefs for symbol retrieval. RegisterFn is nil because
// the daemon owns MCP registration (D-01).
func (s *SymbolRetrievalSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "go_to_definition", Description: "Go to the definition of a symbol at a given position"},
		{Name: "find_references", Description: "Find all references to a symbol at a given position"},
		{Name: "get_symbol_overview", Description: "Get a hierarchical outline of all symbols in a file"},
		{Name: "search_symbols", Description: "Search for symbols across the workspace by name"},
		{Name: "get_hover_info", Description: "Get hover/type information for a symbol at a given position"},
		{Name: "find_implementations", Description: "Find all implementations of an interface or abstract method"},
		{Name: "get_call_hierarchy", Description: "Get call hierarchy (callers and/or callees) for a symbol"},
		{Name: "get_type_hierarchy", Description: "Get type hierarchy (subtypes and/or supertypes) for a symbol"},
		{Name: "analyze_blast_radius", Description: "Analyze the blast radius (impact) of changing a symbol"},
	}
}
