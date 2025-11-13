package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/introspect"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/validation"
	"github.com/spf13/cobra"
)

var planCmd = &cobra.Command{
	Use:   "plan",
	Short: "Generate a migration plan from schema differences",
	Long: `Generate a migration plan by comparing two schemas.

Schemas can be:
  • JSON schema files
  • SQL DDL files or directories
  • Database connection strings (will introspect)

The plan shows all required SQL operations to transform the source schema
into the target schema.`,
	Example: `  # Generate plan from database to schema file
  lockplane plan --from postgresql://localhost/db --to schema.json > plan.json

  # Generate plan between two schema files
  lockplane plan --from old.json --to new.json > plan.json

  # Use environments from lockplane.toml
  lockplane plan --from-environment production --to schema/ > plan.json

  # Validate migration safety
  lockplane plan --from db.json --to new.json --validate > plan.json`,
	Run: runPlan,
}

var (
	planFrom            string
	planTo              string
	planFromEnvironment string
	planToEnvironment   string
	planValidate        bool
	planVerbose         bool
)

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVar(&planFrom, "from", "", "Source schema path (file or directory)")
	planCmd.Flags().StringVar(&planTo, "to", "", "Target schema path (file or directory)")
	planCmd.Flags().StringVar(&planFromEnvironment, "from-environment", "", "Environment providing the source database connection")
	planCmd.Flags().StringVar(&planToEnvironment, "to-environment", "", "Environment providing the target database connection")
	planCmd.Flags().BoolVar(&planValidate, "validate", false, "Validate migration safety and reversibility")
	planCmd.Flags().BoolVarP(&planVerbose, "verbose", "v", false, "Enable verbose logging")
}

