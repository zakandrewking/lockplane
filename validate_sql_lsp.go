package main

import (
	"github.com/lockplane/lockplane/diagnostic"
)

// ValidateSQLFile validates a SQL file and returns diagnostics
// This is the new API that other tools (like LSP server) can use
func ValidateSQLFile(filePath string, sqlContent string) *diagnostic.Collector {
	collector := diagnostic.NewCollector(filePath, sqlContent)
	parser := diagnostic.NewErrorRecoveryParser(collector)

	// Parse SQL - errors will be added to collector
	if result, err := parser.Parse(sqlContent); err == nil && result != nil {
		// Successfully parsed - run semantic analysis
		analyzeSemantic(collector, result)
	}

	// Run dangerous pattern analysis
	analyzeDangerousPatterns(collector, sqlContent)

	return collector
}

// analyzeSemantic performs semantic analysis on successfully parsed SQL
func analyzeSemantic(collector *diagnostic.Collector, result interface{}) {
	// TODO: Analyze AST for:
	// - Missing foreign key references
	// - Type mismatches
	// - Duplicate names
	// - Missing primary keys (warning)
	// - Schema structure issues
}

// analyzeDangerousPatterns checks for dangerous SQL patterns
func analyzeDangerousPatterns(collector *diagnostic.Collector, sqlContent string) {
	// This integrates with existing validateDangerousPatterns
	// but uses the diagnostic collector
	issues := validateDangerousPatterns(collector.Source(), sqlContent)

	// Convert legacy issues to diagnostics
	for _, issue := range issues {
		diag := convertIssueToDiagnostic(issue, sqlContent)
		collector.Add(diag)
	}
}

// convertIssueToDiagnostic converts old ValidationIssue to new Diagnostic
func convertIssueToDiagnostic(issue ValidationIssue, content string) diagnostic.Diagnostic {
	severity := diagnostic.SeverityError
	switch issue.Severity {
	case "warning":
		severity = diagnostic.SeverityWarning
	case "info":
		severity = diagnostic.SeverityInfo
	case "hint":
		severity = diagnostic.SeverityHint
	}

	// Convert 1-indexed to 0-indexed
	line := issue.Line - 1
	if line < 0 {
		line = 0
	}
	col := issue.Column - 1
	if col < 0 {
		col = 0
	}

	pos := diagnostic.PositionFromOffset(content, calculateOffset(content, line, col))
	r := diagnostic.Range{
		Start: pos,
		End:   diagnostic.PositionFromOffset(content, pos.Offset+1),
	}

	return diagnostic.NewDiagnostic(r, severity, issue.Code, issue.Message)
}

func calculateOffset(content string, line, col int) int {
	lines := 0
	offset := 0
	for i := 0; i < len(content); i++ {
		if lines == line {
			return offset + col
		}
		if content[i] == '\n' {
			lines++
			offset = i + 1
		}
	}
	return offset + col
}
