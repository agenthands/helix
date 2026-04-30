package memory

import (
	"crypto/sha256"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// MemoryEntry represents a single memory record with its metadata.
type MemoryEntry struct {
	Name     string
	FilePath string
	Scope    string // "project" or "global"
	Topic    string
	Title    string
	Summary  string
	Rank     float64 // FTS5 rank (populated for search results)
}

// Index manages the SQLite FTS5 index for memory search.
type Index struct {
	db *sql.DB
	mu sync.Mutex
}

// NewIndex opens (or creates) the SQLite database at dbPath, initializes the
// schema, and configures WAL mode with a busy timeout.
func NewIndex(dbPath string) (*Index, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return nil, fmt.Errorf("creating index dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening index db: %w", err)
	}

	// WAL mode + busy timeout per research Pitfall 1
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

	return &Index{db: db}, nil
}

// Upsert inserts or replaces a memory in the index. It extracts title,
// headings, and tags from the markdown content automatically.
func (idx *Index) Upsert(name, filePath, scope, content string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	title := extractTitle(content)
	headings := extractHeadings(content)
	tags := extractTags(content)
	topic := extractTopic(name)
	hash := contentHash(content)

	const q = `INSERT INTO memories (name, file_path, scope, topic, title, headings, tags, summary, content, content_hash, modified_at)
VALUES (?, ?, ?, ?, ?, ?, ?, '', ?, ?, ?)
ON CONFLICT(name) DO UPDATE SET
    file_path=excluded.file_path, scope=excluded.scope, topic=excluded.topic,
    title=excluded.title, headings=excluded.headings, tags=excluded.tags,
    summary=excluded.summary, content=excluded.content,
    content_hash=excluded.content_hash, modified_at=excluded.modified_at`

	_, err := idx.db.Exec(q, name, filePath, scope, topic, title, headings, tags, content, hash, time.Now().UTC())
	return err
}

// Remove deletes a memory from the index by name.
func (idx *Index) Remove(name string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	_, err := idx.db.Exec("DELETE FROM memories WHERE name = ?", name)
	return err
}

// Search performs a full-text search over the index, optionally filtered by scope.
// Returns up to 20 results ordered by FTS5 rank.
func (idx *Index) Search(query string, scope string) ([]MemoryEntry, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var rows *sql.Rows
	var err error

	if scope != "" {
		rows, err = idx.db.Query(
			`SELECT m.name, m.file_path, m.scope, m.topic, m.title, m.summary, f.rank
			 FROM memories_fts f
			 JOIN memories m ON m.id = f.rowid
			 WHERE memories_fts MATCH ? AND m.scope = ?
			 ORDER BY f.rank
			 LIMIT 20`, query, scope)
	} else {
		rows, err = idx.db.Query(
			`SELECT m.name, m.file_path, m.scope, m.topic, m.title, m.summary, f.rank
			 FROM memories_fts f
			 JOIN memories m ON m.id = f.rowid
			 WHERE memories_fts MATCH ?
			 ORDER BY f.rank
			 LIMIT 20`, query)
	}
	if err != nil {
		return nil, fmt.Errorf("search query: %w", err)
	}
	defer rows.Close()

	return scanEntries(rows)
}

// List returns all memories, optionally filtered by scope and/or topic.
func (idx *Index) List(scope string, topic string) ([]MemoryEntry, error) {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	var (
		clauses []string
		args    []any
	)

	if scope != "" {
		clauses = append(clauses, "scope = ?")
		args = append(args, scope)
	}
	if topic != "" {
		clauses = append(clauses, "(topic = ? OR topic LIKE ?)")
		args = append(args, topic, topic+"/%")
	}

	q := "SELECT name, file_path, scope, topic, title, summary FROM memories"
	if len(clauses) > 0 {
		q += " WHERE " + strings.Join(clauses, " AND ")
	}
	q += " ORDER BY name"

	rows, err := idx.db.Query(q, args...)
	if err != nil {
		return nil, fmt.Errorf("list query: %w", err)
	}
	defer rows.Close()

	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.Name, &e.FilePath, &e.Scope, &e.Topic, &e.Title, &e.Summary); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// Rebuild drops all rows and re-indexes markdown files from both directories.
