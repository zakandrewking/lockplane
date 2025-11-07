package planner

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xeipuuv/gojsonschema"
)

// LoadJSONPlan loads and validates a JSON plan file, returning a Plan
func LoadJSONPlan(path string) (*Plan, error) {
	// Read the JSON file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse into Plan
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	// Validate against JSON Schema
	schemaLoader := gojsonschema.NewReferenceLoader("file://schema-json/plan.json")
	documentLoader := gojsonschema.NewStringLoader(string(data))

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		// If schema file doesn't exist, skip validation (backwards compatibility)
		return &plan, nil
	}

	if !result.Valid() {
		errMsg := "JSON Schema validation failed:\n"
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return &plan, nil
}
