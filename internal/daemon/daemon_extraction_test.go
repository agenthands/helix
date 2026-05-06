package daemon

import (
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/semantic"
)

// TestActivateWorkspace_TriggersScheduler asserts the daemon's semantic
// extractor registry is constructed when cfg.SemanticIndex.Enabled = true.
//
// Phase 59 P05 plan-as-written demanded this test also assert
// scheduler.ScheduleInitialExtraction was invoked with
// Reason="workspace_activation" within 200ms of activation. That assertion
// requires the scheduler package (Phase 59 P03) which is being built in a
// parallel worktree — it does not yet exist in HEAD.
//
// Until merge time this test asserts:
//   - the extract registry IS constructed when semantic_index.enabled = true
//   - the registry holds the daemon-singleton GrammarRegistry
//
// Post-merge (when 59-03 lands), this test gains:
//   - kernel.ActivateWorkspace fires scheduler.ScheduleInitialExtraction
//   - the scheduler's status transitions to SemanticIndexing within 200ms
//   - the call is non-blocking (activation returns within the existing budget)
//
// See daemon.go step 6c/6d comments for the merge-time wiring.
func TestActivateWorkspace_TriggersScheduler(t *testing.T) {
	wsDir := t.TempDir()
	// BL-01: store.Path must be workspace-relative; chdir into wsDir.
	t.Chdir(wsDir)
	dbPath := filepath.Join(".helix", "semantic.duckdb")

	cfg := &config.SerenaConfig{
		Profile: "full",
		Mode:    "edit",
		SemanticIndex: semantic.Config{
			Enabled: true,
			Store: semantic.StoreConfig{
				Kind:        "duckdb",
				Path:        dbPath,
				MemoryLimit: "256MiB",
				Threads:     2,
			},
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New(cfg) error: %v", err)
	}

	// Pre-merge assertion: registry IS constructed when enabled.
	if d.SemanticExtractRegistryForTest() == nil {
		t.Fatal("daemon.SemanticExtractRegistryForTest() = nil; expected non-nil because cfg.SemanticIndex.Enabled = true (D-04 invariant)")
	}

	// Pre-merge assertion: registry holds the daemon-singleton grammar registry.
	// (Pointer-equality covered exhaustively by TestBootstrap_GrammarRegistrySingleton.)
	if d.SemanticExtractRegistryForTest().Grammars() != d.GrammarRegistryForTest() {
		t.Error("registry.Grammars() != daemon.grammarRegistry; EXTRACT-05 violation")
	}
}

// TestActivateWorkspace_SemanticDisabledNoOp asserts that when
// cfg.SemanticIndex.Enabled = false, the daemon does NOT construct the
// extract registry — i.e., the no-op path is taken.
//
// At merge time this test gains the corollary scheduler-side assertion:
//   - d.semanticScheduler is nil
//   - kernel.ActivateWorkspace does NOT call ScheduleInitialExtraction
//   - activation returns within the existing activation budget (no extra cost)
func TestActivateWorkspace_SemanticDisabledNoOp(t *testing.T) {
	cfg := &config.SerenaConfig{
		Profile: "full",
		Mode:    "edit",
		SemanticIndex: semantic.Config{
			Enabled: false,
		},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelWarn}))

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New(cfg) error: %v", err)
	}

	if got := d.SemanticExtractRegistryForTest(); got != nil {
		t.Errorf("daemon.SemanticExtractRegistryForTest() = %v; want nil because cfg.SemanticIndex.Enabled = false", got)
	}
	if got := d.SemanticStore(); got != nil {
		t.Errorf("daemon.SemanticStore() = %v; want nil because cfg.SemanticIndex.Enabled = false", got)
	}

	// The grammar registry IS still constructed even when semantic_index is
	// disabled — repomap and the body extractor depend on it. EXTRACT-05's
	// "exactly one registry" invariant applies regardless of semantic_index.
	if got := d.GrammarRegistryForTest(); got == nil {
		t.Error("daemon.GrammarRegistryForTest() = nil; the singleton registry is required by repomap and edit even when semantic_index is disabled")
	}
}
