// Package wizard implements the interactive setup wizard for lockplane init.
//
// This file contains the main wizard flow, prompting users for database
// configuration, testing connections, and generating lockplane.toml files.
package wizard

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// New creates a new wizard model
func New() WizardModel {
	return WizardModel{
		state:        StateWelcome,
		environments: []EnvironmentInput{},
		errors:       make(map[string]string),
		inputs:       []textinput.Model{},
		dbTypeIndex:  0,
	}
}

// Init initializes the wizard (Bubble Tea Init)
func (m WizardModel) Init() tea.Cmd {
	return checkForExistingConfig
}

// Update handles state transitions (Bubble Tea Update)
func (m *WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			// Quit immediately on Ctrl-C
			return m, tea.Quit

		case "esc":
			// On welcome screen, exit immediately
			if m.state == StateWelcome || m.state == StateCheckExisting {
				m.cancelled = true
				return m, tea.Quit
			}
			// Go back to previous state
			return m.handleBack()

		case "enter":
			return m.handleEnter()

		case "up", "k":
			return m.handleUp()

		case "down", "j":
			return m.handleDown()

		case "tab":
			return m.handleTab()

		default:
			// Handle text input
			return m.handleTextInput(msg)
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case connectionTestResultMsg:
		m.testingConnection = false
		if msg.err != nil {
			m.connectionError = msg.err
			m.connectionTestResult = "failed"
		} else {
			m.connectionTestResult = "success"
			m.connectionError = nil
		}
		return m, nil

	case fileCreationResultMsg:
		if msg.err != nil {
			m.err = msg.err
			m.state = StateError
			return m, nil
		}
		m.result = msg.result
		m.state = StateDone
		// Quit immediately after showing the done screen
		return m, tea.Quit

	case existingConfigMsg:
		if msg.path != "" {
			// Found existing config
			m.existingConfigPath = msg.path
			m.existingEnvNames = msg.envNames
			m.allEnvironments = make([]string, len(msg.envNames))
			copy(m.allEnvironments, msg.envNames)
			m.state = StateCheckExisting
		} else {
			// No existing config, go to welcome
			m.state = StateWelcome
		}
		return m, nil
	}

	return m, nil
}

// View renders the wizard UI (Bubble Tea View)
func (m WizardModel) View() string {
	// Show clean exit message if cancelled
	if m.cancelled {
		return labelStyle.Render("lockplane init cancelled")
	}

	switch m.state {
	case StateWelcome:
		return m.renderWelcome()
	case StateCheckExisting:
		return m.renderCheckExisting()
	case StateDatabaseType:
		return m.renderDatabaseType()
	case StatePostgresInputMethod:
		return m.renderPostgresInputMethod()
	case StateShadowInfo:
		return m.renderShadowInfo()
	case StateConnectionDetails:
		return m.renderConnectionDetails()
	case StateTestConnection:
		return m.renderTestConnection()
	case StateAddAnother:
		return m.renderAddAnother()
	case StateSummary:
		return m.renderSummary()
	case StateCreating:
		return m.renderCreating()
	case StateDone:
		return m.renderDone()
	case StateError:
		return m.renderError()
	default:
		return "Unknown state"
	}
}

// State transition handlers

