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
	}
}

// --- Shared helpers ---

// serverConfigJSON returns the standard MCP server config for helix.
func serverConfigJSON(binaryPath string) map[string]any {
	return map[string]any{
		"command": binaryPath,
		"args":    []string{"--mode=stdio"},
	}
}

// mergeJSONConfig reads an existing JSON config file, merges a server entry under the
// given key, and writes back. Creates parent directories and the file if they don't exist.
// Uses encoding/json Marshal/Unmarshal (never string concatenation) per T-34-01.
func mergeJSONConfig(path, key, serverName string, serverConfig map[string]any) error {
	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parsing existing config %s: %w", path, err)
		}
	}

	// Get or create nested map at key
	servers, ok := existing[key].(map[string]any)
	if !ok {
		servers = make(map[string]any)
	}
	servers[serverName] = serverConfig
	existing[key] = servers

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Create parent directories with 0755 per T-34-01
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	// Write file with 0644 per T-34-03
	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing config %s: %w", path, err)
	}
	return nil
}

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

func (r *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
	serverJSON, err := json.Marshal(serverConfigJSON(cfg.BinaryPath))
	if err != nil {
		return fmt.Errorf("marshaling server config: %w", err)
	}

	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	addArgs := []string{"mcp", "add-json", "helix", string(serverJSON), "--scope", scope}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: claude %s", strings.Join(addArgs, " "))
		return nil
	}

	if _, err := exec.LookPath("claude"); err != nil {
		// No claude CLI — fall back to direct .mcp.json write
		cfg.Printer.Info("claude CLI not found; writing directly to .mcp.json")
		configPath := filepath.Join(cfg.ProjectDir, ".mcp.json")
		if cfg.Global {
			home, homeErr := os.UserHomeDir()
			if homeErr != nil {
				return fmt.Errorf("cannot determine home directory: %w", homeErr)
			}
			configPath = filepath.Join(home, ".claude", "settings.json")
		}
		if !cfg.Global {
			return mergeJSONConfig(configPath, "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath))
		}
		return fmt.Errorf("claude CLI not found in PATH; install Claude Code first or add manually")
	}

	cmd := exec.Command("claude", addArgs...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		// If "already exists", remove then re-add to update the config
		if strings.Contains(string(output), "already exists") {
			cfg.Printer.Info("helix already registered — updating")
			rmCmd := exec.Command("claude", "mcp", "remove", "helix", "--scope", scope)
			rmCmd.Stdout = os.Stderr
			rmCmd.Stderr = os.Stderr
			if rmErr := rmCmd.Run(); rmErr != nil {
				// Remove failed — fall back to direct file write
				cfg.Printer.Info("could not remove via CLI; writing directly to .mcp.json")
				return mergeJSONConfig(filepath.Join(cfg.ProjectDir, ".mcp.json"), "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath))
			}
			// Retry add after remove
			retryCmd := exec.Command("claude", addArgs...)
			retryCmd.Stdout = os.Stderr
			retryCmd.Stderr = os.Stderr
			if retryErr := retryCmd.Run(); retryErr != nil {
				// CLI still failing — fall back to direct file write
				cfg.Printer.Info("CLI retry failed; writing directly to .mcp.json")
				return mergeJSONConfig(filepath.Join(cfg.ProjectDir, ".mcp.json"), "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath))
			}
		} else {
			// Non-duplicate error — fall back to direct file write
			cfg.Printer.Info("claude CLI failed (%s); writing directly to .mcp.json", strings.TrimSpace(string(output)))
			return mergeJSONConfig(filepath.Join(cfg.ProjectDir, ".mcp.json"), "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath))
		}
	}

	// Hook installation (per D-01, D-16).
	if !cfg.NoHooks {
		settingsPath := hookSettingsPath(cfg.ProjectDir, cfg.Global)
		if cfg.DryRun {
			cfg.Printer.DryRunAction("would write hooks to %s", settingsPath)
		} else {
			if err := mergeHooksIntoSettings(settingsPath, cfg.BinaryPath); err != nil {
				cfg.Printer.Failure("hook installation failed: %s", err)
				cfg.Printer.Info("MCP registration succeeded; hooks can be installed manually")
				return nil // Non-fatal per D-16.
			}
			cfg.Printer.Success("installed hooks (SessionStart, PreToolUse, Stop)")
		}
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
		cfg.Printer.DryRunAction("would run: claude %s", strings.Join(cmdArgs, " "))
		return nil
	}

	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH; install Claude Code first")
	}

	cmd := exec.Command("claude", cmdArgs...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude mcp remove failed: %w", err)
	}

	// Hook removal (per D-03).
	settingsPath := hookSettingsPath(cfg.ProjectDir, cfg.Global)
	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove hooks from %s", settingsPath)
	} else {
		if err := removeHooksFromSettings(settingsPath); err != nil {
			cfg.Printer.Failure("hook removal failed: %s", err)
		} else {
			cfg.Printer.Success("removed hooks from %s", settingsPath)
		}
	}

	return nil
}

// --- GeminiCLIRegistrar ---

// GeminiCLIRegistrar handles MCP registration for Gemini CLI.
type GeminiCLIRegistrar struct{}

func (r *GeminiCLIRegistrar) Name() string        { return "gemini-cli" }
func (r *GeminiCLIRegistrar) Description() string { return "Gemini CLI (Google)" }

func (r *GeminiCLIRegistrar) Register(cfg RegistrationConfig) error {
	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "add", "--scope", scope, "-t", "stdio", "helix", cfg.BinaryPath, "--", "--mode=stdio"}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: gemini %s", strings.Join(cmdArgs, " "))
		cfg.Printer.DryRunAction("would enable helix in mcp-server-enablement.json")
		return nil
	}

	// Try CLI first
	cliDone := false
	if _, err := exec.LookPath("gemini"); err == nil {
		cmd := exec.Command("gemini", cmdArgs...)
		output, runErr := cmd.CombinedOutput()
		if runErr == nil {
			cliDone = true
		} else {
			// CLI failed — log and fall through to direct file write
			cfg.Printer.Info("gemini CLI failed (%s); writing directly to settings file", strings.TrimSpace(string(output)))
		}
	} else {
		cfg.Printer.Info("gemini CLI not found; writing directly to settings file")
	}

	if !cliDone {
		// Fall back to direct file write for MCP server config
		configPath, err := r.settingsPath(cfg)
		if err != nil {
			return err
		}
		if err := mergeJSONConfig(configPath, "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath)); err != nil {
			return err
		}
	}

	// Always ensure helix is enabled in the enablement file.
	// Gemini CLI uses a separate mcp-server-enablement.json to gate which servers are active.
	// Without this, the server is registered but invisible.
	if err := r.ensureEnabled(cfg); err != nil {
		cfg.Printer.Failure("could not enable helix in mcp-server-enablement.json: %s", err)
		cfg.Printer.Info("MCP registration succeeded; enable manually in ~/.gemini/mcp-server-enablement.json")
	}

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

