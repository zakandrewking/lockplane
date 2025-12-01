package cmd

import (
	"fmt"
	"log"

	"github.com/lockplane/lockplane/internal/schema"
	"github.com/spf13/cobra"
)

func init() {
	rootCmd.AddCommand(checkSchemaCmd)
}

var checkSchemaCmd = &cobra.Command{
	Use:   "check-schema",
	Short: "Check .lp.sql schema files for errors",
	Long:  "Check .lp.sql schema files for errors and print a JSON summary",
	Run:   runCheckSchema,
}

func runCheckSchema(cmd *cobra.Command, args []string) {
	reportJson, err := schema.CheckSchema()
	if err != nil {
		log.Fatalf("Failed to check schema: %v", err)
	}
	fmt.Print(reportJson)
}
