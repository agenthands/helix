//go:build integration || llm || llmjudge

package harness

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/skill"

	// Blank imports trigger skill registration via init() (Caddy-style).
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/profile"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/workflow"
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

// NewGRPCSession creates a fresh MCP client session against the SAME running
// daemon, over the retained in-memory transport that the gRPC StreamMCP wire also
// funnels into (mcpServer.SDK()).
//
// Phase 94 RETIRE-02: this replaces the former NewHTTPSession (httptest.NewServer
// + HTTPHandler), which exercised the deleted Streamable-HTTP /mcp head. Both the
// unix-socket gRPC wire and this in-memory transport drive the identical
// mcpServer.SDK() with all middlewares attached, so an in-memory second session
// is a faithful stand-in for "a new client reconnecting to the warm daemon".
func (r *Runner) NewGRPCSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	// Connect the server side first (required by the MCP SDK), against the same
	// daemon SDK server the original session uses.
	if _, err := r.Daemon.MCPServer().SDK().Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("gRPC-path server Connect: %v", err)
	}

	client := mcp.NewClient(&mcp.Implementation{
		Name:    "test-grpc",
		Version: "1.0",
	}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("gRPC-path client Connect: %v", err)
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
	projectDir := filepath.Join(tmpDir, ".helix")
	globalDir := filepath.Join(tmpDir, ".helix-global")
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

// WaitForLS polls search_symbols until the language server responds without error.
// A successful response (even with no results) indicates the LS is ready — some
// servers (typescript-language-server) return empty results for short queries but
// are fully operational. The check distinguishes transport/init errors from empty results.
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
				Arguments: map[string]any{"query": "e"},
			})
			if err != nil {
				lastErr = fmt.Sprintf("call error: %v", err)
				continue
			}
			if result.IsError {
				lastErr = fmt.Sprintf("tool error: %s", TextContent(result))
				continue
			}
			// LS responded without error — it's ready. Some LS implementations
			// (typescript-language-server) return empty results for short queries
			// but are fully operational for longer queries.
			text := TextContent(result)
			tb.Logf("LS ready: %s", text[:min(len(text), 80)])
			return
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
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	cfg.WorkerPool.MaxWorkers = 4
	cfg.WorkerPool.BaseTTL = 30
	return cfg
}