func (m *WizardModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateWelcome:
		m.state = StateDatabaseType
		return m, nil

	case StateCheckExisting:
		// Continue with guided environment setup
		m.state = StateDatabaseType
		return m, nil

	case StateDatabaseType:
		// Set the selected database type
		dbType := DatabaseTypes[m.dbTypeIndex]
		m.currentEnv = EnvironmentInput{}
		m.currentEnv.DatabaseType = dbType.ID
		// For Postgres, show input method selection
		if dbType.ID == "postgres" {
			m.state = StatePostgresInputMethod
			m.postgresInputMethod = 0 // Reset to default (individual fields)
		} else {
			m.state = StateShadowInfo
		}
		return m, nil

	case StatePostgresInputMethod:
		// User chose input method for Postgres
		m.state = StateShadowInfo
		return m, nil

	case StateShadowInfo:
		m.initializeInputs()
		m.state = StateConnectionDetails
		return m, nil

	case StateConnectionDetails:
		// Collect input values
		if err := m.collectInputValues(); err != nil {
			return m, nil
		}
		m.state = StateTestConnection
		m.testingConnection = true
		return m, m.testConnection()

	case StateTestConnection:
		switch m.connectionTestResult {
		case "success":
			m.state = StateAddAnother
			// Save the current environment
			m.environments = append(m.environments, m.currentEnv)
			// Add to all environments list
			m.allEnvironments = append(m.allEnvironments, m.currentEnv.Name)
			// Reset current environment
			m.currentEnv = EnvironmentInput{}
			// Reset add another choice
			m.addAnotherChoice = 0
			return m, nil
		case "failed":
			// Handle retry choice
			switch m.retryChoice {
			case 0: // Retry
				m.connectionTestResult = ""
				m.connectionError = nil
				m.testingConnection = true
				return m, m.testConnection()
			case 1: // Edit
				m.state = StateConnectionDetails
				m.connectionTestResult = ""
				m.connectionError = nil
				m.retryChoice = 0
				return m, nil
			case 2: // Quit
				return m, tea.Quit
			}
		}
		return m, nil

	case StateAddAnother:
		switch m.addAnotherChoice {
		case 0: // Add another environment
			m.state = StateDatabaseType
			m.addAnotherChoice = 0 // Reset for next time
			return m, nil
		case 1: // Save and finish
			m.state = StateSummary
			return m, nil
		}
		return m, nil

	case StateSummary:
		m.state = StateCreating
		return m, m.createFiles()

	case StateDone:
		return m, tea.Quit

	case StateError:
		return m, tea.Quit
	}

	return m, nil
}

func (m *WizardModel) handleUp() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateDatabaseType:
		if m.dbTypeIndex > 0 {
			m.dbTypeIndex--
		}
	case StatePostgresInputMethod:
		if m.postgresInputMethod > 0 {
			m.postgresInputMethod--
		}
	case StateConnectionDetails:
		if m.focusIndex > 0 {
			m.focusIndex--
			m.updateInputFocus()
		}
	case StateTestConnection:
		if m.connectionTestResult == "failed" && m.retryChoice > 0 {
			m.retryChoice--
		}
	case StateAddAnother:
		if m.addAnotherChoice > 0 {
			m.addAnotherChoice--
		}
	}
	return m, nil
}

func (m *WizardModel) handleDown() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateDatabaseType:
		if m.dbTypeIndex < len(DatabaseTypes)-1 {
			m.dbTypeIndex++
		}
	case StatePostgresInputMethod:
		if m.postgresInputMethod < 1 {
			m.postgresInputMethod++
		}
	case StateConnectionDetails:
		if m.focusIndex < len(m.inputs)-1 {
			m.focusIndex++
			m.updateInputFocus()
		}
	case StateTestConnection:
		if m.connectionTestResult == "failed" && m.retryChoice < 2 {
			m.retryChoice++
		}
	case StateAddAnother:
		if m.addAnotherChoice < 1 {
			m.addAnotherChoice++
		}
	}
	return m, nil
}

func (m *WizardModel) handleTab() (tea.Model, tea.Cmd) {
	if m.state == StateConnectionDetails {
		m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
		m.updateInputFocus()
	}
	return m, nil
}

func (m *WizardModel) handleBack() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateDatabaseType:
		// Go back to welcome or existing config check
		if m.existingConfigPath != "" {
			m.state = StateCheckExisting
		} else {
			m.state = StateWelcome
		}
		return m, nil

	case StatePostgresInputMethod:
		// Go back to database type selection
		m.state = StateDatabaseType
		m.currentEnv = EnvironmentInput{}
		return m, nil

	case StateShadowInfo:
		if m.currentEnv.DatabaseType == "postgres" {
			m.state = StatePostgresInputMethod
		} else {
			m.state = StateDatabaseType
		}
		return m, nil

	case StateConnectionDetails:
		// Go back to appropriate previous state
		if m.currentEnv.DatabaseType == "postgres" {
			// For Postgres, go back to input method selection
			m.state = StatePostgresInputMethod
		} else {
			// For other databases, go back to database type selection
			m.state = StateDatabaseType
		}
		// Clear input data
		m.inputs = []textinput.Model{}
		m.focusIndex = 0
		m.errors = make(map[string]string)
		return m, nil

	case StateTestConnection:
		// If connection failed, go back to edit details
		if m.connectionTestResult == "failed" {
			m.state = StateConnectionDetails
			m.connectionTestResult = ""
			m.connectionError = nil
			m.retryChoice = 0
			return m, nil
		}
		// If testing or success, stay on this screen
		return m, nil

	case StateAddAnother:
		// Review summary when pressing escape here
		if len(m.environments) > 0 {
			m.state = StateSummary
			return m, nil
		}
		return m, nil

	case StateSummary:
		// Return to add another screen to make changes
		m.state = StateAddAnother
		return m, nil

	case StateDone, StateError:
		// Exit on escape
		return m, tea.Quit

	default:
		// For other states (Welcome, CheckExisting, Summary, Creating), do nothing
		return m, nil
	}
}

