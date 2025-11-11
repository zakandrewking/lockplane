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
func (m WizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

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
		return m, nil

	case existingConfigMsg:
		if msg.path != "" {
			// Found existing config
			m.existingConfigPath = msg.path
			m.existingEnvNames = msg.envNames
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
	switch m.state {
	case StateWelcome:
		return m.renderWelcome()
	case StateCheckExisting:
		return m.renderCheckExisting()
	case StateDatabaseType:
		return m.renderDatabaseType()
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

func (m WizardModel) handleEnter() (tea.Model, tea.Cmd) {
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
		m.currentEnv.DatabaseType = dbType.ID
		m.state = StateConnectionDetails
		m.initializeInputs()
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
			// Reset current environment
			m.currentEnv = EnvironmentInput{}
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
		m.state = StateSummary
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

func (m WizardModel) handleUp() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateDatabaseType:
		if m.dbTypeIndex > 0 {
			m.dbTypeIndex--
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
	}
	return m, nil
}

func (m WizardModel) handleDown() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateDatabaseType:
		if m.dbTypeIndex < len(DatabaseTypes)-1 {
			m.dbTypeIndex++
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
	}
	return m, nil
}

func (m WizardModel) handleTab() (tea.Model, tea.Cmd) {
	if m.state == StateConnectionDetails {
		m.focusIndex = (m.focusIndex + 1) % len(m.inputs)
		m.updateInputFocus()
	}
	return m, nil
}

func (m WizardModel) handleTextInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
		m.inputs = append(m.inputs,
			m.makeInput("Environment name", "local", false),
			m.makeInput("Host", "localhost", false),
			m.makeInput("Port", "5432", false),
			m.makeInput("Database", "lockplane", false),
			m.makeInput("User", "lockplane", false),
			m.makeInput("Password", "lockplane", true),
		)
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
	}
}

func (m *WizardModel) makeInput(placeholder, value string, isPassword bool) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.SetValue(value)
	if isPassword {
		input.EchoMode = textinput.EchoPassword
		input.EchoCharacter = '*'
	}
	return input
}

func (m *WizardModel) updateInputFocus() {
	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *WizardModel) collectInputValues() error {
	switch m.currentEnv.DatabaseType {
	case "postgres":
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
	// Check for config in schema/ (preferred)
	configPath := "schema/lockplane.toml"
	envNames, err := getEnvironmentNames(configPath)
	if err == nil && len(envNames) > 0 {
		return existingConfigMsg{path: configPath, envNames: envNames}
	}

	// Check for legacy config at root
	configPath = "lockplane.toml"
	envNames, err = getEnvironmentNames(configPath)
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
	b.WriteString(renderStatusBar("Press Enter to continue, q to quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderCheckExisting() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSuccess("Found existing configuration!"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Config: %s\n", m.existingConfigPath))
	b.WriteString(fmt.Sprintf("Environments: %s\n", strings.Join(m.existingEnvNames, ", ")))
	b.WriteString("\n\n")
	b.WriteString(renderInfo("Let's set up your database environments.\n" +
		"I'll walk you through adding new connections or\n" +
		"updating existing ones."))
	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Press Enter to continue, q to quit"))

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
	b.WriteString(renderStatusBar("↑/↓: navigate  Enter: select  q: quit"))

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
		b.WriteString(renderInfo("SQLite uses file-based storage.\nShadow DB disabled to avoid file clutter."))
	case "libsql":
		b.WriteString(renderInfo("libSQL/Turso is an edge database.\nShadow DB not supported."))
	}

	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("↑/↓ or Tab: navigate  Enter: test connection  q: quit"))

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
		b.WriteString(renderStatusBar("↑/↓: navigate  Enter: select  q: quit"))
	} else {
		b.WriteString(renderStatusBar("Press Enter to continue"))
	}

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderAddAnother() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Add Another Environment?"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("✓ Added environment: %s\n\n", m.environments[len(m.environments)-1].Name))
	b.WriteString("Would you like to add another environment?\n")
	b.WriteString("(e.g., staging, production)\n\n")
	b.WriteString(renderStatusBar("Press Enter to continue, q to quit"))

	return borderStyle.Render(b.String())
}

func (m WizardModel) renderSummary() string {
	var b strings.Builder

	b.WriteString(renderHeader("Lockplane Init Wizard"))
	b.WriteString("\n\n")
	b.WriteString(renderSectionHeader("Summary"))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Ready to create configuration for %d environment(s):\n\n", len(m.environments)))

	for _, env := range m.environments {
		b.WriteString(fmt.Sprintf("  • %s (%s)\n", env.Name, env.DatabaseType))
	}

	b.WriteString("\n")
	b.WriteString("This will create:\n")
	b.WriteString("  • schema/lockplane.toml\n")
	for _, env := range m.environments {
		b.WriteString(fmt.Sprintf("  • .env.%s\n", env.Name))
	}
	b.WriteString("  • Update .gitignore\n")

	b.WriteString("\n\n")
	b.WriteString(renderStatusBar("Press Enter to create files, q to quit"))

	return borderStyle.Render(b.String())
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
	p := tea.NewProgram(New())
	_, err := p.Run()
	return err
}
