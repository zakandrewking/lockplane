package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/lockplane/lockplane/database"
)

func TestSplitSQLStatements(t *testing.T) {
	tests := []struct {
		name     string
		sql      string
		expected []SQLStatement
	}{
		{
			name: "single statement",
			sql:  "CREATE TABLE users(id int);",
			expected: []SQLStatement{
				{Text: "CREATE TABLE users(id int);", StartLine: 1},
			},
		},
		{
			name: "multiple statements",
			sql: `CREATE TABLE users(id int);
CREATE TABLE posts(id int);`,
			expected: []SQLStatement{
				{Text: "CREATE TABLE users(id int);", StartLine: 1},
				{Text: "\nCREATE TABLE posts(id int);", StartLine: 2},
			},
		},
		{
			name: "statements with blank lines",
			sql: `CREATE TABLE users(id int);

CREATE TABLE posts(id int);`,
			expected: []SQLStatement{
				{Text: "CREATE TABLE users(id int);", StartLine: 1},
				{Text: "\n\nCREATE TABLE posts(id int);", StartLine: 3},
			},
		},
		{
			name: "semicolon in string literal",
			sql:  `CREATE TABLE test(name text DEFAULT 'foo;bar');`,
			expected: []SQLStatement{
				{Text: `CREATE TABLE test(name text DEFAULT 'foo;bar');`, StartLine: 1},
			},
		},
		{
			name: "comment with semicolon",
			sql: `-- This is a comment with ; inside
CREATE TABLE users(id int);`,
			expected: []SQLStatement{
				{Text: "-- This is a comment with ; inside\nCREATE TABLE users(id int);", StartLine: 1},
			},
		},
		{
			name: "multiple statements with comments",
			sql: `CREATE TABLE users(id int); -- comment
CREATE TABLE posts(id int);`,
			expected: []SQLStatement{
				{Text: "CREATE TABLE users(id int);", StartLine: 1},
				{Text: " -- comment\nCREATE TABLE posts(id int);", StartLine: 1}, // Space after semicolon is on line 1
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitSQLStatements(tt.sql)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d statements, got %d", len(tt.expected), len(result))
				return
			}
			for i, stmt := range result {
				if stmt.Text != tt.expected[i].Text {
					t.Errorf("statement %d text mismatch:\nexpected: %q\ngot:      %q",
						i, tt.expected[i].Text, stmt.Text)
				}
				if stmt.StartLine != tt.expected[i].StartLine {
					t.Errorf("statement %d line mismatch: expected %d, got %d",
						i, tt.expected[i].StartLine, stmt.StartLine)
				}
			}
		})
	}
}

