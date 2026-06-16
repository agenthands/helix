package profile

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/skill"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadEmbedded_ReturnsAllProfilesAndModes(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err)

	// 6 original profiles + 4 bench ablation profiles (Phase 76-02) = 10.
	assert.Len(t, store.ProfileNames(), 10, "expected 10 profiles")
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
	assert.Equal(t, []string{
		"baseline",
		"bench-full",
		"bench-no-lsp",
		"bench-no-semantic",
		"bench-no-structured-edit",
		"ci-bot",
		"claude-code",
		"codex",
		"full",
		"ide-assistant",
	}, names)
}

// TestLoadEmbeddedValidatesModes asserts LoadEmbedded() succeeds with all 10
// profiles — i.e. every default_mode and transition key across all profiles
// resolves to a loaded mode (the accept path of ProfileStore.Validate, ABLATE-02).
func TestLoadEmbeddedValidatesModes(t *testing.T) {
	store, err := LoadEmbedded()
	require.NoError(t, err, "all embedded profiles must reference only loaded modes")
	require.NotNil(t, store)
}

// TestLoaderRejectsUnknownMode asserts ProfileStore.Validate() fail-closes on a
// profile whose default_mode references an unknown mode name, and on an unknown
// mode in an allowed_mode_transitions key/value. Mitigates T-76-03 (Tampering:
// a malformed mode name silently no-ops the profile filter).
//
// Built on a hand-assembled store (not a bad embedded YAML) so it does not
// regress LoadEmbedded globally (Pitfall 3 analog).
func TestLoaderRejectsUnknownMode(t *testing.T) {
	t.Run("unknown default_mode", func(t *testing.T) {
		store := NewProfileStore()
		store.SetMode("edit", &Mode{ModeSpec: skill.ModeSpec{Name: "edit"}})
		store.SetProfile("bad", &Profile{
			ContextSpec: skill.ContextSpec{Name: "bad"},
			DefaultMode: "nonsense",
		})

		err := store.Validate()
		require.Error(t, err, "Validate must reject an unknown default_mode")
		assert.Contains(t, err.Error(), "nonsense", "error must name the offending mode")
		assert.Contains(t, err.Error(), "bad", "error must name the offending profile")
	})

	t.Run("unknown transition target", func(t *testing.T) {
		store := NewProfileStore()
		store.SetMode("edit", &Mode{ModeSpec: skill.ModeSpec{Name: "edit"}})
		store.SetProfile("bad", &Profile{
			ContextSpec: skill.ContextSpec{Name: "bad"},
			DefaultMode: "edit",
			AllowedModeTransitions: map[string][]string{
				"edit": {"ghost"},
			},
		})

		err := store.Validate()
		require.Error(t, err, "Validate must reject an unknown transition target mode")
		assert.Contains(t, err.Error(), "ghost", "error must name the offending mode")
	})

	t.Run("unknown transition source", func(t *testing.T) {
		store := NewProfileStore()
		store.SetMode("edit", &Mode{ModeSpec: skill.ModeSpec{Name: "edit"}})
		store.SetProfile("bad", &Profile{
			ContextSpec: skill.ContextSpec{Name: "bad"},
			DefaultMode: "edit",
			AllowedModeTransitions: map[string][]string{
				"phantom": {"edit"},
			},
		})

		err := store.Validate()
		require.Error(t, err, "Validate must reject an unknown transition source mode")
		assert.Contains(t, err.Error(), "phantom", "error must name the offending mode")
	})

	t.Run("accepts a fully-resolvable store", func(t *testing.T) {
		store := NewProfileStore()
		store.SetMode("read", &Mode{ModeSpec: skill.ModeSpec{Name: "read"}})
		store.SetMode("edit", &Mode{ModeSpec: skill.ModeSpec{Name: "edit"}})
		store.SetProfile("good", &Profile{
			ContextSpec: skill.ContextSpec{Name: "good"},
			DefaultMode: "edit",
			AllowedModeTransitions: map[string][]string{
				"read": {"edit"},
				"edit": {"read"},
			},
		})

		assert.NoError(t, store.Validate(), "Validate must accept a store whose modes all resolve")
	})
}
