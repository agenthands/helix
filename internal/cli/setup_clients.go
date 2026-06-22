package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ClientRegistrar handles MCP registration for a specific coding agent.
type ClientRegistrar interface {
	// Name returns the client identifier (e.g., "claude-code").
	Name() string
	// Description returns a one-line description for help output.
	Description() string
	// Register installs the Helix skill + hooks for this client and tears down
	// any prior Helix MCP server registration (the Phase 93 "flip").
	Register(cfg RegistrationConfig) error
	// Unregister removes Helix from this client (MCP entry AND hooks).
	Unregister(cfg RegistrationConfig) error
	// teardownMCP removes ONLY the prior Helix MCP server entry for this client.
	// Unlike Unregister it MUST NOT remove hooks (Pitfall 3) and MUST be a
	// best-effort no-op when the prior entry / required CLI is absent.
	teardownMCP(cfg RegistrationConfig) error
}

// RegistrationConfig holds common registration parameters.
type RegistrationConfig struct {
	BinaryPath string        // Absolute path to helix binary (resolved via os.Executable + filepath.EvalSymlinks)
	Global     bool          // --global flag
	DryRun     bool          // --dry-run flag
	ProjectDir string        // Current working directory
	OutputPath string        // --output flag (generic client only)
	NoHooks    bool          // --no-hooks flag (skip hook installation)
	NoSkill    bool          // --no-skill flag (skip Agent Skill installation)
	Printer    *SetupPrinter // Colored output helper
}

// clientRegistry returns all 7 registrars keyed by client name.
func clientRegistry() map[string]ClientRegistrar {
	return map[string]ClientRegistrar{
		"claude-code":    &ClaudeCodeRegistrar{},
		"vscode":         &VSCodeRegistrar{},
		"jetbrains":      &JetBrainsRegistrar{},
		"claude-desktop": &ClaudeDesktopRegistrar{},
		"gemini-cli":     &GeminiCLIRegistrar{},
		"opencode":       &OpenCodeRegistrar{},
		"generic":        &GenericRegistrar{},
		"codex":          &CodexRegistrar{},
	}
}

// --- Shared helpers ---

// removeFromJSONConfig removes a server entry from a JSON config file.
// If the file doesn't exist, returns nil (nothing to remove) per T-34-05.
func removeFromJSONConfig(path, key, serverName string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading config %s: %w", path, err)
	}

	existing := make(map[string]any)
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("parsing config %s: %w", path, err)
	}

	servers, ok := existing[key].(map[string]any)
	if !ok {
		return nil // key doesn't exist, nothing to remove
	}

	delete(servers, serverName)
	existing[key] = servers

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	return os.WriteFile(path, append(out, '\n'), 0644)
}

// teardownOnlyRegister implements the Phase 93 flip for non-Claude clients that
// do NOT consume Agent Skills (gemini-cli, vscode, jetbrains, opencode, generic):
// it tears down any prior helix MCP entry and writes no skill. The Info line
// records that the client does not consume Agent Skills so the migration is
// auditable. DryRun is honored inside the per-client teardownMCP.
func teardownOnlyRegister(r ClientRegistrar, cfg RegistrationConfig, clientName string) error {
	if err := r.teardownMCP(cfg); err != nil {
		return fmt.Errorf("tearing down prior MCP registration: %w", err)
	}
	if !cfg.DryRun {
		cfg.Printer.Info("removed any prior helix MCP entry; %s does not consume Agent Skills (no skill written)", clientName)
	}
	return nil
}

// userConfigDir wraps os.UserConfigDir for platform-specific config directory.
func userConfigDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine user config directory: %w", err)
	}
	return dir, nil
}

// --- ClaudeCodeRegistrar ---

// ClaudeCodeRegistrar handles MCP registration for Claude Code (Anthropic CLI agent).
type ClaudeCodeRegistrar struct{}

func (r *ClaudeCodeRegistrar) Name() string        { return "claude-code" }
func (r *ClaudeCodeRegistrar) Description() string { return "Claude Code (Anthropic CLI agent)" }

