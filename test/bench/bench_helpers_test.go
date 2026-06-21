package bench_test

// bench_helpers.go provides TB-generic test-daemon startup and fixture wiring
// for the test/bench/... benchmark package.
//
// Why not import test/integration? That package is gated by `//go:build
// integration` and lives under `package integration_test`, so it cannot be
// imported as a library. Instead we duplicate the ~100 lines of minimal daemon
// startup logic here — using the same public types from internal/daemon,
// internal/config, internal/skill, and the MCP SDK that the integration
// harness consumes. This keeps the integration build tag unchanged and lets
// `go test -bench=. ./test/bench/...` work with zero special flags
// (per 09-RESEARCH.md Pitfall 2 — "no build tag on bench files").
//
// All helpers accept `testing.TB` so the same wrappers work inside
// `Benchmark*` and `Test*` functions (e.g. TestBenchToolsManifestMatchesRegistry).

import (
	"context"
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
	"github.com/agenthands/helix/internal/skill"

	// Blank imports trigger skill registration via init() (Caddy-style).
	// Must match the set in test/integration/harness.go so the bench daemon
	// exposes the same 41 tools as the integration harness.
	_ "github.com/agenthands/helix/internal/kernel/diag"
	_ "github.com/agenthands/helix/internal/kernel/edit"
	_ "github.com/agenthands/helix/internal/kernel/fileops"
	_ "github.com/agenthands/helix/internal/kernel/symbols"
	_ "github.com/agenthands/helix/internal/profile"
	_ "github.com/agenthands/helix/internal/skill/memory"
	_ "github.com/agenthands/helix/internal/skill/workflow"
)

// benchDaemon wraps a daemon instance with an MCP client session for bench
// scaffolding. It mirrors integration_test.TestDaemon but without the build
// tag and without any *testing.T-only fields.
type benchDaemon struct {
	daemon  *daemon.Daemon
	Session *mcp.ClientSession
	cancel  context.CancelFunc
	tb      testing.TB
}

// Stop cancels the daemon context, which stops the kernel Run goroutine and
// triggers pool shutdown for all LS worker processes.
func (bd *benchDaemon) Stop() {
	bd.cancel()
}

// RegistryNames returns the unfiltered list of tool names registered with the
// daemon's MCP server. This is the canonical 38-tool set and bypasses the
// ProfileFilterMiddleware that hides tools based on mode AllowedTools.
// TestBenchToolsManifestMatchesRegistry uses this to assert D-04 parity.
func (bd *benchDaemon) RegistryNames() []string {
	return bd.daemon.MCPServer().Registry().Names()
}

// BriefDescriptions returns a map of tool name -> BriefDescription for all
// registered tools. Used by golden-file description tests (DESC-03).
func (bd *benchDaemon) BriefDescriptions() map[string]string {
	return bd.daemon.MCPServer().Registry().BriefDescriptions()
}

