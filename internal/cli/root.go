package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// NewRootCommand creates the root cobra command with all flags.
// Per D-02: flat CLI with flags, no subcommands.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "serena",
		Short: "Serena code intelligence MCP server",
		Long:  "Serena 2.0 - LSP-backed MCP runtime for semantic code operations",
		RunE:  runRoot,
		// Per Pitfall 6: prevent help on errors, allow no-args to enter stdio mode
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Transport mode: stdio (default/forwarder), http, auto
	rootCmd.Flags().String("mode", "auto", "Transport mode: stdio, http, auto")
	// Run as daemon directly (skip forwarder)
	rootCmd.Flags().Bool("serve", false, "Run as daemon directly (skip forwarder)")
	// Logging
	rootCmd.Flags().Bool("json", false, "Use JSON log format (default: text)")
	// Socket override
	rootCmd.Flags().String("socket", "", "Override daemon socket path")
	// HTTP listen address
	rootCmd.Flags().String("http-addr", ":8080", "HTTP listen address for Streamable HTTP transport")
	// Config file override
	rootCmd.Flags().String("config", "", "Path to config file")
	// Version
	rootCmd.Flags().Bool("version", false, "Print version and exit")

	return rootCmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		fmt.Println("serena version 2.0.0-dev")
		return nil
	}

	serve, _ := cmd.Flags().GetBool("serve")
	if serve {
		// TODO: Plan 02 implements daemon startup
		return fmt.Errorf("daemon mode not yet implemented")
	}

	mode, _ := cmd.Flags().GetString("mode")
	switch mode {
	case "stdio", "auto":
		// TODO: Plan 03 implements forwarder
		return fmt.Errorf("forwarder mode not yet implemented")
	case "http":
		// HTTP mode IS daemon mode (per research Pattern 1)
		return fmt.Errorf("daemon mode not yet implemented")
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}