// claudeDir resolves the Claude Code `.claude` directory for skill installation:
// <ProjectDir>/.claude (project) or ~/.claude (global). Mirrors hookSettingsPath's
// project-vs-global choice (setup_hooks.go), including its home-dir fallback.
func claudeDir(projectDir string, global bool) string {
	if global {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, ".claude")
		}
	}
	return filepath.Join(projectDir, ".claude")
}

// Register performs the Phase 93 "flip" for Claude Code: tear down any prior
// helix MCP server entry, install the embedded Agent Skill, then install the
// idempotent hooks. It no longer registers an MCP server (the daemon MCP head
// remains intact — Phase 94 owns its deletion).
func (r *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
	skillDir := skillTargetDir(claudeDir(cfg.ProjectDir, cfg.Global))
	settingsPath := hookSettingsPath(cfg.ProjectDir, cfg.Global)

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would tear down any prior helix MCP entry")
		if !cfg.NoSkill {
			cfg.Printer.DryRunAction("would install Agent Skill to %s/SKILL.md", skillDir)
		}
		if !cfg.NoHooks {
			cfg.Printer.DryRunAction("would write hooks to %s", settingsPath)
		}
		return nil
	}

	// 1. Tear down any prior helix MCP registration (hook-preserving, best-effort).
	if err := r.teardownMCP(cfg); err != nil {
		return fmt.Errorf("tearing down prior MCP registration: %w", err)
	}

	// 2. Install the embedded Agent Skill (unless --no-skill).
	if !cfg.NoSkill {
		if err := installSkill(skillDir); err != nil {
			return fmt.Errorf("installing skill: %w", err)
		}
		cfg.Printer.Success("installed Helix skill to %s/SKILL.md", skillDir)
	}

	// 3. Install hooks (per D-01, D-16) — non-fatal on failure.
	if !cfg.NoHooks {
		if err := mergeHooksIntoSettings(settingsPath, cfg.BinaryPath); err != nil {
			cfg.Printer.Failure("hook installation failed: %s", err)
			cfg.Printer.Info("skill installed; hooks can be installed manually")
			return nil // Non-fatal per D-16.
		}
		cfg.Printer.Success("installed hooks (SessionStart, PreToolUse, Stop)")
	}

	return nil
}

// teardownMCP removes the prior Helix MCP server entry for Claude Code WITHOUT
// touching hooks (Pitfall 3). It is best-effort: a missing `claude` CLI or an
// absent entry is a no-op, never a hard error. It mirrors the MCP-removal half
// of Unregister (the `claude mcp remove` + `.mcp.json` / global settings.json
// direct-file removal) but deliberately omits removeHooksFromSettings.
func (r *ClaudeCodeRegistrar) teardownMCP(cfg RegistrationConfig) error {
	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry (claude mcp remove helix --scope %s + .mcp.json/settings.json)", scope)
		return nil
	}

	// Best-effort CLI removal (only when the claude CLI is present).
	if _, err := exec.LookPath("claude"); err == nil {
		rmCmd := exec.Command("claude", "mcp", "remove", "helix", "--scope", scope)
		rmCmd.Stdout = os.Stderr
		rmCmd.Stderr = os.Stderr
		_ = rmCmd.Run() // ignore: absent entry is a no-op
	}

	// Direct-file removal (idempotent — missing file/key → no-op). Cover both
	// the project .mcp.json and the global settings.json so a prior entry written
	// via either path is removed regardless of CLI availability.
	if err := removeFromJSONConfig(filepath.Join(cfg.ProjectDir, ".mcp.json"), "mcpServers", "helix"); err != nil {
		return err
	}
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		globalSettings := filepath.Join(home, ".claude", "settings.json")
		if err := removeFromJSONConfig(globalSettings, "mcpServers", "helix"); err != nil {
			return err
		}
	}
	return nil
}

