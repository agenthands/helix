package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"syscall"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Load builds a SerenaConfig from layered sources in precedence order:
// 1. Built-in defaults
// 2. Global config (~/.serena/serena_config.yml)
// 3. Project config (.serena/project.yml) (D-09)
// 4. CLI flag overrides
func Load(globalPath, projectPath string, cliOverrides map[string]interface{}) (*SerenaConfig, error) {
	k := koanf.New(".")

	// 1. Built-in defaults
	if err := k.Load(confmap.Provider(DefaultConfig(), "."), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// 2. Global config (~/.serena/serena_config.yml)
	if globalPath == "" {
		homeDir, _ := os.UserHomeDir()
		globalPath = filepath.Join(homeDir, ".serena", "serena_config.yml")
	}
	if _, err := os.Stat(globalPath); err == nil {
		if err := k.Load(file.Provider(globalPath), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("loading global config %s: %w", globalPath, err)
		}
	}

	// 3. Project config (.serena/project.yml) (D-09, WRK-04)
	if projectPath != "" {
		if _, err := os.Stat(projectPath); err == nil {
			if err := k.Load(file.Provider(projectPath), yaml.Parser()); err != nil {
				return nil, fmt.Errorf("loading project config %s: %w", projectPath, err)
			}
		}
	}

	// 4. CLI flag overrides (highest precedence)
	if len(cliOverrides) > 0 {
		if err := k.Load(confmap.Provider(cliOverrides, "."), nil); err != nil {
			return nil, fmt.Errorf("loading CLI overrides: %w", err)
		}
	}

	var cfg SerenaConfig
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("unmarshaling config: %w", err)
	}

	// Apply computed defaults
	if cfg.Daemon.SocketPath == "" {
		uid := strconv.Itoa(int(syscall.Getuid()))
		cfg.Daemon.SocketPath = filepath.Join(os.TempDir(), "serena-"+uid, "daemon.sock")
	}

	return &cfg, nil
}

// DefaultSocketPath returns the default Unix socket path for the daemon.
func DefaultSocketPath() string {
	uid := strconv.Itoa(int(syscall.Getuid()))
	return filepath.Join(os.TempDir(), "serena-"+uid, "daemon.sock")
}
