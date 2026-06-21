package daemon

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/skill"
)

// newTestConfig returns a minimal config suitable for bootstrap tests.
// Uses t.TempDir() for all file paths to avoid touching real system dirs.
func newTestConfig(t *testing.T) *config.SerenaConfig {
	t.Helper()
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "s.sock")
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = socketPath
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	return cfg
}

func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
}

func TestBootstrapRegistersAllTools(t *testing.T) {
	// NOTE: We do NOT call skill.Reset() because skills are registered via init()
	// in imports.go and can only be registered once per process. Resetting would
	// lose them permanently since init() won't re-run.

	cfg := newTestConfig(t)
	logger := newTestLogger()

	d, err := New(cfg, logger)
	require.NoError(t, err, "daemon.New() should succeed")

	registry := d.mcpServer.Registry()
	names := registry.Names()
	count := registry.Count()

	// Expect at least 38 tools: 9 symbol + 6 edit + 6 file + 3 diag + 7 memory
	// + 2 workflow + 2 profile + 3 built-in (ping, echo, activate_project).
	assert.GreaterOrEqual(t, count, 38, "expected at least 38 registered tools, got %d", count)

	// Check specific tools from each category.
	nameSet := make(map[string]bool, len(names))
	for _, n := range names {
		nameSet[n] = true
	}

	// Symbol (9)
	for _, tool := range []string{
		"go_to_definition", "find_references", "get_symbol_overview",
		"search_symbols", "get_hover_info", "find_implementations",
		"get_call_hierarchy", "get_type_hierarchy", "analyze_blast_radius",
	} {
		assert.True(t, nameSet[tool], "missing symbol tool: %s", tool)
	}

	// Edit (6)
	for _, tool := range []string{
		"replace_symbol_body", "insert_before_symbol", "insert_after_symbol",
		"rename_symbol", "safe_delete_symbol", "verify_edit",
	} {
		assert.True(t, nameSet[tool], "missing edit tool: %s", tool)
	}

	// File ops (6)
	for _, tool := range []string{
		"read_file", "create_file", "list_directory",
		"find_files", "search_in_files", "replace_in_file",
	} {
		assert.True(t, nameSet[tool], "missing file tool: %s", tool)
	}

	// Diagnostics (3)
	for _, tool := range []string{
		"get_diagnostics", "get_code_actions", "format_code",
	} {
		assert.True(t, nameSet[tool], "missing diag tool: %s", tool)
	}

	// Memory (7)
	for _, tool := range []string{
		"write_memory", "read_memory", "list_memories", "search_memories",
		"rename_memory", "edit_memory", "delete_memory",
	} {
		assert.True(t, nameSet[tool], "missing memory tool: %s", tool)
	}

	// Workflow (2)
	for _, tool := range []string{
		"onboard_project", "prepare_for_new_conversation",
	} {
		assert.True(t, nameSet[tool], "missing workflow tool: %s", tool)
	}

	// Profile (2)
	for _, tool := range []string{
		"switch_mode", "get_token_budget",
	} {
		assert.True(t, nameSet[tool], "missing profile tool: %s", tool)
	}

	// Built-in (3)
	for _, tool := range []string{
		"ping", "echo", "activate_project",
	} {
		assert.True(t, nameSet[tool], "missing built-in tool: %s", tool)
	}
}

func TestBootstrapSkillsInitialized(t *testing.T) {
	cfg := newTestConfig(t)
	logger := newTestLogger()

	_, err := New(cfg, logger)
	require.NoError(t, err, "daemon.New() should succeed")

	allSkills := skill.All()
	assert.GreaterOrEqual(t, len(allSkills), 7, "expected at least 7 skills, got %d", len(allSkills))

	// Check specific skill names.
	skillNames := make(map[string]bool, len(allSkills))
	for _, s := range allSkills {
		skillNames[s.Name()] = true
	}
	for _, expected := range []string{
		"memory", "workflow", "profile",
		"symbol-retrieval", "symbol-editing", "file-ops", "diagnostics",
	} {
		assert.True(t, skillNames[expected], "missing skill: %s", expected)
	}

	// ToolProviders should also return at least 7.
	providers := skill.ToolProviders()
	assert.GreaterOrEqual(t, len(providers), 7, "expected at least 7 ToolProviders, got %d", len(providers))
}

func TestBootstrapProfileResolved(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Profile = "claude-code"
	logger := newTestLogger()

	d, err := New(cfg, logger)
	require.NoError(t, err, "daemon.New() should succeed with claude-code profile")

	// Active profile should be claude-code.
	assert.Equal(t, "claude-code", d.activeProfile.Name, "active profile should be claude-code")
	assert.Equal(t, "edit", d.activeProfile.DefaultMode, "claude-code default mode should be 'edit'")

	// claude-code excludes read_file, create_text_file, etc.
	excludeSet := make(map[string]bool)
	for _, tool := range d.activeProfile.ExcludeTools {
		excludeSet[tool] = true
	}
	assert.True(t, excludeSet["read_file"], "claude-code should exclude read_file")
}

func TestBootstrapDefaultProfileIsFull(t *testing.T) {
	cfg := newTestConfig(t)
	cfg.Profile = "" // defaults to "full"
	logger := newTestLogger()

	d, err := New(cfg, logger)
	require.NoError(t, err, "daemon.New() should succeed with empty profile (defaults to full)")

	// Active profile should be full.
	assert.Equal(t, "full", d.activeProfile.Name, "active profile should be 'full'")

	// Full profile has no exclude_tools.
	assert.Empty(t, d.activeProfile.ExcludeTools, "full profile should have no excluded tools")

	// All tools should be registered (no filtering at bootstrap).
	count := d.mcpServer.Registry().Count()
	assert.GreaterOrEqual(t, count, 38, "full profile should have all tools registered, got %d", count)
}
