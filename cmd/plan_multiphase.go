package cmd

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/planner/multiphase"
	"github.com/spf13/cobra"
)

var planMultiphaseCmd = &cobra.Command{
	Use:   "plan-multiphase",
	Short: "Generate a multi-phase migration plan for breaking changes",
	Long: `Generate a multi-phase migration plan for operations that require
coordination between database changes and code deployments.

Multi-phase migrations use patterns like expand/contract to enable
zero-downtime deployments when making breaking schema changes.

Supported patterns:
  • expand_contract: Column rename or compatible type change
  • deprecation: Safe removal of columns/tables with deprecation period
  • validation: Add constraints with backfill and validation phases
  • type_change: Incompatible column type changes with dual-write`,
	Example: `  # Generate expand/contract plan for column rename
  lockplane plan-multiphase \
    --pattern expand_contract \
    --table users \
    --old-column email \
    --new-column email_address \
    --type TEXT > rename-plan.json

  # Generate deprecation plan for column removal
  lockplane plan-multiphase \
    --pattern deprecation \
    --table users \
    --column last_login > drop-column-plan.json

  # Generate validation plan for adding NOT NULL constraint
  lockplane plan-multiphase \
    --pattern validation \
    --table posts \
    --constraint "CHECK (status IN ('draft', 'published'))" > add-constraint-plan.json

  # Generate type change plan
  lockplane plan-multiphase \
    --pattern type_change \
    --table users \
    --column age \
    --old-type TEXT \
    --new-type INTEGER > type-change-plan.json`,
	Run: runPlanMultiphase,
}

var (
	mpPattern    string
	mpTable      string
	mpColumn     string
	mpOldColumn  string
	mpNewColumn  string
	mpType       string
	mpOldType    string
	mpNewType    string
	mpConstraint string
	mpSourceHash string
)

func init() {
	rootCmd.AddCommand(planMultiphaseCmd)

	planMultiphaseCmd.Flags().StringVar(&mpPattern, "pattern", "", "Migration pattern: expand_contract, deprecation, validation, type_change (required)")
	planMultiphaseCmd.Flags().StringVar(&mpTable, "table", "", "Table name (required)")
	planMultiphaseCmd.Flags().StringVar(&mpColumn, "column", "", "Column name")
	planMultiphaseCmd.Flags().StringVar(&mpOldColumn, "old-column", "", "Old column name (for expand_contract)")
	planMultiphaseCmd.Flags().StringVar(&mpNewColumn, "new-column", "", "New column name (for expand_contract)")
	planMultiphaseCmd.Flags().StringVar(&mpType, "type", "", "Column type")
	planMultiphaseCmd.Flags().StringVar(&mpOldType, "old-type", "", "Old column type (for type_change)")
	planMultiphaseCmd.Flags().StringVar(&mpNewType, "new-type", "", "New column type (for type_change)")
	planMultiphaseCmd.Flags().StringVar(&mpConstraint, "constraint", "", "Constraint definition (for validation pattern)")
	planMultiphaseCmd.Flags().StringVar(&mpSourceHash, "source-hash", "", "Source schema hash (optional)")

	_ = planMultiphaseCmd.MarkFlagRequired("pattern")
	_ = planMultiphaseCmd.MarkFlagRequired("table")
}

func runPlanMultiphase(cmd *cobra.Command, args []string) {
	var multiPhasePlan *planner.MultiPhasePlan
	var err error

	// If no source hash provided, use empty string
	if mpSourceHash == "" {
		mpSourceHash = "0000000000000000000000000000000000000000000000000000000000000000"
	}

	switch mpPattern {
	case "expand_contract":
		if mpOldColumn == "" || mpNewColumn == "" {
			log.Fatal("expand_contract pattern requires --old-column and --new-column")
		}
		if mpType == "" {
			log.Fatal("expand_contract pattern requires --type")
		}

		multiPhasePlan, err = multiphase.GenerateExpandContractPlan(
			mpTable,
			mpOldColumn,
			mpNewColumn,
			mpType,
			mpSourceHash,
		)

	case "deprecation":
		if mpColumn == "" {
			log.Fatal("deprecation pattern requires --column")
		}
		if mpType == "" {
			log.Fatal("deprecation pattern requires --type")
		}

		multiPhasePlan, err = multiphase.GenerateDeprecationPlan(
			mpTable,
			mpColumn,
			mpType,
			false, // archiveData - can be made configurable later
			mpSourceHash,
		)

	case "validation":
		if mpConstraint == "" {
			log.Fatal("validation pattern requires --constraint")
		}
		if mpColumn == "" {
			log.Fatal("validation pattern requires --column")
		}
		if mpType == "" {
			log.Fatal("validation pattern requires --type")
		}

		// Determine constraint type from the constraint string
		var constraintType string
		var checkExpr string
		var backfillValue string

		if mpConstraint == "NOT NULL" {
			constraintType = "not_null"
			backfillValue = "''" // Default backfill value
		} else if len(mpConstraint) >= 5 && mpConstraint[:5] == "CHECK" {
			constraintType = "check"
			checkExpr = mpConstraint
			backfillValue = "" // No backfill for CHECK constraints
		} else if mpConstraint == "UNIQUE" {
			constraintType = "unique"
			backfillValue = ""
		} else {
			log.Fatal("Unsupported constraint type. Use 'NOT NULL', 'CHECK (...)', or 'UNIQUE'")
		}

		multiPhasePlan, err = multiphase.GenerateValidationPhasePlan(
			mpTable,
			mpColumn,
			mpType,
			constraintType,
			backfillValue,
			checkExpr,
			mpSourceHash,
		)

	case "type_change":
		if mpColumn == "" || mpOldType == "" || mpNewType == "" {
			log.Fatal("type_change pattern requires --column, --old-type, and --new-type")
		}

		// Generate default conversion expression
		conversionExpr := fmt.Sprintf("CAST(%s AS %s)", mpColumn, mpNewType)

		multiPhasePlan, err = multiphase.GenerateTypeChangePlan(
			mpTable,
			mpColumn,
			mpOldType,
			mpNewType,
			conversionExpr,
			mpSourceHash,
		)

	default:
		log.Fatalf("Unknown pattern: %s. Supported patterns: expand_contract, deprecation, validation, type_change", mpPattern)
	}

	if err != nil {
		log.Fatalf("Failed to generate multi-phase plan: %v", err)
	}

	// Output as JSON
	output, err := json.MarshalIndent(multiPhasePlan, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal plan to JSON: %v", err)
	}

	fmt.Println(string(output))
}
