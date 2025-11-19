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
	// Source location metadata (optional, for error reporting)
	SourceFile string `json:"source_file,omitempty"` // Original file where this step was defined
	SourceLine int    `json:"source_line,omitempty"` // Line number in the source file
	// Lock analysis metadata (optional, for impact reporting)
	LockMode     string `json:"lock_mode,omitempty"`     // PostgreSQL lock mode (e.g., "ACCESS EXCLUSIVE")
	LockImpact   string `json:"lock_impact,omitempty"`   // Human-readable impact description
	BlocksReads  bool   `json:"blocks_reads,omitempty"`  // Whether this blocks SELECT queries
	BlocksWrites bool   `json:"blocks_writes,omitempty"` // Whether this blocks INSERT/UPDATE/DELETE
	Rewritable   bool   `json:"rewritable,omitempty"`    // Whether this can be rewritten to be lock-safe
}

// ExecutionResult tracks the outcome of executing a plan
type ExecutionResult struct {
	Success      bool     `json:"success"`
	StepsApplied int      `json:"steps_applied"`
	Errors       []string `json:"errors,omitempty"`
}

// MultiPhasePlan represents a migration requiring multiple coordinated phases
// Each phase contains a regular Plan that can be validated and executed independently
type MultiPhasePlan struct {
	MultiPhase  bool     `json:"multi_phase"`           // Always true for multi-phase plans
	Operation   string   `json:"operation"`             // e.g., "rename_column", "drop_column", "alter_type"
	Description string   `json:"description"`           // Human-readable description
	Pattern     string   `json:"pattern"`               // Pattern used: expand_contract, deprecation, validation, type_change
	TotalPhases int      `json:"total_phases"`          // Number of phases
	Phases      []Phase  `json:"phases"`                // The phases
	SafetyNotes []string `json:"safety_notes"`          // Important safety information
	CreatedAt   string   `json:"created_at,omitempty"`  // When this plan was created
	SchemaPath  string   `json:"schema_path,omitempty"` // Path to desired schema file
}

// Phase represents a single phase in a multi-phase migration
type Phase struct {
	PhaseNumber         int            `json:"phase_number"`                    // 1-indexed phase number
	Name                string         `json:"name"`                            // Phase name: expand, migrate_reads, contract, etc.
	Description         string         `json:"description"`                     // What this phase does
	RequiresCodeDeploy  bool           `json:"requires_code_deploy"`            // Must deploy code changes
	DependsOnPhase      int            `json:"depends_on_phase,omitempty"`      // Previous phase that must complete first
	CodeChangesRequired []string       `json:"code_changes_required,omitempty"` // List of code changes needed
	Plan                *Plan          `json:"plan"`                            // The actual migration plan for this phase
	Verification        []string       `json:"verification"`                    // Steps to verify phase completion
	Rollback            *PhaseRollback `json:"rollback"`                        // How to rollback this phase
	EstimatedDuration   string         `json:"estimated_duration,omitempty"`    // Estimated time to run
	LockImpact          string         `json:"lock_impact,omitempty"`           // Lock impact description
}

// PhaseRollback describes how to rollback a phase
type PhaseRollback struct {
	Description  string   `json:"description"`             // Description of rollback
	SQL          []string `json:"sql,omitempty"`           // SQL statements for rollback
	Note         string   `json:"note,omitempty"`          // Additional notes
	Warning      string   `json:"warning,omitempty"`       // Warnings about rollback
	RequiresCode bool     `json:"requires_code,omitempty"` // Rollback requires code changes
}
