# Dialect Configuration via `lockplane.toml`

**Status**: 🚧 In Progress (2025-11-18)

**Combined with**: Multi-schema support (see `multi-schema-and-policies.md`)

## Goal
Move the per-file dialect declaration (currently a comment like `-- dialect: sqlite`) into a first-class option within `lockplane.toml`, making schema dialect selection explicit, tool-friendly, and easier to discover.

## Motivation
- Comments are invisible to tooling and easy to forget.
- Having dialect defined in config improves auto-complete, validation, and CLI behavior.
- Aligns with other Lockplane configuration moving into `lockplane.toml` / `.env`.
- Essential for multi-schema support (all schemas in a database share the same dialect).

## Decisions Made
1. **Scope:** Dialect is set per-environment (all schemas in one database have the same dialect)
2. **Backwards compatibility:** Use clear precedence order (see below) - no breaking changes
3. **CLI defaults:** Auto-detect from connection string if not specified (current behavior)

## Configuration Shape

```toml
[environments.local]
description = "Local development"
database_url = "postgresql://postgres:postgres@localhost:5432/mydb"
shadow_database_url = "postgresql://postgres:postgres@localhost:5432/mydb"
shadow_schema = "lockplane_shadow"
dialect = "postgres"              # NEW: Explicit dialect
schemas = ["public", "storage"]   # NEW: Multi-schema support
```

## Precedence Order (Most to Least Specific)

1. **CLI flag** (future: `--dialect postgres`)
2. **Inline file comment** (`-- dialect: sqlite` in schema file)
3. **Environment config** (`dialect = "postgres"` in `lockplane.toml`)
4. **Auto-detect** from connection string (current behavior)

**Rule**: Most specific wins. If conflict exists between layers, show warning.

## Implementation Plan

### Phase 1 – Schema Types ✅ (Done)
- [x] Add `Policy` type for RLS policies
- [x] Add `Schema` field to `Table` for multi-schema
- [x] Create design document

### Phase 2 – Config Changes ✅ (Done)
- [x] Add `dialect` field to `EnvironmentConfig`
- [x] Add `schemas` field to `EnvironmentConfig`
- [x] Add validation for dialect values ("postgres", "sqlite")
- [x] Update config resolution to use dialect from config

### Phase 3 – Schema Loading ✅ (Done)
- [x] Update schema loader to respect config dialect
- [x] Warn if inline comment conflicts with config dialect
- [x] Use config dialect as fallback before auto-detection
- [x] Update plan, rollback, and apply commands to use config dialect
- [x] Add tests for dialect precedence

### Phase 4 – Multi-Schema Introspection ✅ (Done)
- [x] Update introspection to query multiple schemas
- [x] Add schema name to introspected tables
- [x] Added `IntrospectSchemas` method to Driver interface
- [x] Implemented schema-specific Get methods (GetTablesInSchema, GetColumnsInSchema, etc.)
- [x] Created `LoadSchemaFromConnectionStringWithSchemas` function

### Phase 5 – Policy Support ✅ (Done)
- [x] Add policy introspection (GetPolicies, GetPoliciesInSchema)
- [x] Add policy DDL generation (CreatePolicy, DropPolicy, EnableRLS, DisableRLS)
- [x] Automatically introspect policies when RLS is enabled
- [ ] Update parser for CREATE POLICY (deferred to future phase)

### Phase 6 – Testing & Documentation
- [ ] Unit tests for config dialect resolution
- [ ] Integration tests with multi-schema
- [ ] Update README and getting started guide
- [ ] Add examples for Supabase (multi-schema use case)

## Risks
- Breaking existing comment-based workflows if precedence isn’t clearly defined.
- Introducing conflicting configuration (config vs comments vs CLI) without clear messaging.

## Next Steps
- Finalize the configuration schema and precedence rules.
- Prototype config parsing with feature flag to gather feedback.
