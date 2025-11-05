package main

import (
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

const (
	defaultSchemaDir         = "schema"
	lockplaneConfigFilename  = "lockplane.toml"
	defaultLockplaneTomlBody = `default_environment = "local"

[environments.local]
description = "Local development database"
schema_path = "."
database_url = "postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"
shadow_database_url = "postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable"
`
)

// WizardState represents the current step in the wizard
type WizardState int

const (
	StateWelcome WizardState = iota
	StateCheckExisting
	StateDatabaseType
	StateConnectionDetails
	StateTestingConnection
	StateAddAnother
	StateCreating
	StateDone
)

// DatabaseType represents the type of database
type DatabaseType string

const (
	DBTypePostgres DatabaseType = "postgres"
	DBTypeSQLite   DatabaseType = "sqlite"
	DBTypeLibSQL   DatabaseType = "libsql"
)

// EnvironmentInput holds the collected information for an environment
type EnvironmentInput struct {
	Name         string
	DatabaseType DatabaseType

	// PostgreSQL fields
	Host     string
	Port     string
	Database string
	User     string
	Password string
	SSLMode  string

	// SQLite fields
	FilePath string

	// libSQL fields
	URL       string
	AuthToken string
}

// WizardModel is the bubbletea model for the interactive wizard
type initWizardModel struct {
	state   WizardState
	spinner spinner.Model

	// Environment collection
	currentEnv   EnvironmentInput
	environments []EnvironmentInput

	// Database type selection
	dbTypeChoice int // 0=PostgreSQL, 1=SQLite, 2=libSQL

	// Input fields
	inputs     []textinput.Model
	focusIndex int

	// Connection testing
	testingConnection bool
	testResult        string
	testError         error
	testRetryCount    int

	// Completion
	creating bool
	done     bool
	err      error
	result   *bootstrapResult

	// Control
	shouldQuit bool

	// Config detection
	existingConfigPath string
	addingToExisting   bool
}

type bootstrapResult struct {
	SchemaDir        string
	ConfigPath       string
	EnvFiles         []string
	SchemaDirCreated bool
	ConfigCreated    bool
	ConfigUpdated    bool
	GitignoreUpdated bool
}

func runInit(args []string) {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	yes := fs.Bool("yes", false, "Skip the wizard and accept default values")

	fs.Usage = func() {
		_, _ = fmt.Fprintf(os.Stderr, "Usage: lockplane init [--yes]\n\n")
		_, _ = fmt.Fprintf(os.Stderr, "Launch the interactive Lockplane project wizard. The wizard bootstraps\n")
		_, _ = fmt.Fprintf(os.Stderr, "the schema/ directory and creates schema/lockplane.toml.\n")
		_, _ = fmt.Fprintf(os.Stderr, "Use --yes to accept defaults without prompts.\n")
		_, _ = fmt.Fprintf(os.Stderr, "\nExamples:\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init\n")
		_, _ = fmt.Fprintf(os.Stderr, "  lockplane init --yes\n")
	}

	if err := fs.Parse(args); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Failed to parse flags: %v\n", err)
		os.Exit(1)
	}

	if *yes {
		result, err := bootstrapSchemaDirectory()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		reportBootstrapResult(os.Stdout, result)
		return
	}

	if err := startInitWizard(); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

func ensureSchemaDir(path string) (bool, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		path = defaultSchemaDir
	}

	info, err := os.Stat(path)
	if err == nil {
		if info.IsDir() {
			return false, nil
		}
		return false, fmt.Errorf("%s exists but is not a directory", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return false, err
	}
	return true, os.MkdirAll(path, 0o755)
}

func bootstrapSchemaDirectory() (*bootstrapResult, error) {
	dirCreated, err := ensureSchemaDir(defaultSchemaDir)
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(defaultSchemaDir, lockplaneConfigFilename)
	if info, err := os.Stat(configPath); err == nil && !info.IsDir() {
		return nil, fmt.Errorf("%s already exists. Edit the existing file or delete it if you want to re-initialize", filepath.ToSlash(configPath))
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	if err := os.WriteFile(configPath, []byte(defaultLockplaneTomlBody), 0o644); err != nil {
		return nil, err
	}

	return &bootstrapResult{
		SchemaDir:        defaultSchemaDir,
		ConfigPath:       configPath,
		SchemaDirCreated: dirCreated,
		ConfigCreated:    true,
	}, nil
}

func reportBootstrapResult(out *os.File, result *bootstrapResult) {
	if result == nil {
		return
	}

	if result.SchemaDirCreated {
		_, _ = fmt.Fprintf(out, "✓ Created %s/\n", filepath.ToSlash(result.SchemaDir))
	} else {
		_, _ = fmt.Fprintf(out, "• Using existing %s/\n", filepath.ToSlash(result.SchemaDir))
	}

	if result.ConfigCreated {
		_, _ = fmt.Fprintf(out, "✓ Wrote %s\n", filepath.ToSlash(result.ConfigPath))
	} else if result.ConfigUpdated {
		_, _ = fmt.Fprintf(out, "✓ Updated %s\n", filepath.ToSlash(result.ConfigPath))
	}

	for _, envFile := range result.EnvFiles {
		_, _ = fmt.Fprintf(out, "✓ Wrote %s\n", envFile)
	}

	if result.GitignoreUpdated {
		_, _ = fmt.Fprintf(out, "✓ Updated .gitignore\n")
	}

	_, _ = fmt.Fprintf(out, "\nNext steps:\n")
	_, _ = fmt.Fprintf(out, "  1. Run: lockplane introspect\n")
	_, _ = fmt.Fprintf(out, "  2. Review: %s\n", filepath.ToSlash(result.ConfigPath))
}

func startInitWizard() error {
	model := newInitWizardModel()
	program := tea.NewProgram(model)
	finalModel, err := program.Run()
	if err != nil {
		return err
	}

	m, ok := finalModel.(*initWizardModel)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if m.err != nil {
		return m.err
	}

	reportBootstrapResult(os.Stdout, m.result)
	return nil
}

func newInitWizardModel() *initWizardModel {
	sp := spinner.New()
	sp.Spinner = spinner.Dot

	m := &initWizardModel{
		state:   StateWelcome,
		spinner: sp,
	}

	m.initInputs()
	return m
}

func (m *initWizardModel) initInputs() {
	m.inputs = make([]textinput.Model, 8)

	// Environment name
	m.inputs[0] = textinput.New()
	m.inputs[0].Placeholder = "local"
	m.inputs[0].Focus()
	m.inputs[0].CharLimit = 50
	m.inputs[0].Width = 30
	m.inputs[0].Prompt = "Environment name: "

	// PostgreSQL host
	m.inputs[1] = textinput.New()
	m.inputs[1].Placeholder = "localhost"
	m.inputs[1].CharLimit = 100
	m.inputs[1].Width = 30
	m.inputs[1].Prompt = "Host: "

	// PostgreSQL port
	m.inputs[2] = textinput.New()
	m.inputs[2].Placeholder = "5432"
	m.inputs[2].CharLimit = 5
	m.inputs[2].Width = 30
	m.inputs[2].Prompt = "Port: "

	// PostgreSQL database
	m.inputs[3] = textinput.New()
	m.inputs[3].Placeholder = "lockplane"
	m.inputs[3].CharLimit = 100
	m.inputs[3].Width = 30
	m.inputs[3].Prompt = "Database: "

	// PostgreSQL user
	m.inputs[4] = textinput.New()
	m.inputs[4].Placeholder = "lockplane"
	m.inputs[4].CharLimit = 100
	m.inputs[4].Width = 30
	m.inputs[4].Prompt = "User: "

	// PostgreSQL password
	m.inputs[5] = textinput.New()
	m.inputs[5].Placeholder = "lockplane"
	m.inputs[5].CharLimit = 100
	m.inputs[5].Width = 30
	m.inputs[5].Prompt = "Password: "
	m.inputs[5].EchoMode = textinput.EchoPassword
	m.inputs[5].EchoCharacter = '*'

	// SQLite file path
	m.inputs[6] = textinput.New()
	m.inputs[6].Placeholder = "schema/lockplane.db"
	m.inputs[6].CharLimit = 200
	m.inputs[6].Width = 40
	m.inputs[6].Prompt = "Database file: "

	// libSQL URL
	m.inputs[7] = textinput.New()
	m.inputs[7].Placeholder = "libsql://[db-name]-[org].turso.io?authToken=[token]"
	m.inputs[7].CharLimit = 300
	m.inputs[7].Width = 50
	m.inputs[7].Prompt = "Database URL: "
}

func (m *initWizardModel) Init() tea.Cmd {
	// Check for existing config
	return m.checkExistingConfig()
}

func (m *initWizardModel) checkExistingConfig() tea.Cmd {
	return func() tea.Msg {
		// Check schema/lockplane.toml first (preferred)
		configPath := filepath.Join(defaultSchemaDir, lockplaneConfigFilename)
		if _, err := os.Stat(configPath); err == nil {
			return existingConfigMsg{Path: configPath}
		}

		// Check ./lockplane.toml (legacy)
		legacyPath := lockplaneConfigFilename
		if _, err := os.Stat(legacyPath); err == nil {
			return existingConfigMsg{Path: legacyPath}
		}

		return noExistingConfigMsg{}
	}
}

type existingConfigMsg struct {
	Path string
}

type noExistingConfigMsg struct{}

type connectionTestMsg struct {
	Success bool
	Error   error
}

type createFilesMsg struct {
	Result *bootstrapResult
	Error  error
}

func (m *initWizardModel) testConnection() tea.Cmd {
	return func() tea.Msg {
		var connStr string
		var dbType string

		switch m.currentEnv.DatabaseType {
		case DBTypePostgres:
			host := m.getInputValue(1, "localhost")
			port := m.getInputValue(2, "5432")
			database := m.getInputValue(3, "lockplane")
			user := m.getInputValue(4, "lockplane")
			password := m.getInputValue(5, "lockplane")
			sslmode := "disable"
			if host != "localhost" && host != "127.0.0.1" {
				sslmode = "require"
			}
			connStr = fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
				user, password, host, port, database, sslmode)
			dbType = "postgres"

		case DBTypeSQLite:
			filePath := m.getInputValue(6, "schema/lockplane.db")
			connStr = filePath
			dbType = "sqlite"

		case DBTypeLibSQL:
			connStr = m.getInputValue(7, "")
			dbType = "libsql"
		}

		// Test connection
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		var db *sql.DB
		var err error

		switch dbType {
		case "postgres":
			db, err = sql.Open("postgres", connStr)
		case "sqlite":
			db, err = sql.Open("sqlite", connStr)
		case "libsql":
			db, err = sql.Open("libsql", connStr)
		}

		if err != nil {
			return connectionTestMsg{Success: false, Error: err}
		}
		defer func() { _ = db.Close() }()

		if err := db.PingContext(ctx); err != nil {
			return connectionTestMsg{Success: false, Error: err}
		}

		return connectionTestMsg{Success: true}
	}
}

func (m *initWizardModel) getInputValue(index int, defaultValue string) string {
	if index >= len(m.inputs) {
		return defaultValue
	}
	value := strings.TrimSpace(m.inputs[index].Value())
	if value == "" {
		return defaultValue
	}
	return value
}

func (m *initWizardModel) collectCurrentEnv() {
	m.currentEnv.Name = m.getInputValue(0, "local")

	switch m.currentEnv.DatabaseType {
	case DBTypePostgres:
		m.currentEnv.Host = m.getInputValue(1, "localhost")
		m.currentEnv.Port = m.getInputValue(2, "5432")
		m.currentEnv.Database = m.getInputValue(3, "lockplane")
		m.currentEnv.User = m.getInputValue(4, "lockplane")
		m.currentEnv.Password = m.getInputValue(5, "lockplane")
		if m.currentEnv.Host == "localhost" || m.currentEnv.Host == "127.0.0.1" {
			m.currentEnv.SSLMode = "disable"
		} else {
			m.currentEnv.SSLMode = "require"
		}
	case DBTypeSQLite:
		m.currentEnv.FilePath = m.getInputValue(6, "schema/lockplane.db")
	case DBTypeLibSQL:
		m.currentEnv.URL = m.getInputValue(7, "")
	}

	m.environments = append(m.environments, m.currentEnv)
}

func (m *initWizardModel) createFiles() tea.Cmd {
	return func() tea.Msg {
		result, err := m.generateFiles()
		return createFilesMsg{Result: result, Error: err}
	}
}

func (m *initWizardModel) generateFiles() (*bootstrapResult, error) {
	// Ensure schema directory exists
	dirCreated, err := ensureSchemaDir(defaultSchemaDir)
	if err != nil {
		return nil, err
	}

	result := &bootstrapResult{
		SchemaDir:        defaultSchemaDir,
		SchemaDirCreated: dirCreated,
		EnvFiles:         []string{},
	}

	// Generate lockplane.toml
	configPath := filepath.Join(defaultSchemaDir, lockplaneConfigFilename)
	configExists := false
	if _, err := os.Stat(configPath); err == nil {
		configExists = true
	}

	tomlContent := m.generateTOMLContent()
	if err := os.WriteFile(configPath, []byte(tomlContent), 0o644); err != nil {
		return nil, err
	}

	result.ConfigPath = configPath
	if configExists {
		result.ConfigUpdated = true
	} else {
		result.ConfigCreated = true
	}

	// Generate .env files
	for _, env := range m.environments {
		envFilePath := fmt.Sprintf(".env.%s", env.Name)
		envContent := m.generateEnvContent(env)
		if err := os.WriteFile(envFilePath, []byte(envContent), 0o600); err != nil {
			return nil, err
		}
		result.EnvFiles = append(result.EnvFiles, envFilePath)
	}

	// Update .gitignore
	if err := m.updateGitignore(); err == nil {
		result.GitignoreUpdated = true
	}

	return result, nil
}

func (m *initWizardModel) generateTOMLContent() string {
	var b strings.Builder

	b.WriteString("# Lockplane Configuration\n")
	b.WriteString("# Generated by: lockplane init\n")
	b.WriteString("#\n")
	b.WriteString("# Config location: Always in schema/ directory for consistency\n")
	b.WriteString("# Credentials: Stored in .env.* files (never in this file)\n\n")

	// Use first environment as default
	if len(m.environments) > 0 {
		b.WriteString(fmt.Sprintf("default_environment = \"%s\"\n\n", m.environments[0].Name))
	}

	// Write environment sections
	for _, env := range m.environments {
		b.WriteString(fmt.Sprintf("[environments.%s]\n", env.Name))

		switch env.DatabaseType {
		case DBTypePostgres:
			b.WriteString("description = \"PostgreSQL database\"\n")
		case DBTypeSQLite:
			b.WriteString("description = \"SQLite database\"\n")
		case DBTypeLibSQL:
			b.WriteString("description = \"libSQL/Turso database\"\n")
		}

		b.WriteString(fmt.Sprintf("# Connection: .env.%s\n", env.Name))

		if env.DatabaseType == DBTypePostgres {
			b.WriteString("# Shadow DB: Auto-configured for safe migrations\n")
		}

		b.WriteString("\n")
	}

	return b.String()
}

func (m *initWizardModel) generateEnvContent(env EnvironmentInput) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# Lockplane Environment: %s\n", env.Name))
	b.WriteString("# Generated by: lockplane init\n")
	b.WriteString("#\n")
	b.WriteString("# ⚠️  DO NOT COMMIT THIS FILE\n")
	b.WriteString("# (Already added to .gitignore automatically)\n\n")

	switch env.DatabaseType {
	case DBTypePostgres:
		b.WriteString(fmt.Sprintf("# PostgreSQL connection (sslmode=%s)\n", env.SSLMode))
		connStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			env.User, env.Password, env.Host, env.Port, env.Database, env.SSLMode)
		b.WriteString(fmt.Sprintf("DATABASE_URL=%s\n\n", connStr))

		b.WriteString("# Shadow database (always configured for PostgreSQL - safe migrations)\n")
		shadowPort := "5433"
		if env.Port != "5432" {
			// If custom port, use port+1 for shadow
			shadowPort = env.Port
		}
		shadowConnStr := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s_shadow?sslmode=%s",
			env.User, env.Password, env.Host, shadowPort, env.Database, env.SSLMode)
		b.WriteString(fmt.Sprintf("SHADOW_DATABASE_URL=%s\n", shadowConnStr))

	case DBTypeSQLite:
		b.WriteString("# SQLite connection (file-based)\n")
		b.WriteString(fmt.Sprintf("DATABASE_URL=sqlite://%s\n\n", env.FilePath))
		b.WriteString("# Shadow database disabled for SQLite (avoids file clutter)\n")
		b.WriteString("# Migrations will run without shadow validation\n")

	case DBTypeLibSQL:
		b.WriteString("# libSQL/Turso connection\n")
		b.WriteString(fmt.Sprintf("DATABASE_URL=%s\n\n", env.URL))
		b.WriteString("# Shadow database not supported by Turso\n")
		b.WriteString("# Migrations run directly (use with caution in production)\n")
	}

	return b.String()
}

