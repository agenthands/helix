package cli

import (
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	serenav1 "github.com/postfix/serena/api/proto/serena/v1"
	"github.com/postfix/serena/internal/config"
)

func newDeactivateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "deactivate",
		Short: "Deactivate workspace for current project",
		Long:  "Cleans up session-scoped state for the given directory. Used by Claude Code Stop hook.",
		RunE:          runDeactivate,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.Flags().String("workspace", "", "Workspace directory (default: current directory)")
	return cmd
}

func runDeactivate(cmd *cobra.Command, _ []string) error {
	wsPath, _ := cmd.Flags().GetString("workspace")
	if wsPath == "" {
		var err error
		wsPath, err = os.Getwd()
		if err != nil {
			return fmt.Errorf("getting working directory: %w", err)
		}
	}

	// Resolve to absolute path (T-36-07)
	absPath, err := filepath.Abs(wsPath)
	if err != nil {
		return fmt.Errorf("resolving workspace path: %w", err)
	}

	// Clean up local session files first (always, regardless of daemon state)
	statsPath := filepath.Join(absPath, ".serena", "session-stats.json")
	os.Remove(statsPath) // Ignore error -- file may not exist

	// Try to notify daemon, but silently succeed if daemon is not running (D-15)
	socketPath := config.DefaultSocketPath()
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		// Daemon not running -- silent success
		return nil
	}

	// Verify socket is alive with a short dial timeout
	testConn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		// Daemon not responding -- silent success
		return nil
	}
	testConn.Close()

	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		// Connection failed -- silent success
		return nil
	}
	defer conn.Close()

	client := serenav1.NewForwarderServiceClient(conn)
	ctx, cancel := context.WithTimeout(cmd.Context(), 10*time.Second)
	defer cancel()

	_, _ = client.DeactivateWorkspace(ctx, &serenav1.DeactivateRequest{
		WorkspacePath: absPath,
	})
	// Ignore errors -- deactivation is best-effort (D-15)

	return nil
}
