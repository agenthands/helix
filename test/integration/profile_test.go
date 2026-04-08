//go:build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProfile_ModeAndBudget(t *testing.T) {
	td := StartTestDaemon(t, Options{SkipLS: true})

	t.Run("get_token_budget", func(t *testing.T) {
		result := callTool(t, td.Session, "get_token_budget", map[string]any{})
		text := textContent(result)
		assert.NotEmpty(t, text, "get_token_budget should return budget info")
		// Should contain token count information.
		assert.Contains(t, text, "total_tokens")
	})

	t.Run("switch_mode", func(t *testing.T) {
		result := callTool(t, td.Session, "switch_mode", map[string]any{
			"target_mode": "read",
		})
		text := textContent(result)
		assert.NotEmpty(t, text, "switch_mode should return mode change info")

		// Verify get_token_budget still works after mode switch.
		result = callTool(t, td.Session, "get_token_budget", map[string]any{})
		text = textContent(result)
		assert.NotEmpty(t, text, "get_token_budget should still work after mode switch")
	})
}
