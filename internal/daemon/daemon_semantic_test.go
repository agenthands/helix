//go:build cgo && integration

package daemon

import (
	"fmt"
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

// TestDaemon_SemanticStore_FromYAML_Enabled proves the full koanf-loading
// path: a project YAML sets `semantic_index.enabled: true` and a custom
// store path; config.Load merges defaults + YAML through 4-layer precedence;
// the resulting SerenaConfig flows into daemon.New(); SemanticStore() is
// non-nil and the DuckDB file exists at the YAML-supplied path.
//
// This is the test that proves the koanf binding tag added by P03 Task 2
// is wired correctly end-to-end (defaults map → koanf → SerenaConfig.
// SemanticIndex → daemon step 6b → semanticstore.Open).
func TestDaemon_SemanticStore_FromYAML_Enabled(t *testing.T) {
	wsDir := t.TempDir()
	dbPath := filepath.Join(wsDir, ".helix", "semantic.duckdb")

	// Project YAML enables semantic index and points at the temp DB path.
	projectYAML := fmt.Sprintf(""+
		"profile: full\n"+
		"semantic_index:\n"+
		"  enabled: true\n"+
		"  store:\n"+
		"    kind: duckdb\n"+
		"    path: %q\n"+
		"    memory_limit: \"256MiB\"\n"+
		"    threads: 2\n", dbPath)
	projectPath := filepath.Join(wsDir, "project.yml")
	if err := os.WriteFile(projectPath, []byte(projectYAML), 0o600); err != nil {
		t.Fatalf("write project yml: %v", err)
	}

	cfg, err := config.Load("/nonexistent/global.yml", projectPath, nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	// Sanity-check the koanf binding actually populated the field.
	if !cfg.SemanticIndex.Enabled {
		t.Fatalf("config.Load: SemanticIndex.Enabled=false after YAML load (koanf binding broken)")
	}
	if cfg.SemanticIndex.Store.Path != dbPath {
		t.Fatalf("config.Load: store.path=%q, want %q", cfg.SemanticIndex.Store.Path, dbPath)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New(cfg): %v", err)
	}
	if d.SemanticStore() == nil {
		t.Fatal("daemon.SemanticStore(): want non-nil after YAML enabled=true, got nil")
	}

	st, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Stat(%s): %v", dbPath, err)
	}
	if !st.Mode().IsRegular() {
		t.Errorf("Stat(%s): not a regular file", dbPath)
	}
}

// TestDaemon_SemanticStore_FromYAML_Disabled proves STORE-02 wire-through:
// when a project YAML explicitly sets `semantic_index.enabled: false`, the
// daemon starts cleanly and SemanticStore() returns nil. Without P03's
// SPEC §25 default `enabled=true`, this case would be indistinguishable
// from the field's Go zero value — the YAML must override the default for
// this test to be meaningful, and that round-trip is itself an assertion
// that the koanf precedence chain is functioning.
func TestDaemon_SemanticStore_FromYAML_Disabled(t *testing.T) {
	wsDir := t.TempDir()
	projectYAML := "" +
		"profile: full\n" +
		"semantic_index:\n" +
		"  enabled: false\n"
	projectPath := filepath.Join(wsDir, "project.yml")
	if err := os.WriteFile(projectPath, []byte(projectYAML), 0o600); err != nil {
		t.Fatalf("write project yml: %v", err)
	}

	cfg, err := config.Load("/nonexistent/global.yml", projectPath, nil)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	if cfg.SemanticIndex.Enabled {
		t.Fatalf("config.Load: SemanticIndex.Enabled=true after YAML load with enabled=false (precedence broken)")
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, &slog.HandlerOptions{Level: slog.LevelInfo}))
	d, err := New(cfg, logger)
	if err != nil {
		t.Fatalf("New(cfg): %v", err)
	}
	if d.SemanticStore() != nil {
		t.Fatal("daemon.SemanticStore(): want nil after YAML enabled=false, got non-nil")
	}
}

// _ unused import guard for future helpers. Touching the semantic package
// import keeps the existing field-construction smoke test (above) honest:
// if semantic.Config drifts again, the build fails at this site.
var _ = semantic.Config{}
