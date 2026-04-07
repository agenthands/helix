package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load with defaults failed: %v", err)
	}

	if cfg.Daemon.HTTPAddr != ":8080" {
		t.Errorf("expected default http_addr :8080, got %s", cfg.Daemon.HTTPAddr)
	}
	if cfg.Daemon.ShutdownTimeout != 10 {
		t.Errorf("expected default shutdown_timeout 10, got %d", cfg.Daemon.ShutdownTimeout)
	}
	if cfg.Logging.Format != "text" {
		t.Errorf("expected default logging format text, got %s", cfg.Logging.Format)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected default logging level info, got %s", cfg.Logging.Level)
	}
	// SocketPath should be auto-computed (non-empty)
	if cfg.Daemon.SocketPath == "" {
		t.Error("expected auto-computed socket path, got empty")
	}
}

func TestLoad_CLIOverrides(t *testing.T) {
	overrides := map[string]interface{}{
		"daemon.socket_path": "/custom/path.sock",
		"daemon.http_addr":   ":9090",
	}
	cfg, err := Load("/nonexistent/global.yml", "", overrides)
	if err != nil {
		t.Fatalf("Load with CLI overrides failed: %v", err)
	}

	if cfg.Daemon.SocketPath != "/custom/path.sock" {
		t.Errorf("expected CLI override socket path /custom/path.sock, got %s", cfg.Daemon.SocketPath)
	}
	if cfg.Daemon.HTTPAddr != ":9090" {
		t.Errorf("expected CLI override http_addr :9090, got %s", cfg.Daemon.HTTPAddr)
	}
}

func TestLoad_ProjectConfigOverridesGlobal(t *testing.T) {
	dir := t.TempDir()

	// Create global config
	globalPath := filepath.Join(dir, "global.yml")
	os.WriteFile(globalPath, []byte("daemon:\n  http_addr: \":7070\"\nlogging:\n  level: debug\n"), 0600)

	// Create project config that overrides http_addr
	projectPath := filepath.Join(dir, "project.yml")
	os.WriteFile(projectPath, []byte("daemon:\n  http_addr: \":8888\"\n"), 0600)

	cfg, err := Load(globalPath, projectPath, nil)
	if err != nil {
		t.Fatalf("Load with project config failed: %v", err)
	}

	// Project overrides global for http_addr
	if cfg.Daemon.HTTPAddr != ":8888" {
		t.Errorf("expected project override http_addr :8888, got %s", cfg.Daemon.HTTPAddr)
	}
	// Global sets logging level (not overridden by project)
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected global logging level debug, got %s", cfg.Logging.Level)
	}
}
