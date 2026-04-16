package repomap

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	_ "modernc.org/sqlite"
)

// TagCache provides SQLite-backed persistence for extracted tags with
// mtime-based invalidation. Tags survive daemon restarts and client
// reconnects. The cache uses a separate tags.db file independent from
// the memory store (per D-09).
type TagCache struct {
	db *sql.DB
	mu sync.Mutex
}

// NewTagCache opens (or creates) the SQLite tag cache at dbPath,
// initializes the schema, and configures WAL mode with a busy timeout.
// Follows the same pattern as internal/memory/index.go.
func NewTagCache(dbPath string) (*TagCache, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating tag cache dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening tag cache db: %w", err)
	}

	for _, pragma := range []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA busy_timeout=5000",
	} {
		if _, err := db.Exec(pragma); err != nil {
			db.Close()
			return nil, fmt.Errorf("setting pragma %q: %w", pragma, err)
		}
	}

	if _, err := db.Exec(createSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("creating schema: %w", err)
	}

	return &TagCache{db: db}, nil
}

// GetOrExtract returns cached tags for filePath if the file's mtime has
// not changed. On cache miss or mtime mismatch, it calls extractFn to
// get fresh tags, stores them, and returns them.
//
// Per D-10: file-level mtime invalidation. Per D-12: lazy (no eager warming).
// Per T-27-05: all SQL uses parameterized queries only.
func (c *TagCache) GetOrExtract(filePath string, extractFn func() ([]Tag, error)) ([]Tag, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("stat %s: %w", filePath, err)
	}
	mtime := info.ModTime().UnixNano()

	c.mu.Lock()

	// Check if we have a cached mtime for this file.
	var cachedMtime int64
	err = c.db.QueryRow(
		"SELECT mtime_ns FROM file_tags WHERE file_path = ? LIMIT 1",
		filePath,
	).Scan(&cachedMtime)

	if err == nil && cachedMtime == mtime {
		// Cache hit: load all tags for this file.
		tags, loadErr := c.loadTags(filePath)
		c.mu.Unlock()
		if loadErr != nil {
			return nil, fmt.Errorf("loading cached tags: %w", loadErr)
		}
		return tags, nil
	}

	// Cache miss or mtime mismatch: extract fresh tags.
	c.mu.Unlock()

	tags, err := extractFn()
	if err != nil {
		return nil, err
	}

	// Store fresh tags in the cache.
	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.storeTags(filePath, mtime, tags); err != nil {
		return nil, fmt.Errorf("storing tags: %w", err)
	}

	return tags, nil
}

// InvalidateFile removes all cached tags for the given file.
func (c *TagCache) InvalidateFile(filePath string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec("DELETE FROM file_tags WHERE file_path = ?", filePath)
	return err
}

// Clear removes all cached tags for all files.
func (c *TagCache) Clear() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	_, err := c.db.Exec("DELETE FROM file_tags")
	return err
}

// Close closes the underlying database connection.
func (c *TagCache) Close() error {
	return c.db.Close()
}

// loadTags reads all tags for a file from the database.
// Caller must hold c.mu.
func (c *TagCache) loadTags(filePath string) ([]Tag, error) {
	rows, err := c.db.Query(
		"SELECT name, kind, line, col, start_byte, end_byte FROM file_tags WHERE file_path = ? ORDER BY line, col",
		filePath,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var t Tag
		var kind string
		if err := rows.Scan(&t.Name, &kind, &t.Line, &t.Column, &t.StartByte, &t.EndByte); err != nil {
			return nil, err
		}
		t.Kind = TagKind(kind)
		t.File = filePath
		tags = append(tags, t)
	}
	return tags, rows.Err()
}

// storeTags deletes old tags for a file and batch-inserts new ones
// inside a transaction for atomicity and performance.
// Caller must hold c.mu.
func (c *TagCache) storeTags(filePath string, mtime int64, tags []Tag) error {
	tx, err := c.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	// Delete old tags for this file.
	if _, err := tx.Exec("DELETE FROM file_tags WHERE file_path = ?", filePath); err != nil {
		return fmt.Errorf("delete old tags: %w", err)
	}

	// Batch insert new tags.
	stmt, err := tx.Prepare(
		"INSERT INTO file_tags (file_path, mtime_ns, name, kind, line, col, start_byte, end_byte) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
	)
	if err != nil {
		return fmt.Errorf("prepare insert: %w", err)
	}
	defer stmt.Close()

	for _, t := range tags {
		if _, err := stmt.Exec(filePath, mtime, t.Name, string(t.Kind), t.Line, t.Column, t.StartByte, t.EndByte); err != nil {
			return fmt.Errorf("insert tag %q: %w", t.Name, err)
		}
	}

	return tx.Commit()
}
