package skill

import (
	"fmt"
	"log/slog"
	"testing"

	"github.com/postfix/serena/internal/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockSkill is a basic Skill for testing.
type mockSkill struct {
	name    string
	desc    string
	initErr error
	inited  bool
}

func (m *mockSkill) Name() string        { return m.name }
func (m *mockSkill) Description() string  { return m.desc }
func (m *mockSkill) Init(deps SkillDeps) error {
	if m.initErr != nil {
		return m.initErr
	}
	m.inited = true
	return nil
}

// mockToolProvider is a ToolProvider for testing.
type mockToolProvider struct {
	mockSkill
	tools []*mcp.ToolDef
}

func (m *mockToolProvider) Tools() []*mcp.ToolDef { return m.tools }

// mockWorkflowProvider is a WorkflowProvider for testing.
type mockWorkflowProvider struct {
	mockSkill
	prompts map[string]string
}

func (m *mockWorkflowProvider) Prompts() map[string]string { return m.prompts }

func TestRegisterAndGet(t *testing.T) {
	Reset()
	defer Reset()

	s := &mockSkill{name: "test-skill", desc: "A test skill"}
	Register(s)

	got, ok := Get("test-skill")
	require.True(t, ok)
	assert.Equal(t, "test-skill", got.Name())
	assert.Equal(t, "A test skill", got.Description())
}

func TestGetNotFound(t *testing.T) {
	Reset()
	defer Reset()

	_, ok := Get("nonexistent")
	assert.False(t, ok)
}

func TestAll(t *testing.T) {
	Reset()
	defer Reset()

	Register(&mockSkill{name: "beta"})
	Register(&mockSkill{name: "alpha"})

	all := All()
	require.Len(t, all, 2)
	assert.Equal(t, "alpha", all[0].Name())
	assert.Equal(t, "beta", all[1].Name())
}

func TestToolProviders(t *testing.T) {
	Reset()
	defer Reset()

	tp := &mockToolProvider{
		mockSkill: mockSkill{name: "tool-skill"},
		tools:     []*mcp.ToolDef{{Name: "my-tool", Description: "a tool"}},
	}
	plain := &mockSkill{name: "plain-skill"}
	Register(tp)
	Register(plain)

	providers := ToolProviders()
	require.Len(t, providers, 1)
	assert.Equal(t, "tool-skill", providers[0].Name())
	assert.Len(t, providers[0].Tools(), 1)
}

func TestWorkflowProviders(t *testing.T) {
	Reset()
	defer Reset()

	wp := &mockWorkflowProvider{
		mockSkill: mockSkill{name: "wf-skill"},
		prompts:   map[string]string{"greeting": "Hello {{.Name}}"},
	}
	plain := &mockSkill{name: "plain-skill"}
	Register(wp)
	Register(plain)

	providers := WorkflowProviders()
	require.Len(t, providers, 1)
	assert.Equal(t, "wf-skill", providers[0].Name())
	assert.Contains(t, providers[0].Prompts(), "greeting")
}

func TestInitAll(t *testing.T) {
	Reset()
	defer Reset()

	s1 := &mockSkill{name: "s1"}
	s2 := &mockSkill{name: "s2"}
	Register(s1)
	Register(s2)

	deps := SkillDeps{
		ProjectDir: "/tmp/project",
		GlobalDir:  "/tmp/global",
		Logger:     slog.Default(),
	}
	err := InitAll(deps)
	require.NoError(t, err)
	assert.True(t, s1.inited)
	assert.True(t, s2.inited)
}

func TestInitAllError(t *testing.T) {
	Reset()
	defer Reset()

	s1 := &mockSkill{name: "s1", initErr: fmt.Errorf("boom")}
	Register(s1)

	deps := SkillDeps{Logger: slog.Default()}
	err := InitAll(deps)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "boom")
}

func TestReset(t *testing.T) {
	Reset()

	Register(&mockSkill{name: "temp"})
	require.Len(t, All(), 1)

	Reset()
	assert.Empty(t, All())
}

func TestDuplicateRegistration(t *testing.T) {
	Reset()
	defer Reset()

	s1 := &mockSkill{name: "dup", desc: "first"}
	s2 := &mockSkill{name: "dup", desc: "second"}
	Register(s1)
	Register(s2)

	got, ok := Get("dup")
	require.True(t, ok)
	assert.Equal(t, "second", got.Description(), "last registration should win")
}
