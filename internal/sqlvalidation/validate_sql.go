package sqlvalidation

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/schema"
)

// ValidationIssue represents a validation error or warning
type ValidationIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // "error" or "warning"
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
}

// SQLValidationResult contains all validation issues for SQL files
type SQLValidationResult struct {
	Valid  bool              `json:"valid"`
	Issues []ValidationIssue `json:"issues"`
}

func RunValidateSQL(args []string) {
	fs := flag.NewFlagSet("validate sql", flag.ExitOnError)
	formatFlag := fs.String("format", "text", "Output format: text or json")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane validate sql [options] <file-or-directory>\n\n")
		fmt.Fprintf(os.Stderr, "Validate a SQL schema file or directory of .lp.sql files.\n\n")
		fmt.Fprintf(os.Stderr, "ABOUT .lp.sql FILES:\n\n")
		fmt.Fprintf(os.Stderr, "  .lp.sql files are declarative schema definition files that describe your database\n")
		fmt.Fprintf(os.Stderr, "  structure using standard PostgreSQL DDL syntax. They serve as the source of truth\n")
		fmt.Fprintf(os.Stderr, "  for your schema and are used to generate migration plans.\n\n")
		fmt.Fprintf(os.Stderr, "  VALID SQL STATEMENTS:\n")
		fmt.Fprintf(os.Stderr, "    - CREATE TABLE - Define tables with columns, constraints, and defaults\n")
		fmt.Fprintf(os.Stderr, "    - CREATE INDEX - Create indexes (prefer CONCURRENTLY for production)\n")
		fmt.Fprintf(os.Stderr, "    - CREATE UNIQUE INDEX - Create unique indexes\n")
		fmt.Fprintf(os.Stderr, "    - ALTER TABLE - Add/modify columns, constraints, and foreign keys\n")
		fmt.Fprintf(os.Stderr, "    - Comments (-- and /* */) - Documentation and notes\n\n")
		fmt.Fprintf(os.Stderr, "  INVALID SQL STATEMENTS (rejected by validation):\n\n")
		fmt.Fprintf(os.Stderr, "    Data Loss Operations (ERROR):\n")
		fmt.Fprintf(os.Stderr, "      - DROP TABLE - Permanently deletes all data\n")
		fmt.Fprintf(os.Stderr, "      - DROP COLUMN - Irreversible data loss\n")
		fmt.Fprintf(os.Stderr, "      - TRUNCATE TABLE - Deletes all rows\n")
		fmt.Fprintf(os.Stderr, "      - DELETE without WHERE - Unintentional data deletion\n\n")
		fmt.Fprintf(os.Stderr, "    Non-Declarative Patterns (ERROR):\n")
		fmt.Fprintf(os.Stderr, "      - IF NOT EXISTS clauses - Makes schema non-deterministic\n")
		fmt.Fprintf(os.Stderr, "      - Transaction control (BEGIN, COMMIT, ROLLBACK) - Lockplane manages transactions\n")
		fmt.Fprintf(os.Stderr, "      - CREATE OR REPLACE - Use plain CREATE statements\n\n")
		fmt.Fprintf(os.Stderr, "  MULTI-FILE SUPPORT:\n\n")
		fmt.Fprintf(os.Stderr, "    You can organize schema across multiple .lp.sql files in a directory.\n")
		fmt.Fprintf(os.Stderr, "    Files are combined in lexicographic order, so use prefixes for explicit\n")
		fmt.Fprintf(os.Stderr, "    ordering (e.g., 001_tables.lp.sql, 010_indexes.lp.sql).\n\n")
		fmt.Fprintf(os.Stderr, "    Only top-level files are considered - subdirectories and symlinks are\n")
		fmt.Fprintf(os.Stderr, "    skipped to avoid accidental recursion.\n\n")
		fmt.Fprintf(os.Stderr, "  VALIDATION CHECKS:\n\n")
		fmt.Fprintf(os.Stderr, "    1. SQL Syntax - Statement-by-statement parsing using PostgreSQL parser\n")
		fmt.Fprintf(os.Stderr, "       - Detects multiple syntax errors in a single pass\n")
		fmt.Fprintf(os.Stderr, "       - Reports exact line numbers for each error\n\n")
		fmt.Fprintf(os.Stderr, "    2. database.Schema Structure - Referential integrity and consistency\n")
		fmt.Fprintf(os.Stderr, "       - Duplicate column names\n")
		fmt.Fprintf(os.Stderr, "       - Missing data types\n")
		fmt.Fprintf(os.Stderr, "       - Missing primary keys (warning)\n")
		fmt.Fprintf(os.Stderr, "       - Invalid foreign key references\n")
		fmt.Fprintf(os.Stderr, "       - Duplicate index names\n")
		fmt.Fprintf(os.Stderr, "       - Invalid index column references\n\n")
		fmt.Fprintf(os.Stderr, "    3. Dangerous Patterns - Data loss risks flagged as errors\n")
		fmt.Fprintf(os.Stderr, "    4. Non-Declarative Patterns - Imperative SQL not allowed\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Validate SQL schema file (text output)\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql schema.lp.sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate with JSON output (for IDE integration)\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql --format json schema.lp.sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate directory of SQL files\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql schema/\n\n")
		fmt.Fprintf(os.Stderr, "  # Example valid .lp.sql file:\n")
		fmt.Fprintf(os.Stderr, "  #   CREATE TABLE users (\n")
		fmt.Fprintf(os.Stderr, "  #     id BIGINT PRIMARY KEY,\n")
		fmt.Fprintf(os.Stderr, "  #     email TEXT NOT NULL,\n")
		fmt.Fprintf(os.Stderr, "  #     created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()\n")
		fmt.Fprintf(os.Stderr, "  #   );\n")
		fmt.Fprintf(os.Stderr, "  #   CREATE UNIQUE INDEX users_email_idx ON users(email);\n\n")
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	// Check if path is a directory
	info, err := os.Stat(path)
	if err != nil {
		log.Fatalf("Failed to access path: %v", err)
	}

	var allIssues []ValidationIssue

	if info.IsDir() {
		// Handle directory: validate all .lp.sql files
		entries, err := os.ReadDir(path)
		if err != nil {
			log.Fatalf("Failed to read directory: %v", err)
		}

		var sqlFiles []string
		for _, entry := range entries {
			if entry.IsDir() || entry.Type()&os.ModeSymlink != 0 {
				continue
			}
			if strings.HasSuffix(strings.ToLower(entry.Name()), ".lp.sql") {
				sqlFiles = append(sqlFiles, entry.Name())
			}
		}

		if len(sqlFiles) == 0 {
			log.Fatalf("No .lp.sql files found in directory: %s", path)
		}

		// Sort files for consistent ordering
		var sortedFiles []string
		copy(sortedFiles, sqlFiles)

		// Validate each file
		for _, fileName := range sqlFiles {
			filePath := filepath.Join(path, fileName)
			fileIssues := validateSQLFile(filePath)
			allIssues = append(allIssues, fileIssues...)
		}

		// Also validate schema structure across all files
		loadedSchema, err := schema.LoadSchema(path)
		if err == nil {
			structureIssues := validateSchemaStructure(loadedSchema, path)
			allIssues = append(allIssues, structureIssues...)
		}
	} else {
		// Handle single file
		allIssues = validateSQLFile(path)

		// Also validate schema structure
		loadedSchema, err := schema.LoadSchema(path)
		if err == nil {
			structureIssues := validateSchemaStructure(loadedSchema, path)
			allIssues = append(allIssues, structureIssues...)
		}
	}

	issues := allIssues

	// Output results
	if *formatFlag == "json" {
		result := SQLValidationResult{
			Valid:  len(issues) == 0,
			Issues: issues,
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal validation result: %v", err)
		}
		fmt.Println(string(jsonBytes))

		if !result.Valid {
			os.Exit(1)
		}
	} else {
		// Text output
		if len(issues) == 0 {
			fmt.Fprintf(os.Stderr, "✓ SQL schema is valid: %s\n", path)
		} else {
			fmt.Fprintf(os.Stderr, "✗ SQL schema validation failed: %s\n\n", path)
			for _, issue := range issues {
				severity := "ERROR"
				if issue.Severity == "warning" {
					severity = "WARNING"
				}
				fmt.Fprintf(os.Stderr, "%s:%d:%d: %s: %s\n",
					issue.File, issue.Line, issue.Column, severity, issue.Message)
			}
			os.Exit(1)
		}
	}
}

