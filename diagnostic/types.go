package diagnostic

import "fmt"

// Severity indicates how serious a diagnostic is
type Severity int

const (
	SeverityError Severity = iota
	SeverityWarning
	SeverityInfo
	SeverityHint
)

func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityInfo:
		return "info"
	case SeverityHint:
		return "hint"
	default:
		return "unknown"
	}
}

// Position represents a position in a text document (0-indexed for compatibility with LSP)
type Position struct {
	Line      int // 0-indexed line number
	Character int // 0-indexed character offset in line
	Offset    int // Byte offset from start of document
}

// Range represents a text range in a document
type Range struct {
	Start Position
	End   Position
}

// Location represents a location in a source file
type Location struct {
	URI   string // File path or URI
	Range Range
}

// DiagnosticRelatedInformation represents related information for a diagnostic
type DiagnosticRelatedInformation struct {
	Location Location
	Message  string
}

// CodeAction represents a suggested fix or action
type CodeAction struct {
	Title       string
	Kind        string // e.g., "quickfix", "refactor"
	Diagnostics []Diagnostic
	Edit        *WorkspaceEdit // The edit to apply
}

// WorkspaceEdit represents changes to apply to the workspace
type WorkspaceEdit struct {
	Changes map[string][]TextEdit // URI -> edits
}

// TextEdit represents a change to a text document
type TextEdit struct {
	Range   Range
	NewText string
}

// Diagnostic represents a diagnostic message (error, warning, etc.)
type Diagnostic struct {
	Range    Range
	Severity Severity
	Code     string // Diagnostic code (e.g., "syntax_error", "dangerous_drop_table")
	Source   string // Source of diagnostic (e.g., "lockplane")
	Message  string
	Related  []DiagnosticRelatedInformation
	Tags     []DiagnosticTag
	Data     interface{} // Additional data for code actions
}

// DiagnosticTag provides additional metadata about a diagnostic
type DiagnosticTag int

const (
	DiagnosticTagUnnecessary DiagnosticTag = iota
	DiagnosticTagDeprecated
)

// NewDiagnostic creates a new diagnostic with sensible defaults
func NewDiagnostic(r Range, severity Severity, code, message string) Diagnostic {
	return Diagnostic{
		Range:    r,
		Severity: severity,
		Code:     code,
		Source:   "lockplane",
		Message:  message,
		Related:  []DiagnosticRelatedInformation{},
		Tags:     []DiagnosticTag{},
	}
}

// WithRelated adds related information to a diagnostic
func (d Diagnostic) WithRelated(location Location, message string) Diagnostic {
	d.Related = append(d.Related, DiagnosticRelatedInformation{
		Location: location,
		Message:  message,
	})
	return d
}

// WithTag adds a tag to a diagnostic
func (d Diagnostic) WithTag(tag DiagnosticTag) Diagnostic {
	d.Tags = append(d.Tags, tag)
	return d
}

// FormatMessage formats a diagnostic message for display
func (d Diagnostic) FormatMessage(includeLocation bool) string {
	var msg string
	if includeLocation {
		msg = fmt.Sprintf("%d:%d: %s: %s",
			d.Range.Start.Line+1, // Convert to 1-indexed
			d.Range.Start.Character+1,
			d.Severity,
			d.Message)
	} else {
		msg = fmt.Sprintf("%s: %s", d.Severity, d.Message)
	}

	if len(d.Related) > 0 {
		msg += "\n  Related:"
		for _, rel := range d.Related {
			msg += fmt.Sprintf("\n    %s:%d:%d: %s",
				rel.Location.URI,
				rel.Location.Range.Start.Line+1,
				rel.Location.Range.Start.Character+1,
				rel.Message)
		}
	}

	return msg
}

// PositionFromOffset converts a byte offset to a Position
func PositionFromOffset(content string, offset int) Position {
	if offset < 0 {
		offset = 0
	}
	if offset > len(content) {
		offset = len(content)
	}

	line := 0
	lineStart := 0

	for i := 0; i < offset && i < len(content); i++ {
		if content[i] == '\n' {
			line++
			lineStart = i + 1
		}
	}

	character := offset - lineStart

	return Position{
		Line:      line,
		Character: character,
		Offset:    offset,
	}
}

// OffsetFromPosition converts a Position to a byte offset
func OffsetFromPosition(content string, pos Position) int {
	lines := 0
	offset := 0

	for i := 0; i < len(content); i++ {
		if lines == pos.Line && offset-getLineStart(content, pos.Line) == pos.Character {
			return i
		}
		if content[i] == '\n' {
			lines++
		}
		offset++
	}

	return offset
}

func getLineStart(content string, line int) int {
	lines := 0
	for i := 0; i < len(content); i++ {
		if lines == line {
			return i
		}
		if content[i] == '\n' {
			lines++
		}
	}
	return len(content)
}

// RangeFromOffsets creates a Range from start and end byte offsets
func RangeFromOffsets(content string, start, end int) Range {
	return Range{
		Start: PositionFromOffset(content, start),
		End:   PositionFromOffset(content, end),
	}
}
