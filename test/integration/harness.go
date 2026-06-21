package integration_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/daemon"
	"github.com/agenthands/helix/internal/kernel/lspool"
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

// Options configures a test daemon instance.
type Options struct {
	// WorkspaceDir is the project path to activate after startup.
	WorkspaceDir string
	// SkipLS skips waiting for language server readiness.
	SkipLS bool
	// LSTimeout is the maximum time to wait for LS readiness (default 30s).
	LSTimeout time.Duration
	// LSQuery overrides the workspace/symbol query used for readiness detection.
	// Default "main" works for Go/Java; use "helper" for Python/TypeScript/Rust.
	LSQuery string
	// Profile overrides cfg.Profile (default "full" preserves existing behavior).
	Profile string
	// Mode, if non-empty, triggers a switch_mode call after session connect.
	// Default "" uses the profile's default_mode (no switch).
	Mode string
	// MaxWorkers overrides cfg.WorkerPool.MaxWorkers (default 2 preserves existing behavior).
	MaxWorkers int
	// JdtlsDataDir, if non-empty, is exported via the HELIX_TEST_JDTLS_DATA_DIR
	// environment variable so that JdtlsAdapter.ExtraArgs uses it as jdtls's -data
	// path (shared warm workspace across test runs; Phase 48, BUG-03).
	JdtlsDataDir string
}

// TestDaemon wraps a daemon instance with an MCP client session for integration tests.
type TestDaemon struct {
	daemon  *daemon.Daemon
	Session *mcp.ClientSession
	cancel  context.CancelFunc
	tb      testing.TB
}

// Stop cancels the daemon context, which stops the kernel Run goroutine
// and triggers pool.stopAll to shut down all LS worker processes.
func (td *TestDaemon) Stop() {
	td.cancel()
}

// LSPoolWorker returns the warm LS worker for (language, workspaceDir) from
// the test daemon's pool. Test-only — used by Phase 56 integration tests to
// reach into adapter readiness state (TestRustAnalyzer_NotificationDispatchEndToEnd,
// waitJavaReady). Returns nil if no matching worker is currently in the pool.
func (td *TestDaemon) LSPoolWorker(language, workspaceDir string) *lspool.Worker {
	return td.daemon.KernelInstance().Pool().WorkerForTests(language, workspaceDir)
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
//
// NOTE: This intentionally keeps *testing.T (not testing.TB). Its callers are
// always tests (subtest-scoped transport smoke), never benchmarks, and its
// t.Cleanup semantics are tied to the concrete *testing.T lifecycle.
func (td *TestDaemon) NewGRPCSession(t *testing.T) *mcp.ClientSession {
	t.Helper()

	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	if _, err := td.daemon.MCPServer().SDK().Connect(ctx, serverTransport, nil); err != nil {
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

// StartTestDaemon creates a daemon in-process, wires an MCP client via
// InMemoryTransports, and optionally activates a workspace with LS readiness wait.
func StartTestDaemon(tb testing.TB, opts Options) *TestDaemon {
	tb.Helper()

	if opts.JdtlsDataDir != "" {
		tb.Setenv("HELIX_TEST_JDTLS_DATA_DIR", opts.JdtlsDataDir)
	}

	cfg := defaultTestConfig(tb)
	if opts.Profile != "" {
		cfg.Profile = opts.Profile
	}
	if opts.Mode != "" {
		// Set initial mode via config to bypass switch_mode transition rules.
		// switch_mode enforces allowed_mode_transitions (e.g., admin is unreachable),
		// but tests need to exercise every valid (profile, mode) seed state.
		cfg.Mode = opts.Mode
	}
	if opts.MaxWorkers > 0 {
		cfg.WorkerPool.MaxWorkers = opts.MaxWorkers
	}
	var logOut io.Writer = io.Discard
	if os.Getenv("HELIX_TEST_LS_DEBUG") != "" {
		logOut = os.Stderr
	}
	logger := slog.New(slog.NewTextHandler(logOut, &slog.HandlerOptions{Level: slog.LevelDebug}))

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
		_ = d.KernelInstance().Run(ctx)
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

	td := &TestDaemon{
		daemon:  d,
		Session: session,
		cancel:  cancel,
		tb:      tb,
	}

	tb.Cleanup(td.Stop)

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
			tb.Fatalf("activate_project failed: %s", textContent(result))
		}

		if !opts.SkipLS {
			timeout := opts.LSTimeout
			if timeout == 0 {
				timeout = 30 * time.Second
			}
			// Env-var override lets slow environments (CI cold-start, low-spec
			// machines) raise the LS-readiness limit without code changes. Local
			// dev stays fast-fail at the test-defined default.
			if v := os.Getenv("HELIX_TEST_LS_TIMEOUT"); v != "" {
				if d, err := time.ParseDuration(v); err == nil && d > timeout {
					timeout = d
				}
			}
			query := opts.LSQuery
			if query == "" {
				query = "main"
			}
			WaitForLS(tb, session, timeout, query)
		}
	}

	return td
}

// WaitForLS polls search_symbols until the language server has indexed the workspace.
// Requires non-empty results to ensure the LS has completed indexing — some servers
// (rust-analyzer) return empty results without error before indexing finishes.
// The query parameter should match a known symbol in the fixture (e.g. "main" for Go/Java,
// "helper" for Python/TypeScript/Rust).
func WaitForLS(tb testing.TB, session *mcp.ClientSession, timeout time.Duration, query string) {
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
				Arguments: map[string]any{"query": query},
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
				tb.Logf("LS ready: %s", text[:min(len(text), 80)])
				return
			}
			lastErr = fmt.Sprintf("no results yet: %q", text)
		}
	}
}

// requireGopls skips the test if gopls is not installed.
func requireGopls(tb testing.TB) {
	tb.Helper()
	if _, err := exec.LookPath("gopls"); err != nil {
		tb.Skip("gopls not installed, skipping LSP integration test")
	}
}

// PrepareFixture copies testdata/fixtures/{lang}/ to a temp directory and returns
// the destination path. Each test gets its own copy to prevent cross-test contamination.
func PrepareFixture(tb testing.TB, lang string) string {
	tb.Helper()

	srcDir := filepath.Join(projectRoot(), "testdata", "fixtures", lang)
	dstDir := filepath.Join(tb.TempDir(), lang)

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
		tb.Fatalf("PrepareFixture(%s): %v", lang, err)
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
func defaultTestConfig(tb testing.TB) *config.SerenaConfig {
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
