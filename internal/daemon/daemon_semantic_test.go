//go:build cgo && integration

package daemon

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/agenthands/helix/internal/config"
	"github.com/agenthands/helix/internal/semantic"
)

// TestDaemon_SemanticStore_Open_FromConfig is an integration-tagged smoke
// test that constructs a *config.SerenaConfig with SemanticIndex.Enabled =
// true, calls New(cfg, ...), and asserts:
//   - daemon.SemanticStore() returns a non-nil *Store
//   - the DuckDB file exists at the configured path
//
// Phase 57 plan P03 Task 3 will activate this test from the koanf side
// (loading config from YAML); the field-level construction tested here does
// not require P03. Run with: `go test -tags=integration ./internal/daemon`.
func TestDaemon_SemanticStore_Open_FromConfig(t *testing.T) {
	wsDir := t.TempDir()
	dbPath := filepath.Join(wsDir, ".helix", "semantic.duckdb")

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

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))

	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New(cfg): %v", err)
	}
	if d.SemanticStore() == nil {
		t.Fatal("daemon.SemanticStore(): want non-nil, got nil (cfg.SemanticIndex.Enabled was true)")
	}

	st, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat(%s): %v", dbPath, err)
	}
	if !st.Mode().IsRegular() {
		t.Errorf("Stat(%s): not a regular file", dbPath)
	}
}
