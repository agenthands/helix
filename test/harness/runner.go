//go:build integration || llm || llmjudge

package harness

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
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

// RunnerOptions configures a test Runner instance.
type RunnerOptions struct {
	// WorkspaceDir is the project path to activate after startup.
	WorkspaceDir string
	// SkipLS skips waiting for language server readiness.
	SkipLS bool
	// LSTimeout is the maximum time to wait for LS readiness (default 30s).
	LSTimeout time.Duration
	// Profile overrides cfg.Profile (default "full" preserves existing behavior).
	Profile string
	// Mode, if non-empty, triggers a switch_mode call after session connect.
	// Default "" uses the profile's default_mode (no switch).
	Mode string
	// MaxWorkers overrides cfg.WorkerPool.MaxWorkers (default 4).
	MaxWorkers int
}

// Runner wraps a daemon instance with an MCP client session for integration tests.
type Runner struct {
	Daemon  *daemon.Daemon
	Session *mcp.ClientSession
	cancel  context.CancelFunc
	tb      testing.TB
}

// Stop cancels the daemon context, which stops the kernel Run goroutine
// and triggers pool.stopAll to shut down all LS worker processes.
func (r *Runner) Stop() {
	r.cancel()
}

// NewHTTPSession creates an MCP client session over HTTP transport.
// Uses httptest.NewServer + StreamableClientTransport to validate the full
// HTTP serialization path.
func (r *Runner) NewHTTPSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ts := httptest.NewServer(r.Daemon.MCPServer().HTTPHandler())
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

// StartRunner creates a daemon in-process, wires an MCP client via
// InMemoryTransports, and optionally activates a workspace with LS readiness wait.
func StartRunner(tb testing.TB, opts RunnerOptions) *Runner {
	tb.Helper()

	cfg := DefaultTestConfig(tb)
	if opts.Profile != "" {
		cfg.Profile = opts.Profile
	}
	if opts.Mode != "" {
		cfg.Mode = opts.Mode
	}
	if opts.MaxWorkers > 0 {
		cfg.WorkerPool.MaxWorkers = opts.MaxWorkers
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Initialize skills with temp dirs.
	tmpDir := tb.TempDir()
	projectDir := filepath.Join(tmpDir, ".serena")
	globalDir := filepath.Join(tmpDir, ".serena-global")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		tb.Fatalf("creating project dir: %v", err)
	}
	if err := os.MkdirAll(globalDir, 0o755); err != nil {
		tb.Fatalf("creating global dir: %v", err)
	}
	deps := skill.SkillDeps{
		ProjectDir: projectDir,
		GlobalDir:  globalDir,
		Logger:     logger,
	}
	if err := skill.InitAll(deps); err != nil {
		tb.Fatalf("skill.InitAll: %v", err)
	}

	d, err := daemon.New(cfg, logger)
	if err != nil {
		tb.Fatalf("daemon.New: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	// Start kernel in background goroutine.
	go func() {
		if err := d.KernelInstance().Run(ctx); err != nil && ctx.Err() == nil {
			tb.Errorf("kernel.Run exited with error: %v", err)
		}
	}()

	// Create in-memory transports for MCP protocol.
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	// Connect server side first (required by MCP SDK).
	_, err = d.MCPServer().SDK().Connect(ctx, serverTransport, nil)
	if err != nil {
		cancel()
		tb.Fatalf("server Connect: %v", err)
	}

	// Create and connect client.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-client",
		Version: "1.0",
	}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		tb.Fatalf("client Connect: %v", err)
	}

	r := &Runner{
		Daemon:  d,
		Session: session,
		cancel:  cancel,
		tb:      tb,
	}

	tb.Cleanup(r.Stop)

	// Activate workspace if requested.
	if opts.WorkspaceDir != "" {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "activate_project",
			Arguments: map[string]any{"repo_path": opts.WorkspaceDir},
		})
		if err != nil {
			tb.Fatalf("activate_project: %v", err)
		}
		if result.IsError {
			tb.Fatalf("activate_project failed: %s", TextContent(result))
		}

		if !opts.SkipLS {
			timeout := opts.LSTimeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			WaitForLS(tb, session, timeout)
		}
	}

	return r
}

// WaitForLS polls search_symbols until the language server has indexed the workspace.
// It fails the test if the timeout is exceeded.
func WaitForLS(tb testing.TB, session *mcp.ClientSession, timeout time.Duration) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	var lastErr string
	for {
		select {
		case <-ctx.Done():
			tb.Fatalf("LS readiness timeout after %v (last: %s)", timeout, lastErr)
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
				lastErr = fmt.Sprintf("tool error: %s", TextContent(result))
				continue
			}
			text := TextContent(result)
			if len(result.Content) > 0 && text != "" && text != "(no results)" {
				tb.Logf("LS ready: %s", text[:min(len(text), 80)])
				return
			}
			lastErr = fmt.Sprintf("no results yet: %q", text)
		}
	}
}

// ProjectRoot returns the repository root directory by walking up from this source file.
// This ensures testdata paths resolve correctly regardless of the working directory.
func ProjectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine project root via runtime.Caller")
	}
	// file is .../test/harness/runner.go -> go up 3 levels
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

// DefaultTestConfig creates a minimal config for integration tests.
func DefaultTestConfig(tb testing.TB) *config.SerenaConfig {
	tb.Helper()
	tmpDir := tb.TempDir()
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = filepath.Join(tmpDir, "s.sock")
	cfg.Daemon.HTTPAddr = ""
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	cfg.WorkerPool.MaxWorkers = 4
	cfg.WorkerPool.BaseTTL = 30
	return cfg
}
