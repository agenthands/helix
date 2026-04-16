package repomap

// createSchema is the SQL for initializing the tag cache database.
// Stores extracted def/ref tags per file with mtime for invalidation.
const createSchema = `
CREATE TABLE IF NOT EXISTS file_tags (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    file_path TEXT NOT NULL,
    mtime_ns INTEGER NOT NULL,
    name TEXT NOT NULL,
    kind TEXT NOT NULL CHECK(kind IN ('def', 'ref')),
    line INTEGER NOT NULL,
    col INTEGER NOT NULL,
    start_byte INTEGER NOT NULL,
    end_byte INTEGER NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_file_tags_path ON file_tags(file_path);
CREATE INDEX IF NOT EXISTS idx_file_tags_name ON file_tags(name);
`
