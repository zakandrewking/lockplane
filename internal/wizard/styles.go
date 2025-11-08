package wizard

import (
	"github.com/charmbracelet/lipgloss"
)

// Color palette
var (
	// Primary colors
	colorPrimary = lipgloss.Color("86")  // Cyan
	colorSuccess = lipgloss.Color("42")  // Green
	colorError   = lipgloss.Color("196") // Red
	colorInfo    = lipgloss.Color("75")  // Blue
	colorMuted   = lipgloss.Color("240") // Gray
)

// Style definitions
var (
	// Header styles
	headerStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1)

	// Section styles
	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(colorPrimary).
				Bold(true).
				MarginTop(1)

	// Text styles
	labelStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Status styles
	successStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	errorStyle = lipgloss.NewStyle().
			Foreground(colorError).
			Bold(true)

	infoStyle = lipgloss.NewStyle().
			Foreground(colorInfo)

	// Selection styles
	selectedStyle = lipgloss.NewStyle().
			Foreground(colorSuccess).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(colorMuted)

	// Border styles
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorMuted).
			Padding(1, 2)

	// Tip box style
	tipBoxStyle = lipgloss.NewStyle().
			Foreground(colorInfo).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorInfo).
			Padding(0, 1).
			MarginTop(1)

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorMuted).
			Italic(true).
			MarginTop(1)
)

// Icons
const (
	iconTool     = "🔧"
	iconDatabase = "📦"
	iconSuccess  = "✓"
	iconError    = "✗"
	iconWarning  = "⚠"
	iconInfo     = "💡"
	iconSecurity = "🔒"
	iconCheck    = "✓"
	iconSpinner  = "⏳"
	iconArrow    = "►"
)

// Helper functions for styled output
func renderHeader(text string) string {
	return headerStyle.Render(iconTool + " " + text)
}

func renderSectionHeader(text string) string {
	return sectionHeaderStyle.Render(iconDatabase + " " + text)
}

func renderSuccess(text string) string {
	return successStyle.Render(iconSuccess + " " + text)
}

func renderError(text string) string {
	return errorStyle.Render(iconError + " " + text)
}

func renderInfo(text string) string {
	return tipBoxStyle.Render(iconInfo + " " + text)
}

func renderOption(index int, selected bool, text string) string {
	if selected {
		return selectedStyle.Render(iconArrow + " " + text)
	}
	return unselectedStyle.Render("  " + text)
}

func renderStatusBar(text string) string {
	return statusBarStyle.Render(text)
}
