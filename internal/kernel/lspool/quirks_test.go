package lspool

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/postfix/serena/internal/langregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuirkAdapter_DefaultReturnsEntryInitOptions(t *testing.T) {
	entry := langregistry.LSEntry{
		Language:    "bash",
		Command:     "bash-language-server",
		Args:        []string{"start"},
		InitOptions: map[string]any{"foo": "bar"},
	}
	adapter := &DefaultQuirkAdapter{Entry: entry}

	opts := adapter.InitOptions("/tmp/workspace")
	assert.Equal(t, map[string]any{"foo": "bar"}, opts)
}

func TestQuirkAdapter_DefaultNormalizeSymbolNameIdentity(t *testing.T) {
	entry := langregistry.LSEntry{Language: "bash"}
	adapter := &DefaultQuirkAdapter{Entry: entry}

	assert.Equal(t, "myFunc", adapter.NormalizeSymbolName("myFunc"))
}

func TestQuirkAdapter_DefaultNotificationHandlersNil(t *testing.T) {
	entry := langregistry.LSEntry{Language: "bash"}
	adapter := &DefaultQuirkAdapter{Entry: entry}

	assert.Nil(t, adapter.NotificationHandlers())
}

func TestQuirkAdapter_DefaultPostInitializeNoOp(t *testing.T) {
	entry := langregistry.LSEntry{Language: "bash"}
	adapter := &DefaultQuirkAdapter{Entry: entry}

	err := adapter.PostInitialize(context.Background(), nil)
	assert.NoError(t, err)
}

func TestQuirkAdapter_GoplsNormalizeSymbolName(t *testing.T) {
	entry := langregistry.LSEntry{Language: "go", Command: "gopls"}
	adapter := &GoplsAdapter{Entry: entry}

	tests := []struct {
		input    string
		expected string
	}{
		{"pkg.Foo", "Foo"},
		{"net/http.Handler", "Handler"},
		{"Foo", "Foo"},
		{"a.b.c.Method", "Method"},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, adapter.NormalizeSymbolName(tt.input), "input: %s", tt.input)
	}
}

func TestQuirkAdapter_GoplsInitOptionsIncludesExperimentalWorkspaceModule(t *testing.T) {
	entry := langregistry.LSEntry{
		Language:    "go",
		Command:     "gopls",
		InitOptions: map[string]any{"staticcheck": true},
	}
	adapter := &GoplsAdapter{Entry: entry}

	opts := adapter.InitOptions("/tmp/workspace")
	assert.Equal(t, true, opts["experimentalWorkspaceModule"])
	assert.Equal(t, true, opts["staticcheck"])
}

func TestQuirkAdapter_RustAnalyzerInitOptionsIncludesCargoScripts(t *testing.T) {
	entry := langregistry.LSEntry{
		Language:    "rust",
		Command:     "rust-analyzer",
		InitOptions: map[string]any{},
	}
	adapter := &RustAnalyzerAdapter{Entry: entry}

	opts := adapter.InitOptions("/tmp/workspace")
	cargo, ok := opts["cargo"].(map[string]any)
	require.True(t, ok, "expected cargo key in init options")
	buildScripts, ok := cargo["buildScripts"].(map[string]any)
	require.True(t, ok, "expected buildScripts key")
	assert.Equal(t, true, buildScripts["enable"])
}

func TestQuirkAdapter_GetQuirkAdapter_ReturnsSpecificForGo(t *testing.T) {
	entry := langregistry.LSEntry{Language: "go", Command: "gopls"}
	adapter := GetQuirkAdapter(entry)
	_, ok := adapter.(*GoplsAdapter)
	assert.True(t, ok, "expected GoplsAdapter for go")
}

func TestQuirkAdapter_GetQuirkAdapter_ReturnsSpecificForRust(t *testing.T) {
	entry := langregistry.LSEntry{Language: "rust", Command: "rust-analyzer"}
	adapter := GetQuirkAdapter(entry)
	_, ok := adapter.(*RustAnalyzerAdapter)
	assert.True(t, ok, "expected RustAnalyzerAdapter for rust")
}

func TestQuirkAdapter_GetQuirkAdapter_ReturnsSpecificForJava(t *testing.T) {
	entry := langregistry.LSEntry{Language: "java", Command: "jdtls"}
	adapter := GetQuirkAdapter(entry)
	_, ok := adapter.(*JdtlsAdapter)
	assert.True(t, ok, "expected JdtlsAdapter for java")
}

func TestQuirkAdapter_GetQuirkAdapter_ReturnsSpecificForVue(t *testing.T) {
	entry := langregistry.LSEntry{Language: "vue", Command: "vue-language-server"}
	adapter := GetQuirkAdapter(entry)
	_, ok := adapter.(*VueAdapter)
	assert.True(t, ok, "expected VueAdapter for vue")
}

func TestQuirkAdapter_GetQuirkAdapter_ReturnsDefaultForBash(t *testing.T) {
	entry := langregistry.LSEntry{Language: "bash", Command: "bash-language-server"}
	adapter := GetQuirkAdapter(entry)
	_, ok := adapter.(*DefaultQuirkAdapter)
	assert.True(t, ok, "expected DefaultQuirkAdapter for bash")
}

func TestQuirkAdapter_JdtlsEnsureDataDir(t *testing.T) {
	tmpDir := t.TempDir()
	entry := langregistry.LSEntry{Language: "java", Command: "jdtls"}
	adapter := &JdtlsAdapter{Entry: entry}

	dataDir, err := adapter.EnsureDataDir(tmpDir)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(tmpDir, ".jdtls-data"), dataDir)

	// Verify directory was created.
	info, err := os.Stat(dataDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestQuirkAdapter_ClangdDetectsCompileCommands(t *testing.T) {
	tmpDir := t.TempDir()
	// Create compile_commands.json in build/ subdirectory.
	buildDir := filepath.Join(tmpDir, "build")
	require.NoError(t, os.MkdirAll(buildDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(buildDir, "compile_commands.json"), []byte("[]"), 0o644))

	entry := langregistry.LSEntry{Language: "c", Command: "clangd"}
	adapter := &ClangdAdapter{Entry: entry}

	opts := adapter.InitOptions(tmpDir)
	assert.Equal(t, buildDir, opts["compilationDatabasePath"])
}

func TestQuirkAdapter_ClangdNoCompileCommands(t *testing.T) {
	tmpDir := t.TempDir()
	entry := langregistry.LSEntry{Language: "c", Command: "clangd"}
	adapter := &ClangdAdapter{Entry: entry}

	opts := adapter.InitOptions(tmpDir)
	_, hasPath := opts["compilationDatabasePath"]
	assert.False(t, hasPath, "should not set compilationDatabasePath when no compile_commands.json")
}
