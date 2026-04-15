// Package memory implements the memory skill, contributing 7 MCP tools
// for project and global memory persistence via the skill interface.
package memory

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/mcp"
	"github.com/postfix/serena/internal/memory"
	"github.com/postfix/serena/internal/skill"
)

// MemorySkill implements skill.ToolProvider, wrapping MemoryStore as MCP tools.
type MemorySkill struct {
	store  *memory.MemoryStore
	logger *slog.Logger
}

func init() {
	skill.Register(&MemorySkill{})
}

// Name returns the skill identifier.
func (s *MemorySkill) Name() string { return "memory" }

// Description returns a human-readable description.
func (s *MemorySkill) Description() string { return "Project and global memory persistence" }

// Init creates the underlying MemoryStore from skill dependencies.
func (s *MemorySkill) Init(deps skill.SkillDeps) error {
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}

	projectMemDir := filepath.Join(deps.ProjectDir, "memories")
	globalMemDir := filepath.Join(deps.GlobalDir, "memories")
	indexDBPath := filepath.Join(deps.ProjectDir, "index", "memories.db")

	store, err := memory.NewMemoryStore(projectMemDir, globalMemDir, indexDBPath, s.logger)
	if err != nil {
		return serr.Wrap(serr.Internal, "memory skill init", err)
	}
	s.store = store
	return nil
}

// Tools returns the 7 MCP tool definitions for memory CRUD operations.
func (s *MemorySkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		s.writeMemoryTool(),
		s.readMemoryTool(),
		s.listMemoriesTool(),
		s.searchMemoriesTool(),
		s.renameMemoryTool(),
		s.editMemoryTool(),
		s.deleteMemoryTool(),
	}
}

// writeMemoryTool creates or overwrites a memory with the given content.
func (s *MemorySkill) writeMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "write_memory",
		Description: "Write information about this project that can be useful for future tasks to a memory in md format. " +
			"The memory name should be meaningful and can include \"/\" to organize into topics (e.g., \"auth/login/logic\"). " +
			"Use the \"global/\" prefix for writing a memory that is shared across projects.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// readMemoryTool reads the content of a memory by name.
func (s *MemorySkill) readMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "read_memory",
		Description: "Read the content of a memory file. Only use if the information is relevant to the current task. " +
			"Infer relevance from the memory name. Do not read the same memory multiple times in one conversation.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// listMemoriesTool lists available memories, optionally filtered by scope and topic.
func (s *MemorySkill) listMemoriesTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "list_memories",
		Description: "List available memories, optionally filtered by scope (\"project\" or \"global\") and topic. " +
			"Any memory can be read using the read_memory tool.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// searchMemoriesTool performs full-text search over memories.
func (s *MemorySkill) searchMemoriesTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "search_memories",
		Description: "Search memories by full-text query, optionally filtered by scope (\"project\" or \"global\").",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// renameMemoryTool renames or moves a memory, supporting cross-scope moves.
func (s *MemorySkill) renameMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "rename_memory",
		Description: "Rename or move a memory. Use \"/\" in the name to organize into topics. " +
			"Moving between project and global scope is supported (e.g., renaming \"global/foo\" to \"bar\" moves from global to project scope).",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// editMemoryTool replaces content in a memory using search/replace.
func (s *MemorySkill) editMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "edit_memory",
		Description: "Edit a memory by replacing a search string with a replacement string. " +
			"The search is performed as a literal string match.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// deleteMemoryTool removes a memory file and its index entry.
func (s *MemorySkill) deleteMemoryTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "delete_memory",
		Description: "Delete a memory file. Should only be called if explicitly instructed or permission was granted by the user.",
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// ExecuteTool runs a memory tool by name with the given JSON arguments.
// Returns the tool result as a string.
func (s *MemorySkill) ExecuteTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "write_memory":
		return s.execWrite(args)
	case "read_memory":
		return s.execRead(args)
	case "list_memories":
		return s.execList(args)
	case "search_memories":
		return s.execSearch(args)
	case "rename_memory":
		return s.execRename(args)
	case "edit_memory":
		return s.execEdit(args)
	case "delete_memory":
		return s.execDelete(args)
	default:
		return "", serr.New(serr.InvalidArgs, "unknown memory tool").WithTool(name)
	}
}

