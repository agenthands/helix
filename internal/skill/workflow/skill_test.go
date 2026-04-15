package workflow

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	serr "github.com/postfix/serena/internal/errors"
	"github.com/postfix/serena/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestWorkflow(t *testing.T) *WorkflowSkill {
	t.Helper()
	tmpDir := t.TempDir()

	// Create a fake project structure.
	projectRoot := tmpDir
	serenaDir := filepath.Join(projectRoot, ".serena")
	require.NoError(t, os.MkdirAll(serenaDir, 0o755))

	// Create some sample files to detect.
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "src", "lib.go"), []byte("package src"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "src", "util.py"), []byte("# python"), 0o644))

	globalDir := filepath.Join(tmpDir, "global")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))

	s := &WorkflowSkill{}
	err := s.Init(skill.SkillDeps{
		ProjectDir: serenaDir,
		GlobalDir:  globalDir,
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	require.NoError(t, err)
	return s
}

func TestWorkflowSkill_Name(t *testing.T) {
	s := &WorkflowSkill{}
	assert.Equal(t, "workflow", s.Name())
}

func TestWorkflowSkill_Description(t *testing.T) {
	s := &WorkflowSkill{}
	assert.NotEmpty(t, s.Description())
}

func TestWorkflowSkill_Init(t *testing.T) {
	s := setupTestWorkflow(t)
	assert.NotEmpty(t, s.projectDir)
}

func TestWorkflowSkill_Prompts(t *testing.T) {
	s := setupTestWorkflow(t)
	prompts := s.Prompts()

	assert.Contains(t, prompts, "onboarding")
	assert.Contains(t, prompts, "handoff")
	assert.NotEmpty(t, prompts["onboarding"])
	assert.NotEmpty(t, prompts["handoff"])
}

func TestWorkflowSkill_Tools_Returns2(t *testing.T) {
	s := setupTestWorkflow(t)
	tools := s.Tools()
	assert.Len(t, tools, 2)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	assert.Contains(t, names, "onboard_project")
	assert.Contains(t, names, "prepare_for_new_conversation")
}

func TestWorkflowSkill_OnboardProject(t *testing.T) {
	s := setupTestWorkflow(t)
	result, err := s.ExecuteTool("onboard_project", nil)
	require.NoError(t, err)

	// Should contain project info.
	assert.Contains(t, result, "Project Onboarding")
	assert.Contains(t, result, "src/")
	// Should detect Go and Python.
	assert.Contains(t, result, "Go")
	assert.Contains(t, result, "Python")
}

func TestWorkflowSkill_PrepareForNewConversation(t *testing.T) {
	s := setupTestWorkflow(t)
	result, err := s.ExecuteTool("prepare_for_new_conversation", map[string]interface{}{
		"tools_used":       []interface{}{"read_file", "find_symbol"},
		"files_modified":   []interface{}{"main.go", "lib.go"},
		"memories_created": []interface{}{"architecture/overview"},
		"open_context":     "Still need to implement the auth module.",
	})
	require.NoError(t, err)

	assert.Contains(t, result, "Session Handoff Summary")
	assert.Contains(t, result, "read_file")
	assert.Contains(t, result, "main.go")
	assert.Contains(t, result, "architecture/overview")
	assert.Contains(t, result, "auth module")
}

func TestWorkflowSkill_PrepareForNewConversation_Empty(t *testing.T) {
	s := setupTestWorkflow(t)
	result, err := s.ExecuteTool("prepare_for_new_conversation", map[string]interface{}{})
	require.NoError(t, err)

	assert.Contains(t, result, "Session Handoff Summary")
	assert.Contains(t, result, "None recorded")
}

func TestWorkflowSkill_PrepareForNewConversation_SaveAsMemory(t *testing.T) {
	s := setupTestWorkflow(t)
	result, err := s.ExecuteTool("prepare_for_new_conversation", map[string]interface{}{
		"save_as_memory": true,
		"open_context":   "Testing the save feature.",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "Handoff summary saved as memory")
}

func TestWorkflowSkill_RegisterViaInit(t *testing.T) {
	sk, ok := skill.Get("workflow")
	require.True(t, ok, "workflow skill should be registered via init()")
	assert.Equal(t, "workflow", sk.Name())

	_, isTP := sk.(skill.ToolProvider)
	assert.True(t, isTP, "workflow skill should implement ToolProvider")

	_, isWP := sk.(skill.WorkflowProvider)
	assert.True(t, isWP, "workflow skill should implement WorkflowProvider")
}

func TestWorkflowSkill_UnknownTool(t *testing.T) {
	s := setupTestWorkflow(t)
	_, err := s.ExecuteTool("nonexistent", nil)
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs), "expected InvalidArgs kind")
	assert.Contains(t, err.Error(), "unknown workflow tool")
}

func TestRenderOnboarding_WithData(t *testing.T) {
	data := OnboardingData{
		ProjectRoot:        "/tmp/test-project",
		DetectedLanguages:  []string{"Go", "Python"},
		FileCount:          42,
		DirectoryStructure: []string{"cmd/", "internal/", "pkg/"},
	}
	result := renderOnboarding(data)
	assert.Contains(t, result, "/tmp/test-project")
	assert.Contains(t, result, "Go")
	assert.Contains(t, result, "Python")
	assert.Contains(t, result, "42 files")
	assert.Contains(t, result, "cmd/")
}

func TestRenderHandoff_WithData(t *testing.T) {
	data := HandoffData{
		SessionID:       "session-test-123",
		ToolsUsed:       []string{"read_file", "write_memory"},
		FilesModified:   []string{"main.go"},
		MemoriesCreated: []string{"overview"},
		OpenContext:      "Need to finish error handling.",
	}
	result := renderHandoff(data)
	assert.Contains(t, result, "session-test-123")
	assert.Contains(t, result, "read_file")
	assert.Contains(t, result, "main.go")
	assert.Contains(t, result, "overview")
	assert.Contains(t, result, "error handling")
}

func TestToStringSlice(t *testing.T) {
	// []interface{}
	result := toStringSlice([]interface{}{"a", "b"})
	assert.Equal(t, []string{"a", "b"}, result)

	// []string
	result = toStringSlice([]string{"x", "y"})
	assert.Equal(t, []string{"x", "y"}, result)

	// JSON string
	result = toStringSlice(`["p","q"]`)
	assert.Equal(t, []string{"p", "q"}, result)

	// single string
	result = toStringSlice("hello")
	assert.Equal(t, []string{"hello"}, result)

	// nil
	result = toStringSlice(nil)
	assert.Nil(t, result)
}