// validateSQLFile validates a single SQL file for syntax and dangerous patterns
func validateSQLFile(filePath string) []ValidationIssue {
	sqlContent, err := os.ReadFile(filePath)
	if err != nil {
		return []ValidationIssue{{
			File:     filePath,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  fmt.Sprintf("Failed to read file: %v", err),
			Code:     "file_read_error",
		}}
	}

	// First, do statement-by-statement syntax validation
	syntaxIssues := validateSQLSyntax(filePath, string(sqlContent))

	// Check for dangerous patterns (data loss operations, etc.)
	dangerousIssues := validateDangerousPatterns(filePath, string(sqlContent))

	// Combine all issues
	return append(syntaxIssues, dangerousIssues...)
}

func validateSchemaStructure(schema *database.Schema, path string) []ValidationIssue {
	var issues []ValidationIssue

	if schema == nil {
		issues = append(issues, ValidationIssue{
			File:     path,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  "database.Schema is empty",
		})
		return issues
	}

	// Track table names for foreign key validation
	tableNames := make(map[string]bool)
	for _, table := range schema.Tables {
		tableNames[table.Name] = true
	}

	// Validate each table
	for _, table := range schema.Tables {
		// Check table has at least one column
		if len(table.Columns) == 0 {
			issues = append(issues, ValidationIssue{
				File:     path,
				Line:     1,
				Column:   1,
				Severity: "warning",
				Message:  fmt.Sprintf("Table '%s' has no columns", table.Name),
				Code:     "empty_table",
			})
		}

		// Validate columns
		columnNames := make(map[string]bool)
		hasPrimaryKey := false

		for _, col := range table.Columns {
			// Check for duplicate column names
			if columnNames[col.Name] {
				issues = append(issues, ValidationIssue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Duplicate column name '%s' in table '%s'", col.Name, table.Name),
					Code:     "duplicate_column",
				})
			}
			columnNames[col.Name] = true

			if col.IsPrimaryKey {
				hasPrimaryKey = true
			}

			// Warn about missing data types (shouldn't happen with SQL parser, but check anyway)
			if col.Type == "" {
				issues = append(issues, ValidationIssue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Column '%s.%s' has no data type", table.Name, col.Name),
					Code:     "missing_type",
				})
			}
		}

		// Warn if table has no primary key
		if !hasPrimaryKey {
			issues = append(issues, ValidationIssue{
				File:     path,
				Line:     1,
				Column:   1,
				Severity: "warning",
				Message:  fmt.Sprintf("Table '%s' has no primary key", table.Name),
				Code:     "no_primary_key",
			})
		}

		// Validate foreign keys
		for _, fk := range table.ForeignKeys {
			// Check referenced table exists
			if !tableNames[fk.ReferencedTable] {
				issues = append(issues, ValidationIssue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "error",
					Message:  fmt.Sprintf("Foreign key '%s' in table '%s' references non-existent table '%s'", fk.Name, table.Name, fk.ReferencedTable),
					Code:     "invalid_fk_table",
				})
			}

			// Check foreign key columns exist
			for _, colName := range fk.Columns {
				if !columnNames[colName] {
					issues = append(issues, ValidationIssue{
						File:     path,
						Line:     1,
						Column:   1,
						Severity: "error",
						Message:  fmt.Sprintf("Foreign key '%s' in table '%s' references non-existent column '%s'", fk.Name, table.Name, colName),
						Code:     "invalid_fk_column",
					})
				}
			}
		}

		// Validate indexes
		indexNames := make(map[string]bool)
		for _, idx := range table.Indexes {
			// Check for duplicate index names
			if indexNames[idx.Name] {
				issues = append(issues, ValidationIssue{
					File:     path,
					Line:     1,
					Column:   1,
					Severity: "warning",
					Message:  fmt.Sprintf("Duplicate index name '%s' in table '%s'", idx.Name, table.Name),
					Code:     "duplicate_index",
				})
			}
			indexNames[idx.Name] = true

			// Check index columns exist
			for _, colName := range idx.Columns {
				if !columnNames[colName] {
					issues = append(issues, ValidationIssue{
						File:     path,
						Line:     1,
						Column:   1,
						Severity: "error",
						Message:  fmt.Sprintf("Index '%s' in table '%s' references non-existent column '%s'", idx.Name, table.Name, colName),
						Code:     "invalid_index_column",
					})
				}
			}
		}
	}

	return issues
}