func (m *WizardModel) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.state == StateConnectionDetails && len(m.inputs) > 0 {
		var cmd tea.Cmd
		m.inputs[m.focusIndex], cmd = m.inputs[m.focusIndex].Update(msg)
		return m, cmd
	}
	return m, nil
}

// Input management

func (m *WizardModel) initializeInputs() {
	m.inputs = []textinput.Model{}
	m.focusIndex = 0

	switch m.currentEnv.DatabaseType {
	case "postgres":
		if m.postgresInputMethod == 1 {
			// Connection string input
			m.inputs = append(m.inputs,
				m.makeInput("Environment name", "local", false),
				m.makeInput("Connection string", "postgresql://user:password@localhost:5432/database?sslmode=disable", false),
			)
		} else {
			// Individual fields input
			m.inputs = append(m.inputs,
				m.makeInput("Environment name", "local", false),
				m.makeInput("Host", "localhost", false),
				m.makeInput("Port", "5432", false),
				m.makeInput("Database", "lockplane", false),
				m.makeInput("User", "lockplane", false),
				m.makeInput("Password", "lockplane", true),
			)
		}
	case "sqlite":
		m.inputs = append(m.inputs,
			m.makeInput("Environment name", "local", false),
			m.makeInput("Database file path", "schema/lockplane.db", false),
		)
	case "libsql":
		m.inputs = append(m.inputs,
			m.makeInput("Environment name", "production", false),
			m.makeInput("Database URL", "libsql://[name]-[org].turso.io", false),
			m.makeInput("Auth token", "", true),
		)
	}

	if len(m.inputs) > 0 {
		m.inputs[0].Focus()
		m.inputs[0].PromptStyle = focusedPromptStyle
	}
}

func (m *WizardModel) makeInput(placeholder, value string, isPassword bool) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.SetValue(value)
	input.Prompt = "→ "
	input.PromptStyle = blurredPromptStyle
	input.TextStyle = infoStyle
	input.Width = 50
	if isPassword {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '•'
	}
	return input
}

func (m *WizardModel) updateInputFocus() {
	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i].Focus()
			m.inputs[i].PromptStyle = focusedPromptStyle
		} else {
			m.inputs[i].Blur()
			m.inputs[i].PromptStyle = blurredPromptStyle
		}
	}
}

func (m *WizardModel) collectInputValues() error {
	switch m.currentEnv.DatabaseType {
	case "postgres":
		if m.postgresInputMethod == 1 {
			// Connection string input
			if len(m.inputs) < 2 {
				return fmt.Errorf("not enough inputs")
			}
			m.currentEnv.Name = m.inputs[0].Value()
			connStr := m.inputs[1].Value()

			// Validate environment name
			if err := ValidateEnvironmentName(m.currentEnv.Name); err != nil {
				m.errors["name"] = err.Error()
				return err
			}

			// Parse connection string
			parsedEnv, err := ParsePostgresConnectionString(connStr)
			if err != nil {
				m.errors["connection_string"] = err.Error()
				return err
			}

			// Copy parsed values to current environment
			m.currentEnv.Host = parsedEnv.Host
			m.currentEnv.Port = parsedEnv.Port
			m.currentEnv.Database = parsedEnv.Database
			m.currentEnv.User = parsedEnv.User
			m.currentEnv.Password = parsedEnv.Password
			m.currentEnv.SSLMode = parsedEnv.SSLMode
			m.currentEnv.ShadowDBPort = parsedEnv.ShadowDBPort
		} else {
			// Individual fields input
			if len(m.inputs) < 6 {
				return fmt.Errorf("not enough inputs")
			}
			m.currentEnv.Name = m.inputs[0].Value()
			m.currentEnv.Host = m.inputs[1].Value()
			m.currentEnv.Port = m.inputs[2].Value()
			m.currentEnv.Database = m.inputs[3].Value()
			m.currentEnv.User = m.inputs[4].Value()
			m.currentEnv.Password = m.inputs[5].Value()

			// Validate
			if err := ValidateEnvironmentName(m.currentEnv.Name); err != nil {
				m.errors["name"] = err.Error()
				return err
			}
			if err := ValidatePort(m.currentEnv.Port); err != nil {
				m.errors["port"] = err.Error()
				return err
			}
		}

	case "sqlite":
		if len(m.inputs) < 2 {
			return fmt.Errorf("not enough inputs")
		}
		m.currentEnv.Name = m.inputs[0].Value()
		m.currentEnv.FilePath = m.inputs[1].Value()

		if err := ValidateEnvironmentName(m.currentEnv.Name); err != nil {
			m.errors["name"] = err.Error()
			return err
		}

	case "libsql":
		if len(m.inputs) < 3 {
			return fmt.Errorf("not enough inputs")
		}
		m.currentEnv.Name = m.inputs[0].Value()
		m.currentEnv.URL = m.inputs[1].Value()
		m.currentEnv.AuthToken = m.inputs[2].Value()

		if err := ValidateEnvironmentName(m.currentEnv.Name); err != nil {
			m.errors["name"] = err.Error()
			return err
		}
	}

	return nil
}

