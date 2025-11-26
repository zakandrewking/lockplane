package wizard

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNew(t *testing.T) {
	m := New()

	if m.state != StateWelcome {
		t.Errorf("expected initial state to be StateWelcome, got %v", m.state)
	}

	if len(m.environments) != 0 {
		t.Errorf("expected empty environments slice, got %d", len(m.environments))
	}

	if len(m.errors) != 0 {
		t.Errorf("expected empty errors map, got %d", len(m.errors))
	}
}

func TestHandleEnterWelcome(t *testing.T) {
	m := New()
	m.state = StateWelcome

	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)

	if m.state != StateDatabaseType {
		t.Errorf("expected state to be StateDatabaseType after Enter on Welcome, got %v", m.state)
	}
}

func TestHandleEnterDatabaseType(t *testing.T) {
	m := New()
	m.state = StateDatabaseType
	m.dbTypeIndex = 0 // PostgreSQL

	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)

	// For Postgres, should go to input method selection first
	if m.state != StatePostgresInputMethod {
		t.Errorf("expected state to be StatePostgresInputMethod after selecting Postgres, got %v", m.state)
	}

	if m.currentEnv.DatabaseType != "postgres" {
		t.Errorf("expected database type to be 'postgres', got %s", m.currentEnv.DatabaseType)
	}

	// Now select input method and proceed to connection details
	newModel, _ = m.handleEnter()
	m = *newModel.(*WizardModel)

	if m.state != StateConnectionDetails {
		t.Errorf("expected state to be StateConnectionDetails after selecting input method, got %v", m.state)
	}

	if len(m.inputs) == 0 {
		t.Error("expected inputs to be initialized after selecting input method")
	}
}

func TestHandleEnterDatabaseTypeSQLite(t *testing.T) {
	m := New()
	m.state = StateDatabaseType
	m.dbTypeIndex = 1 // SQLite

	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)

	// After shadow info, go to connection details
	if m.state != StateConnectionDetails {
		t.Errorf("expected state to be StateConnectionDetails after selecting SQLite, got %v", m.state)
	}

	if m.currentEnv.DatabaseType != "sqlite" {
		t.Errorf("expected database type to be 'sqlite', got %s", m.currentEnv.DatabaseType)
	}

	if len(m.inputs) == 0 {
		t.Error("expected inputs to be initialized after selecting database type")
	}
}

func TestCollectInputValuesClearsErrors(t *testing.T) {
	m := New()
	m.state = StateConnectionDetails
	m.currentEnv.DatabaseType = "sqlite"
	m.initializeInputs()
	m.inputs[0].SetValue("local")
	m.inputs[1].SetValue("./schema/local.db")
	m.errors["name"] = "previous error"

	if err := m.collectInputValues(); err != nil {
		t.Fatalf("expected collectInputValues to succeed: %v", err)
	}
	if len(m.errors) != 0 {
		t.Fatalf("expected errors map to be cleared, got %v", m.errors)
	}
}

func TestHandleUpDown(t *testing.T) {
	m := New()
	m.state = StateDatabaseType
	m.dbTypeIndex = 0

	// Test down
	newModel, _ := m.handleDown()
	m = *newModel.(*WizardModel)
	if m.dbTypeIndex != 1 {
		t.Errorf("expected dbTypeIndex to be 1 after down, got %d", m.dbTypeIndex)
	}

	// Test up
	newModel, _ = m.handleUp()
	m = *newModel.(*WizardModel)
	if m.dbTypeIndex != 0 {
		t.Errorf("expected dbTypeIndex to be 0 after up, got %d", m.dbTypeIndex)
	}

	// Test up at boundary (should stay at 0)
	newModel, _ = m.handleUp()
	m = *newModel.(*WizardModel)
	if m.dbTypeIndex != 0 {
		t.Errorf("expected dbTypeIndex to stay at 0 at boundary, got %d", m.dbTypeIndex)
	}
}

func TestConnectionRetryChoices(t *testing.T) {
	m := New()
	m.state = StateTestConnection
	m.connectionTestResult = "failed"
	m.retryChoice = 0

	// Test down navigation
	newModel, _ := m.handleDown()
	m = *newModel.(*WizardModel)
	if m.retryChoice != 1 {
		t.Errorf("expected retryChoice to be 1, got %d", m.retryChoice)
	}

	// Test up navigation
	newModel, _ = m.handleUp()
	m = *newModel.(*WizardModel)
	if m.retryChoice != 0 {
		t.Errorf("expected retryChoice to be 0, got %d", m.retryChoice)
	}
}

func TestHandleEnterTestConnectionSuccess(t *testing.T) {
	m := New()
	m.state = StateTestConnection
	m.connectionTestResult = "success"
	m.currentEnv.Name = "test"
	m.currentEnv.DatabaseType = "postgres"

	// First press Enter - should go to shadow preview
	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)

	if m.state != StateShadowPreview {
		t.Errorf("expected state to be StateShadowPreview after successful connection, got %v", m.state)
	}

	// Environment should not be saved yet (saved after preview)
	if len(m.environments) != 0 {
		t.Errorf("expected 0 environments (not saved yet), got %d", len(m.environments))
	}

	// Second press Enter - should save and go to StateAddAnother
	newModel, _ = m.handleEnter()
	m = *newModel.(*WizardModel)

	if m.state != StateAddAnother {
		t.Errorf("expected state to be StateAddAnother after shadow preview, got %v", m.state)
	}

	if len(m.environments) != 1 {
		t.Errorf("expected 1 environment to be saved, got %d", len(m.environments))
	}

	if m.environments[0].Name != "test" {
		t.Errorf("expected environment name to be 'test', got %s", m.environments[0].Name)
	}
}