func (r *ClaudeCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "remove", "helix", "--scope", scope}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: claude %s (best-effort)", strings.Join(cmdArgs, " "))
		cfg.Printer.DryRunAction("would remove hooks from %s", hookSettingsPath(cfg.ProjectDir, cfg.Global))
		cfg.Printer.DryRunAction("would remove Agent Skill from %s/SKILL.md", skillTargetDir(claudeDir(cfg.ProjectDir, cfg.Global)))
		return nil
	}

	// 1. MCP removal is BEST-EFFORT (CR-93-01). Post-flip, setup no longer
	// registers an MCP entry, so `claude mcp remove helix` normally exits
	// non-zero (no such entry). Treat that as a no-op — mirroring teardownMCP —
	// and ALWAYS proceed to hook + skill removal, which is what --uninstall
	// actually promises now. A missing `claude` CLI is likewise non-fatal.
	if _, err := exec.LookPath("claude"); err == nil {
		cmd := exec.Command("claude", cmdArgs...)
		cmd.Stdout = os.Stderr
		cmd.Stderr = os.Stderr
		_ = cmd.Run() // ignore: absent MCP entry is a no-op, not a fatal error
	}
	// Direct-file MCP removal (idempotent — missing file/key → no-op), matching
	// teardownMCP so an entry written via either path is removed.
	if err := removeFromJSONConfig(filepath.Join(cfg.ProjectDir, ".mcp.json"), "mcpServers", "helix"); err != nil {
		cfg.Printer.Failure("removing .mcp.json entry failed: %s", err)
	}
	if home, homeErr := os.UserHomeDir(); homeErr == nil {
		globalSettings := filepath.Join(home, ".claude", "settings.json")
		if err := removeFromJSONConfig(globalSettings, "mcpServers", "helix"); err != nil {
			cfg.Printer.Failure("removing global settings entry failed: %s", err)
		}
	}

	// 2. Hook removal (per D-03).
	settingsPath := hookSettingsPath(cfg.ProjectDir, cfg.Global)
	if err := removeHooksFromSettings(settingsPath); err != nil {
		cfg.Printer.Failure("hook removal failed: %s", err)
	} else {
		cfg.Printer.Success("removed hooks from %s", settingsPath)
	}

	// 3. Skill removal (WR-93-01) — the skill is the primary artifact setup now
	// installs, so --uninstall must remove it too. Best-effort, path-contained.
	skillDir := skillTargetDir(claudeDir(cfg.ProjectDir, cfg.Global))
	if err := uninstallSkill(skillDir); err != nil {
		cfg.Printer.Failure("skill removal failed: %s", err)
	} else {
		cfg.Printer.Success("removed Agent Skill from %s/SKILL.md", skillDir)
	}

	return nil
}

// --- GeminiCLIRegistrar ---

// GeminiCLIRegistrar handles MCP registration for Gemini CLI.
type GeminiCLIRegistrar struct{}

func (r *GeminiCLIRegistrar) Name() string        { return "gemini-cli" }
func (r *GeminiCLIRegistrar) Description() string { return "Gemini CLI (Google)" }

// Register tears down any prior helix MCP entry (the Phase 93 flip) and then ALSO
// writes a GEMINI.md instruction file with the idempotent Helix steering block
// (AGENT-02/03). Gemini CLI has NO PreToolUse-equivalent, so NO hook artifact is
// written — instruction-file steering only (locked anti-feature). Gemini documents
// no hard project-doc byte cap, so the writer uses no cap (maxBytes <= 0).
func (r *GeminiCLIRegistrar) Register(cfg RegistrationConfig) error {
	if err := teardownOnlyRegister(r, cfg, "gemini-cli"); err != nil {
		return err
	}
	path := geminiInstructionPath(cfg.ProjectDir, cfg.Global)
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write Helix steering block to %s (no hook — Gemini has no PreToolUse)", path)
		return nil
	}
	if err := writeAgentInstructions(path, agentInstructionBody(), 0); err != nil {
		return fmt.Errorf("writing GEMINI.md instructions: %w", err)
	}
	cfg.Printer.Success("wrote Helix steering block to %s (instruction-file only; Gemini has no hook surface)", path)
	return nil
}

// ensureDisabled sets {"helix": {"enabled": false}} in Gemini's mcp-server-enablement.json.
func (r *GeminiCLIRegistrar) ensureDisabled() error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	enablementPath := filepath.Join(home, ".gemini", "mcp-server-enablement.json")

	existing := make(map[string]any)
	if data, readErr := os.ReadFile(enablementPath); readErr == nil {
		_ = json.Unmarshal(data, &existing)
	}

	existing["helix"] = map[string]any{"enabled": false}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling enablement config: %w", err)
	}

	return os.WriteFile(enablementPath, append(data, '\n'), 0644)
}

