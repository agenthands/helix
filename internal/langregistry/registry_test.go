package langregistry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRegistryDefaultEntriesCount(t *testing.T) {
	assert.GreaterOrEqual(t, len(defaultEntries), 40,
		"defaultEntries should have at least 40 language entries")
}

func TestRegistryGet(t *testing.T) {
	reg, err := NewRegistry()
	require.NoError(t, err)

	tests := []struct {
		lang    string
		command string
	}{
		{"go", "gopls"},
		{"python", "pyright-langserver"},
		{"typescript", "typescript-language-server"},
		{"rust", "rust-analyzer"},
		{"bash", "bash-language-server"},
	}
	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			entry, ok := reg.Get(tt.lang)
			assert.True(t, ok, "language %q should exist", tt.lang)
			assert.Equal(t, tt.command, entry.Command)
			assert.Equal(t, tt.lang, entry.Language)
		})
	}
}

func TestRegistryGetUnknown(t *testing.T) {
	reg, err := NewRegistry()
	require.NoError(t, err)
	_, ok := reg.Get("nonexistent_language_xyz")
	assert.False(t, ok)
}

func TestRegistryLanguages(t *testing.T) {
	reg, err := NewRegistry()
	require.NoError(t, err)
	langs := reg.Languages()
	assert.GreaterOrEqual(t, len(langs), 40)
	// Verify sorted order.
	for i := 1; i < len(langs); i++ {
		assert.True(t, langs[i-1] < langs[i], "Languages() should be sorted: %q >= %q", langs[i-1], langs[i])
	}
}

func TestRegistryByExtension(t *testing.T) {
	reg, err := NewRegistry()
	require.NoError(t, err)

	pyEntries := reg.ByExtension(".py")
	assert.NotEmpty(t, pyEntries, "should find entries for .py")
	found := false
	for _, e := range pyEntries {
		if e.Language == "python" {
			found = true
			break
		}
	}
	assert.True(t, found, ".py should match the python entry")

	goEntries := reg.ByExtension(".go")
	assert.NotEmpty(t, goEntries)
	assert.Equal(t, "go", goEntries[0].Language)
}

func TestRegistryYAMLOverrideDeepMerge(t *testing.T) {
	// Create a temp YAML override that changes only the command for "go"
	// but preserves InitOptions and other fields.
	tmpDir := t.TempDir()
	overrideFile := filepath.Join(tmpDir, "languages.yaml")
	content := `go:
  command: "gopls-custom"
`
	require.NoError(t, os.WriteFile(overrideFile, []byte(content), 0644))

	reg, err := NewRegistry(overrideFile)
	require.NoError(t, err)

	entry, ok := reg.Get("go")
	require.True(t, ok)
	// Command should be overridden.
	assert.Equal(t, "gopls-custom", entry.Command)
	// Args should be preserved from defaults.
	assert.Equal(t, []string{"serve"}, entry.Args)
	// InitOptions should be preserved from defaults.
	assert.NotNil(t, entry.InitOptions)
	assert.Equal(t, true, entry.InitOptions["experimentalWorkspaceModule"])
	// NeedsWorkspace should be preserved.
	assert.True(t, entry.NeedsWorkspace)
}

func TestRegistryYAMLOverrideNewLanguage(t *testing.T) {
	tmpDir := t.TempDir()
	overrideFile := filepath.Join(tmpDir, "languages.yaml")
	content := `custom_lang:
  command: "custom-ls"
  args: ["--stdio"]
  file_exts: [".custom"]
`
	require.NoError(t, os.WriteFile(overrideFile, []byte(content), 0644))

	reg, err := NewRegistry(overrideFile)
	require.NoError(t, err)

	entry, ok := reg.Get("custom_lang")
	require.True(t, ok)
	assert.Equal(t, "custom-ls", entry.Command)
	assert.Equal(t, []string{"--stdio"}, entry.Args)
	assert.Equal(t, []string{".custom"}, entry.FileExts)
}

func TestRegistryMissingOverrideFileSkipped(t *testing.T) {
	reg, err := NewRegistry("/nonexistent/path/override.yaml")
	require.NoError(t, err)
	// Should still have all defaults.
	assert.GreaterOrEqual(t, len(reg.Languages()), 40)
}

func TestInstallHint(t *testing.T) {
	tests := []struct {
		name    string
		entry   LSEntry
		contain string
	}{
		{
			name:    "npm with version",
			entry:   LSEntry{Command: "ts-ls", Install: &InstallInfo{Type: "npm", Package: "typescript-language-server", Version: "5.1.3"}},
			contain: "npm install -g typescript-language-server@5.1.3",
		},
		{
			name:    "pip without version",
			entry:   LSEntry{Command: "pyright-langserver", Install: &InstallInfo{Type: "pip", Package: "pyright"}},
			contain: "pip install pyright",
		},
		{
			name:    "system only",
			entry:   LSEntry{Command: "gopls"},
			contain: "Install \"gopls\" manually",
		},
		{
			name:    "cargo",
			entry:   LSEntry{Command: "taplo", Install: &InstallInfo{Type: "cargo", Package: "taplo-cli"}},
			contain: "cargo install taplo-cli",
		},
		{
			name:    "gem",
			entry:   LSEntry{Command: "solargraph", Install: &InstallInfo{Type: "gem", Package: "solargraph"}},
			contain: "gem install solargraph",
		},
		{
			name:    "dotnet",
			entry:   LSEntry{Command: "fsautocomplete", Install: &InstallInfo{Type: "dotnet", Package: "fsautocomplete"}},
			contain: "dotnet tool install -g fsautocomplete",
		},
		{
			name:    "binary",
			entry:   LSEntry{Command: "clangd", Install: &InstallInfo{Type: "binary", Package: "clangd"}},
			contain: "Download clangd",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hint := tt.entry.InstallHint()
			assert.Contains(t, hint, tt.contain)
		})
	}
}
