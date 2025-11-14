package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/sqlvalidation"
	"github.com/lockplane/lockplane/internal/validation"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate schema and plan files",
	Long: `Validate schema and plan files in different formats.

Subcommands:
  schema - Validate JSON schema file against JSON Schema
  sql    - Validate SQL DDL file or directory of .lp.sql files
  plan   - Validate migration plan JSON file`,
	Example: `  # Validate JSON schema
  lockplane validate schema schema.json

  # Validate SQL schema
  lockplane validate sql schema.lp.sql

  # Validate migration plan
  lockplane validate plan migration.json

  # Validate with JSON output (for IDE integration)
  lockplane validate sql --output-format json schema.lp.sql`,
}

var validateSchemaCmd = &cobra.Command{
	Use:   "schema [file]",
	Short: "Validate a JSON schema file against the Lockplane JSON Schema",
	Long: `Validate a JSON schema file against the Lockplane JSON Schema.

The file can be specified as a positional argument or with --file flag.`,
	Example: `  # Validate JSON schema file
  lockplane validate schema schema.json

  # Validate using --file flag
  lockplane validate schema --file schema.json`,
	Args: cobra.MaximumNArgs(1),
	Run:  runValidateSchema,
}

var validateSQLCmd = &cobra.Command{
	Use:   "sql [file]",
	Short: "Validate SQL DDL file or directory of .lp.sql files",
	Long:  `Validate SQL DDL file or directory of .lp.sql files for syntax and schema correctness.`,
	Run:   runValidateSQL,
}

var validatePlanCmd = &cobra.Command{
	Use:   "plan [file]",
	Short: "Validate migration plan JSON file",
	Long:  `Validate a migration plan JSON file for correctness and completeness.`,
	Run:   runValidatePlan,
}

var (
	validateSchemaFile      string
	validateSQLOutputFormat string
)

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.AddCommand(validateSchemaCmd)
	validateCmd.AddCommand(validateSQLCmd)
	validateCmd.AddCommand(validatePlanCmd)

	validateSchemaCmd.Flags().StringVarP(&validateSchemaFile, "file", "f", "", "Path to schema JSON file")
	validateSQLCmd.Flags().StringVar(&validateSQLOutputFormat, "output-format", "text", "Output format: text (default) or json")
}

func runValidateSchema(cmd *cobra.Command, args []string) {
	path := validateSchemaFile
	if path == "" && len(args) > 0 {
		path = args[0]
	}
	if path == "" {
		_ = cmd.Usage()
		os.Exit(1)
	}

	if err := schema.ValidateJSONSchema(path); err != nil {
		log.Fatalf("Schema validation failed: %v", err)
	}

	fmt.Fprintf(os.Stderr, "✓ Schema JSON is valid: %s\n", path)
}

func runValidateSQL(cmd *cobra.Command, args []string) {
	// Add the output-format flag to args (RunValidateSQL will parse it)
	args = append([]string{"--output-format", validateSQLOutputFormat}, args...)
	// Delegate to the existing SQL validation implementation
	sqlvalidation.RunValidateSQL(args)
}

func runValidatePlan(cmd *cobra.Command, args []string) {
	// Delegate to the existing plan validation implementation
	validation.RunValidatePlan(args)
}
