//go:build llm || llmjudge

package llm

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/test/harness"
)

// toolTaskDescriptions maps tool names to natural-language task descriptions
// for tool selection tests.
var toolTaskDescriptions = map[string]string{
	"activate_project":            "I need to set up my workspace to work on a specific project",
	"search_symbols":              "I want to find a symbol by name across the codebase",
	"find_references":             "I need to find all places where a specific symbol is used",
	"get_symbol_overview":         "I want to see the structure and members of a class or module",
	"get_hover_info":              "I need the type signature and documentation for a specific symbol at a location",
	"read_file":                   "I need to read the contents of a source file",
	"write_file":                  "I need to create or overwrite a file with new content",
	"search_in_files":             "I want to search for a text pattern across files in the project",
	"find_files":                  "I need to find files matching a name pattern in the project",
	"list_directory":              "I want to see what files and folders are in a directory",
	"go_to_definition":            "I need to navigate to where a symbol is defined",
	"find_implementations":        "I want to find all concrete implementations of an interface or abstract method",
	"get_call_hierarchy":          "I need to see what functions call a given function and what it calls",
	"get_type_hierarchy":          "I want to see the inheritance chain of a type",
	"get_diagnostics":             "I need to check for compilation errors and warnings in a file",
	"get_code_actions":            "I want to see available quick fixes and refactorings for a code location",
	"format_code":                 "I need to auto-format a source file",
	"edit_symbol_body":            "I need to replace the body of an existing function or method",
	"rename_symbol":               "I want to rename a symbol across all files that reference it",
	"replace_symbol":              "I need to completely replace a symbol definition with new code",
	"safe_delete_symbol":          "I want to safely remove a symbol only if nothing references it",
	"insert_before_symbol":        "I need to add new code before an existing symbol definition",
	"insert_after_symbol":         "I need to add new code after an existing symbol definition",
	"read_memory":                 "I want to read a previously stored memory note by its path",
	"write_memory":                "I need to store a note in the project's memory system",
	"list_memories":               "I want to see all stored memory notes",
	"search_memories":             "I need to find memory notes matching a search query",
	"switch_mode":                 "I need to change the operating mode (read, edit, admin)",
	"onboard_project":             "I want to get an overview of the project structure and conventions",
	"prepare_for_new_conversation": "I need to prepare a handoff summary for a new conversation",
}

// disambiguationTasks maps (toolA, toolB, target) to curated task descriptions
// designed to select the target tool from the pair.
var disambiguationTasks = map[[3]string]string{
	// search_symbols vs find_references
	{"search_symbols", "find_references", "search_symbols"}:   "I know part of a symbol name and want to find matching symbols",
	{"search_symbols", "find_references", "find_references"}:  "I have a specific symbol and want to find everywhere it's used",
	// get_symbol_overview vs get_hover_info
	{"get_symbol_overview", "get_hover_info", "get_symbol_overview"}: "I want to see all the methods and fields of a class",
	{"get_symbol_overview", "get_hover_info", "get_hover_info"}:     "I need the type annotation and docstring for a variable at line 42",
	// read_file vs read_memory
	{"read_file", "read_memory", "read_file"}:   "I need to read the source code of a specific file in the project",
	{"read_file", "read_memory", "read_memory"}: "I want to retrieve a previously saved note about the project architecture",
	// write_file vs write_memory
	{"write_file", "write_memory", "write_file"}:   "I need to create a new source file with Go code in the project",
	{"write_file", "write_memory", "write_memory"}: "I want to save a note about a design decision for future reference",
	// find_files vs search_in_files
	{"find_files", "search_in_files", "find_files"}:      "I need to find all files named config.yaml in the project",
	{"find_files", "search_in_files", "search_in_files"}:  "I want to find all files containing the string TODO in their content",
	// list_directory vs find_files
	{"list_directory", "find_files", "list_directory"}: "I want to see the immediate contents of the src/ directory",
	{"list_directory", "find_files", "find_files"}:     "I need to locate all .go files anywhere in the project tree",
	// insert_before_symbol vs insert_after_symbol
	{"insert_before_symbol", "insert_after_symbol", "insert_before_symbol"}: "I need to add a comment block right above an existing function",
	{"insert_before_symbol", "insert_after_symbol", "insert_after_symbol"}:  "I need to add a helper function right below an existing function",
	// edit_symbol_body vs replace_symbol
	{"edit_symbol_body", "replace_symbol", "edit_symbol_body"}: "I need to change the implementation inside a function while keeping its signature",
	{"edit_symbol_body", "replace_symbol", "replace_symbol"}:  "I need to completely rewrite a function including its signature and body",
	// get_call_hierarchy vs get_type_hierarchy
	{"get_call_hierarchy", "get_type_hierarchy", "get_call_hierarchy"}: "I need to trace which functions call a given function and what it calls downstream",
	{"get_call_hierarchy", "get_type_hierarchy", "get_type_hierarchy"}: "I want to see the parent types and subtypes in the inheritance chain of a class",
	// search_symbols vs search_in_files
	{"search_symbols", "search_in_files", "search_symbols"}:   "I want to find Go functions and types whose names contain 'Handler'",
	{"search_symbols", "search_in_files", "search_in_files"}:  "I want to grep for the literal string 'FIXME' across all source files",
	// onboard_project vs list_directory
	{"onboard_project", "list_directory", "onboard_project"}: "I want a high-level overview of the project's architecture, conventions, and key modules",
	{"onboard_project", "list_directory", "list_directory"}:  "I want to see what files exist in the root directory of the project",
}

