package diagnostic

import (
	"strings"
	"testing"
)

func TestPositionFromOffset(t *testing.T) {
	content := "line 1\nline 2\nline 3"

	tests := []struct {
		offset   int
		wantLine int
		wantChar int
	}{
		{0, 0, 0},                    // Start of file
		{5, 0, 5},                    // End of first line
		{7, 1, 0},                    // Start of second line
		{len(content), 2, len("line 3")}, // End of file
	}

	for _, tt := range tests {
		pos := PositionFromOffset(content, tt.offset)
		if pos.Line != tt.wantLine || pos.Character != tt.wantChar {
			t.Errorf("PositionFromOffset(%d) = Line:%d, Char:%d; want Line:%d, Char:%d",
				tt.offset, pos.Line, pos.Character, tt.wantLine, tt.wantChar)
		}
		if pos.Offset != tt.offset {
			t.Errorf("PositionFromOffset(%d).Offset = %d; want %d",
				tt.offset, pos.Offset, tt.offset)
		}
	}
}

func TestCollector(t *testing.T) {
	content := "CREATE TABLE users (\n  id BIGINT\n);"
	collector := NewCollector("test.sql", content)

	// Add some diagnostics
	r1 := RangeFromOffsets(content, 0, 6) // "CREATE"
	collector.AddError(r1, "test_error", "Test error message")

	r2 := RangeFromOffsets(content, 23, 25) // "id"
	collector.AddWarning(r2, "test_warning", "Test warning message")

	// Check counts
	if collector.Count() != 2 {
		t.Errorf("Count() = %d; want 2", collector.Count())
	}

	if !collector.HasErrors() {
		t.Error("HasErrors() = false; want true")
	}

	// Check filtering
	errors := collector.Errors()
	if len(errors) != 1 {
		t.Errorf("Errors() returned %d; want 1", len(errors))
	}

	warnings := collector.Warnings()
	if len(warnings) != 1 {
		t.Errorf("Warnings() returned %d; want 1", len(warnings))
	}

	// Check sorting
	all := collector.All()
	if len(all) != 2 {
		t.Errorf("All() returned %d diagnostics; want 2", len(all))
	}
	// Should be sorted by position
	if all[0].Severity != SeverityError {
		t.Error("First diagnostic should be the error (earlier position)")
	}
}

func TestDiagnosticFormatter(t *testing.T) {
	content := "CREATE TABLE users (\n  id BIGINT UNQUE\n);"
	r := RangeFromOffsets(content, strings.Index(content, "UNQUE"), strings.Index(content, "UNQUE")+5)

	diag := NewDiagnostic(r, SeverityError, "syntax_error", "Invalid keyword 'UNQUE'. Did you mean 'UNIQUE'?")

	formatter := NewFormatter()
	formatter.ShowSource = true
	formatter.ShowCodeContext = true

	formatted := formatter.Format(diag, "test.sql", content)

	// Check that formatted output contains expected elements
	if !strings.Contains(formatted, "test.sql") {
		t.Error("Formatted output should contain filename")
	}
	if !strings.Contains(formatted, "ERROR") {
		t.Error("Formatted output should contain severity")
	}
	if !strings.Contains(formatted, "UNQUE") {
		t.Error("Formatted output should contain the problematic token")
	}
	if !strings.Contains(formatted, "→") {
		t.Error("Formatted output should contain arrow marker")
	}
	if !strings.Contains(formatted, "~") {
		t.Error("Formatted output should contain squiggly underline")
	}
}

func TestRangeFromOffsets(t *testing.T) {
	content := "line 1\nline 2"

	r := RangeFromOffsets(content, 0, 6)

	if r.Start.Line != 0 || r.Start.Character != 0 {
		t.Errorf("Range start = Line:%d, Char:%d; want Line:0, Char:0",
			r.Start.Line, r.Start.Character)
	}

	if r.End.Line != 0 || r.End.Character != 6 {
		t.Errorf("Range end = Line:%d, Char:%d; want Line:0, Char:6",
			r.End.Line, r.End.Character)
	}
}