func runPlan(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	fromInput := strings.TrimSpace(planFrom)
	toInput := strings.TrimSpace(planTo)

	if fromInput == "" {
		resolvedFrom, err := config.ResolveEnvironment(cfg, planFromEnvironment)
		if err != nil {
			log.Fatalf("Failed to resolve source environment: %v", err)
		}
		fromInput = resolvedFrom.DatabaseURL
		if fromInput == "" {
			fmt.Fprintf(os.Stderr, "Error: environment %q does not define a source database. Provide --from or configure .env.%s.\n", resolvedFrom.Name, resolvedFrom.Name)
			os.Exit(1)
		}
	}

	if toInput == "" {
		// Try to auto-detect schema directory first (like apply command does)
		if info, err := os.Stat("schema"); err == nil && info.IsDir() {
			toInput = "schema"
			if planVerbose {
				fmt.Fprintf(os.Stderr, "ℹ️  Auto-detected schema directory: schema/\n")
			}
		} else {
			// Fall back to environment resolution
			resolvedTo, err := config.ResolveEnvironment(cfg, planToEnvironment)
			if err != nil {
				log.Fatalf("Failed to resolve target environment: %v", err)
			}
			toInput = resolvedTo.DatabaseURL
			if toInput == "" {
				fmt.Fprintf(os.Stderr, "Error: environment %q does not define a target database. Provide --to or configure .env.%s.\n", resolvedTo.Name, resolvedTo.Name)
				os.Exit(1)
			}
		}
	}

	if fromInput == "" || toInput == "" {
		log.Fatalf("Usage: lockplane plan --from <before.json|db> --to <after.json|db> [--validate]\n\n       lockplane plan --from-environment <name> --to <schema.json>\n       lockplane plan --from <schema.json> --to-environment <name>")
	}

	// Generate diff first
	var diff *schema.SchemaDiff
	var before *database.Schema
	var after *database.Schema

	var fromFallback, toFallback database.Dialect
	if introspect.IsConnectionString(fromInput) {
		fromFallback = schema.DriverNameToDialect(executor.DetectDriver(fromInput))
		if !introspect.IsConnectionString(toInput) {
			toFallback = fromFallback
		}
	}
	if introspect.IsConnectionString(toInput) {
		toFallback = schema.DriverNameToDialect(executor.DetectDriver(toInput))
		if fromFallback == database.DialectUnknown {
			fromFallback = toFallback
		}
	}

	var loadErr error
	if planVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Loading 'from' schema: %s\n", fromInput)
	}
	before, loadErr = executor.LoadSchemaOrIntrospectWithOptions(fromInput, executor.BuildSchemaLoadOptions(fromInput, fromFallback))
	if loadErr != nil {
		if planVerbose {
			fmt.Fprintf(os.Stderr, "❌ Failed to load from schema\n")
			fmt.Fprintf(os.Stderr, "   Input: %s\n", fromInput)
			fmt.Fprintf(os.Stderr, "   isConnectionString: %v\n", introspect.IsConnectionString(fromInput))
			fmt.Fprintf(os.Stderr, "   Error: %v\n", loadErr)
		}
		log.Fatalf("Failed to load from schema: %v", loadErr)
	}
	if planVerbose {
		fmt.Fprintf(os.Stderr, "✓ Loaded 'from' schema (%d tables)\n", len(before.Tables))
	}

	if planVerbose {
		fmt.Fprintf(os.Stderr, "🔍 Loading 'to' schema: %s\n", toInput)
	}
	after, loadErr = executor.LoadSchemaOrIntrospectWithOptions(toInput, executor.BuildSchemaLoadOptions(toInput, toFallback))
	if loadErr != nil {
		if planVerbose {
			fmt.Fprintf(os.Stderr, "❌ Failed to load to schema\n")
			fmt.Fprintf(os.Stderr, "   Input: %s\n", toInput)
			fmt.Fprintf(os.Stderr, "   isConnectionString: %v\n", introspect.IsConnectionString(toInput))
			fmt.Fprintf(os.Stderr, "   Error: %v\n", loadErr)
		}
		log.Fatalf("Failed to load to schema: %v", loadErr)
	}
	if planVerbose {
		fmt.Fprintf(os.Stderr, "✓ Loaded 'to' schema (%d tables)\n", len(after.Tables))
	}

	diff = schema.DiffSchemas(before, after)

	// Validate the diff if requested
	if planValidate {
		validationResults := validation.ValidateSchemaDiffWithSchema(diff, after)

		if len(validationResults) > 0 {
			fmt.Fprintf(os.Stderr, "\n=== Migration Safety Report ===\n\n")

			for i, result := range validationResults {
				// Show safety classification with icon
				if result.Safety != nil {
					fmt.Fprintf(os.Stderr, "%s %s", result.Safety.Level.Icon(), result.Safety.Level.String())
					if result.Valid {
						fmt.Fprintf(os.Stderr, " (Operation %d)\n", i+1)
					} else {
						fmt.Fprintf(os.Stderr, " - BLOCKED (Operation %d)\n", i+1)
					}
				} else if result.Valid {
					fmt.Fprintf(os.Stderr, "✓ Operation %d: PASS\n", i+1)
				} else {
					fmt.Fprintf(os.Stderr, "✗ Operation %d: FAIL\n", i+1)
				}

				// Show safety details
				if result.Safety != nil {
					if result.Safety.BreakingChange {
						fmt.Fprintf(os.Stderr, "  ⚠️  Breaking change - will affect running applications\n")
					}
					if result.Safety.DataLoss {
						fmt.Fprintf(os.Stderr, "  💥 Permanent data loss\n")
					}
					if !result.Reversible && result.Safety.RollbackDescription != "" {
						fmt.Fprintf(os.Stderr, "  ↩️  Rollback: %s\n", result.Safety.RollbackDescription)
					} else if result.Reversible && result.Safety.RollbackDataLoss {
						fmt.Fprintf(os.Stderr, "  ↩️  Rollback: %s\n", result.Safety.RollbackDescription)
					}
				} else if !result.Reversible {
					fmt.Fprintf(os.Stderr, "  ⚠️  NOT REVERSIBLE\n")
				}

				for _, err := range result.Errors {
					fmt.Fprintf(os.Stderr, "  ❌ Error: %s\n", err)
				}

				for _, warning := range result.Warnings {
					fmt.Fprintf(os.Stderr, "  ⚠️  Warning: %s\n", warning)
				}

				// Show safer alternatives for dangerous operations
				if result.Safety != nil && len(result.Safety.SaferAlternatives) > 0 {
					fmt.Fprintf(os.Stderr, "\n  💡 Safer alternatives:\n")
					for _, alt := range result.Safety.SaferAlternatives {
						fmt.Fprintf(os.Stderr, "     • %s\n", alt)
					}
				}

				fmt.Fprintf(os.Stderr, "\n")
			}

			// Summary section
			fmt.Fprintf(os.Stderr, "=== Summary ===\n\n")

			// Count by safety level
			safeCnt, reviewCnt, lossyCnt, dangerousCnt, multiPhaseCnt := 0, 0, 0, 0, 0
			for _, r := range validationResults {
				if r.Safety != nil {
					switch r.Safety.Level {
					case validation.SafetyLevelSafe:
						safeCnt++
					case validation.SafetyLevelReview:
						reviewCnt++
					case validation.SafetyLevelLossy:
						lossyCnt++
					case validation.SafetyLevelDangerous:
						dangerousCnt++
					case validation.SafetyLevelMultiPhase:
						multiPhaseCnt++
					}
				}
			}

			if safeCnt > 0 {
				fmt.Fprintf(os.Stderr, "  ✅ %d safe operation(s)\n", safeCnt)
			}
			if reviewCnt > 0 {
				fmt.Fprintf(os.Stderr, "  ⚠️  %d operation(s) require review\n", reviewCnt)
			}
			if lossyCnt > 0 {
				fmt.Fprintf(os.Stderr, "  🔶 %d lossy operation(s)\n", lossyCnt)
			}
			if dangerousCnt > 0 {
				fmt.Fprintf(os.Stderr, "  ❌ %d dangerous operation(s)\n", dangerousCnt)
			}
			if multiPhaseCnt > 0 {
				fmt.Fprintf(os.Stderr, "  🔄 %d operation(s) require multi-phase migration\n", multiPhaseCnt)
			}

			fmt.Fprintf(os.Stderr, "\n")

			if !validation.AllValid(validationResults) {
				fmt.Fprintf(os.Stderr, "❌ Validation FAILED: Some operations are not safe\n\n")
				os.Exit(1)
			}

			// Warn about dangerous operations
			if validation.HasDangerousOperations(validationResults) {
				fmt.Fprintf(os.Stderr, "⚠️  WARNING: This migration contains dangerous operations.\n")
				fmt.Fprintf(os.Stderr, "   Review safer alternatives above before proceeding.\n\n")
			}

			if validation.AllReversible(validationResults) {
				fmt.Fprintf(os.Stderr, "✓ All operations are reversible\n\n")
			} else {
				fmt.Fprintf(os.Stderr, "⚠️  Warning: Some operations are NOT reversible\n")
				fmt.Fprintf(os.Stderr, "   Data loss may be permanent. Test on shadow DB first.\n\n")
			}
		}
	}

	// Detect database driver from target schema (the "to" state)
	// We generate SQL for the target database type
	// First check if the schema has a dialect set (from SQL file or JSON)
	var targetDriverType string
	if after.Dialect != "" && after.Dialect != database.DialectUnknown {
		// Use the dialect from the loaded schema
		targetDriverType = string(after.Dialect)
	} else {
		// Fall back to detecting from connection string/path
		targetDriverType = executor.DetectDriver(toInput)
	}
	targetDriver, err := executor.NewDriver(targetDriverType)
	if err != nil {
		log.Fatalf("Failed to create database driver: %v", err)
	}

	// Generate plan with source hash
	plan, err := planner.GeneratePlanWithHash(diff, before, targetDriver)
	if err != nil {
		log.Fatalf("Failed to generate plan: %v", err)
	}

	// Output plan as JSON
	jsonBytes, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal plan to JSON: %v", err)
	}

	fmt.Println(string(jsonBytes))
}
