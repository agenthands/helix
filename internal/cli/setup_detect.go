package cli

import (
	"context"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"

	"github.com/postfix/serena/internal/langregistry"
)

// skipDirs contains directory names to skip during language detection.
// Skipping these avoids walking massive dependency trees (T-34-07).
var skipDirs = map[string]bool{
	"node_modules": true,
	"vendor":       true,
	"__pycache__":  true,
	".git":         true,
}

// detectLanguages walks the project directory, detects file extensions, and
// returns unique LSEntry items for each detected language using Registry.ByExtension().
// Results are sorted alphabetically by Language.
func detectLanguages(dir string, reg *langregistry.Registry, printer *SetupPrinter) ([]langregistry.LSEntry, error) {
	seen := make(map[string]bool)
	var entries []langregistry.LSEntry

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip errors, best-effort walk
		}
		if d.IsDir() {
			name := d.Name()
			// Skip hidden directories and known large dependency trees.
			if strings.HasPrefix(name, ".") || skipDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		ext := filepath.Ext(d.Name())
		if ext == "" {
			return nil
		}
		matches := reg.ByExtension(ext)
		for _, m := range matches {
			if !seen[m.Language] {
				seen[m.Language] = true
				entries = append(entries, m)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sort by language name for stable output.
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Language < entries[j].Language
	})

	if len(entries) > 0 {
		names := make([]string, len(entries))
		for i, e := range entries {
			names[i] = e.Language
		}
		printer.Info("Detected languages: %s", strings.Join(names, ", "))
	}

	return entries, nil
}

// preInstallLanguageServers resolves language server binaries for detected languages
// using the three-tier Installer (PATH lookup > managed download > helpful error).
// Returns the list of successfully resolved entries for downstream health check.
func preInstallLanguageServers(ctx context.Context, entries []langregistry.LSEntry, installer *langregistry.Installer, printer *SetupPrinter, dryRun bool) []langregistry.LSEntry {
	if dryRun {
		for _, entry := range entries {
			printer.DryRunAction("would install %s (%s)", entry.Language, entry.Command)
		}
		return entries // assume all would succeed for dry-run
	}

	var installed []langregistry.LSEntry
	for _, entry := range entries {
		command, _, err := installer.Resolve(ctx, entry)
		if err != nil {
			printer.Failure("%s: %v", entry.Language, err)
			continue
		}
		printer.Success("%s (%s) ready", entry.Language, command)
		installed = append(installed, entry)
	}
	return installed
}
