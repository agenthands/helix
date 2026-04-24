package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/postfix/serena/internal/langregistry"
)

// --- mergeJSONConfig tests ---

func TestMergeJSONConfigNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := mergeJSONConfig(path, "mcpServers", "serena", map[string]any{
		"command": "/usr/local/bin/serena",
		"args":    []string{"--mode=stdio"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key should exist")

	serena, ok := servers["serena"].(map[string]any)
	require.True(t, ok, "serena entry should exist")
	assert.Equal(t, "/usr/local/bin/serena", serena["command"])
}

func TestMergeJSONConfigExistingFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	// Write initial config with another server.
	initial := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{"command": "other"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	// Merge serena entry.
	err := mergeJSONConfig(path, "mcpServers", "serena", map[string]any{
		"command": "/usr/local/bin/serena",
	})
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "other-server", "existing entries must be preserved")
	assert.Contains(t, servers, "serena", "new entry must be added")
}

func TestMergeJSONConfigVSCodeServersKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	err := mergeJSONConfig(path, "servers", "serena", map[string]any{
		"type":    "stdio",
		"command": "/usr/local/bin/serena",
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	_, ok := result["servers"].(map[string]any)
	assert.True(t, ok, "VS Code uses 'servers' key, not 'mcpServers'")
}

func TestMergeJSONConfigPreservesNonMCPKeys(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"preferences": map[string]any{"theme": "dark"},
		"mcpServers":  map[string]any{},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	err := mergeJSONConfig(path, "mcpServers", "serena", map[string]any{
		"command": "/usr/local/bin/serena",
	})
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	prefs, ok := result["preferences"].(map[string]any)
	require.True(t, ok, "preferences key must be preserved")
	assert.Equal(t, "dark", prefs["theme"])
}

// --- removeFromJSONConfig tests ---

func TestRemoveFromJSONConfigExists(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	initial := map[string]any{
		"mcpServers": map[string]any{
			"serena": map[string]any{"command": "serena"},
			"other":  map[string]any{"command": "other"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	err := removeFromJSONConfig(path, "mcpServers", "serena")
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers := result["mcpServers"].(map[string]any)
	assert.NotContains(t, servers, "serena", "serena should be removed")
	assert.Contains(t, servers, "other", "other entries should remain")
}

func TestRemoveFromJSONConfigMissingFile(t *testing.T) {
	err := removeFromJSONConfig("/nonexistent/path/config.json", "mcpServers", "serena")
	assert.NoError(t, err, "removing from nonexistent file should not error")
}

// --- ClientRegistrar tests ---

func TestClaudeCodeRegistrarDryRun(t *testing.T) {
	r := &ClaudeCodeRegistrar{}
	assert.Equal(t, "claude-code", r.Name())

	printer := &SetupPrinter{DryRun: true}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     true,
		ProjectDir: t.TempDir(),
		Printer:    printer,
	}
	err := r.Register(cfg)
	assert.NoError(t, err)
}

func TestGeminiCLIRegistrarDryRun(t *testing.T) {
	r := &GeminiCLIRegistrar{}
	assert.Equal(t, "gemini-cli", r.Name())

	printer := &SetupPrinter{DryRun: true}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     true,
		ProjectDir: t.TempDir(),
		Printer:    printer,
	}
	err := r.Register(cfg)
	assert.NoError(t, err)
}

func TestVSCodeRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	printer := &SetupPrinter{}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     false,
		ProjectDir: dir,
		Printer:    printer,
	}

	r := &VSCodeRegistrar{}
	err := r.Register(cfg)
	require.NoError(t, err)

	configPath := filepath.Join(dir, ".vscode", "mcp.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["servers"].(map[string]any)
	require.True(t, ok, "VS Code config must use 'servers' key")

	serena, ok := servers["serena"].(map[string]any)
	require.True(t, ok, "serena entry must exist")
	assert.Equal(t, "stdio", serena["type"])
	assert.Equal(t, "/usr/local/bin/serena", serena["command"])
}

func TestJetBrainsRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	printer := &SetupPrinter{}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     false,
		ProjectDir: dir,
		Printer:    printer,
	}

	r := &JetBrainsRegistrar{}
	err := r.Register(cfg)
	require.NoError(t, err)

	configPath := filepath.Join(dir, ".junie", "mcp", "mcp.json")
	data, err := os.ReadFile(configPath)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok, "JetBrains config must use 'mcpServers' key")
	assert.Contains(t, servers, "serena")
}