// settingsPath returns the Gemini CLI settings file path.
func (r *GeminiCLIRegistrar) settingsPath(cfg RegistrationConfig) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	if cfg.Global {
		return filepath.Join(home, ".gemini", "settings.json"), nil
	}
	return filepath.Join(cfg.ProjectDir, ".gemini", "settings.json"), nil
}

// teardownMCP removes the prior Helix MCP server entry for Gemini CLI WITHOUT
// touching hooks. It removes the settings-file mcpServers.helix entry and marks
// helix disabled in the enablement file. Best-effort: missing files are no-ops
// and enablement-write failure is non-fatal.
func (r *GeminiCLIRegistrar) teardownMCP(cfg RegistrationConfig) error {
	configPath, err := r.settingsPath(cfg)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry from %s and disable in mcp-server-enablement.json", configPath)
		return nil
	}

	if err := removeFromJSONConfig(configPath, "mcpServers", "helix"); err != nil {
		return err
	}
	// Best-effort: disabling failure (e.g., missing home dir) must not break teardown.
	_ = r.ensureDisabled()
	return nil
}

func (r *GeminiCLIRegistrar) Unregister(cfg RegistrationConfig) error {
	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "remove", "helix", "--scope", scope}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: gemini %s", strings.Join(cmdArgs, " "))
		cfg.Printer.DryRunAction("would disable helix in mcp-server-enablement.json")
		return nil
	}

	// Disable in enablement file
	if err := r.ensureDisabled(); err != nil {
		cfg.Printer.Failure("could not disable in mcp-server-enablement.json: %s", err)
	}

	if _, err := exec.LookPath("gemini"); err != nil {
		// No CLI — remove directly from settings file
		configPath, pathErr := r.settingsPath(cfg)
		if pathErr != nil {
			return pathErr
		}
		return removeFromJSONConfig(configPath, "mcpServers", "helix")
	}

	cmd := exec.Command("gemini", cmdArgs...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gemini mcp remove failed: %w", err)
	}
	return nil
}

// --- VSCodeRegistrar ---

// VSCodeRegistrar handles MCP registration for VS Code / Copilot.
// NOTE: VS Code uses "servers" key, NOT "mcpServers" (Pitfall 1).
type VSCodeRegistrar struct{}

func (r *VSCodeRegistrar) Name() string        { return "vscode" }
func (r *VSCodeRegistrar) Description() string { return "VS Code / Copilot (Microsoft)" }

// Register performs the Phase 93 flip for VS Code: MCP-teardown only (no skill consumer).
func (r *VSCodeRegistrar) Register(cfg RegistrationConfig) error {
	return teardownOnlyRegister(r, cfg, "vscode")
}

// teardownMCP removes the prior Helix MCP server entry for VS Code WITHOUT
// touching hooks. VS Code uses the "servers" key (Pitfall 1). Missing file → no-op.
func (r *VSCodeRegistrar) teardownMCP(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry from %s", configPath)
		return nil
	}
	return removeFromJSONConfig(configPath, "servers", "helix")
}

func (r *VSCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove helix from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "servers", "helix")
}

func (r *VSCodeRegistrar) configPath(cfg RegistrationConfig) (string, error) {
	if cfg.Global {
		dir, err := userConfigDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(dir, "Code", "User", "mcp.json"), nil
	}
	return filepath.Join(cfg.ProjectDir, ".vscode", "mcp.json"), nil
}

// --- JetBrainsRegistrar ---

// JetBrainsRegistrar handles MCP registration for JetBrains IDEs via Junie.
type JetBrainsRegistrar struct{}

func (r *JetBrainsRegistrar) Name() string        { return "jetbrains" }
func (r *JetBrainsRegistrar) Description() string { return "JetBrains IDEs via Junie" }

