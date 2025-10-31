package main

import (
	"fmt"
	"regexp"
	"strings"
)

// enhanceSQLError takes a generic pg_query error and enriches it with better context and suggestions
func enhanceSQLError(filePath string, sqlContent string, pgErr error) []ValidationIssue {
	errorMsg := pgErr.Error()
	errorMsg = strings.TrimPrefix(errorMsg, "failed to parse SQL: ")

	// Extract location information if available from pg_query error
	// pg_query errors sometimes include location info
	var stmt string
	var location int

	// Try to find what token caused the error
	nearMatch := regexp.MustCompile(`at or near "([^"]+)"`).FindStringSubmatch(errorMsg)
	if len(nearMatch) > 1 {
		stmt = nearMatch[1]
	}

	// Try different enhancement strategies
	var issue ValidationIssue

	// Strategy 1: Pattern-based error enhancement
	issue = enhanceByPattern(filePath, sqlContent, errorMsg, stmt)
	if issue.Message != "" {
		return []ValidationIssue{issue}
	}

	// Strategy 2: Context-based analysis
	issue = enhanceByContext(filePath, sqlContent, errorMsg, stmt, location)
	if issue.Message != "" {
		return []ValidationIssue{issue}
	}

	// Fallback: Return enhanced version of original error with context
	return []ValidationIssue{
		enhancedGenericError(filePath, sqlContent, errorMsg, stmt),
	}
}

// enhanceByPattern matches specific error patterns and provides targeted help
func enhanceByPattern(filePath, sqlContent, errorMsg, nearToken string) ValidationIssue {
	patterns := []struct {
		match   *regexp.Regexp
		handler func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue
	}{
		{
			// Missing comma between columns
			match: regexp.MustCompile(`syntax error at or near "(\w+)"`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				// Check if this looks like a column definition following another column
				if isLikelyMissingComma(sqlContent, nearToken) {
					line, col := findToken(sqlContent, nearToken)
					prevLine := getLine(sqlContent, line-1)
					return ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   col,
						Severity: "error",
						Message: fmt.Sprintf("Missing comma between column definitions\n"+
							"  Previous line: %s\n"+
							"  Problem at: %s\n"+
							"  Fix: Add a comma after the previous column definition",
							strings.TrimSpace(prevLine), nearToken),
						Code: "syntax_error",
					}
				}
				return ValidationIssue{}
			},
		},
		{
			// Common typos
			match: regexp.MustCompile(`syntax error at or near "([A-Z]+)"`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				suggestion := getSuggestionForTypo(nearToken)
				if suggestion != "" {
					line, col := findToken(sqlContent, nearToken)
					return ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   col,
						Severity: "error",
						Message: fmt.Sprintf("Invalid SQL keyword '%s'\n"+
							"  Did you mean '%s'?\n"+
							"  %s",
							nearToken, suggestion, getCodeContext(sqlContent, line)),
						Code: "syntax_error",
					}
				}
				return ValidationIssue{}
			},
		},
		{
			// MySQL-specific syntax
			match: regexp.MustCompile(`AUTO_INCREMENT|AUTO INCREMENT`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				line, col := findToken(sqlContent, "AUTO_INCREMENT")
				if line == 0 {
					line, col = findToken(sqlContent, "AUTO INCREMENT")
				}
				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: "AUTO_INCREMENT is MySQL syntax, not supported in PostgreSQL\n" +
						"  PostgreSQL alternatives:\n" +
						"    • GENERATED ALWAYS AS IDENTITY (recommended for new tables)\n" +
						"    • SERIAL or BIGSERIAL (traditional approach)\n" +
						"  Example: id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY",
					Code: "mysql_syntax",
				}
			},
		},
		{
			// Backticks
			match: regexp.MustCompile("`"),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				line, col := findBacktick(sqlContent)
				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: "Backticks (`) are MySQL syntax, not supported in PostgreSQL\n" +
						"  For identifiers: Use double quotes \"identifier\"\n" +
						"  For strings: Use single quotes 'string'\n" +
						"  Note: In most cases, you don't need quotes at all",
					Code: "mysql_syntax",
				}
			},
		},
		{
			// NOTNULL typo
			match: regexp.MustCompile(`syntax error at or near "NOTNULL"`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				line, col := findToken(sqlContent, "NOTNULL")
				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: "NOTNULL is not valid\n" +
						"  Did you mean 'NOT NULL' (two words)?\n" +
						"  Example: email TEXT NOT NULL",
					Code: "syntax_error",
				}
			},
		},
	}

	for _, p := range patterns {
		if p.match.MatchString(sqlContent) || p.match.MatchString(errorMsg) {
			matches := p.match.FindStringSubmatch(errorMsg)
			if len(matches) == 0 {
				matches = p.match.FindStringSubmatch(sqlContent)
			}
			issue := p.handler(filePath, sqlContent, errorMsg, nearToken, matches)
			if issue.Message != "" {
				return issue
			}
		}
	}

	return ValidationIssue{}
}

