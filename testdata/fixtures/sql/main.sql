-- Helper table for testing file operations
CREATE TABLE helper (
    id INTEGER PRIMARY KEY,
    name TEXT NOT NULL,
    value INTEGER DEFAULT 0
);

-- DemoView referencing helper table
CREATE VIEW demo_view AS
SELECT id, name, value
FROM helper
WHERE value > 0;

-- Insert sample data
INSERT INTO helper (name, value) VALUES ('hello', 42);
INSERT INTO helper (name, value) VALUES ('world', 99);

-- Unused table for delete tests
CREATE TABLE unused_table (
    id INTEGER PRIMARY KEY
);

-- Query using helper
SELECT h.name, h.value
FROM helper h
WHERE h.name = 'hello';
