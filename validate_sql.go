package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
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

func runValidateSQL(args []string) {
	fs := flag.NewFlagSet("validate sql", flag.ExitOnError)
	formatFlag := fs.String("format", "text", "Output format: text or json")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane validate sql [options] <file-or-directory>\n\n")
		fmt.Fprintf(os.Stderr, "Validate a SQL schema file or directory of .lp.sql files.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Validate SQL schema file (text output)\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql schema.lp.sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate with JSON output (for IDE integration)\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql --format json schema.lp.sql\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate directory of SQL files\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate sql --format json lockplane/schema/\n\n")
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	// Read the SQL file to do statement-by-statement validation
	sqlContent, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("Failed to read SQL file: %v", err)
	}

	// First, do statement-by-statement syntax validation
	syntaxIssues := validateSQLSyntax(path, string(sqlContent))

	// If there are syntax errors, report them and exit
	if len(syntaxIssues) > 0 {
		if *formatFlag == "json" {
			result := SQLValidationResult{
				Valid:  false,
				Issues: syntaxIssues,
			}
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonBytes))
		} else {
			fmt.Fprintf(os.Stderr, "✗ SQL syntax errors in %s:\n\n", path)
			for _, issue := range syntaxIssues {
				fmt.Fprintf(os.Stderr, "  Line %d: %s\n", issue.Line, issue.Message)
			}
			fmt.Fprintf(os.Stderr, "\nFound %d syntax error(s). Please fix these before running validation.\n", len(syntaxIssues))
		}
		os.Exit(1)
	}

	// Check for dangerous patterns (data loss operations, etc.)
	dangerousIssues := validateDangerousPatterns(path, string(sqlContent))

	// If syntax is valid, try to load and validate schema structure
	// Note: Loading may fail for files with only DROP/DELETE statements
	var structureIssues []ValidationIssue
	schema, err := LoadSchema(path)
	if err == nil {
		// Validate the schema structure (referential integrity, etc.)
		structureIssues = validateSchemaStructure(schema, path)
	}
	// If schema loading failed but we have dangerous issues, that's OK
	// We'll report the dangerous issues. If no dangerous issues and schema
	// loading failed, that's a real error.
	if err != nil && len(dangerousIssues) == 0 {
		log.Fatalf("Failed to load schema: %v", err)
	}

	// Combine all issues (dangerous patterns + structure issues)
	issues := append(dangerousIssues, structureIssues...)

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

func validateSchemaStructure(schema *Schema, path string) []ValidationIssue {
	var issues []ValidationIssue

	if schema == nil {
		issues = append(issues, ValidationIssue{
			File:     path,
			Line:     1,
			Column:   1,
			Severity: "error",
			Message:  "Schema is empty",
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

	// Try to parse the entire SQL first - if it succeeds, we're done
	if _, err := pg_query.Parse(sqlContent); err == nil {
		return issues // No syntax errors
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
			// Extract error message
			errorMsg := err.Error()
			// Remove "failed to parse SQL: " prefix if present
			errorMsg = strings.TrimPrefix(errorMsg, "failed to parse SQL: ")

			issues = append(issues, ValidationIssue{
				File:     filePath,
				Line:     stmt.startLine,
				Column:   1,
				Severity: "error",
				Message:  errorMsg,
				Code:     "syntax_error",
			})
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
			stmtStartLine = currentLine
			continue
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

// truncateString truncates a string to maxLen characters, adding "..." if truncated
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
