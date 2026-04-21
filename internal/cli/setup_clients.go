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
	// Register adds Serena as an MCP server for this client.
	Register(cfg RegistrationConfig) error
	// Unregister removes Serena from this client.
	Unregister(cfg RegistrationConfig) error
}

// RegistrationConfig holds common registration parameters.
type RegistrationConfig struct {
	BinaryPath string        // Absolute path to serena binary (resolved via os.Executable + filepath.EvalSymlinks)
	Global     bool          // --global flag
	DryRun     bool          // --dry-run flag
	ProjectDir string        // Current working directory
	OutputPath string        // --output flag (generic client only)
	Printer    *SetupPrinter // Colored output helper
}

// clientRegistry returns all 6 registrars keyed by client name.
func clientRegistry() map[string]ClientRegistrar {
	return map[string]ClientRegistrar{
		"claude-code":    &ClaudeCodeRegistrar{},
		"vscode":         &VSCodeRegistrar{},
		"jetbrains":      &JetBrainsRegistrar{},
		"claude-desktop": &ClaudeDesktopRegistrar{},
		"gemini-cli":     &GeminiCLIRegistrar{},
		"generic":        &GenericRegistrar{},
	}
}

// --- Shared helpers ---

// serverConfigJSON returns the standard MCP server config for serena.
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
func userConfigDir() string {
	dir, _ := os.UserConfigDir()
	return dir
}

// --- ClaudeCodeRegistrar ---

// ClaudeCodeRegistrar handles MCP registration for Claude Code (Anthropic CLI agent).
type ClaudeCodeRegistrar struct{}

func (r *ClaudeCodeRegistrar) Name() string       { return "claude-code" }
func (r *ClaudeCodeRegistrar) Description() string { return "Claude Code (Anthropic CLI agent)" }

func (r *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH; install Claude Code first")
	}

	serverJSON, err := json.Marshal(serverConfigJSON(cfg.BinaryPath))
	if err != nil {
		return fmt.Errorf("marshaling server config: %w", err)
	}

	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "add-json", "serena", string(serverJSON), "--scope", scope}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: claude %s", strings.Join(cmdArgs, " "))
		return nil
	}

	cmd := exec.Command("claude", cmdArgs...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude mcp add-json failed: %w", err)
	}
	return nil
}

func (r *ClaudeCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	if _, err := exec.LookPath("claude"); err != nil {
		return fmt.Errorf("claude CLI not found in PATH; install Claude Code first")
	}

	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "remove", "serena", "--scope", scope}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: claude %s", strings.Join(cmdArgs, " "))
		return nil
	}

	cmd := exec.Command("claude", cmdArgs...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("claude mcp remove failed: %w", err)
	}
	return nil
}

// --- GeminiCLIRegistrar ---

// GeminiCLIRegistrar handles MCP registration for Gemini CLI.
type GeminiCLIRegistrar struct{}

func (r *GeminiCLIRegistrar) Name() string       { return "gemini-cli" }
func (r *GeminiCLIRegistrar) Description() string { return "Gemini CLI (Google)" }

func (r *GeminiCLIRegistrar) Register(cfg RegistrationConfig) error {
	if _, err := exec.LookPath("gemini"); err != nil {
		return fmt.Errorf("gemini CLI not found in PATH; install Gemini CLI first")
	}

	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "add", "--scope", scope, "-t", "stdio", "serena", cfg.BinaryPath, "--", "--mode=stdio"}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: gemini %s", strings.Join(cmdArgs, " "))
		return nil
	}

	cmd := exec.Command("gemini", cmdArgs...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("gemini mcp add failed: %w", err)
	}
	return nil
}

func (r *GeminiCLIRegistrar) Unregister(cfg RegistrationConfig) error {
	if _, err := exec.LookPath("gemini"); err != nil {
		return fmt.Errorf("gemini CLI not found in PATH; install Gemini CLI first")
	}

	scope := "project"
	if cfg.Global {
		scope = "user"
	}

	cmdArgs := []string{"mcp", "remove", "serena", "--scope", scope}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would run: gemini %s", strings.Join(cmdArgs, " "))
		return nil
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

func (r *VSCodeRegistrar) Name() string       { return "vscode" }
func (r *VSCodeRegistrar) Description() string { return "VS Code / Copilot (Microsoft)" }

func (r *VSCodeRegistrar) Register(cfg RegistrationConfig) error {
	configPath := r.configPath(cfg)

	serverEntry := map[string]any{
		"type":    "stdio",
		"command": cfg.BinaryPath,
		"args":    []string{"--mode=stdio"},
	}

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: servers.serena = %v", configPath, serverEntry)
		return nil
	}

	return mergeJSONConfig(configPath, "servers", "serena", serverEntry)
}

