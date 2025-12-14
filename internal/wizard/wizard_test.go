package wizard

import (
	"fmt"
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
		state    WizardState
		contains string
	}{
		{
			name:     "start state shows prompt",
			state:    StateStart,
			contains: "Press Enter",
		},
		{
			name:     "creating state shows loading",
			state:    StateCreating,
			contains: "Creating file",
		},
		{
			name:     "success state shows result",
			state:    StateCreateSucceeded,
			contains: "File created",
		},
		{
			name:     "failed state shows error",
			state:    StateCreateFailed,
			contains: "Could not create",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := wizardModel{state: tt.state}
			view := m.View()

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
