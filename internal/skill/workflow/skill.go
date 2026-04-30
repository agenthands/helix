package workflow

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
)

// WorkflowSkill implements both skill.ToolProvider and skill.WorkflowProvider,
// providing onboarding and session handoff as MCP tools and prompt templates.
type WorkflowSkill struct {
	projectDir string
	logger     *slog.Logger
}

func init() {
	skill.Register(&WorkflowSkill{})
}

// Name returns the skill identifier.
func (s *WorkflowSkill) Name() string { return "workflow" }

// Description returns a human-readable description.
func (s *WorkflowSkill) Description() string {
	return "Onboarding and session management workflows"
}

// Init stores the project directory for later use.
func (s *WorkflowSkill) Init(deps skill.SkillDeps) error {
	s.projectDir = deps.ProjectDir
	s.logger = deps.Logger
	if s.logger == nil {
		s.logger = slog.Default()
	}
	return nil
}

// Prompts returns pre-rendered prompt templates for onboarding and handoff.
func (s *WorkflowSkill) Prompts() map[string]string {
	// Gather project info for the onboarding prompt.
	data := s.gatherProjectInfo()
	return map[string]string{
		"onboarding": renderOnboarding(data),
		"handoff":    renderHandoff(HandoffData{}),
	}
}

// Tools returns the MCP tool definitions for workflow operations.
func (s *WorkflowSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		s.onboardProjectTool(),
		s.prepareForNewConversationTool(),
	}
}

// --- help text constants ---

const onboardProjectHelp = `## Usage Examples

Run onboarding for the current project:
  onboard_project()

## Common Patterns
- Call once at the start of a session if onboarding has not been performed
- Returns a prompt guiding you to create project memories
- Detects languages, counts files, and analyzes project structure`

const prepareForNewConversationHelp = `## Usage Examples

Prepare a session handoff summary:
  prepare_for_new_conversation(tools_used=["find_symbol", "replace_symbol_body"], files_modified=["src/auth.go"], open_context="Refactoring auth middleware")

Save handoff as a memory:
  prepare_for_new_conversation(tools_used=["read_file"], open_context="Investigating bug #123", save_as_memory=true)

## Common Patterns
- Call at the end of a session to preserve context for the next session
- Include open_context to describe work in progress
- Set save_as_memory=true to persist the summary as a memory file`

func (s *WorkflowSkill) onboardProjectTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "onboard_project",
		Description: "Analyze the project structure, detect languages, count files, and return onboarding instructions. " +
			"Call this tool if onboarding has not been performed yet. Returns a prompt guiding you to create project memories.",
		BriefDescription: "Get an orientation guide for the current project",
		HelpText:         onboardProjectHelp,
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

func (s *WorkflowSkill) prepareForNewConversationTool() *mcp.ToolDef {
	return &mcp.ToolDef{
		Name: "prepare_for_new_conversation",
		Description: "Prepare a session handoff summary for continuation in a new conversation. " +
			"Takes session context (tools used, files modified, open questions) and produces a summary. " +
			"Optionally writes the summary as a session handoff memory.",
		BriefDescription: "Prepare a summary for handing off to the next session",
		HelpText:         prepareForNewConversationHelp,
		RegisterFn: func(server interface{}) error {
			return nil
		},
	}
}

// ExecuteTool runs a workflow tool by name with the given arguments.
func (s *WorkflowSkill) ExecuteTool(name string, args map[string]interface{}) (string, error) {
	switch name {
	case "onboard_project":
		return s.execOnboard()
	case "prepare_for_new_conversation":
		return s.execHandoff(args)
	default:
		return "", serr.New(serr.InvalidArgs, "unknown workflow tool").WithTool(name)
	}
}

func (s *WorkflowSkill) execOnboard() (string, error) {
	data := s.gatherProjectInfo()
	return renderOnboarding(data), nil
}