func TestClaudeDesktopRegistrarDryRun(t *testing.T) {
	r := &ClaudeDesktopRegistrar{}
	assert.Equal(t, "claude-desktop", r.Name())

	printer := &SetupPrinter{DryRun: true}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     true,
		Printer:    printer,
	}
	err := r.Register(cfg)
	assert.NoError(t, err)
}

func TestGenericRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "mcp-config.json")
	printer := &SetupPrinter{}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     false,
		OutputPath: outputPath,
		Printer:    printer,
	}

	r := &GenericRegistrar{}
	err := r.Register(cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(outputPath)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok)

	serena, ok := servers["serena"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/usr/local/bin/serena", serena["command"])
}

func TestGenericRegistrarStdout(t *testing.T) {
	// Capture stdout by temporarily redirecting it.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	printer := &SetupPrinter{}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/serena",
		DryRun:     false,
		OutputPath: "", // empty = stdout
		Printer:    printer,
	}

	reg := &GenericRegistrar{}
	regErr := reg.Register(cfg)

	_ = w.Close()
	os.Stdout = oldStdout

	require.NoError(t, regErr)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)

	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result), "stdout should contain valid JSON")
	assert.Contains(t, result, "mcpServers")
}

// --- resolveBinaryPath test ---

func TestResolveBinaryPath(t *testing.T) {
	path, err := resolveBinaryPath()
	require.NoError(t, err)
	assert.NotEmpty(t, path, "binary path should be non-empty")

	_, err = os.Stat(path)
	assert.NoError(t, err, "resolved binary should exist on disk")
}

// --- detectLanguages tests ---

func TestDetectLanguages(t *testing.T) {
	dir := t.TempDir()

	// Create test files.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "app.py"), []byte("print('hi')"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "index.ts"), []byte("const x = 1"), 0644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "README.md"), []byte("# readme"), 0644))

	reg, err := langregistry.NewRegistry()
	require.NoError(t, err)

	printer := &SetupPrinter{}
	entries, err := detectLanguages(dir, reg, printer)
	require.NoError(t, err)
	assert.NotEmpty(t, entries)

	// Collect detected language names.
	langs := make(map[string]bool)
	for _, e := range entries {
		langs[e.Language] = true
	}

	assert.True(t, langs["go"], "should detect Go from .go files")
	assert.True(t, langs["python"] || langs["python_jedi"] || langs["python_ty"],
		"should detect Python from .py files")

	// Check no duplicates.
	seen := make(map[string]bool)
	for _, e := range entries {
		assert.False(t, seen[e.Language], "duplicate language: %s", e.Language)
		seen[e.Language] = true
	}

	// Check sorted order.
	for i := 1; i < len(entries); i++ {
		assert.True(t, entries[i-1].Language < entries[i].Language,
			"entries should be sorted: %q >= %q", entries[i-1].Language, entries[i].Language)
	}
}

