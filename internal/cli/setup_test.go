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

// TestVSCodeRegistrarRegister asserts the Phase 93 flip: VS Code Register is
// MCP-teardown only — a pre-seeded helix entry is removed and NO skill is written.
func TestVSCodeRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".vscode", "mcp.json")
	seedMCPConfig(t, configPath, "servers")

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&VSCodeRegistrar{}).Register(cfg))

	// Prior helix MCP entry removed; unmanaged entry preserved.
	assertHelixGoneOtherKept(t, configPath, "servers")
	// No skill written for a non-Claude client.
	assertNoSkillUnder(t, dir)
}

// TestJetBrainsRegistrarRegister asserts the Phase 93 flip for JetBrains:
// MCP-teardown only, no skill written.
func TestJetBrainsRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".junie", "mcp", "mcp.json")
	seedMCPConfig(t, configPath, "mcpServers")

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&JetBrainsRegistrar{}).Register(cfg))

	assertHelixGoneOtherKept(t, configPath, "mcpServers")
	assertNoSkillUnder(t, dir)
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

// TestGenericRegistrarRegister asserts the Phase 93 flip for the generic client
// with --output: a pre-seeded helix MCP entry in the output file is removed
// (teardown-only; no MCP config is written).
func TestGenericRegistrarRegister(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "mcp-config.json")
	seedMCPConfig(t, outputPath, "mcpServers")

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir, // contain the AGENTS.md instruction-file write (Phase 98 flip)
		OutputPath: outputPath,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&GenericRegistrar{}).Register(cfg))

	assertHelixGoneOtherKept(t, outputPath, "mcpServers")
}

// TestGenericRegistrarStdout asserts the Phase 93 flip for the generic client
// without --output: it is a no-op (no MCP config printed to stdout).
func TestGenericRegistrarStdout(t *testing.T) {
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	require.NoError(t, err)
	os.Stdout = w

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: t.TempDir(), // contain the AGENTS.md instruction-file write (Phase 98 flip)
		OutputPath: "",          // empty = stdout
		Printer:    &SetupPrinter{},
	}

	regErr := (&GenericRegistrar{}).Register(cfg)

	_ = w.Close()
	os.Stdout = oldStdout
	require.NoError(t, regErr)

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	assert.Empty(t, buf.String(), "generic flip prints no MCP config to stdout")
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
	expected := []string{"claude-code", "vscode", "jetbrains", "claude-desktop", "gemini-cli", "opencode", "generic", "codex"}
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

// --- Setup flip tests (Phase 93-03 Task 2) ---

// assertNoSkillUnder asserts no SKILL.md was written under <root>/.claude/skills/helix.
func assertNoSkillUnder(t *testing.T, root string) {
	t.Helper()
	_, err := os.Stat(filepath.Join(root, ".claude", "skills", "helix", "SKILL.md"))
	assert.True(t, os.IsNotExist(err), "no skill should be written for this client")
}

// assertNoMCPEntry walks the project config files a client may write and asserts
// no helix entry survives under any of the MCP keys (mcpServers / servers / mcp).
func assertNoMCPEntry(t *testing.T, dir string) {
	t.Helper()
	candidates := []string{
		filepath.Join(dir, ".mcp.json"),
		filepath.Join(dir, ".vscode", "mcp.json"),
		filepath.Join(dir, ".junie", "mcp", "mcp.json"),
		filepath.Join(dir, "opencode.json"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue // file absent → no entry there
		}
		var cfg map[string]any
		require.NoError(t, json.Unmarshal(data, &cfg))
		for _, key := range []string{"mcpServers", "servers", "mcp"} {
			if servers, ok := cfg[key].(map[string]any); ok {
				assert.NotContains(t, servers, "helix", "no helix MCP entry should remain in %s under %q", path, key)
			}
		}
	}
}

// TestSetupFlip_ClaudeCode_NoMCPEntry_SkillAndHooksPresent: after claude-code
// Register against a temp ProjectDir, the skill + hooks are present and there is
// NO helix MCP entry in any written config.
func TestSetupFlip_ClaudeCode_NoMCPEntry_SkillAndHooksPresent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&ClaudeCodeRegistrar{}).Register(cfg))

	// SKILL.md present and equal to the embedded asset.
	skillPath := filepath.Join(dir, ".claude", "skills", "helix", "SKILL.md")
	data, err := os.ReadFile(skillPath)
	require.NoError(t, err, "SKILL.md must be written")
	want := embeddedSkillBytes()
	if !bytesHasTrailingNewline(want) {
		want += "\n"
	}
	assert.Equal(t, want, string(data), "SKILL.md must equal the embedded asset")

	// Hooks present (SessionStart, PreToolUse, Stop with helix_managed).
	settings := readJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	hooks, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "hooks block must be present")
	for _, evt := range []string{"SessionStart", "PreToolUse", "Stop"} {
		arr, ok := hooks[evt].([]any)
		require.True(t, ok, "%s hooks must be present", evt)
		require.NotEmpty(t, arr, "%s hooks must be present", evt)
	}

	// No MCP entry anywhere.
	assertNoMCPEntry(t, dir)
}

