package cli

import (
	"fmt"
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

// ClaudeCodeRegistrar handles MCP registration for Claude Code (Anthropic CLI agent).
type ClaudeCodeRegistrar struct{}

func (r *ClaudeCodeRegistrar) Name() string        { return "claude-code" }
func (r *ClaudeCodeRegistrar) Description() string  { return "Claude Code (Anthropic CLI agent)" }
func (r *ClaudeCodeRegistrar) Register(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
func (r *ClaudeCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}

// VSCodeRegistrar handles MCP registration for VS Code / Copilot.
type VSCodeRegistrar struct{}

func (r *VSCodeRegistrar) Name() string        { return "vscode" }
func (r *VSCodeRegistrar) Description() string  { return "VS Code / Copilot (Microsoft)" }
func (r *VSCodeRegistrar) Register(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
func (r *VSCodeRegistrar) Unregister(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}

// JetBrainsRegistrar handles MCP registration for JetBrains IDEs via Junie.
type JetBrainsRegistrar struct{}

func (r *JetBrainsRegistrar) Name() string        { return "jetbrains" }
func (r *JetBrainsRegistrar) Description() string  { return "JetBrains IDEs via Junie" }
func (r *JetBrainsRegistrar) Register(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
func (r *JetBrainsRegistrar) Unregister(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}

// ClaudeDesktopRegistrar handles MCP registration for Claude Desktop app.
type ClaudeDesktopRegistrar struct{}

func (r *ClaudeDesktopRegistrar) Name() string        { return "claude-desktop" }
func (r *ClaudeDesktopRegistrar) Description() string  { return "Claude Desktop app" }
func (r *ClaudeDesktopRegistrar) Register(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
func (r *ClaudeDesktopRegistrar) Unregister(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}

// GeminiCLIRegistrar handles MCP registration for Gemini CLI.
type GeminiCLIRegistrar struct{}

func (r *GeminiCLIRegistrar) Name() string        { return "gemini-cli" }
func (r *GeminiCLIRegistrar) Description() string  { return "Gemini CLI (Google)" }
func (r *GeminiCLIRegistrar) Register(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
func (r *GeminiCLIRegistrar) Unregister(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}

// GenericRegistrar handles MCP registration for any stdio-based MCP client.
type GenericRegistrar struct{}

func (r *GenericRegistrar) Name() string        { return "generic" }
func (r *GenericRegistrar) Description() string  { return "Generic MCP stdio config (any client)" }
func (r *GenericRegistrar) Register(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
func (r *GenericRegistrar) Unregister(cfg RegistrationConfig) error {
	return fmt.Errorf("not implemented")
}
