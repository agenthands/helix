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

	"github.com/agenthands/helix/internal/profile"
)

// Load builds a SerenaConfig from layered sources in precedence order:
// 1. Built-in defaults
// 2. Global config (~/.helix/helix_config.yml)
// 3. Project config (.helix/project.yml) (D-09)
// 4. CLI flag overrides
func Load(globalPath, projectPath string, cliOverrides map[string]interface{}) (*SerenaConfig, error) {
	k := koanf.New(".")

	// 1. Built-in defaults
	if err := k.Load(confmap.Provider(DefaultConfig(), "."), nil); err != nil {
		return nil, fmt.Errorf("loading defaults: %w", err)
	}

	// 2. Global config (~/.helix/helix_config.yml)
	if globalPath == "" {
		homeDir, _ := os.UserHomeDir()
		globalPath = filepath.Join(homeDir, ".helix", "helix_config.yml")
	}
	if _, err := os.Stat(globalPath); err == nil {
		if err := k.Load(file.Provider(globalPath), yaml.Parser()); err != nil {
			return nil, fmt.Errorf("loading global config %s: %w", globalPath, err)
		}
	}

	// 3. Project config (.helix/project.yml) (D-09, WRK-04)
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
		cfg.Daemon.SocketPath = filepath.Join(os.TempDir(), "helix-"+uid, "daemon.sock")
	}

	return &cfg, nil
}

// DefaultSocketPath returns the default Unix socket path for the daemon.
func DefaultSocketPath() string {
	uid := strconv.Itoa(int(syscall.Getuid()))
	return filepath.Join(os.TempDir(), "helix-"+uid, "daemon.sock")
}

// ResolveProfile loads the ProfileStore and returns the active profile based on
// the config's Profile field. The profile name flows through koanf precedence
// (CLI > project > global > default "full"); the profile content comes from
// the ProfileStore (embedded + overrides from globalDir).
func ResolveProfile(cfg *SerenaConfig, globalDir string) (*profile.ProfileStore, *profile.Profile, error) {
	store, err := profile.LoadEmbedded()
	if err != nil {
		return nil, nil, fmt.Errorf("loading embedded profiles: %w", err)
	}

	// Apply overrides from the global Helix directory if available.
	if globalDir != "" {
		if err := profile.LoadOverrides(store, filepath.Join(globalDir, "profiles"), filepath.Join(globalDir, "modes")); err != nil {
			return nil, nil, fmt.Errorf("loading profile overrides: %w", err)
		}
		// ABLATE-02 fail-closed (WR-03): re-validate after overrides so a disk
		// override that introduces an unknown default_mode / transition (or a
		// brand-new profile) cannot reintroduce the nil-allow-list hole that
		// Validate() closes inside LoadEmbedded.
		if err := store.Validate(); err != nil {
			return nil, nil, fmt.Errorf("validating profiles after overrides: %w", err)
		}
	}

	p, ok := store.Profile(cfg.Profile)
	if !ok {
		// Fall back to the "full" default profile.
		return store, store.DefaultProfile(), nil
	}
	return store, p, nil
}