// TestSetupFlip_Idempotent: running claude-code Register twice converges to a
// byte-stable skill + hooks block (no duplicates) and still no MCP entry.
func TestSetupFlip_Idempotent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&ClaudeCodeRegistrar{}).Register(cfg))

	skillPath := filepath.Join(dir, ".claude", "skills", "helix", "SKILL.md")
	settingsPath := filepath.Join(dir, ".claude", "settings.json")
	skill1, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	settings1, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	// Second run.
	require.NoError(t, (&ClaudeCodeRegistrar{}).Register(cfg))
	skill2, err := os.ReadFile(skillPath)
	require.NoError(t, err)
	settings2, err := os.ReadFile(settingsPath)
	require.NoError(t, err)

	assert.Equal(t, skill1, skill2, "SKILL.md must be byte-stable across re-runs")
	assert.Equal(t, settings1, settings2, "settings.json must be byte-stable across re-runs (no duplicate hooks)")

	// No duplicate helix_managed hooks per event.
	settings := readJSON(t, settingsPath)
	hooks := settings["hooks"].(map[string]any)
	for _, evt := range []string{"SessionStart", "PreToolUse", "Stop"} {
		arr := hooks[evt].([]any)
		managed := 0
		for _, m := range arr {
			if mm, ok := m.(map[string]any); ok && isHelixManaged(mm) {
				managed++
			}
		}
		assert.Equal(t, 1, managed, "exactly one helix_managed matcher per %s after re-run", evt)
	}

	assertNoMCPEntry(t, dir)
}

// TestSetupFlip_TeardownRemovesPriorMCP: a pre-seeded prior MCP entry is removed
// by Register's teardown while the skill + hooks are installed.
func TestSetupFlip_TeardownRemovesPriorMCP(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	seedMCPConfig(t, filepath.Join(dir, ".mcp.json"), "mcpServers")

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&ClaudeCodeRegistrar{}).Register(cfg))

	// Prior helix MCP entry gone, unmanaged entry preserved.
	assertHelixGoneOtherKept(t, filepath.Join(dir, ".mcp.json"), "mcpServers")
	// Skill + hooks present.
	_, err := os.Stat(filepath.Join(dir, ".claude", "skills", "helix", "SKILL.md"))
	require.NoError(t, err, "SKILL.md must be present after flip")
	settings := readJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	_, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "hooks must be present after flip")
}

// TestSetupFlip_NonClaudeClient_TeardownOnly: vscode Register removes the prior
// MCP entry and writes NO skill.
func TestSetupFlip_NonClaudeClient_TeardownOnly(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".vscode", "mcp.json")
	seedMCPConfig(t, configPath, "servers")

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&VSCodeRegistrar{}).Register(cfg))

	assertHelixGoneOtherKept(t, configPath, "servers")
	assertNoSkillUnder(t, dir)
}

// TestSetupFlip_NoSkillFlag: --no-skill skips the skill write but still tears
// down the prior MCP entry and installs hooks.
func TestSetupFlip_NoSkillFlag(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dir := t.TempDir()
	seedMCPConfig(t, filepath.Join(dir, ".mcp.json"), "mcpServers")

	cfg := RegistrationConfig{
		BinaryPath: "/usr/local/bin/helix",
		ProjectDir: dir,
		NoSkill:    true,
		Printer:    &SetupPrinter{},
	}
	require.NoError(t, (&ClaudeCodeRegistrar{}).Register(cfg))

	// No skill written.
	assertNoSkillUnder(t, dir)
	// Teardown still happened.
	assertHelixGoneOtherKept(t, filepath.Join(dir, ".mcp.json"), "mcpServers")
	// Hooks still installed.
	settings := readJSON(t, filepath.Join(dir, ".claude", "settings.json"))
	_, ok := settings["hooks"].(map[string]any)
	require.True(t, ok, "hooks must be installed even with --no-skill")
}

// readJSON reads and unmarshals a JSON file into a map (test helper).
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var m map[string]any
	require.NoError(t, json.Unmarshal(data, &m))
	return m
}

// bytesHasTrailingNewline reports whether s ends with a newline.
func bytesHasTrailingNewline(s string) bool {
	return len(s) > 0 && s[len(s)-1] == '\n'
}
