package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/xeipuuv/gojsonschema"
)

// LoadJSONSchema loads and validates a JSON schema file, returning a Schema
func LoadJSONSchema(path string) (*Schema, error) {
	// Read the JSON file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read JSON file: %w", err)
	}

	// Parse into Schema
	var schema Schema
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	// Validate against JSON Schema
	schemaLoader := gojsonschema.NewReferenceLoader("file://schema-json/schema.json")
	documentLoader := gojsonschema.NewStringLoader(string(data))

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		// If schema file doesn't exist, skip validation (backwards compatibility)
		return &schema, nil
	}

	if !result.Valid() {
		errMsg := "JSON Schema validation failed:\n"
		for _, desc := range result.Errors() {
			errMsg += fmt.Sprintf("- %s\n", desc)
		}
		return nil, fmt.Errorf("%s", errMsg)
	}

	return &schema, nil
}

// ValidateJSONSchema validates a JSON file without loading it
func ValidateJSONSchema(path string) error {
	_, err := LoadJSONSchema(path)
	return err
}

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
