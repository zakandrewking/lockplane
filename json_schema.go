package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

// LoadSchema loads a schema from either JSON (.json) or SQL DDL (.lp.sql) file
func LoadSchema(path string) (*Schema, error) {
	ext := strings.ToLower(filepath.Ext(path))

	// Check for .lp.sql extension
	if ext == ".sql" && strings.HasSuffix(strings.ToLower(path), ".lp.sql") {
		return LoadSQLSchema(path)
	}

	// Otherwise assume JSON
	return LoadJSONSchema(path)
}

// LoadSQLSchema loads a schema from a SQL DDL file
func LoadSQLSchema(path string) (*Schema, error) {
	// Read the SQL file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read SQL file: %w", err)
	}

	// Parse SQL DDL
	schema, err := ParseSQLSchema(string(data))
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL DDL: %w", err)
	}

	// Convert from database.Schema to main.Schema (they're type aliases, so just cast)
	return (*Schema)(schema), nil
}

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