// SelectionSystemPrompt returns the system prompt for tool selection tests.
func SelectionSystemPrompt(toolList string) string {
	return fmt.Sprintf(`You are an expert at selecting the right MCP tool for a task. You have access to the following tools:

%s

When given a task description, respond with ONLY the tool name that best accomplishes the task. No explanation, no formatting — just the exact tool name.`, toolList)
}

// SelectionUserPrompt returns the user prompt for tool selection tests.
func SelectionUserPrompt(taskDescription string) string {
	return fmt.Sprintf("Task: %s\n\nWhich tool should be used?", taskDescription)
}

// DisambiguationSystemPrompt returns the system prompt for disambiguation tests.
func DisambiguationSystemPrompt(toolA, descA, toolB, descB string) string {
	return fmt.Sprintf(`You are an expert at selecting the right MCP tool for a task. You have exactly two tools available:

- %s: %s
- %s: %s

When given a task, respond with ONLY the name of the more appropriate tool. No explanation.`, toolA, descA, toolB, descB)
}

// DisambiguationUserPrompt returns the user prompt for disambiguation tests.
func DisambiguationUserPrompt(taskDescription string) string {
	return fmt.Sprintf("Task: %s", taskDescription)
}

// FormatToolList formats a list of MCP tools as "- name: description" lines.
func FormatToolList(tools []*mcp.Tool) string {
	var sb strings.Builder
	for _, tool := range tools {
		fmt.Fprintf(&sb, "- %s: %s\n", tool.Name, tool.Description)
	}
	return sb.String()
}

// ToolTaskDescription returns a natural-language task description for a tool.
// Used in selection tests to prompt the LLM without revealing the tool name.
func ToolTaskDescription(toolName string) string {
	if desc, ok := toolTaskDescriptions[toolName]; ok {
		return desc
	}
	return fmt.Sprintf("I need to use the %s tool", toolName)
}

// DisambiguationTask returns a task description designed to select the target
// tool from a confusable pair.
func DisambiguationTask(toolA, toolB, target string) string {
	key := [3]string{toolA, toolB, target}
	if task, ok := disambiguationTasks[key]; ok {
		return task
	}
	// Fallback: use the generic tool task description for the target.
	return ToolTaskDescription(target)
}

// GoldenCase represents a single golden file loaded for interpretation testing.
type GoldenCase struct {
	ToolName string // e.g. "read_file"
	Scenario string // e.g. "success"
	Content  string // raw golden file content
}

