package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/lockplane/lockplane/database/postgres"
	"github.com/lockplane/lockplane/internal/schema"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert",
	Short: "Convert schema between SQL DDL and JSON formats",
	Long: `Convert schema between SQL DDL and JSON formats.

Input can be:
  • A .lp.sql file
  • A directory containing .lp.sql files
  • A .json schema file

Output format defaults to JSON but can be changed with --to flag.`,
	Example: `  # Convert SQL to JSON
  lockplane convert --input schema.lp.sql --output schema.json

  # Convert a directory of .lp.sql files to JSON
  lockplane convert --input schema/ --output schema.json

  # Convert JSON to SQL
  lockplane convert --input schema.json --output schema.lp.sql --to sql

  # Output to stdout
  lockplane convert --input schema.lp.sql`,
	Run: runConvert,
}

var (
	convertInput  string
	convertOutput string
	convertFrom   string
	convertTo     string
)

func init() {
	rootCmd.AddCommand(convertCmd)

	convertCmd.Flags().StringVar(&convertInput, "input", "", "Input schema (.lp.sql file, directory, or .json)")
	convertCmd.Flags().StringVar(&convertOutput, "output", "", "Output file (defaults to stdout)")
	convertCmd.Flags().StringVar(&convertFrom, "from", "", "Input format: sql or json (auto-detected if not specified)")
	convertCmd.Flags().StringVar(&convertTo, "to", "json", "Output format: json or sql")

	_ = convertCmd.MarkFlagRequired("input")
}

func runConvert(cmd *cobra.Command, args []string) {
	if convertInput == "" {
		_ = cmd.Usage()
		log.Fatal("--input is required")
	}

	// Load the schema
	loadedSchema, err := schema.LoadSchema(convertInput)
	if err != nil {
		log.Fatalf("Failed to load schema: %v", err)
	}

	// Convert to target format
	var outputData []byte
	switch convertTo {
	case "json":
		outputData, err = json.MarshalIndent(loadedSchema, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal JSON: %v", err)
		}

	case "sql":
		// Generate SQL DDL from schema
		driver := postgres.NewDriver() // Use PostgreSQL SQL generator
		var sqlBuilder strings.Builder

		for _, table := range loadedSchema.Tables {
			sql, _ := driver.CreateTable(table)
			sqlBuilder.WriteString(sql)
			sqlBuilder.WriteString(";\n\n")

			// Add indexes
			for _, idx := range table.Indexes {
				sql, _ := driver.AddIndex(table.Name, idx)
				sqlBuilder.WriteString(sql)
				sqlBuilder.WriteString(";\n")
			}

			if len(table.Indexes) > 0 {
				sqlBuilder.WriteString("\n")
			}

			// Add foreign keys
			for _, fk := range table.ForeignKeys {
				sql, _ := driver.AddForeignKey(table.Name, fk)
				if !strings.HasPrefix(sql, "--") { // Skip comment-only SQL
					sqlBuilder.WriteString(sql)
					sqlBuilder.WriteString(";\n")
				}
			}

			if len(table.ForeignKeys) > 0 {
				sqlBuilder.WriteString("\n")
			}
		}

		outputData = []byte(sqlBuilder.String())

	default:
		log.Fatalf("Unsupported output format: %s (use 'json' or 'sql')", convertTo)
	}

	// Write output
	if convertOutput == "" {
		// Write to stdout
		fmt.Print(string(outputData))
	} else {
		if err := os.WriteFile(convertOutput, outputData, 0644); err != nil {
			log.Fatalf("Failed to write output file: %v", err)
		}
		fmt.Printf("Converted %s to %s: %s\n", convertInput, convertTo, convertOutput)
	}
}
