# Basic Migration Scenario

## Overview

Tests the fundamental Lockplane workflow for schema migrations without any AI assistance. This is a smoke test to ensure Lockplane's core functionality works correctly.

## What This Tests

1. **Database Introspection**: Can read current schema from Postgres
2. **Schema Definition**: Create simple `.lp.sql` schema files
3. **Migration Planning**: Generate migration plans from schema diffs
4. **Plan Validation**: Validate migrations are safe and reversible
5. **Migration Execution**: Apply migrations with shadow DB validation

## Workflow

```
Empty DB → Introspect → Define Schema → Plan → Validate → Apply
```

## Prerequisites

- PostgreSQL database running on localhost:5432
- Shadow database running on localhost:5433
- Or set `DATABASE_URL` and `SHADOW_DATABASE_URL` environment variables

## Running the Scenario

### Standalone

```bash
./scenario.py
```

### Via Eval Runner

```bash
cd ../..
scenarios/run-evals.py basic-migration
```

## Validation

```bash
./validate.py
```

Checks:
- ✅ Schema files were created (`current.json`, `schema.lp.sql`)
- ✅ Migration plan was generated (`migration.json`)
- ✅ Migration plan has steps
- ✅ Migration includes CREATE TABLE
- ✅ Database now contains the users table

## Expected Output

The scenario creates a simple `users` table with:
- `id` (BIGINT PRIMARY KEY)
- `email` (TEXT NOT NULL)
- `created_at` (TIMESTAMP WITH TIME ZONE)
- Unique index on `email`

## Success Criteria

- All commands execute successfully
- Migration plan contains 1-2 steps (CREATE TABLE + CREATE INDEX)
- Validation passes (safe, reversible operations)
- Database state matches desired schema after apply