// enhanceByContext analyzes the SQL structure to provide context-aware suggestions
func enhanceByContext(filePath, sqlContent, errorMsg, nearToken string, location int) ValidationIssue {
	// Check for missing semicolons
	if strings.Contains(errorMsg, "syntax error at or near \"CREATE\"") {
		if hasMissingSemicolon(sqlContent) {
			line := findCreateAfterMissingSemicolon(sqlContent)
			return ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: "Missing semicolon after previous statement\n" +
					"  Each SQL statement must end with a semicolon (;)\n" +
					"  Add ';' after the closing parenthesis of the previous CREATE TABLE\n" +
					"  " + getCodeContext(sqlContent, line-1),
				Code: "missing_semicolon",
			}
		}
	}

	// Check for trailing commas
	if strings.Contains(errorMsg, "syntax error at or near \")\"") {
		if hasTrailingComma(sqlContent) {
			line := findTrailingComma(sqlContent)
			return ValidationIssue{
				File:     filePath,
				Line:     line + 1,
				Column:   1,
				Severity: "error",
				Message: "Trailing comma before closing parenthesis\n" +
					"  Remove the comma after the last column definition\n" +
					"  " + getCodeContext(sqlContent, line),
				Code: "trailing_comma",
			}
		}
	}

	// Check for missing opening parenthesis
	if strings.Contains(sqlContent, "CREATE TABLE") && !strings.Contains(sqlContent, "CREATE TABLE ") ||
		(strings.Contains(errorMsg, "syntax error") && isAfterTableName(sqlContent, nearToken)) {
		line := findTableDefinitionStart(sqlContent, nearToken)
		if line > 0 {
			tableName := extractTableName(getLine(sqlContent, line))
			return ValidationIssue{
				File:     filePath,
				Line:     line + 1,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("Missing opening parenthesis after table name\n"+
					"  Expected: CREATE TABLE %s (\n"+
					"  Add '(' after the table name", tableName),
				Code: "missing_paren",
			}
		}
	}

	return ValidationIssue{}
}

// enhancedGenericError creates a better version of generic errors with context
func enhancedGenericError(filePath, sqlContent, errorMsg, nearToken string) ValidationIssue {
	line := 1
	col := 1

	if nearToken != "" {
		line, col = findToken(sqlContent, nearToken)
	}

	// Add code context
	context := getCodeContext(sqlContent, line)

	enhancedMsg := errorMsg
	if context != "" {
		enhancedMsg = fmt.Sprintf("%s\n  %s", errorMsg, context)
	}

	return ValidationIssue{
		File:     filePath,
		Line:     line,
		Column:   col,
		Severity: "error",
		Message:  enhancedMsg,
		Code:     "syntax_error",
	}
}

// Helper functions

func findToken(content, token string) (line, col int) {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if idx := strings.Index(l, token); idx >= 0 {
			return i + 1, idx + 1
		}
	}
	return 1, 1
}

