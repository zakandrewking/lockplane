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
			// Missing comma between columns (but not for special keywords)
			match: regexp.MustCompile(`syntax error at or near "(\w+)"`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				// Skip if this is a special case that should be handled differently
				specialTokens := []string{"id", "NOT", "CREATE", "AUTO_INCREMENT"}
				for _, special := range specialTokens {
					if nearToken == special || strings.Contains(sqlContent, "CREATE TABLE") && strings.Contains(getLine(sqlContent, 1), nearToken) {
						// Don't treat line 1 tokens or special tokens as missing comma
						return ValidationIssue{}
					}
				}

				// Check if this looks like a column definition following another column
				if isLikelyMissingComma(sqlContent, nearToken) {
					line, col := findToken(sqlContent, nearToken)
					prevLine := strings.TrimSpace(getLine(sqlContent, line-1))
					currentLine := strings.TrimSpace(getLine(sqlContent, line))
					return ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   col,
						Severity: "error",
						Message: fmt.Sprintf("You're missing comma between column definitions\n"+
							"  Previous line: %s\n"+
							"  Current line: %s\n"+
							"  Add a comma after '%s'",
							prevLine, currentLine, prevLine),
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
					lineContent := getLine(sqlContent, line)
					fullSuggestion := suggestion

					// Check if this is part of a CREATE statement
					if strings.Contains(lineContent, "CREATE") && nearToken == "TABEL" {
						fullSuggestion = "CREATE TABLE"
					}

					return ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   col,
						Severity: "error",
						Message: fmt.Sprintf("Invalid SQL keyword '%s'\n"+
							"  Did you mean '%s'?\n"+
							"  %s",
							nearToken, fullSuggestion, getCodeContext(sqlContent, line)),
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
						"  Use 'GENERATED ALWAYS AS IDENTITY' or 'SERIAL' instead\n" +
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
				// Extract the identifier within backticks for a more helpful message
				lineContent := getLine(sqlContent, line)
				backtickPattern := regexp.MustCompile("`([^`]+)`")
				match := backtickPattern.FindStringSubmatch(lineContent)
				identifier := "identifier"
				if len(match) > 1 {
					identifier = match[1]
				}

				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: fmt.Sprintf("backticks are MySQL syntax, not supported in PostgreSQL\n"+
						"  Use double quotes \"%s\" for identifiers in PostgreSQL", identifier),
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
		{
			// TIMESTAMPZ - invalid data type
			match: regexp.MustCompile(`TIMESTAMPZ`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				line, col := findToken(sqlContent, "TIMESTAMPZ")
				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: "Unknown data type 'TIMESTAMPZ'\n" +
						"  Did you mean 'TIMESTAMP' or 'TIMESTAMPTZ'?\n" +
						"  TIMESTAMPTZ includes timezone info, TIMESTAMP does not",
					Code: "invalid_data_type",
				}
			},
		},
		{
			// Incomplete REFERENCES (foreign key)
			match: regexp.MustCompile(`REFERENCES\s*\)` + "|" + `syntax error at or near "\)"`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				// Check if there's an incomplete REFERENCES
				if strings.Contains(sqlContent, "REFERENCES") && strings.Contains(sqlContent, "REFERENCES\n") {
					line, col := findToken(sqlContent, "REFERENCES")
					return ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   col + len("REFERENCES"),
						Severity: "error",
						Message: "incomplete FOREIGN KEY definition\n" +
							"  Syntax: REFERENCES table_name(column_name)\n" +
							"  Example: user_id BIGINT REFERENCES users(id)",
						Code: "incomplete_foreign_key",
					}
				}
				return ValidationIssue{}
			},
		},
		{
			// Missing DEFAULT keyword
			match: regexp.MustCompile(`TIMESTAMP\s+NOW\(\)`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				line, col := findToken(sqlContent, "NOW")
				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: "missing DEFAULT keyword before NOW()\n" +
						"  Correct syntax: created_at TIMESTAMP DEFAULT NOW()\n" +
						"  The DEFAULT keyword is required for default values",
					Code: "missing_default",
				}
			},
		},
		{
			// Incomplete CREATE INDEX
			match: regexp.MustCompile(`CREATE INDEX.*ON\s*$`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				// Return line 1 - the adjustment logic in validate_sql.go will add stmt.startLine - 1
				return ValidationIssue{
					File:     filePath,
					Line:     1,
					Column:   len(strings.TrimSpace(sqlContent)) + 1,
					Severity: "error",
					Message: "incomplete CREATE INDEX statement\n" +
						"  Expected: CREATE INDEX index_name ON table_name(column_name)\n" +
						"  Example: CREATE INDEX users_email_idx ON users(email)",
					Code: "incomplete_index",
				}
			},
		},
		{
			// Missing column name - detect when a type appears where a column name should be
			match: regexp.MustCompile(`syntax error at or near "TEXT|NOT"`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				// Check if TEXT appears right after a comma or opening paren (missing column name)
				if regexp.MustCompile(`[,(]\s*TEXT`).MatchString(sqlContent) {
					line, col := findToken(sqlContent, "TEXT")
					return ValidationIssue{
						File:     filePath,
						Line:     line,
						Column:   col,
						Severity: "error",
						Message: "missing column name before data type\n" +
							"  Expected: column_name data_type\n" +
							"  Example: email TEXT NOT NULL",
						Code: "missing_column_name",
					}
				}
				return ValidationIssue{}
			},
		},
		{
			// Duplicate PRIMARY KEY
			match: regexp.MustCompile(`(?i)PRIMARY\s+KEY.*PRIMARY\s+KEY`),
			handler: func(filePath, sqlContent, errorMsg, nearToken string, matches []string) ValidationIssue {
				// Find the second PRIMARY KEY
				firstPos := regexp.MustCompile(`(?i)PRIMARY\s+KEY`).FindStringIndex(sqlContent)
				if firstPos != nil {
					remaining := sqlContent[firstPos[1]:]
					secondMatch := regexp.MustCompile(`(?i)PRIMARY\s+KEY`).FindStringIndex(remaining)
					if secondMatch != nil {
						line, col := findPositionFromOffset(sqlContent, firstPos[1]+secondMatch[0])
						return ValidationIssue{
							File:     filePath,
							Line:     line,
							Column:   col,
							Severity: "error",
							Message: "Multiple PRIMARY KEY constraints defined\n" +
								"  A table can only have one PRIMARY KEY\n" +
								"  Use UNIQUE constraint for additional unique columns",
							Code: "duplicate_primary_key",
						}
					}
				}
				return ValidationIssue{}
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
			// The line returned is where CREATE appears, and we want to report
			// the error there (where the unexpected CREATE is)
			return ValidationIssue{
				File:     filePath,
				Line:     line,
				Column:   1,
				Severity: "error",
				Message: "missing semicolon after previous statement\n" +
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
			lineContent := strings.TrimSpace(strings.TrimSuffix(getLine(sqlContent, line), ","))
			return ValidationIssue{
				File:     filePath,
				Line:     line + 1,
				Column:   1,
				Severity: "error",
				Message: fmt.Sprintf("trailing comma before closing parenthesis\n"+
					"  Remove the comma after '%s'\n"+
					"  %s", lineContent, getCodeContext(sqlContent, line)),
				Code: "trailing_comma",
			}
		}
	}

	// Check for missing closing parenthesis
	if strings.Contains(errorMsg, "syntax error at or near \";\"") ||
		strings.Contains(errorMsg, "syntax error at or near \"CREATE\"") {
		// Count open and close parens in CREATE TABLE statements
		if strings.Contains(sqlContent, "CREATE TABLE") {
			openCount := strings.Count(sqlContent, "(")
			closeCount := strings.Count(sqlContent, ")")
			if openCount > closeCount {
				// Find the line where we have unclosed parentheses
				// by tracking balance as we go through the content
				problemLine := findUnclosedParenLine(sqlContent)
				if problemLine > 0 {
					return ValidationIssue{
						File:     filePath,
						Line:     problemLine,
						Column:   1,
						Severity: "error",
						Message: "You're missing closing parenthesis in CREATE TABLE statement\n" +
							"  Expected ')' before the semicolon\n" +
							"  Check DEFAULT clauses and nested expressions\n" +
							"  " + getCodeContext(sqlContent, problemLine),
						Code: "missing_paren",
					}
				}

				// Fallback to old behavior
				line, col := findToken(sqlContent, ";")
				return ValidationIssue{
					File:     filePath,
					Line:     line,
					Column:   col,
					Severity: "error",
					Message: "missing closing parenthesis\n" +
						"  Expected ')' before the semicolon\n" +
						"  Check that all column definitions are properly closed",
					Code: "missing_paren",
				}
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
				Message: fmt.Sprintf("missing opening parenthesis after table name\n"+
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
		"TABEL":      "TABLE",
		"TALBE":      "TABLE",
		"PRIMAY":     "PRIMARY",
		"PRIMERY":    "PRIMARY",
		"FORIEGN":    "FOREIGN",
		"FOREGIN":    "FOREIGN",
		"REFERNCES":  "REFERENCES",
		"TIMESTAMPZ": "TIMESTAMPTZ or TIMESTAMP",
		"NOTNULL":    "NOT NULL",
		"INTEGR":     "INTEGER",
		"DEFALT":     "DEFAULT",
		"UNQUE":      "UNIQUE",
		"UNIUQE":     "UNIQUE",
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
		// Count newlines up to this point to find the line with CREATE
		// We want the line number where CREATE appears (1-indexed)
		return strings.Count(content[:loc[1]], "\n") + 1
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

func findPositionFromOffset(content string, offset int) (line int, col int) {
	if offset < 0 || offset > len(content) {
		return 1, 1
	}

	line = 1
	col = 1

	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}

	return line, col
}

// findUnclosedParenLine finds the line where parentheses become unbalanced
// This helps identify the actual line with the problem, not where the parser gave up
func findUnclosedParenLine(content string) int {
	lines := strings.Split(content, "\n")
	balance := 0
	lastLineWithNetPositive := 0
	seenFirstCreate := false

	for i, line := range lines {
		lineNum := i + 1

		// Skip comment lines
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "--") {
			continue
		}

		// Track if we've seen a CREATE TABLE
		hasCreateTable := strings.Contains(line, "CREATE TABLE")
		if hasCreateTable && !seenFirstCreate {
			seenFirstCreate = true
		}

		// Count parentheses in this line
		openCount := 0
		closeCount := 0
		for _, ch := range line {
			switch ch {
			case '(':
				balance++
				openCount++
			case ')':
				balance--
				closeCount++
			}
		}

		// Track the last line that added net positive parens (more open than close)
		// This is likely where the problem is
		netChange := openCount - closeCount
		if netChange > 0 {
			lastLineWithNetPositive = lineNum
		}

		// If we hit a semicolon with unclosed parens, we've found the statement with the problem
		if strings.Contains(line, ";") && balance > 0 {
			// If the only line with net positive is the CREATE TABLE itself, or only 1 unclosed paren,
			// report at the semicolon. Otherwise, report where the extras were added.
			if lastLineWithNetPositive > 0 && balance > 1 {
				// Multiple unclosed parens, likely from a DEFAULT clause or nested expression
				return lastLineWithNetPositive
			}
			// Just missing the closing paren for CREATE TABLE, report at semicolon
			return lineNum
		}

		// If we hit a SECOND CREATE TABLE with open parens, problem is in previous statement
		if hasCreateTable && seenFirstCreate && balance > 0 && lastLineWithNetPositive > 0 {
			// But only if this isn't the first CREATE TABLE we're tracking
			if lastLineWithNetPositive < lineNum {
				return lastLineWithNetPositive
			}
		}
	}

	// Return the last line that added unclosed parens
	return lastLineWithNetPositive
}