// InterpretationSystemPrompt returns the system prompt for output interpretation tests (LLM-03/D-04).
func InterpretationSystemPrompt() string {
	return `You are an expert at interpreting MCP tool results. You will be shown the output of an MCP tool call.

Answer the following questions about the output:
1. STATUS: Did the tool call succeed, fail, or is the outcome unclear? Answer exactly one of: SUCCESS, FAILURE, UNCLEAR.
2. SUMMARY: In one sentence, what did the tool find or do?
3. LIMITATIONS: What can you NOT conclude from this output? List specific things the output does NOT tell you.
4. NEXT_ACTION: What would be a logical next step after seeing this output?

Respond in exactly this format:
STATUS: <SUCCESS|FAILURE|UNCLEAR>
SUMMARY: <one sentence>
LIMITATIONS: <comma-separated list>
NEXT_ACTION: <one sentence>`
}

// InterpretationUserPrompt returns the user prompt for output interpretation tests.
func InterpretationUserPrompt(toolName, goldenContent string) string {
	return fmt.Sprintf("Tool: %s\nOutput:\n%s", toolName, goldenContent)
}

// LoadGoldenFiles walks test/oracle/contract/testdata/golden/ and reads each
// .golden file, returning a slice of GoldenCase. Skips the errors/ subdirectory
// (use LoadErrorGoldenFiles for those).
func LoadGoldenFiles(t *testing.T) []GoldenCase {
	t.Helper()

	goldenRoot := filepath.Join(harness.ProjectRoot(), "test", "oracle", "contract", "testdata", "golden")
	var cases []GoldenCase

	entries, err := os.ReadDir(goldenRoot)
	if err != nil {
		t.Fatalf("reading golden root %s: %v", goldenRoot, err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Skip errors/ subdirectory — handled by LoadErrorGoldenFiles.
		if entry.Name() == "errors" {
			continue
		}
		toolDir := filepath.Join(goldenRoot, entry.Name())
		files, err := os.ReadDir(toolDir)
		if err != nil {
			t.Fatalf("reading golden dir %s: %v", toolDir, err)
		}
		for _, f := range files {
			if f.IsDir() || !strings.HasSuffix(f.Name(), ".golden") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(toolDir, f.Name()))
			if err != nil {
				t.Fatalf("reading golden file %s/%s: %v", entry.Name(), f.Name(), err)
			}
			scenario := strings.TrimSuffix(f.Name(), ".golden")
			cases = append(cases, GoldenCase{
				ToolName: entry.Name(),
				Scenario: scenario,
				Content:  string(data),
			})
		}
	}

	// Sort for deterministic ordering.
	sort.Slice(cases, func(i, j int) bool {
		if cases[i].ToolName != cases[j].ToolName {
			return cases[i].ToolName < cases[j].ToolName
		}
		return cases[i].Scenario < cases[j].Scenario
	})

	return cases
}

// LoadErrorGoldenFiles reads golden files from the errors/ subdirectory.
// These represent failure/error cases for interpretation testing.
func LoadErrorGoldenFiles(t *testing.T) []GoldenCase {
	t.Helper()

	errDir := filepath.Join(harness.ProjectRoot(), "test", "oracle", "contract", "testdata", "golden", "errors")
	var cases []GoldenCase

	files, err := os.ReadDir(errDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("reading error golden dir %s: %v", errDir, err)
	}

	for _, f := range files {
		if f.IsDir() || !strings.HasSuffix(f.Name(), ".golden") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(errDir, f.Name()))
		if err != nil {
			t.Fatalf("reading error golden file %s: %v", f.Name(), err)
		}
		scenario := strings.TrimSuffix(f.Name(), ".golden")
		cases = append(cases, GoldenCase{
			ToolName: "errors",
			Scenario: scenario,
			Content:  string(data),
		})
	}

	sort.Slice(cases, func(i, j int) bool {
		return cases[i].Scenario < cases[j].Scenario
	})

	return cases
}
