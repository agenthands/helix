//go:build windows

package forwarder_test

// Windows local-dial smoke (Phase 94 success criterion 2 / VALIDATION.md
// Manual-Only). It stands up a REAL helix daemon as a subprocess and drives a
// single tools/call through the RETAINED CLI dial path (forwarder.CallTool ->
// ConnectOrStartDaemon -> StreamMCP) over the local same-host endpoint, mirroring
// the intent of the !windows TestCLI_E2E_OneShot.
//
// It is CI-Windows-gated: it does NOT run on the Linux executor host, and the
// 94-02 executor reports it honestly as Manual-Only (never claimed as a local
// pass). It deliberately avoids the internal/eval/sandbox harness because that
// package is Unix-only (syscall.Kill / Setpgid) and does not compile on Windows;
// the daemon is spawned directly here instead.
//
// On Windows the local socket path may be flaky; the gated loopback gRPC TCP
// opt-in from 94-01 (HELIX_GRPC_ADDR / --grpc-addr) is the supported fallback.
// This smoke drives the DEFAULT local path so a regression in the retained
// unix-equivalent dial is caught. detachFromProcessGroup (dial_windows.go) is
// left untouched and is exercised by the production auto-start path.

import (
	"context"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/forwarder"
)

// resolveHelixBin returns the helix binary to drive the smoke, or "" if none is
// available (the caller SKIPs). Mirrors the cli_e2e_test convention: HELIX_BIN
// env override first, then `helix` on PATH.
func resolveHelixBin() string {
	if env := os.Getenv("HELIX_BIN"); env != "" {
		if _, err := os.Stat(env); err == nil {
			return env
		}
	}
	if p, err := exec.LookPath("helix"); err == nil {
		return p
	}
	return ""
}

// TestWindowsLocalDial_OneShot brings up a daemon subprocess and drives one
// tools/call over the retained local dial path on Windows.
func TestWindowsLocalDial_OneShot(t *testing.T) {
	helixBin := resolveHelixBin()
	if helixBin == "" {
		t.Skip("helix binary not resolvable (set HELIX_BIN or 'go build -o helix ./cmd/helix'); skipping Windows local-dial smoke")
	}

	socket := filepath.Join(t.TempDir(), "win-dial.sock")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	// Spawn a real daemon bound to a private socket. CallTool's
	// ConnectOrStartDaemon would also auto-start one, but spawning explicitly
	// keeps the lifecycle (and reaping) under the test's control.
	daemon := exec.CommandContext(ctx, helixBin, "--serve", "--socket="+socket)
	if err := daemon.Start(); err != nil {
		t.Fatalf("starting daemon subprocess: %v", err)
	}
	t.Cleanup(func() {
		_ = daemon.Process.Kill()
		_, _ = daemon.Process.Wait()
	})

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Poll the retained dial path until the daemon is up (or the deadline hits).
	// The empty tcpAddr selects the local-socket default — the path under test.
	deadline := time.Now().Add(20 * time.Second)
	var lastErr error
	for time.Now().Before(deadline) {
		res, err := forwarder.CallTool(ctx, socket, "", logger, "win-dial-smoke",
			"list_memories", map[string]any{})
		if err == nil {
			if res.IsError {
				t.Fatalf("list_memories reported error over Windows local dial")
			}
			return // round-trip succeeded
		}
		lastErr = err
		time.Sleep(200 * time.Millisecond)
	}
	t.Fatalf("Windows local dial did not round-trip before deadline: %v", lastErr)
}