func findBacktick(content string) (line, col int) {
	lines := strings.Split(content, "\n")
	for i, l := range lines {
		if idx := strings.Index(l, "`"); idx >= 0 {
			return i + 1, idx + 1
		}
	}
	return 1, 1
}

func getLine(content string, lineNum int) string {
	lines := strings.Split(content, "\n")
	if lineNum > 0 && lineNum <= len(lines) {
		return lines[lineNum-1]
	}
	return ""
}

func getCodeContext(content string, lineNum int) string {
	lines := strings.Split(content, "\n")
	if lineNum < 1 || lineNum > len(lines) {
		return ""
	}

	// Show the problematic line and one line before/after
	start := max(0, lineNum-2)
	end := min(len(lines), lineNum+1)

	var context strings.Builder
	context.WriteString("Code context:\n")
	for i := start; i < end; i++ {
		marker := "  "
		if i == lineNum-1 {
			marker = "→ " // Arrow pointing to problem line
		}
		context.WriteString(fmt.Sprintf("  %s%3d: %s\n", marker, i+1, lines[i]))
	}

	return context.String()
}

func isLikelyMissingComma(content, nearToken string) bool {
	// Check if the token appears to be a column name after another column definition
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, nearToken) && i > 0 {
			prevLine := strings.TrimSpace(lines[i-1])
			// Previous line looks like a column definition without a comma
			if regexp.MustCompile(`^\s*\w+\s+\w+`).MatchString(prevLine) &&
				!strings.HasSuffix(prevLine, ",") &&
				!strings.HasPrefix(prevLine, "--") {
				return true
			}
		}
	}
	return false
}

func getSuggestionForTypo(word string) string {
	typos := map[string]string{
		"TABEL":       "TABLE",
		"TALBE":       "TABLE",
		"PRIMAY":      "PRIMARY",
		"PRIMERY":     "PRIMARY",
		"FORIEGN":     "FOREIGN",
		"FOREGIN":     "FOREIGN",
		"REFERNCES":   "REFERENCES",
		"TIMESTAMPZ":  "TIMESTAMPTZ or TIMESTAMP",
		"NOTNULL":     "NOT NULL",
		"INTEGR":      "INTEGER",
		"DEFALT":      "DEFAULT",
		"UNQUE":       "UNIQUE",
		"UNIUQE":      "UNIQUE",
	}

	return typos[word]
}

func hasMissingSemicolon(content string) bool {
	// Look for pattern: ) followed by CREATE without semicolon
	pattern := regexp.MustCompile(`\)\s*\n\s*CREATE`)
	return pattern.MatchString(content)
}

func findCreateAfterMissingSemicolon(content string) int {
	pattern := regexp.MustCompile(`\)\s*\n\s*CREATE`)
	if loc := pattern.FindStringIndex(content); loc != nil {
		// Count newlines up to this point
		return strings.Count(content[:loc[1]], "\n")
	}
	return 1
}

func hasTrailingComma(content string) bool {
	pattern := regexp.MustCompile(`,\s*\n\s*\)`)
	return pattern.MatchString(content)
}

func findTrailingComma(content string) int {
	pattern := regexp.MustCompile(`,\s*\n\s*\)`)
	if loc := pattern.FindStringIndex(content); loc != nil {
		return strings.Count(content[:loc[0]], "\n") + 1
	}
	return 1
}

func isAfterTableName(content, token string) bool {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, token) {
			if i > 0 && strings.Contains(lines[i-1], "CREATE TABLE") {
				return true
			}
		}
	}
	return false
}

func findTableDefinitionStart(content, nearToken string) int {
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if strings.Contains(line, "CREATE TABLE") {
			return i + 1
		}
	}
	return 0
}

func extractTableName(line string) string {
	pattern := regexp.MustCompile(`CREATE TABLE\s+(\w+)`)
	matches := pattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return "table_name"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