// validateSQLSyntax validates SQL syntax statement by statement to find multiple errors
func validateSQLSyntax(filePath string, sqlContent string) []ValidationIssue {
	var issues []ValidationIssue

	// Proactive checks for issues that pg_query might not catch
	proactiveIssues := proactiveValidation(filePath, sqlContent)
	if len(proactiveIssues) > 0 {
		return proactiveIssues
	}

	// Try to parse the entire SQL first
	if _, err := pg_query.Parse(sqlContent); err == nil {
		// SQL is syntactically valid, now check for semantic issues
		semanticIssues := semanticValidation(filePath, sqlContent)
		return semanticIssues
	}

	// If full parse failed, validate statement by statement
	// Split by semicolons, but be careful about semicolons in strings/comments
	statements := splitSQLStatements(sqlContent)

	for _, stmt := range statements {
		trimmed := strings.TrimSpace(stmt.sql)

		// Skip empty statements
		if trimmed == "" {
			continue
		}

		// Skip comment-only statements (all lines are comments)
		lines := strings.Split(trimmed, "\n")
		allComments := true
		for _, line := range lines {
			lineTrimmmed := strings.TrimSpace(line)
			if lineTrimmmed != "" && !strings.HasPrefix(lineTrimmmed, "--") {
				allComments = false
				break
			}
		}
		if allComments {
			continue
		}

		// Try to parse this statement
		if _, err := pg_query.Parse(stmt.sql); err != nil {
			// Use enhanced error analysis to provide better messages
			enhancedIssues := enhanceSQLError(filePath, stmt.sql, err)

			// Adjust line numbers to account for statement position in full file
			for i := range enhancedIssues {
				if enhancedIssues[i].Line > 0 {
					enhancedIssues[i].Line += stmt.startLine - 1
				} else {
					enhancedIssues[i].Line = stmt.startLine
				}
			}

			issues = append(issues, enhancedIssues...)
		}
	}

	return issues
}