// startBenchDaemon creates a daemon in-process, wires an MCP client via
// InMemoryTransports, and registers cleanup via tb.Cleanup. Callers MUST NOT
// invoke this inside a b.Loop() body — one daemon per bench function, shared
// across iterations (09-RESEARCH.md Pattern 2 + Pitfall 6).
func startBenchDaemon(tb testing.TB) *benchDaemon {
	tb.Helper()

	cfg := defaultBenchConfig(tb)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	// Initialize skills with temp dirs so each benchmark function gets a
	// fresh memory store / onboarding scratchpad.
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
	if _, err := d.MCPServer().SDK().Connect(ctx, serverTransport, nil); err != nil {
		cancel()
		tb.Fatalf("server Connect: %v", err)
	}

	// Create and connect client.
	client := mcp.NewClient(&mcp.Implementation{
		Name:    "bench-client",
		Version: "1.0",
	}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	if err != nil {
		cancel()
		tb.Fatalf("client Connect: %v", err)
	}

	bd := &benchDaemon{
		daemon:  d,
		Session: session,
		cancel:  cancel,
		tb:      tb,
	}

	// Register cleanup once, at the bench-function boundary (Pitfall 6:
	// never register inside b.Loop — cleanups accumulate across iterations).
	tb.Cleanup(bd.Stop)

	return bd
}

// defaultBenchConfig returns a minimal config for benchmark daemons. Profile
// "full" exposes all 41 tools so the manifest parity test can compare against
// the unfiltered registry.
func defaultBenchConfig(tb testing.TB) *config.SerenaConfig {
	tb.Helper()
	tmpDir := tb.TempDir()
	cfg := &config.SerenaConfig{}
	cfg.Daemon.SocketPath = filepath.Join(tmpDir, "s.sock")
	cfg.Daemon.ShutdownTimeout = 2
	cfg.Profile = "full"
	// MaxWorkers must accommodate the edit-tool sub-benchmarks in Plan 09-03,
	// which re-activate a fresh fixture copy per iteration. Each activation
	// can hold an LS worker until the previous workspace's worker is evicted
	// by adaptive TTL; a ceiling of 2 exhausts the pool almost immediately
	// when -benchtime=Nx grows past the smoke value. 50 gives headroom for
	// baseline -count=10 runs while still bounding resource use.
	cfg.WorkerPool.MaxWorkers = 256
	cfg.WorkerPool.BaseTTL = 5
	return cfg
}

// prepareGoFixtureB returns the absolute path to the repo's shared read-only
// testdata/fixtures/go/ directory WITHOUT copying.
//
// Benchmarks that only READ the fixture (all symbol/diagnostic/file-read
// tools) prefer this helper because copying the fixture into tb.TempDir() for
// every iteration adds CoW cost that pollutes the measurement (09-RESEARCH.md
// Pitfall 6).
//
// Edit-tool benchmarks — replace_symbol_body, insert_before_symbol,
// insert_after_symbol, rename_symbol, safe_delete_symbol, verify_edit — MUST
// use prepareGoFixtureCopyB instead, because mutating the shared fixture
// corrupts state for every other sub-benchmark in the same process.
func prepareGoFixtureB(tb testing.TB) string {
	tb.Helper()
	return filepath.Join(projectRoot(), "testdata", "fixtures", "go")
}

// prepareGoFixtureCopyB copies testdata/fixtures/go/ into tb.TempDir() and
// returns the absolute path to the copy. Directory structure is preserved and
// `.git` entries are skipped.
//
// Edit tool benchmarks (Plan 09-03) MUST use this helper, NOT prepareGoFixtureB,
// because tools like replace_symbol_body, insert_before_symbol,
// insert_after_symbol, rename_symbol, safe_delete_symbol, and verify_edit
// MUTATE files and would corrupt the shared read-only fixture. Plan 09-02
// revision addressed this fixture-mutation hazard by splitting the helper.
func prepareGoFixtureCopyB(tb testing.TB) string {
	tb.Helper()

	srcDir := filepath.Join(projectRoot(), "testdata", "fixtures", "go")
	dstDir := filepath.Join(tb.TempDir(), "go")

	err := filepath.WalkDir(srcDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		if rel == ".git" || rel == ".git/" {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
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
		tb.Fatalf("prepareGoFixtureCopyB: %v", err)
	}

	return dstDir
}

// activateWorkspaceB points the daemon at the given workspace directory and
// waits for LS readiness (up to 30s) by polling search_symbols for a known
// fixture symbol.
func activateWorkspaceB(tb testing.TB, bd *benchDaemon, workspaceDir string) {
	tb.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()

	result, err := bd.Session.CallTool(ctx, &mcp.CallToolParams{
		Name:      "activate_project",
		Arguments: map[string]any{"repo_path": workspaceDir},
	})
	if err != nil {
		tb.Fatalf("activate_project: %v", err)
	}
	if result.IsError {
		tb.Fatalf("activate_project failed: %s", benchTextContent(result))
	}

	deadline := time.Now().Add(30 * time.Second)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()
	var lastErr string
	for time.Now().Before(deadline) {
		res, callErr := bd.Session.CallTool(ctx, &mcp.CallToolParams{
			Name:      "search_symbols",
			Arguments: map[string]any{"query": "Helper"},
		})
		if callErr != nil {
			lastErr = callErr.Error()
		} else if res.IsError {
			lastErr = benchTextContent(res)
		} else {
			text := benchTextContent(res)
			if text != "" && text != "(no results)" {
				return
			}
			lastErr = "no results yet: " + text
		}
		<-ticker.C
	}
	tb.Fatalf("LS readiness timeout after 30s (last: %s)", lastErr)
}

// callToolB invokes an MCP tool and fatals on any error (protocol or tool).
// Callers get the raw *mcp.CallToolResult; retries are NOT performed — the
// intent is to observe the real tool latency for the baseline.
func callToolB(tb testing.TB, session *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	tb.Helper()
	result, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	})
	if err != nil {
		tb.Fatalf("tool %s call error: %v", name, err)
	}
	if result.IsError {
		tb.Fatalf("tool %s failed: %s", name, benchTextContent(result))
	}
	return result
}

// listSessionToolsB returns the names of all tools visible to the given MCP
// session. Used by TestBenchToolsManifestMatchesRegistry to enforce 38-tool
// parity between benchTools and the live registry.
func listSessionToolsB(tb testing.TB, session *mcp.ClientSession) []string {
	tb.Helper()
	result, err := session.ListTools(context.Background(), &mcp.ListToolsParams{})
	if err != nil {
		tb.Fatalf("tools/list: %v", err)
	}
	names := make([]string, 0, len(result.Tools))
	for _, tool := range result.Tools {
		names = append(names, tool.Name)
	}
	return names
}

// requireGoplsB skips the test or benchmark if gopls is not installed.
// Benchmarks that exercise Go LSP tools call this as their first line.
func requireGoplsB(tb testing.TB) {
	tb.Helper()
	if _, err := exec.LookPath("gopls"); err != nil {
		tb.Skipf("gopls not installed: %v", err)
	}
}

// benchTextContent extracts the text from the first TextContent element in a
// CallToolResult. Returns empty string if no TextContent is found.
func benchTextContent(r *mcp.CallToolResult) string {
	for _, c := range r.Content {
		if tc, ok := c.(*mcp.TextContent); ok {
			return tc.Text
		}
	}
	return ""
}

// projectRoot returns the repository root directory by walking up from this
// source file. This mirrors test/integration/harness.go's projectRoot so bench
// helpers resolve testdata paths correctly regardless of the working directory.
func projectRoot() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		panic("cannot determine project root via runtime.Caller")
	}
	// file is .../test/bench/bench_helpers.go -> go up 3 levels to repo root.
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}
