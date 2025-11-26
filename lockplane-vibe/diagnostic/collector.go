package diagnostic

import (
	"sort"
)

// Collector collects diagnostics from various analysis passes
type Collector struct {
	diagnostics []Diagnostic
	source      string // Source file path
	content     string // Source content for position calculations
}

// NewCollector creates a new diagnostic collector
func NewCollector(source, content string) *Collector {
	return &Collector{
		diagnostics: []Diagnostic{},
		source:      source,
		content:     content,
	}
}

// Add adds a diagnostic to the collection
func (c *Collector) Add(diag Diagnostic) {
	c.diagnostics = append(c.diagnostics, diag)
}

// AddError adds an error diagnostic
func (c *Collector) AddError(r Range, code, message string) {
	c.Add(NewDiagnostic(r, SeverityError, code, message))
}

// AddWarning adds a warning diagnostic
func (c *Collector) AddWarning(r Range, code, message string) {
	c.Add(NewDiagnostic(r, SeverityWarning, code, message))
}

// AddInfo adds an info diagnostic
func (c *Collector) AddInfo(r Range, code, message string) {
	c.Add(NewDiagnostic(r, SeverityInfo, code, message))
}

// AddHint adds a hint diagnostic
func (c *Collector) AddHint(r Range, code, message string) {
	c.Add(NewDiagnostic(r, SeverityHint, code, message))
}

// AddErrorAtOffset adds an error at a specific byte offset
func (c *Collector) AddErrorAtOffset(offset int, length int, code, message string) {
	r := RangeFromOffsets(c.content, offset, offset+length)
	c.AddError(r, code, message)
}

// AddWarningAtOffset adds a warning at a specific byte offset
func (c *Collector) AddWarningAtOffset(offset int, length int, code, message string) {
	r := RangeFromOffsets(c.content, offset, offset+length)
	c.AddWarning(r, code, message)
}

// All returns all collected diagnostics, sorted by location
func (c *Collector) All() []Diagnostic {
	// Sort by position
	sort.Slice(c.diagnostics, func(i, j int) bool {
		if c.diagnostics[i].Range.Start.Line != c.diagnostics[j].Range.Start.Line {
			return c.diagnostics[i].Range.Start.Line < c.diagnostics[j].Range.Start.Line
		}
		return c.diagnostics[i].Range.Start.Character < c.diagnostics[j].Range.Start.Character
	})
	return c.diagnostics
}

// Errors returns only error-level diagnostics
func (c *Collector) Errors() []Diagnostic {
	var errors []Diagnostic
	for _, d := range c.diagnostics {
		if d.Severity == SeverityError {
			errors = append(errors, d)
		}
	}
	return errors
}

// Warnings returns only warning-level diagnostics
func (c *Collector) Warnings() []Diagnostic {
	var warnings []Diagnostic
	for _, d := range c.diagnostics {
		if d.Severity == SeverityWarning {
			warnings = append(warnings, d)
		}
	}
	return warnings
}

// HasErrors returns true if there are any errors
func (c *Collector) HasErrors() bool {
	for _, d := range c.diagnostics {
		if d.Severity == SeverityError {
			return true
		}
	}
	return false
}

// Count returns the total number of diagnostics
func (c *Collector) Count() int {
	return len(c.diagnostics)
}

// Clear removes all diagnostics
func (c *Collector) Clear() {
	c.diagnostics = []Diagnostic{}
}

// Merge adds diagnostics from another collector
func (c *Collector) Merge(other *Collector) {
	c.diagnostics = append(c.diagnostics, other.diagnostics...)
}

// Content returns the source content
func (c *Collector) Content() string {
	return c.content
}

// Source returns the source file path
func (c *Collector) Source() string {
	return c.source
}
