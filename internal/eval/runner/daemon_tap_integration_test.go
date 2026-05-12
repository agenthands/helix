//go:build !windows
// +build !windows

package runner

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/eval/trace"
)

// TestDaemonTapIntegration is the F-07 regression guard. It boots a real
// helix daemon via sandbox.StartDaemon, drives one MCP tools/call through the
// helix forwarder over the daemon's Unix socket, then asserts that
// trace.TapDaemonLog captures the call and trace.Merge surfaces it in the
// MergedTrace.ToolCallSummary.
//
// Skipped if `helix` is not on PATH (developer must run `go build ./cmd/helix`
// first; CI compiles via the existing go-test.yml job before running tests).
func TestDaemonTapIntegration(t *testing.T) {
	helixBin, err := exec.LookPath("helix")
	if err != nil {
		t.Skip("helix binary not on PATH; build via 'go build ./cmd/helix' first")
	}

	sb, err := sandbox.NewSandbox("tap-integration", helixBin)
	require.NoError(t, err)
	t.Cleanup(func() { _ = sb.Cleanup() })

	const taskID = "tapcheck"
	const mode = "baseline"
	require.NoError(t, sb.Prepare(taskID, mode))

	cfgPath := filepath.Join(sb.HomeFor(taskID, mode), ".helix", "helix_config.yml")
	require.NoError(t, os.MkdirAll(filepath.Dir(cfgPath), 0700))
	require.NoError(t, os.WriteFile(cfgPath,
		[]byte("profile: baseline\nsemantic_index:\n  enabled: false\nguardrails:\n  enforcement: off\n"),
		0600))

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	h, err := sb.StartDaemon(ctx, taskID, mode, "baseline", cfgPath)
	require.NoError(t, err)
	require.NotNil(t, h)
	require.Greater(t, h.Pid(), 0)

	// Drive a tools/call against the daemon via the helix stdio forwarder.
	// get_health is a side-effect-free tool registered in every profile
	// (including baseline) via internal/kernel/health/skill_adapter.go.
	sockPath := sb.SocketFor(taskID, mode)
	if err := driveSingleToolCall(ctx, helixBin, sockPath, "get_health"); err != nil {
		t.Logf("driveSingleToolCall: %v (continuing to tap anyway)", err)
	}

	// Stop the daemon so its slog buffer flushes to daemon.log.
	// Capture pre-Kill PID — h.Pid() after Wait() may return 0 depending on
	// platform / Go version semantics, which would defeat the tap's PID gate.
	preKillPid := h.Pid()
	require.NoError(t, h.Kill())

	daemonLog := filepath.Join(sb.ModeDir(taskID, mode), "daemon.log")
	require.FileExists(t, daemonLog)

	tap, err := trace.TapDaemonLog(daemonLog, preKillPid)
	require.NoError(t, err)
	require.NotEmpty(t, tap.Events, "tap captured zero daemon events — verify TelemetryMiddleware emits msg=\"tool call\"")

	sawCall := false
	for _, ev := range tap.Events {
		if ev.Tool != "" && ev.Outcome != "" {
			sawCall = true
			break
		}
	}
	assert.True(t, sawCall, "no daemon event had Tool/Outcome populated")

	start := time.Now().Add(-time.Minute)
	merged, err := trace.Merge(trace.MergeInput{
		TaskID:    taskID,
		Mode:      mode,
		RunID:     "tap-integration",
		StartedAt: start,
		EndedAt:   time.Now(),
		Daemon:    tap,
	})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, merged.ToolCallSummary.Total, 1,
		"expected at least one tool call in MergedTrace.ToolCallSummary")
}

// driveSingleToolCall spawns `helix --mode=stdio --socket=<sockPath>` as a
// child process and writes a minimal MCP JSON-RPC handshake + one tools/call
// frame to its stdin. The forwarder proxies the frames to the daemon over the
// Unix socket, triggering the TelemetryMiddleware "tool call" emission on the
// daemon side. Returns nil on a clean response, or an error describing the
// failure (the caller logs and proceeds to the tap regardless, since the tap
// is the regression guard, not the response payload).
func driveSingleToolCall(ctx context.Context, helixBin, sockPath, tool string) error {
	fwdCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(fwdCtx, helixBin, "--mode=stdio", "--socket="+sockPath)
	cmd.Env = append(os.Environ(), "HELIX_LOG_LEVEL=info")

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start forwarder: %w", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	// Write initialize then tools/call as newline-delimited JSON-RPC frames.
	// (The forwarder accepts NDJSON per forwarder_test.go.) Critically: do
	// NOT close stdin between frames or before responses arrive — closing
	// stdin makes the forwarder's stdin->gRPC goroutine reach EOF and
	// CloseSend the gRPC stream, which can race response delivery and cause
	// the daemon's reply to be discarded before we read it.
	initFrame := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"tap-integration","version":"0"}}}`
	callFrame := fmt.Sprintf(`{"jsonrpc":"2.0","id":2,"method":"tools/call","params":{"name":%q,"arguments":{}}}`, tool)

	if _, err := fmt.Fprintln(stdin, initFrame); err != nil {
		return fmt.Errorf("write initialize: %w", err)
	}
	if _, err := fmt.Fprintln(stdin, callFrame); err != nil {
		return fmt.Errorf("write tools/call: %w", err)
	}

	// Read responses on a goroutine so we can apply a deadline without leaking
	// the reader. We're looking for id=2 (the tools/call response).
	type readResult struct {
		got bool
		err error
	}
	resultCh := make(chan readResult, 1)
	go func() {
		reader := bufio.NewReader(stdout)
		for {
			line, err := reader.ReadString('\n')
			if line != "" {
				line = strings.TrimSpace(line)
				if line != "" {
					var resp map[string]interface{}
					if json.Unmarshal([]byte(line), &resp) == nil {
						if id, ok := resp["id"]; ok {
							if v, ok := id.(float64); ok && int(v) == 2 {
								resultCh <- readResult{got: true}
								return
							}
						}
					}
				}
			}
			if err != nil {
				resultCh <- readResult{got: false, err: err}
				return
			}
		}
	}()

	select {
	case r := <-resultCh:
		if r.got {
			// Close stdin AFTER we have the response so subsequent kill is
			// clean; the response was already proxied to the daemon log too.
			_ = stdin.Close()
			return nil
		}
		return fmt.Errorf("read response: %v", r.err)
	case <-time.After(10 * time.Second):
		_ = stdin.Close()
		return fmt.Errorf("did not receive tools/call response within deadline")
	}
}
