// Package subprocess owns the bench-side subprocess lifecycle: the `helix daemon`
// process per (task, mode) cell and (eventually, D-01) the real `claude` CLI.
//
// D-06 (no TCP ports): the daemon is spawned over a per-cell Unix domain socket
// with HTTP disabled. This package delegates the spawn to the bench sandbox's
// embedded eval StartDaemon, which builds the argv
//
//	helix --serve --socket=<cell>/daemon.sock --http-addr= --json [--profile=…]
//
// — note the empty --http-addr. Because there is no TCP listener, "no port
// collisions on --parallel=N" holds by construction; this package must never
// pass a non-empty --http-addr.
//
// D-01 (wired-not-gating): this package also OWNS the eventual `--agent=claude`
// spawn branch (internal/eval/agent.Agent.Run + WriteMCPConfig). Plan 04 wires
// that branch here. Phase 77's CI gate exercises only the scripted path, so no
// claude process is spawned in this plan — see StartClaude's doc comment below.
package subprocess

import (
	"context"
	"fmt"

	benchsandbox "github.com/agenthands/helix/bench/runtime/sandbox"
	evalsandbox "github.com/agenthands/helix/internal/eval/sandbox"
)

// StartDaemon spawns the per-cell helix daemon over a Unix socket with HTTP
// disabled (D-06) by delegating to the bench sandbox's embedded StartDaemon. It
// returns the *evalsandbox.DaemonHandle so the caller can capture handle.Pid()
// for the PID-gated daemon-tap BEFORE calling handle.Kill() (criterion #4).
//
// profileName is the resolved profile (e.g. "bench-full" from the mode
// resolver); cfgPath is an optional per-cell config path ("" to omit). The
// no-TCP-port invariant is enforced upstream by StartDaemon's fixed
// "--http-addr=" argv — this wrapper adds no transport flags of its own.
//
// Optional evalsandbox.DaemonOptions (e.g. WithWorkingDir for per-cell store
// isolation, D-03) are forwarded additively to the embedded StartDaemon.
func StartDaemon(ctx context.Context, sb *benchsandbox.Sandbox, taskID, mode, profileName, cfgPath string, opts ...evalsandbox.DaemonOption) (*evalsandbox.DaemonHandle, error) {
	if sb == nil {
		return nil, fmt.Errorf("subprocess: nil sandbox")
	}
	h, err := sb.StartDaemon(ctx, taskID, mode, profileName, cfgPath, opts...)
	if err != nil {
		return nil, fmt.Errorf("subprocess: start daemon %s/%s: %w", taskID, mode, err)
	}
	return h, nil
}

// StartClaude (the real `claude` CLI subprocess spawn for D-01, wired-not-gating)
// is IMPLEMENTED in the sibling file claude.go in this package:
//
//	func StartClaude(ctx context.Context, sb *benchsandbox.Sandbox, cfg ClaudeConfig) (*agent.Result, error)
//
// It wires internal/eval/agent.Agent.Run + WriteMCPConfig, pointing the claude
// MCP config at the cell's forwarder socket; bench/runtime.RunCell's
// `case "claude"` branch calls it. The Phase 77/78 CI gate runs only the hermetic
// scripted agent, so no claude process is spawned by default — but the spawn lives
// in claude.go, not here. (This package doc keeps the ownership note; see
// claude.go for the implementation.)
