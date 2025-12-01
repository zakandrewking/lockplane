package postgres

import (
	"testing"
)

// TODO integration test of TestConnection with a real DB

func TestDriver_Name(t *testing.T) {
	driver := NewDriver()

	if driver.Name() != "postgres" {
		t.Errorf("Expected name 'postgres', got '%s'", driver.Name())
	}
}