func (m *initWizardModel) updateGitignore() error {
	gitignorePath := ".gitignore"

	// Read existing content
	content, err := os.ReadFile(gitignorePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}

	existingContent := string(content)

	// Check if already contains .env.* pattern
	if strings.Contains(existingContent, ".env.*") {
		return nil // Already present
	}

	// Append our pattern
	var b strings.Builder
	if len(existingContent) > 0 {
		b.WriteString(existingContent)
		if !strings.HasSuffix(existingContent, "\n") {
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString("# Lockplane environment files (added by lockplane init)\n")
	b.WriteString("# DO NOT remove - contains database credentials\n")
	b.WriteString(".env.*\n")
	b.WriteString("!.env.*.example\n")

	return os.WriteFile(gitignorePath, []byte(b.String()), 0o644)
}

func (m *initWizardModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case spinner.TickMsg:
		if m.creating || m.testingConnection {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case existingConfigMsg:
		m.existingConfigPath = msg.Path
		m.state = StateCheckExisting
		return m, nil

	case noExistingConfigMsg:
		m.state = StateDatabaseType
		return m, nil

	case connectionTestMsg:
		m.testingConnection = false
		if msg.Success {
			m.testResult = "✓ Connection successful"
			m.testError = nil
			m.collectCurrentEnv()
			m.state = StateAddAnother
		} else {
			m.testError = msg.Error
			m.testRetryCount++
			if m.testRetryCount >= 3 {
				m.testResult = fmt.Sprintf("✗ Connection failed after 3 attempts: %v\nCheck if database is running and credentials are correct.", msg.Error)
			} else {
				m.testResult = fmt.Sprintf("✗ Connection failed: %v", msg.Error)
			}
		}
		return m, nil

	case createFilesMsg:
		m.creating = false
		if msg.Error != nil {
			m.err = msg.Error
			return m, nil
		}
		m.result = msg.Result
		m.done = true
		m.shouldQuit = true
		return m, tea.Quit
	}

	// Update focused input
	if m.state == StateConnectionDetails {
		cmd := m.updateInputs(msg)
		return m, cmd
	}

	return m, nil
}

func (m *initWizardModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.Type {
	case tea.KeyCtrlC, tea.KeyEsc:
		m.shouldQuit = true
		return m, tea.Quit

	case tea.KeyEnter:
		return m.handleEnter()

	case tea.KeyUp, tea.KeyShiftTab:
		if m.state == StateConnectionDetails {
			m.focusIndex--
			if m.focusIndex < 0 {
				m.focusIndex = m.getInputCount() - 1
			}
			m.updateInputFocus()
		} else if m.state == StateDatabaseType {
			m.dbTypeChoice--
			if m.dbTypeChoice < 0 {
				m.dbTypeChoice = 2
			}
		}

	case tea.KeyDown, tea.KeyTab:
		if m.state == StateConnectionDetails {
			m.focusIndex++
			if m.focusIndex >= m.getInputCount() {
				m.focusIndex = 0
			}
			m.updateInputFocus()
		} else if m.state == StateDatabaseType {
			m.dbTypeChoice++
			if m.dbTypeChoice > 2 {
				m.dbTypeChoice = 0
			}
		}
	}

	return m, nil
}

func (m *initWizardModel) handleEnter() (tea.Model, tea.Cmd) {
	switch m.state {
	case StateWelcome:
		m.state = StateCheckExisting
		return m, m.checkExistingConfig()

	case StateCheckExisting:
		// User wants to add new environment
		m.addingToExisting = true
		m.state = StateDatabaseType
		return m, nil

	case StateDatabaseType:
		// Set database type based on choice
		switch m.dbTypeChoice {
		case 0:
			m.currentEnv.DatabaseType = DBTypePostgres
		case 1:
			m.currentEnv.DatabaseType = DBTypeSQLite
		case 2:
			m.currentEnv.DatabaseType = DBTypeLibSQL
		}
		m.state = StateConnectionDetails
		m.focusIndex = 0
		m.updateInputFocus()
		return m, nil

	case StateConnectionDetails:
		// Validate inputs
		if err := m.validateInputs(); err != nil {
			m.testResult = fmt.Sprintf("✗ %v", err)
			return m, nil
		}
		// Test connection
		m.state = StateTestingConnection
		m.testingConnection = true
		m.testResult = ""
		m.testError = nil
		return m, tea.Batch(m.testConnection(), m.spinner.Tick)

	case StateTestingConnection:
		if m.testError != nil {
			// Retry or edit
			m.state = StateConnectionDetails
			m.testRetryCount = 0
			return m, nil
		}
		return m, nil

	case StateAddAnother:
		// For now, just create files
		// TODO: Implement adding another environment
		m.state = StateCreating
		m.creating = true
		return m, tea.Batch(m.createFiles(), m.spinner.Tick)

	case StateCreating:
		return m, nil

	case StateDone:
		return m, tea.Quit
	}

	return m, nil
}

func (m *initWizardModel) getInputCount() int {
	switch m.currentEnv.DatabaseType {
	case DBTypePostgres:
		return 6 // name, host, port, db, user, pass
	case DBTypeSQLite:
		return 2 // name, filepath
	case DBTypeLibSQL:
		return 2 // name, url
	default:
		return 1
	}
}

func (m *initWizardModel) updateInputFocus() {
	for i := range m.inputs {
		if i == m.focusIndex {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *initWizardModel) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))
	for i := range m.inputs {
		m.inputs[i], cmds[i] = m.inputs[i].Update(msg)
	}
	return tea.Batch(cmds...)
}

func (m *initWizardModel) validateInputs() error {
	envName := m.getInputValue(0, "local")
	if !regexp.MustCompile(`^[a-zA-Z0-9_]+$`).MatchString(envName) {
		return fmt.Errorf("environment name must contain only alphanumeric characters and underscores")
	}

	switch m.currentEnv.DatabaseType {
	case DBTypePostgres:
		port := m.getInputValue(2, "5432")
		if !regexp.MustCompile(`^\d+$`).MatchString(port) {
			return fmt.Errorf("port must be a number")
		}
	case DBTypeLibSQL:
		url := m.getInputValue(7, "")
		if !strings.HasPrefix(url, "libsql://") {
			return fmt.Errorf("libSQL URL must start with libsql://")
		}
	}

	return nil
}

func (m *initWizardModel) View() string {
	var b strings.Builder

	b.WriteString("\n  Lockplane Init Wizard\n")
	b.WriteString("  " + strings.Repeat("─", 60) + "\n\n")

	switch m.state {
	case StateWelcome:
		b.WriteString("  This wizard will set up your Lockplane project.\n\n")
		b.WriteString("  Press Enter to continue or Esc to cancel.\n")

	case StateCheckExisting:
		if m.existingConfigPath != "" {
			b.WriteString(fmt.Sprintf("  Found existing config at: %s\n\n", m.existingConfigPath))
			b.WriteString("  Add new environment? (Press Enter for Yes, Esc to cancel)\n")
		}

	case StateDatabaseType:
		b.WriteString("  What database are you using?\n\n")
		choices := []string{
			"PostgreSQL (recommended for production)",
			"SQLite (simple, file-based)",
			"libSQL/Turso (edge database)",
		}
		for i, choice := range choices {
			if i == m.dbTypeChoice {
				b.WriteString(fmt.Sprintf("  > %d. %s\n", i+1, choice))
			} else {
				b.WriteString(fmt.Sprintf("    %d. %s\n", i+1, choice))
			}
		}
		b.WriteString("\n  Use ↑/↓ to select, Enter to continue\n")

	case StateConnectionDetails:
		b.WriteString("  Enter connection details:\n\n")
		b.WriteString(fmt.Sprintf("  %s\n", m.inputs[0].View()))

		switch m.currentEnv.DatabaseType {
		case DBTypePostgres:
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[1].View()))
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[2].View()))
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[3].View()))
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[4].View()))
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[5].View()))
		case DBTypeSQLite:
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[6].View()))
		case DBTypeLibSQL:
			b.WriteString(fmt.Sprintf("  %s\n", m.inputs[7].View()))
		}

		if m.testResult != "" {
			b.WriteString(fmt.Sprintf("\n  %s\n", m.testResult))
		}

		b.WriteString("\n  Tab/Shift+Tab to navigate, Enter to test connection\n")

	case StateTestingConnection:
		b.WriteString(fmt.Sprintf("  %s Testing connection...\n", m.spinner.View()))

	case StateAddAnother:
		b.WriteString(fmt.Sprintf("  %s\n\n", m.testResult))
		b.WriteString("  Press Enter to create configuration\n")
		b.WriteString("  (Adding another environment will be supported in a future update)\n")

	case StateCreating:
		b.WriteString(fmt.Sprintf("  %s Creating configuration files...\n", m.spinner.View()))

	case StateDone:
		b.WriteString("  ✓ Configuration created successfully!\n")
	}

	if m.err != nil {
		b.WriteString(fmt.Sprintf("\n  Error: %v\n", m.err))
	}

	b.WriteString("\n")
	return b.String()
}
