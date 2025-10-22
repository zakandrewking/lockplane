package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
)

// PlanValidationIssue represents a validation error or warning for plans
type PlanValidationIssue struct {
	File     string `json:"file"`
	Line     int    `json:"line"`
	Column   int    `json:"column"`
	Severity string `json:"severity"` // "error" or "warning"
	Message  string `json:"message"`
	Code     string `json:"code,omitempty"`
}

// PlanValidationResult contains all validation issues for plan files
type PlanValidationResult struct {
	Valid  bool                  `json:"valid"`
	Issues []PlanValidationIssue `json:"issues"`
}

func runValidatePlan(args []string) {
	fs := flag.NewFlagSet("validate plan", flag.ExitOnError)
	formatFlag := fs.String("format", "text", "Output format: text or json")

	// Custom usage function
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: lockplane validate plan [options] <file>\n\n")
		fmt.Fprintf(os.Stderr, "Validate a migration plan JSON file against the Lockplane plan schema.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  # Validate plan file (text output)\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate plan migration.json\n\n")
		fmt.Fprintf(os.Stderr, "  # Validate with JSON output\n")
		fmt.Fprintf(os.Stderr, "  lockplane validate plan --format json migration.json\n\n")
	}

	if err := fs.Parse(args); err != nil {
		log.Fatalf("Failed to parse flags: %v", err)
	}

	if fs.NArg() == 0 {
		fs.Usage()
		os.Exit(1)
	}

	path := fs.Arg(0)

	// Load and validate the plan
	_, err := LoadJSONPlan(path)
	if err != nil {
		if *formatFlag == "json" {
			// Output as JSON for programmatic consumption
			result := PlanValidationResult{
				Valid: false,
				Issues: []PlanValidationIssue{
					{
						File:     path,
						Line:     1,
						Column:   1,
						Severity: "error",
						Message:  err.Error(),
						Code:     "plan_validation_error",
					},
				},
			}
			jsonBytes, _ := json.MarshalIndent(result, "", "  ")
			fmt.Println(string(jsonBytes))
		} else {
			fmt.Fprintf(os.Stderr, "✗ Plan validation failed: %s\n\n", path)
			fmt.Fprintf(os.Stderr, "  %s\n", err.Error())
		}
		os.Exit(1)
	}

	// Plan is valid
	if *formatFlag == "json" {
		result := PlanValidationResult{
			Valid:  true,
			Issues: []PlanValidationIssue{},
		}
		jsonBytes, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			log.Fatalf("Failed to marshal validation result: %v", err)
		}
		fmt.Println(string(jsonBytes))
	} else {
		fmt.Fprintf(os.Stderr, "✓ Plan is valid: %s\n", path)
	}
}
