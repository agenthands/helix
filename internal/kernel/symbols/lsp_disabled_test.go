package symbols

import (
	"context"
	"log/slog"
	"os"
	"strings"
	"testing"

	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/kernel"
	"github.com/agenthands/helix/internal/kernel/lspool"
	"github.com/agenthands/helix/internal/langregistry"
	"github.com/agenthands/helix/internal/workspace"
)

// newTestKernel builds a minimal kernel for guard tests. No live language
// server is wired — the no_lsp backstop in acquireLease must short-circuit
// before any pool/session acquisition, so this needs no workspace activation.
func newTestKernel(t *testing.T, cfg kernel.KernelConfig) *kernel.Kernel {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))
	wsReg := workspace.NewRegistry()
	langReg, err := langregistry.NewRegistry()
	if err != nil {
		t.Fatalf("langregistry.NewRegistry: %v", err)
	}
	installer := langregistry.NewInstaller(langregistry.InstallerConfig{}, logger)
	return kernel.NewKernel(wsReg, langReg, installer, cfg, nil, logger, lspool.NoopSink{}, nil)
}

// TestAcquireLeaseLSPDisabled asserts the Phase 76 WR-02 backstop: the shared
// LS-leasing chokepoint for the symbol-retrieval tools refuses with a
// serr.Unsupported error carrying the greppable "subsystem_disabled:" prefix
// when DisableLSPSubsystem is set — before any GetRuntime/AcquireSession — so
// a back-channel tools/call cannot lease a live worker or emit lspool.lsp.*
// spans under the no_lsp ablation arm.
func TestAcquireLeaseLSPDisabled(t *testing.T) {
	wsKeyFn := func() workspace.WorkspaceKey { return workspace.WorkspaceKey{RepoRoot: t.TempDir()} }

	t.Run("flag ON returns Unsupported before workspace lookup", func(t *testing.T) {
		k := newTestKernel(t, kernel.KernelConfig{DisableLSPSubsystem: true})
		_, err := acquireLease(context.Background(), k, wsKeyFn)
		if err == nil {
			t.Fatal("expected error under DisableLSPSubsystem, got nil")
		}
		msg := err.Error()
		if !strings.Contains(msg, "subsystem_disabled:") {
			t.Errorf("error missing 'subsystem_disabled:' prefix: %s", msg)
		}
		if !strings.Contains(msg, string(serr.Unsupported)) {
			t.Errorf("error missing %q kind: %s", serr.Unsupported, msg)
		}
	})

	t.Run("flag OFF falls through to workspace lookup", func(t *testing.T) {
		k := newTestKernel(t, kernel.KernelConfig{})
		_, err := acquireLease(context.Background(), k, wsKeyFn)
		// With the flag OFF the guard is not hit; the workspace is not
		// activated in this harness, so the failure must be NoWorkspace —
		// NOT the subsystem_disabled branch.
		if err == nil {
			t.Fatal("expected NoWorkspace error (no workspace activated), got nil")
		}
		if strings.Contains(err.Error(), "subsystem_disabled:") {
			t.Errorf("LSP guard hit with flag OFF: %s", err.Error())
		}
	})
}
