// Package diagnostic provides SQL parsing diagnostics for better error messages.
//
// This package enhances pg_query parser errors with contextual information,
// suggestions, and user-friendly error messages for SQL syntax issues.
package diagnostic

import (
	"fmt"
	"regexp"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// ErrorRecoveryParser attempts to parse SQL with error recovery
type ErrorRecoveryParser struct {
	collector *Collector
}

// NewErrorRecoveryParser creates a new error recovery parser
func NewErrorRecoveryParser(collector *Collector) *ErrorRecoveryParser {
	return &ErrorRecoveryParser{
		collector: collector,
	}
}

// Parse attempts to parse SQL, recovering from errors and building partial AST
func (p *ErrorRecoveryParser) Parse(sql string) (*pg_query.ParseResult, error) {
	// Try full parse first
	result, err := pg_query.Parse(sql)
	if err == nil {
		return result, nil
	}

	// Parse failed - extract what we can from the error
	p.analyzeError(sql, err)

	// Try progressive parsing to find how much we can parse
	p.progressiveParse(sql)

	return nil, err
}

// analyzeError extracts information from pg_query parse errors
func (p *ErrorRecoveryParser) analyzeError(sql string, err error) {
	errorMsg := err.Error()
	errorMsg = strings.TrimPrefix(errorMsg, "failed to parse SQL: ")

	// Extract location information from error message
	location := p.extractErrorLocation(sql, errorMsg)

	// Extract the token that caused the error
	token := p.extractErrorToken(errorMsg)

	// Try to provide enhanced diagnostics
	p.enhanceError(sql, errorMsg, token, location)
}

// extractErrorLocation tries to find the error location from the error message
func (p *ErrorRecoveryParser) extractErrorLocation(sql, errorMsg string) *Position {
	// pg_query sometimes includes position in error message
	// Look for patterns like "at line X" or similar

	// Try "syntax error at or near "token""
	if nearMatch := regexp.MustCompile(`at or near "([^"]+)"`).FindStringSubmatch(errorMsg); len(nearMatch) > 1 {
		token := nearMatch[1]
		offset := strings.Index(sql, token)
		if offset >= 0 {
			pos := PositionFromOffset(sql, offset)
			return &pos
		}
	}

	// Try "at end of input"
	if strings.Contains(errorMsg, "at end of input") {
		pos := PositionFromOffset(sql, len(sql))
		return &pos
	}

	return nil
}

// extractErrorToken gets the problematic token from error message
func (p *ErrorRecoveryParser) extractErrorToken(errorMsg string) string {
	if nearMatch := regexp.MustCompile(`at or near "([^"]+)"`).FindStringSubmatch(errorMsg); len(nearMatch) > 1 {
		return nearMatch[1]
	}
	return ""
}

// enhanceError provides enhanced diagnostics based on error analysis
func (p *ErrorRecoveryParser) enhanceError(sql, errorMsg, token string, location *Position) {
	// Use location if we found one, otherwise try to find token
	var pos Position
	if location != nil {
		pos = *location
	} else if token != "" {
		offset := strings.Index(sql, token)
		if offset >= 0 {
			pos = PositionFromOffset(sql, offset)
		}
	}

	// Create range for the token
	tokenLength := len(token)
	if tokenLength == 0 {
		tokenLength = 1
	}
	r := Range{
		Start: pos,
		End:   PositionFromOffset(sql, pos.Offset+tokenLength),
	}

	// Analyze the error and provide enhanced message
	enhancedMsg := p.analyzeErrorPattern(sql, errorMsg, token, pos)
	if enhancedMsg == "" {
		enhancedMsg = errorMsg
	}

	// Add diagnostic
	p.collector.Add(NewDiagnostic(r, SeverityError, "syntax_error", enhancedMsg))
}

// analyzeErrorPattern analyzes error patterns and provides enhanced messages
func (p *ErrorRecoveryParser) analyzeErrorPattern(sql, errorMsg, token string, pos Position) string {
	// Check for common patterns
	analyzers := []func(string, string, string, Position) string{
		p.analyzeMySQLSyntax,
		p.analyzeTypo,
		p.analyzeMissingComma,
		p.analyzeTrailingComma,
		p.analyzeMissingSemicolon,
		p.analyzeMissingParenthesis,
		p.analyzeIncompleteStatement,
	}

	for _, analyzer := range analyzers {
		if msg := analyzer(sql, errorMsg, token, pos); msg != "" {
			return msg
		}
	}

	// Add code context to generic errors
	return p.addCodeContext(sql, errorMsg, pos)
}

// analyzeMySQLSyntax detects MySQL-specific syntax
func (p *ErrorRecoveryParser) analyzeMySQLSyntax(sql, errorMsg, token string, pos Position) string {
	// Backticks
	if strings.Contains(sql, "`") {
		return "Backticks (`) are MySQL syntax, not supported in PostgreSQL\n" +
			"  For identifiers: Use double quotes \"identifier\"\n" +
			"  For strings: Use single quotes 'string'\n" +
			"  Note: In most cases, you don't need quotes at all"
	}

	// AUTO_INCREMENT
	if strings.Contains(strings.ToUpper(sql), "AUTO_INCREMENT") || strings.Contains(strings.ToUpper(sql), "AUTO INCREMENT") {
		return "AUTO_INCREMENT is MySQL syntax, not supported in PostgreSQL\n" +
			"  PostgreSQL alternatives:\n" +
			"    • GENERATED ALWAYS AS IDENTITY (recommended for new tables)\n" +
			"    • SERIAL or BIGSERIAL (traditional approach)\n" +
			"  Example: id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY"
	}

	return ""
}

// analyzeTypo detects common SQL keyword typos
func (p *ErrorRecoveryParser) analyzeTypo(sql, errorMsg, token string, pos Position) string {
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

	if suggestion, found := typos[strings.ToUpper(token)]; found {
		return fmt.Sprintf("Invalid SQL keyword '%s'\n"+
			"  Did you mean '%s'?\n"+
			"  %s",
			token, suggestion, p.getCodeContext(sql, pos.Line))
	}

	return ""
}

// analyzeMissingComma detects missing commas between column definitions
func (p *ErrorRecoveryParser) analyzeMissingComma(sql, errorMsg, token string, pos Position) string {
	// Check if this looks like a column name after another column definition
	lines := strings.Split(sql, "\n")
	if pos.Line > 0 && pos.Line < len(lines) {
		prevLine := strings.TrimSpace(lines[pos.Line-1])
		currentLine := strings.TrimSpace(lines[pos.Line])

		// Previous line looks like a column definition without a comma
		if regexp.MustCompile(`^\w+\s+\w+`).MatchString(prevLine) &&
			!strings.HasSuffix(prevLine, ",") &&
			!strings.HasPrefix(prevLine, "--") &&
			regexp.MustCompile(`^\w+`).MatchString(currentLine) {
			return fmt.Sprintf("Missing comma between column definitions\n"+
				"  Previous line: %s\n"+
				"  Current line: %s\n"+
				"  Fix: Add a comma after the previous column definition",
				prevLine, currentLine)
		}
	}

	return ""
}

// analyzeTrailingComma detects trailing commas before closing parenthesis
func (p *ErrorRecoveryParser) analyzeTrailingComma(sql, errorMsg, token string, pos Position) string {
	if token == ")" && strings.Contains(errorMsg, "syntax error") {
		// Check if there's a comma before this position
		if pos.Offset > 0 {
			// Look backwards for comma
			for i := pos.Offset - 1; i >= 0; i-- {
				ch := sql[i]
				if ch == ',' {
					return "Trailing comma before closing parenthesis\n" +
						"  Remove the comma after the last column definition"
				}
				if ch != ' ' && ch != '\t' && ch != '\n' && ch != '\r' {
					break
				}
			}
		}
	}

	return ""
}

// analyzeMissingSemicolon detects missing semicolons between statements
func (p *ErrorRecoveryParser) analyzeMissingSemicolon(sql, errorMsg, token string, pos Position) string {
	if token == "CREATE" && strings.Contains(errorMsg, "syntax error") {
		// Look for pattern: ) followed by CREATE without semicolon
		pattern := regexp.MustCompile(`\)\s*\n\s*CREATE`)
		if pattern.MatchString(sql) {
			return "Missing semicolon after previous statement\n" +
				"  Each SQL statement must end with a semicolon (;)\n" +
				"  Add ';' after the closing parenthesis of the previous statement"
		}
	}

	return ""
}

// analyzeMissingParenthesis detects missing opening or closing parentheses
func (p *ErrorRecoveryParser) analyzeMissingParenthesis(sql, errorMsg, token string, pos Position) string {
	lines := strings.Split(sql, "\n")

	// Missing opening parenthesis after CREATE TABLE
	if pos.Line > 0 && pos.Line < len(lines) {
		prevLine := lines[pos.Line-1]
		if strings.Contains(prevLine, "CREATE TABLE") && !strings.Contains(prevLine, "(") {
			tableName := extractTableName(prevLine)
			return fmt.Sprintf("Missing opening parenthesis after table name\n"+
				"  Expected: CREATE TABLE %s (\n"+
				"  Add '(' after the table name", tableName)
		}
	}

	// Missing closing parenthesis
	if token == ";" {
		openCount := strings.Count(sql[:pos.Offset], "(")
		closeCount := strings.Count(sql[:pos.Offset], ")")
		if openCount > closeCount {
			return fmt.Sprintf("Missing closing parenthesis\n"+
				"  Expected ')' before ';'\n"+
				"  Found %d opening and %d closing parentheses", openCount, closeCount)
		}
	}

	return ""
}

// analyzeIncompleteStatement detects incomplete statements
func (p *ErrorRecoveryParser) analyzeIncompleteStatement(sql, errorMsg, token string, pos Position) string {
	if strings.Contains(errorMsg, "at end of input") {
		// Check what kind of statement is incomplete
		sqlUpper := strings.ToUpper(sql)

		if strings.Contains(sqlUpper, "CREATE INDEX") && !strings.Contains(sqlUpper, "ON") {
			return "Incomplete CREATE INDEX statement\n" +
				"  Expected: CREATE INDEX index_name ON table_name(column_name)\n" +
				"  Add the table and column specifications"
		}

		if strings.Contains(sqlUpper, "REFERENCES") && pos.Offset > 0 {
			return "Incomplete FOREIGN KEY constraint\n" +
				"  Expected: REFERENCES table_name(column_name)\n" +
				"  Add the referenced table and column"
		}

		if strings.Contains(sqlUpper, "CREATE TABLE") {
			openCount := strings.Count(sql, "(")
			closeCount := strings.Count(sql, ")")
			if openCount > closeCount {
				return "Incomplete CREATE TABLE statement\n" +
					"  Missing closing parenthesis ')'\n" +
					"  Add ')' after the last column definition"
			}
		}
	}

	return ""
}

// progressiveParse tries to parse progressively smaller chunks to find valid SQL
func (p *ErrorRecoveryParser) progressiveParse(sql string) {
	// Split into statements and try to parse each
	statements := splitStatements(sql)

	for _, stmt := range statements {
		// Try to parse each statement
		_, _ = pg_query.Parse(stmt.sql)
		// Future: build partial AST from valid statements
	}
}

// Helper functions

func (p *ErrorRecoveryParser) getCodeContext(sql string, line int) string {
	lines := strings.Split(sql, "\n")
	if line < 0 || line >= len(lines) {
		return ""
	}

	start := max(0, line-1)
	end := min(len(lines), line+2)

	var context strings.Builder
	context.WriteString("Code context:\n")
	for i := start; i < end; i++ {
		marker := "  "
		if i == line {
			marker = "→ "
		}
		context.WriteString(fmt.Sprintf("  %s%3d: %s\n", marker, i+1, lines[i]))
	}

	return context.String()
}

func (p *ErrorRecoveryParser) addCodeContext(sql, errorMsg string, pos Position) string {
	context := p.getCodeContext(sql, pos.Line)
	if context != "" {
		return fmt.Sprintf("%s\n  %s", errorMsg, context)
	}
	return errorMsg
}

type statement struct {
	sql       string
	startLine int
	startPos  int
}

func splitStatements(sql string) []statement {
	// Simple semicolon-based splitting for now
	// (Future: more sophisticated splitting that handles strings/comments)
	var statements []statement
	parts := strings.Split(sql, ";")

	offset := 0
	for i, part := range parts {
		if strings.TrimSpace(part) == "" {
			offset += len(part) + 1
			continue
		}

		statements = append(statements, statement{
			sql:       part,
			startLine: strings.Count(sql[:offset], "\n"),
			startPos:  offset,
		})

		if i < len(parts)-1 {
			offset += len(part) + 1 // +1 for semicolon
		}
	}

	return statements
}

func extractTableName(line string) string {
	pattern := regexp.MustCompile(`CREATE TABLE\s+(\w+)`)
	matches := pattern.FindStringSubmatch(line)
	if len(matches) > 1 {
		return matches[1]
	}
	return "table_name"
}
