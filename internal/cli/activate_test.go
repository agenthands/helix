package cli

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewActivateCommand_Structure(t *testing.T) {
	cmd := newActivateCommand()

	assert.Equal(t, "activate", cmd.Use)
	assert.NotNil(t, cmd.RunE)
	assert.True(t, cmd.SilenceUsage)
	assert.True(t, cmd.SilenceErrors)
}

func TestNewActivateCommand_WorkspaceFlag(t *testing.T) {
	cmd := newActivateCommand()

	wsFlag := cmd.Flags().Lookup("workspace")
	assert.NotNil(t, wsFlag, "workspace flag should exist")
	assert.Equal(t, "", wsFlag.DefValue, "workspace flag should default to empty string")
}
