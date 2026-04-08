package memory

// createSchema is the SQL for initializing the memory index database.
// Uses FTS5 for full-text search over memory content and metadata.
const createSchema = `
CREATE TABLE IF NOT EXISTS memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    file_path TEXT NOT NULL,
    scope TEXT NOT NULL CHECK(scope IN ('project', 'global')),
    topic TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL DEFAULT '',
    headings TEXT NOT NULL DEFAULT '',
    tags TEXT NOT NULL DEFAULT '',
    summary TEXT NOT NULL DEFAULT '',
    content TEXT NOT NULL DEFAULT '',
    content_hash TEXT NOT NULL,
    modified_at DATETIME NOT NULL
);
CREATE VIRTUAL TABLE IF NOT EXISTS memories_fts USING fts5(
    name, title, headings, tags, summary, content,
    content='memories', content_rowid='id'
);
CREATE TRIGGER IF NOT EXISTS memories_ai AFTER INSERT ON memories BEGIN
    INSERT INTO memories_fts(rowid, name, title, headings, tags, summary, content)
    VALUES (new.id, new.name, new.title, new.headings, new.tags, new.summary, new.content);
END;
CREATE TRIGGER IF NOT EXISTS memories_ad AFTER DELETE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, name, title, headings, tags, summary, content)
    VALUES ('delete', old.id, old.name, old.title, old.headings, old.tags, old.summary, old.content);
END;
CREATE TRIGGER IF NOT EXISTS memories_au AFTER UPDATE ON memories BEGIN
    INSERT INTO memories_fts(memories_fts, rowid, name, title, headings, tags, summary, content)
    VALUES ('delete', old.id, old.name, old.title, old.headings, old.tags, old.summary, old.content);
    INSERT INTO memories_fts(rowid, name, title, headings, tags, summary, content)
    VALUES (new.id, new.name, new.title, new.headings, new.tags, new.summary, new.content);
END;
CREATE INDEX IF NOT EXISTS idx_memories_scope ON memories(scope);
CREATE INDEX IF NOT EXISTS idx_memories_topic ON memories(topic);
`
