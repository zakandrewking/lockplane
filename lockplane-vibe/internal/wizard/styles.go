package wizard

import (
	"github.com/charmbracelet/lipgloss"
)

// ASCII art logo for the header
const logoArt = `
 ╔═════════════════════════════════════════════════════════════╗
 ║                                                             ║
 ║   ┃ ┏━┓┏━╸╻┏ ┏━┓╻  ┏━┓┏┓╻┏━╸   ╻┏┓╻╻╺┳╸  ╻ ╻╻┏━┓┏━┓┏━┓╺┳┓   ║
 ║   ┃ ┃ ┃┃  ┣┻┓┣━┛┃  ┣━┫┃┗┫┣╸    ┃┃┗┫┃ ┃   ┃╻┃┃┏━┛┣━┫┣┳┛ ┃┃   ║
 ║   ┗╸┗━┛┗━╸╹ ╹╹  ┗━╸╹ ╹╹ ╹┗━╸   ╹╹ ╹╹ ╹   ┗┻┛╹┗━╸╹ ╹╹┗╸╺┻┛   ║
 ║                                                             ║
 ╚═════════════════════════════════════════════════════════════╝
`

// Color palette - using hex colors for richer, more modern look
var (
	// Primary colors (muted palette that works on light backgrounds)
	colorPrimary   = lipgloss.Color("#374151") // Slate
	colorSuccess   = lipgloss.Color("#2F855A") // Muted green
	colorError     = lipgloss.Color("#C53030") // Muted red
	colorInfo      = lipgloss.Color("#2563EB") // Mid blue
	colorSubtle    = lipgloss.Color("#4B5563") // Gray
	colorHighlight = lipgloss.Color("#1D4ED8") // Accent blue
)

// Style definitions
var (
	// Logo style
	logoStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true)

	// Header styles
	headerStyle = lipgloss.NewStyle().
			Foreground(colorPrimary).
			Bold(true).
			Padding(0, 1)

	// Section styles
	sectionHeaderStyle = lipgloss.NewStyle().
				Foreground(colorInfo).
				Bold(true).
				MarginTop(1)

	// Text styles
	labelStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

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
			Foreground(colorHighlight).
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	// Input styles
	focusedPromptStyle = lipgloss.NewStyle().
				Foreground(colorHighlight).
				Bold(true)

	blurredPromptStyle = lipgloss.NewStyle().
				Foreground(colorSubtle)

	// Border styles
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(1, 2)

	// Tip box style
	tipBoxStyle = lipgloss.NewStyle().
			Foreground(colorInfo).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorSubtle).
			Padding(0, 1).
			MarginTop(1)

	// Status bar style
	statusBarStyle = lipgloss.NewStyle().
			Foreground(colorSubtle).
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
	iconArrow    = "▶"
	iconPostgres = "🐘"
	iconSQLite   = "📁"
	iconLibSQL   = "🌐"
	iconSparkles = "✨"
	iconRocket   = "🚀"
	iconFiles    = "📄"
)

// Helper functions for styled output
func renderLogo() string {
	return logoStyle.Render(logoArt)
}

func renderHeader(text string) string {
	return renderLogo() + "\n" + headerStyle.Render(text)
}

func renderSectionHeader(text string) string {
	return sectionHeaderStyle.Render(text)
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
	return unselectedStyle.Render("    " + text)
}

func renderStatusBar(text string) string {
	return statusBarStyle.Render(text)
}

func renderCallToAction(text string) string {
	return selectedStyle.Render(iconArrow + " " + text)
}
