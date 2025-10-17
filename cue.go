package main

import (
	"encoding/json"
	"fmt"
	"os"

	"cuelang.org/go/cue"
	"cuelang.org/go/cue/cuecontext"
	"cuelang.org/go/cue/load"
)

// LoadCUESchema loads and validates a CUE schema file, returning a Schema
func LoadCUESchema(path string) (*Schema, error) {
	ctx := cuecontext.New()

	// Load the CUE files
	instances := load.Instances([]string{path}, nil)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no CUE instances found")
	}

	inst := instances[0]
	if inst.Err != nil {
		return nil, fmt.Errorf("failed to load CUE: %w", inst.Err)
	}

	// Build the instance
	value := ctx.BuildInstance(inst)
	if value.Err() != nil {
		return nil, fmt.Errorf("failed to build CUE: %w", value.Err())
	}

	// Validate
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("CUE validation failed: %w", err)
	}

	// Export to JSON
	jsonBytes, err := value.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to export CUE to JSON: %w", err)
	}

	// Parse into Schema
	var schema Schema
	if err := json.Unmarshal(jsonBytes, &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema JSON: %w", err)
	}

	return &schema, nil
}

// ValidateCUESchema validates a CUE file without loading it
func ValidateCUESchema(path string) error {
	ctx := cuecontext.New()

	instances := load.Instances([]string{path}, nil)
	if len(instances) == 0 {
		return fmt.Errorf("no CUE instances found")
	}

	inst := instances[0]
	if inst.Err != nil {
		return fmt.Errorf("failed to load CUE: %w", inst.Err)
	}

	value := ctx.BuildInstance(inst)
	if value.Err() != nil {
		return fmt.Errorf("failed to build CUE: %w", value.Err())
	}

	if err := value.Validate(cue.Concrete(true)); err != nil {
		return fmt.Errorf("CUE validation failed: %w", err)
	}

	return nil
}

// ExportCUEToJSON exports a CUE schema to JSON format
func ExportCUEToJSON(cuePath string, jsonPath string) error {
	schema, err := LoadCUESchema(cuePath)
	if err != nil {
		return err
	}

	jsonBytes, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	if err := os.WriteFile(jsonPath, jsonBytes, 0644); err != nil {
		return fmt.Errorf("failed to write JSON file: %w", err)
	}

	return nil
}

// LoadCUEPlan loads and validates a CUE plan file, returning a Plan
func LoadCUEPlan(path string) (*Plan, error) {
	ctx := cuecontext.New()

	// Load the CUE files
	instances := load.Instances([]string{path}, nil)
	if len(instances) == 0 {
		return nil, fmt.Errorf("no CUE instances found")
	}

	inst := instances[0]
	if inst.Err != nil {
		return nil, fmt.Errorf("failed to load CUE: %w", inst.Err)
	}

	// Build the instance
	value := ctx.BuildInstance(inst)
	if value.Err() != nil {
		return nil, fmt.Errorf("failed to build CUE: %w", value.Err())
	}

	// Validate
	if err := value.Validate(cue.Concrete(true)); err != nil {
		return nil, fmt.Errorf("CUE validation failed: %w", err)
	}

	// Export to JSON
	jsonBytes, err := value.MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to export CUE to JSON: %w", err)
	}

	// Parse into Plan
	var plan Plan
	if err := json.Unmarshal(jsonBytes, &plan); err != nil {
		return nil, fmt.Errorf("failed to parse plan JSON: %w", err)
	}

	return &plan, nil
}
