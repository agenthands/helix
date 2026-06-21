package profile

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// LoadEmbedded reads all embedded profile and mode YAML files and returns a
// populated ProfileStore.
func LoadEmbedded() (*ProfileStore, error) {
	store := NewProfileStore()

	if err := loadProfilesFromFS(embeddedProfiles, "profiles", store); err != nil {
		return nil, fmt.Errorf("loading embedded profiles: %w", err)
	}
	if err := loadModesFromFS(embeddedModes, "modes", store); err != nil {
		return nil, fmt.Errorf("loading embedded modes: %w", err)
	}

	// ABLATE-02 fail-closed: reject any profile whose default_mode or mode
	// transitions reference an unknown mode name, rather than silently producing
	// a nil allow-list at session start (T-76-03 Tampering / measurement validity).
	if err := store.Validate(); err != nil {
		return nil, fmt.Errorf("validating embedded profiles: %w", err)
	}

	return store, nil
}

// Validate fail-closes on any profile that references an unknown mode name.
// For each loaded profile it verifies that DefaultMode resolves to a loaded mode
// and that every source and target mode in AllowedModeTransitions resolves too.
// Returns a typed error naming the offending profile and mode on the first miss.
//
// Phase 76 ABLATE-02 (T-76-03): a malformed/unknown mode name would otherwise
// silently no-op the profile's tool filter (resolveAllowedToolsForMode returns
// nil on a missed store.Mode(name)), defeating an ablation arm. Validate makes
// that a load-time error instead.
func (s *ProfileStore) Validate() error {
	for _, name := range s.ProfileNames() {
		p := s.profiles[name]
		if p.DefaultMode != "" {
			if _, ok := s.Mode(p.DefaultMode); !ok {
				return fmt.Errorf("profile %q references unknown mode %q (default_mode)", p.Name, p.DefaultMode)
			}
		}
		for src, targets := range p.AllowedModeTransitions {
			if _, ok := s.Mode(src); !ok {
				return fmt.Errorf("profile %q references unknown mode %q (transition source)", p.Name, src)
			}
			for _, dst := range targets {
				if _, ok := s.Mode(dst); !ok {
					return fmt.Errorf("profile %q references unknown mode %q (transition target from %q)", p.Name, dst, src)
				}
			}
		}
	}
	return nil
}

// LoadOverrides reads YAML files from disk directories and merges them into
// an existing ProfileStore. Fields present in override files overwrite embedded
// defaults; missing fields keep their embedded values.
// If a directory does not exist, it is silently skipped.
func LoadOverrides(store *ProfileStore, profileDir, modeDir string) error {
	if err := loadProfileOverridesFromDir(store, profileDir); err != nil {
		return fmt.Errorf("loading profile overrides from %s: %w", profileDir, err)
	}
	if err := loadModeOverridesFromDir(store, modeDir); err != nil {
		return fmt.Errorf("loading mode overrides from %s: %w", modeDir, err)
	}
	return nil
}

// loadProfilesFromFS loads all .yaml files from the given directory in an embed.FS.
func loadProfilesFromFS(fsys fs.FS, dir string, store *ProfileStore) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		var p Profile
		if err := yaml.Unmarshal(data, &p); err != nil {
			return fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		store.profiles[p.Name] = &p
	}
	return nil
}

// loadModesFromFS loads all .yaml files from the given directory in an embed.FS.
func loadModesFromFS(fsys fs.FS, dir string, store *ProfileStore) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Name(), err)
		}
		var m Mode
		if err := yaml.Unmarshal(data, &m); err != nil {
			return fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}
		store.modes[m.Name] = &m
	}
	return nil
}

// loadProfileOverridesFromDir reads YAML files from a disk directory and merges
// them into existing profiles. If the directory does not exist, it is skipped.
func loadProfileOverridesFromDir(store *ProfileStore, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		// Parse to get the name for lookup.
		var partial Profile
		if err := yaml.Unmarshal(data, &partial); err != nil {
			return fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}

		if existing, ok := store.profiles[partial.Name]; ok {
			// Unmarshal override on top of existing profile.
			// This overwrites set fields but keeps unset fields from the original.
			if err := yaml.Unmarshal(data, existing); err != nil {
				return fmt.Errorf("merging override %s: %w", entry.Name(), err)
			}
		} else {
			// New profile from override.
			store.profiles[partial.Name] = &partial
		}
	}
	return nil
}

// loadModeOverridesFromDir reads YAML files from a disk directory and merges
// them into existing modes. If the directory does not exist, it is skipped.
func loadModeOverridesFromDir(store *ProfileStore, dir string) error {
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("reading directory %s: %w", dir, err)
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".yaml") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("reading %s: %w", entry.Name(), err)
		}

		var partial Mode
		if err := yaml.Unmarshal(data, &partial); err != nil {
			return fmt.Errorf("parsing %s: %w", entry.Name(), err)
		}

		if existing, ok := store.modes[partial.Name]; ok {
			if err := yaml.Unmarshal(data, existing); err != nil {
				return fmt.Errorf("merging override %s: %w", entry.Name(), err)
			}
		} else {
			store.modes[partial.Name] = &partial
		}
	}
	return nil
}
