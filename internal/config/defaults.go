package config

import (
	"os"
	"path/filepath"
)

// DefaultConfig returns built-in default values.
func DefaultConfig() map[string]interface{} {
	homeDir, _ := os.UserHomeDir()
	return map[string]interface{}{
		"daemon.socket_path":      "", // empty means auto-compute from UID
		"daemon.http_addr":        ":8080",
		"daemon.shutdown_timeout": 10,
		"logging.format":          "text",
		"logging.level":           "info",
		"logging.dir":             filepath.Join(homeDir, ".serena", "logs"),
		"profile":                 "full", // default profile is the neutral escape hatch (D-02)
		"mode":                    "",     // empty = use profile's default_mode
	}
}
