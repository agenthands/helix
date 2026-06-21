//go:build !windows
// +build !windows

package runner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/agenthands/helix/internal/eval/sandbox"
	"github.com/agenthands/helix/internal/eval/trace"
	"github.com/agenthands/helix/internal/forwarder"
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

	// Drive a tools/call against the daemon over the retained gRPC StreamMCP wire
	// (Phase 94 fix(94-02)). We call activate_project: it is an always-allowed
	// control-plane tool (internal/mcp/profile_enforce.go:alwaysAllowedCoreTools),
	// so it is NOT refused by ProfileEnforcementMiddleware under the baseline
	// profile's empty tool whitelist (baseline.yaml: `tools: []`) — the refusal
	// short-circuits BEFORE TelemetryMiddleware, so a denied call would emit zero
	// "tool call" lines and defeat the tap. activate_project reaches the handler
	// and TelemetryMiddleware emits the msg="tool call" line the tap asserts.
	// (The deleted stdio forwarder head historically drove get_health here.)
	sockPath := sb.SocketFor(taskID, mode)
	repoDir := sb.RepoFor(taskID, mode)
	if err := driveSingleToolCall(ctx, helixBin, sockPath, "activate_project",
		map[string]any{"repo_path": repoDir}); err != nil {
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

// driveSingleToolCall issues one MCP tools/call against the daemon over the
// RETAINED gRPC StreamMCP wire via forwarder.CallTool (Phase 94 fix(94-02)).
//
// History: this previously spawned `helix --mode=stdio --socket=<sockPath>` and
// piped a hand-framed initialize + tools/call over the forwarder's stdin/stdout.
// Phase 94 deleted that stdio forwarder head, so the call now dials the daemon's
// unix socket directly in-process. forwarder.CallTool wraps the gRPC StreamMCP
// stream with the MCP SDK client (which performs the initialize handshake the
// daemon requires) and issues the single tools/call, which is exactly what
// triggers the daemon-side TelemetryMiddleware "tool call" emission the tap
// asserts. The helixBin parameter is no longer needed to spawn a forwarder; it
// is retained for signature stability (the caller passes the resolved path).
//
// The old "do not close stdin until the response arrives" race is moot: there is
// no stdin pipe under gRPC. forwarder.CallTool performs the ordered teardown
// (SDK shutdown flush → stream CloseSend → conn close) internally so the daemon
// records the session with outcome="ended", not an RST abort.
//
// Returns nil on a clean response (including a tool-level isError result, since
// the tap — not the payload — is the regression guard), or an error describing a
// transport-level failure. The caller logs and proceeds to the tap regardless.
func driveSingleToolCall(ctx context.Context, helixBin, sockPath, tool string, args map[string]any) error {
	_ = helixBin // retained for signature stability; the gRPC dial needs no binary path.

	callCtx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Empty tcpAddr selects the unix-socket default (with auto-start). The daemon
	// is already up (sandbox.StartDaemon), so this takes the warm-reuse fast path.
	if _, err := forwarder.CallTool(callCtx, sockPath, "", logger, "tap-integration", tool, args); err != nil {
		return fmt.Errorf("call %q over gRPC: %w", tool, err)
	}
	return nil
}
