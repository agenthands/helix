package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// hookSettingsPath returns the path to Claude Code's settings.json.
// If global is true, returns ~/.claude/settings.json (user-scoped).
// Otherwise returns <projectDir>/.claude/settings.json (project-scoped).
func hookSettingsPath(projectDir string, global bool) string {
	if global {
		home, err := os.UserHomeDir()
		if err != nil {
			// Fallback to project-scoped if home dir unavailable.
			return filepath.Join(projectDir, ".claude", "settings.json")
		}
		return filepath.Join(home, ".claude", "settings.json")
	}
	return filepath.Join(projectDir, ".claude", "settings.json")
}

// serenaHookConfig returns the hook configuration map for Claude Code.
// Each event type maps to an array of matcher objects containing hook commands.
// All Serena hook commands include "serena_managed": true for idempotent add/remove.
func serenaHookConfig(binaryPath string) map[string][]any {
	return map[string][]any{
		"SessionStart": {
			map[string]any{
				"matcher": "startup",
				"hooks": []any{
					map[string]any{
						"type":           "command",
						"command":        fmt.Sprintf("%s activate --workspace \"$CLAUDE_PROJECT_DIR\"", binaryPath),
						"timeout":        30,
						"statusMessage":  "Activating Serena workspace...",
						"serena_managed": true,
					},
				},
			},
		},
		"PreToolUse": {
			map[string]any{
				"matcher": "Grep|Read|Bash",
				"hooks": []any{
					map[string]any{
						"type":           "command",
						"command":        fmt.Sprintf("%s nudge", binaryPath),
						"timeout":        5,
						"serena_managed": true,
					},
				},
			},
		},
		"Stop": {
			map[string]any{
				"hooks": []any{
					map[string]any{
						"type":           "command",
						"command":        fmt.Sprintf("%s deactivate --workspace \"$CLAUDE_PROJECT_DIR\"", binaryPath),
						"timeout":        10,
						"serena_managed": true,
					},
				},
			},
		},
	}
}

// mergeHooksIntoSettings reads existing settings.json (or creates a new one),
// merges Serena hook entries for each event type, and writes back.
// Existing user hooks are preserved. Previous Serena entries (identified by
// "serena_managed": true) are removed before adding new ones (idempotent).
// Uses encoding/json Marshal (never string concat) per T-36-02.
func mergeHooksIntoSettings(path string, binaryPath string) error {
	existing := make(map[string]any)
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			return fmt.Errorf("parsing existing settings %s: %w", path, err)
		}
	}

	hooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		hooks = make(map[string]any)
	}

	serenaHooks := serenaHookConfig(binaryPath)
	for eventType, newMatchers := range serenaHooks {
		// Get existing matchers for this event type.
		var existingMatchers []any
		if raw, ok := hooks[eventType]; ok {
			if arr, ok := raw.([]any); ok {
				existingMatchers = arr
			}
		}

		// Filter out previous Serena entries.
		filtered := filterOutSerenaEntries(existingMatchers)

		// Append new Serena entries.
		filtered = append(filtered, newMatchers...)
		hooks[eventType] = filtered
	}

	existing["hooks"] = hooks

	data, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating settings directory: %w", err)
	}

	if err := os.WriteFile(path, append(data, '\n'), 0644); err != nil {
		return fmt.Errorf("writing settings %s: %w", path, err)
	}
	return nil
}

// removeHooksFromSettings removes all Serena-managed hook entries from settings.json.
// User hooks are preserved. If the file doesn't exist, returns nil.
// If no matchers remain for an event type, the event type key is removed.
// If the hooks object is empty, it is removed.
func removeHooksFromSettings(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("reading settings %s: %w", path, err)
	}

	existing := make(map[string]any)
	if err := json.Unmarshal(data, &existing); err != nil {
		return fmt.Errorf("parsing settings %s: %w", path, err)
	}

	hooks, ok := existing["hooks"].(map[string]any)
	if !ok {
		return nil // No hooks object, nothing to remove.
	}

	for eventType, raw := range hooks {
		arr, ok := raw.([]any)
		if !ok {
			continue
		}
		filtered := filterOutSerenaEntries(arr)
		if len(filtered) == 0 {
			delete(hooks, eventType)
		} else {
			hooks[eventType] = filtered
		}
	}

	if len(hooks) == 0 {
		delete(existing, "hooks")
	} else {
		existing["hooks"] = hooks
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}

	return os.WriteFile(path, append(out, '\n'), 0644)
}

// filterOutSerenaEntries removes matcher objects where any hook command has
// "serena_managed": true. Returns only non-Serena matcher objects.
func filterOutSerenaEntries(matchers []any) []any {
	var result []any
	for _, m := range matchers {
		matcher, ok := m.(map[string]any)
		if !ok {
			result = append(result, m)
			continue
		}
		if isSerenaManaged(matcher) {
			continue
		}
		result = append(result, m)
	}
	return result
}

// isSerenaManaged checks whether a matcher object contains any hook command
// with the "serena_managed" field set to true.
func isSerenaManaged(matcher map[string]any) bool {
	hooksRaw, ok := matcher["hooks"]
	if !ok {
		return false
	}
	hooksArr, ok := hooksRaw.([]any)
	if !ok {
		return false
	}
	for _, h := range hooksArr {
		hookMap, ok := h.(map[string]any)
		if !ok {
			continue
		}
		if managed, ok := hookMap["serena_managed"].(bool); ok && managed {
			return true
		}
	}
	return false
}
