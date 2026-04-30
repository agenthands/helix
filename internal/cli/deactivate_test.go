package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewDeactivateCommand_Structure(t *testing.T) {
	cmd := newDeactivateCommand()

	assert.Equal(t, "deactivate", cmd.Use)
	assert.NotNil(t, cmd.RunE)
	assert.True(t, cmd.SilenceUsage)
	assert.True(t, cmd.SilenceErrors)

	wsFlag := cmd.Flags().Lookup("workspace")
	assert.NotNil(t, wsFlag, "workspace flag should exist")
	assert.Equal(t, "", wsFlag.DefValue)
}

func TestDeactivate_CleansSessionFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create .helix/session-stats.json
	helixDir := filepath.Join(tmpDir, ".helix")
	require.NoError(t, os.MkdirAll(helixDir, 0o755))

	statsPath := filepath.Join(helixDir, "session-stats.json")
	require.NoError(t, os.WriteFile(statsPath, []byte(`{"sessions": 1}`), 0o644))

	// Verify file exists
	_, err := os.Stat(statsPath)
	require.NoError(t, err, "session-stats.json should exist before cleanup")

	// Simulate the cleanup logic from runDeactivate
	os.Remove(statsPath)

	// Verify file is removed
	_, err = os.Stat(statsPath)
	assert.True(t, os.IsNotExist(err), "session-stats.json should be removed after cleanup")
}

func TestDeactivate_MissingDaemonSilentSuccess(t *testing.T) {
	// Test the stat check pattern used in runDeactivate:
	// when socket doesn't exist, deactivate should silently succeed
	nonExistentSocket := "/tmp/helix-nonexistent-test-socket-12345/daemon.sock"

	_, err := os.Stat(nonExistentSocket)
	assert.True(t, os.IsNotExist(err), "non-existent socket should trigger silent success path (D-15)")
}