type sqlStatement struct {
	sql       string
	startLine int
}

// splitSQLStatements splits SQL into individual statements by semicolons
// while preserving line numbers for error reporting
func splitSQLStatements(sql string) []sqlStatement {
	var statements []sqlStatement
	var currentStmt strings.Builder
	currentLine := 1
	stmtStartLine := 1
	hasSeenNonWhitespace := false

	inSingleQuote := false
	inDoubleQuote := false
	inLineComment := false
	inBlockComment := false

	runes := []rune(sql)
	for i := 0; i < len(runes); i++ {
		ch := runes[i]

		// Handle newlines
		if ch == '\n' {
			currentLine++
			if inLineComment {
				inLineComment = false
			}
		}

		// Handle comments
		if !inSingleQuote && !inDoubleQuote {
			// Line comment start
			if !inBlockComment && i+1 < len(runes) && ch == '-' && runes[i+1] == '-' {
				inLineComment = true
			}
			// Block comment start
			if !inLineComment && i+1 < len(runes) && ch == '/' && runes[i+1] == '*' {
				inBlockComment = true
			}
			// Block comment end
			if inBlockComment && i+1 < len(runes) && ch == '*' && runes[i+1] == '/' {
				inBlockComment = false
				currentStmt.WriteRune(ch)
				i++
				if i < len(runes) {
					currentStmt.WriteRune(runes[i])
				}
				continue
			}
		}

		// Handle string literals
		if !inLineComment && !inBlockComment {
			if ch == '\'' && (i == 0 || runes[i-1] != '\\') {
				inSingleQuote = !inSingleQuote
			}
			if ch == '"' && (i == 0 || runes[i-1] != '\\') {
				inDoubleQuote = !inDoubleQuote
			}
		}

		// Check for statement terminator (semicolon)
		if ch == ';' && !inSingleQuote && !inDoubleQuote && !inLineComment && !inBlockComment {
			currentStmt.WriteRune(ch)
			statements = append(statements, sqlStatement{
				sql:       currentStmt.String(),
				startLine: stmtStartLine,
			})
			currentStmt.Reset()
			hasSeenNonWhitespace = false
			// Don't set stmtStartLine here - wait until we see actual content
			continue
		}

		// Track the first non-whitespace, non-comment character for line number
		if !hasSeenNonWhitespace && !inLineComment && !inBlockComment {
			if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
				stmtStartLine = currentLine
				hasSeenNonWhitespace = true
			}
		}

		currentStmt.WriteRune(ch)
	}

	// Add final statement if there's content
	if currentStmt.Len() > 0 {
		statements = append(statements, sqlStatement{
			sql:       currentStmt.String(),
			startLine: stmtStartLine,
		})
	}

	return statements
}

