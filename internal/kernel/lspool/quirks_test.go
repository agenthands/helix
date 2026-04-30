package lspool

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/agenthands/helix/internal/langregistry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRustAnalyzerAdapter_ServerStatusReadiness verifies the experimental/serverStatus
// notification handler flips the readiness flag correctly and that WaitUntilRenameReady
// observes quiescent transitions. This backs BUG-02 / Plan 01 Task 2.
func TestRustAnalyzerAdapter_ServerStatusReadiness(t *testing.T) {
	r := &RustAnalyzerAdapter{}
	handlers := r.NotificationHandlers()
	handler, ok := handlers["experimental/serverStatus"]
	require.True(t, ok, "rust-analyzer adapter must register experimental/serverStatus handler")

	// Not ready before any notification.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	assert.False(t, r.WaitUntilRenameReady(ctx), "adapter should not be ready before any serverStatus notification")

	// Becomes ready on quiescent=true.
	handler(json.RawMessage(`{"health":"ok","quiescent":true}`))
	assert.True(t, r.WaitUntilRenameReady(context.Background()), "adapter should be ready after quiescent=true")

	// Non-quiescent resets the readiness.
	handler(json.RawMessage(`{"health":"ok","quiescent":false}`))
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	assert.False(t, r.WaitUntilRenameReady(ctx2), "adapter should reset to not-ready after quiescent=false")

	// Flip back to ready on a second quiescent=true.
	handler(json.RawMessage(`{"health":"ok","quiescent":true}`))
	assert.True(t, r.WaitUntilRenameReady(context.Background()), "adapter should become ready again after second quiescent=true")
}

// TestRustAnalyzerAdapter_ServerStatusMalformed verifies malformed payloads
// do not panic and leave state unchanged (T-47-01 in threat model).
func TestRustAnalyzerAdapter_ServerStatusMalformed(t *testing.T) {
	r := &RustAnalyzerAdapter{}
	handler := r.NotificationHandlers()["experimental/serverStatus"]
	require.NotNil(t, handler)
	assert.NotPanics(t, func() {
		handler(json.RawMessage(`not-json`))
	})
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	assert.False(t, r.WaitUntilRenameReady(ctx), "malformed payload must not mark adapter ready")
}

// TestRustAnalyzerAdapter_ImplementsQuirkAdapter guarantees the base interface is
// still structurally satisfied (no required-method additions).
func TestRustAnalyzerAdapter_ImplementsQuirkAdapter(t *testing.T) {
	var _ QuirkAdapter = (*RustAnalyzerAdapter)(nil)
}

// TestRustAnalyzerAdapter_ExperimentalCapabilities verifies the optional
// ExperimentalCapabilities interface is implemented and advertises
// serverStatusNotification=true.
func TestRustAnalyzerAdapter_ExperimentalCapabilities(t *testing.T) {
	var r QuirkAdapter = &RustAnalyzerAdapter{}
	ec, ok := r.(ExperimentalCapabilities)
	require.True(t, ok, "RustAnalyzerAdapter must implement ExperimentalCapabilities")
	caps := ec.ExperimentalCapabilities()
	assert.Equal(t, true, caps["serverStatusNotification"])
}

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

