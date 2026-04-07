package fileops

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"
)

// skipDirs contains directory names to skip during file walks.
var skipDirs = map[string]bool{
	".git":          true,
	"node_modules":  true,
	"__pycache__":   true,
	".serena":       true,
}

// maxFindResults is the maximum number of results from FindFiles.
const maxFindResults = 1000

// FindFiles finds files matching a glob pattern relative to root.
// Supports standard glob patterns (* and ?). For ** patterns, it matches
// any number of path components. Returns paths relative to root.
func FindFiles(root, pattern string) ([]string, error) {
	absRoot, err := ValidatePath(root, root)
	if err != nil {
		return nil, err
	}

	var results []string
	hasDoublestar := strings.Contains(pattern, "**")

	err = filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // skip entries with errors
		}

		// Skip excluded directories
		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}

		if matchPattern(pattern, relPath, hasDoublestar) {
			results = append(results, relPath)
			if len(results) >= maxFindResults {
				return fmt.Errorf("result limit reached")
			}
		}
		return nil
	})

	// Ignore the "result limit reached" error — it's expected flow control
	if err != nil && err.Error() != "result limit reached" {
		return nil, fmt.Errorf("walking directory: %w", err)
	}

	return results, nil
}

// matchPattern matches a relative path against a glob pattern.
// If hasDoublestar is true, ** matches any number of path segments.
func matchPattern(pattern, relPath string, hasDoublestar bool) bool {
	if hasDoublestar {
		return matchDoublestar(pattern, relPath)
	}

	// Standard glob match against the full relative path
	matched, _ := filepath.Match(pattern, relPath)
	if matched {
		return true
	}

	// Also try matching against just the filename
	matched, _ = filepath.Match(pattern, filepath.Base(relPath))
	return matched
}

// matchDoublestar handles ** glob patterns by expanding them to match any depth.
func matchDoublestar(pattern, relPath string) bool {
	// Split pattern into segments
	patternParts := strings.Split(filepath.ToSlash(pattern), "/")
	pathParts := strings.Split(filepath.ToSlash(relPath), "/")

	return matchParts(patternParts, pathParts)
}

// matchParts recursively matches pattern parts against path parts.
func matchParts(patternParts, pathParts []string) bool {
	if len(patternParts) == 0 {
		return len(pathParts) == 0
	}

	if patternParts[0] == "**" {
		// ** can match zero or more path segments
		rest := patternParts[1:]
		for i := 0; i <= len(pathParts); i++ {
			if matchParts(rest, pathParts[i:]) {
				return true
			}
		}
		return false
	}

	if len(pathParts) == 0 {
		return false
	}

	matched, _ := filepath.Match(patternParts[0], pathParts[0])
	if !matched {
		return false
	}
	return matchParts(patternParts[1:], pathParts[1:])
}
