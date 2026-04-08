//go:build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMemory_CRUD(t *testing.T) {
	td := StartTestDaemon(t, Options{SkipLS: true})

	// 1. write_memory
	result := callTool(t, td.Session, "write_memory", map[string]any{
		"name":    "test-note",
		"content": "integration test content",
	})
	assert.Contains(t, textContent(result), "written successfully")

	// 2. read_memory
	result = callTool(t, td.Session, "read_memory", map[string]any{
		"name": "test-note",
	})
	assert.Contains(t, textContent(result), "integration test content")

	// 3. list_memories
	result = callTool(t, td.Session, "list_memories", map[string]any{})
	assert.Contains(t, textContent(result), "test-note")

	// 4. search_memories
	result = callTool(t, td.Session, "search_memories", map[string]any{
		"query": "integration",
	})
	assert.Contains(t, textContent(result), "test-note")

	// 5. edit_memory -- uses search/replace
	result = callTool(t, td.Session, "edit_memory", map[string]any{
		"name":    "test-note",
		"search":  "integration test content",
		"replace": "updated content",
	})
	assert.Contains(t, textContent(result), "edited successfully")

	// Verify edit
	result = callTool(t, td.Session, "read_memory", map[string]any{
		"name": "test-note",
	})
	assert.Contains(t, textContent(result), "updated content")

	// 6. rename_memory -- uses old_name/new_name
	result = callTool(t, td.Session, "rename_memory", map[string]any{
		"old_name": "test-note",
		"new_name": "renamed-note",
	})
	assert.Contains(t, textContent(result), "renamed")

	// Verify rename
	result = callTool(t, td.Session, "read_memory", map[string]any{
		"name": "renamed-note",
	})
	assert.Contains(t, textContent(result), "updated content")

	// 7. delete_memory
	result = callTool(t, td.Session, "delete_memory", map[string]any{
		"name": "renamed-note",
	})
	assert.Contains(t, textContent(result), "deleted")

	// Verify delete -- list should not contain renamed-note
	result = callTool(t, td.Session, "list_memories", map[string]any{})
	require.NotContains(t, textContent(result), "renamed-note")
}
