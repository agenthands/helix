package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWriteMCPConfig verifies the JSON shape and file mode of the written config.
func TestWriteMCPConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-config.json")

	helixBin := "/usr/local/bin/helix"
	sockPath := "/tmp/helix-eval-test/t1/baseline/daemon.sock"
	homePath := "/tmp/helix-eval-test/t1/baseline/home"

	require.NoError(t, WriteMCPConfig(path, helixBin, sockPath, homePath))

	// Read and parse JSON.
	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg MCPConfig
	require.NoError(t, json.Unmarshal(data, &cfg))

	helix, ok := cfg.MCPServers["helix"]
	require.True(t, ok, "mcpServers.helix must exist")
	assert.Equal(t, "stdio", helix.Type)
	assert.Equal(t, helixBin, helix.Command)
	assert.Equal(t, []string{"--socket", sockPath}, helix.Args)
	assert.Equal(t, map[string]string{"HOME": homePath}, helix.Env)

	// File mode must be 0600.
	info, err := os.Stat(path)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm())
}

// TestMCPConfigRejectsRelativeBin verifies that a relative helixBin path is rejected.
func TestMCPConfigRejectsRelativeBin(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp-config.json")

	err := WriteMCPConfig(path, "helix", "/tmp/sock", "/tmp/home")
	assert.Error(t, err, "relative helixBin should be rejected (T-67-03 mitigation)")
}
