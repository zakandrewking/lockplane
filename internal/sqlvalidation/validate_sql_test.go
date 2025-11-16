package sqlvalidation

import (
	"strings"
	"testing"
)

func TestValidateSQLSyntax_LineNumbers(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedLine int
		expectedMsg  string
	}{
		{
			name: "syntax error after blank lines",
			sql: `-- Comment
CREATE TABLE projects (
  id TEXT PRIMARY KEY
);

-- Another comment
CREATE ha TABLE todos (
  id TEXT PRIMARY KEY
);`,
			expectedLine: 7, // CREATE ha TABLE (line 7 in the SQL string)
			expectedMsg:  "syntax error at or near \"ha\"",
		},
		{
			name: "syntax error on first statement",
			sql: `CREATE ha TABLE users (
  id TEXT PRIMARY KEY
);`,
			expectedLine: 1,
			expectedMsg:  "syntax error at or near \"ha\"",
		},
		{
			name: "syntax error with multiple blank lines",
			sql: `CREATE TABLE valid (
  id TEXT PRIMARY KEY
);



CREATE INVALID syntax here (
  id TEXT PRIMARY KEY
);`,
			expectedLine: 7, // CREATE INVALID (line 7 in the SQL string)
			expectedMsg:  "syntax error at or near \"INVALID\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := validateSQLSyntax("test.lp.sql", tt.sql)

			if len(issues) == 0 {
				t.Fatal("expected syntax error, got none")
			}

			if issues[0].Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, issues[0].Line)
			}

			if !strings.Contains(issues[0].Message, tt.expectedMsg) {
				t.Errorf("expected message to contain %q, got %q", tt.expectedMsg, issues[0].Message)
			}
		})
	}
}

func TestValidateDangerousPatterns_LineNumbers(t *testing.T) {
	tests := []struct {
		name         string
		sql          string
		expectedLine int
		expectedCode string
	}{
		{
			name: "DROP TABLE after blank lines",
			sql: `CREATE TABLE users (
  id TEXT PRIMARY KEY
);


DROP TABLE users;`,
			expectedLine: 6, // DROP TABLE
			expectedCode: "dangerous_drop_table",
		},
		{
			name: "TRUNCATE after comments",
			sql: `CREATE TABLE users (
  id TEXT PRIMARY KEY
);
-- This is a comment
-- Another comment
TRUNCATE TABLE users;`,
			expectedLine: 6, // TRUNCATE
			expectedCode: "dangerous_truncate",
		},
		{
			name: "DELETE without WHERE after blank lines",
			sql: `CREATE TABLE users (
  id TEXT PRIMARY KEY
);

DELETE FROM users;`,
			expectedLine: 5, // DELETE
			expectedCode: "dangerous_delete_all",
		},
		{
			name: "DROP COLUMN",
			sql: `CREATE TABLE users (
  id TEXT PRIMARY KEY,
  name TEXT
);

ALTER TABLE users DROP COLUMN name;`,
			expectedLine: 6, // ALTER TABLE
			expectedCode: "dangerous_drop_column",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := validateDangerousPatterns("test.lp.sql", tt.sql)

			if len(issues) == 0 {
				t.Fatal("expected dangerous pattern error, got none")
			}

			if issues[0].Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, issues[0].Line)
			}

			if issues[0].Code != tt.expectedCode {
				t.Errorf("expected code %q, got %q", tt.expectedCode, issues[0].Code)
			}
		})
	}
}

func TestValidateSQLFile_Integration(t *testing.T) {
	// Integration test that validates a complete file
	sql := `-- Declarative Lockplane schema for the todos table.
-- Projects table to organize todos
CREATE TABLE projects(
  id text PRIMARY KEY,
  name text NOT NULL,
  description text,
  created_at text NOT NULL DEFAULT (datetime('now'))
);

-- Todos table with optional project relationship
CREATE TABLE todos(
  id text PRIMARY KEY,
  text text NOT NULL,
  completed integer NOT NULL DEFAULT 0,
  project_id text,
  created_at text NOT NULL DEFAULT (datetime('now')),
  FOREIGN KEY (project_id) REFERENCES projects(id) ON DELETE SET NULL
);

DROP TABLE todos;`

	issues := validateDangerousPatterns("test.lp.sql", sql)

	// Should find the DROP TABLE error on line 20
	found := false
	for _, issue := range issues {
		if issue.Code == "dangerous_drop_table" {
			found = true
			if issue.Line != 20 {
				t.Errorf("expected DROP TABLE error on line 20, got line %d", issue.Line)
			}
		}
	}

	if !found {
		t.Error("expected to find DROP TABLE error")
	}
}

// TestvalidateSQLSyntax_BlankLinesBeforeError ensures that line numbers
// are correctly reported when there are blank lines before a syntax error.
// Regression test for issue where line numbers were offset by the number
// of leading blank lines in a statement.
func TestValidateSQLSyntax_BlankLinesBeforeError(t *testing.T) {
	sql := `CREATE INDEX idx_genomes_species ON genomes(species);

CRETE INDEX idx_genomes_name ON genomes(name);

ALTER TABLE genomes ENABLE ROW LEVEL SECURITY;`

	issues := validateSQLSyntax("test.sql", sql)

	if len(issues) == 0 {
		t.Fatal("expected validation issues, got none")
	}

	// The error "CRETE" is on line 3, not line 5
	if issues[0].Line != 3 {
		t.Errorf("expected error on line 3, got line %d", issues[0].Line)
	}

	if !strings.Contains(issues[0].Message, "CRETE") {
		t.Errorf("expected error message to mention 'CRETE', got: %s", issues[0].Message)
	}
}
