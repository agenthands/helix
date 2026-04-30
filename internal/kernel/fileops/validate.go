package fileops

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	serr "github.com/agenthands/helix/internal/errors"
)

// ValidatePath resolves symlinks and ensures the path is within the workspace root.
// Returns the cleaned absolute path or an error if the path escapes the root.
func ValidatePath(root, path string) (string, error) {
	if root == "" {
		return "", serr.New(serr.NoWorkspace, "workspace root is empty")
	}

	// Resolve the root to an absolute, symlink-resolved path
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "resolving workspace root", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(absRoot)
	if err != nil {
		return "", serr.Wrap(serr.Internal, "resolving workspace root symlinks", err)
	}

	// Build the target path
	var absPath string
	if filepath.IsAbs(path) {
		absPath = filepath.Clean(path)
	} else {
		absPath = filepath.Join(resolvedRoot, path)
	}

	// Resolve symlinks for the target. If the file doesn't exist yet,
	// resolve the parent directory instead (for create operations).
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist yet — resolve parent dir
			parentResolved, parentErr := filepath.EvalSymlinks(filepath.Dir(absPath))
			if parentErr != nil {
				// Parent doesn't exist either — use cleaned abs path
				resolvedPath = absPath
			} else {
				resolvedPath = filepath.Join(parentResolved, filepath.Base(absPath))
			}
		} else {
			return "", serr.Wrap(serr.Internal, "resolving path symlinks", err)
		}
	}

	// Check containment: resolved path must be within or equal to resolved root
	if !strings.HasPrefix(resolvedPath, resolvedRoot+string(filepath.Separator)) && resolvedPath != resolvedRoot {
		return "", serr.New(serr.InvalidArgs, "path outside workspace root").WithDetail(fmt.Sprintf("%s outside %s", path, root))
	}

	return resolvedPath, nil
}
