package agent

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/agenthands/helix/internal/eval/sandbox"
)

// ErrClaudeNotFound is returned by Run when the claude CLI binary is not found
// on PATH. Callers should treat this as a configuration error, not a transient
// failure.
var ErrClaudeNotFound = errors.New("claude CLI not found on PATH")

// BudgetParams carries the D-08 budget axes used by the agent runner.
// MaxToolCalls drives --max-turns on the claude CLI invocation.
type BudgetParams struct {
	MaxToolCalls int
}

// Task describes a single eval task to run.
type Task struct {
	ID     string
	Prompt string
	Budget BudgetParams
}

// Result is the outcome of a single claude subprocess invocation.
type Result struct {
	StdoutPath string
	StderrPath string
	ExitCode   int
	Duration   time.Duration
}

// Agent orchestrates claude CLI subprocess invocations against an isolated
// sandbox environment.
type Agent struct {
	sandbox  *sandbox.Sandbox
	helixBin string
}

// NewAgent returns an Agent that will spawn the claude CLI using the given
// sandbox for environment isolation and helixBin as the helix daemon binary
// path embedded in the per-mode MCP config.
func NewAgent(sb *sandbox.Sandbox, helixBin string) *Agent {
	return &Agent{sandbox: sb, helixBin: helixBin}
}

// cleanEnv builds the env slice for the claude subprocess.
// Allowlist (T-67-Pitfall-1 mitigation — strict allowlist prevents dev
// credentials and config paths from leaking into eval runs):
//   - PATH      (inherited from host; needed to find tools)
//   - HOME      (overridden to sandbox.HomeFor so ~/.claude/ is isolated)
//   - ANTHROPIC_API_KEY (needed for API calls; absent if not set)
//   - HELIX_LOG_LEVEL   (propagated if set; controls daemon verbosity)
func (a *Agent) cleanEnv(taskID, mode string) []string {
	env := []string{
		"HOME=" + a.sandbox.HomeFor(taskID, mode),
	}
	for _, key := range []string{"PATH", "ANTHROPIC_API_KEY", "HELIX_LOG_LEVEL"} {
		if v := os.Getenv(key); v != "" {
			env = append(env, key+"="+v)
		}
	}
	return env
}

// buildArgv constructs the claude CLI argument list.
// Order is fixed per plan spec and asserted by TestAgentBuildsArgv.
func (a *Agent) buildArgv(task Task, mode string) []string {
	maxTurns := strconv.Itoa(task.Budget.MaxToolCalls)
	return []string{
		"--print",
		"--bare",
		"--strict-mcp-config",
		"--mcp-config", a.sandbox.McpConfigPath(task.ID, mode),
		"--output-format", "stream-json",
		"--verbose",
		"--include-partial-messages",
		"--max-turns", maxTurns,
		task.Prompt,
	}
}

// Run spawns the claude CLI subprocess for the given task and mode. It:
//  1. Writes the per-mode MCP config JSON (helixBin must be absolute).
//  2. Resolves the claude binary via exec.LookPath; returns ErrClaudeNotFound if absent.
//  3. Starts claude with the scrubbed allowlist env and captures stdout/stderr
//     to <modeDir>/claude.stdout and <modeDir>/claude.stderr (mode 0600).
//  4. Waits for the subprocess; returns Result with paths and exit code.
//
// The subprocess is bound to ctx — cancellation kills it and returns ctx.Err().
func (a *Agent) Run(ctx context.Context, task Task, mode string) (*Result, error) {
	// Write MCP config for this (task, mode) pair.
	cfgPath := a.sandbox.McpConfigPath(task.ID, mode)
	sockPath := a.sandbox.SocketFor(task.ID, mode)
	homePath := a.sandbox.HomeFor(task.ID, mode)
	if err := WriteMCPConfig(cfgPath, a.helixBin, sockPath, homePath); err != nil {
		return nil, fmt.Errorf("agent: write mcp config: %w", err)
	}

	// Resolve claude binary — never use os.Exec("claude") without LookPath
	// so failures surface as a typed error rather than a generic exec failure.
	claudePath, err := exec.LookPath("claude")
	if err != nil {
		return nil, ErrClaudeNotFound
	}

	argv := a.buildArgv(task, mode)
	cmd := exec.CommandContext(ctx, claudePath, argv...)
	cmd.Dir = a.sandbox.RepoFor(task.ID, mode)
	cmd.Env = a.cleanEnv(task.ID, mode)

	modeDir := a.sandbox.ModeDir(task.ID, mode)

	stdoutPath := modeDir + "/claude.stdout"
	stderrPath := modeDir + "/claude.stderr"

	stdoutFile, err := os.OpenFile(stdoutPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("agent: open stdout capture: %w", err)
	}
	defer stdoutFile.Close()

	stderrFile, err := os.OpenFile(stderrPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return nil, fmt.Errorf("agent: open stderr capture: %w", err)
	}
	defer stderrFile.Close()

	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	start := time.Now()
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("agent: start claude: %w", err)
	}

	waitErr := cmd.Wait()
	duration := time.Since(start)

	// If context was cancelled, surface that as the primary error.
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	exitCode := 0
	if waitErr != nil {
		var exitErr *exec.ExitError
		if errors.As(waitErr, &exitErr) {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("agent: wait claude: %w", waitErr)
		}
	}

	return &Result{
		StdoutPath: stdoutPath,
		StderrPath: stderrPath,
		ExitCode:   exitCode,
		Duration:   duration,
	}, nil
}
