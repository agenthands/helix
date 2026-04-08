//go:build integration

package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/postfix/serena/internal/config"
	"github.com/postfix/serena/internal/daemon"
	"github.com/postfix/serena/internal/skill"

	// Blank imports trigger skill registration via init() (Caddy-style).
	_ "github.com/postfix/serena/internal/kernel/diag"
	_ "github.com/postfix/serena/internal/kernel/edit"
	_ "github.com/postfix/serena/internal/kernel/fileops"
	_ "github.com/postfix/serena/internal/kernel/symbols"
	_ "github.com/postfix/serena/internal/profile"
	_ "github.com/postfix/serena/internal/skill/memory"
	_ "github.com/postfix/serena/internal/skill/workflow"
)

// Options configures a test daemon instance.
type Options struct {
	// WorkspaceDir is the project path to activate after startup.
	WorkspaceDir string
	// SkipLS skips waiting for language server readiness.
	SkipLS bool
	// LSTimeout is the maximum time to wait for LS readiness (default 30s).
	LSTimeout time.Duration
}

// TestDaemon wraps a daemon instance with an MCP client session for integration tests.
type TestDaemon struct {
	daemon  *daemon.Daemon
	Session *mcp.ClientSession
	cancel  context.CancelFunc
	t       *testing.T
}

// Stop cancels the daemon context, which stops the kernel Run goroutine
// and triggers pool.stopAll to shut down all LS worker processes.
func (td *TestDaemon) Stop() {
	td.cancel()
}

// NewHTTPSession creates an MCP client session over HTTP transport.
// Uses httptest.NewServer + StreamableClientTransport to validate the full
// HTTP serialization path (D-03/D-04).
func (td *TestDaemon) NewHTTPSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ts := httptest.NewServer(td.daemon.MCPServer().HTTPHandler())
	t.Cleanup(ts.Close)

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-http",
		Version: "1.0",
	}, nil)
	session, err := client.Connect(context.Background(), &mcp.StreamableClientTransport{
		Endpoint: ts.URL,
	}, nil)
	if err != nil {
		t.Fatalf("HTTP client Connect: %v", err)
	}
	t.Cleanup(func() { session.Close() })
	return session
}

// StartTestDaemon creates a daemon in-process, wires an MCP client via
// InMemoryTransports, and optionally activates a workspace with LS readiness wait.
func StartTestDaemon(t *testing.T, opts Options) *TestDaemon {
	t.Helper()

	cfg := defaultTestConfig(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Initialize skills with temp dirs.
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, ".serena")
	globalDir := filepath.Join(tmpDir, ".serena-global")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatalf("creating project dir: %v", err)
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		t.Fatalf("creating global dir: %v", err)
	}
	deps := skill.SkillDeps{
		ProjectDir: projectDir,
		GlobalDir:  globalDir,
		Logger:     logger,
	}
	if err := skill.InitAll(deps); err != nil {
		t.Fatalf("skill.InitAll: %v", err)
	}

	d, err := daemon.New(cfg, logger)
	if err != nil {
		t.Fatalf("daemon.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start kernel in background goroutine.
	go func() {
		_ = d.KernelInstance().Run(ctx)
	}()

	// Create in-memory transports for MCP protocol.
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	// Connect server side first (required by MCP SDK).
	_, err = d.MCPServer().SDK().Connect(ctx, serverTransport, nil)
	if err != nil {
		cancel()
		t.Fatalf("server Connect: %v", err)
	}

	// Create and connect client.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0",
	}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		t.Fatalf("client Connect: %v", err)
	}

	td := &TestDaemon{
		daemon:  d,
		Session: session,
		cancel:  cancel,
		t:       t,
	}

	t.Cleanup(td.Stop)

	// Activate workspace if requested.
	if opts.WorkspaceDir != "" {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "activate_project",
			Arguments: map[string]any{"repo_path": opts.WorkspaceDir},
		})
		if err != nil {
			t.Fatalf("activate_project: %v", err)
		}
		if result.IsError {
			t.Fatalf("activate_project failed: %s", textContent(result))
		}

		if !opts.SkipLS {
			timeout := opts.LSTimeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			WaitForLS(t, session, timeout)
		}
	}

	return td
}

// WaitForLS polls search_symbols until the language server has indexed the workspace.
// It fails the test if the timeout is exceeded.
func WaitForLS(t *testing.T, session *mcp.ClientSession, timeout time.Duration) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr string
	for {
		select {
		case <-ctx.Done():
			t.Fatalf("LS readiness timeout after %v (last: %s)", timeout, lastErr)
		case <-ticker.C:
			result, err := session.CallTool(ctx, &mcp.CallToolParams{
				Name:      "search_symbols",
				Arguments: map[string]any{"query": "main"},
			})
			if err != nil {
				lastErr = fmt.Sprintf("call error: %v", err)
				continue
			}
			if result.IsError {
				lastErr = fmt.Sprintf("tool error: %s", textContent(result))
				continue
			}
			// Check that we got actual symbol results, not "(no results)".
			text := textContent(result)
			if len(result.Content) > 0 && text != "" && text != "(no results)" {
				t.Logf("LS ready: %s", text[:min(len(text), 80)])
				return
			}
			lastErr = fmt.Sprintf("no results yet: %q", text)
		}
	}
}

// requireGopls skips the test if gopls is not installed.
func requireGopls(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("gopls"); err != nil {
		t.Skip("gopls not installed, skipping LSP integration test")
	}
}

// PrepareFixture copies testdata/fixtures/{lang}/ to a temp directory and returns
// the destination path. Each test gets its own copy to prevent cross-test contamination.
func PrepareFixture(t *testing.T, lang string) string {
	t.Helper()

	srcDir := filepath.Join(projectRoot(), "testdata", "fixtures", lang)
	dstDir := filepath.Join(t.TempDir(), lang)

	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		dst := filepath.Join(dstDir, rel)
		if d.IsDir() {
			return os.MkdirAll(dst, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dst, data, 0o644)
	})
	if err != nil {
		t.Fatalf("PrepareFixture(%s): %v", lang, err)
	}

	return dstDir
}

// projectRoot returns the repository root directory by walking up from this source file.
// This ensures testdata paths resolve correctly regardless of the working directory.
func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine project root via runtime.Caller")
	}
	// file is .../test/integration/harness.go -> go up 3 levels
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// defaultTestConfig creates a minimal config for integration tests.
func defaultTestConfig(t *testing.T) *config.SerenaConfig {
	t.Helper()
	tmpDir := t.TempDir()
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = filepath.Join(tmpDir, "s.sock")
	cfg.Daemon.HTTPAddr = ""
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	cfg.WorkerPool.MaxWorkers = 2
	cfg.WorkerPool.BaseTTL = 30
	return cfg
}