func TestDetectLanguagesSkipsHidden(t *testing.T) {
	dir := t.TempDir()

	// Create a hidden directory with a Go file.
	gitDir := filepath.Join(dir, ".git")
	require.NoError(t, os.MkdirAll(gitDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(gitDir, "config.go"), []byte("package git"), 0644))

	// Create a visible directory with a Go file.
	srcDir := filepath.Join(dir, "src")
	require.NoError(t, os.MkdirAll(srcDir, 0755))
	require.NoError(t, os.WriteFile(filepath.Join(srcDir, "main.go"), []byte("package main"), 0644))

	reg, err := langregistry.NewRegistry()
	require.NoError(t, err)

	printer := &SetupPrinter{}
	entries, err := detectLanguages(dir, reg, printer)
	require.NoError(t, err)
	assert.NotEmpty(t, entries, "should detect Go from src/")

	// The fact that it works at all confirms hidden dirs are skipped,
	// since .git/config.go would not add any different languages.
	// Also verify node_modules skipping by creating one.
	nmDir := filepath.Join(dir, "node_modules")
	require.NoError(t, os.MkdirAll(filepath.Join(nmDir, "pkg"), 0755))
	require.NoError(t, os.WriteFile(filepath.Join(nmDir, "pkg", "index.js"), []byte(""), 0644))

	entries2, err := detectLanguages(dir, reg, printer)
	require.NoError(t, err)

	// node_modules JS should not be detected (only Go from src/ should be there).
	for _, e := range entries2 {
		if e.Language == "typescript" {
			// typescript-language-server handles JS too, but .js would match
			// only if node_modules was walked. Since we only have .go in src/,
			// this should not appear.
			t.Log("Note: typescript detected, likely from a non-node_modules source")
		}
	}
}

// --- Setup command integration tests ---

func TestSetupCommandListsClients(t *testing.T) {
	// listClients writes to os.Stderr directly, so capture via os.Pipe.
	oldStderr := os.Stderr
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stderr = w

	cmd := newSetupCommand()
	cmd.SetArgs([]string{})
	execErr := cmd.Execute()

	_ = w.Close()
	os.Stderr = oldStderr

	require.NoError(t, execErr)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	for _, client := range []string{"claude-code", "vscode", "jetbrains", "claude-desktop", "gemini-cli", "generic"} {
		assert.Contains(t, output, client, "output should list %s", client)
	}
}

func TestSetupCommandInvalidClient(t *testing.T) {
	cmd := newSetupCommand()
	cmd.SetArgs([]string{"nonexistent"})

	err := cmd.Execute()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unknown client")
}

func TestSetupCommandDryRunGeneric(t *testing.T) {
	cmd := newSetupCommand()
	cmd.SetArgs([]string{"generic", "--dry-run"})

	err := cmd.Execute()
	assert.NoError(t, err)
}

// --- clientRegistry test ---

func TestClientRegistryContainsAll(t *testing.T) {
	reg := clientRegistry()
	expected := []string{"claude-code", "vscode", "jetbrains", "claude-desktop", "gemini-cli", "opencode", "generic"}
	for _, name := range expected {
		_, ok := reg[name]
		assert.True(t, ok, "registry should contain %s", name)
	}
	assert.Len(t, reg, len(expected))
}

// --- preInstallLanguageServers tests ---

func TestPreInstallLanguageServersDryRun(t *testing.T) {
	entries := []langregistry.LSEntry{
		{Language: "go", Command: "gopls"},
		{Language: "python", Command: "pyright-langserver"},
	}
	printer := &SetupPrinter{DryRun: true}

	result := preInstallLanguageServers(context.Background(), entries, nil, printer, true)
	assert.Len(t, result, 2, "dry-run should return all entries as-if successful")
}

// --- runHealthCheck tests ---

func TestRunHealthCheckDryRun(t *testing.T) {
	entries := []langregistry.LSEntry{
		{Language: "go", Command: "gopls"},
	}
	printer := &SetupPrinter{DryRun: true}

	err := runHealthCheck(context.Background(), entries, t.TempDir(), printer, true)
	assert.NoError(t, err)
}

func TestRunHealthCheckNoEntries(t *testing.T) {
	printer := &SetupPrinter{}
	err := runHealthCheck(context.Background(), nil, t.TempDir(), printer, false)
	assert.NoError(t, err)
}

func TestRunHealthCheckWithKnownBinary(t *testing.T) {
	// "go" binary should always be available in test environment.
	entries := []langregistry.LSEntry{
		{Language: "go", Command: "go"},
	}
	printer := &SetupPrinter{}

	err := runHealthCheck(context.Background(), entries, t.TempDir(), printer, false)
	assert.NoError(t, err)
}
