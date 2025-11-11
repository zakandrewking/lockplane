package state

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// StateFile is the filename for state tracking
const StateFile = ".lockplane-state.json"

// State tracks multi-phase migration progress
// Stored in .lockplane-state.json in the project root (git-ignored)
type State struct {
	Version         string           `json:"version"` // State file format version
	ActiveMigration *ActiveMigration `json:"active_migration,omitempty"`
}

// ActiveMigration tracks the currently running multi-phase migration
type ActiveMigration struct {
	ID              string    `json:"id"`               // Unique identifier for this migration
	Operation       string    `json:"operation"`        // Operation type: rename_column, drop_column, etc.
	Pattern         string    `json:"pattern"`          // Pattern: expand_contract, deprecation, etc.
	Table           string    `json:"table"`            // Table being migrated
	Column          string    `json:"column,omitempty"` // Column being migrated (if applicable)
	TotalPhases     int       `json:"total_phases"`     // Total number of phases
	CurrentPhase    int       `json:"current_phase"`    // Current phase number (0 = not started)
	PhasesCompleted []int     `json:"phases_completed"` // List of completed phase numbers
	StartedAt       time.Time `json:"started_at"`       // When migration started
	LastUpdated     time.Time `json:"last_updated"`     // Last state update
	PlanPath        string    `json:"plan_path"`        // Path to the multi-phase plan JSON file
}

// Load reads state from .lockplane-state.json
// Returns empty state if file doesn't exist
func Load() (*State, error) {
	data, err := os.ReadFile(StateFile)
	if os.IsNotExist(err) {
		// No state file yet - return empty state
		return &State{
			Version: "1",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// Save writes state to .lockplane-state.json
func (s *State) Save() error {
	// Ensure directory exists
	dir := filepath.Dir(StateFile)
	if dir != "." && dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	// Marshal with indentation for readability
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	// Write atomically (write to temp file, then rename)
	tempFile := StateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	if err := os.Rename(tempFile, StateFile); err != nil {
		return fmt.Errorf("failed to save state file: %w", err)
	}

	return nil
}

// StartMigration initializes a new active migration
func (s *State) StartMigration(id, operation, pattern, table, column string, totalPhases int, planPath string) error {
	if s.ActiveMigration != nil {
		return fmt.Errorf("cannot start migration: another migration is already in progress (%s)", s.ActiveMigration.ID)
	}

	s.ActiveMigration = &ActiveMigration{
		ID:              id,
		Operation:       operation,
		Pattern:         pattern,
		Table:           table,
		Column:          column,
		TotalPhases:     totalPhases,
		CurrentPhase:    0, // Not started yet
		PhasesCompleted: []int{},
		StartedAt:       time.Now(),
		LastUpdated:     time.Now(),
		PlanPath:        planPath,
	}

	return s.Save()
}

// CompletePhase marks a phase as completed and advances to next phase
func (s *State) CompletePhase(phaseNumber int) error {
	if s.ActiveMigration == nil {
		return fmt.Errorf("no active migration")
	}

	// Verify phase is the current or next phase
	if phaseNumber != s.ActiveMigration.CurrentPhase && phaseNumber != s.ActiveMigration.CurrentPhase+1 {
		return fmt.Errorf("cannot complete phase %d: current phase is %d", phaseNumber, s.ActiveMigration.CurrentPhase)
	}

	// Add to completed list if not already there
	alreadyCompleted := false
	for _, completed := range s.ActiveMigration.PhasesCompleted {
		if completed == phaseNumber {
			alreadyCompleted = true
			break
		}
	}

	if !alreadyCompleted {
		s.ActiveMigration.PhasesCompleted = append(s.ActiveMigration.PhasesCompleted, phaseNumber)
	}

	// Update current phase
	s.ActiveMigration.CurrentPhase = phaseNumber
	s.ActiveMigration.LastUpdated = time.Now()

	// If all phases complete, clear active migration
	if phaseNumber >= s.ActiveMigration.TotalPhases {
		s.ActiveMigration = nil
	}

	return s.Save()
}

// CanExecutePhase checks if it's safe to execute a given phase
func (s *State) CanExecutePhase(phaseNumber int) error {
	if s.ActiveMigration == nil {
		// No active migration - can start phase 1
		if phaseNumber == 1 {
			return nil
		}
		return fmt.Errorf("no active migration: start with phase 1")
	}

	// Check if phase already completed
	for _, completed := range s.ActiveMigration.PhasesCompleted {
		if completed == phaseNumber {
			return fmt.Errorf("phase %d already completed", phaseNumber)
		}
	}

	// Can execute current phase or next phase
	if phaseNumber == s.ActiveMigration.CurrentPhase || phaseNumber == s.ActiveMigration.CurrentPhase+1 {
		return nil
	}

	if phaseNumber < s.ActiveMigration.CurrentPhase {
		return fmt.Errorf("phase %d already completed (current phase: %d)", phaseNumber, s.ActiveMigration.CurrentPhase)
	}

	return fmt.Errorf("cannot skip to phase %d: must complete phase %d first", phaseNumber, s.ActiveMigration.CurrentPhase+1)
}

// ClearActiveMigration removes the active migration (use after completion or cancellation)
func (s *State) ClearActiveMigration() error {
	s.ActiveMigration = nil
	return s.Save()
}

// GetNextPhase returns the next phase number to execute, or 0 if complete
func (s *State) GetNextPhase() int {
	if s.ActiveMigration == nil {
		return 1 // Start with phase 1
	}

	if s.ActiveMigration.CurrentPhase >= s.ActiveMigration.TotalPhases {
		return 0 // All complete
	}

	return s.ActiveMigration.CurrentPhase + 1
}
