package fileops

import (
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
)

// FileOpsSkill is a ToolProvider adapter that exposes the 7 file operation
// tools through the skill interface for daemon discovery.
type FileOpsSkill struct{}

func init() {
	skill.Register(&FileOpsSkill{})
}

// Name returns the unique skill identifier.
func (s *FileOpsSkill) Name() string { return "file-ops" }

// Description returns a human-readable description of this skill.
func (s *FileOpsSkill) Description() string {
	return "File operation tools (read_file, create_file, search_in_files, fuzzy_edit, etc.)"
}

// Init is a no-op; kernel tools get their dependencies from the daemon, not SkillDeps.
func (s *FileOpsSkill) Init(deps skill.SkillDeps) error { return nil }

// Tools returns the 7 ToolDefs for file operations. RegisterFn is nil because
// the daemon owns MCP registration (D-01).
func (s *FileOpsSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "read_file", Description: "Read a file's content, optionally a specific line range"},
		{Name: "create_file", Description: "Create a new file with content (errors if file already exists)"},
		{Name: "list_directory", Description: "List directory contents with file type, size, and modification time"},
		{Name: "find_files", Description: "Find files matching a glob pattern (supports ** for recursive matching)"},
		{Name: "search_in_files", Description: "Search for a regex pattern across the codebase, with optional context lines"},
		{Name: "replace_in_file", Description: "Replace all occurrences of a pattern in a file (literal or regex)"},
		{Name: "fuzzy_edit", Description: "Fuzzy-match and replace text in a file using 4-strategy cascade"},
	}
}
