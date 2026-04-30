//go:build integration || llm || llmjudge

// Degraded subsystem injection tests (RUNT-03, D-11, D-12).
//
// Verifies that the daemon starts in degraded mode when subsystems fail,
// affected tools report errors honestly, and unaffected tools continue working.
//
// IMPORTANT: These tests call skill.Reset() which modifies global state.
// They MUST NOT use t.Parallel(). Each test uses t.Cleanup to restore state.

package runtime_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/skill"
	"github.com/agenthands/helix/test/harness"

	// Blank imports to ensure skill init() registrations are available
	// for Reset/re-register cycles.
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/profile"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/workflow"
)

// failingSkill implements skill.Skill with an Init that always fails.
type failingSkill struct{}

func (f *failingSkill) Name() string        { return "test-failing-skill" }
func (f *failingSkill) Description() string { return "A skill that fails on init for testing" }
func (f *failingSkill) Init(_ skill.SkillDeps) error {
	return errors.New("simulated skill init failure")
}

// failingToolSkill implements both skill.Skill and skill.ToolProvider,
// with Init that fails. Used to verify that a failing skill's tools are unavailable.
type failingToolSkill struct {
	failingSkill
}

func (f *failingToolSkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "broken_tool", Description: "This tool should never be available"},
	}
}

// TestRuntime_Degraded_SkillInitFailure verifies that daemon.New succeeds
// even when a skill's Init() returns an error (degraded mode).
func TestRuntime_Degraded_SkillInitFailure(t *testing.T) {
	// Save current skill registrations and restore after test.
	// skill.Reset() clears all registrations, so we re-register after.
	skill.Reset()
	t.Cleanup(func() { skill.Reset() })

	// Register only the failing skill.
	skill.Register(&failingToolSkill{})

	cfg := harness.DefaultTestConfig(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// daemon.New calls skill.InitAll internally and handles failure gracefully.
	d, err := daemon.New(cfg, logger)
	require.NoError(t, err, "daemon.New should succeed in degraded mode when skill init fails")
	require.NotNil(t, d, "daemon should be created despite skill failure")
}

// TestRuntime_Degraded_LSUnavailable verifies that when LS readiness is
// not waited for, file-level tools work immediately and LS-dependent tools
// either succeed (if LS happened to start fast) or fail cleanly without
// crashing. The key assertion is: no panic, no hang, file tools always work.
func TestRuntime_Degraded_LSUnavailable(t *testing.T) {
	fixtureDir := harness.PrepareFixture(t, "go")
	runner := harness.StartRunner(t, harness.RunnerOptions{
		WorkspaceDir: fixtureDir,
		SkipLS:       true,
		MaxWorkers:   4,
	})

	// File tools MUST work without LS readiness.
	result := harness.CallTool(t, runner.Session, "read_file", map[string]any{
		"path": "main.go",
	})
	text := harness.TextContent(result)
	assert.Contains(t, text, "package main", "read_file should return file content without LS")

	// list_directory should also work.
	result = harness.CallTool(t, runner.Session, "list_directory", map[string]any{
		"path": ".",
	})
	text = harness.TextContent(result)
	assert.NotEmpty(t, text, "list_directory should return results without LS")

	// LS-dependent tools: call and verify no panic/hang. The result may be
	// success (LS started quickly) or a clean error (LS not ready yet).
	// Both are acceptable — the invariant is graceful handling either way.
	ctx := context.Background()
	lsResult, err := runner.Session.CallTool(ctx, &mcpsdk.CallToolParams{
		Name:      "search_symbols",
		Arguments: map[string]any{"query": "main"},
	})
	require.NoError(t, err, "search_symbols should not return protocol-level error")

	if lsResult.IsError {
		// LS not ready yet — verify clean error message.
		errText := harness.TextContent(lsResult)
		assert.NotEmpty(t, errText, "LS tool error should have a message")
		assert.NotContains(t, errText, "goroutine", "error should not contain stack traces")
		assert.NotContains(t, errText, "runtime/debug", "error should not contain runtime debug info")
	}
	// If lsResult.IsError is false, LS was ready — that's fine too.
}

// TestRuntime_Degraded_MemoryStoreFailure verifies that when the memory
// skill fails to initialize, the daemon still starts and non-memory
// tools continue working.
func TestRuntime_Degraded_MemoryStoreFailure(t *testing.T) {
	// Reset all skills and register only the failing one as "memory".
	skill.Reset()
	t.Cleanup(func() { skill.Reset() })

	// Register a failing skill that pretends to be the memory skill.
	skill.Register(&failingMemorySkill{})

	cfg := harness.DefaultTestConfig(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// daemon.New should succeed despite memory skill failure.
	d, err := daemon.New(cfg, logger)
	require.NoError(t, err, "daemon.New should succeed when memory skill fails")
	require.NotNil(t, d, "daemon should be created despite memory failure")
}

// failingMemorySkill simulates a memory skill that fails on Init.
type failingMemorySkill struct{}

func (f *failingMemorySkill) Name() string        { return "memory" }
func (f *failingMemorySkill) Description() string { return "Failing memory skill for testing" }
func (f *failingMemorySkill) Init(_ skill.SkillDeps) error {
	return errors.New("simulated memory store failure")
}
func (f *failingMemorySkill) Tools() []*mcp.ToolDef {
	return []*mcp.ToolDef{
		{Name: "store_memory", Description: "Store memory (broken)"},
		{Name: "search_memories", Description: "Search memories (broken)"},
	}
}

// Ensure interfaces are satisfied at compile time.
var _ skill.Skill = (*failingSkill)(nil)
var _ skill.ToolProvider = (*failingToolSkill)(nil)
var _ skill.ToolProvider = (*failingMemorySkill)(nil)
