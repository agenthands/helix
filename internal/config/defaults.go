package config

import (
	"os"
	"path/filepath"
)

// DefaultConfig returns built-in default values.
func DefaultConfig() map[string]interface{} {
	homeDir, _ := os.UserHomeDir()
	return map[string]interface{}{
		"daemon.socket_path":                 "", // empty means auto-compute from UID
		"daemon.http_addr":                   ":8080",
		"daemon.shutdown_timeout":            10,
		"logging.format":                     "text",
		"logging.level":                      "info",
		"logging.dir":                        filepath.Join(homeDir, ".serena", "logs"),
		"profile":                            "full",     // default profile is the neutral escape hatch (D-02)
		"mode":                               "",         // empty = use profile's default_mode
		"observability.admin_addr":           "",         // empty = admin listener disabled (D-02, D-05)
		"observability.enable_pprof":         false,      // zero attack surface by default (D-12)
		"observability.tracing_endpoint":     "",         // empty = tracing disabled (D-11)
		"observability.tracing_sample_ratio": float64(0), // 0.0 = off by default (D-11)
		"observability.service_name":         "serena",   // OTel resource service.name (D-11)
		"degradation.timeout_read":           5,          // seconds (D-02)
		"degradation.timeout_search":         15,         // seconds (D-02)
		"degradation.timeout_edit":           10,         // seconds (D-02)
		"degradation.timeout_index":          120,        // seconds (D-02)
		"degradation.timeout_diagnostics":    20,         // seconds (D-02)
		"degradation.memory_limit_mb":        0,          // 0 = don't set (D-09)
		"degradation.restart_budget":         3,          // consecutive crashes before circuit stays open (D-05)
	}
}
