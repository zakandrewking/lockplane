package sqlite

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/lockplane/lockplane/database"
)

func getTestDB(t *testing.T) *sql.DB {
	t.Helper()

	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open SQLite: %v", err)
	}

	// Enable foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		_ = db.Close()
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	return db
}

func TestIntrospector_GetTables(t *testing.T) {
	db := getTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create test tables
	_, err := db.ExecContext(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            email TEXT NOT NULL
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.ExecContext(ctx, `
        CREATE TABLE posts (
            id INTEGER PRIMARY KEY,
            title TEXT NOT NULL
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Test introspection
	tables, err := introspector.GetTables(ctx, db)
	if err != nil {
		t.Fatalf("GetTables failed: %v", err)
	}

	// Verify we got both tables
	if len(tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(tables))
	}

	foundUsers := false
	foundPosts := false
	for _, table := range tables {
		if table == "users" {
			foundUsers = true
		}
		if table == "posts" {
			foundPosts = true
		}
	}

	if !foundUsers {
		t.Error("Expected to find 'users' table")
	}
	if !foundPosts {
		t.Error("Expected to find 'posts' table")
	}
}

func TestIntrospector_GetColumns(t *testing.T) {
	db := getTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create test table with various column types
	_, err := db.ExecContext(ctx, `
        CREATE TABLE test_columns (
            id INTEGER PRIMARY KEY,
            name TEXT NOT NULL,
            age INTEGER,
            score REAL DEFAULT 0.0,
            created_at TEXT DEFAULT CURRENT_TIMESTAMP
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	// Get columns
	columns, err := introspector.GetColumns(ctx, db, "test_columns")
	if err != nil {
		t.Fatalf("GetColumns failed: %v", err)
	}

	if len(columns) != 5 {
		t.Errorf("Expected 5 columns, got %d", len(columns))
	}

	// Verify column details
	idCol := findColumn(columns, "id")
	if idCol == nil {
		t.Fatal("Expected to find 'id' column")
	}
	if !idCol.IsPrimaryKey {
		t.Error("Expected id to be primary key")
	}
	// Note: SQLite reports PRIMARY KEY columns as nullable in PRAGMA table_info
	// unless they explicitly have NOT NULL. This is a SQLite quirk.
	// In practice, PRIMARY KEY columns are always NOT NULL in SQLite.

	nameCol := findColumn(columns, "name")
	if nameCol == nil {
		t.Fatal("Expected to find 'name' column")
	}
	if nameCol.Nullable {
		t.Error("Expected name to be NOT NULL")
	}

	ageCol := findColumn(columns, "age")
	if ageCol == nil {
		t.Fatal("Expected to find 'age' column")
	}
	if !ageCol.Nullable {
		t.Error("Expected age to be nullable")
	}

	scoreCol := findColumn(columns, "score")
	if scoreCol == nil {
		t.Fatal("Expected to find 'score' column")
	}
	if scoreCol.Default == nil {
		t.Error("Expected score to have a default value")
	}
}

func TestIntrospector_GetIndexes(t *testing.T) {
	db := getTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create table with index
	_, err := db.ExecContext(ctx, `
        CREATE TABLE test_indexes (
            id INTEGER PRIMARY KEY,
            email TEXT,
            username TEXT
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create table: %v", err)
	}

	_, err = db.ExecContext(ctx, "CREATE UNIQUE INDEX idx_email ON test_indexes (email)")
	if err != nil {
		t.Fatalf("Failed to create unique index: %v", err)
	}

	_, err = db.ExecContext(ctx, "CREATE INDEX idx_username ON test_indexes (username)")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Get indexes
	indexes, err := introspector.GetIndexes(ctx, db, "test_indexes")
	if err != nil {
		t.Fatalf("GetIndexes failed: %v", err)
	}

	// Verify indexes (should have 2 - not counting auto-created primary key index)
	foundEmailIdx := false
	foundUsernameIdx := false

	for _, idx := range indexes {
		if idx.Name == "idx_email" {
			foundEmailIdx = true
			if !idx.Unique {
				t.Error("Expected idx_email to be unique")
			}
		}
		if idx.Name == "idx_username" {
			foundUsernameIdx = true
			if idx.Unique {
				t.Error("Expected idx_username to be non-unique")
			}
		}
	}

	if !foundEmailIdx {
		t.Error("Expected to find idx_email index")
	}
	if !foundUsernameIdx {
		t.Error("Expected to find idx_username index")
	}
}

func TestIntrospector_GetForeignKeys(t *testing.T) {
	db := getTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create tables with foreign key
	_, err := db.ExecContext(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            email TEXT NOT NULL
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = db.ExecContext(ctx, `
        CREATE TABLE posts (
            id INTEGER PRIMARY KEY,
            user_id INTEGER NOT NULL,
            title TEXT NOT NULL,
            FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
        )
    `)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	// Get foreign keys
	foreignKeys, err := introspector.GetForeignKeys(ctx, db, "posts")
	if err != nil {
		t.Fatalf("GetForeignKeys failed: %v", err)
	}

	if len(foreignKeys) != 1 {
		t.Fatalf("Expected 1 foreign key, got %d", len(foreignKeys))
	}

	fk := foreignKeys[0]
	if len(fk.Columns) != 1 || fk.Columns[0] != "user_id" {
		t.Errorf("Expected column 'user_id', got %v", fk.Columns)
	}
	if fk.ReferencedTable != "users" {
		t.Errorf("Expected referenced table 'users', got '%s'", fk.ReferencedTable)
	}
	if len(fk.ReferencedColumns) != 1 || fk.ReferencedColumns[0] != "id" {
		t.Errorf("Expected referenced column 'id', got %v", fk.ReferencedColumns)
	}
	if fk.OnDelete == nil || *fk.OnDelete != "CASCADE" {
		if fk.OnDelete == nil {
			t.Error("Expected ON DELETE CASCADE, got nil")
		} else {
			t.Errorf("Expected ON DELETE CASCADE, got '%s'", *fk.OnDelete)
		}
	}
}

func TestIntrospector_IntrospectSchema(t *testing.T) {
	db := getTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Create comprehensive test schema
	_, err := db.ExecContext(ctx, `
        CREATE TABLE users (
            id INTEGER PRIMARY KEY,
            email TEXT NOT NULL UNIQUE,
            username TEXT NOT NULL,
            created_at TEXT DEFAULT CURRENT_TIMESTAMP
        );
    `)
	if err != nil {
		t.Fatalf("Failed to create users table: %v", err)
	}

	_, err = db.ExecContext(ctx, `
        CREATE TABLE posts (
            id INTEGER PRIMARY KEY,
            user_id INTEGER NOT NULL,
            title TEXT NOT NULL,
            content TEXT,
            published INTEGER DEFAULT 0,
            FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE
        );
    `)
	if err != nil {
		t.Fatalf("Failed to create posts table: %v", err)
	}

	_, err = db.ExecContext(ctx, "CREATE INDEX idx_posts_user_id ON posts (user_id)")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	_, err = db.ExecContext(ctx, "CREATE INDEX idx_users_username ON users (username)")
	if err != nil {
		t.Fatalf("Failed to create index: %v", err)
	}

	// Introspect full schema
	schema, err := introspector.IntrospectSchema(ctx, db)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	if len(schema.Tables) != 2 {
		t.Errorf("Expected 2 tables, got %d", len(schema.Tables))
	}

	// Verify users table
	usersTable := findTable(schema.Tables, "users")
	if usersTable == nil {
		t.Fatal("Expected to find users table")
	}
	if len(usersTable.Columns) != 4 {
		t.Errorf("Expected 4 columns in users, got %d", len(usersTable.Columns))
	}

	// Verify email column exists
	emailCol := findColumnInTable(*usersTable, "email")
	if emailCol == nil {
		t.Fatal("Expected to find email column")
	}

	// Note: UNIQUE constraints create auto-generated indexes in SQLite.
	// The introspector only returns user-created indexes (origin="c"),
	// not auto-generated ones. This is intentional to distinguish between
	// explicit indexes and constraint-generated indexes.

	// Verify posts table and foreign key
	postsTable := findTable(schema.Tables, "posts")
	if postsTable == nil {
		t.Fatal("Expected to find posts table")
	}
	if len(postsTable.Columns) != 5 {
		t.Errorf("Expected 5 columns in posts, got %d", len(postsTable.Columns))
	}
	if len(postsTable.ForeignKeys) != 1 {
		t.Errorf("Expected 1 foreign key in posts, got %d", len(postsTable.ForeignKeys))
	}

	// Verify indexes
	foundPostsUserIdx := false
	for _, idx := range postsTable.Indexes {
		if idx.Name == "idx_posts_user_id" {
			foundPostsUserIdx = true
		}
	}
	if !foundPostsUserIdx {
		t.Error("Expected to find idx_posts_user_id index")
	}
}

func TestIntrospector_EmptyDatabase(t *testing.T) {
	db := getTestDB(t)
	defer func() { _ = db.Close() }()

	ctx := context.Background()
	introspector := NewIntrospector()

	// Introspect empty database
	schema, err := introspector.IntrospectSchema(ctx, db)
	if err != nil {
		t.Fatalf("IntrospectSchema failed: %v", err)
	}

	if schema == nil {
		t.Fatal("Expected non-nil schema")
	}

	if len(schema.Tables) != 0 {
		t.Errorf("Expected 0 tables in empty database, got %d", len(schema.Tables))
	}
}

// Helper functions
func findColumn(columns []database.Column, name string) *database.Column {
	for i := range columns {
		if columns[i].Name == name {
			return &columns[i]
		}
	}
	return nil
}

func findColumnInTable(table database.Table, name string) *database.Column {
	for i := range table.Columns {
		if table.Columns[i].Name == name {
			return &table.Columns[i]
		}
	}
	return nil
}

func findTable(tables []database.Table, name string) *database.Table {
	for i := range tables {
		if tables[i].Name == name {
			return &tables[i]
		}
	}
	return nil
}
