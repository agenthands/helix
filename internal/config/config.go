package config

// SerenaConfig is the top-level configuration for the Serena daemon.
// Mirrors the Python Serena config schema (D-11) with Go types.
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
}

// ObservabilityConfig holds admin listener + pprof gating settings (Phase 10).
// AdminAddr empty = disabled. Must be loopback (127.0.0.1/localhost/::1) — v1.3 adds auth.
type ObservabilityConfig struct {
	// AdminAddr is the loopback bind address for the admin listener.
	// Empty string disables the listener (D-02, D-05).
	AdminAddr string `koanf:"admin_addr"`
	// EnablePprof registers /debug/pprof/* handlers on the admin listener when true (D-10).
	// Default false: zero attack surface when disabled (D-12).
	EnablePprof bool `koanf:"enable_pprof"`
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
	// SocketPath overrides default /tmp/serena-$UID/daemon.sock (D-14)
	SocketPath string `koanf:"socket_path"`
	// HTTPAddr is the listen address for Streamable HTTP (default ":8080")
	HTTPAddr string `koanf:"http_addr"`
	// ShutdownTimeout in seconds for graceful shutdown
	ShutdownTimeout int `koanf:"shutdown_timeout"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	// Format: "text" (default) or "json" (D-16)
	Format string `koanf:"format"`
	// Level: "debug", "info", "warn", "error" (default: "info")
	Level string `koanf:"level"`
	// Dir: log file directory (default: ~/.serena/logs/) (D-17)
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
