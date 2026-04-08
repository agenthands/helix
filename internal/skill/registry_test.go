package skill

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
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

// --- Spec tests ---

func TestResolveToolsWithRegisteredSkill(t *testing.T) {
	Reset()
	defer Reset()

	tp := &mockToolProvider{
		mockSkill: mockSkill{name: "file-ops"},
		tools: []*mcp.ToolDef{
			{Name: "read_file", Description: "Read a file"},
			{Name: "write_file", Description: "Write a file"},
		},
	}
	Register(tp)

	tools := ResolveTools([]string{"file-ops"}, nil, nil)
	require.Len(t, tools, 2)

	names := make(map[string]bool)
	for _, td := range tools {
		names[td.Name] = true
	}
	assert.True(t, names["read_file"])
	assert.True(t, names["write_file"])
}

func TestResolveToolsExclude(t *testing.T) {
	Reset()
	defer Reset()

	tp := &mockToolProvider{
		mockSkill: mockSkill{name: "file-ops"},
		tools: []*mcp.ToolDef{
			{Name: "read_file", Description: "Read a file"},
			{Name: "write_file", Description: "Write a file"},
			{Name: "delete_file", Description: "Delete a file"},
		},
	}
	Register(tp)

	tools := ResolveTools([]string{"file-ops"}, nil, []string{"delete_file"})
	require.Len(t, tools, 2)

	names := make(map[string]bool)
	for _, td := range tools {
		names[td.Name] = true
	}
	assert.True(t, names["read_file"])
	assert.True(t, names["write_file"])
	assert.False(t, names["delete_file"])
}

func TestResolveToolsIncludeFromOtherSkill(t *testing.T) {
	Reset()
	defer Reset()

	tp1 := &mockToolProvider{
		mockSkill: mockSkill{name: "core"},
		tools:     []*mcp.ToolDef{{Name: "ping", Description: "Ping"}},
	}
	tp2 := &mockToolProvider{
		mockSkill: mockSkill{name: "extra"},
		tools:     []*mcp.ToolDef{{Name: "special", Description: "Special tool"}},
	}
	Register(tp1)
	Register(tp2)

	// Only activate "core" skill, but explicitly include "special" from extra.
	tools := ResolveTools([]string{"core"}, []string{"special"}, nil)
	require.Len(t, tools, 2)

	names := make(map[string]bool)
	for _, td := range tools {
		names[td.Name] = true
	}
	assert.True(t, names["ping"])
	assert.True(t, names["special"])
}

func TestLoadContextSpecs(t *testing.T) {
	yamlContent := `contexts:
  - name: agent
    description: "Agent context for Claude Code"
    skills:
      - file-ops
      - symbol-ops
    tools:
      - ping
    exclude_tools:
      - dangerous_tool
  - name: ide
    description: "IDE assistant context"
    skills:
      - file-ops
`
	dir := t.TempDir()
	path := filepath.Join(dir, "contexts.yml")
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0644))

	specs, err := LoadContextSpecs(path)
	require.NoError(t, err)
	require.Len(t, specs, 2)

	assert.Equal(t, "agent", specs[0].Name)
	assert.Equal(t, "Agent context for Claude Code", specs[0].Description)
	assert.Equal(t, []string{"file-ops", "symbol-ops"}, specs[0].Skills)
	assert.Equal(t, []string{"ping"}, specs[0].Tools)
	assert.Equal(t, []string{"dangerous_tool"}, specs[0].ExcludeTools)

	assert.Equal(t, "ide", specs[1].Name)
	assert.Equal(t, []string{"file-ops"}, specs[1].Skills)
}

func TestLoadModeSpecs(t *testing.T) {
	yamlContent := `modes:
  - name: planning
    description: "Planning mode"
    skills:
      - memory
    tools: []
    exclude_tools:
      - write_file
    prompts:
      system: "You are in planning mode"
  - name: editing
    description: "Editing mode"
    skills:
      - file-ops
      - symbol-ops
`
	dir := t.TempDir()
	path := filepath.Join(dir, "modes.yml")
	require.NoError(t, os.WriteFile(path, []byte(yamlContent), 0644))

	specs, err := LoadModeSpecs(path)
	require.NoError(t, err)
	require.Len(t, specs, 2)

	assert.Equal(t, "planning", specs[0].Name)
	assert.Equal(t, "Planning mode", specs[0].Description)
	assert.Equal(t, []string{"memory"}, specs[0].Skills)
	assert.Equal(t, []string{"write_file"}, specs[0].ExcludeTools)
	assert.Equal(t, map[string]string{"system": "You are in planning mode"}, specs[0].Prompts)

	assert.Equal(t, "editing", specs[1].Name)
	assert.Equal(t, []string{"file-ops", "symbol-ops"}, specs[1].Skills)
}

func TestLoadContextSpecsFileNotFound(t *testing.T) {
	_, err := LoadContextSpecs("/nonexistent/path.yml")
	require.Error(t, err)
}

func TestLoadModeSpecsFileNotFound(t *testing.T) {
	_, err := LoadModeSpecs("/nonexistent/path.yml")
	require.Error(t, err)
}
