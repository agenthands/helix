package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	serenav1 "github.com/agenthands/helix/api/proto/serena/v1"
	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/kernel/lspool"
)

// newStatusCommand creates the status subcommand for displaying workspace health.
func newStatusCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show workspace health and language server status",
		Long: `Query the running Helix daemon and display language server health.
Defaults to showing only unhealthy servers. Use --verbose for full output.`,
		RunE:          runStatus,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().Bool("json", false, "Output as JSON (same format as get_health tool)")
	cmd.Flags().BoolP("verbose", "v", false, "Show all language servers including healthy ones")

	return cmd
}

// runStatus connects to the running daemon via gRPC and displays health status.
func runStatus(cmd *cobra.Command, _ []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	verbose, _ := cmd.Flags().GetBool("verbose")

	socketPath := config.DefaultSocketPath()

	// Check if daemon is running per D-07.
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		fmt.Fprintln(os.Stderr, "Daemon not running")
		os.Exit(1)
	}

	// Verify socket is alive with a short dial timeout per T-35-06.
	testConn, err := net.DialTimeout("unix", socketPath, 2*time.Second)
	if err != nil {
		fmt.Fprintln(os.Stderr, "Daemon not running")
		os.Exit(1)
	}
	testConn.Close()

	// Connect via gRPC.
	conn, err := grpc.NewClient("unix://"+socketPath,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return fmt.Errorf("connecting to daemon: %w", err)
	}
	defer conn.Close()

	client := serenav1.NewForwarderServiceClient(conn)

	// Query health with 5-second timeout per T-35-06.
	ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
	defer cancel()

	resp, err := client.GetStatus(ctx, &serenav1.StatusRequest{Verbose: verbose})
	if err != nil {
		return fmt.Errorf("querying daemon status: %w", err)
	}

	var report lspool.HealthReport
	if err := json.Unmarshal(resp.Payload, &report); err != nil {
		return fmt.Errorf("parsing health report: %w", err)
	}

	// JSON output per D-08: always return verbose data.
	if jsonOut {
		if !verbose {
			// Re-query with verbose=true for full data.
			resp, err = client.GetStatus(ctx, &serenav1.StatusRequest{Verbose: true})
			if err != nil {
				return fmt.Errorf("querying daemon status (verbose): %w", err)
			}
			if err := json.Unmarshal(resp.Payload, &report); err != nil {
				return fmt.Errorf("parsing health report: %w", err)
			}
		}
		out, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling JSON output: %w", err)
		}
		fmt.Println(string(out))
		return nil
	}

	// Colored terminal output.
	printer := &StatusPrinter{}
	printer.PrintReport(&report, verbose)

	// Exit code 1 if any failures per D-07.
	if printer.HasFailures(&report) {
		os.Exit(1)
	}

	return nil
}
