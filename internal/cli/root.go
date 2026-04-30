package cli

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spf13/cobra"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/forwarder"
	"github.com/agenthands/helix/internal/obs"
)

// currentVersion holds the binary version string. Defaults to "dev" and is
// overwritten by SetVersion(), which the cmd/helix/main.go entrypoint calls
// with the ldflag-injected `var version` from the goreleaser build (see
// .goreleaser.yaml `-X main.version={{.Version}}`).
var currentVersion = "dev"

// SetVersion records the binary version string so `helix --version` reports
// the goreleaser-injected value. Called once from cmd/helix/main.go before
// Execute(). An empty argument is ignored so callers can pass a possibly-empty
// ldflag value without clobbering the "dev" default.
func SetVersion(v string) {
	if v == "" {
		return
	}
	currentVersion = v
}

// CurrentVersion returns the binary version string set via SetVersion().
// Defaults to "dev" until the cmd/helix/main.go entrypoint threads the
// ldflag-injected value. Used by internal/mcp/server.go to populate the
// MCP `Implementation.Version` field so the MCP client identity stays in
// lockstep with the CLI `--version` output (per RESEARCH.md A6).
func CurrentVersion() string {
	return currentVersion
}

// NewRootCommand creates the root cobra command with all flags.
// Per D-02: flat CLI with flags, no subcommands.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "helix",
		Short: "Helix code intelligence MCP server",
		Long:  "Helix - LSP-backed MCP runtime for semantic code operations",
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
	// Agent profile
	rootCmd.Flags().String("profile", "", "Agent profile (claude-code, codex, ide-assistant, ci-bot, full)")
	// Admin listener bind address (Phase 10 observability). Empty = disabled.
	// Must be loopback (127.0.0.1/localhost/::1); non-loopback deferred to v1.3 auth.
	rootCmd.Flags().String("admin-addr", "", "Loopback admin listener address (e.g. 127.0.0.1:9090); empty = disabled")
	// Version
	rootCmd.Flags().Bool("version", false, "Print version and exit")

	// Subcommands
	rootCmd.AddCommand(newSetupCommand())
	rootCmd.AddCommand(newStatusCommand())
	rootCmd.AddCommand(newActivateCommand())
	rootCmd.AddCommand(newDeactivateCommand())
	rootCmd.AddCommand(newNudgeCommand())

	return rootCmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		fmt.Printf("helix version %s\n", currentVersion)
		return nil
	}

	serve, _ := cmd.Flags().GetBool("serve")
	mode, _ := cmd.Flags().GetString("mode")

	if serve || mode == "http" {
		return runDaemon(cmd)
	}

	switch mode {
	case "stdio", "auto":
		return runForwarder(cmd)
	default:
		return fmt.Errorf("unknown mode: %s", mode)
	}
}

// newLogger creates a structured logger writing to stderr with the given format.
// It wraps the base handler with obs.ContextHandler so traced requests
// automatically get trace_id/span_id fields.
func newLogger(jsonLog bool) *slog.Logger {
	var handler slog.Handler
	if jsonLog {
		handler = slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	} else {
		handler = slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})
	}
	handler = obs.NewContextHandler(handler)
	return slog.New(handler)
}

// runForwarder starts the stdio forwarder that proxies MCP traffic to the daemon.
func runForwarder(cmd *cobra.Command) error {
	socketPath, _ := cmd.Flags().GetString("socket")
	jsonLog, _ := cmd.Flags().GetBool("json")

	logger := newLogger(jsonLog)

	// Use default socket path if not specified
	if socketPath == "" {
		socketPath = config.DefaultSocketPath()
	}

	return forwarder.RunForwarder(cmd.Context(), socketPath, logger)
}

// runDaemon starts the Helix daemon with config loading and signal handling.
func runDaemon(cmd *cobra.Command) error {
	jsonLog, _ := cmd.Flags().GetBool("json")
	socketPath, _ := cmd.Flags().GetString("socket")
	httpAddr, _ := cmd.Flags().GetString("http-addr")
	configPath, _ := cmd.Flags().GetString("config")
	profileName, _ := cmd.Flags().GetString("profile")
	adminAddr, _ := cmd.Flags().GetString("admin-addr")

	logger := newLogger(jsonLog)

	// Load config with CLI overrides
	overrides := make(map[string]interface{})
	if socketPath != "" {
		overrides["daemon.socket_path"] = socketPath
	}
	if cmd.Flags().Changed("http-addr") {
		overrides["daemon.http_addr"] = httpAddr
	}
	if profileName != "" {
		overrides["profile"] = profileName
	}
	// Pitfall #5: only apply the --admin-addr override when non-empty so a
	// blank CLI invocation cannot wipe a project config value.
	if adminAddr != "" {
		overrides["observability.admin_addr"] = adminAddr
	}

	cfg, err := config.Load("", configPath, overrides)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	d, err := daemon.New(cfg, logger)
	if err != nil {
		return fmt.Errorf("initializing daemon: %w", err)
	}
	return d.Run(cmd.Context())
}
