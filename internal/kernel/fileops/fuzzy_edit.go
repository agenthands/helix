package fileops

import (
	serr "github.com/agenthands/helix/internal/errors"
	"github.com/agenthands/helix/internal/fuzzy"
)

// FuzzyEdit reads a file, runs fuzzy.Match against its content, and writes
// the result via atomic OverwriteFile. Returns the fuzzy.Result on success.
func FuzzyEdit(root, path, search, replacement string, allowEllipsis bool) (*fuzzy.Result, error) {
	_, err := ValidatePath(root, path)
	if err != nil {
		return nil, err
	}

	content, err := ReadFile(root, path)
	if err != nil {
		return nil, err
	}

	result, err := fuzzy.Match(content, search, fuzzy.Options{
		Replacement:   replacement,
		AllowEllipsis: allowEllipsis,
	})
	if err != nil {
		return nil, err
	}

	newContent := content[:result.StartByte] + result.ReplacementText + content[result.EndByte:]
	if err := OverwriteFile(root, path, newContent); err != nil {
		return nil, serr.Wrap(serr.Internal, "writing fuzzy edit", err)
	}

	return result, nil
}
