package wizard

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestWizardModel_Update(t *testing.T) {
	tests := []struct {
		name          string
		initialState  WizardState
		msg           tea.Msg
		expectedState WizardState
		expectCmd     bool
		description   string
	}{
		{
			name:          "enter key at start triggers file creation",
			initialState:  StateStart,
			msg:           tea.KeyMsg{Type: tea.KeyEnter},
			expectedState: StateStart,
			expectCmd:     true,
			description:   "pressing enter in StateStart should return writeDefaultInit command",
		},
		{
			name:          "successful file creation",
			initialState:  StateStart,
			msg:           fileCreationResultMsg{err: nil},
			expectedState: StateCreateSucceeded,
			expectCmd:     true,
			description:   "nil error should transition to StateCreateSucceeded",
		},
		{
			name:          "failed file creation",
			initialState:  StateStart,
			msg:           fileCreationResultMsg{err: fmt.Errorf("file already exists")},
			expectedState: StateCreateFailed,
			expectCmd:     true,
			description:   "error should transition to StateCreateFailed",
		},
		{
			name:          "quit on other keys",
			initialState:  StateStart,
			msg:           tea.KeyMsg{Type: tea.KeyCtrlC},
			expectedState: StateStart,
			expectCmd:     true,
			description:   "any key other than enter should quit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wizardModel{state: tt.initialState}
			newModel, cmd := m.Update(tt.msg)

			// Check state transition
			if newModel.(wizardModel).state != tt.expectedState {
				t.Errorf("expected state %v, got %v", tt.expectedState, newModel.(wizardModel).state)
			}

			// Check if command was returned
			if tt.expectCmd && cmd == nil {
				t.Error("expected command to be returned, got nil")
			}
		})
	}
}

func TestWizardModel_View(t *testing.T) {
	tests := []struct {
		name     string
		model    wizardModel
		contains string
	}{
		{
			name:     "start state shows prompt",
			model:    wizardModel{state: StateStart},
			contains: "Press Enter",
		},
		{
			name:     "success state shows result",
			model:    wizardModel{state: StateCreateSucceeded},
			contains: "Created lockplane.toml and schema/ directory",
		},
		{
			name:     "failed state shows error",
			model:    wizardModel{state: StateCreateFailed, finalErr: fmt.Errorf("file already exists")},
			contains: "Could not initialize",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.model.View()

			if view == "" {
				t.Error("expected non-empty view")
			}

			if tt.contains != "" && !contains(view, tt.contains) {
				t.Errorf("expected view to contain %q, got %q", tt.contains, view)
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Integration tests for edge cases
func TestWriteDefaultInit_EdgeCases(t *testing.T) {
	tests := []struct {
		name              string
		setupFunc         func(dir string) error
		force             bool
		expectError       bool
		expectConfigFile  bool
		expectSchemaDir   bool
		expectExampleFile bool
		description       string
	}{
		{
			name: "lockplane.toml exists, schema/ does not",
			setupFunc: func(dir string) error {
				// Create lockplane.toml
				return os.WriteFile(filepath.Join(dir, "lockplane.toml"), []byte("existing"), 0600)
			},
			force:             false,
			expectError:       true,
			expectConfigFile:  true,
			expectSchemaDir:   false,
			expectExampleFile: false,
			description:       "should error when config exists and force=false",
		},
		{
			name: "lockplane.toml does not exist, schema/ empty",
			setupFunc: func(dir string) error {
				// Create empty schema directory
				return os.Mkdir(filepath.Join(dir, "schema"), 0755)
			},
			force:             false,
			expectError:       false,
			expectConfigFile:  true,
			expectSchemaDir:   true,
			expectExampleFile: true,
			description:       "should create config and example file in existing empty schema dir",
		},
		{
			name: "lockplane.toml exists, schema/ exists and is empty",
			setupFunc: func(dir string) error {
				// Create lockplane.toml and empty schema directory
				if err := os.WriteFile(filepath.Join(dir, "lockplane.toml"), []byte("existing"), 0600); err != nil {
					return err
				}
				return os.Mkdir(filepath.Join(dir, "schema"), 0755)
			},
			force:             true,
			expectError:       false,
			expectConfigFile:  true,
			expectSchemaDir:   true,
			expectExampleFile: true,
			description:       "should overwrite config with force=true and add example to empty schema",
		},
		{
			name: "schema/ exists and is not empty",
			setupFunc: func(dir string) error {
				// Create schema directory with a file
				schemaDir := filepath.Join(dir, "schema")
				if err := os.Mkdir(schemaDir, 0755); err != nil {
					return err
				}
				return os.WriteFile(filepath.Join(schemaDir, "001_existing.sql"), []byte("SELECT 1;"), 0644)
			},
			force:             false,
			expectError:       false,
			expectConfigFile:  true,
			expectSchemaDir:   true,
			expectExampleFile: false,
			description:       "should not create example file when schema dir has files",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir := t.TempDir()

			// Change to temp directory
			originalDir, err := os.Getwd()
			if err != nil {
				t.Fatalf("failed to get working directory: %v", err)
			}
			defer func() {
				if err := os.Chdir(originalDir); err != nil {
					t.Errorf("failed to restore working directory: %v", err)
				}
			}()

			if err := os.Chdir(tmpDir); err != nil {
				t.Fatalf("failed to change to temp directory: %v", err)
			}

			// Setup test scenario
			if tt.setupFunc != nil {
				if err := tt.setupFunc(tmpDir); err != nil {
					t.Fatalf("setup failed: %v", err)
				}
			}

			// Execute the command
			cmd := writeDefaultInit(tt.force)
			msg := cmd()

			// Check for errors
			resultMsg, ok := msg.(fileCreationResultMsg)
			if !ok {
				t.Fatal("expected fileCreationResultMsg")
			}

			if tt.expectError && resultMsg.err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && resultMsg.err != nil {
				t.Errorf("unexpected error: %v", resultMsg.err)
			}

			// Verify lockplane.toml
			configPath := filepath.Join(tmpDir, "lockplane.toml")
			_, err = os.Stat(configPath)
			configExists := err == nil

			if tt.expectConfigFile && !configExists {
				t.Error("expected lockplane.toml to exist")
			}

			// Verify schema directory
			schemaPath := filepath.Join(tmpDir, "schema")
			_, err = os.Stat(schemaPath)
			schemaExists := err == nil

			if tt.expectSchemaDir && !schemaExists {
				t.Error("expected schema/ directory to exist")
			}

			// Verify example.lp.sql
			examplePath := filepath.Join(tmpDir, "schema", "example.lp.sql")
			_, err = os.Stat(examplePath)
			exampleExists := err == nil

			if tt.expectExampleFile && !exampleExists {
				t.Error("expected schema/example.lp.sql to exist")
			}
			if !tt.expectExampleFile && exampleExists {
				t.Error("expected schema/example.lp.sql to NOT exist")
			}
		})
	}
}
