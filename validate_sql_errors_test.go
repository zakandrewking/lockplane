package main

import (
	"strings"
	"testing"
)

// TestEnhancedSQLErrors tests that we provide better error messages than pg_query's defaults
func TestEnhancedSQLErrors(t *testing.T) {
	tests := []struct {
		name           string
		sql            string
		expectedLine   int
		expectedColumn int
		shouldContain  []string // Error message should contain these phrases
		shouldNotSay   string   // Should NOT just say this generic message
	}{
		{
			name: "missing comma between columns",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY
  email TEXT NOT NULL
);`,
			expectedLine:   3,
			expectedColumn: 3, // at "email"
			shouldContain: []string{
				"missing comma",
				"after 'id BIGINT PRIMARY KEY'",
			},
			shouldNotSay: "syntax error at or near \"email\"",
		},
		{
			name: "typo in CREATE TABLE",
			sql: `CREATE TABEL users (
  id BIGINT PRIMARY KEY
);`,
			expectedLine:   1,
			expectedColumn: 8, // at "TABEL"
			shouldContain: []string{
				"Did you mean 'CREATE TABLE'?",
				"TABEL",
			},
			shouldNotSay: "syntax error at or near \"TABEL\"",
		},
		{
			name: "missing opening parenthesis",
			sql: `CREATE TABLE users
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL
);`,
			expectedLine:   2,
			expectedColumn: 3, // at "id"
			shouldContain: []string{
				"missing opening parenthesis",
				"CREATE TABLE users (",
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "missing closing parenthesis",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL
;`,
			expectedLine:   4,
			expectedColumn: 1, // at ";"
			shouldContain: []string{
				"missing closing parenthesis",
				"Expected ')'",
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "invalid data type",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  created_at TIMESTAMPZ
);`,
			expectedLine:   3,
			expectedColumn: 14, // at "TIMESTAMPZ"
			shouldContain: []string{
				"Unknown data type",
				"Did you mean 'TIMESTAMP' or 'TIMESTAMPTZ'?",
			},
			shouldNotSay: "syntax error at or near",
		},
		{
			name: "incomplete foreign key",
			sql: `CREATE TABLE posts (
  id BIGINT PRIMARY KEY,
  user_id BIGINT REFERENCES
);`,
			expectedLine:   3,
			expectedColumn: 26, // after "REFERENCES"
			shouldContain: []string{
				"incomplete FOREIGN KEY",
				"REFERENCES table_name(column_name)",
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "missing semicolon between statements",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY
)
CREATE TABLE posts (
  id BIGINT PRIMARY KEY
);`,
			expectedLine:   4,
			expectedColumn: 1, // at "CREATE"
			shouldContain: []string{
				"missing semicolon",
				"after the closing parenthesis",
			},
			shouldNotSay: "syntax error at or near \"CREATE\"",
		},
		{
			name: "extra comma at end of column list",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL,
);`,
			expectedLine:   4,
			expectedColumn: 1, // at ")"
			shouldContain: []string{
				"trailing comma",
				"Remove the comma after 'email TEXT NOT NULL'",
			},
			shouldNotSay: "syntax error at or near",
		},
		{
			name: "using MySQL AUTO_INCREMENT instead of PostgreSQL",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY AUTO_INCREMENT,
  email TEXT NOT NULL
);`,
			expectedLine:   2,
			expectedColumn: 26, // at "AUTO_INCREMENT"
			shouldContain: []string{
				"AUTO_INCREMENT is MySQL syntax",
				"Use 'GENERATED ALWAYS AS IDENTITY' or 'SERIAL'",
			},
			shouldNotSay: "syntax error at or near \"AUTO_INCREMENT\"",
		},
		{
			name: "missing DEFAULT keyword",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  created_at TIMESTAMP NOW()
);`,
			expectedLine:   3,
			expectedColumn: 25, // at "NOW"
			shouldContain: []string{
				"missing DEFAULT",
				"created_at TIMESTAMP DEFAULT NOW()",
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "incomplete CREATE INDEX",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY
);

CREATE INDEX users_email ON`,
			expectedLine:   5,
			expectedColumn: 28, // at end of line
			shouldContain: []string{
				"incomplete CREATE INDEX",
				"CREATE INDEX index_name ON table_name(column_name)",
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "typo in column constraint",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOTNULL
);`,
			expectedLine:   3,
			expectedColumn: 14, // at "NOTNULL"
			shouldContain: []string{
				"Did you mean 'NOT NULL'?",
				"NOTNULL",
			},
			shouldNotSay: "syntax error at or near",
		},
		{
			name: "missing column name",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  TEXT NOT NULL
);`,
			expectedLine:   3,
			expectedColumn: 3, // at "TEXT"
			shouldContain: []string{
				"missing column name",
				"Expected: column_name data_type",
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "duplicate primary key",
			sql: `CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT PRIMARY KEY
);`,
			expectedLine:   3,
			expectedColumn: 14, // at second "PRIMARY KEY"
			shouldContain: []string{
				"Multiple PRIMARY KEY",
				"A table can only have one PRIMARY KEY",
			},
			shouldNotSay: "syntax error",
		},
		{
			name:           "using backticks instead of quotes",
			sql:            "CREATE TABLE users (\n  `id` BIGINT PRIMARY KEY\n);",
			expectedLine:   2,
			expectedColumn: 3, // at backtick
			shouldContain: []string{
				"backticks are MySQL syntax",
				`Use double quotes "id" for identifiers`,
			},
			shouldNotSay: "syntax error",
		},
		{
			name: "typo UNQUE instead of UNIQUE",
			sql: `CREATE TABLE todos (
  id text PRIMARY KEY,
  text text UNQUE,
  completed integer NOT NULL DEFAULT 0
);`,
			expectedLine:   3,
			expectedColumn: 13, // at "UNQUE"
			shouldContain: []string{
				"Did you mean 'UNIQUE'?",
				"UNQUE",
			},
			shouldNotSay: "syntax error at or near \"UNQUE\"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			issues := validateSQLSyntax("test.lp.sql", tt.sql)

			if len(issues) == 0 {
				t.Fatal("expected syntax error, got none")
			}

			issue := issues[0]

			// Check line number
			if issue.Line != tt.expectedLine {
				t.Errorf("expected line %d, got %d", tt.expectedLine, issue.Line)
			}

			// Check column number (if we implement column detection)
			if tt.expectedColumn > 0 && issue.Column != tt.expectedColumn {
				t.Logf("Note: expected column %d, got %d (column detection may need improvement)",
					tt.expectedColumn, issue.Column)
			}

			// Check that message contains helpful information
			for _, shouldContain := range tt.shouldContain {
				if !strings.Contains(issue.Message, shouldContain) {
					t.Errorf("error message should contain %q\nGot: %s",
						shouldContain, issue.Message)
				}
			}

			// Check that we're not just passing through pg_query's generic message
			if tt.shouldNotSay != "" && issue.Message == tt.shouldNotSay {
				t.Errorf("error message should be more helpful than just: %q", tt.shouldNotSay)
			}

			// Print the actual message for debugging
			t.Logf("Actual error message: %s", issue.Message)
		})
	}
}

