package fileops

import (
	"fmt"
	"regexp"
	"strings"
)

// ReplaceInFile replaces all occurrences of a pattern in a file.
// If isRegex is true, pattern is compiled as a regex; otherwise it's a literal string match.
// Uses atomic write to prevent partial writes on crash.
// Returns the number of replacements made.
func ReplaceInFile(root, path, pattern, replacement string, isRegex bool) (int, error) {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return 0, err
	}

	// Read existing content
	content, err := ReadFile(root, path)
	if err != nil {
		return 0, err
	}

	var newContent string
	var count int

	if isRegex {
		re, err := regexp.Compile(pattern)
		if err != nil {
			return 0, fmt.Errorf("invalid regex pattern: %w", err)
		}
		// Count matches
		matches := re.FindAllStringIndex(content, -1)
		count = len(matches)
		if count == 0 {
			return 0, nil
		}
		newContent = re.ReplaceAllString(content, replacement)
	} else {
		count = strings.Count(content, pattern)
		if count == 0 {
			return 0, nil
		}
		newContent = strings.ReplaceAll(content, pattern, replacement)
	}

	// Use atomic write via OverwriteFile (which validates the path again, but that's fine)
	_ = absPath // path already validated
	if err := OverwriteFile(root, path, newContent); err != nil {
		return 0, fmt.Errorf("writing replacement: %w", err)
	}

	return count, nil
}
