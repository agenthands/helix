package edit

import (
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/skill"
)

// SymbolEditingSkill is a ToolProvider adapter that exposes the 6 symbol
// editing tools through the skill interface for daemon discovery.
type SymbolEditingSkill struct{}

func init() {
	skill.Register(&SymbolEditingSkill{})
}

// Name returns the unique skill identifier.
func (s *SymbolEditingSkill) Name() string { return "symbol-editing" }

// Description returns a human-readable description of this skill.
func (s *SymbolEditingSkill) Description() string {
	return "Symbol editing tools (replace_symbol_body, rename_symbol, etc.)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *SymbolEditingSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the 6 ToolDefs for symbol editing. RegisterFn is nil because
// the daemon owns MCP registration (D-01).
func (s *SymbolEditingSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "replace_symbol_body", Description: "Replace a symbol's body with new content using tree-sitter for precise extraction"},
		{Name: "insert_before_symbol", Description: "Insert content immediately before a symbol"},
		{Name: "insert_after_symbol", Description: "Insert content immediately after a symbol"},
		{Name: "rename_symbol", Description: "Rename a symbol across all files in the workspace"},
		{Name: "safe_delete_symbol", Description: "Delete a symbol if it has no references; reports reference count if blocked"},
		{Name: "verify_edit", Description: "Check for compilation errors after an edit; returns diagnostic summary"},
	}
}
