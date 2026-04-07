package fileops

import (
	"fmt"
	"os"
	"path/filepath"
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
		return fmt.Errorf("file already exists: %s (use OverwriteFile to replace)", path)
	}

	// Create parent directories
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating directories: %w", err)
	}

	if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing file: %w", err)
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
		return fmt.Errorf("creating directories: %w", err)
	}

	// Write to temp file in the same directory for atomic rename
	tmp, err := os.CreateTemp(dir, ".serena-write-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.WriteString(content); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("closing temp file: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpName, absPath); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
