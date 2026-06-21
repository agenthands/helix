package cli

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	"github.com/spf13/cobra"

	"github.com/agenthands/helix/internal/langregistry"
)

// newSetupCommand creates the setup subcommand for registering Helix with coding agents.
func newSetupCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup [client]",
		Short: "Install the Helix skill + hooks for a coding agent and remove any prior MCP registration",
		Long: `Install the Helix Agent Skill and hooks for a supported coding agent, and
remove any prior Helix MCP server registration.

For Claude-family clients (claude-code, claude-desktop) this writes the embedded
Agent Skill that teaches the helix CLI verbs; claude-code also installs the
SessionStart/PreToolUse/Stop hooks. For other clients it removes any prior helix
MCP entry (those clients do not consume Agent Skills). In all cases the helix
binary's MCP daemon head is left intact.

Run without arguments to list available clients.
Run with a client name to set up Helix for that client.`,
		ValidArgs:     []string{"claude-code", "vscode", "jetbrains", "claude-desktop", "gemini-cli", "opencode", "generic"},
		Args:          cobra.MaximumNArgs(1),
		RunE:          runSetup,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	cmd.Flags().Bool("global", false, "Set up globally (user-scoped) instead of project-scoped")
	cmd.Flags().Bool("uninstall", false, "Remove Helix from the client (any prior MCP entry, hooks, and the Agent Skill)")
	cmd.Flags().Bool("skip-install", false, "Skip language server pre-installation")
	cmd.Flags().Bool("dry-run", false, "Show what would happen without making changes")
	cmd.Flags().String("output", "", "Output path for generic client config (default: stdout)")
	cmd.Flags().Bool("no-hooks", false, "Skip hook installation (Claude Code only)")
	cmd.Flags().Bool("no-skill", false, "Skip Agent Skill installation (Claude-family clients only)")

	return cmd
}

// runSetup orchestrates the setup flow: list clients, validate, resolve binary, register.
func runSetup(cmd *cobra.Command, args []string) error {
	registry := clientRegistry()
	printer := &SetupPrinter{}

	// No args: list available clients
	if len(args) == 0 {
		listClients(registry, printer)
		return nil
	}

	clientName := args[0]

	// Validate client name
	registrar, ok := registry[clientName]
	if !ok {
		validClients := make([]string, 0, len(registry))
		for name := range registry {
			validClients = append(validClients, name)
		}
		sort.Strings(validClients)
		return fmt.Errorf("unknown client %q; valid clients: %v", clientName, validClients)
	}

	// Resolve binary path
	binaryPath, err := resolveBinaryPath()
	if err != nil {
		return fmt.Errorf("resolving binary path: %w", err)
	}

	// Get current working directory
	projectDir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	// Extract flags
	global, _ := cmd.Flags().GetBool("global")
	uninstall, _ := cmd.Flags().GetBool("uninstall")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	outputPath, _ := cmd.Flags().GetString("output")
	noHooks, _ := cmd.Flags().GetBool("no-hooks")
	noSkill, _ := cmd.Flags().GetBool("no-skill")

	printer.DryRun = dryRun

	cfg := RegistrationConfig{
		BinaryPath: binaryPath,
		Global:     global,
		DryRun:     dryRun,
		ProjectDir: projectDir,
		OutputPath: outputPath,
		NoHooks:    noHooks,
		NoSkill:    noSkill,
		Printer:    printer,
	}

	// Uninstall flow
	if uninstall {
		if err := registrar.Unregister(cfg); err != nil {
			printer.Failure("%s unregistration failed: %s", clientName, err)
			return err
		}
		printer.Success("%s unregistered", clientName)
		return nil
	}

	// Setup flow (install skill + hooks, tear down prior MCP registration)
	if err := registrar.Register(cfg); err != nil {
		printer.Failure("%s setup failed: %s", clientName, err)
		return err
	}
	printer.Success("%s set up", clientName)

	// Language detection
	reg, regErr := langregistry.NewRegistry()
	if regErr != nil {
		printer.Failure("language registry unavailable: %s", regErr)
		// Non-fatal: skip detection and installation
		return nil
	}

	entries, _ := detectLanguages(cfg.ProjectDir, reg, printer)

	// LS pre-installation (unless --skip-install)
	skipInstall, _ := cmd.Flags().GetBool("skip-install")
	var installed []langregistry.LSEntry
	if !skipInstall && len(entries) > 0 {
		logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn}))
		inst := langregistry.NewInstaller(langregistry.InstallerConfig{AutoInstall: true}, logger)
		installed = preInstallLanguageServers(cmd.Context(), entries, inst, printer, cfg.DryRun)
	} else if skipInstall && len(entries) > 0 {
		printer.Info("Skipping language server installation (--skip-install)")
		installed = entries // pass through for health check binary lookup
	}

	// Health check
	if len(installed) > 0 {
		_ = runHealthCheck(cmd.Context(), installed, cfg.ProjectDir, printer, cfg.DryRun)
	}

	return nil
}

// listClients prints all available clients with their descriptions.
func listClients(registry map[string]ClientRegistrar, printer *SetupPrinter) {
	printer.Info("Available clients:")

	// Sort client names for stable output
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		registrar := registry[name]
		fmt.Fprintf(os.Stderr, "  %-16s %s\n", name, registrar.Description())
	}
}

// resolveBinaryPath returns the absolute path to the helix binary.
// Uses os.Executable() + filepath.EvalSymlinks(), falling back to unresolved if symlink eval fails.
func resolveBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("cannot determine helix binary path: %w", err)
	}
	resolved, err := filepath.EvalSymlinks(exe)
	if err != nil {
		// Fall back to unresolved path
		return exe, nil
	}
	return resolved, nil
}
