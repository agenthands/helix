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

// Command group IDs for organizing the help screen by capability (cobra
// AddGroup, v1.10.2). These IDs are part of the help contract — Phase 91 adds
// the generated verb groups (navigation/edit/fileops/diagnostics/repomap/
// memory) on top of this scaffold, so DO NOT rename these without updating the
// commands assigned to them.
const (
	// groupWorkspace: per-project workspace lifecycle commands.
	groupWorkspace = "workspace"
	// groupRuntime: setup/status of the helix runtime and its clients.
	groupRuntime = "runtime"
	// groupMaintenance: self-update / upgrade of the helix binary.
	groupMaintenance = "maintenance"

	// The 6 generated-verb capability groups (Phase 91). These IDs are part of
	// the help contract and MUST match the categoryToGroup map in
	// cmd/helix-cligen/render.go — the generator emits these literals into
	// verbSpecs[*].groupID.
	groupNavigation  = "navigation"
	groupEdit        = "edit"
	groupFileops     = "fileops"
	groupDiagnostics = "diagnostics"
	groupRepomap     = "repomap"
	groupMemory      = "memory"
)

// runForwarderFn / runDaemonFn are overridable seams over the real
// runForwarder / runDaemon entry points. Production code uses the real
// functions; tests substitute fakes to assert routing (which entry point a
// given invocation reaches) without spawning a daemon or opening a stdio MCP
// session. See root_test.go.
var (
	runForwarderFn = runForwarder
	runDaemonFn    = runDaemon
)

// NewRootCommand creates the root cobra command with all flags.
// Per D-02: flat CLI with flags, no subcommands.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "helix",
		Short: "Helix code intelligence MCP server",
		Long:  "Helix - LSP-backed MCP runtime for semantic code operations",
		RunE:  runRoot,
		// Per Pitfall 6: prevent help on errors. As of CLI-04 (Phase 90), a
		// bare no-arg `helix` prints grouped help and exits 0 (see runRoot)
		// instead of entering the stdio forwarder.
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	// Command groups organize the help screen by capability (cobra AddGroup).
	// Phase 91 adds the 6 generated-verb capability groups on top of the
	// original scaffold; registerGeneratedVerbs attaches each generated verb to
	// one of these by GroupID.
	rootCmd.AddGroup(
		&cobra.Group{ID: groupWorkspace, Title: "Workspace Commands:"},
		&cobra.Group{ID: groupRuntime, Title: "Runtime Commands:"},
		&cobra.Group{ID: groupMaintenance, Title: "Maintenance Commands:"},
		&cobra.Group{ID: groupNavigation, Title: "Navigation:"},
		&cobra.Group{ID: groupEdit, Title: "Edit:"},
		&cobra.Group{ID: groupFileops, Title: "File Operations:"},
		&cobra.Group{ID: groupDiagnostics, Title: "Diagnostics:"},
		&cobra.Group{ID: groupRepomap, Title: "Repo Map & Semantic:"},
		&cobra.Group{ID: groupMemory, Title: "Memory & Workflow:"},
	)

	// Transport mode: stdio (default/forwarder), http, auto
	rootCmd.Flags().String("mode", "auto", "Transport mode: stdio, http, auto")
	// Run as daemon directly (skip forwarder)
	rootCmd.Flags().Bool("serve", false, "Run as daemon directly (skip forwarder)")
	// --json is DUAL-PURPOSE and PERSISTENT (Phase 92-02, RESEARCH Pitfall 1
	// option a). The daemon/forwarder dispatch paths read it as the log format
	// (runForwarder/runDaemon below), while the disjoint verb path reads it as
	// "emit verb output as compact JSON lines" (render.go resolveRenderOpts).
	// These two read sites never overlap for a single invocation, so one flag
	// safely serves both. It must be persistent so generated verbs inherit it.
	rootCmd.PersistentFlags().Bool("json", false, "JSON output: log format for the daemon/forwarder; compact JSON lines for verb output")
	// --color and --abs are persistent verb-output flags inherited by every
	// generated verb (Phase 92-02). --color gates ANSI generation; --abs emits
	// absolute paths instead of the workspace-relative default (OUT-06/07).
	rootCmd.PersistentFlags().String("color", "auto", "Colorize verb output: auto|always|never")
	rootCmd.PersistentFlags().Bool("abs", false, "Emit absolute paths instead of workspace-relative")
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
	// Phase 76 ABLATE-05/07 ablation overrides. Default false (opt-in
	// disable). When set, force-disables the named subsystem regardless of
	// the resolved profile (CLI > profile > default-off precedence, D-02/D-03).
	rootCmd.Flags().Bool("disable-lsp-subsystem", false, "Ablation override: force-disable the LSP subsystem (no LS workers, no enrichment)")
	rootCmd.Flags().Bool("disable-structured-edit-subsystem", false, "Ablation override: force-disable structured-edit tools (replace_symbol_body, fuzzy_edit, insert_*)")
	// Phase 81 ABLATE-06: force-disable the semantic-store read seam (the
	// `no_semantic` arm). Build-but-block — the store still builds but every
	// SemanticLookup read is forced to NoopLookup (D-04). Maps to the NESTED
	// koanf key semantic_index.bench_disabled (distinct from the top-level
	// disable_lsp_subsystem path).
	rootCmd.Flags().Bool("disable-semantic-subsystem", false, "Ablation override: force-disable the semantic-store read seam (reads forced to tree-sitter fallback)")
	// Version
	rootCmd.Flags().Bool("version", false, "Print version and exit")

	// Subcommands, each assigned to a capability group so the help output is
	// organized rather than a flat list. addGrouped sets GroupID at
	// registration to keep the group wiring centralized in root.go.
	addGrouped := func(group string, cmd *cobra.Command) {
		cmd.GroupID = group
		rootCmd.AddCommand(cmd)
	}
	addGrouped(groupRuntime, newSetupCommand())
	addGrouped(groupRuntime, newStatusCommand())
	addGrouped(groupWorkspace, newActivateCommand())
	addGrouped(groupWorkspace, newDeactivateCommand())
	addGrouped(groupWorkspace, newNudgeCommand())
	addGrouped(groupMaintenance, newUpdateCommand())
	addGrouped(groupMaintenance, newUpgradeCommand())

	// Phase 91: attach the generated verb surface (one root subcommand per
	// live-registry tool) grouped by capability. registerGeneratedVerbs sets
	// each verb's GroupID from the generated verbSpecs catalog.
	registerGeneratedVerbs(rootCmd)

	return rootCmd
}

