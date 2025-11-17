package cmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/lockplane/lockplane/database"
	"github.com/lockplane/lockplane/internal/config"
	"github.com/lockplane/lockplane/internal/executor"
	"github.com/lockplane/lockplane/internal/introspect"
	"github.com/lockplane/lockplane/internal/planner"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/validation"
	pg_query "github.com/pganalyze/pg_query_go/v6"
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
	planCheckSchema     bool
	planVerbose         bool
	planOutput          string
	planShadowDB        string
	planShadowSchema    string
	planCacheDir        string
)

func init() {
	rootCmd.AddCommand(planCmd)

	planCmd.Flags().StringVar(&planFrom, "from", "", "Source schema path (file or directory)")
	planCmd.Flags().StringVar(&planTo, "to", "", "Target schema path (file or directory)")
	planCmd.Flags().StringVar(&planFromEnvironment, "from-environment", "", "Environment providing the source database connection")
	planCmd.Flags().StringVar(&planToEnvironment, "to-environment", "", "Environment providing the target database connection")
	planCmd.Flags().BoolVar(&planCheckSchema, "check-schema", false, "Check schema files for SQL validity by applying them to a clean shadow database")
	planCmd.Flags().BoolVarP(&planVerbose, "verbose", "v", false, "Enable verbose logging")
	planCmd.Flags().StringVar(&planOutput, "output", "", "Output format (default: text, set to 'json' for IDE integration)")
	planCmd.Flags().StringVar(&planShadowDB, "shadow-db", "", "Shadow database URL for validation")
	planCmd.Flags().StringVar(&planShadowSchema, "shadow-schema", "", "Shadow schema name when reusing an existing database")
	planCmd.Flags().StringVar(&planCacheDir, "cache-dir", "", "Directory for caching shadow DB state (for incremental validation)")
}

