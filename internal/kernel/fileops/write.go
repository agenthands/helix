// Package fileops write.go contains the CreateFile and OverwriteFile
// helpers used by the create_file, replace_in_file, and fuzzy_edit
// tools.
//
// Phase 60 D-03: per-tool MCP handlers in tools.go fire
// EditNotifier.OnEdit after every successful CreateFile / OverwriteFile
// call. The free functions here remain notifier-unaware on purpose —
// the *kernel.Kernel handle is held by the registered handler closure,
// not the helper. RESEARCH.md Open Question O-1 was resolved in favor
// of per-tool wiring (3 register* closures) over centralized
// OverwriteFile wiring; the latter would require ripple changes to
// every OverwriteFile caller without a corresponding correctness gain.
package fileops

import (
	"os"
	"path/filepath"

	serr "github.com/agenthands/helix/internal/errors"
)

// CreateFile creates a new file with the given content.
// Parent directories are created if they don't exist.
// Returns an error if the file already exists.
func CreateFile(root, path, content string) error {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return err
	}

	// Check if file already exists
	if _, err := os.Stat(absPath); err == nil {
		return serr.New(serr.InvalidArgs, "file already exists").WithDetail(path)
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return serr.Wrap(serr.Internal, "creating directories", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return serr.Wrap(serr.Internal, "writing file", err)
	}
	return nil
}

// OverwriteFile writes content to a file using atomic write (temp file + rename).
// Creates parent directories if needed.
func OverwriteFile(root, path, content string) error {
	absPath, err := ValidatePath(root, path)
	if err != nil {
		return err
	}

	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return serr.Wrap(serr.Internal, "creating directories", err)
	}

	// Write to temp file in the same directory for atomic rename
	tmp, err := os.CreateTemp(dir, ".helix-write-*")
	if err != nil {
		return serr.Wrap(serr.Internal, "creating temp file", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return serr.Wrap(serr.Internal, "writing temp file", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return serr.Wrap(serr.Internal, "closing temp file", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, absPath); err != nil {
		os.Remove(tmpName)
		return serr.Wrap(serr.Internal, "renaming temp file", err)
	}
	return nil
}
