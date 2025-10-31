package diagnostic

import (
	"fmt"
	"strings"
)

// LegacyValidationIssue is the old format - we'll convert from Diagnostic
// This maintains compatibility while we migrate
type LegacyValidationIssue struct {
	File     string
	Line     int // 1-indexed for backward compatibility
	Column   int // 1-indexed for backward compatibility
	Severity string
	Message  string
	Code     string
}

// ToLegacyIssue converts a Diagnostic to the old ValidationIssue format
func ToLegacyIssue(d Diagnostic, filePath string) LegacyValidationIssue {
	return LegacyValidationIssue{
		File:     filePath,
		Line:     d.Range.Start.Line + 1,      // Convert to 1-indexed
		Column:   d.Range.Start.Character + 1, // Convert to 1-indexed
		Severity: d.Severity.String(),
		Message:  d.Message,
		Code:     d.Code,
	}
}

// ToLegacyIssues converts multiple diagnostics to legacy format
func ToLegacyIssues(diagnostics []Diagnostic, filePath string) []LegacyValidationIssue {
	issues := make([]LegacyValidationIssue, len(diagnostics))
	for i, d := range diagnostics {
		issues[i] = ToLegacyIssue(d, filePath)
	}
	return issues
}

// FromLegacyIssue converts old ValidationIssue to new Diagnostic format
func FromLegacyIssue(issue LegacyValidationIssue, content string) Diagnostic {
	severity := SeverityError
	switch strings.ToLower(issue.Severity) {
	case "warning":
		severity = SeverityWarning
	case "info":
		severity = SeverityInfo
	case "hint":
		severity = SeverityHint
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

	// Calculate offset from line and column
	lines := strings.Split(content, "\n")
	offset := 0
	for i := 0; i < line && i < len(lines); i++ {
		offset += len(lines[i]) + 1 // +1 for newline
	}
	offset += col

	pos := Position{
		Line:      line,
		Character: col,
		Offset:    offset,
	}

	// Create a range for the issue (single character if we don't know better)
	r := Range{
		Start: pos,
		End:   Position{Line: line, Character: col + 1, Offset: offset + 1},
	}

	return NewDiagnostic(r, severity, issue.Code, issue.Message)
}

// DiagnosticFormatter formats diagnostics for display
type DiagnosticFormatter struct {
	ShowSource      bool
	ShowCodeContext bool
	ColorEnabled    bool
}

// NewFormatter creates a new diagnostic formatter
func NewFormatter() *DiagnosticFormatter {
	return &DiagnosticFormatter{
		ShowSource:      true,
		ShowCodeContext: true,
		ColorEnabled:    false,
	}
}

// Format formats a diagnostic for display
func (f *DiagnosticFormatter) Format(d Diagnostic, source, content string) string {
	var b strings.Builder

	// File:Line:Col: severity: message
	if f.ShowSource {
		b.WriteString(fmt.Sprintf("%s:%d:%d: ", source, d.Range.Start.Line+1, d.Range.Start.Character+1))
	}

	// Severity
	severity := strings.ToUpper(d.Severity.String())
	if f.ColorEnabled {
		severity = f.colorize(severity, d.Severity)
	}
	b.WriteString(fmt.Sprintf("%s: ", severity))

	// Message
	b.WriteString(d.Message)

	// Code
	if d.Code != "" {
		b.WriteString(fmt.Sprintf(" [%s]", d.Code))
	}

	// Code context
	if f.ShowCodeContext && content != "" {
		context := f.formatCodeContext(content, d.Range)
		if context != "" {
			b.WriteString("\n")
			b.WriteString(context)
		}
	}

	// Related information
	if len(d.Related) > 0 {
		b.WriteString("\n  Related:")
		for _, rel := range d.Related {
			b.WriteString(fmt.Sprintf("\n    %s:%d:%d: %s",
				rel.Location.URI,
				rel.Location.Range.Start.Line+1,
				rel.Location.Range.Start.Character+1,
				rel.Message))
		}
	}

	return b.String()
}

// formatCodeContext formats code context around a range
func (f *DiagnosticFormatter) formatCodeContext(content string, r Range) string {
	lines := strings.Split(content, "\n")
	if r.Start.Line < 0 || r.Start.Line >= len(lines) {
		return ""
	}

	var b strings.Builder

	// Show line before, problem line, and line after
	start := max(0, r.Start.Line-1)
	end := min(len(lines), r.Start.Line+2)

	for i := start; i < end; i++ {
		lineNum := i + 1
		marker := "  "
		if i == r.Start.Line {
			marker = "→ " // Arrow for problem line
		}

		b.WriteString(fmt.Sprintf("  %s%3d: %s\n", marker, lineNum, lines[i]))

		// Add squiggly underline for the problem range
		if i == r.Start.Line {
			// Calculate where to put the squigglies
			indent := "       " // Match the line number prefix
			spaces := strings.Repeat(" ", r.Start.Character)
			length := r.End.Character - r.Start.Character
			if length < 1 {
				length = 1
			}
			if length > 80 {
				length = 80 // Cap length for readability
			}
			squigglies := strings.Repeat("~", length)

			b.WriteString(fmt.Sprintf("%s%s%s\n", indent, spaces, squigglies))
		}
	}

	return b.String()
}

// colorize adds ANSI color codes to severity text
func (f *DiagnosticFormatter) colorize(text string, severity Severity) string {
	const (
		red    = "\033[31m"
		yellow = "\033[33m"
		blue   = "\033[34m"
		gray   = "\033[90m"
		reset  = "\033[0m"
	)

	var color string
	switch severity {
	case SeverityError:
		color = red
	case SeverityWarning:
		color = yellow
	case SeverityInfo:
		color = blue
	case SeverityHint:
		color = gray
	}

	return color + text + reset
}
