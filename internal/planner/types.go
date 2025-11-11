package planner

// Plan represents a migration plan with a series of steps
type Plan struct {
	SourceHash string     `json:"source_hash"`
	Steps      []PlanStep `json:"steps"`
}

// PlanStep represents a single logical migration operation
// that may consist of multiple SQL statements executed atomically
type PlanStep struct {
	Description string   `json:"description"`
	SQL         []string `json:"sql"` // Array of SQL statements to execute in order
}

// ExecutionResult tracks the outcome of executing a plan
type ExecutionResult struct {
	Success      bool     `json:"success"`
	StepsApplied int      `json:"steps_applied"`
	Errors       []string `json:"errors,omitempty"`
}
