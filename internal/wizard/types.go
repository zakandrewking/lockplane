package wizard

import (
	"github.com/charmbracelet/bubbles/textinput"
)

// WizardState represents the current step in the wizard flow
type WizardState int

const (
	StateWelcome WizardState = iota
	StateCheckExisting
	StateDatabaseType
	StateConnectionDetails
	StateTestConnection
	StateAddAnother
	StateSummary
	StateCreating
	StateDone
	StateError
)

// WizardModel holds the state for the Bubble Tea wizard
type WizardModel struct {
	state WizardState

	// Existing config detection
	existingConfigPath string
	existingEnvNames   []string

	// Current environment being configured
	currentEnv      EnvironmentInput
	environments    []EnvironmentInput // New environments being added
	allEnvironments []string           // All environment names (existing + new)

	// Connection testing
	testingConnection    bool
	connectionTestResult string
	connectionError      error
	retryChoice          int // 0=retry, 1=edit, 2=quit

	// Add another environment choice
	addAnotherChoice int // 0=add another, 1=finish and save

	// Input fields (using bubbletea textinput)
	inputs     []textinput.Model
	focusIndex int

	// Database type selection
	dbTypeIndex int

	// Validation
	errors map[string]string

	// Final output
	result *InitResult
	err    error

	// Terminal dimensions
	width  int
	height int
}

// EnvironmentInput holds user input for a single environment
type EnvironmentInput struct {
	Name         string
	Description  string
	DatabaseType string // "postgres", "sqlite", "libsql"

	// PostgreSQL fields
	Host         string
	Port         string
	Database     string
	User         string
	Password     string
	SSLMode      string
	ShadowDBPort string

	// SQLite fields
	FilePath string

	// libSQL fields
	URL       string
	AuthToken string

	// Common
	SchemaPath string
}

// InitResult contains the outcome of running the wizard
type InitResult struct {
	ConfigPath        string
	ConfigCreated     bool
	ConfigUpdated     bool
	EnvFiles          []string
	SchemaDir         string
	SchemaDirCreated  bool
	GitignoreUpdated  bool
	EnvExampleCreated bool
	EnvExampleUpdated bool
}

// DatabaseType represents a database option
type DatabaseType struct {
	ID          string
	DisplayName string
	Description string
	Icon        string
}

// Available database types
var DatabaseTypes = []DatabaseType{
	{
		ID:          "postgres",
		DisplayName: "PostgreSQL",
		Description: "recommended for production",
		Icon:        "🐘",
	},
	{
		ID:          "sqlite",
		DisplayName: "SQLite",
		Description: "simple, file-based",
		Icon:        "📁",
	},
	{
		ID:          "libsql",
		DisplayName: "libSQL/Turso",
		Description: "edge database",
		Icon:        "🌐",
	},
}