func TestQuirkAdapter_JdtlsExtraArgs(t *testing.T) {
	tmpDir := t.TempDir()
	entry := langregistry.LSEntry{Language: "java", Command: "jdtls"}
	adapter := &JdtlsAdapter{Entry: entry}

	args := adapter.ExtraArgs(tmpDir, nil)
	require.Len(t, args, 2)
	assert.Equal(t, "-data", args[0])
	assert.Equal(t, filepath.Join(tmpDir, ".jdtls-data"), args[1])

	// Verify directory was created.
	info, err := os.Stat(filepath.Join(tmpDir, ".jdtls-data"))
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

// TestJdtlsAdapter_ExtraArgs_DefaultDataDir verifies the production codepath when
// SERENA_TEST_JDTLS_DATA_DIR is unset: the adapter prepends `-data <workDir>/.jdtls-data`
// and creates that dir on disk. Backs Phase 48 BUG-03.
func TestJdtlsAdapter_ExtraArgs_DefaultDataDir(t *testing.T) {
	// Ensure env is unset (some shells may leak in CI).
	t.Setenv("SERENA_TEST_JDTLS_DATA_DIR", "")
	// t.Setenv with empty string still sets the var; explicitly unset for the "unset" branch
	// by using os.Unsetenv. t.Setenv guarantees auto-restore for the empty value, then
	// we unset; both branches share fallback semantics so behavior is identical.
	require.NoError(t, os.Unsetenv("SERENA_TEST_JDTLS_DATA_DIR"))

	workDir := t.TempDir()
	adapter := &JdtlsAdapter{}
	got := adapter.ExtraArgs(workDir, []string{"tail1", "tail2"})

	expectedDir := filepath.Join(workDir, ".jdtls-data")
	require.Equal(t, []string{"-data", expectedDir, "tail1", "tail2"}, got)

	st, err := os.Stat(expectedDir)
	require.NoError(t, err, "default data dir must be created on disk")
	assert.True(t, st.IsDir(), "default data dir must be a directory")
}

// TestJdtlsAdapter_ExtraArgs_EnvOverride verifies the test-only override branch:
// when SERENA_TEST_JDTLS_DATA_DIR is set, that path is used verbatim and created.
func TestJdtlsAdapter_ExtraArgs_EnvOverride(t *testing.T) {
	overridePath := filepath.Join(t.TempDir(), "warm")
	t.Setenv("SERENA_TEST_JDTLS_DATA_DIR", overridePath)

	workDir := t.TempDir()
	adapter := &JdtlsAdapter{}
	got := adapter.ExtraArgs(workDir, []string{"tail"})

	require.Equal(t, []string{"-data", overridePath, "tail"}, got)

	st, err := os.Stat(overridePath)
	require.NoError(t, err, "override data dir must be created on disk")
	assert.True(t, st.IsDir(), "override data dir must be a directory")

	// And the default workDir/.jdtls-data must NOT have been created.
	_, err = os.Stat(filepath.Join(workDir, ".jdtls-data"))
	assert.True(t, os.IsNotExist(err), "default dir must not be created when override is set")
}

// TestJdtlsAdapter_ExtraArgs_EnvEmptyFallsBack verifies that an explicitly-empty
// env var falls back to the default branch (treated identically to unset).
func TestJdtlsAdapter_ExtraArgs_EnvEmptyFallsBack(t *testing.T) {
	t.Setenv("SERENA_TEST_JDTLS_DATA_DIR", "")

	workDir := t.TempDir()
	adapter := &JdtlsAdapter{}
	got := adapter.ExtraArgs(workDir, nil)

	expectedDir := filepath.Join(workDir, ".jdtls-data")
	require.Equal(t, []string{"-data", expectedDir}, got)

	st, err := os.Stat(expectedDir)
	require.NoError(t, err)
	assert.True(t, st.IsDir())
}

// TestJdtlsAdapter_ExtraArgs_PreservesExtraArgs verifies argv composition:
// `-data <dir>` is prepended and the tail args are preserved in order.
func TestJdtlsAdapter_ExtraArgs_PreservesExtraArgs(t *testing.T) {
	require.NoError(t, os.Unsetenv("SERENA_TEST_JDTLS_DATA_DIR"))

	workDir := t.TempDir()
	adapter := &JdtlsAdapter{}
	got := adapter.ExtraArgs(workDir, []string{"extra", "flag"})

	expectedDir := filepath.Join(workDir, ".jdtls-data")
	require.Equal(t, []string{"-data", expectedDir, "extra", "flag"}, got)
}

// TestJdtlsAdapter_ImplementsQuirkAdapter is a compile-time guard that the
// adapter still satisfies QuirkAdapter after the Phase 56 readiness extension.
func TestJdtlsAdapter_ImplementsQuirkAdapter(t *testing.T) {
	var _ QuirkAdapter = (*JdtlsAdapter)(nil)
}

// TestJdtlsAdapter_LanguageStatusReadiness verifies the language/status handler
// is registered and that WaitUntilJavaReady requires BOTH ServiceReady AND
// ProjectStatus=OK signals (D-08). Backs Phase 56 Plan 03.
func TestJdtlsAdapter_LanguageStatusReadiness(t *testing.T) {
	j := &JdtlsAdapter{}
	handlers := j.NotificationHandlers()
	handler, ok := handlers["language/status"]
	require.True(t, ok, "jdtls adapter must register language/status handler")

	// Not ready before any notification.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.Error(t, j.WaitUntilJavaReady(ctx))

	// ServiceReady alone insufficient (D-08).
	handler(json.RawMessage(`{"type":"ServiceReady","message":"ServiceReady"}`))
	ctx2, cancel2 := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel2()
	require.Error(t, j.WaitUntilJavaReady(ctx2))

	// After ProjectStatus=OK both gates closed → success.
	handler(json.RawMessage(`{"type":"ProjectStatus","message":"OK"}`))
	ctx3, cancel3 := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel3()
	require.NoError(t, j.WaitUntilJavaReady(ctx3))
}

// TestJdtlsAdapter_LanguageStatusMalformed verifies malformed payloads do not
// panic and leave the readiness gate closed (T-56-07).
func TestJdtlsAdapter_LanguageStatusMalformed(t *testing.T) {
	j := &JdtlsAdapter{}
	handler := j.NotificationHandlers()["language/status"]
	require.NotPanics(t, func() {
		handler(json.RawMessage(`not json`))
		handler(json.RawMessage(`{"type":42,"message":null}`)) // wrong types
		handler(json.RawMessage(`{}`))
	})
	// Gate must stay closed.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	require.Error(t, j.WaitUntilJavaReady(ctx))
}

// TestJdtlsAdapter_WaitUntilJavaReady_ContextCancel verifies WaitUntilJavaReady
// honors a pre-cancelled context promptly without waiting for the internal
// javaReadinessTimeout.
func TestJdtlsAdapter_WaitUntilJavaReady_ContextCancel(t *testing.T) {
	j := &JdtlsAdapter{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel
	start := time.Now()
	err := j.WaitUntilJavaReady(ctx)
	require.Error(t, err)
	require.ErrorIs(t, err, context.Canceled)
	require.Less(t, time.Since(start), 100*time.Millisecond, "must honor pre-cancelled ctx promptly")
}
