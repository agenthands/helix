package langregistry

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Registry holds all language server entries with thread-safe access.
// It starts with compiled-in defaults and layers YAML overrides on top.
type Registry struct {
	entries map[string]LSEntry
	mu      sync.RWMutex
}

// yamlEntry mirrors LSEntry for YAML deserialization with pointer fields
// so we can distinguish "not set" from zero-value during deep merge.
type yamlEntry struct {
	Command        *string           `yaml:"command"`
	Args           []string          `yaml:"args"`
	InitOptions    map[string]any    `yaml:"init_options"`
	NeedsWorkspace *bool             `yaml:"needs_workspace"`
	FileExts       []string          `yaml:"file_exts"`
	Install        *yamlInstallInfo  `yaml:"install"`
	IgnoredDirs    []string          `yaml:"ignored_dirs"`
}

type yamlInstallInfo struct {
	Type    *string           `yaml:"type"`
	Package *string           `yaml:"package"`
	Version *string           `yaml:"version"`
	URLs    map[string]string `yaml:"urls"`
	SHA256  map[string]string `yaml:"sha256"`
}

// NewRegistry creates a Registry pre-loaded with defaultEntries and applies
// YAML overrides from the given file paths (in order, later overrides win).
// Override files that do not exist are silently skipped.
func NewRegistry(overridePaths ...string) (*Registry, error) {
	// Deep-copy defaults so callers cannot mutate the package-level map.
	entries := make(map[string]LSEntry, len(defaultEntries))
	for k, v := range defaultEntries {
		entries[k] = v
	}

	for _, p := range overridePaths {
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("langregistry: reading override %s: %w", p, err)
		}
		var overrides map[string]yamlEntry
		if err := yaml.Unmarshal(data, &overrides); err != nil {
			return nil, fmt.Errorf("langregistry: parsing override %s: %w", p, err)
		}
		for lang, ov := range overrides {
			base, exists := entries[lang]
			if !exists {
				base = LSEntry{Language: lang}
			}
			mergeOverride(&base, &ov)
			entries[lang] = base
		}
	}

	return &Registry{entries: entries}, nil
}

// Get returns the LSEntry for the given language key.
func (r *Registry) Get(language string) (LSEntry, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	e, ok := r.entries[language]
	return e, ok
}

// Languages returns a sorted list of all registered language keys.
func (r *Registry) Languages() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	langs := make([]string, 0, len(r.entries))
	for k := range r.entries {
		langs = append(langs, k)
	}
	sort.Strings(langs)
	return langs
}

// ByExtension returns all entries whose FileExts contain the given extension.
// The extension should include the dot (e.g. ".py").
func (r *Registry) ByExtension(ext string) []LSEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext = strings.ToLower(ext)
	var result []LSEntry
	for _, e := range r.entries {
		for _, fe := range e.FileExts {
			if strings.ToLower(fe) == ext {
				result = append(result, e)
				break
			}
		}
	}
	return result
}

// mergeOverride applies non-zero YAML fields onto a base LSEntry (deep merge).
func mergeOverride(base *LSEntry, ov *yamlEntry) {
	if ov.Command != nil {
		base.Command = *ov.Command
	}
	if ov.Args != nil {
		base.Args = ov.Args
	}
	if ov.InitOptions != nil {
		if base.InitOptions == nil {
			base.InitOptions = make(map[string]any)
		}
		for k, v := range ov.InitOptions {
			base.InitOptions[k] = v
		}
	}
	if ov.NeedsWorkspace != nil {
		base.NeedsWorkspace = *ov.NeedsWorkspace
	}
	if ov.FileExts != nil {
		base.FileExts = ov.FileExts
	}
	if ov.IgnoredDirs != nil {
		base.IgnoredDirs = ov.IgnoredDirs
	}
	if ov.Install != nil {
		if base.Install == nil {
			base.Install = &InstallInfo{}
		}
		mergeInstallOverride(base.Install, ov.Install)
	}
}

// mergeInstallOverride applies non-zero YAML fields onto a base InstallInfo.
func mergeInstallOverride(base *InstallInfo, ov *yamlInstallInfo) {
	if ov.Type != nil {
		base.Type = *ov.Type
	}
	if ov.Package != nil {
		base.Package = *ov.Package
	}
	if ov.Version != nil {
		base.Version = *ov.Version
	}
	if ov.URLs != nil {
		base.URLs = ov.URLs
	}
	if ov.SHA256 != nil {
		base.SHA256 = ov.SHA256
	}
}