// Register performs the Phase 93 flip for JetBrains/Junie: MCP-teardown only (no skill consumer).
func (r *JetBrainsRegistrar) Register(cfg RegistrationConfig) error {
	return teardownOnlyRegister(r, cfg, "jetbrains")
}

// teardownMCP removes the prior Helix MCP server entry for JetBrains/Junie
// WITHOUT touching hooks. Missing file → no-op.
func (r *JetBrainsRegistrar) teardownMCP(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry from %s", configPath)
		return nil
	}
	return removeFromJSONConfig(configPath, "mcpServers", "helix")
}

func (r *JetBrainsRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove helix from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "mcpServers", "helix")
}

func (r *JetBrainsRegistrar) configPath(cfg RegistrationConfig) (string, error) {
	if cfg.Global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine user home directory: %w", err)
		}
		return filepath.Join(home, ".junie", "mcp", "mcp.json"), nil
	}
	return filepath.Join(cfg.ProjectDir, ".junie", "mcp", "mcp.json"), nil
}

// --- ClaudeDesktopRegistrar ---

// ClaudeDesktopRegistrar handles MCP registration for Claude Desktop app.
// Claude Desktop is global-only (the Global flag is ignored).
type ClaudeDesktopRegistrar struct{}

func (r *ClaudeDesktopRegistrar) Name() string        { return "claude-desktop" }
func (r *ClaudeDesktopRegistrar) Description() string { return "Claude Desktop app" }

// Register performs the Phase 93 flip for Claude Desktop: tear down any prior
// helix MCP entry, then install the embedded Agent Skill into the global
// ~/.claude skills dir. Claude Desktop is global-only and has no hook installer
// here, so only the skill is written.
func (r *ClaudeDesktopRegistrar) Register(cfg RegistrationConfig) error {
	// Claude Desktop is always global; resolve the global ~/.claude skills dir.
	skillDir := skillTargetDir(claudeDir(cfg.ProjectDir, true))

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would tear down any prior helix MCP entry")
		if !cfg.NoSkill {
			cfg.Printer.DryRunAction("would install Agent Skill to %s/SKILL.md", skillDir)
		}
		return nil
	}

	if err := r.teardownMCP(cfg); err != nil {
		return fmt.Errorf("tearing down prior MCP registration: %w", err)
	}

	if !cfg.NoSkill {
		if err := installSkill(skillDir); err != nil {
			return fmt.Errorf("installing skill: %w", err)
		}
		cfg.Printer.Success("installed Helix skill to %s/SKILL.md", skillDir)
	}
	return nil
}

// teardownMCP removes the prior Helix MCP server entry for Claude Desktop
// WITHOUT touching hooks. Claude Desktop is global-only. Missing file → no-op.
func (r *ClaudeDesktopRegistrar) teardownMCP(cfg RegistrationConfig) error {
	configPath, err := r.configPath()
	if err != nil {
		return err
	}
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry from %s", configPath)
		return nil
	}
	return removeFromJSONConfig(configPath, "mcpServers", "helix")
}

func (r *ClaudeDesktopRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath, err := r.configPath()
	if err != nil {
		return err
	}

	// Claude Desktop is global-only; resolve the global ~/.claude skills dir.
	skillDir := skillTargetDir(claudeDir(cfg.ProjectDir, true))

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove helix from %s", configPath)
		cfg.Printer.DryRunAction("would remove Agent Skill from %s/SKILL.md", skillDir)
		return nil
	}

	// MCP removal is best-effort (a missing entry is a no-op); always proceed to
	// skill removal so --uninstall tears down the primary artifact (WR-93-01).
	if err := removeFromJSONConfig(configPath, "mcpServers", "helix"); err != nil {
		cfg.Printer.Failure("removing MCP entry failed: %s", err)
	}

	if err := uninstallSkill(skillDir); err != nil {
		cfg.Printer.Failure("skill removal failed: %s", err)
	} else {
		cfg.Printer.Success("removed Agent Skill from %s/SKILL.md", skillDir)
	}
	return nil
}

