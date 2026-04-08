//go:build integration

package integration_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWorkflow_Onboard(t *testing.T) {
	fixture := PrepareFixture(t, "go")
	td := StartTestDaemon(t, Options{WorkspaceDir: fixture, SkipLS: true})

	t.Run("onboard_project", func(t *testing.T) {
		result := callTool(t, td.Session, "onboard_project", map[string]any{})
		text := textContent(result)
		assert.NotEmpty(t, text, "onboard_project should return project info")
	})

	t.Run("prepare_for_new_conversation", func(t *testing.T) {
		result := callTool(t, td.Session, "prepare_for_new_conversation", map[string]any{})
		text := textContent(result)
		assert.NotEmpty(t, text, "prepare_for_new_conversation should return a handoff summary")
	})
}