func TestHandleEnterTestConnectionFailedRetry(t *testing.T) {
	m := New()
	m.state = StateTestConnection
	m.connectionTestResult = "failed"
	m.retryChoice = 0 // Retry

	newModel, cmd := m.handleEnter()
	m = *newModel.(*WizardModel)

	if m.connectionTestResult != "" {
		t.Errorf("expected connectionTestResult to be reset for retry, got %s", m.connectionTestResult)
	}

	if !m.testingConnection {
		t.Error("expected testingConnection to be true when retrying")
	}

	if cmd == nil {
		t.Error("expected a command to be returned for testing connection")
	}
}

func TestHandleEnterTestConnectionFailedEdit(t *testing.T) {
	m := New()
	m.state = StateTestConnection
	m.connectionTestResult = "failed"
	m.retryChoice = 1 // Edit

	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)

	if m.state != StateConnectionDetails {
		t.Errorf("expected state to be StateConnectionDetails when editing, got %v", m.state)
	}

	if m.connectionTestResult != "" {
		t.Errorf("expected connectionTestResult to be reset, got %s", m.connectionTestResult)
	}
}

func TestHandleEnterTestConnectionFailedQuit(t *testing.T) {
	m := New()
	m.state = StateTestConnection
	m.connectionTestResult = "failed"
	m.retryChoice = 2 // Quit

	_, cmd := m.handleEnter()

	if cmd == nil {
		t.Error("expected quit command to be returned")
	}

	// Check if it's a Quit command by running it
	msg := cmd()
	if _, ok := msg.(tea.QuitMsg); !ok {
		t.Error("expected QuitMsg from quit command")
	}
}

func TestAddAnotherSaveAndFinishSkipsSummary(t *testing.T) {
	m := New()
	m.state = StateAddAnother
	m.addAnotherChoice = 1
	m.environments = []EnvironmentInput{{Name: "local", DatabaseType: "postgres"}}

	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)
	if m.state != StateCreating {
		t.Errorf("expected StateCreating when finishing, got %v", m.state)
	}
}

func TestExistingConfigDetection(t *testing.T) {
	m := New()

	// Test handling of existing config message
	msg := existingConfigMsg{
		path:     "lockplane.toml",
		envNames: []string{"local", "staging"},
	}

	newModel, _ := m.Update(msg)
	m = *newModel.(*WizardModel)

	if m.state != StateCheckExisting {
		t.Errorf("expected state to be StateCheckExisting, got %v", m.state)
	}

	if m.existingConfigPath != "lockplane.toml" {
		t.Errorf("expected existingConfigPath to be set, got %s", m.existingConfigPath)
	}

	if len(m.existingEnvNames) != 2 {
		t.Errorf("expected 2 existing environments, got %d", len(m.existingEnvNames))
	}
}

func TestDuplicateEnvironmentNameReturnsToCheckExisting(t *testing.T) {
	m := New()
	m.state = StateConnectionDetails
	m.currentEnv.DatabaseType = "sqlite"
	m.existingEnvNames = []string{"local"}
	m.initializeInputs()
	m.inputs[0].SetValue("local")
	m.inputs[1].SetValue("./schema/lockplane.db")

	err := m.collectInputValues()
	if err == nil {
		t.Fatalf("expected error for duplicate environment name")
	}
	if m.state != StateCheckExisting {
		t.Fatalf("expected to return to StateCheckExisting, got %v", m.state)
	}
	if !strings.Contains(m.noticeMessage, "already exists") {
		t.Fatalf("expected notice message about duplicate, got %q", m.noticeMessage)
	}
}

func TestSelectExistingEnvironmentFlow(t *testing.T) {
	m := New()
	m.state = StateCheckExisting
	m.existingEnvNames = []string{"local", "staging"}
	m.existingChoice = 1

	newModel, _ := m.handleEnter()
	m = *newModel.(*WizardModel)
	if m.state != StateSelectExisting {
		t.Fatalf("expected StateSelectExisting, got %v", m.state)
	}

	newModel, _ = m.handleEnter()
	m = *newModel.(*WizardModel)
	if m.state != StateDatabaseType {
		t.Fatalf("expected StateDatabaseType after selecting existing env, got %v", m.state)
	}
	if !m.editingExisting || m.editingEnvName == "" {
		t.Fatalf("expected editingExisting flag to be set")
	}
}

func TestNoExistingConfig(t *testing.T) {
	m := New()

	// Test handling of no existing config
	msg := existingConfigMsg{}

	newModel, _ := m.Update(msg)
	m = *newModel.(*WizardModel)

	if m.state != StateWelcome {
		t.Errorf("expected state to be StateWelcome when no config exists, got %v", m.state)
	}
}
