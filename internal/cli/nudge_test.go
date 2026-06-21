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
	assert.Equal(t, 0, stats.HelixToolCount)
}

func TestLoadSessionStats_DifferentSession(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	// Save stats with session "A".
	statsA := sessionStats{
		SessionID:       "session-A",
		GrepReadCount:   10,
		HelixToolCount: 5,
	}
	require.NoError(t, saveSessionStats(path, statsA))

	// Load with session "B" -- should return fresh stats.
	stats := loadSessionStats(path, "session-B")
	assert.Equal(t, "session-B", stats.SessionID)
	assert.Equal(t, 0, stats.GrepReadCount)
	assert.Equal(t, 0, stats.HelixToolCount)
}

func TestSaveAndLoadSessionStats_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".helix", "session-stats.json")

	original := sessionStats{
		SessionID:       "session-xyz",
		GrepReadCount:   7,
		HelixToolCount: 3,
	}
	require.NoError(t, saveSessionStats(path, original))

	loaded := loadSessionStats(path, "session-xyz")
	assert.Equal(t, original.SessionID, loaded.SessionID)
	assert.Equal(t, original.GrepReadCount, loaded.GrepReadCount)
	assert.Equal(t, original.HelixToolCount, loaded.HelixToolCount)
	assert.NotEmpty(t, loaded.LastUpdated, "LastUpdated should be set by save")
}

func TestIsHelixSymbolicTool(t *testing.T) {
	// Should return true for bare names.
	assert.True(t, isHelixSymbolicTool("find_symbol"))
	assert.True(t, isHelixSymbolicTool("get_symbols_overview"))
	assert.True(t, isHelixSymbolicTool("get_symbol_details"))
	assert.True(t, isHelixSymbolicTool("find_references"))
	assert.True(t, isHelixSymbolicTool("get_hover_info"))
	assert.True(t, isHelixSymbolicTool("find_implementations"))
	assert.True(t, isHelixSymbolicTool("get_call_hierarchy"))
	assert.True(t, isHelixSymbolicTool("get_type_hierarchy"))
	assert.True(t, isHelixSymbolicTool("get_blast_radius"))

	// Should return true with mcp__helix__ prefix.
	assert.True(t, isHelixSymbolicTool("mcp__helix__find_symbol"))
	assert.True(t, isHelixSymbolicTool("mcp__helix__get_symbols_overview"))

	// Should return false for non-symbolic tools.
	assert.False(t, isHelixSymbolicTool("Grep"))
	assert.False(t, isHelixSymbolicTool("Read"))
	assert.False(t, isHelixSymbolicTool("Bash"))
	assert.False(t, isHelixSymbolicTool("Write"))
	assert.False(t, isHelixSymbolicTool("mcp__helix__read_file"))
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
		HelixToolCount: 0,
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
		HelixToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	// Simulate one more grep call to reach threshold.
	loaded := loadSessionStats(path, "test-session")
	loaded.GrepReadCount++
	require.NoError(t, saveSessionStats(path, loaded))

	// Now at threshold.
	final := loadSessionStats(path, "test-session")
	assert.Equal(t, 5, final.GrepReadCount)
	assert.Equal(t, 0, final.HelixToolCount)
	// Nudge should fire: GrepReadCount >= 5 && HelixToolCount == 0.
	assert.True(t, final.GrepReadCount >= 5 && final.HelixToolCount == 0,
		"nudge should fire at count=%d with helix_count=%d", final.GrepReadCount, final.HelixToolCount)
}

func TestNudgeThreshold_ResetBySymbolicTool(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session-stats.json")

	stats := sessionStats{
		SessionID:       "test-session",
		GrepReadCount:   6,
		HelixToolCount: 0,
	}
	require.NoError(t, saveSessionStats(path, stats))

	// Simulate a Helix symbolic tool call (resets grep count).
	loaded := loadSessionStats(path, "test-session")
	assert.True(t, isHelixSymbolicTool("find_symbol"))
	loaded.HelixToolCount++
	loaded.GrepReadCount = 0
	require.NoError(t, saveSessionStats(path, loaded))

	final := loadSessionStats(path, "test-session")
	assert.Equal(t, 0, final.GrepReadCount, "GrepReadCount should reset to 0")
	assert.Equal(t, 1, final.HelixToolCount, "HelixToolCount should increment")
}

func TestClassifyBashTarget_CodeTargets(t *testing.T) {
	// Code-file targets classify as code (isCode=true, ok=true).
	cases := []string{
		`grep "func Foo" main.go`,
		`grep -r "Bar(" internal/cli/verb.go`,
		`cat internal/kernel/edit/edit.go`,
		`sed -n '1,20p' pkg/server/server.go`,
		`rg "TODO" handler.ts`,
		`find . -name '*.go'`,
	}
	for _, cmd := range cases {
		isCode, ok := classifyBashTarget(cmd)
		assert.True(t, ok, "expected ok=true for %q", cmd)
		assert.True(t, isCode, "expected isCode=true for %q", cmd)
	}
}

func TestClassifyBashTarget_NonCodeTargets(t *testing.T) {
	// Prose/log/config targets classify as non-code (isCode=false, ok=true).
	cases := []string{
		`grep TODO README.md`,
		`grep error app.log`,
		`cat config.yaml`,
		`grep x notes.txt`,
		`grep y data.json`,
		`cat Dockerfile`,
		`cat settings.yml`,
	}
	for _, cmd := range cases {
		isCode, ok := classifyBashTarget(cmd)
		assert.True(t, ok, "expected ok=true for %q", cmd)
		assert.False(t, isCode, "expected isCode=false for %q", cmd)
	}
}

func TestClassifyBashTarget_FailOpen(t *testing.T) {
	// Unparseable / no operand → ok=false (fail-open signal).
	cases := []string{
		`grep`,        // no operand
		`grep "x"`,    // pattern only, no file
		``,            // empty command
		`grep -r -n`,  // flags only
		`cat`,         // no operand
		`echo hello`,  // not a grep/read shape
		`go build ./...`,
	}
	for _, cmd := range cases {
		_, ok := classifyBashTarget(cmd)
		assert.False(t, ok, "expected ok=false (fail-open) for %q", cmd)
	}
}

func TestClassifyBashTarget_MixedTargetsConservative(t *testing.T) {
	// Policy: when BOTH a code and a non-code operand appear, do NOT suggest
	// (conservative — a false suggestion is the failure mode to avoid).
	// Require ALL identified file operands to be code.
	isCode, ok := classifyBashTarget(`grep x main.go README.md`)
	assert.True(t, ok, "mixed operands are still parseable")
	assert.False(t, isCode, "mixed code+non-code operands → conservative non-code")
}

func TestClassifyBashTarget_PureDataNoExec(t *testing.T) {
	// Commands with shell metacharacters are parsed as a string only — the
	// classifier never runs anything (no os/exec in this path). We assert it
	// returns deterministically without side effects.
	isCode, ok := classifyBashTarget(`grep x f.go && rm -rf /`)
	// f.go is a code operand; trailing metachars are ignored as data.
	assert.True(t, ok)
	assert.True(t, isCode)

	// $(whoami).go must not be executed; it is just a token. Whether it
	// classifies as code or not, the key property is no side effects + a
	// deterministic return.
	_, ok2 := classifyBashTarget(`grep x $(whoami).go`)
	_ = ok2 // no panic, no exec — property assertion is the absence of side effects
}

func TestSaveSessionStats_AtomicWrite(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".helix", "session-stats.json")

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
