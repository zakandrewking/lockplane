# SQLite Dialect Redesign Plan

## Context
Lockplane currently runs all schema parsing through the PostgreSQL `pg_query` parser, even when the source schema is SQLite. This normalizes SQLite column types and default expressions into PostgreSQL-specific representations (for example, `integer` becomes `pg_catalog.int4`, and `datetime('now')` is reinterpreted). The generated plans then fail when executed against SQLite shadow databases or targets, revealing a fundamental design gap in our dialect handling.

## Goals
- Preserve dialect-specific syntax (types, defaults, constraints) while still generating a common intermediate representation for diffing.
- Ensure migration plans emit SQL that matches the target dialect.
- Support shadow database validation flows for both PostgreSQL and SQLite, including cases where tables already exist.
- Add regression coverage so these issues stay fixed.

## Phase 1 Findings

### Parser Touchpoints
- `sql_parser.go:ParseSQLSchema` routes *all* `.lp.sql` schema loads and concatenated directory loads through `pg_query.Parse`, regardless of dialect, then normalizes types via `normalizePostgreSQLType`.
- `json_schema.go:LoadSQLSchema` → `loadSQLSchemaFromBytes` is the entry point used by CLI `plan`, `diff`, `apply`, and `rollback` whenever a schema path resolves to `.lp.sql`; this code path is triggered for SQLite schema dumps, so SQLite DDL hits the PostgreSQL parser today.
- `json_schema.go:LoadSchemaOrIntrospect` decides between file-based parsing and live introspection. Connection strings matching SQLite (e.g. `sqlite://`, `.db`, `:memory:`) short-circuit to the SQLite driver, bypassing `pg_query`; file inputs continue to use the PostgreSQL parser.
- Schema validation tooling (`validate_sql.go`, `validate_sql_safety.go`, `diagnostic/parser.go`) also imports `pg_query` to lint generated SQL prior to execution. These helpers inherit the same dialect assumptions.

### Schema Data Flow (SQLite vs PostgreSQL)
1. **Source detection** – CLI commands call `LoadSchemaOrIntrospect`. Inputs that look like SQLite connection strings use `database/sqlite.Introspector`, preserving type/default strings from `PRAGMA table_info`.
2. **File-based schemas** – Any `.lp.sql` file, including SQLite exports, is parsed via `ParseSQLSchema` which outputs `database.Schema` with PostgreSQL-normalized types (`integer` → `pg_catalog.int4`, `datetime('now')` → `DEFAULT` placeholder on failure).
3. **Diff/plan generation** – `DiffSchemas` compares `Schema` objects field-by-field. Mismatched type strings (`integer` vs `pg_catalog.int4`) or default literals produce diffs even when the logical column definition is identical, causing noisy or broken plans.
4. **SQL emission** – `GeneratePlanWithHash` selects the target driver (`detectDriver`). SQLite plans eventually call `database/sqlite.Generator`, which assumes incoming schema objects already contain SQLite-friendly type/default text. When upstream parsing rewrites types/defaults, emitted SQL becomes invalid for SQLite.
5. **Shadow DB validation** – `runApply` introspects the live target DB (returns SQLite-native schema) and compares it with the parsed desired schema (potentially PostgreSQL-normalized). Type drift at this stage blocks validation before migrations can run.

### Immediate Pain Points
- Type normalization rewrites `integer` → `pg_catalog.int4`, `text` → `text` (safe), `datetime` defaults → often collapse to `DEFAULT` placeholder because `formatExpr` cannot re-render SQLite expressions like `datetime('now')`.
- Default expressions such as `CURRENT_TIMESTAMP` or `datetime('now')` pass through `pg_query` as PostgreSQL AST nodes; our formatter returns `DEFAULT` when it doesn't recognise the node shape.
- Shadow DB migrations comparing introspected SQLite schema (human strings) against `.lp.sql` (pg-normalized) report false positives and fail safe-guards (e.g., Step 4 in the current failure report).

