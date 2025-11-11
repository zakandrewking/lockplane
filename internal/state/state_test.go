package state

import (
	"os"
	"testing"
)

func TestStateLoadAndSave(t *testing.T) {
	// Clean up before and after
	defer func() { _ = os.Remove(StateFile) }()

	// Load non-existent state
	state, err := Load()
	if err != nil {
		t.Fatalf("Failed to load empty state: %v", err)
	}

	if state.Version != "1" {
		t.Errorf("Expected version 1, got %s", state.Version)
	}

	if state.ActiveMigration != nil {
		t.Error("Expected no active migration")
	}

	// Save state
	if err := state.Save(); err != nil {
		t.Fatalf("Failed to save state: %v", err)
	}

	// Load again
	state2, err := Load()
	if err != nil {
		t.Fatalf("Failed to load saved state: %v", err)
	}

	if state2.Version != state.Version {
		t.Errorf("Version mismatch after reload")
	}
}

func TestStartMigration(t *testing.T) {
	defer func() { _ = os.Remove(StateFile) }()

	state, err := Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Start migration
	err = state.StartMigration("test-migration", "rename_column", "expand_contract", "users", "email", 3, "test-plan.json")
	if err != nil {
		t.Fatalf("Failed to start migration: %v", err)
	}

	if state.ActiveMigration == nil {
		t.Fatal("Expected active migration")
	}

	if state.ActiveMigration.ID != "test-migration" {
		t.Errorf("Expected ID test-migration, got %s", state.ActiveMigration.ID)
	}

	if state.ActiveMigration.CurrentPhase != 0 {
		t.Errorf("Expected current phase 0, got %d", state.ActiveMigration.CurrentPhase)
	}

	// Try to start another migration - should fail
	err = state.StartMigration("test-migration-2", "drop_column", "deprecation", "users", "old_col", 4, "test-plan-2.json")
	if err == nil {
		t.Error("Expected error when starting second migration")
	}
}

func TestCompletePhase(t *testing.T) {
	defer func() { _ = os.Remove(StateFile) }()

	state, err := Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// Start migration
	err = state.StartMigration("test-migration", "rename_column", "expand_contract", "users", "email", 3, "test-plan.json")
	if err != nil {
		t.Fatalf("Failed to start migration: %v", err)
	}

	// Complete phase 1
	err = state.CompletePhase(1)
	if err != nil {
		t.Fatalf("Failed to complete phase 1: %v", err)
	}

	if state.ActiveMigration.CurrentPhase != 1 {
		t.Errorf("Expected current phase 1, got %d", state.ActiveMigration.CurrentPhase)
	}

	if len(state.ActiveMigration.PhasesCompleted) != 1 {
		t.Errorf("Expected 1 completed phase, got %d", len(state.ActiveMigration.PhasesCompleted))
	}

	// Complete phase 2
	err = state.CompletePhase(2)
	if err != nil {
		t.Fatalf("Failed to complete phase 2: %v", err)
	}

	// Complete phase 3 - should clear active migration
	err = state.CompletePhase(3)
	if err != nil {
		t.Fatalf("Failed to complete phase 3: %v", err)
	}

	if state.ActiveMigration != nil {
		t.Error("Expected active migration to be cleared after final phase")
	}
}

func TestCanExecutePhase(t *testing.T) {
	defer func() { _ = os.Remove(StateFile) }()

	state, err := Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// No active migration - can only execute phase 1
	if err := state.CanExecutePhase(1); err != nil {
		t.Errorf("Should be able to execute phase 1: %v", err)
	}

	if err := state.CanExecutePhase(2); err == nil {
		t.Error("Should not be able to skip to phase 2")
	}

	// Start migration
	err = state.StartMigration("test-migration", "rename_column", "expand_contract", "users", "email", 3, "test-plan.json")
	if err != nil {
		t.Fatalf("Failed to start migration: %v", err)
	}

	// Can execute phase 1
	if err := state.CanExecutePhase(1); err != nil {
		t.Errorf("Should be able to execute phase 1: %v", err)
	}

	// Cannot skip to phase 3
	if err := state.CanExecutePhase(3); err == nil {
		t.Error("Should not be able to skip to phase 3")
	}

	// Complete phase 1
	err = state.CompletePhase(1)
	if err != nil {
		t.Fatalf("Failed to complete phase 1: %v", err)
	}

	// Cannot re-execute phase 1
	if err := state.CanExecutePhase(1); err == nil {
		t.Error("Should not be able to re-execute completed phase")
	}

	// Can execute phase 2
	if err := state.CanExecutePhase(2); err != nil {
		t.Errorf("Should be able to execute phase 2: %v", err)
	}
}

func TestGetNextPhase(t *testing.T) {
	defer func() { _ = os.Remove(StateFile) }()

	state, err := Load()
	if err != nil {
		t.Fatalf("Failed to load state: %v", err)
	}

	// No active migration - next is phase 1
	if next := state.GetNextPhase(); next != 1 {
		t.Errorf("Expected next phase 1, got %d", next)
	}

	// Start migration
	err = state.StartMigration("test-migration", "rename_column", "expand_contract", "users", "email", 3, "test-plan.json")
	if err != nil {
		t.Fatalf("Failed to start migration: %v", err)
	}

	// Next is still phase 1
	if next := state.GetNextPhase(); next != 1 {
		t.Errorf("Expected next phase 1, got %d", next)
	}

	// Complete phase 1
	if err := state.CompletePhase(1); err != nil {
		t.Fatalf("Failed to complete phase 1: %v", err)
	}

	// Next is phase 2
	if next := state.GetNextPhase(); next != 2 {
		t.Errorf("Expected next phase 2, got %d", next)
	}

	// Complete phases 2 and 3
	if err := state.CompletePhase(2); err != nil {
		t.Fatalf("Failed to complete phase 2: %v", err)
	}
	if err := state.CompletePhase(3); err != nil {
		t.Fatalf("Failed to complete phase 3: %v", err)
	}

	// All complete - migration cleared, next is 1 (ready for new migration)
	if next := state.GetNextPhase(); next != 1 {
		t.Errorf("Expected next phase 1 (ready for new migration), got %d", next)
	}

	// Verify active migration is cleared
	if state.ActiveMigration != nil {
		t.Error("Expected active migration to be cleared")
	}
}