func TestPreValidateSQLSyntax_PostgreSQL(t *testing.T) {
	// Create temporary directory with test SQL files
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		files         map[string]string
		expectedCount int
		checkErrors   func(t *testing.T, errors []SyntaxError)
	}{
		{
			name: "valid SQL",
			files: map[string]string{
				"schema.sql": "CREATE TABLE users(id serial PRIMARY KEY, email text NOT NULL);",
			},
			expectedCount: 0,
		},
		{
			name: "single syntax error",
			files: map[string]string{
				"schema.sql": "CEATE TABLE users(id int);",
			},
			expectedCount: 1,
			checkErrors: func(t *testing.T, errors []SyntaxError) {
				if errors[0].Line != 1 {
					t.Errorf("expected error on line 1, got line %d", errors[0].Line)
				}
				if errors[0].Message != `syntax error at or near "CEATE"` {
					t.Errorf("unexpected error message: %s", errors[0].Message)
				}
			},
		},
		{
			name: "multiple syntax errors in one file",
			files: map[string]string{
				"schema.sql": `CREATE TABLE users(id int);
CEATE INDEX idx1 ON users(id);
CEATE INDEX idx2 ON users(id);`,
			},
			expectedCount: 2,
			checkErrors: func(t *testing.T, errors []SyntaxError) {
				if errors[0].Line != 2 {
					t.Errorf("expected first error on line 2, got line %d", errors[0].Line)
				}
				if errors[1].Line != 3 {
					t.Errorf("expected second error on line 3, got line %d", errors[1].Line)
				}
			},
		},
		{
			name: "errors across multiple files",
			files: map[string]string{
				"users.sql": "CEATE TABLE users(id int);",
				"posts.sql": "CEATE TABLE posts(id int);",
			},
			expectedCount: 2,
		},
		{
			name: "trailing comma error",
			files: map[string]string{
				"schema.sql": "CREATE TABLE test(id int,);",
			},
			expectedCount: 1,
		},
		{
			name: "valid with CURRENT_TIMESTAMP",
			files: map[string]string{
				"schema.sql": "CREATE TABLE events(created_at timestamp DEFAULT CURRENT_TIMESTAMP);",
			},
			expectedCount: 0,
		},
		{
			name: "valid with all SQL value functions",
			files: map[string]string{
				"schema.sql": `CREATE TABLE audit(
					date_col date DEFAULT CURRENT_DATE,
					time_col time DEFAULT CURRENT_TIME,
					ts_col timestamp DEFAULT CURRENT_TIMESTAMP,
					local_time time DEFAULT LOCALTIME,
					local_ts timestamp DEFAULT LOCALTIMESTAMP
				);`,
			},
			expectedCount: 0,
		},
		{
			name: "empty file",
			files: map[string]string{
				"empty.sql": "",
			},
			expectedCount: 0,
		},
		{
			name: "only comments",
			files: map[string]string{
				"comments.sql": "-- This is a comment\n-- Another comment",
			},
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directory
			testDir := filepath.Join(tmpDir, tt.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatal(err)
			}

			// Write test files
			for filename, content := range tt.files {
				path := filepath.Join(testDir, filename)
				if err := os.WriteFile(path, []byte(content), 0644); err != nil {
					t.Fatal(err)
				}
			}

			// Run validation
			errors := preValidateSQLSyntax(testDir, database.DialectPostgres)

			// Check error count
			if len(errors) != tt.expectedCount {
				t.Errorf("expected %d errors, got %d", tt.expectedCount, len(errors))
				for i, err := range errors {
					t.Logf("  Error %d: %s:%d:%d: %s", i+1, err.File, err.Line, err.Column, err.Message)
				}
			}

			// Run custom checks
			if tt.checkErrors != nil && len(errors) > 0 {
				tt.checkErrors(t, errors)
			}
		})
	}
}

func TestPreValidateSQLSyntax_MultipleStatementsAccuracy(t *testing.T) {
	tmpDir := t.TempDir()

	// Test that line numbers are accurate for multiple statements with blank lines
	content := `CREATE TABLE users(
    id serial PRIMARY KEY,
    name text NOT NULL
);

CEATE INDEX idx_users_name ON users(name);

CREATE TABLE posts(
    id serial PRIMARY KEY
);

CEATE INDEX idx_posts_id ON posts(id);`

	testFile := filepath.Join(tmpDir, "schema.sql")
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	errors := preValidateSQLSyntax(tmpDir, database.DialectPostgres)

	if len(errors) != 2 {
		t.Fatalf("expected 2 errors, got %d", len(errors))
	}

	// First error should be on line 6 (CEATE INDEX idx_users_name)
	if errors[0].Line != 6 {
		t.Errorf("expected first error on line 6, got line %d", errors[0].Line)
	}

	// Second error should be on line 12 (CEATE INDEX idx_posts_id)
	if errors[1].Line != 12 {
		t.Errorf("expected second error on line 12, got line %d", errors[1].Line)
	}
}

func TestPreValidateSQLSyntax_StringLiterals(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name          string
		sql           string
		expectedCount int
	}{
		{
			name:          "semicolon in single quotes",
			sql:           "CREATE TABLE test(val text DEFAULT 'foo;bar');",
			expectedCount: 0,
		},
		{
			name:          "semicolon in double quotes",
			sql:           `CREATE TABLE test(val text DEFAULT "foo;bar");`,
			expectedCount: 0,
		},
		{
			name:          "SQL keyword in string",
			sql:           "CREATE TABLE test(val text DEFAULT 'CREATE TABLE');",
			expectedCount: 0,
		},
		{
			name:          "multiple strings with semicolons",
			sql:           `CREATE TABLE test(a text DEFAULT 'foo;', b text DEFAULT 'bar;');`,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testFile := filepath.Join(tmpDir, "test.sql")
			if err := os.WriteFile(testFile, []byte(tt.sql), 0644); err != nil {
				t.Fatal(err)
			}

			errors := preValidateSQLSyntax(tmpDir, database.DialectPostgres)

			if len(errors) != tt.expectedCount {
				t.Errorf("expected %d errors, got %d", tt.expectedCount, len(errors))
				for _, err := range errors {
					t.Logf("  %s:%d:%d: %s", err.File, err.Line, err.Column, err.Message)
				}
			}

			// Cleanup
			_ = os.Remove(testFile)
		})
	}
}
