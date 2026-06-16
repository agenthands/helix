package fileops

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
	tracenoop "go.opentelemetry.io/otel/trace/noop"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/workspace"
)

// buildFileopsServer registers the fileops tools against a kernel built with
// the given config. The replace_in_file / fuzzy_edit handlers operate on a
// tempdir filesystem (rootFn) and acquire no LSP lease, so this harness needs
// no live language server.
func buildFileopsServer(t *testing.T, cfg kernel.KernelConfig, root string) *mcp.SerenaMCPServer {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsReg := workspace.NewRegistry()
	langReg, err := langregistry.NewRegistry()
	if err != nil {
		t.Fatalf("langregistry.NewRegistry: %v", err)
	}
	installer := langregistry.NewInstaller(langregistry.InstallerConfig{}, logger)
	k := kernel.NewKernel(wsReg, langReg, installer, cfg, nil, logger, lspool.NoopSink{}, nil)

	srv := mcp.NewSerenaMCPServer(wsReg, logger, nil)
	rootFn := func() string { return root }
	wsKeyFn := func() workspace.WorkspaceKey { return workspace.WorkspaceKey{RepoRoot: root} }
	tracer := tracenoop.NewTracerProvider().Tracer("test")
	RegisterTools(srv, k, rootFn, wsKeyFn, tracer)
	return srv
}

func callFileopsTool(t *testing.T, srv *mcp.SerenaMCPServer, name string, args map[string]any) *mcpsdk.CallToolResult {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientT, serverT := mcpsdk.NewInMemoryTransports()
	serverErr := make(chan error, 1)
	go func() { serverErr <- srv.SDK().Run(ctx, serverT) }()

	client := mcpsdk.NewClient(&mcpsdk.Implementation{Name: "test-client", Version: "v0"}, nil)
	cs, err := client.Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("client connect: %v", err)
	}
	defer cs.Close()

	res, err := cs.CallTool(ctx, &mcpsdk.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("CallTool(%s): %v", name, err)
	}
	return res
}

func toolResultText(res *mcpsdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// TestReplaceInFileNoFuzzyWhenDisabled asserts ABLATE-07 / D-05: under
// DisableStructuredEditSubsystem, replace_in_file does exact-match only — a
// literal no-match falls through to the plain "0 replacement(s) made" path
// with no fuzzy cascade (no match_strategy line). Flag-off fuzzy behavior is
// preserved; exact matches still replace under the flag.
func TestReplaceInFileNoFuzzyWhenDisabled(t *testing.T) {
	// Source has trailing whitespace, so a clean literal pattern misses on the
	// exact pass but the whitespace-normalized fuzzy tier would match.
	const dirty = "func hello() {  \n\tfmt.Println(\"hello\")  \n}\n"
	const cleanPattern = "func hello() {\n\tfmt.Println(\"hello\")\n}"

	write := func(t *testing.T, root string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(root, "test.go"), []byte(dirty), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
	}

	t.Run("flag ON: literal no-match returns plain path, no fuzzy", func(t *testing.T) {
		root := t.TempDir()
		write(t, root)
		srv := buildFileopsServer(t, kernel.KernelConfig{DisableStructuredEditSubsystem: true}, root)

		res := callFileopsTool(t, srv, "replace_in_file", map[string]any{
			"path": "test.go", "pattern": cleanPattern, "replacement": "replacement",
		})
		text := toolResultText(res)
		if res.IsError {
			t.Fatalf("unexpected error result: %s", text)
		}
		if !strings.Contains(text, "0 replacement(s) made") {
			t.Errorf("expected plain '0 replacement(s) made' path, got: %s", text)
		}
		if strings.Contains(text, "match_strategy:") {
			t.Errorf("fuzzy cascade ran under the flag (match_strategy present): %s", text)
		}
		// File must be unmodified.
		got, _ := os.ReadFile(filepath.Join(root, "test.go"))
		if string(got) != dirty {
			t.Errorf("file modified under exact-match-only flag: %q", string(got))
		}
	})

	t.Run("flag OFF: fuzzy cascade preserved", func(t *testing.T) {
		root := t.TempDir()
		write(t, root)
		srv := buildFileopsServer(t, kernel.KernelConfig{}, root)

		res := callFileopsTool(t, srv, "replace_in_file", map[string]any{
			"path": "test.go", "pattern": cleanPattern, "replacement": "replacement",
		})
		text := toolResultText(res)
		if res.IsError {
			t.Fatalf("unexpected error result: %s", text)
		}
		// Existing behavior: the whitespace-normalized fuzzy tier matches and a
		// match_strategy line appears.
		if !strings.Contains(text, "match_strategy:") {
			t.Errorf("expected fuzzy cascade with flag OFF (match_strategy line), got: %s", text)
		}
	})

	t.Run("flag ON: exact match still replaces", func(t *testing.T) {
		root := t.TempDir()
		const exact = "func hello() {}\n"
		if err := os.WriteFile(filepath.Join(root, "ex.go"), []byte(exact), 0o644); err != nil {
			t.Fatalf("write: %v", err)
		}
		srv := buildFileopsServer(t, kernel.KernelConfig{DisableStructuredEditSubsystem: true}, root)

		res := callFileopsTool(t, srv, "replace_in_file", map[string]any{
			"path": "ex.go", "pattern": "func hello() {}", "replacement": "func world() {}",
		})
		text := toolResultText(res)
		if res.IsError {
			t.Fatalf("unexpected error result: %s", text)
		}
		if !strings.Contains(text, "1 replacement(s) made") {
			t.Errorf("expected exact match to replace under the flag, got: %s", text)
		}
		got, _ := os.ReadFile(filepath.Join(root, "ex.go"))
		if !strings.Contains(string(got), "world") {
			t.Errorf("exact replacement not applied: %q", string(got))
		}
	})
}

// TestFuzzyEditDisabled asserts the fuzzy_edit structured-edit tool returns
// serr.Unsupported under the flag (ABLATE-07 D-04). fuzzy_edit lives in the
// fileops package, so its guard coverage belongs here alongside the
// replace_in_file behavior.
func TestFuzzyEditDisabled(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "f.go"), []byte("func a(){}\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	t.Run("flag ON returns Unsupported", func(t *testing.T) {
		srv := buildFileopsServer(t, kernel.KernelConfig{DisableStructuredEditSubsystem: true}, root)
		res := callFileopsTool(t, srv, "fuzzy_edit", map[string]any{
			"path": "f.go", "search": "func a(){}", "replacement": "func b(){}",
		})
		text := toolResultText(res)
		if !res.IsError {
			t.Fatalf("expected error result under flag, got: %s", text)
		}
		if !strings.Contains(text, "subsystem_disabled:") {
			t.Errorf("missing subsystem_disabled: prefix: %s", text)
		}
		if !strings.Contains(text, string(serr.Unsupported)) {
			t.Errorf("missing %q kind: %s", serr.Unsupported, text)
		}
	})

	t.Run("flag OFF does not hit the guard", func(t *testing.T) {
		srv := buildFileopsServer(t, kernel.KernelConfig{}, root)
		res := callFileopsTool(t, srv, "fuzzy_edit", map[string]any{
			"path": "f.go", "search": "func a(){}", "replacement": "func b(){}",
		})
		if strings.Contains(toolResultText(res), "subsystem_disabled:") {
			t.Errorf("guard hit with flag OFF: %s", toolResultText(res))
		}
	})
}