func (r *VSCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath := r.configPath(cfg)

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove serena from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "servers", "serena")
}

func (r *VSCodeRegistrar) configPath(cfg RegistrationConfig) string {
	if cfg.Global {
		return filepath.Join(userConfigDir(), "Code", "User", "mcp.json")
	}
	return filepath.Join(cfg.ProjectDir, ".vscode", "mcp.json")
}

// --- JetBrainsRegistrar ---

// JetBrainsRegistrar handles MCP registration for JetBrains IDEs via Junie.
type JetBrainsRegistrar struct{}

func (r *JetBrainsRegistrar) Name() string       { return "jetbrains" }
func (r *JetBrainsRegistrar) Description() string { return "JetBrains IDEs via Junie" }

func (r *JetBrainsRegistrar) Register(cfg RegistrationConfig) error {
	configPath := r.configPath(cfg)

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: mcpServers.serena", configPath)
		return nil
	}

	return mergeJSONConfig(configPath, "mcpServers", "serena", serverConfigJSON(cfg.BinaryPath))
}

func (r *JetBrainsRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath := r.configPath(cfg)

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove serena from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "mcpServers", "serena")
}

func (r *JetBrainsRegistrar) configPath(cfg RegistrationConfig) string {
	if cfg.Global {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".junie", "mcp", "mcp.json")
	}
	return filepath.Join(cfg.ProjectDir, ".junie", "mcp", "mcp.json")
}

// --- ClaudeDesktopRegistrar ---

// ClaudeDesktopRegistrar handles MCP registration for Claude Desktop app.
// Claude Desktop is global-only (the Global flag is ignored).
type ClaudeDesktopRegistrar struct{}

func (r *ClaudeDesktopRegistrar) Name() string       { return "claude-desktop" }
func (r *ClaudeDesktopRegistrar) Description() string { return "Claude Desktop app" }

func (r *ClaudeDesktopRegistrar) Register(cfg RegistrationConfig) error {
	configPath := r.configPath()

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would write to %s: mcpServers.serena", configPath)
		return nil
	}

	return mergeJSONConfig(configPath, "mcpServers", "serena", serverConfigJSON(cfg.BinaryPath))
}

func (r *ClaudeDesktopRegistrar) Unregister(cfg RegistrationConfig) error {
	configPath := r.configPath()

	if cfg.DryRun {
		cfg.Printer.DryRunAction("would remove serena from %s", configPath)
		return nil
	}

	return removeFromJSONConfig(configPath, "mcpServers", "serena")
}

// configPath returns the platform-specific Claude Desktop config path.
// Claude Desktop is always global -- there is no project-scoped config.
func (r *ClaudeDesktopRegistrar) configPath() string {
	switch runtime.GOOS {
	case "darwin":
		home, _ := os.UserHomeDir()
		return filepath.Join(home, "Library", "Application Support", "Claude", "claude_desktop_config.json")
	case "windows":
		return filepath.Join(os.Getenv("APPDATA"), "Claude", "claude_desktop_config.json")
	default: // linux and others
		home, _ := os.UserHomeDir()
		return filepath.Join(home, ".config", "Claude", "claude_desktop_config.json")
	}
}

// --- GenericRegistrar ---

// GenericRegistrar handles MCP registration for any stdio-based MCP client.
// Outputs JSON to stdout (default) or writes to a file via --output.
type GenericRegistrar struct{}

func (r *GenericRegistrar) Name() string       { return "generic" }
func (r *GenericRegistrar) Description() string { return "Generic MCP stdio config (any client)" }

func (r *GenericRegistrar) Register(cfg RegistrationConfig) error {
	fullConfig := map[string]any{
		"mcpServers": map[string]any{
			"serena": serverConfigJSON(cfg.BinaryPath),
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

func (r *GenericRegistrar) Unregister(cfg RegistrationConfig) error {
	if cfg.OutputPath != "" {
		if cfg.DryRun {
			cfg.Printer.DryRunAction("would remove serena from %s", cfg.OutputPath)
			return nil
		}
		return removeFromJSONConfig(cfg.OutputPath, "mcpServers", "serena")
	}
	cfg.Printer.Info("generic config was printed to stdout and cannot be unregistered automatically")
	return nil
}
