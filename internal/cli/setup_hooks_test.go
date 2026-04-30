package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMergeHooksIntoSettings_NewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude", "settings.json")

	err := mergeHooksIntoSettings(path, "/usr/local/bin/helix")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	hooks, ok := result["hooks"].(map[string]any)
	require.True(t, ok, "hooks key should exist")

	for _, eventType := range []string{"SessionStart", "PreToolUse", "Stop"} {
		arr, ok := hooks[eventType].([]any)
		require.True(t, ok, "%s should be an array", eventType)
		require.NotEmpty(t, arr, "%s should have entries", eventType)

		// Check that the entry has helix_managed marker.
		matcher := arr[0].(map[string]any)
		hooksArr := matcher["hooks"].([]any)
		hookCmd := hooksArr[0].(map[string]any)
		assert.Equal(t, true, hookCmd["helix_managed"], "%s hook should have helix_managed", eventType)
	}
}

func TestMergeHooksIntoSettings_ExistingHooks(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Write settings.json with existing user hooks under PreToolUse.
	initial := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Lint",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "my-linter check",
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	err := mergeHooksIntoSettings(path, "/usr/local/bin/helix")
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	hooks := result["hooks"].(map[string]any)
	preToolUse := hooks["PreToolUse"].([]any)

	// Should have both user hook and Helix hook.
	assert.Len(t, preToolUse, 2, "should have user hook + Helix hook")

	// First entry should be the user's linter hook (preserved).
	userMatcher := preToolUse[0].(map[string]any)
	assert.Equal(t, "Lint", userMatcher["matcher"])

	// Second entry should be the Helix hook.
	helixMatcher := preToolUse[1].(map[string]any)
	assert.Equal(t, "Grep|Read|Bash", helixMatcher["matcher"])
}

func TestMergeHooksIntoSettings_Idempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Merge twice with same binary path.
	err := mergeHooksIntoSettings(path, "/usr/local/bin/helix")
	require.NoError(t, err)

	err = mergeHooksIntoSettings(path, "/usr/local/bin/helix")
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	hooks := result["hooks"].(map[string]any)

	// Each event type should have exactly one Helix entry (not duplicated).
	for _, eventType := range []string{"SessionStart", "PreToolUse", "Stop"} {
		arr := hooks[eventType].([]any)
		assert.Len(t, arr, 1, "%s should have exactly one entry after double merge", eventType)
	}
}

func TestRemoveHooksFromSettings_RemovesHelixOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings.json")

	// Write settings with both Helix and user hooks.
	initial := map[string]any{
		"hooks": map[string]any{
			"PreToolUse": []any{
				map[string]any{
					"matcher": "Lint",
					"hooks": []any{
						map[string]any{
							"type":    "command",
							"command": "my-linter check",
						},
					},
				},
				map[string]any{
					"matcher": "Grep|Read|Bash",
					"hooks": []any{
						map[string]any{
							"type":           "command",
							"command":        "/usr/local/bin/helix nudge",
							"helix_managed": true,
						},
					},
				},
			},
			"SessionStart": []any{
				map[string]any{
					"matcher": "startup",
					"hooks": []any{
						map[string]any{
							"type":           "command",
							"command":        "/usr/local/bin/helix activate",
							"helix_managed": true,
						},
					},
				},
			},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	err := removeHooksFromSettings(path)
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	hooks := result["hooks"].(map[string]any)

	// User hook should remain under PreToolUse.
	preToolUse := hooks["PreToolUse"].([]any)
	assert.Len(t, preToolUse, 1, "user hook should remain")
	userMatcher := preToolUse[0].(map[string]any)
	assert.Equal(t, "Lint", userMatcher["matcher"])

	// SessionStart had only Helix entries, so it should be removed entirely.
	_, hasSessionStart := hooks["SessionStart"]
	assert.False(t, hasSessionStart, "SessionStart should be removed (no user hooks)")
}

func TestRemoveHooksFromSettings_MissingFile(t *testing.T) {
	err := removeHooksFromSettings("/nonexistent/path/settings.json")
	assert.NoError(t, err, "removing from nonexistent file should not error")
}

func TestHelixHookConfig_Structure(t *testing.T) {
	config := helixHookConfig("/usr/local/bin/helix")

	// SessionStart
	sessionStart := config["SessionStart"]
	require.Len(t, sessionStart, 1)
	ss := sessionStart[0].(map[string]any)
	assert.Equal(t, "startup", ss["matcher"])
	ssHooks := ss["hooks"].([]any)
	ssCmd := ssHooks[0].(map[string]any)
	assert.Contains(t, ssCmd["command"], "/usr/local/bin/helix")
	assert.Contains(t, ssCmd["command"], "activate")
	assert.Equal(t, 30, ssCmd["timeout"])
	assert.Equal(t, true, ssCmd["helix_managed"])

	// PreToolUse
	preToolUse := config["PreToolUse"]
	require.Len(t, preToolUse, 1)
	ptu := preToolUse[0].(map[string]any)
	assert.Equal(t, "Grep|Read|Bash", ptu["matcher"])
	ptuHooks := ptu["hooks"].([]any)
	ptuCmd := ptuHooks[0].(map[string]any)
	assert.Contains(t, ptuCmd["command"], "/usr/local/bin/helix")
	assert.Contains(t, ptuCmd["command"], "nudge")
	assert.Equal(t, 5, ptuCmd["timeout"])

	// Stop
	stop := config["Stop"]
	require.Len(t, stop, 1)
	st := stop[0].(map[string]any)
	_, hasMatcher := st["matcher"]
	assert.False(t, hasMatcher, "Stop should not have a matcher")
	stHooks := st["hooks"].([]any)
	stCmd := stHooks[0].(map[string]any)
	assert.Contains(t, stCmd["command"], "/usr/local/bin/helix")
	assert.Contains(t, stCmd["command"], "deactivate")
	assert.Equal(t, 10, stCmd["timeout"])
}

func TestHookSettingsPath_ProjectScoped(t *testing.T) {
	path := hookSettingsPath("/my/project", false)
	assert.Equal(t, "/my/project/.claude/settings.json", path)
}

func TestHookSettingsPath_Global(t *testing.T) {
	path := hookSettingsPath("/my/project", true)
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(home, ".claude", "settings.json"), path)
}