// configPath returns the platform-specific Claude Desktop config path.
// Claude Desktop is always global -- there is no project-scoped config.
func (r *ClaudeDesktopRegistrar) configPath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine user home directory: %w", err)
		}
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json"), nil
	case "windows":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return "", fmt.Errorf("APPDATA environment variable is not set")
		}
		return filepath.Join(appData, "Claude", "claude_desktop_config.json"), nil
	default: // linux and others
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine user home directory: %w", err)
		}
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json"), nil
	}
}

// --- OpenCodeRegistrar ---

// OpenCodeRegistrar handles MCP registration for OpenCode.
// OpenCode uses "mcp" key (not "mcpServers") and "command" is an array.
type OpenCodeRegistrar struct{}

func (r *OpenCodeRegistrar) Name() string        { return "opencode" }
func (r *OpenCodeRegistrar) Description() string { return "OpenCode" }

// Register performs the Phase 93 flip for OpenCode: MCP-teardown only (no skill consumer).
func (r *OpenCodeRegistrar) Register(cfg RegistrationConfig) error {
	return teardownOnlyRegister(r, cfg, "opencode")
}

// teardownMCP removes the prior Helix MCP server entry for OpenCode WITHOUT
// touching hooks. OpenCode uses the "mcp" key. Missing file → no-op.
func (r *OpenCodeRegistrar) teardownMCP(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry from %s", configPath)
		return nil
	}
	return removeFromJSONConfig(configPath, "mcp", "helix")
}

func (r *OpenCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove helix from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "mcp", "helix")
}

func (r *OpenCodeRegistrar) configPath(cfg RegistrationConfig) (string, error) {
	if cfg.Global {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot determine home directory: %w", err)
		}
		return filepath.Join(home, ".config", "opencode", "opencode.json"), nil
	}
	return filepath.Join(cfg.ProjectDir, "opencode.json"), nil
}

// --- GenericRegistrar ---

// GenericRegistrar handles MCP registration for any stdio-based MCP client.
// Outputs JSON to stdout (default) or writes to a file via --output.
type GenericRegistrar struct{}

func (r *GenericRegistrar) Name() string        { return "generic" }
func (r *GenericRegistrar) Description() string { return "Generic MCP stdio config (any client)" }

// Register tears down any prior helix MCP entry (the Phase 93 flip) and then ALSO
// writes the generic cross-tool instruction file (AGENTS.md at the project root —
// the emerging agents.md standard) with the idempotent Helix steering block
// (AGENT-02/03). No hook is written (the generic client has no hook surface).
// DryRun is honored (no write); --output continues to govern any prior on-disk MCP
// entry removal in teardownMCP.
func (r *GenericRegistrar) Register(cfg RegistrationConfig) error {
	if err := teardownOnlyRegister(r, cfg, "generic"); err != nil {
		return err
	}
	path := genericInstructionPath(cfg.ProjectDir)
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write Helix steering block to %s (no hook — generic instruction file)", path)
		return nil
	}
	if err := writeAgentInstructions(path, agentInstructionBody(), 0); err != nil {
		return fmt.Errorf("writing generic AGENTS.md instructions: %w", err)
	}
	cfg.Printer.Success("wrote Helix steering block to %s (instruction-file only)", path)
	return nil
}

// teardownMCP for the generic client is a documented no-op when the config is
// emitted to stdout (there is nothing on disk to clean). When --output points at
// a file, the prior helix entry is removed from it. Best-effort; missing file → no-op.
func (r *GenericRegistrar) teardownMCP(cfg RegistrationConfig) error {
	if cfg.OutputPath == "" {
		// stdout-only config: no prior on-disk MCP entry to tear down.
		return nil
	}
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry from %s", cfg.OutputPath)
		return nil
	}
	return removeFromJSONConfig(cfg.OutputPath, "mcpServers", "helix")
}

func (r *GenericRegistrar) Unregister(cfg RegistrationConfig) error {
	if cfg.OutputPath != "" {
		if cfg.DryRun {
			cfg.Printer.DryRunAction("would remove helix from %s", cfg.OutputPath)
			return nil
		}
		return removeFromJSONConfig(cfg.OutputPath, "mcpServers", "helix")
	}
	cfg.Printer.Info("generic config was printed to stdout and cannot be unregistered automatically")
	return nil
}