func runPlan(cmd *cobra.Command, args []string) {
	cfg, err := config.LoadConfig()
	if err != nil {
		log.Fatalf("Failed to load config file: %v", err)
	}

	// NEW MODE: plan --validate <schema-dir>
	// If --validate is set and neither --from nor --to are provided, run shadow DB validation
	fromInput := strings.TrimSpace(planFrom)
	toInput := strings.TrimSpace(planTo)

	if planCheckSchema && fromInput == "" && toInput == "" && planFromEnvironment == "" && planToEnvironment == "" {
		// This is the new shadow DB validation mode
		runShadowDBValidation(cfg, args)
		return
	}

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
	if planCheckSchema {
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

// SyntaxError represents a SQL syntax error in a specific file
type SyntaxError struct {
	File    string
	Line    int
	Column  int
	Message string
}

// preValidateSQLSyntax checks all SQL files for syntax errors before hitting the database.
// Returns all syntax errors found across all files.
func preValidateSQLSyntax(schemaDir string, dialect database.Dialect) []SyntaxError {
	var errors []SyntaxError

	// Find all .sql files in the schema directory
	err := filepath.Walk(schemaDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories and non-.sql files
		if info.IsDir() || !strings.HasSuffix(path, ".sql") {
			return nil
		}

		// Read the file
		content, readErr := os.ReadFile(path)
		if readErr != nil {
			errors = append(errors, SyntaxError{
				File:    path,
				Line:    1,
				Column:  1,
				Message: fmt.Sprintf("Failed to read file: %v", readErr),
			})
			return nil // Continue processing other files
		}

		// Parse the SQL based on dialect
		var parseErr error
		if dialect == database.DialectPostgres || dialect == database.DialectUnknown {
			_, parseErr = pg_query.Parse(string(content))
		} else {
			// For SQLite and other dialects, we currently only support PostgreSQL parsing
			// TODO: Add SQLite-specific syntax validation
			return nil
		}

		if parseErr != nil {
			// Extract line number from error if possible
			errMsg := parseErr.Error()
			line := 1
			column := 1

			// pg_query error messages often contain line numbers like "syntax error at or near ... at character 42"
			// We'll just use line 1 for now and show the full error message
			errors = append(errors, SyntaxError{
				File:    path,
				Line:    line,
				Column:  column,
				Message: errMsg,
			})
		}

		return nil
	})

	if err != nil {
		errors = append(errors, SyntaxError{
			File:    schemaDir,
			Line:    1,
			Column:  1,
			Message: fmt.Sprintf("Failed to walk directory: %v", err),
		})
	}

	return errors
}

// runShadowDBValidation validates schema files by applying them to a shadow database.
// This is the new validation mode: plan --check-schema <schema-dir>
func runShadowDBValidation(cfg *config.Config, args []string) {
	ctx := context.Background()

	// Step 1: Determine schema directory
	schemaDir := ""
	if len(args) > 0 {
		schemaDir = strings.TrimSpace(args[0])
	}

	// Auto-detect if not provided
	if schemaDir == "" {
		if info, err := os.Stat("schema"); err == nil && info.IsDir() {
			schemaDir = "schema"
			if planVerbose {
				fmt.Fprintf(os.Stderr, "ℹ️  Auto-detected schema directory: schema/\n")
			}
		}
	}

	if schemaDir == "" {
		fmt.Fprintf(os.Stderr, "Error: No schema directory specified.\n\n")
		fmt.Fprintf(os.Stderr, "Usage: lockplane plan --check-schema <schema-dir>\n")
		fmt.Fprintf(os.Stderr, "   Or: lockplane plan --check-schema (will auto-detect schema/ directory)\n\n")
		os.Exit(1)
	}

	// Step 1.5: Pre-validate SQL syntax (fast fail before connecting to DB)
	if planVerbose {
		fmt.Fprintf(os.Stderr, "📋 Pre-validating SQL syntax...\n")
	}

	// Detect dialect based on connection string (we'll infer it)
	// For now, use Postgres as the default since that's our primary dialect
	dialect := database.DialectPostgres

	syntaxErrors := preValidateSQLSyntax(schemaDir, dialect)
	if len(syntaxErrors) > 0 {
		// Convert syntax errors to diagnostic format
		var diagnosticMsgs []string
		for _, syntaxErr := range syntaxErrors {
			msg := fmt.Sprintf("%s:%d:%d: %s", syntaxErr.File, syntaxErr.Line, syntaxErr.Column, syntaxErr.Message)
			diagnosticMsgs = append(diagnosticMsgs, msg)
		}

		// Report all syntax errors
		mainMsg := fmt.Sprintf("Found %d syntax error(s) in schema files", len(syntaxErrors))
		validationFailure(mainMsg, diagnosticMsgs)
	}

	if planVerbose {
		fmt.Fprintf(os.Stderr, "✓ SQL syntax validation passed\n")
	}

	// Step 2: Resolve shadow DB connection
	shadowConnStr := strings.TrimSpace(planShadowDB)
	shadowSchema := strings.TrimSpace(planShadowSchema)

	var resolvedShadow *config.ResolvedEnvironment
	if shadowConnStr == "" || shadowSchema == "" {
		if env, err := config.ResolveEnvironment(cfg, ""); err == nil {
			resolvedShadow = env
			if shadowConnStr == "" {
				shadowConnStr = env.ShadowDatabaseURL
			}
			if shadowSchema == "" {
				shadowSchema = env.ShadowSchema
			}
			if shadowSchema != "" && shadowConnStr == "" {
				shadowConnStr = env.DatabaseURL
			}
		}
	}

	if shadowConnStr == "" {
		exampleEnv := "local"
		if resolvedShadow != nil && resolvedShadow.Name != "" {
			exampleEnv = resolvedShadow.Name
		}
		fmt.Fprintf(os.Stderr, "Error: No shadow database configured.\n\n")
		fmt.Fprintf(os.Stderr, "Provide shadow DB via:\n")
		fmt.Fprintf(os.Stderr, "  - --shadow-db flag\n")
		fmt.Fprintf(os.Stderr, "  - SHADOW_DATABASE_URL or SHADOW_SCHEMA in .env.%s\n", exampleEnv)
		fmt.Fprintf(os.Stderr, "  - lockplane init (auto-configures shadow DB settings)\n\n")
		os.Exit(1)
	}

	// Step 3: Connect to shadow DB
	if planVerbose {
		fmt.Fprintf(os.Stderr, "🔗 Connecting to shadow database...\n")
	}

	driverType := executor.DetectDriver(shadowConnStr)
	driver, err := executor.NewDriver(driverType)
	if err != nil {
		validationFailure(fmt.Sprintf("Failed to create database driver: %v", err), nil)
	}

	shadowDB, err := sql.Open(driverType, shadowConnStr)
	if err != nil {
		validationFailure(fmt.Sprintf("Failed to connect to shadow database: %v", err), nil)
	}
	defer func() {
		_ = shadowDB.Close()
	}()

	if shadowSchema != "" && driver.SupportsSchemas() {
		if err := driver.CreateSchema(ctx, shadowDB, shadowSchema); err != nil {
			validationFailure(fmt.Sprintf("Failed to create shadow schema: %v", err), nil)
		}
		if err := driver.SetSchema(ctx, shadowDB, shadowSchema); err != nil {
			validationFailure(fmt.Sprintf("Failed to set shadow schema: %v", err), nil)
		}
		if !isJSONOutput() {
			fmt.Fprintf(os.Stderr, "ℹ️  Using shadow schema %q for validation\n", shadowSchema)
		}
	}

	// Step 4: Clean shadow DB
	if planVerbose {
		fmt.Fprintf(os.Stderr, "🧹 Cleaning shadow database...\n")
	}

	if err := executor.CleanupShadowDB(ctx, shadowDB, driver, planVerbose); err != nil {
		validationFailure(fmt.Sprintf("Failed to clean shadow database: %v", err), nil)
	}

	// Step 5: Load schema files
	if planVerbose {
		fmt.Fprintf(os.Stderr, "📖 Loading schema from %s...\n", schemaDir)
	}

	dialect = schema.DriverNameToDialect(driverType)
	opts := executor.BuildSchemaLoadOptions(schemaDir, dialect)
	desiredSchema, err := executor.LoadSchemaOrIntrospectWithOptions(schemaDir, opts)
	if err != nil {
		validationFailure(fmt.Sprintf("Failed to load schema: %v", err), nil)
	}

	// Step 6: Generate a plan from empty schema to desired schema
	emptySchema := &database.Schema{Tables: []database.Table{}, Dialect: dialect}
	diff := schema.DiffSchemas(emptySchema, desiredSchema)

	plan, err := planner.GeneratePlanWithHash(diff, emptySchema, driver)
	if err != nil {
		validationFailure(fmt.Sprintf("Failed to generate plan: %v", err), nil)
	}

	if planVerbose {
		fmt.Fprintf(os.Stderr, "✓ Generated plan with %d steps\n", len(plan.Steps))
	}

	// Step 7: Execute plan on shadow DB (this validates the schema)
	if planVerbose {
		fmt.Fprintf(os.Stderr, "🧪 Validating schema by applying to shadow database...\n")
	}

	result, err := executor.ApplyPlan(ctx, shadowDB, plan, nil, emptySchema, driver, planVerbose)

	// Step 8: Output results
	if err != nil {
		var extras []string
		if result != nil {
			extras = result.Errors
		}
		validationFailure(fmt.Sprintf("Schema validation failed: %v", err), extras)
	}

	validationSuccess(result)
}

func isJSONOutput() bool {
	return strings.EqualFold(strings.TrimSpace(planOutput), "json")
}

func validationFailure(message string, details []string) {
	mainMsg := strings.TrimSpace(message)
	if mainMsg == "" {
		mainMsg = "Schema validation failed."
	}
	formatted := mainMsg
	if len(details) > 0 {
		formatted = fmt.Sprintf("%s\n%s", mainMsg, strings.Join(details, "\n"))
	}

	if isJSONOutput() {
		diagnostics := map[string]interface{}{
			"diagnostics": []map[string]interface{}{
				{
					"severity": "error",
					"message":  formatted,
					"code":     "validation_error",
				},
			},
			"summary": map[string]interface{}{
				"errors": 1,
				"valid":  false,
			},
		}
		jsonBytes, _ := json.MarshalIndent(diagnostics, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Fprintf(os.Stderr, "❌ Schema validation FAILED\n\n")
		fmt.Fprintf(os.Stderr, "%s\n", mainMsg)
		for _, detail := range details {
			fmt.Fprintf(os.Stderr, "  - %s\n", detail)
		}
	}
	os.Exit(1)
}

func validationSuccess(result *planner.ExecutionResult) {
	steps := 0
	if result != nil {
		steps = result.StepsApplied
	}
	if isJSONOutput() {
		diagnostics := map[string]interface{}{
			"diagnostics": []map[string]interface{}{},
			"summary": map[string]interface{}{
				"errors":        0,
				"valid":         true,
				"steps_applied": steps,
			},
		}
		jsonBytes, _ := json.MarshalIndent(diagnostics, "", "  ")
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Fprintf(os.Stderr, "✅ Schema validation PASSED\n")
		fmt.Fprintf(os.Stderr, "   Applied %d steps successfully\n", steps)
	}
}
