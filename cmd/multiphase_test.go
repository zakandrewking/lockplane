package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestPlanMultiphaseCommand(t *testing.T) {
	cmd := rootCmd
	var planMultiphaseCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "plan-multiphase" {
			planMultiphaseCmd = c
			break
		}
	}

	if planMultiphaseCmd == nil {
		t.Fatal("plan-multiphase command should be registered")
	}

	if planMultiphaseCmd.Short == "" {
		t.Error("plan-multiphase command.Short should not be empty")
	}
}

func TestApplyPhaseCommand(t *testing.T) {
	cmd := rootCmd
	var applyPhaseCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "apply-phase" {
			applyPhaseCmd = c
			break
		}
	}

	if applyPhaseCmd == nil {
		t.Fatal("apply-phase command should be registered")
	}

	if applyPhaseCmd.Short == "" {
		t.Error("apply-phase command.Short should not be empty")
	}
}

func TestRollbackPhaseCommand(t *testing.T) {
	cmd := rootCmd
	var rollbackPhaseCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "rollback-phase" {
			rollbackPhaseCmd = c
			break
		}
	}

	if rollbackPhaseCmd == nil {
		t.Fatal("rollback-phase command should be registered")
	}

	if rollbackPhaseCmd.Short == "" {
		t.Error("rollback-phase command.Short should not be empty")
	}
}

func TestPhaseStatusCommand(t *testing.T) {
	cmd := rootCmd
	var phaseStatusCmd *cobra.Command
	for _, c := range cmd.Commands() {
		if c.Name() == "phase-status" {
			phaseStatusCmd = c
			break
		}
	}

	if phaseStatusCmd == nil {
		t.Fatal("phase-status command should be registered")
	}

	if phaseStatusCmd.Short == "" {
		t.Error("phase-status command.Short should not be empty")
	}
}
