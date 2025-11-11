package main

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite"
)

// isSQLiteFilePath checks if a string looks like a SQLite file path
func isSQLiteFilePath(s string) bool {
	s = strings.ToLower(s)

	// Skip special cases
	if s == ":memory:" || strings.HasPrefix(s, "libsql://") {
		return false
	}

	// Check for sqlite:// prefix
	if strings.HasPrefix(s, "sqlite://") {
		return true
	}

	// Check for file: prefix
	if strings.HasPrefix(s, "file:") {
		return true
	}

	// Check for common SQLite file extensions
	if strings.HasSuffix(s, ".db") ||
		strings.HasSuffix(s, ".sqlite") ||
		strings.HasSuffix(s, ".sqlite3") {
		return true
	}

	return false
}

// extractSQLiteFilePath extracts the actual file path from a SQLite connection string
func extractSQLiteFilePath(connStr string) string {
	// Handle sqlite:// prefix
	if strings.HasPrefix(connStr, "sqlite://") {
		path := strings.TrimPrefix(connStr, "sqlite://")
		// Remove query parameters
		if idx := strings.Index(path, "?"); idx >= 0 {
			path = path[:idx]
		}
		return path
	}

	// Handle file: prefix
	if strings.HasPrefix(connStr, "file:") {
		path := strings.TrimPrefix(connStr, "file:")
		// Remove query parameters
		if idx := strings.Index(path, "?"); idx >= 0 {
			path = path[:idx]
		}
		return path
	}

	// Otherwise, it's already a file path
	return connStr
}

// checkSQLiteDatabase checks if a SQLite database file exists and is valid
// Returns (exists, isEmpty, error)
func checkSQLiteDatabase(connStr string) (exists bool, isEmpty bool, err error) {
	filePath := extractSQLiteFilePath(connStr)

	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if it's actually a file
	if info.IsDir() {
		return false, false, fmt.Errorf("path is a directory, not a file: %s", filePath)
	}

	// Check if file is empty
	if info.Size() == 0 {
		return true, true, nil
	}

	// Try to open it as a SQLite database
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return true, false, fmt.Errorf("failed to open database: %w", err)
	}
	defer func() { _ = db.Close() }()

	// Try to query it to see if it's valid
	if err := db.Ping(); err != nil {
		// File exists but isn't a valid SQLite database
		return true, false, fmt.Errorf("file exists but is not a valid SQLite database: %w", err)
	}

	return true, false, nil
}

// createSQLiteDatabase creates an empty SQLite database file
func createSQLiteDatabase(connStr string) error {
	filePath := extractSQLiteFilePath(connStr)
	
	// Create parent directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	
	// Create the database
	db, err := sql.Open("sqlite", connStr)
	if err != nil {
		return fmt.Errorf("failed to create database: %w", err)
	}
	defer func() { _ = db.Close() }()
	
	// Initialize the database by creating a minimal table
	// SQLite won't create the file until we actually write something
	// We create and immediately drop a table to ensure the file is created
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS _lockplane_init (id INTEGER PRIMARY KEY); DROP TABLE IF EXISTS _lockplane_init;")
	if err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}
	
	return nil
}

// ensureSQLiteDatabase checks if a SQLite database exists and offers to create it
func ensureSQLiteDatabase(connStr string, dbName string, autoCreate bool) error {
	if !isSQLiteFilePath(connStr) {
		return nil // Not a SQLite file path, skip
	}

	exists, isEmpty, err := checkSQLiteDatabase(connStr)
	if err != nil {
		return err
	}

	filePath := extractSQLiteFilePath(connStr)

	if !exists {
		if autoCreate {
			fmt.Fprintf(os.Stderr, "📁 Creating %s database: %s\n", dbName, filePath)
			if err := createSQLiteDatabase(connStr); err != nil {
				return fmt.Errorf("failed to create %s database: %w", dbName, err)
			}
			fmt.Fprintf(os.Stderr, "✓ Created %s database\n", dbName)
			return nil
		}

		fmt.Fprintf(os.Stderr, "\n⚠️  %s database file does not exist: %s\n", dbName, filePath)
		fmt.Fprintf(os.Stderr, "Would you like to create it? [Y/n]: ")

		var response string
		_, _ = fmt.Scanln(&response)
		response = strings.ToLower(strings.TrimSpace(response))

		if response == "" || response == "y" || response == "yes" {
			if err := createSQLiteDatabase(connStr); err != nil {
				return fmt.Errorf("failed to create %s database: %w", dbName, err)
			}
			fmt.Fprintf(os.Stderr, "✓ Created %s database: %s\n", dbName, filePath)
			return nil
		}

		return fmt.Errorf("%s database file does not exist: %s", dbName, filePath)
	}

	if isEmpty {
		fmt.Fprintf(os.Stderr, "⚠️  Warning: %s database file exists but is empty: %s\n", dbName, filePath)
		fmt.Fprintf(os.Stderr, "Initializing empty database...\n")
		if err := createSQLiteDatabase(connStr); err != nil {
			return fmt.Errorf("failed to initialize %s database: %w", dbName, err)
		}
		fmt.Fprintf(os.Stderr, "✓ Initialized %s database\n", dbName)
	}

	return nil
}