### SQLite Regression Fixtures & Syntax Requirements
- **Type preservation**
  - `completed integer NOT NULL DEFAULT 0` currently round-trips as `pg_catalog.int4` when parsed via `ParseSQLSchema` (see `sql_parser.go` pipeline). Shadow validation then sees a diff against the introspected `integer`.
  - `id BIGINT PRIMARY KEY` is emitted as `pg_catalog.int8` today (verified by `json_schema_test.go:57` expectations); we need the IR to store `bigint` or the original declaration (`BIGINT`) depending on strategy.
  - Collect additional affinity cases: `TEXT`, `BLOB`, `REAL`, `NUMERIC`, `BOOLEAN`, `UUID`, `DOUBLE PRECISION`.
- **Default expressions to support**
  - Bare keywords: `DEFAULT CURRENT_TIMESTAMP`, `DEFAULT CURRENT_DATE`, `DEFAULT CURRENT_TIME`.
  - Function-style expressions: `DEFAULT datetime('now')`, `DEFAULT (datetime('now'))`, `DEFAULT strftime('%s','now')`.
  - Literal defaults with/without parentheses: `DEFAULT 0`, `DEFAULT 'pending'`, `DEFAULT (0)`.
  - Generated columns (if/when enabled) should preserve expressions verbatim.
- **Constraint syntax**
  - `PRIMARY KEY AUTOINCREMENT` and multi-column primary keys (`PRIMARY KEY (id, tenant_id)`).
  - Inline `REFERENCES other_table(column) ON DELETE SET NULL` clauses (SQLite allows inline FK definitions).
  - Table-level `FOREIGN KEY` clauses and `CHECK` constraints should round-trip without rewriting into PostgreSQL variants.
- **Shadow DB prep**
  - `dryRunPlan` currently calls `applySchemaToDB` with the introspected schema. When desired schema types/defaults diverge (e.g., `pg_catalog.int4` vs `integer`), the generated shadow DDL fails to execute. The redesign must ensure the pre-shadow sync and migration steps share the same dialect-aware formatting.
- **Fixtures to add**
  1. Simple SQLite schema file with `INTEGER`, `TEXT`, `DEFAULT CURRENT_TIMESTAMP`, and `datetime('now')` defaults to reproduce failures.
  2. Existing-table shadow validation scenario where schema already contains rows/tables to ensure `applySchemaToDB` handles pre-existing objects gracefully.
  3. Diff regression covering `integer` vs `pg_catalog.int4` to lock behaviour.

## Phases

### 1. Scope and Diagnosis
- Audit the code paths that feed SQLite schema text into PostgreSQL parsing (`diff.go`, `planner.go`, schema introspection).
- Document schema data flow for each dialect and catalogue the currently failing fixtures (types, defaults, existing-table migrations).

### 2. Dialect Detection and Parser Abstraction
- Introduce explicit dialect metadata when loading schemas, with fallback inference if metadata is absent.
- Hide parser choice behind a new interface (e.g., `Parser.Parse(schema) -> LockplaneSchema`).
- Continue using `pg_query` for PostgreSQL; select or implement a dedicated SQLite parser (prefer an existing Go library such as `modernc.org/sqlite` rather than rolling a custom parser).

### 3. Intermediate Representation Normalization
- Define a dialect-neutral intermediate representation that preserves logical type names and default expressions verbatim unless we explicitly normalize them.
- For PostgreSQL parsing, add a controlled normalization layer mapping catalog types (`pg_catalog.int4`) to Lockplane logical types (`integer`).
- For SQLite parsing, avoid remapping types or defaults; keep `integer`, `CURRENT_TIMESTAMP`, `datetime('now')`, etc., intact.

### 4. Planner and Diff Updates
- Update schema diffing and migration planning to operate on the shared intermediate representation.
- Ensure type comparison uses logical names so dialect-specific aliases no longer cause false positives.
- Introduce dialect-specific SQL emitters for forward and rollback plans so SQLite plans stay SQLite-flavored while PostgreSQL plans remain PostgreSQL-flavored.

