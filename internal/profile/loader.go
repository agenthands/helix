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

	return store, nil
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
