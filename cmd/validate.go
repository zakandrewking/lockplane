package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/lockplane/lockplane/internal/schema"
	"github.com/lockplane/lockplane/internal/validation"
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate schema and plan files",
	Long: `Validate schema and plan files in different formats.

Subcommands:
  schema - Validate JSON schema file against JSON Schema
  plan   - Validate migration plan JSON file

For SQL validation, use: lockplane plan --validate <schema-dir>`,
	Example: `  # Validate JSON schema
  lockplane validate schema schema.json

  # Validate migration plan
  lockplane validate plan migration.json`,
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

var validatePlanCmd = &cobra.Command{
	Use:   "plan [file]",
	Short: "Validate migration plan JSON file",
	Long:  `Validate a migration plan JSON file for correctness and completeness.`,
	Run:   runValidatePlan,
}

var (
	validateSchemaFile string
)

func init() {
	rootCmd.AddCommand(validateCmd)
	validateCmd.AddCommand(validateSchemaCmd)
	validateCmd.AddCommand(validatePlanCmd)

	validateSchemaCmd.Flags().StringVarP(&validateSchemaFile, "file", "f", "", "Path to schema JSON file")
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

func runValidatePlan(cmd *cobra.Command, args []string) {
	// Delegate to the existing plan validation implementation
	validation.RunValidatePlan(args)
}