### 5. Shadow Database Strategy
- Extend shadow database setup to branch on dialect. SQLite should run against an ephemeral file or in-memory DB, while PostgreSQL retains the current Docker-based workflow.
- Ensure the SQLite shadow workflow handles pre-existing tables by cloning the production schema into the shadow DB before applying migrations.

### 6. Testing and Tooling
- Build parallel fixture suites covering both PostgreSQL and SQLite schemas for the same logical structures.
- Add regression tests for the reported failures: type preservation, `CURRENT_TIMESTAMP`, `datetime('now')`, and existing-table migrations.
- Update CI to run SQLite-focused integration tests (leveraging docker or in-memory sqlite3).
- Apply the standard checklist (`go fmt`, `go vet`, `go test`, docs) once the redesign lands.

## Next Decisions
1. Confirm the preferred SQLite parsing approach (external library versus sqlite3 CLI introspection).
2. Finalize the intermediate representation schema and wire it into `planner.go` and `diff.go`.
3. Prioritize the regression tests so they fail before implementation begins, guiding development.

## Phase 2 – Parser Abstraction Options

### Option A: Execute Schema in Ephemeral SQLite + Reuse Introspector
- **Approach**: Spin up an in-memory SQLite database (via `modernc.org/sqlite`, already in go.mod), execute each statement from the `.lp.sql`, then run the existing `database/sqlite.Introspector` to extract the schema.
- **Benefits**:
  - Leverages SQLite’s native parser and semantics—whatever SQLite accepts/normalizes is what we record.
  - No new dependency; `modernc.org/sqlite` is already vendored for drivers.
  - Keeps defaults/types exactly as SQLite reports via `PRAGMA table_info`, satisfying preservation requirements.
- **Drawbacks**:
  - Runs statements with side effects; we need to guard against unexpected DML/DDL (e.g., `DROP TABLE`) in schema files or enforce declarative-only subset before execution.
  - Error messaging mirrors SQLite’s raw diagnostics (less context than current pg_query enrichments).
  - Requires sandboxing to ensure unsafe statements don’t mutate the host; may need read-only connection pragmas.

### Option B: Embed a SQLite AST Parser Library
- **Candidates**: `github.com/akito0107/sqlbunny/sqlparser` (ANTLR-based), `github.com/sqlc-dev/sqlite-parser` (WIP), or regenerating ANTLR parser from SQLite grammar.
- **Benefits**:
  - Pure parsing—no execution side effects.
  - Gives us direct AST control for richer diagnostics and validation.
- **Drawbacks**:
  - Introduces or maintains a large dependency (ANTLR runtime + generated code) and ongoing maintenance when SQLite grammar evolves.
  - Most OSS parsers lag behind latest SQLite syntax; we’d need to verify coverage for default expressions, generated columns, etc.
  - Higher upfront engineering effort versus leveraging the engine we already ship.

### Option C: Shell Out to `sqlite3` CLI with `EXPLAIN`
- **Approach**: Feed statements to `sqlite3` (system binary) using `EXPLAIN` or `PRAGMA` to capture parse output.
- **Benefits**:
  - Relies on official binary; keeps parsing logic outside our codebase.
- **Drawbacks**:
  - Adds external dependency on local `sqlite3` executable (installation + portability issues).
  - Harder to sandbox/secure, especially for hosted or cross-platform environments.
  - Increased I/O overhead and error handling complexity.

### Recommendation
Proceed with **Option A** for the first iteration:
- Fastest path to dialect-correct schemas using tooling we already depend on.
- Keeps Lockplane’s schema IR in sync with how SQLite actually materializes tables.
- We can layer additional static validation (our `validate_sql_*` modules) before executing statements to block destructive operations.

