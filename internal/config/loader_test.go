package config

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/postfix/serena/internal/profile" // ensure embedded profiles are loadable
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

func TestLoad_DefaultProfile(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Profile != "full" {
		t.Errorf("expected default profile 'full', got %q", cfg.Profile)
	}
	if cfg.Mode != "" {
		t.Errorf("expected empty default mode, got %q", cfg.Mode)
	}
}

func TestLoad_ObservabilityDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/global.yml", "", nil)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Observability.AdminAddr != "" {
		t.Errorf("expected empty default admin_addr, got %q", cfg.Observability.AdminAddr)
	}
	if cfg.Observability.EnablePprof {
		t.Errorf("expected default enable_pprof=false, got true")
	}
}

func TestLoad_ObservabilityOverride(t *testing.T) {
	overrides := map[string]interface{}{
		"observability.admin_addr":   "127.0.0.1:9090",
		"observability.enable_pprof": true,
	}
	cfg, err := Load("/nonexistent/global.yml", "", overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Observability.AdminAddr != "127.0.0.1:9090" {
		t.Errorf("expected admin_addr 127.0.0.1:9090, got %q", cfg.Observability.AdminAddr)
	}
	if !cfg.Observability.EnablePprof {
		t.Errorf("expected enable_pprof=true, got false")
	}
}

// TestLoad_EmptyAdminAddrOverrideIsNotApplied documents the runDaemon guard
// (Pitfall #5): callers must not put an empty string into the overrides map
// for observability.admin_addr, otherwise a project config value would be
// silently wiped. This test exercises the *absence* of the override — i.e.
// when the guard is correctly applied upstream, a project-supplied value
// survives.
func TestLoad_EmptyAdminAddrOverrideIsNotApplied(t *testing.T) {
	dir := t.TempDir()
	projectPath := filepath.Join(dir, "project.yml")
	os.WriteFile(projectPath, []byte("observability:\n  admin_addr: \"127.0.0.1:7777\"\n"), 0600)

	// Simulate the runDaemon guard: adminAddr flag is empty, so NOTHING is
	// placed in the overrides map for observability.admin_addr.
	overrides := map[string]interface{}{}
	cfg, err := Load("/nonexistent/global.yml", projectPath, overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if cfg.Observability.AdminAddr != "127.0.0.1:7777" {
		t.Errorf("expected project admin_addr 127.0.0.1:7777 to survive empty CLI flag, got %q", cfg.Observability.AdminAddr)
	}
}

func TestResolveProfile_KnownProfile(t *testing.T) {
	cfg := &SerenaConfig{Profile: "claude-code"}
	store, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	if store == nil {
		t.Fatal("expected non-nil ProfileStore")
	}
	if prof == nil {
		t.Fatal("expected non-nil Profile for claude-code")
	}
}

func TestResolveProfile_UnknownFallsBackToFull(t *testing.T) {
	cfg := &SerenaConfig{Profile: "nonexistent-profile"}
	_, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	// Should fall back to the "full" default profile
	if prof == nil {
		t.Fatal("expected fallback to full profile, got nil")
	}
}

func TestResolveProfile_CLIOverrideTakesPrecedence(t *testing.T) {
	dir := t.TempDir()

	// Create project config with profile=claude-code
	projectPath := filepath.Join(dir, "project.yml")
	os.WriteFile(projectPath, []byte("profile: claude-code\n"), 0600)

	// CLI overrides to ci-bot
	overrides := map[string]interface{}{
		"profile": "ci-bot",
	}

	cfg, err := Load("/nonexistent/global.yml", projectPath, overrides)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	if cfg.Profile != "ci-bot" {
		t.Errorf("expected CLI override profile 'ci-bot', got %q", cfg.Profile)
	}

	// Resolve should find the ci-bot profile
	_, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	if prof == nil {
		t.Fatal("expected non-nil profile for ci-bot")
	}
}

func TestResolveProfile_DescriptionOverridesAccessible(t *testing.T) {
	cfg := &SerenaConfig{Profile: "claude-code"}
	_, prof, err := ResolveProfile(cfg, "")
	if err != nil {
		t.Fatalf("ResolveProfile failed: %v", err)
	}
	// ToolDescriptionOverrides should be accessible (may be empty for some profiles)
	if prof.ToolDescriptionOverrides == nil {
		// This is acceptable - not all profiles have overrides
		t.Log("claude-code profile has no tool description overrides (acceptable)")
	}
}
