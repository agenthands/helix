// subprocess/claude.go wires the real `claude` CLI agent branch (D-01:
// wired-not-gating). It is reachable ONLY via `helix-bench run --agent=claude`
// and is NEVER on the scripted code path that the hermetic CI gate exercises —
// the Phase 77 smoke runs the in-process scripted agent (drive.go), so no claude
// process is spawned in CI.
//
// This branch is deliberately a thin delegation to internal/eval/agent.Agent:
// the argv (--bare --strict-mcp-config --output-format=stream-json …), the
// strict env allowlist (cleanEnv), and the per-mode MCP config writer
// (WriteMCPConfig) all live there and MUST NOT be re-rolled here (T-77-11:
// reuse the credential allowlist so dev creds do not leak into the subprocess).
//
// When the `claude` binary is absent, agent.Run returns agent.ErrClaudeNotFound;
// StartClaude surfaces it verbatim so callers can present a clean "claude CLI not
// found" message instead of a panic or an opaque exec error.
package subprocess

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	"github.com/agenthands/helix/internal/eval/agent"
)

// ErrClaudeNotFound is re-exported from internal/eval/agent so bench callers can
// detect the "claude CLI absent" condition without importing the eval agent
// package directly. It is the SAME sentinel (errors.Is matches across the alias).
var ErrClaudeNotFound = agent.ErrClaudeNotFound

// ClaudeConfig carries the per-cell inputs for one real-claude run. The daemon
// must already be running over the cell's Unix socket (StartDaemon) — the claude
// MCP config points the helix forwarder at that same socket, so claude reaches the
// per-cell daemon exactly as the scripted forwarder-drive does (D-06).
type ClaudeConfig struct {
	// HelixBin is the helix binary path embedded in the MCP config as the stdio
	// forwarder command. It MUST be absolute (WriteMCPConfig rejects a relative
	// path, T-67-03) — the caller resolves it before dispatch.
	HelixBin string
	// TaskID and Mode key the per-cell sandbox paths (socket/home/mcp config).
	TaskID string
	Mode   string
	// Prompt is the task instruction handed to claude (e.g. task.json "prompt").
	Prompt string
	// MaxToolCalls bounds the run via claude's --max-turns (budget axis).
	MaxToolCalls int
}

// StartClaude runs the real `claude` CLI agent against the per-cell daemon socket
// (D-01, wired-not-gating). It delegates to internal/eval/agent.Agent.Run, which
// writes the per-mode MCP config (helix forwarder -> cell socket), spawns claude
// with the strict env allowlist (T-77-11), and captures stdout/stderr under the
// cell mode dir.
//
// It returns agent.ErrClaudeNotFound (via the re-exported sentinel) when the
// claude binary is not on PATH — a clean, typed configuration error, never a
// panic. This function is NOT reached by the scripted smoke; it is the opt-in
// `--agent=claude` branch only.
func StartClaude(ctx context.Context, sb *benchsandbox.Sandbox, cfg ClaudeConfig) (*agent.Result, error) {
	if sb == nil {
		return nil, fmt.Errorf("subprocess: nil sandbox")
	}
	if !filepath.IsAbs(cfg.HelixBin) {
		// WriteMCPConfig hard-rejects a relative helixBin (T-67-03); fail fast here
		// with a clearer message rather than deep inside agent.Run.
		return nil, fmt.Errorf("subprocess: claude branch requires an absolute --helix-bin path, got %q", cfg.HelixBin)
	}

	a := agent.NewAgent(sb.Sandbox, cfg.HelixBin)
	res, err := a.Run(ctx, agent.Task{
		ID:     cfg.TaskID,
		Prompt: cfg.Prompt,
		Budget: agent.BudgetParams{MaxToolCalls: cfg.MaxToolCalls},
	}, cfg.Mode)
	if err != nil {
		// Surface ErrClaudeNotFound verbatim so callers can errors.Is-match it and
		// present a clean message; wrap any other error with context.
		if errors.Is(err, agent.ErrClaudeNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("subprocess: run claude %s/%s: %w", cfg.TaskID, cfg.Mode, err)
	}
	return res, nil
}
