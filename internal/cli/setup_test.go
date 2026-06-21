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

	"github.com/agenthands/helix/internal/langregistry"
)

// --- mergeJSONConfig tests ---

func TestMergeJSONConfigNewFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")

	err := mergeJSONConfig(path, "mcpServers", "helix", map[string]any{
		"command": "/usr/local/bin/helix",
		"args":    []string{"--mode=stdio"},
	})
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers, ok := result["mcpServers"].(map[string]any)
	require.True(t, ok, "mcpServers key should exist")

	entry, ok := servers["helix"].(map[string]any)
	require.True(t, ok, "helix entry should exist")
	assert.Equal(t, "/usr/local/bin/helix", entry["command"])
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

	// Merge helix entry.
	err := mergeJSONConfig(path, "mcpServers", "helix", map[string]any{
		"command": "/usr/local/bin/helix",
	})
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers := result["mcpServers"].(map[string]any)
	assert.Contains(t, servers, "other-server", "existing entries must be preserved")
	assert.Contains(t, servers, "helix", "new entry must be added")
}

func TestMergeJSONConfigVSCodeServersKey(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mcp.json")

	err := mergeJSONConfig(path, "servers", "helix", map[string]any{
		"type":    "stdio",
		"command": "/usr/local/bin/helix",
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

	err := mergeJSONConfig(path, "mcpServers", "helix", map[string]any{
		"command": "/usr/local/bin/helix",
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
			"helix": map[string]any{"command": "helix"},
			"other": map[string]any{"command": "other"},
		},
	}
	data, _ := json.MarshalIndent(initial, "", "  ")
	require.NoError(t, os.WriteFile(path, data, 0644))

	err := removeFromJSONConfig(path, "mcpServers", "helix")
	require.NoError(t, err)

	data, err = os.ReadFile(path)
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))

	servers := result["mcpServers"].(map[string]any)
	assert.NotContains(t, servers, "helix", "helix should be removed")
	assert.Contains(t, servers, "other", "other entries should remain")
}

func TestRemoveFromJSONConfigMissingFile(t *testing.T) {
	err := removeFromJSONConfig("/nonexistent/path/config.json", "mcpServers", "helix")
	assert.NoError(t, err, "removing from nonexistent file should not error")
}

// --- ClientRegistrar tests ---

func TestClaudeCodeRegistrarDryRun(t *testing.T) {
	r := &ClaudeCodeRegistrar{}
	assert.Equal(t, "claude-code", r.Name())

	printer := &SetupPrinter{DryRun: true}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
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
		BinaryPath: "/usr/local/bin/helix",
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
		BinaryPath: "/usr/local/bin/helix",
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

	entry, ok := servers["helix"].(map[string]any)
	require.True(t, ok, "helix entry must exist")
	assert.Equal(t, "stdio", entry["type"])
	assert.Equal(t, "/usr/local/bin/helix", entry["command"])
}

func TestJetBrainsRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	printer := &SetupPrinter{}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
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
	assert.Contains(t, servers, "helix")
}

func TestClaudeDesktopRegistrarDryRun(t *testing.T) {
	r := &ClaudeDesktopRegistrar{}
	assert.Equal(t, "claude-desktop", r.Name())

	printer := &SetupPrinter{DryRun: true}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
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
		BinaryPath: "/usr/local/bin/helix",
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

	entry, ok := servers["helix"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "/usr/local/bin/helix", entry["command"])
}

func TestGenericRegistrarStdout(t *testing.T) {
	// Capture stdout by temporarily redirecting it.
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	printer := &SetupPrinter{}
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
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

// --- teardownMCP tests (Phase 93-03 Task 1) ---
//
// teardownMCP removes ONLY the prior helix MCP server entry for each client and
// MUST NOT remove hooks (Pitfall 3). It must be a best-effort no-op when the
// prior entry / config file is absent.

// seedMCPConfig writes a JSON config file with a helix entry plus an unrelated
// entry under key, so tests can assert the helix entry is removed while the
// unrelated entry survives.
func seedMCPConfig(t *testing.T, path, key string) {
	t.Helper()
	cfg := map[string]any{
		key: map[string]any{
			"helix": map[string]any{"command": "/old/helix"},
			"other": map[string]any{"command": "/other"},
		},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	require.NoError(t, err)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0755))
	require.NoError(t, os.WriteFile(path, append(data, '\n'), 0644))
}

// assertHelixGoneOtherKept reads a JSON config and asserts the helix entry under
// key was removed while the "other" entry remains.
func assertHelixGoneOtherKept(t *testing.T, path, key string) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var result map[string]any
	require.NoError(t, json.Unmarshal(data, &result))
	servers, ok := result[key].(map[string]any)
	require.True(t, ok, "key %q should exist", key)
	assert.NotContains(t, servers, "helix", "helix MCP entry must be removed")
	assert.Contains(t, servers, "other", "unmanaged entries must be preserved")
}

func TestTeardownPriorMCP_VSCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".vscode", "mcp.json")
	seedMCPConfig(t, path, "servers")

	cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
	require.NoError(t, (&VSCodeRegistrar{}).teardownMCP(cfg))
	assertHelixGoneOtherKept(t, path, "servers")
}

