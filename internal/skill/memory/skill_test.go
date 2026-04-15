package memory

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

func setupTestSkill(t *testing.T) *MemorySkill {
	t.Helper()
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, ".serena")
	globalDir := filepath.Join(tmpDir, "global")

	s := &MemorySkill{}
	err := s.Init(skill.SkillDeps{
		ProjectDir: projectDir,
		GlobalDir:  globalDir,
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError})),
	})
	require.NoError(t, err)
	t.Cleanup(func() { s.store.Close() })
	return s
}

func TestMemorySkill_Name(t *testing.T) {
	s := &MemorySkill{}
	assert.Equal(t, "memory", s.Name())
}

func TestMemorySkill_Description(t *testing.T) {
	s := &MemorySkill{}
	assert.NotEmpty(t, s.Description())
}

func TestMemorySkill_Init(t *testing.T) {
	s := setupTestSkill(t)
	assert.NotNil(t, s.store)
}

func TestMemorySkill_Tools_Returns7(t *testing.T) {
	s := setupTestSkill(t)
	tools := s.Tools()
	assert.Len(t, tools, 7)

	names := make([]string, len(tools))
	for i, tool := range tools {
		names[i] = tool.Name
	}
	assert.Contains(t, names, "write_memory")
	assert.Contains(t, names, "read_memory")
	assert.Contains(t, names, "list_memories")
	assert.Contains(t, names, "search_memories")
	assert.Contains(t, names, "rename_memory")
	assert.Contains(t, names, "edit_memory")
	assert.Contains(t, names, "delete_memory")
}

func TestMemorySkill_WriteReadRoundtrip(t *testing.T) {
	s := setupTestSkill(t)

	// Write
	result, err := s.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "test/hello",
		"content": "Hello, world!",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "written successfully")

	// Read
	content, err := s.ExecuteTool("read_memory", map[string]interface{}{
		"name": "test/hello",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello, world!", content)
}

func TestMemorySkill_SearchAfterWrite(t *testing.T) {
	s := setupTestSkill(t)

	_, err := s.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "architecture/overview",
		"content": "The system uses a microservices architecture with gRPC communication.",
	})
	require.NoError(t, err)

	result, err := s.ExecuteTool("search_memories", map[string]interface{}{
		"query": "microservices",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "architecture/overview")
}

func TestMemorySkill_ListMemories(t *testing.T) {
	s := setupTestSkill(t)

	_, err := s.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "notes/first",
		"content": "First note",
	})
	require.NoError(t, err)

	result, err := s.ExecuteTool("list_memories", map[string]interface{}{})
	require.NoError(t, err)
	assert.Contains(t, result, "notes/first")
}

func TestMemorySkill_RenameMemory(t *testing.T) {
	s := setupTestSkill(t)

	_, err := s.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "old-name",
		"content": "some content",
	})
	require.NoError(t, err)

	_, err = s.ExecuteTool("rename_memory", map[string]interface{}{
		"old_name": "old-name",
		"new_name": "new-name",
	})
	require.NoError(t, err)

	// Verify old name is gone, new name exists
	_, err = s.ExecuteTool("read_memory", map[string]interface{}{
		"name": "old-name",
	})
	assert.Error(t, err)

	content, err := s.ExecuteTool("read_memory", map[string]interface{}{
		"name": "new-name",
	})
	require.NoError(t, err)
	assert.Equal(t, "some content", content)
}

func TestMemorySkill_EditMemory(t *testing.T) {
	s := setupTestSkill(t)

	_, err := s.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "editable",
		"content": "Hello, world!",
	})
	require.NoError(t, err)

	_, err = s.ExecuteTool("edit_memory", map[string]interface{}{
		"name":    "editable",
		"search":  "world",
		"replace": "Go",
	})
	require.NoError(t, err)

	content, err := s.ExecuteTool("read_memory", map[string]interface{}{
		"name": "editable",
	})
	require.NoError(t, err)
	assert.Equal(t, "Hello, Go!", content)
}

func TestMemorySkill_DeleteMemory(t *testing.T) {
	s := setupTestSkill(t)

	_, err := s.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "to-delete",
		"content": "temporary",
	})
	require.NoError(t, err)

	_, err = s.ExecuteTool("delete_memory", map[string]interface{}{
		"name": "to-delete",
	})
	require.NoError(t, err)

	_, err = s.ExecuteTool("read_memory", map[string]interface{}{
		"name": "to-delete",
	})
	assert.Error(t, err)
}

func TestMemorySkill_RegisterViaInit(t *testing.T) {
	// The init() function should have already registered the skill.
	// Verify it's discoverable.
	s, ok := skill.Get("memory")
	require.True(t, ok, "memory skill should be registered via init()")
	assert.Equal(t, "memory", s.Name())

	tp, ok := s.(skill.ToolProvider)
	require.True(t, ok, "memory skill should implement ToolProvider")
	assert.NotNil(t, tp)
}

func TestMemorySkill_ErrorOnMissingParams(t *testing.T) {
	s := setupTestSkill(t)

	_, err := s.ExecuteTool("write_memory", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))

	_, err = s.ExecuteTool("read_memory", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))

	_, err = s.ExecuteTool("search_memories", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))

	_, err = s.ExecuteTool("rename_memory", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))

	_, err = s.ExecuteTool("edit_memory", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))

	_, err = s.ExecuteTool("delete_memory", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))
}

func TestMemorySkill_UnknownTool(t *testing.T) {
	s := setupTestSkill(t)
	_, err := s.ExecuteTool("nonexistent_tool", map[string]interface{}{})
	assert.Error(t, err)
	assert.True(t, errors.Is(err, serr.ErrInvalidArgs))
	assert.Contains(t, err.Error(), "unknown memory tool")
}
