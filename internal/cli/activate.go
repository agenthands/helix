package cli

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"go.opentelemetry.io/otel/trace/noop"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/forwarder"
)

func newActivateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "activate",
		Short:         "Activate workspace for current project",
		Long:          "Ensures the Helix daemon is running and activates the workspace for the given directory. Used by Claude Code SessionStart hook.",
		RunE:          runActivate,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().String("workspace", "", "Workspace directory (default: current directory)")
	return cmd
}

func runActivate(cmd *cobra.Command, _ []string) error {
	wsPath, _ := cmd.Flags().GetString("workspace")
	if wsPath == "" {
		var err error
		wsPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Resolve to absolute path (T-36-06)
	absPath, err := filepath.Abs(wsPath)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}

	// Honor --socket / HELIX_SOCKET (resolveVerbSocket falls back to the per-uid
	// default), matching the verb dial path so activate and the verbs target the
	// SAME daemon — required for socket-isolated tests and multi-daemon setups.
	socketPath := resolveVerbSocket(cmd)
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))

	// Use ConnectOrStartDaemon to auto-start daemon if not running (per D-05, D-06)
	// Pass noop tracer -- activate is a CLI command, no OTel needed
	client, conn, err := forwarder.ConnectOrStartDaemon(cmd.Context(), socketPath, resolveVerbGRPCAddr(cmd), logger, noop.NewTracerProvider())
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	// 30-second timeout for activation (covers cold-start with LS installation)
	ctx, cancel := context.WithTimeout(cmd.Context(), 30*time.Second)
	defer cancel()

	resp, err := client.ActivateWorkspace(ctx, &serenav1.ActivateRequest{
		WorkspacePath: absPath,
	})
	if err != nil {
		return fmt.Errorf("activating workspace: %w", err)
	}

	// Also activate via the MCP `activate_project` tool. The gRPC
	// ActivateWorkspace above sets only kernel/LS state; the daemon's file-tool
	// active-workspace state (read by read_file / the edit verbs, set globally via
	// the ActivateCallback and persisted across one-shot CLI sessions) is set ONLY
	// by activate_project. Without this, the CLI-first file/edit verbs return
	// `no_workspace` after a plain `helix activate` (a one-shot `helix <verb>`
	// sends no cwd, so LazyInit — which keys on repo_path — cannot auto-activate),
	// leaving the SessionStart-hook flow unable to use those verbs. Best-effort:
	// a failure warns but does not fail the hook (fail-open, like the priming text).
	if _, aerr := callToolFn(ctx, socketPath, resolveVerbGRPCAddr(cmd), logger, CurrentVersion(), "activate_project", map[string]any{"repo_path": absPath}); aerr != nil {
		fmt.Fprintf(os.Stderr, "warning: activate_project failed (%v); file/edit verbs may report no_workspace\n", aerr)
	}

	// Output for Claude Code hook stdout (added to agent context)
	fmt.Fprintf(os.Stdout, "Helix workspace activated: %s (status: %s)\n", absPath, resp.Status)

	// STEER-02: emit the terse "use X not Y" priming matrix once per session,
	// best-effort. sessionPrimingText() is a pure compiled constant (size-capped,
	// no I/O), so this never fails the session; a write error is ignored on purpose
	// (fail-open — the activation return value is unchanged).
	fmt.Fprintln(os.Stdout, sessionPrimingText())
	return nil
}