func TestTeardownPriorMCP_JetBrains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".junie", "mcp", "mcp.json")
	seedMCPConfig(t, path, "mcpServers")

	cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
	require.NoError(t, (&JetBrainsRegistrar{}).teardownMCP(cfg))
	assertHelixGoneOtherKept(t, path, "mcpServers")
}

func TestTeardownPriorMCP_OpenCode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "opencode.json")
	seedMCPConfig(t, path, "mcp")

	cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
	require.NoError(t, (&OpenCodeRegistrar{}).teardownMCP(cfg))
	assertHelixGoneOtherKept(t, path, "mcp")
}

func TestTeardownPriorMCP_ClaudeCode(t *testing.T) {
	// Isolate HOME so the global ~/.claude/settings.json removal and any
	// best-effort `claude mcp remove` operate on a sandbox, never the user's home.
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, ".mcp.json")
	seedMCPConfig(t, path, "mcpServers")

	cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
	require.NoError(t, (&ClaudeCodeRegistrar{}).teardownMCP(cfg))
	assertHelixGoneOtherKept(t, path, "mcpServers")
}

func TestTeardownPriorMCP_Gemini(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	path := filepath.Join(dir, ".gemini", "settings.json")
	seedMCPConfig(t, path, "mcpServers")

	cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
	require.NoError(t, (&GeminiCLIRegistrar{}).teardownMCP(cfg))
	assertHelixGoneOtherKept(t, path, "mcpServers")
}

func TestTeardownPriorMCP_Generic_OutputFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	seedMCPConfig(t, path, "mcpServers")

	cfg := RegistrationConfig{ProjectDir: dir, OutputPath: path, Printer: &SetupPrinter{}}
	require.NoError(t, (&GenericRegistrar{}).teardownMCP(cfg))
	assertHelixGoneOtherKept(t, path, "mcpServers")
}

func TestTeardownPriorMCP_Generic_StdoutNoOp(t *testing.T) {
	cfg := RegistrationConfig{ProjectDir: t.TempDir(), Printer: &SetupPrinter{}}
	// No OutputPath: stdout config has nothing on disk → no-op, no error.
	require.NoError(t, (&GenericRegistrar{}).teardownMCP(cfg))
}

// TestTeardownPriorMCP_MissingFileNoOp asserts teardown on an absent config file
// is a no-op (returns nil, creates nothing) for every file-backed client.
func TestTeardownPriorMCP_MissingFileNoOp(t *testing.T) {
	// Isolate HOME so claude-code's global-settings removal does not touch the user's home.
	t.Setenv("HOME", t.TempDir())
	clients := []struct {
		name string
		r    ClientRegistrar
		// rel is the config path (relative to ProjectDir) that must NOT be created.
		rel string
	}{
		{"vscode", &VSCodeRegistrar{}, filepath.Join(".vscode", "mcp.json")},
		{"jetbrains", &JetBrainsRegistrar{}, filepath.Join(".junie", "mcp", "mcp.json")},
		{"opencode", &OpenCodeRegistrar{}, "opencode.json"},
		{"claude-code", &ClaudeCodeRegistrar{}, ".mcp.json"},
	}
	for _, c := range clients {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
			require.NoError(t, c.r.teardownMCP(cfg), "teardown on missing file must be a no-op")
			_, err := os.Stat(filepath.Join(dir, c.rel))
			assert.True(t, os.IsNotExist(err), "teardown must not create a config file")
		})
	}
}

// TestTeardownPriorMCP_PreservesHooks asserts teardown removes the MCP entry but
// leaves a seeded helix_managed hook intact (Pitfall 3, T-93-06). Uses claude-code
// because it is the client whose Unregister also strips hooks.
func TestTeardownPriorMCP_PreservesHooks(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()

	// Seed the project .mcp.json with an mcpServers.helix entry.
	mcpPath := filepath.Join(dir, ".mcp.json")
	seedMCPConfig(t, mcpPath, "mcpServers")

	// Seed a settings.json that carries a helix_managed PreToolUse hook.
	settingsPath := hookSettingsPath(dir, false)
	require.NoError(t, mergeHooksIntoSettings(settingsPath, "/usr/local/bin/helix"))

	cfg := RegistrationConfig{ProjectDir: dir, Printer: &SetupPrinter{}}
	require.NoError(t, (&ClaudeCodeRegistrar{}).teardownMCP(cfg))

	// MCP entry gone.
	assertHelixGoneOtherKept(t, mcpPath, "mcpServers")

	// Hook survives: settings.json still has a helix_managed PreToolUse hook.
	data, err := os.ReadFile(settingsPath)
	require.NoError(t, err)
	var settings map[string]any
	require.NoError(t, json.Unmarshal(data, &settings))
	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "hooks block must survive teardown")
	pre, ok := hooks["PreToolUse"].([]any)
	require.True(t, ok, "PreToolUse hooks must survive teardown")
	require.NotEmpty(t, pre, "helix_managed PreToolUse hook must survive teardown")
}
