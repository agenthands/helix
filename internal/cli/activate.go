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
	"github.com/agenthands/helix/internal/config"
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

	socketPath := config.DefaultSocketPath()
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

	// Output for Claude Code hook stdout (added to agent context)
	fmt.Fprintf(os.Stdout, "Helix workspace activated: %s (status: %s)\n", absPath, resp.Status)
	return nil
}