func (idx *Index) Rebuild(projectDir, globalDir string) error {
	idx.mu.Lock()
	defer idx.mu.Unlock()

	if _, err := idx.db.Exec("DELETE FROM memories"); err != nil {
		return fmt.Errorf("clearing index: %w", err)
	}

	// Re-insert from both directories (unlock temporarily for upserts)
	idx.mu.Unlock()

	if projectDir != "" {
		if err := idx.indexDir(projectDir, "project"); err != nil {
			idx.mu.Lock()
			return err
		}
	}
	if globalDir != "" {
		if err := idx.indexDir(globalDir, "global"); err != nil {
			idx.mu.Lock()
			return err
		}
	}

	idx.mu.Lock()
	return nil
}

// Close closes the underlying database connection.
func (idx *Index) Close() error {
	return idx.db.Close()
}

// indexDir walks a directory and upserts every .md file found.
func (idx *Index) indexDir(dir, scope string) error {
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip inaccessible paths
		}
		if info.IsDir() || !strings.HasSuffix(info.Name(), ".md") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil // skip unreadable files
		}
		rel, _ := filepath.Rel(dir, path)
		name := strings.TrimSuffix(rel, ".md")
		name = filepath.ToSlash(name) // normalize separators
		if scope == "global" {
			name = "global/" + name
		}
		return idx.Upsert(name, path, scope, string(data))
	})
}

func scanEntries(rows *sql.Rows) ([]MemoryEntry, error) {
	var entries []MemoryEntry
	for rows.Next() {
		var e MemoryEntry
		if err := rows.Scan(&e.Name, &e.FilePath, &e.Scope, &e.Topic, &e.Title, &e.Summary, &e.Rank); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// --- Markdown metadata extraction (regex-based, kept simple) ---

var (
	reTitle    = regexp.MustCompile(`(?m)^#\s+(.+)$`)
	reHeading  = regexp.MustCompile(`(?m)^#{2,6}\s+(.+)$`)
	reTagsYAML = regexp.MustCompile(`(?m)^tags:\s*\[(.+?)\]`)
	reTagsList = regexp.MustCompile(`(?m)^tags:\s*\n((?:\s*-\s*.+\n?)+)`)
	reTagItem  = regexp.MustCompile(`(?m)^\s*-\s*(.+)`)
)

func extractTitle(content string) string {
	m := reTitle.FindStringSubmatch(content)
	if len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	return ""
}

func extractHeadings(content string) string {
	matches := reHeading.FindAllStringSubmatch(content, -1)
	var headings []string
	for _, m := range matches {
		headings = append(headings, strings.TrimSpace(m[1]))
	}
	return strings.Join(headings, "; ")
}

func extractTags(content string) string {
	// Try inline YAML tags: [tag1, tag2]
	if m := reTagsYAML.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// Try list YAML tags
	if m := reTagsList.FindStringSubmatch(content); len(m) > 1 {
		items := reTagItem.FindAllStringSubmatch(m[1], -1)
		var tags []string
		for _, item := range items {
			tags = append(tags, strings.TrimSpace(item[1]))
		}
		return strings.Join(tags, ", ")
	}
	return ""
}

func extractTopic(name string) string {
	// Topic is the directory portion of the name.
	// "auth/login/logic" -> "auth/login"
	// "readme" -> ""
	// "global/foo" -> "" (global scope, no sub-topic)
	n := name
	if strings.HasPrefix(n, "global/") {
		n = strings.TrimPrefix(n, "global/")
	}
	if idx := strings.LastIndex(n, "/"); idx >= 0 {
		return n[:idx]
	}
	return ""
}

func contentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:16]) // 128-bit prefix is sufficient
}