func (s *WorkflowSkill) execHandoff(args map[string]interface{}) (string, error) {
	data := HandoffData{
		SessionID:   fmt.Sprintf("session-%s", time.Now().Format("2006-01-02T15-04-05")),
		OpenContext: "",
	}

	if v, ok := args["tools_used"]; ok {
		data.ToolsUsed = toStringSlice(v)
	}
	if v, ok := args["files_modified"]; ok {
		data.FilesModified = toStringSlice(v)
	}
	if v, ok := args["memories_created"]; ok {
		data.MemoriesCreated = toStringSlice(v)
	}
	if v, ok := args["open_context"].(string); ok {
		data.OpenContext = v
	}

	result := renderHandoff(data)

	// If save_as_memory is true, write the handoff as a memory.
	if save, ok := args["save_as_memory"].(bool); ok && save {
		memName := fmt.Sprintf("session-handoff-%s", time.Now().Format("2006-01-02T15-04-05"))
		memPath := filepath.Join(s.projectDir, "memories", memName+".md")
		if err := os.MkdirAll(filepath.Dir(memPath), 0o755); err != nil {
			s.logger.Warn("failed to create memory dir for handoff", "error", err)
		} else if err := os.WriteFile(memPath, []byte(result), 0o644); err != nil {
			s.logger.Warn("failed to write handoff memory", "error", err)
		} else {
			result += fmt.Sprintf("\n\nHandoff summary saved as memory: %s", memName)
		}
	}

	return result, nil
}

// gatherProjectInfo walks the project directory to collect onboarding data.
func (s *WorkflowSkill) gatherProjectInfo() OnboardingData {
	// The project dir is .helix/, so the actual project root is one level up.
	projectRoot := filepath.Dir(s.projectDir)

	data := OnboardingData{
		ProjectRoot: projectRoot,
	}

	// Collect top-level directories.
	entries, err := os.ReadDir(projectRoot)
	if err != nil {
		s.logger.Warn("failed to read project root", "error", err)
		return data
	}

	langExtensions := map[string]string{
		".go":    "Go",
		".py":    "Python",
		".js":    "JavaScript",
		".ts":    "TypeScript",
		".java":  "Java",
		".rs":    "Rust",
		".rb":    "Ruby",
		".php":   "PHP",
		".c":     "C",
		".cpp":   "C++",
		".cs":    "C#",
		".kt":    "Kotlin",
		".swift": "Swift",
		".ex":    "Elixir",
		".hs":    "Haskell",
		".scala": "Scala",
		".vue":   "Vue",
		".pl":    "Perl",
		".sh":    "Shell",
		".tf":    "Terraform",
	}

	detectedLangs := make(map[string]bool)
	fileCount := 0

	for _, entry := range entries {
		name := entry.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		if entry.IsDir() {
			data.DirectoryStructure = append(data.DirectoryStructure, name+"/")
			// Walk one level into each dir to detect languages.
			subEntries, err := os.ReadDir(filepath.Join(projectRoot, name))
			if err != nil {
				continue
			}
			for _, sub := range subEntries {
				if !sub.IsDir() {
					fileCount++
					ext := filepath.Ext(sub.Name())
					if lang, ok := langExtensions[ext]; ok {
						detectedLangs[lang] = true
					}
				}
			}
		} else {
			fileCount++
			ext := filepath.Ext(name)
			if lang, ok := langExtensions[ext]; ok {
				detectedLangs[lang] = true
			}
		}
	}

	data.FileCount = fileCount
	for lang := range detectedLangs {
		data.DetectedLanguages = append(data.DetectedLanguages, lang)
	}

	return data
}

// toStringSlice converts an interface{} to a []string.
// Supports []interface{} (from JSON unmarshal) and []string.
func toStringSlice(v interface{}) []string {
	switch val := v.(type) {
	case []string:
		return val
	case []interface{}:
		result := make([]string, 0, len(val))
		for _, item := range val {
			if s, ok := item.(string); ok {
				result = append(result, s)
			}
		}
		return result
	case string:
		// Try JSON array
		var arr []string
		if err := json.Unmarshal([]byte(val), &arr); err == nil {
			return arr
		}
		return []string{val}
	default:
		return nil
	}
}