// proactiveValidation performs checks for issues that pg_query might not catch
func proactiveValidation(filePath, sqlContent string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for TIMESTAMPZ (should be TIMESTAMP or TIMESTAMPTZ)
	if strings.Contains(sqlContent, "TIMESTAMPZ") {
		// Make sure it's TIMESTAMPZ and not TIMESTAMPTZ
		if regexp.MustCompile(`TIMESTAMPZ[^T]`).MatchString(sqlContent) || strings.HasSuffix(sqlContent, "TIMESTAMPZ") || strings.Contains(sqlContent, "TIMESTAMPZ\n") {
			line, col := findTokenInContent(sqlContent, "TIMESTAMPZ")
			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   col,
				Severity: "error",
				Message: "Unknown data type 'TIMESTAMPZ'\n" +
					"  Did you mean 'TIMESTAMP' or 'TIMESTAMPTZ'?\n" +
					"  TIMESTAMPTZ includes timezone info, TIMESTAMP does not",
				Code: "invalid_data_type",
			})
			return issues
		}
	}

	return issues
}

// semanticValidation checks for semantic issues in syntactically valid SQL
func semanticValidation(filePath, sqlContent string) []ValidationIssue {
	var issues []ValidationIssue

	// Check for duplicate PRIMARY KEY within a single properly-terminated statement
	// Split by "); " to get individual statements
	statements := strings.Split(sqlContent, ");")

	for _, stmt := range statements {
		// Only check statements that are CREATE TABLE and properly terminated
		if !strings.Contains(stmt, "CREATE TABLE") {
			continue
		}

		// Check if this statement has multiple PRIMARY KEY
		primaryKeyPattern := regexp.MustCompile(`(?i)PRIMARY\s+KEY`)
		matches := primaryKeyPattern.FindAllStringIndex(stmt, -1)
		if len(matches) > 1 {
			// Find where this statement starts in the original content
			stmtStartIdx := strings.Index(sqlContent, stmt)
			if stmtStartIdx == -1 {
				continue
			}

			// Find the absolute position of the second PRIMARY KEY
			secondRelativePos := matches[1][0]
			secondAbsolutePos := stmtStartIdx + secondRelativePos

			line, col := findPositionFromOffsetInSQL(sqlContent, secondAbsolutePos)
			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   col,
				Severity: "error",
				Message: "Multiple PRIMARY KEY constraints defined\n" +
					"  A table can only have one PRIMARY KEY\n" +
					"  Use UNIQUE constraint for additional unique columns",
				Code: "duplicate_primary_key",
			})
			return issues
		}
	}

	return issues
}

func findTokenInContent(content, token string) (int, int) {
	idx := strings.Index(content, token)
	if idx == -1 {
		return 1, 1
	}

	line := 1
	col := 1
	for i := 0; i < idx; i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}

func findPositionFromOffsetInSQL(content string, offset int) (int, int) {
	if offset < 0 || offset >= len(content) {
		return 1, 1
	}

	line := 1
	col := 1

	for i := 0; i < offset; i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}
