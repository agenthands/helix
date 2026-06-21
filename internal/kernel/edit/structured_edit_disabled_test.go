package edit

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"
	"time"

	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/diag"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/mcp"
	"github.com/agenthands/helix/internal/treesitter"
	"github.com/agenthands/helix/internal/workspace"
)

// buildEditServer constructs an MCP server with the 6 edit tools registered
// against a kernel built with the given config. When cfg sets
// DisableStructuredEditSubsystem, the structured-edit handlers must short-
// circuit with serr.Unsupported before any LSP/session acquisition — so this
// harness needs no live language server.
func buildEditServer(t *testing.T, cfg kernel.KernelConfig, root string) *mcp.SerenaMCPServer {
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
	extractor := NewBodyExtractor(treesitter.NewGrammarRegistry())
	diagStore := diag.NewDiagnosticStore()
	wsKeyFn := func() workspace.WorkspaceKey { return workspace.WorkspaceKey{RepoRoot: root} }
	RegisterTools(srv, k, extractor, diagStore, wsKeyFn)
	return srv
}

// callEditTool invokes a registered tool via the SDK in-memory transport,
// mirroring runSkillToolViaSDK in internal/mcp/server_trace_test.go.
func callEditTool(t *testing.T, srv *mcp.SerenaMCPServer, name string, args map[string]any) *mcpsdk.CallToolResult {
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

func resultText(res *mcpsdk.CallToolResult) string {
	var b strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(*mcpsdk.TextContent); ok {
			b.WriteString(tc.Text)
		}
	}
	return b.String()
}

// TestStructuredEditDisabled asserts ABLATE-07 / D-04: with
// DisableStructuredEditSubsystem set, each structured-edit tool returns a
// serr.Unsupported error carrying the greppable "subsystem_disabled:" prefix,
// and with the flag OFF the guard is not hit (the handler proceeds past it to
// workspace/session acquisition).
func TestStructuredEditDisabled(t *testing.T) {
	root := t.TempDir()

	// The four structured-edit tools owned by this package + their required args.
	cases := []struct {
		tool string
		args map[string]any
	}{
		{"replace_symbol_body", map[string]any{"path": "x.go", "symbol_name": "Foo", "new_body": "func Foo() {}"}},
		{"insert_before_symbol", map[string]any{"path": "x.go", "symbol_name": "Foo", "content": "// before"}},
		{"insert_after_symbol", map[string]any{"path": "x.go", "symbol_name": "Foo", "content": "// after"}},
	}

	t.Run("flag ON returns Unsupported", func(t *testing.T) {
		srv := buildEditServer(t, kernel.KernelConfig{DisableStructuredEditSubsystem: true}, root)
		for _, tc := range cases {
			t.Run(tc.tool, func(t *testing.T) {
				res := callEditTool(t, srv, tc.tool, tc.args)
				if !res.IsError {
					t.Fatalf("%s: expected IsError result under the flag, got success: %s", tc.tool, resultText(res))
				}
				text := resultText(res)
				if !strings.Contains(text, "subsystem_disabled:") {
					t.Errorf("%s: error text missing 'subsystem_disabled:' prefix: %s", tc.tool, text)
				}
				if !strings.Contains(text, string(serr.Unsupported)) {
					t.Errorf("%s: error text missing %q kind: %s", tc.tool, serr.Unsupported, text)
				}
			})
		}
	})

	t.Run("flag OFF does not hit the guard", func(t *testing.T) {
		srv := buildEditServer(t, kernel.KernelConfig{}, root)
		for _, tc := range cases {
			t.Run(tc.tool, func(t *testing.T) {
				res := callEditTool(t, srv, tc.tool, tc.args)
				text := resultText(res)
				// With the subsystem enabled, the handler proceeds past the
				// guard into workspace/session acquisition (workspace not
				// activated in this harness) — it must NOT be the
				// subsystem_disabled branch.
				if strings.Contains(text, "subsystem_disabled:") {
					t.Errorf("%s: guard hit with flag OFF: %s", tc.tool, text)
				}
			})
		}
	})
}

// TestRenameSymbolLSPDisabled asserts the Phase 76 WR-02 backstop:
// rename_symbol leases a live LS worker, so under DisableLSPSubsystem a
// back-channel tools/call must refuse with a serr.Unsupported error carrying
// the greppable "subsystem_disabled:" prefix BEFORE AcquireSession — never
// leasing a worker. With the flag OFF the guard is not hit (the handler
// proceeds past it to workspace/session acquisition).
func TestRenameSymbolLSPDisabled(t *testing.T) {
	root := t.TempDir()
	args := map[string]any{"path": "x.go", "new_name": "Bar", "line": 1, "column": 1}

	t.Run("flag ON returns Unsupported", func(t *testing.T) {
		srv := buildEditServer(t, kernel.KernelConfig{DisableLSPSubsystem: true}, root)
		res := callEditTool(t, srv, "rename_symbol", args)
		if !res.IsError {
			t.Fatalf("rename_symbol: expected IsError under DisableLSPSubsystem, got success: %s", resultText(res))
		}
		text := resultText(res)
		if !strings.Contains(text, "subsystem_disabled:") {
			t.Errorf("rename_symbol: error text missing 'subsystem_disabled:' prefix: %s", text)
		}
		if !strings.Contains(text, string(serr.Unsupported)) {
			t.Errorf("rename_symbol: error text missing %q kind: %s", serr.Unsupported, text)
		}
	})

	t.Run("flag OFF does not hit the guard", func(t *testing.T) {
		srv := buildEditServer(t, kernel.KernelConfig{}, root)
		res := callEditTool(t, srv, "rename_symbol", args)
		text := resultText(res)
		if strings.Contains(text, "subsystem_disabled:") {
			t.Errorf("rename_symbol: LSP guard hit with flag OFF: %s", text)
		}
	})
}
