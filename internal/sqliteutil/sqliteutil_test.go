package sqliteutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsSQLiteFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"memory database", ":memory:", false},
		{"libsql URL", "libsql://mydb.turso.io", false},
		{"postgres URL", "postgres://localhost:5432/db", false},
		{"sqlite URL", "sqlite:///path/to/db.sqlite", true},
		{"file URL", "file:/path/to/db.db", true},
		{"db file", "myapp.db", true},
		{"sqlite file", "data.sqlite", true},
		{"sqlite3 file", "database.sqlite3", true},
		{"regular path", "/var/data/app.db", true},
		{"relative path", "./local.db", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsSQLiteFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("IsSQLiteFilePath(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestExtractSQLiteFilePath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"sqlite URL", "sqlite:///path/to/db.sqlite", "/path/to/db.sqlite"},
		{"sqlite URL with query", "sqlite:///path/to/db.sqlite?mode=ro", "/path/to/db.sqlite"},
		{"file URL", "file:/path/to/db.db", "/path/to/db.db"},
		{"file URL with query", "file:/path/to/db.db?mode=rw", "/path/to/db.db"},
		{"plain path", "/var/data/app.db", "/var/data/app.db"},
		{"relative path", "./local.db", "./local.db"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExtractSQLiteFilePath(tt.input)
			if result != tt.expected {
				t.Errorf("ExtractSQLiteFilePath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

func TestCheckSQLiteDatabase(t *testing.T) {
	// Create temp directory for tests
	tmpDir := t.TempDir()

	t.Run("non-existent file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nonexistent.db")
		exists, isEmpty, err := CheckSQLiteDatabase(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if exists {
			t.Error("expected exists=false for non-existent file")
		}
		if isEmpty {
			t.Error("expected isEmpty=false for non-existent file")
		}
	})

	t.Run("empty file", func(t *testing.T) {
		path := filepath.Join(tmpDir, "empty.db")
		if err := os.WriteFile(path, []byte{}, 0o644); err != nil {
			t.Fatalf("failed to create empty file: %v", err)
		}

		exists, isEmpty, err := CheckSQLiteDatabase(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected exists=true for empty file")
		}
		if !isEmpty {
			t.Error("expected isEmpty=true for empty file")
		}
	})

	t.Run("valid database", func(t *testing.T) {
		path := filepath.Join(tmpDir, "valid.db")
		if err := CreateSQLiteDatabase(path); err != nil {
			t.Fatalf("failed to create database: %v", err)
		}

		exists, isEmpty, err := CheckSQLiteDatabase(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !exists {
			t.Error("expected exists=true for valid database")
		}
		// Note: A freshly created SQLite database has a non-zero size
		// due to the database header, so isEmpty should be false
		if isEmpty {
			t.Error("expected isEmpty=false for freshly created database")
		}
	})
}

func TestCreateSQLiteDatabase(t *testing.T) {
	tmpDir := t.TempDir()

	t.Run("create in existing directory", func(t *testing.T) {
		path := filepath.Join(tmpDir, "test.db")
		if err := CreateSQLiteDatabase(path); err != nil {
			t.Fatalf("failed to create database: %v", err)
		}

		// Check file was created
		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("database file not created: %v", err)
		}
		// SQLite creates a database header, so size should be non-zero
		if info.Size() == 0 {
			t.Error("database file has zero size (expected SQLite header)")
		}

		// Check it's a valid database
		exists, isEmpty, err := CheckSQLiteDatabase(path)
		if err != nil {
			t.Fatalf("created database is not valid: %v", err)
		}
		if !exists {
			t.Error("created database should exist")
		}
		// A properly initialized SQLite database should not be empty
		if isEmpty {
			t.Error("created database should not be empty")
		}
	})

	t.Run("create with nested directory", func(t *testing.T) {
		path := filepath.Join(tmpDir, "nested", "dir", "test.db")
		if err := CreateSQLiteDatabase(path); err != nil {
			t.Fatalf("failed to create database: %v", err)
		}

		// Check file was created
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("database file not created: %v", err)
		}

		// Check parent directories were created
		parentDir := filepath.Join(tmpDir, "nested", "dir")
		if _, err := os.Stat(parentDir); err != nil {
			t.Fatalf("parent directory not created: %v", err)
		}
	})
}

func TestGenerateShadowDBPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"with .db extension", "lockplane.db", "lockplane_shadow.db"},
		{"with .sqlite extension", "data.sqlite", "data_shadow.sqlite"},
		{"with .sqlite3 extension", "test.sqlite3", "test_shadow.sqlite3"},
		{"with path", "./schema/lockplane.db", "./schema/lockplane_shadow.db"},
		{"with nested path", "/var/data/app.db", "/var/data/app_shadow.db"},
		{"no extension", "database", "database_shadow"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := GenerateShadowDBPath(tt.input)
			if result != tt.expected {
				t.Errorf("GenerateShadowDBPath(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}