// Message types for async operations

type connectionTestResultMsg struct {
	err error
}

func (m WizardModel) testConnection() tea.Cmd {
	return func() tea.Msg {
		var connStr string
		switch m.currentEnv.DatabaseType {
		case "postgres":
			connStr = BuildPostgresConnectionString(m.currentEnv)
		case "sqlite":
			connStr = BuildSQLiteConnectionString(m.currentEnv)
		case "libsql":
			connStr = BuildLibSQLConnectionString(m.currentEnv)
		}

		err := TestConnection(connStr, m.currentEnv.DatabaseType)
		return connectionTestResultMsg{err: err}
	}
}

type fileCreationResultMsg struct {
	result *InitResult
	err    error
}

func (m WizardModel) createFiles() tea.Cmd {
	return func() tea.Msg {
		result, err := GenerateFiles(m.environments)
		return fileCreationResultMsg{result: result, err: err}
	}
}

type existingConfigMsg struct {
	path     string
	envNames []string
}

func checkForExistingConfig() tea.Msg {
	// Check for config in current directory
	configPath := "lockplane.toml"
	envNames, err := getEnvironmentNames(configPath)
	if err == nil && len(envNames) > 0 {
		return existingConfigMsg{path: configPath, envNames: envNames}
	}

	// No existing config
	return existingConfigMsg{}
}

func getEnvironmentNames(configPath string) ([]string, error) {
	// Simple TOML parsing to extract environment names
	// We look for [environments.NAME] sections
	content, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var envNames []string
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "[environments.") && strings.HasSuffix(line, "]") {
			// Extract environment name from [environments.NAME]
			envName := strings.TrimPrefix(line, "[environments.")
			envName = strings.TrimSuffix(envName, "]")
			envNames = append(envNames, envName)
		}
	}

	return envNames, nil
}

// View renderers

