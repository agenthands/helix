package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEmbedded_ReturnsAllProfilesAndModes(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	assert.Len(t, store.ProfileNames(), 6, "expected 6 profiles")
	assert.Len(t, store.ModeNames(), 4, "expected 4 modes")
}

func TestProfileStore_ClaudeCodeExcludeTools(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	p, ok := store.Profile("claude-code")
	require.True(t, ok, "claude-code profile should exist")
	assert.Contains(t, p.ExcludeTools, "read_file")
	assert.Contains(t, p.ExcludeTools, "create_text_file")
	assert.Contains(t, p.ExcludeTools, "replace_content")
	assert.Equal(t, "edit", p.DefaultMode)
	assert.True(t, p.SingleProject)
}

func TestProfileStore_EditModeHasPrompt(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	m, ok := store.Mode("edit")
	require.True(t, ok, "edit mode should exist")
	assert.NotEmpty(t, m.Prompt, "edit mode should have a non-empty prompt")
}

func TestProfileStore_DefaultProfile(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	p := store.DefaultProfile()
	require.NotNil(t, p, "DefaultProfile should return the full profile")
	assert.Equal(t, "full", p.Name)
}

func TestLoadOverrides_MergesOnTop(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	// Create temp override directory with a partial profile override.
	tmpDir := t.TempDir()
	profileDir := filepath.Join(tmpDir, "profiles")
	modeDir := filepath.Join(tmpDir, "modes")
	require.NoError(t, os.MkdirAll(profileDir, 0o755))
	require.NoError(t, os.MkdirAll(modeDir, 0o755))

	// Override claude-code: change default_mode and add a tool_description_override.
	override := `name: claude-code
default_mode: review
tool_description_overrides:
  find_symbol: "Overridden description"
  new_tool: "Brand new override"
exclude_tools:
  - only_this_one
`
	require.NoError(t, os.WriteFile(filepath.Join(profileDir, "claude-code.yaml"), []byte(override), 0o644))

	err = LoadOverrides(store, profileDir, modeDir)
	require.NoError(t, err)

	p, ok := store.Profile("claude-code")
	require.True(t, ok)

	// Overridden fields should reflect the override.
	assert.Equal(t, "review", p.DefaultMode, "default_mode should be overridden")
	assert.Equal(t, "Overridden description", p.ToolDescriptionOverrides["find_symbol"])
	assert.Equal(t, "Brand new override", p.ToolDescriptionOverrides["new_tool"])

	// exclude_tools should be replaced, not merged.
	assert.Equal(t, []string{"only_this_one"}, p.ExcludeTools)
}

func TestProfileStore_ProfileNames_Sorted(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	names := store.ProfileNames()
	assert.Equal(t, []string{"baseline", "ci-bot", "claude-code", "codex", "full", "ide-assistant"}, names)
}