// --- CodexRegistrar ---

// CodexRegistrar handles setup for the OpenAI Codex CLI — the only non-Claude
// client with a PreToolUse-equivalent. Register writes BOTH an AGENTS.md (terse
// Helix steering block, 32-KiB capped) AND a hooks.json whose PreToolUse command
// invokes `<bin> nudge` (the SAME runNudge engine Claude uses — one engine, two
// runtimes; AGENT-03). The hook is advisory-only (additionalContext, exit 0); it
// NEVER sets a deny default.
type CodexRegistrar struct{}

func (r *CodexRegistrar) Name() string { return "codex" }
func (r *CodexRegistrar) Description() string {
	return "OpenAI Codex CLI (AGENTS.md + PreToolUse hook)"
}

// Register writes the Codex AGENTS.md instruction file (idempotent sentinel block,
// 32-KiB cap) and the Codex hooks.json (PreToolUse → `helix nudge`). Codex has no
// prior helix MCP entry today, so the MCP teardown is a documented best-effort
// no-op. DryRun is honored (no writes).
func (r *CodexRegistrar) Register(cfg RegistrationConfig) error {
	agentsPath := codexAgentsPath(cfg.ProjectDir, cfg.Global)
	hooksPath := codexHooksPath(cfg.ProjectDir, cfg.Global)

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would tear down any prior helix MCP entry (codex: best-effort no-op)")
		cfg.Printer.DryRunAction("would write Helix steering block to %s (≤32 KiB)", agentsPath)
		cfg.Printer.DryRunAction("would write Codex PreToolUse hook to %s (command: %s nudge)", hooksPath, cfg.BinaryPath)
		return nil
	}

	// Best-effort MCP teardown (no prior codex helix MCP entry today — documented no-op).
	if err := r.teardownMCP(cfg); err != nil {
		return fmt.Errorf("tearing down prior MCP registration: %w", err)
	}

	if err := writeAgentInstructions(agentsPath, agentInstructionBody(), codexAGENTSMaxBytes); err != nil {
		return fmt.Errorf("writing Codex AGENTS.md instructions: %w", err)
	}
	cfg.Printer.Success("wrote Helix steering block to %s (≤32 KiB)", agentsPath)

	if err := writeCodexHooks(hooksPath, cfg.BinaryPath); err != nil {
		return fmt.Errorf("writing Codex hooks.json: %w", err)
	}
	cfg.Printer.Success("wrote Codex PreToolUse hook to %s (advisory-only → helix nudge)", hooksPath)
	return nil
}

// teardownMCP is a best-effort no-op for Codex: there is no prior helix MCP entry
// to remove (codex was never an MCP-registered client). It mirrors the
// teardown-only contract (never touches hooks) and never errors on absence.
func (r *CodexRegistrar) teardownMCP(cfg RegistrationConfig) error {
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove prior helix MCP entry (codex: none today — no-op)")
	}
	return nil
}

// Unregister strips the Helix block from AGENTS.md (remove-between-sentinels) and
// removes the Helix-managed hooks.json, best-effort. A missing file is a no-op.
func (r *CodexRegistrar) Unregister(cfg RegistrationConfig) error {
	agentsPath := codexAgentsPath(cfg.ProjectDir, cfg.Global)
	hooksPath := codexHooksPath(cfg.ProjectDir, cfg.Global)

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove Helix block from %s", agentsPath)
		cfg.Printer.DryRunAction("would remove Codex hooks.json %s", hooksPath)
		return nil
	}

	if err := removeAgentInstructions(agentsPath); err != nil {
		cfg.Printer.Failure("removing Helix block from %s failed: %s", agentsPath, err)
	} else {
		cfg.Printer.Success("removed Helix block from %s", agentsPath)
	}

	// The hooks.json is entirely Helix-managed (setup created it), so removal is a
	// best-effort file delete; a missing file is a no-op.
	if err := os.Remove(hooksPath); err != nil && !os.IsNotExist(err) {
		cfg.Printer.Failure("removing %s failed: %s", hooksPath, err)
	} else {
		cfg.Printer.Success("removed Codex hook %s", hooksPath)
	}
	return nil
}
