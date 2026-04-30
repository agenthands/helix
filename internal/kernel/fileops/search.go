package fileops

import (
	"bufio"
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"

	serr "github.com/agenthands/helix/internal/errors"
)

// SearchOpts configures search behavior.
type SearchOpts struct {
	MaxResults   int
	ContextLines int
	IncludeGlob  string
	ExcludeGlob  string
}

// SearchMatch represents a single search match.
type SearchMatch struct {
	Path          string
	Line          int
	Text          string
	ContextBefore []string
	ContextAfter  []string
}

// defaultMaxResults is used when MaxResults is 0.
const defaultMaxResults = 100

// SearchPattern searches files under root for a regex pattern.
// Skips binary files and excluded directories.
func SearchPattern(root, pattern string, opts SearchOpts) ([]SearchMatch, error) {
	absRoot, err := ValidatePath(root, root)
	if err != nil {
		return nil, err
	}

	re, err := regexp.Compile(pattern)
	if err != nil {
		return nil, serr.Wrap(serr.InvalidArgs, "invalid regex pattern", err)
	}

	maxResults := opts.MaxResults
	if maxResults <= 0 {
		maxResults = defaultMaxResults
	}

	var results []SearchMatch

	walkErr := filepath.WalkDir(absRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if d.IsDir() && skipDirs[d.Name()] {
			return filepath.SkipDir
		}
		if d.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(absRoot, path)
		if err != nil {
			return nil
		}

		// Apply include/exclude globs
		if opts.IncludeGlob != "" {
			matched, _ := filepath.Match(opts.IncludeGlob, filepath.Base(relPath))
			if !matched {
				return nil
			}
		}
		if opts.ExcludeGlob != "" {
			matched, _ := filepath.Match(opts.ExcludeGlob, filepath.Base(relPath))
			if matched {
				return nil
			}
		}

		// Check if file is binary
		if isBinaryFile(path) {
			return nil
		}

		matches, err := searchFile(path, relPath, re, opts.ContextLines)
		if err != nil {
			return nil // skip files we can't read
		}

		results = append(results, matches...)
		if len(results) >= maxResults {
			results = results[:maxResults]
			return fmt.Errorf("result limit reached")
		}
		return nil
	})

	if walkErr != nil && walkErr.Error() != "result limit reached" {
		return nil, serr.Wrap(serr.Internal, "walking directory", walkErr)
	}

	return results, nil
}

// isBinaryFile checks the first 512 bytes for null bytes (binary indicator).
func isBinaryFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return true // can't read, skip
	}
	defer f.Close()

	buf := make([]byte, 512)
	n, err := f.Read(buf)
	if n == 0 {
		return false // empty file is not binary
	}
	return bytes.ContainsRune(buf[:n], 0)
}

// searchFile searches a single file for regex matches with optional context lines.
func searchFile(absPath, relPath string, re *regexp.Regexp, contextLines int) ([]SearchMatch, error) {
	f, err := os.Open(absPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read all lines for context support
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var matches []SearchMatch
	for i, line := range lines {
		if re.MatchString(line) {
			m := SearchMatch{
				Path: relPath,
				Line: i + 1,
				Text: line,
			}

			if contextLines > 0 {
				start := i - contextLines
				if start < 0 {
					start = 0
				}
				end := i + contextLines
				if end >= len(lines) {
					end = len(lines) - 1
				}

				m.ContextBefore = make([]string, 0, i-start)
				for j := start; j < i; j++ {
					m.ContextBefore = append(m.ContextBefore, lines[j])
				}
				m.ContextAfter = make([]string, 0, end-i)
				for j := i + 1; j <= end; j++ {
					m.ContextAfter = append(m.ContextAfter, lines[j])
				}
			}

			matches = append(matches, m)
		}
	}
	return matches, nil
}
