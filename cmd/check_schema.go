package cmd

import (
	"fmt"
	"log"
	"os"

	"github.com/lockplane/lockplane/internal/schema"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(checkSchemaCmd)
}

var checkSchemaCmd = &cobra.Command{
	Use:   "check [schema dir or .lp.sql file]",
	Short: "Check .lp.sql schema files for errors",
	Long: `Check .lp.sql schema files for errors and print a JSON summary

When provided a directory, lockplane will check all .lp.sql files in the root
of that directory.

Examples:
lockplane check schema/
lockplane check my-schema.lp.sql
lockplane check my-schema.lp.sql > report.json
`,
	Run: runCheckSchema,
}

func runCheckSchema(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		fmt.Printf(`Missing a schema file.

Usage: lockplane check [schema dir or .lp.sql file]
Help: lockplane check --help
`)
		os.Exit(1)
	}
	schemaPath := args[0]

	reportJson, err := schema.CheckSchema(schemaPath)
	if err != nil {
		log.Fatalf("Failed to check schema: %v", err)
	}
	fmt.Print(reportJson)
}