Implementation sketch:
1. Extend `LoadSQLSchema` with dialect detection; when input marked SQLite, run statements against in-memory SQLite connection using a transaction that is rolled back after introspection.
2. Replace the resulting schema with the introspected `database.Schema` (no pg_query involvement).
3. Factor parsing behind an interface so Postgres path still uses `pg_query`, while SQLite path uses the execution+introspection workflow.
4. Update validation tooling to skip `pg_query` for SQLite files or provide dialect-aware linting (future work).

## Phase 2 – Intermediate Representation Draft

### Design Principles
- Preserve the *raw dialect strings* we ingest so we can regenerate byte-identical SQL when needed.
- Provide a *logical view* for diffing that normalizes equivalent constructs across dialects (`integer` vs `pg_catalog.int4` → logical `integer`).
- Keep the public `database.Schema` API stable as long as possible; introduce new fields gradually to avoid massive call-site churn.

### Proposed Struct Changes
```go
type Column struct {
    Name         string
    Type         LogicalType // replaces string
    Nullable     bool
    Default      *ColumnDefault
    IsPrimaryKey bool
    // Future: Constraints []ConstraintRef
}

type LogicalType struct {
    LogicalName string // e.g. "integer"
    Raw         string // exact text from source dialect ("pg_catalog.int4", "INTEGER PRIMARY KEY")
    Dialect     Dialect
}

type ColumnDefault struct {
    Raw     string   // e.g. "datetime('now')"
    Kind    DefaultKind // literal, function, expression
    Dialect Dialect
}

type Dialect string // "postgres", "sqlite", "generic"
```

Interim compatibility plan:
- Maintain `database.Column.Type` as a string for now, but add helper accessors to expose logical/raw names. Internally, wrap the string in a `LogicalType` while keeping JSON marshalling unchanged.
- Introduce lightweight helpers (`column.GetLogicalType()`) so `diff.go` and `planner.go` can compare logical names without touching raw text.
- For defaults, store both the raw expression and a logical classification to aid future constraint validation.

### Diff & Plan Adjustments
- Replace direct string comparisons (`current.Type != desired.Type`) with helper that compares `LogicalName`.
- Keep raw text available when emitting SQL: SQLite generator uses `Raw`, Postgres generator can re-normalize if needed.
- Extend schema hash computation to include both logical and raw forms to avoid collisions when dialect mapping changes.

### Dialect Metadata Propagation
- Enrich `Schema` with `Dialect Dialect` field at the root so downstream components know which generator/parser transformed it.
- When diffing schemas of different dialects (e.g., migrating from SQLite to PostgreSQL), require explicit conversion with future tooling rather than implicit comparison.

## Regression Test Plan

1. **Parser Fixture Roundtrip**
   - Add `testdata/fixtures/sqlite/basic.lp.sql` with `INTEGER`, `TEXT`, `DEFAULT CURRENT_TIMESTAMP`, `DEFAULT datetime('now')`.
   - New test: `TestLoadSchema_SQLitePreservesTypes` exercising the execution+introspection parser path to ensure `integer` persists and defaults stay verbatim.
2. **Diff Normalization**
   - Unit test in `diff_test.go` to confirm columns with logical type `integer` but raw text `pg_catalog.int4` do not register as modified when logical names match.
3. **Plan Generation**
   - Extend `database/sqlite/generator_test.go` (or new integration test) to verify column defaults and types from IR produce valid SQLite DDL (`DEFAULT CURRENT_TIMESTAMP`, parentheses preserved).
4. **Shadow Validation Flow**
   - Integration test in `main_test.go` simulating `lockplane apply` against SQLite target with pre-existing schema to ensure `dryRunPlan` succeeds (no type/default drift) and migrations apply cleanly.
5. **Validation Tooling**
   - Update `validate_sql_test.go` to cover dialect-aware linting (e.g., SQLite schema should not be fed to `pg_query`; expect friendly skip or SQLite-native error when syntax invalid).
6. **Schema Hash Compatibility**
   - Regression test covering schema hashing with logical/raw types to guarantee same logical schema yields stable hash across dialects.