func (s *MemorySkill) execWrite(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", serr.New(serr.InvalidArgs, "'name' parameter is required").WithTool("write_memory")
	}
	content, ok := args["content"].(string)
	if !ok {
		return "", serr.New(serr.InvalidArgs, "'content' parameter is required").WithTool("write_memory")
	}
	if err := s.store.Write(name, content); err != nil {
		return "", serr.Wrap(serr.Internal, "write operation failed", err).WithTool("write_memory")
	}
	return fmt.Sprintf("Memory %q written successfully.", name), nil
}

func (s *MemorySkill) execRead(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", serr.New(serr.InvalidArgs, "'name' parameter is required").WithTool("read_memory")
	}
	content, err := s.store.Read(name)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "read operation failed", err).WithTool("read_memory")
	}
	return content, nil
}

func (s *MemorySkill) execList(args map[string]interface{}) (string, error) {
	scope, _ := args["scope"].(string)
	topic, _ := args["topic"].(string)

	entries, err := s.store.List(scope, topic)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "list operation failed", err).WithTool("list_memories")
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "marshaling results", err).WithTool("list_memories")
	}
	return string(data), nil
}

func (s *MemorySkill) execSearch(args map[string]interface{}) (string, error) {
	query, ok := args["query"].(string)
	if !ok || query == "" {
		return "", serr.New(serr.InvalidArgs, "'query' parameter is required").WithTool("search_memories")
	}
	scope, _ := args["scope"].(string)

	entries, err := s.store.Search(query, scope)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "search operation failed", err).WithTool("search_memories")
	}
	data, err := json.Marshal(entries)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "marshaling results", err).WithTool("search_memories")
	}
	return string(data), nil
}

func (s *MemorySkill) execRename(args map[string]interface{}) (string, error) {
	oldName, ok := args["old_name"].(string)
	if !ok || oldName == "" {
		return "", serr.New(serr.InvalidArgs, "'old_name' parameter is required").WithTool("rename_memory")
	}
	newName, ok := args["new_name"].(string)
	if !ok || newName == "" {
		return "", serr.New(serr.InvalidArgs, "'new_name' parameter is required").WithTool("rename_memory")
	}
	if err := s.store.Rename(oldName, newName); err != nil {
		return "", serr.Wrap(serr.Internal, "rename operation failed", err).WithTool("rename_memory")
	}
	return fmt.Sprintf("Memory renamed from %q to %q.", oldName, newName), nil
}

func (s *MemorySkill) execEdit(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", serr.New(serr.InvalidArgs, "'name' parameter is required").WithTool("edit_memory")
	}
	search, ok := args["search"].(string)
	if !ok {
		return "", serr.New(serr.InvalidArgs, "'search' parameter is required").WithTool("edit_memory")
	}
	replace, ok := args["replace"].(string)
	if !ok {
		return "", serr.New(serr.InvalidArgs, "'replace' parameter is required").WithTool("edit_memory")
	}

	err := s.store.Edit(name, func(content string) string {
		return strings.ReplaceAll(content, search, replace)
	})
	if err != nil {
		return "", serr.Wrap(serr.Internal, "edit operation failed", err).WithTool("edit_memory")
	}
	return fmt.Sprintf("Memory %q edited successfully.", name), nil
}

func (s *MemorySkill) execDelete(args map[string]interface{}) (string, error) {
	name, ok := args["name"].(string)
	if !ok || name == "" {
		return "", serr.New(serr.InvalidArgs, "'name' parameter is required").WithTool("delete_memory")
	}
	if err := s.store.Delete(name); err != nil {
		return "", serr.Wrap(serr.Internal, "delete operation failed", err).WithTool("delete_memory")
	}
	return fmt.Sprintf("Memory %q deleted.", name), nil
}