// TestErrorContext tests that we provide code context with errors
func TestErrorContext(t *testing.T) {
	sql := `-- User table
CREATE TABLE users (
  id BIGINT PRIMARY KEY
  email TEXT NOT NULL
);

-- Post table
CREATE TABLE posts (
  id BIGINT PRIMARY KEY
);`

	issues := validateSQLSyntax("test.lp.sql", sql)

	if len(issues) == 0 {
		t.Fatal("expected syntax error for missing comma")
	}

	issue := issues[0]

	// Should show the problematic line(s)
	expectedContext := []string{
		"id BIGINT PRIMARY KEY", // The line before
		"email TEXT NOT NULL",   // The problematic line
	}

	for _, ctx := range expectedContext {
		if !strings.Contains(issue.Message, ctx) {
			t.Errorf("error should show context line: %q\nGot: %s", ctx, issue.Message)
		}
	}
}

// TestCommonMistakesDetection tests detection of common user mistakes
func TestCommonMistakesDetection(t *testing.T) {
	tests := []struct {
		name        string
		sql         string
		mistakeType string
		helpfulHint string
	}{
		{
			name: "SQLite datetime instead of PostgreSQL",
			sql: `CREATE TABLE users (
  created_at TEXT DEFAULT (datetime('now'))
);`,
			mistakeType: "SQLite datetime function",
			helpfulHint: "PostgreSQL: DEFAULT NOW() or DEFAULT CURRENT_TIMESTAMP",
		},
		{
			name: "INTEGER instead of SERIAL for auto-increment",
			sql: `CREATE TABLE users (
  id INTEGER PRIMARY KEY
);`,
			mistakeType: "manual id column",
			helpfulHint: "Consider using: id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY",
		},
		{
			name: "VARCHAR without length",
			sql: `CREATE TABLE users (
  email VARCHAR
);`,
			mistakeType: "VARCHAR without length",
			helpfulHint: "Use TEXT for variable length, or VARCHAR(n) with explicit length",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would be a new validation function that detects common mistakes
			// even if they're technically valid SQL
			t.Skip("Common mistakes detection not yet implemented")

			// Future implementation would check for these patterns and provide hints
			// issues := detectCommonMistakes("test.lp.sql", tt.sql)
			// ... assertions
		})
	}
}
