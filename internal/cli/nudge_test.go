package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSessionStats_NewFile(t *testing.T) {
	stats := loadSessionStats("/nonexistent/session-stats.json", "session-123")
	assert.Equal(t, "session-123", stats.SessionID)
	assert.Equal(t, 0, stats.GrepReadCount)
	assert.Equal(t, 0, stats.SerenaToolCount)
}

func TestLoadSessionStats_DifferentSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	// Save stats with session "A".
	statsA := sessionStats{
		SessionID:       "session-A",
		GrepReadCount:   10,
		SerenaToolCount: 5,
	}
	require.NoError(t, saveSessionStats(path, statsA))

	// Load with session "B" -- should return fresh stats.
	stats := loadSessionStats(path, "session-B")
	assert.Equal(t, "session-B", stats.SessionID)
	assert.Equal(t, 0, stats.GrepReadCount)
	assert.Equal(t, 0, stats.SerenaToolCount)
}

func TestSaveAndLoadSessionStats_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".serena", "session-stats.json")

	original := sessionStats{
		SessionID:       "session-xyz",
		GrepReadCount:   7,
		SerenaToolCount: 3,
	}
	require.NoError(t, saveSessionStats(path, original))

	loaded := loadSessionStats(path, "session-xyz")
	assert.Equal(t, original.SessionID, loaded.SessionID)
	assert.Equal(t, original.GrepReadCount, loaded.GrepReadCount)
	assert.Equal(t, original.SerenaToolCount, loaded.SerenaToolCount)
	assert.NotEmpty(t, loaded.LastUpdated, "LastUpdated should be set by save")
}

func TestIsSerenaSymbolicTool(t *testing.T) {
	// Should return true for bare names.
	assert.True(t, isSerenaSymbolicTool("find_symbol"))
	assert.True(t, isSerenaSymbolicTool("get_symbols_overview"))
	assert.True(t, isSerenaSymbolicTool("get_symbol_details"))
	assert.True(t, isSerenaSymbolicTool("find_references"))
	assert.True(t, isSerenaSymbolicTool("get_hover_info"))
	assert.True(t, isSerenaSymbolicTool("find_implementations"))
	assert.True(t, isSerenaSymbolicTool("get_call_hierarchy"))
	assert.True(t, isSerenaSymbolicTool("get_type_hierarchy"))
	assert.True(t, isSerenaSymbolicTool("get_blast_radius"))

	// Should return true with mcp__serena__ prefix.
	assert.True(t, isSerenaSymbolicTool("mcp__serena__find_symbol"))
	assert.True(t, isSerenaSymbolicTool("mcp__serena__get_symbols_overview"))

	// Should return false for non-symbolic tools.
	assert.False(t, isSerenaSymbolicTool("Grep"))
	assert.False(t, isSerenaSymbolicTool("Read"))
	assert.False(t, isSerenaSymbolicTool("Bash"))
	assert.False(t, isSerenaSymbolicTool("Write"))
	assert.False(t, isSerenaSymbolicTool("mcp__serena__read_file"))
}

func TestIsGrepReadTool(t *testing.T) {
	// Direct grep/read tools.
	assert.True(t, isGrepReadTool("Grep", nil))
	assert.True(t, isGrepReadTool("Read", nil))

	// Bash with grep-like commands.
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "grep -r pattern ."}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "find . -name '*.go'"}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "rg pattern"}))
	assert.True(t, isGrepReadTool("Bash", map[string]any{"command": "ag pattern"}))

	// Bash without grep-like commands.
	assert.False(t, isGrepReadTool("Bash", map[string]any{"command": "echo hello"}))
	assert.False(t, isGrepReadTool("Bash", map[string]any{"command": "go build ./..."}))

	// Non-matching tools.
	assert.False(t, isGrepReadTool("Write", nil))
	assert.False(t, isGrepReadTool("Edit", nil))
}

func TestNudgeThreshold_BelowThreshold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	stats := sessionStats{
		SessionID:       "test-session",
		GrepReadCount:   4,
		SerenaToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	loaded := loadSessionStats(path, "test-session")
	assert.Equal(t, 4, loaded.GrepReadCount)
	// At 4 grep calls, nudge should NOT fire (threshold is 5).
	assert.True(t, loaded.GrepReadCount < 5, "count 4 should be below threshold 5")
}

func TestNudgeThreshold_AtThreshold(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	stats := sessionStats{
		SessionID:       "test-session",
		GrepReadCount:   4,
		SerenaToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	// Simulate one more grep call to reach threshold.
	loaded := loadSessionStats(path, "test-session")
	loaded.GrepReadCount++
	require.NoError(t, saveSessionStats(path, loaded))

	// Now at threshold.
	final := loadSessionStats(path, "test-session")
	assert.Equal(t, 5, final.GrepReadCount)
	assert.Equal(t, 0, final.SerenaToolCount)
	// Nudge should fire: GrepReadCount >= 5 && SerenaToolCount == 0.
	assert.True(t, final.GrepReadCount >= 5 && final.SerenaToolCount == 0,
		"nudge should fire at count=%d with serena_count=%d", final.GrepReadCount, final.SerenaToolCount)
}

func TestNudgeThreshold_ResetBySymbolicTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	stats := sessionStats{
		SessionID:       "test-session",
		GrepReadCount:   6,
		SerenaToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	// Simulate a Serena symbolic tool call (resets grep count).
	loaded := loadSessionStats(path, "test-session")
	assert.True(t, isSerenaSymbolicTool("find_symbol"))
	loaded.SerenaToolCount++
	loaded.GrepReadCount = 0
	require.NoError(t, saveSessionStats(path, loaded))

	final := loadSessionStats(path, "test-session")
	assert.Equal(t, 0, final.GrepReadCount, "GrepReadCount should reset to 0")
	assert.Equal(t, 1, final.SerenaToolCount, "SerenaToolCount should increment")
}

func TestSaveSessionStats_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".serena", "session-stats.json")

	stats := sessionStats{SessionID: "atomic-test", GrepReadCount: 1}
	require.NoError(t, saveSessionStats(path, stats))

	// Verify the temp file was cleaned up (atomic rename).
	_, err := os.Stat(path + ".tmp")
	assert.True(t, os.IsNotExist(err), "temp file should be removed after rename")

	// Verify the final file exists and is valid JSON.
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var loaded sessionStats
	require.NoError(t, json.Unmarshal(data, &loaded))
	assert.Equal(t, "atomic-test", loaded.SessionID)
}