// ensureEnabled writes {"helix": {"enabled": true}} into Gemini's mcp-server-enablement.json.
func (r *GeminiCLIRegistrar) ensureEnabled(cfg RegistrationConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("cannot determine home directory: %w", err)
	}
	enablementPath := filepath.Join(home, ".gemini", "mcp-server-enablement.json")

	existing := make(map[string]any)
	if data, readErr := os.ReadFile(enablementPath); readErr == nil {
		_ = json.Unmarshal(data, &existing)
	}

	existing["helix"] = map[string]any{"enabled": true}

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling enablement config: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(enablementPath), 0755); err != nil {
		return fmt.Errorf("creating directory: %w", err)
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

func (r *VSCodeRegistrar) Register(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}

	serverEntry := map[string]any{
		"type":    "stdio",
		"command": cfg.BinaryPath,
		"args":    []string{"--mode=stdio"},
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: servers.helix = %v", configPath, serverEntry)
		return nil
	}

	return mergeJSONConfig(configPath, "servers", "helix", serverEntry)
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

func (r *JetBrainsRegistrar) Register(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: mcpServers.helix", configPath)
		return nil
	}

	return mergeJSONConfig(configPath, "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath))
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

func (r *ClaudeDesktopRegistrar) Register(cfg RegistrationConfig) error {
	configPath, err := r.configPath()
	if err != nil {
		return err
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: mcpServers.helix", configPath)
		return nil
	}

	return mergeJSONConfig(configPath, "mcpServers", "helix", serverConfigJSON(cfg.BinaryPath))
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

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove helix from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "mcpServers", "helix")
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

func (r *OpenCodeRegistrar) Register(cfg RegistrationConfig) error {
	configPath, err := r.configPath(cfg)
	if err != nil {
		return err
	}

	// OpenCode uses a different server config shape: "command" is an array, plus "type" and "enabled" fields.
	serverEntry := map[string]any{
		"type":    "local",
		"command": []string{cfg.BinaryPath, "--mode=stdio"},
		"enabled": true,
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: mcp.helix = %v", configPath, serverEntry)
		return nil
	}

	return mergeJSONConfig(configPath, "mcp", "helix", serverEntry)
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

func (r *GenericRegistrar) Register(cfg RegistrationConfig) error {
	fullConfig := map[string]any{
		"mcpServers": map[string]any{
			"helix": serverConfigJSON(cfg.BinaryPath),
		},
	}

	data, err := json.MarshalIndent(fullConfig, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	if cfg.OutputPath != "" {
		if cfg.DryRun {
			cfg.Printer.DryRunAction("would write MCP config to %s", cfg.OutputPath)
			return nil
		}
		if err := os.MkdirAll(filepath.Dir(cfg.OutputPath), 0755); err != nil {
			return fmt.Errorf("creating output directory: %w", err)
		}
		if err := os.WriteFile(cfg.OutputPath, append(data, '\n'), 0644); err != nil {
			return fmt.Errorf("writing config to %s: %w", cfg.OutputPath, err)
		}
		cfg.Printer.Success("MCP config written to %s", cfg.OutputPath)
		return nil
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would print MCP config JSON to stdout")
		return nil
	}

	// Print to stdout -- this is the only registrar that writes to stdout
	fmt.Fprintf(os.Stdout, "%s\n", data)
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