func (m WizardModel) renderWelcome() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString("Welcome! Let's set up Lockplane for your project.\n\n")
	b.WriteString(renderInfo("This wizard will help you:\n" +
		"  • Configure database connections\n" +
		"  • Set up shadow databases for safe migrations\n" +
		"  • Create environment-specific config files"))
	b.WriteString("\n\n")
	b.WriteString(renderCallToAction("Press Enter to continue"))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Ctrl-C to quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderCheckExisting() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSuccess("Found existing configuration!"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Config: %s\n\n", m.existingConfigPath))

	if len(m.existingEnvNames) > 0 {
		b.WriteString(renderSectionHeader("Existing Environments:"))
		b.WriteString("\n")
		for _, envName := range m.existingEnvNames {
			b.WriteString(fmt.Sprintf("  • %s\n", envName))
		}
		b.WriteString("\n")
	}

	b.WriteString(renderInfo("You can add new environments or update existing ones.\n" +
		"If you add an environment with the same name as an\n" +
		"existing one, it will be updated."))
	b.WriteString("\n\n")
	b.WriteString(renderCallToAction("Press Enter to add environment"))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Esc: back  •  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderDatabaseType() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Database Type Selection"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("What database are you using?"))
	b.WriteString("\n\n")

	for i, dbType := range DatabaseTypes {
		line := fmt.Sprintf("%d. %s %s (%s)",
			i+1, dbType.Icon, dbType.DisplayName, dbType.Description)
		b.WriteString(renderOption(i, i == m.dbTypeIndex, line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(renderInfo("PostgreSQL provides the most features including\nshadow databases for safe migration testing."))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("↑/↓: navigate  Enter: select  Esc: back  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderPostgresInputMethod() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("PostgreSQL Connection Input"))
	b.WriteString("\n\n")
	b.WriteString(labelStyle.Render("How would you like to provide connection details?"))
	b.WriteString("\n\n")

	// Option 0: Individual fields
	option0 := "Enter individual fields (host, port, database, user, password)"
	b.WriteString(renderOption(0, m.postgresInputMethod == 0, option0))
	b.WriteString("\n")

	// Option 1: Connection string
	option1 := "Paste connection string (postgresql://...)"
	b.WriteString(renderOption(1, m.postgresInputMethod == 1, option1))
	b.WriteString("\n\n")

	if m.postgresInputMethod == 0 {
		b.WriteString(renderInfo("Individual fields provide guided input with\ndefaults for local development."))
	} else {
		b.WriteString(renderInfo("Paste a connection string like:\npostgresql://user:pass@host:5432/db?sslmode=disable"))
	}

	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("↑/↓: navigate  Enter: select  Esc: back  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderShadowInfo() string {
	var b strings.Builder

	dbType := DatabaseTypes[m.dbTypeIndex]

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Shadow Database Preview"))
	b.WriteString("\n\n")
	b.WriteString(renderInfo("Lockplane always tests migrations on a dedicated\nshadow database before touching your real data."))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Selected database: %s %s\n\n", dbType.Icon, dbType.DisplayName))

	switch m.currentEnv.DatabaseType {
	case "postgres":
		b.WriteString(renderInfo("PostgreSQL shadow configuration:"))
		b.WriteString("\n")
		b.WriteString("  • Default name: <database>_shadow (myapp → myapp_shadow)\n")
		b.WriteString("  • Default port: 5433 (customizable via .env or --shadow-db-port)\n")
		b.WriteString("  • Uses the same host, user, and password as your primary DB\n")
		b.WriteString("  • Works with SHADOW_SCHEMA if you prefer schema-based isolation\n")
	case "sqlite":
		b.WriteString(renderInfo("SQLite shadow configuration:"))
		b.WriteString("\n")
		b.WriteString("  • Creates a real file next to your main database\n")
		b.WriteString("  • Naming convention: <filename>_shadow.db\n")
		b.WriteString("  • Example: schema/lockplane.db → schema/lockplane_shadow.db\n")
		b.WriteString("  • Keeps migrations reproducible between runs\n")
	case "libsql":
		b.WriteString(renderInfo("libSQL/Turso shadow configuration:"))
		b.WriteString("\n")
		b.WriteString("  • Uses a local SQLite file (./schema/turso_shadow.db)\n")
		b.WriteString("  • No second Turso database required\n")
		b.WriteString("  • Keeps edge traffic low while fully validating migrations\n")
	}

	b.WriteString("\n")
	b.WriteString(renderInfo("You can override these settings later via\n`--shadow-db`, `.env.<env>`, or `SHADOW_SCHEMA`."))
	b.WriteString("\n\n")
	b.WriteString(renderCallToAction("Press Enter to configure connection details"))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Enter: continue  Esc: back  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderConnectionDetails() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Connection Details"))
	b.WriteString("\n\n")

	dbType := DatabaseTypes[m.dbTypeIndex]
	b.WriteString(fmt.Sprintf("Database: %s %s\n\n", dbType.Icon, dbType.DisplayName))

	// Render input fields
	for i, input := range m.inputs {
		label := input.Placeholder
		if i == m.focusIndex {
			b.WriteString(selectedStyle.Render("► " + label + ":"))
		} else {
			b.WriteString(labelStyle.Render("  " + label + ":"))
		}
		b.WriteString("\n  ")
		b.WriteString(input.View())
		b.WriteString("\n\n")
	}

	// Show validation errors
	if len(m.errors) > 0 {
		for _, errMsg := range m.errors {
			b.WriteString(renderError(errMsg))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Database-specific tips
	switch m.currentEnv.DatabaseType {
	case "postgres":
		b.WriteString(renderInfo("Shadow DB will be auto-configured on port 5433\nfor safe migration testing."))
	case "sqlite":
		b.WriteString(renderInfo("SQLite uses file-based storage.\nShadow DB will be configured automatically."))
	case "libsql":
		b.WriteString(renderInfo("libSQL/Turso is an edge database.\nLocal SQLite shadow DB for validation."))
	}

	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("↑/↓ or Tab: navigate  Enter: test connection  Esc: back  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderTestConnection() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Testing Connection"))
	b.WriteString("\n\n")

	if m.testingConnection {
		b.WriteString(infoStyle.Render(iconSpinner + " Testing connection..."))
	} else if m.connectionTestResult == "success" {
		b.WriteString(renderSuccess("Connection successful!"))
		b.WriteString("\n\n")
		b.WriteString("Connected to: " + m.currentEnv.Name)
	} else if m.connectionTestResult == "failed" {
		b.WriteString(renderError("Connection failed"))
		b.WriteString("\n\n")
		if m.connectionError != nil {
			b.WriteString(errorStyle.Render("Error: " + m.connectionError.Error()))
		}
		b.WriteString("\n\n")
		b.WriteString("What would you like to do?\n\n")

		// Retry option
		b.WriteString(renderOption(0, m.retryChoice == 0, "Retry connection"))
		b.WriteString("\n")

		// Edit option
		b.WriteString(renderOption(1, m.retryChoice == 1, "Edit connection details"))
		b.WriteString("\n")

		// Quit option
		b.WriteString(renderOption(2, m.retryChoice == 2, "Quit wizard"))
		b.WriteString("\n")
	}

	b.WriteString("\n\n")
	if m.connectionTestResult == "failed" {
		b.WriteString(renderStatusBar("↑/↓: navigate  Enter: select  Esc: back  Ctrl-C: quit"))
	} else {
		b.WriteString(renderStatusBar("Press Enter to continue"))
	}

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderAddAnother() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Environment Added"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("✓ Added environment: %s\n\n", m.environments[len(m.environments)-1].Name))

	// Show current environment count
	if len(m.allEnvironments) > 0 {
		b.WriteString(renderInfo(fmt.Sprintf("Total environments configured: %d", len(m.allEnvironments))))
		b.WriteString("\n\n")
	}

	b.WriteString("What would you like to do next?\n\n")

	// Option 0: Add another environment
	b.WriteString(renderOption(0, m.addAnotherChoice == 0, "Add another environment (e.g., staging, production)"))
	b.WriteString("\n")

	// Option 1: Save and finish
	b.WriteString(renderOption(1, m.addAnotherChoice == 1, "Save and finish"))
	b.WriteString("\n\n")

	b.WriteString(renderStatusBar("↑/↓: navigate  Enter: select  Esc: review summary  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderSummary() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Configuration Summary"))
	b.WriteString("\n\n")

	if len(m.allEnvironments) > 0 {
		b.WriteString(fmt.Sprintf("Total configured environments: %d\n\n", len(m.allEnvironments)))
	}

	if len(m.existingEnvNames) > 0 {
		b.WriteString(renderInfo("Existing environments (will be preserved unless noted):"))
		b.WriteString("\n")
		for _, envName := range m.existingEnvNames {
			status := ""
			for _, newEnv := range m.environments {
				if newEnv.Name == envName {
					status = " (will be updated)"
					break
				}
			}
			b.WriteString(fmt.Sprintf("  • %s%s\n", envName, status))
		}
		b.WriteString("\n")
	}

	if len(m.environments) > 0 {
		b.WriteString(renderSectionHeader("New / Updated environments"))
		b.WriteString("\n")
		for _, env := range m.environments {
			b.WriteString(renderSuccess(fmt.Sprintf("%s (%s)", env.Name, strings.ToUpper(env.DatabaseType))))
			b.WriteString("\n")
			b.WriteString(fmt.Sprintf("  • Primary: %s\n", formatPrimaryConnection(env)))
			b.WriteString(fmt.Sprintf("  • Shadow:  %s\n", formatShadowConfiguration(env)))
			if env.DatabaseType == "postgres" {
				b.WriteString("    Tip: Set SHADOW_SCHEMA in .env if you prefer schema-based isolation.\n")
			}
			b.WriteString("\n")
		}
	}

	b.WriteString(renderSectionHeader("Files to create/update"))
	b.WriteString("\n")
	if len(m.existingEnvNames) > 0 {
		b.WriteString("  • lockplane.toml (update existing configuration)\n")
	} else {
		b.WriteString("  • lockplane.toml (new)\n")
	}
	for _, env := range m.environments {
		b.WriteString(fmt.Sprintf("  • .env.%s (new credentials & shadow DB settings)\n", env.Name))
	}
	b.WriteString("  • .gitignore (ensure secrets stay untracked)\n")

	b.WriteString("\n")
	b.WriteString(renderInfo("Need to make changes? Press Esc to go back before files are generated."))
	b.WriteString("\n\n")
	b.WriteString(renderCallToAction("Press Enter to create configuration files"))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Enter: save  Esc: back  Ctrl-C: quit"))

	return borderStyle.Render(b.String())
}

func formatPrimaryConnection(env EnvironmentInput) string {
	switch env.DatabaseType {
	case "postgres":
		host := fallback(env.Host, "localhost")
		port := fallback(env.Port, "5432")
		db := fallback(env.Database, "lockplane")
		user := fallback(env.User, "lockplane")
		ssl := fallback(env.SSLMode, defaultSSLMode(host))
		return fmt.Sprintf("%s@%s:%s/%s (sslmode=%s)", user, host, port, db, ssl)
	case "sqlite":
		path := fallback(env.FilePath, "schema/lockplane.db")
		return path
	case "libsql":
		url := fallback(env.URL, "libsql://<org>-<db>.turso.io")
		return url
	default:
		return "n/a"
	}
}

func formatShadowConfiguration(env EnvironmentInput) string {
	switch env.DatabaseType {
	case "postgres":
		host := fallback(env.Host, "localhost")
		port := fallback(env.ShadowDBPort, "5433")
		db := fallback(env.Database, "lockplane") + "_shadow"
		user := fallback(env.User, "lockplane")
		return fmt.Sprintf("%s@%s:%s/%s", user, host, port, db)
	case "sqlite":
		path := BuildSQLiteShadowConnectionString(env)
		if strings.TrimSpace(path) == "" {
			path = fallback(env.FilePath, "schema/lockplane.db") + "_shadow"
		}
		return path
	case "libsql":
		path := BuildLibSQLShadowConnectionString(env)
		if strings.TrimSpace(path) == "" {
			path = "./schema/turso_shadow.db"
		}
		return path
	default:
		return "n/a"
	}
}

func fallback(value, alt string) string {
	if strings.TrimSpace(value) == "" {
		return alt
	}
	return value
}

func defaultSSLMode(host string) string {
	if host == "localhost" || host == "127.0.0.1" {
		return "disable"
	}
	return "require"
}

func (m WizardModel) renderCreating() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(infoStyle.Render(iconSpinner + " Creating project structure..."))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderDone() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSuccess("Setup complete!"))
	b.WriteString("\n\n")

	if m.result != nil {
		b.WriteString("Created:\n")
		if m.result.ConfigCreated {
			b.WriteString(fmt.Sprintf("  %s %s\n", iconCheck, m.result.ConfigPath))
		}
		for _, envFile := range m.result.EnvFiles {
			b.WriteString(fmt.Sprintf("  %s %s\n", iconCheck, envFile))
		}
		if m.result.GitignoreUpdated {
			b.WriteString(fmt.Sprintf("  %s .gitignore updated\n", iconCheck))
		}
	}

	b.WriteString("\n")
	b.WriteString(renderInfo("Ready to introspect your database!\n" +
		"  Run: lockplane introspect\n\n" +
		"  This will capture your current schema and\n" +
		"  save it to schema/schema.json"))
	b.WriteString("\n\n")
	b.WriteString("Next steps:\n")
	b.WriteString("  1. Run introspection to capture current schema\n")
	b.WriteString("  2. Review generated files\n")
	b.WriteString("  3. Make schema changes and generate migration plans\n")

	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Press Enter to exit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderError() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderError("An error occurred"))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(m.err.Error()))
	}

	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Press Enter to exit"))

	return borderStyle.Render(b.String())
}

// Run starts the wizard
func Run() error {
	m := New()
	p := tea.NewProgram(&m)
	_, err := p.Run()
	return err
}
