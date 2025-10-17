# Lockplane — v0 Design Doc

## Summary
Lockplane is a thin, Postgres-first control plane that lets humans and AIs **plan, apply, and audit**
schema and data changes with **zero ambiguity**. The user API is an **MCP server** exposing a tiny set
of safe operators. Application traffic uses stock Postgres and optional PostgREST/PostGraphile APIs.

Lockplane ensures every database change is **deterministic, explainable, and reproducible** by
tracking a canonical catalog manifest, computing a cryptographic hash for each version, and
recording a ledger of all applied plans.

---

## Problem

- SQL migrations are often ambiguous and non-reproducible.
- AI or automation tools cannot safely reason about state or rollback.
- Developers want Firebase-like ease with relational guarantees.
- Users of tools like Lovable and V0 should not need to learn all the intricacies of database
  performance and safety

---

## Principles

- **Postgres is the source of truth** — no proprietary DB engine or forks.
- **Determinism everywhere** — each catalog state has a unique hash.
- **Human + AI co-editable** — DSL and APIs are machine-readable and human-readable.
- **Explainability first** — query plans, RLS policies, and changes can all be inspected.

---

## Architecture Overview
- **Postgres Core**: actual data store and single source of truth.
- **Lockplane Core**: introspector, planner/compiler, executor, and ledger.
- **MCP Server**: exposes Lockplane tools for AI and automation.
- **DSL Engine**: parses and serializes schema definitions.
- **Optional Gateway**: injects `app.user_id` and `app.tenant_id` for RLS contexts.

```text
          +--------------------------+
          |        MCP Client        |
          +-----------+--------------+
                      |
                      v
          +--------------------------+
          |   Lockplane MCP Server   |
          +-----------+--------------+
                      |
                      v
          +--------------------------+
          |     Lockplane Core       |
          | (Introspect, Plan, Exec) |
          +-----------+--------------+
                      |
                      v
              +---------------+
              |   Postgres    |
              +---------------+
```
---

## Implementation Status

**Completed (M1 Foundation):**
- ✅ Schema introspector - reads Postgres catalog, outputs JSON
- ✅ Plan structure - migration steps with SQL and descriptions
- ✅ Transactional executor - applies plans atomically
- ✅ Shadow DB validation - dry-run testing before production
- ✅ Golden test suite - fixture-based testing framework
- ✅ Docker Compose setup - main and shadow databases

**In Progress:**
- DSL format for defining desired schema state
- Diff engine to compare desired vs actual
- Plan generator to auto-create migrations
- Rollback plan generation

**Planned:**
- MCP server interface
- Catalog hash computation and ledger
- Durable execution for long-running operations
- pgroll integration for zero-downtime migrations

---

## Components

### Introspector
Reads current schema from Postgres `information_schema` and `pg_catalog`:
- Tables, columns (types, nullability, defaults, primary keys)
- Indexes (name, unique flag)
- Outputs canonical JSON representation

### Executor
Applies migration plans with safety guarantees:
- Runs all steps in a single transaction
- Shadow DB dry-run validation (if provided)
- Detailed error reporting with step-level failures
- Automatic rollback on any failure

### Plan Structure
JSON format defining migration steps:
```json
{
  "steps": [
    {
      "description": "Create users table",
      "sql": "CREATE TABLE users (id SERIAL PRIMARY KEY, email TEXT)"
    }
  ]
}
```

---

## Migration Strategy

**Current: Transactional (simple dev)**
All DDL operations run in a single transaction. Best for development and simple schemas. Limitations:
- Some operations don't support transactions (concurrent index creation)
- Table locks during DDL can block reads/writes

**Future: pgroll (zero-downtime)**
Dual-schema approach for production deployments:
1. Create new schema version alongside old
2. Deploy application pointing to new schema
3. Remove old schema after verification

---

## Safety Model

**Shadow DB Validation**
Before applying to production, every plan runs on a shadow database:
- Catches syntax errors, constraint violations, type mismatches
- Tests operations without risk
- Always rolls back shadow changes

**Transactional Execution**
All migration steps succeed or all roll back:
- No partial migrations
- No inconsistent state
- Failed migrations leave database unchanged

**Explainability**
Every plan shows exactly what will happen:
- SQL that will execute
- Order of operations
- Expected outcome

**Future Safety Features:**
- Strict SQL allow-list (via parser)
- Advisory lock per apply
- Two-phase confirmation for destructive ops
- Durable execution with timeouts, retries, progress tracking