func runRoot(cmd *cobra.Command, args []string) error {
	showVersion, _ := cmd.Flags().GetBool("version")
	if showVersion {
		// Write to the command's configured output (defaults to os.Stdout) so
		// the version line honors cobra's writer and is testable.
		fmt.Fprintf(cmd.OutOrStdout(), "helix version %s\n", currentVersion)
		return nil
	}

	serve, _ := cmd.Flags().GetBool("serve")
	mode, _ := cmd.Flags().GetString("mode")

	if serve || mode == "http" {
		return runDaemonFn(cmd)
	}

	switch mode {
	case "stdio":
		// Explicit stdio: still run the forwarder. Phase 94 owns the full
		// forwarder-head deletion; CLI-04 only re-routes the no-arg/auto path.
		return runForwarderFn(cmd)
	case "auto":
		// CLI-04: a bare `helix` (default mode=auto, no subcommand) prints
		// grouped help and exits 0. It must NOT open a stdio MCP session — an
		// agent never asked for one. Cobra dispatches subcommands before
		// RunE, so reaching here means no verb was given.
		return cmd.Help()
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
	disableLSP, _ := cmd.Flags().GetBool("disable-lsp-subsystem")
	disableStructuredEdit, _ := cmd.Flags().GetBool("disable-structured-edit-subsystem")
	disableSemantic, _ := cmd.Flags().GetBool("disable-semantic-subsystem")

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
	// Phase 76 ABLATE-05/07: only apply the disable overrides when the flag
	// was explicitly set so a blank invocation cannot wipe a profile/config
	// value (these are opt-in force-disables; the effective flag is the OR
	// of CLI and resolved-profile fields at the daemon composition root).
	if disableLSP {
		overrides["disable_lsp_subsystem"] = true
	}
	if disableStructuredEdit {
		overrides["disable_structured_edit_subsystem"] = true
	}
	// Phase 81 ABLATE-06: only-when-set so a blank invocation cannot clear a
	// profile value. Note the NESTED koanf path (semantic_index.bench_disabled),
	// distinct from the top-level disable_lsp_subsystem key above.
	if disableSemantic {
		overrides["semantic_index.bench_disabled"] = true
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
