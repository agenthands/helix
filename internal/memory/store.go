package memory

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Scope represents the storage scope for a memory.
type Scope string

const (
	// ScopeProject stores memories in the project's .serena/memories/ directory.
	ScopeProject Scope = "project"
	// ScopeGlobal stores memories in ~/.serena/memories/.
	ScopeGlobal Scope = "global"
)

// MemoryStore provides CRUD operations for markdown-based memories with
// SQLite FTS5 indexing. It manages two directories: project-local and global.
type MemoryStore struct {
	projectDir string // e.g. /path/to/project/.serena/memories/
	globalDir  string // e.g. ~/.serena/memories/
	index      *Index
	logger     *slog.Logger
}

// NewMemoryStore creates a MemoryStore with the given directories and index DB path.
// Both projectDir and globalDir are created if they do not exist.
func NewMemoryStore(projectDir, globalDir, indexDBPath string, logger *slog.Logger) (*MemoryStore, error) {
	if logger == nil {
		logger = slog.Default()
	}

	for _, dir := range []string{projectDir, globalDir} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0o755); err != nil {
				return nil, fmt.Errorf("creating memory dir %s: %w", dir, err)
			}
		}
	}

	idx, err := NewIndex(indexDBPath)
	if err != nil {
		return nil, fmt.Errorf("creating index: %w", err)
	}

	// Build initial index from existing files.
	if err := idx.Rebuild(projectDir, globalDir); err != nil {
		idx.Close()
		return nil, fmt.Errorf("initial index build: %w", err)
	}

	return &MemoryStore{
		projectDir: projectDir,
		globalDir:  globalDir,
		index:      idx,
		logger:     logger,
	}, nil
}

// Write creates or overwrites a memory. The name determines scope and path:
//   - "global/foo"        -> globalDir/foo.md  (scope=global)
//   - "foo"               -> projectDir/foo.md (scope=project)
//   - "topic/sub/name"    -> projectDir/topic/sub/name.md (scope=project)
func (s *MemoryStore) Write(name, content string) error {
	name = normalizeName(name)
	scope, fpath := s.resolve(name)

	if err := os.MkdirAll(filepath.Dir(fpath), 0o755); err != nil {
		return fmt.Errorf("creating parent dirs: %w", err)
	}
	if err := os.WriteFile(fpath, []byte(content), 0o644); err != nil {
		return fmt.Errorf("writing memory file: %w", err)
	}

	if err := s.index.Upsert(name, fpath, string(scope), content); err != nil {
		s.logger.Warn("failed to index memory", "name", name, "error", err)
	}

	s.logger.Info("memory written", "name", name, "scope", scope)
	return nil
}

// Read returns the content of a memory by name.
func (s *MemoryStore) Read(name string) (string, error) {
	name = normalizeName(name)
	_, fpath := s.resolve(name)

	data, err := os.ReadFile(fpath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("memory %q not found", name)
		}
		return "", fmt.Errorf("reading memory: %w", err)
	}
	return string(data), nil
}

// List returns all memories, optionally filtered by scope and topic.
func (s *MemoryStore) List(scope string, topic string) ([]MemoryEntry, error) {
	return s.index.List(scope, topic)
}

// Search performs a full-text search over all memories.
func (s *MemoryStore) Search(query string, scope string) ([]MemoryEntry, error) {
	return s.index.Search(query, scope)
}

// Rename moves a memory from oldName to newName. This supports cross-scope moves.
func (s *MemoryStore) Rename(oldName, newName string) error {
	oldName = normalizeName(oldName)
	newName = normalizeName(newName)

	_, oldPath := s.resolve(oldName)
	newScope, newPath := s.resolve(newName)

	if err := os.MkdirAll(filepath.Dir(newPath), 0o755); err != nil {
		return fmt.Errorf("creating parent dirs: %w", err)
	}
	if err := os.Rename(oldPath, newPath); err != nil {
		return fmt.Errorf("renaming memory file: %w", err)
	}

	// Read the content for reindexing.
	data, err := os.ReadFile(newPath)
	if err != nil {
		return fmt.Errorf("reading renamed memory: %w", err)
	}

	if err := s.index.Remove(oldName); err != nil {
		s.logger.Warn("failed to remove old index entry", "name", oldName, "error", err)
	}
	if err := s.index.Upsert(newName, newPath, string(newScope), string(data)); err != nil {
		s.logger.Warn("failed to index renamed memory", "name", newName, "error", err)
	}

	return nil
}

// Edit reads a memory, applies editFn to its content, and writes the result back.
func (s *MemoryStore) Edit(name string, editFn func(content string) string) error {
	name = normalizeName(name)

	content, err := s.Read(name)
	if err != nil {
		return err
	}

	newContent := editFn(content)
	return s.Write(name, newContent)
}

// Delete removes a memory file and its index entry.
func (s *MemoryStore) Delete(name string) error {
	name = normalizeName(name)
	_, fpath := s.resolve(name)

	if err := os.Remove(fpath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing memory file: %w", err)
	}
	if err := s.index.Remove(name); err != nil {
		s.logger.Warn("failed to remove index entry", "name", name, "error", err)
	}

	s.logger.Info("memory deleted", "name", name)
	return nil
}

// Close closes the underlying index.
func (s *MemoryStore) Close() error {
	return s.index.Close()
}

// Index returns the underlying Index for direct access (e.g., by the watcher).
func (s *MemoryStore) Index() *Index {
	return s.index
}

// resolve maps a memory name to its scope and absolute file path.
func (s *MemoryStore) resolve(name string) (Scope, string) {
	if strings.HasPrefix(name, "global/") {
		sub := strings.TrimPrefix(name, "global/")
		return ScopeGlobal, filepath.Join(s.globalDir, sub+".md")
	}
	return ScopeProject, filepath.Join(s.projectDir, name+".md")
}

// normalizeName strips a trailing .md extension if the user included one.
func normalizeName(name string) string {
	return strings.TrimSuffix(name, ".md")
}
