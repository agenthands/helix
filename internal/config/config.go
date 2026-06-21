package config

import (
	"github.com/agenthands/helix/internal/semantic"
)

// SerenaConfig is the top-level configuration for the Serena daemon.
// Mirrors the Python Helix config schema (D-11) with Go types.
type SerenaConfig struct {
	// Daemon settings
	Daemon DaemonConfig `koanf:"daemon"`
	// Logging settings
	Logging LoggingConfig `koanf:"logging"`
	// Default project settings (overridden per-project)
	Defaults ProjectDefaults `koanf:"defaults"`
	// Worker pool settings for LS process management
	WorkerPool WorkerPoolConfig `koanf:"worker_pool"`
	// Profile is the active agent profile name (default: "full").
	// Precedence: CLI --profile > project config > user config > default (D-10, PRF-05).
	Profile string `koanf:"profile"`
	// Mode is the initial operational mode override.
	// Empty means use the profile's DefaultMode.
	Mode string `koanf:"mode"`
	// Observability holds admin listener + pprof gating settings (Phase 10).
	Observability ObservabilityConfig `koanf:"observability"`
	// Degradation holds timeout budgets and resilience settings (Phase 13).
	Degradation DegradationConfig `koanf:"degradation"`
	// DisableLSPSubsystem requests the daemon disable the LSP subsystem
	// (Phase 76 ABLATE-05). Opt-in, default OFF: the bench no_lsp ablation
	// arm sets it true so the daemon structurally suppresses every
	// LSP-touching seam (no live EditNotifier, no RepoMap LSP enrichment,
	// no pool-leasing fallback paths) — proven by zero lspool.lsp.* spans.
	// Precedence at the composition root: CLI --disable-lsp-subsystem OR
	// resolved profile field (a one-way force-disable, D-02/D-03).
	DisableLSPSubsystem bool `koanf:"disable_lsp_subsystem"`
	// DisableStructuredEditSubsystem requests the daemon disable the
	// structured-edit tools (Phase 76 ABLATE-07). Opt-in, default OFF:
	// the bench no_structured_edit arm sets it true so the four
	// structured-edit tools return serr.Unsupported and replace_in_file
	// runs exact-match-only. Precedence: CLI flag OR resolved profile
	// field (D-02/D-03).
	DisableStructuredEditSubsystem bool `koanf:"disable_structured_edit_subsystem"`
	// SemanticIndex holds Phase 57+ semantic graph settings (SPEC §25).
	//
	// Phase 57 plan P02 landed this field as a stub (zero values, no koanf
	// tag); Phase 57 plan P03 (this commit) attaches the
	// `koanf:"semantic_index"` binding tag so the SPEC §25 defaults from
	// internal/config/defaults.go and any user/project YAML files actually
	// populate the field through the standard 4-layer precedence.
	SemanticIndex semantic.Config `koanf:"semantic_index"`
}

// DegradationConfig holds timeout budgets and resilience settings (Phase 13).
type DegradationConfig struct {
	TimeoutRead        int `koanf:"timeout_read"`        // seconds, default 5
	TimeoutSearch      int `koanf:"timeout_search"`      // seconds, default 15
	TimeoutEdit        int `koanf:"timeout_edit"`        // seconds, default 10
	TimeoutIndex       int `koanf:"timeout_index"`       // seconds, default 120
	TimeoutDiagnostics int `koanf:"timeout_diagnostics"` // seconds, default 20
	MemoryLimitMB      int `koanf:"memory_limit_mb"`     // 0 = don't set (use GOMEMLIMIT env if present)
	RestartBudget      int `koanf:"restart_budget"`      // default 3, consecutive crashes before circuit stays open
}

// ObservabilityConfig holds admin listener, pprof gating, and tracing settings.
// AdminAddr empty = disabled. Must be loopback (127.0.0.1/localhost/::1) — v1.3 adds auth.
type ObservabilityConfig struct {
	// AdminAddr is the loopback bind address for the admin listener.
	// Empty string disables the listener (D-02, D-05).
	AdminAddr string `koanf:"admin_addr"`
	// EnablePprof registers /debug/pprof/* handlers on the admin listener when true (D-10).
	// Default false: zero attack surface when disabled (D-12).
	EnablePprof bool `koanf:"enable_pprof"`

	// Phase 12: Tracing
	// TracingEndpoint is the OTLP/gRPC collector endpoint.
	// Empty string disables tracing entirely (D-11).
	TracingEndpoint string `koanf:"tracing_endpoint"`
	// TracingSampleRatio is the TraceIDRatioBased fraction.
	// 0.0 = off (default), 1.0 = sample everything (D-11).
	TracingSampleRatio float64 `koanf:"tracing_sample_ratio"`
	// ServiceName is the OTel resource service.name attribute.
	// Default: "helix" (D-11).
	ServiceName string `koanf:"service_name"`
}

// WorkerPoolConfig holds configuration for the LS worker pool.
type WorkerPoolConfig struct {
	// BaseTTL is the base idle timeout in seconds (default 300).
	BaseTTL int `koanf:"base_ttl"`
	// CeilingTTL is the maximum idle timeout in seconds (default 3600).
	CeilingTTL int `koanf:"ceiling_ttl"`
	// MaxWorkers is the maximum number of concurrent LS workers (default 10).
	MaxWorkers int `koanf:"max_workers"`
	// RSSHardCapMB is the per-worker RSS hard cap in MB for pressure eviction (default 2048).
	RSSHardCapMB int `koanf:"rss_hard_cap_mb"`
	// PressureCheckInterval is the interval in seconds between pressure checks (default 10).
	PressureCheckInterval int `koanf:"pressure_check_interval"`
}

// DaemonConfig holds daemon-specific settings.
type DaemonConfig struct {
	// SocketPath overrides default /tmp/helix-$UID/daemon.sock (D-14)
	SocketPath string `koanf:"socket_path"`
	// GRPCAddr is the OPTIONAL loopback gRPC TCP listen address for split-host
	// CLI↔daemon use (Phase 94 RETIRE-04). Empty (default) = unix-socket only;
	// when set it MUST be loopback (127.0.0.1/localhost/::1) — non-loopback is
	// refused (validateGRPCAddr), deferred to REMOTE-01. It reuses the same
	// ForwarderService.StreamMCP RPC as the unix socket (no proto change).
	GRPCAddr string `koanf:"grpc_addr"`
	// ShutdownTimeout in seconds for graceful shutdown
	ShutdownTimeout int `koanf:"shutdown_timeout"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	// Format: "text" (default) or "json" (D-16)
	Format string `koanf:"format"`
	// Level: "debug", "info", "warn", "error" (default: "info")
	Level string `koanf:"level"`
	// Dir: log file directory (default: ~/.helix/logs/) (D-17)
	Dir string `koanf:"dir"`
}

// ProjectDefaults holds default project settings.
type ProjectDefaults struct {
	// Contexts define tool sets for different environments
	Contexts map[string]ContextConfig `koanf:"contexts"`
	// Modes define operational patterns
	Modes map[string]ModeConfig `koanf:"modes"`
}

// ContextConfig defines a tool context.
type ContextConfig struct {
	// Tools lists tool names available in this context
	Tools []string `koanf:"tools"`
	// Description of this context
	Description string `koanf:"description"`
}

// ModeConfig defines an operational mode.
type ModeConfig struct {
	// Tools lists tool names available in this mode
	Tools []string `koanf:"tools"`
	// Description of this mode
	Description string `koanf:"description"`
}
