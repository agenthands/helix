// Package agent wraps the Claude Code CLI subprocess (and the scripted-agent
// shim for eval-quick) used by the Phase 67 evaluation harness. It builds the
// correct argv (--bare --strict-mcp-config --output-format=stream-json, etc.),
// starts the process, captures stdout/stderr, and feeds the output to the trace
// collector. Real implementation lands in Wave 1+.
package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// MCPServerConfig describes a single MCP server entry in the Claude Code
// --mcp-config JSON file. The shape is the CC-documented stdio server record.
type MCPServerConfig struct {
	Type    string            `json:"type"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}

// MCPConfig is the top-level structure written to the --mcp-config JSON file.
type MCPConfig struct {
	MCPServers map[string]MCPServerConfig `json:"mcpServers"`
}

// WriteMCPConfig writes a Claude Code MCP config JSON to path. helixAbsBin
// must be an absolute path (T-67-03 mitigation: never depend on PATH lookup
// at agent invocation time). The file is written with mode 0600.
func WriteMCPConfig(path, helixAbsBin, sockPath, homePath string) error {
	if !filepath.IsAbs(helixAbsBin) {
		return fmt.Errorf("agent: helixBin %q must be absolute (T-67-03 mitigation)", helixAbsBin)
	}

	cfg := MCPConfig{
		MCPServers: map[string]MCPServerConfig{
			"helix": {
				Type:    "stdio",
				Command: helixAbsBin,
				Args:    []string{"--socket", sockPath},
				Env:     map[string]string{"HOME": homePath},
			},
		},
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("agent: marshal mcp config: %w", err)
	}

	return os.WriteFile(path, data, 0600)
}
