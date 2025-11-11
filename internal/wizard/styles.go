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
	// Primary colors
	colorPrimary = lipgloss.Color("#7D56F4") // Purple - main brand color
	colorSuccess = lipgloss.Color("#04B575") // Green - success states
	colorError   = lipgloss.Color("#FF4672") // Red - errors
	colorInfo    = lipgloss.Color("#00D9FF") // Cyan - information
	colorSubtle  = lipgloss.Color("#777777") // Gray - muted text
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
			Foreground(lipgloss.Color("#FF6AD5")). // Pink - highlights
			Bold(true)

	unselectedStyle = lipgloss.NewStyle().
			Foreground(colorSubtle)

	// Input styles
	focusedPromptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FF6AD5")). // Pink - focus
				Bold(true)

	blurredPromptStyle = lipgloss.NewStyle().
				Foreground(colorSubtle)

	// Border styles
	borderStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(colorPrimary).
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
			Foreground(colorSubtle).
			Italic(true).
			MarginTop(1)

	// Call to action style - prominent button-like appearance
	callToActionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(colorPrimary).
				Bold(true).
				Padding(1, 4).
				MarginTop(2).
				Align(lipgloss.Center).
				Border(lipgloss.RoundedBorder()).
				BorderForeground(colorPrimary)
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
	return callToActionStyle.Render("⏎  " + text + "  ⏎")
}
