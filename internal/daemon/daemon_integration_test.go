package daemon

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/config"
	helixMCP "github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/profile"
	"github.com/agenthands/helix/internal/skill"
)

// newE2EConfig creates a config with temp dirs and a short socket path.
func newE2EConfig(t *testing.T) *config.SerenaConfig {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = filepath.Join(tmpDir, "s.sock")
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	return cfg
}

func TestE2EMemoryToolInvocation(t *testing.T) {
	tmpDir := t.TempDir()
	cfg := newE2EConfig(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Re-initialize skills with our temp project dir so memory writes go to tmpDir.
	deps := skill.SkillDeps{
		ProjectDir: filepath.Join(tmpDir, ".helix"),
		GlobalDir:  filepath.Join(tmpDir, ".helix-global"),
		Logger:     logger,
	}
	require.NoError(t, os.MkdirAll(deps.ProjectDir, 0o755))
	require.NoError(t, os.MkdirAll(deps.GlobalDir, 0o755))
	require.NoError(t, skill.InitAll(deps))

	_, err := New(cfg, logger)
	require.NoError(t, err)

	// Get the memory skill and assert it implements ExecuteTool.
	memSkill, ok := skill.Get("memory")
	require.True(t, ok, "memory skill should be registered")
	executor, ok := memSkill.(SkillToolExecutor)
	require.True(t, ok, "memory skill should implement SkillToolExecutor")

	// Write a memory.
	result, err := executor.ExecuteTool("write_memory", map[string]interface{}{
		"name":    "test-memory",
		"content": "hello from integration test",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "written successfully")

	// Read it back.
	result, err = executor.ExecuteTool("read_memory", map[string]interface{}{
		"name": "test-memory",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "hello from integration test")

	// List memories.
	result, err = executor.ExecuteTool("list_memories", map[string]interface{}{})
	require.NoError(t, err)
	assert.NotEmpty(t, result, "list_memories should return non-empty result")
	assert.Contains(t, result, "test-memory")

	// Delete the memory.
	result, err = executor.ExecuteTool("delete_memory", map[string]interface{}{
		"name": "test-memory",
	})
	require.NoError(t, err)
	assert.Contains(t, result, "deleted")
}

func TestE2EModeSwitching(t *testing.T) {
	cfg := newE2EConfig(t)
	cfg.Profile = "full"
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	_, err := New(cfg, logger)
	require.NoError(t, err)

	ps := profile.GetProfileSkill()
	require.NotNil(t, ps, "profile skill should be available")

	// Wire a session provider with initial mode="edit" and profile="full".
	session := &helixMCP.SessionInfo{
		Profile: "full",
		Mode:    "edit",
	}
	ps.SetSessionProvider(&testSessionProvider{session: session})

	// Switch from edit -> read (allowed by full profile).
	result, err := ps.ExecuteSwitchMode("read")
	require.NoError(t, err)
	assert.Equal(t, "edit", result.PreviousMode)
	assert.Equal(t, "read", result.CurrentMode)
	assert.Greater(t, result.ToolsAvailable, 0)

	// Switch from read -> review (allowed by full profile).
	result, err = ps.ExecuteSwitchMode("review")
	require.NoError(t, err)
	assert.Equal(t, "read", result.PreviousMode)
	assert.Equal(t, "review", result.CurrentMode)

	// Verify mode history has 2 transitions.
	assert.Len(t, session.ModeHistory, 2, "should have 2 mode transitions")
	assert.Equal(t, "edit", session.ModeHistory[0].From)
	assert.Equal(t, "read", session.ModeHistory[0].To)
	assert.Equal(t, "read", session.ModeHistory[1].From)
	assert.Equal(t, "review", session.ModeHistory[1].To)
}

func TestE2ETokenBudget(t *testing.T) {
	cfg := newE2EConfig(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	_, err := New(cfg, logger)
	require.NoError(t, err)

	ps := profile.GetProfileSkill()
	require.NotNil(t, ps, "profile skill should be available")

	result, err := ps.ExecuteGetTokenBudget("full", "edit", "detailed")
	require.NoError(t, err)
	assert.Greater(t, result.TotalTokens, 0, "TotalTokens should be > 0")
	assert.Greater(t, result.ToolCount, 0, "ToolCount should be > 0")
	assert.NotEmpty(t, result.PerTool, "PerTool should be non-empty for detailed format")
}

func TestE2ECleanShutdown(t *testing.T) {
	cfg := newE2EConfig(t)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	d, err := New(cfg, logger)
	require.NoError(t, err)

	// Capture goroutine count before starting.
	runtime.GC()
	goroutinesBefore := runtime.NumGoroutine()

	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	go func() {
		errCh <- d.Run(ctx)
	}()

	// Wait for the daemon to start (socket file appears).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(cfg.Daemon.SocketPath); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	// Cancel context to trigger shutdown.
	cancel()
	runErr := <-errCh

	// context.Canceled is the expected shutdown error.
	assert.ErrorIs(t, runErr, context.Canceled, "Run() should return context.Canceled")

	// Give goroutines a moment to wind down.
	time.Sleep(100 * time.Millisecond)
	runtime.GC()
	goroutinesAfter := runtime.NumGoroutine()

	// Allow a tolerance of 5 goroutines (runtime overhead, test framework, etc.).
	tolerance := 5
	assert.LessOrEqual(t, goroutinesAfter, goroutinesBefore+tolerance,
		"goroutine leak detected: before=%d, after=%d", goroutinesBefore, goroutinesAfter)
}

func TestE2EProfileConfigLayering(t *testing.T) {
	tmpDir := t.TempDir()
	globalDir := filepath.Join(tmpDir, ".helix")
	require.NoError(t, os.MkdirAll(globalDir, 0o755))

	cfg := &config.SerenaConfig{
		Profile: "ci-bot",
	}

	store, activeProfile, err := config.ResolveProfile(cfg, globalDir)
	require.NoError(t, err)
	require.NotNil(t, store)
	require.NotNil(t, activeProfile)

	assert.Equal(t, "ci-bot", activeProfile.Name, "resolved profile should be ci-bot")
	assert.Equal(t, "review", activeProfile.DefaultMode, "ci-bot default mode should be review")

	// ci-bot excludes editing tools.
	excludeSet := make(map[string]bool)
	for _, tool := range activeProfile.ExcludeTools {
		excludeSet[tool] = true
	}
	assert.True(t, excludeSet["replace_symbol_body"], "ci-bot should exclude replace_symbol_body")
	assert.True(t, excludeSet["insert_before_symbol"], "ci-bot should exclude insert_before_symbol")

	// ResolveTools with ci-bot's skills should produce a list without editing tools.
	resolved := skill.ResolveTools(activeProfile.Skills, activeProfile.Tools, activeProfile.ExcludeTools)
	resolvedNames := make(map[string]bool)
	for _, t := range resolved {
		resolvedNames[t.Name] = true
	}
	assert.False(t, resolvedNames["replace_symbol_body"],
		"ci-bot resolved tools should not include replace_symbol_body")
	assert.False(t, resolvedNames["insert_after_symbol"],
		"ci-bot resolved tools should not include insert_after_symbol")
}

func TestE2EWorkflowOnboarding(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a project structure for onboarding to scan.
	projectRoot := filepath.Join(tmpDir, "myproject")
	require.NoError(t, os.MkdirAll(filepath.Join(projectRoot, "src"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "main.go"), []byte("package main"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(projectRoot, "src", "app.go"), []byte("package src"), 0o644))

	// Re-initialize skills with project dir pointing inside our temp project.
	deps := skill.SkillDeps{
		ProjectDir: filepath.Join(projectRoot, ".helix"),
		GlobalDir:  filepath.Join(tmpDir, ".helix-global"),
		Logger:     slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})),
	}
	require.NoError(t, os.MkdirAll(deps.ProjectDir, 0o755))
	require.NoError(t, os.MkdirAll(deps.GlobalDir, 0o755))
	require.NoError(t, skill.InitAll(deps))

	wfSkill, ok := skill.Get("workflow")
	require.True(t, ok, "workflow skill should be registered")
	executor, ok := wfSkill.(SkillToolExecutor)
	require.True(t, ok, "workflow skill should implement SkillToolExecutor")

	result, err := executor.ExecuteTool("onboard_project", map[string]interface{}{})
	require.NoError(t, err)
	assert.NotEmpty(t, result, "onboarding result should not be empty")
	// Onboarding output should mention the project root or detected languages.
	assert.True(t,
		strings.Contains(result, "Project Root") || strings.Contains(result, "Go") || strings.Contains(result, projectRoot),
		"onboarding should mention project root or detected language, got: %s", result,
	)
}

// testSessionProvider is a minimal SessionProvider for testing mode switching.
type testSessionProvider struct {
	session *helixMCP.SessionInfo
}

func (p *testSessionProvider) CurrentSession() *helixMCP.SessionInfo {
	return p.session
}
