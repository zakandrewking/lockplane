-- dialect: sqlite
-- SQLite schema fixture testing type preservation and default expressions

CREATE TABLE tasks (
    -- Integer types
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    priority INTEGER NOT NULL DEFAULT 0,
    count BIGINT DEFAULT 100,

    -- Text types
    title TEXT NOT NULL,
    description TEXT,
    status VARCHAR(50) DEFAULT 'pending',

    -- Real/Numeric types
    progress REAL DEFAULT 0.0,
    score NUMERIC(10, 2),

    -- Boolean (stored as INTEGER in SQLite)
    completed INTEGER NOT NULL DEFAULT 0,
    archived BOOLEAN DEFAULT 0,

    -- Date/Time with various default expressions
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT DEFAULT (datetime('now')),
    due_date TEXT DEFAULT (date('now', '+7 days')),
    timestamp_value TEXT DEFAULT (strftime('%s','now')),

    -- Blob type
    metadata BLOB
);

CREATE TABLE notes (
    id INTEGER PRIMARY KEY,
    task_id INTEGER REFERENCES tasks(id) ON DELETE CASCADE,
    content TEXT NOT NULL,
    created_at TEXT DEFAULT CURRENT_DATE
);

CREATE INDEX idx_tasks_status ON tasks(status);
CREATE UNIQUE INDEX idx_tasks_title ON tasks(title);
