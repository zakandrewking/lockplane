# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

## Why Lockplane?

**Shadow DB validation catches problems before production.** Most tools roll back after failure. Lockplane tests migrations on a shadow database first, so bad plans never touch your real data. *(Implemented)*

**Every change is explainable.** See exactly what SQL runs, in what order, with clear descriptions. *(Implemented)*

**Rollbacks will be generated and validated, not manually written.** For every forward migration, Lockplane will compute the reverse operation and validate it works. *(Planned)*

**Long-running operations will execute durably.** Building an index on 100M rows? Backfilling a column? Lockplane will handle timeouts, retries, and progress tracking so operations complete even if connections drop. *(Planned)*

## Quick Start

### Prerequisites
- Go 1.24+
- Docker & Docker Compose

### Setup

1. Start Postgres:
```bash
docker compose up -d
```

2. Run the introspector:
```bash
go run main.go
```

The introspector will connect to Postgres and output the current schema as JSON.

### Example: Create a test schema

```bash
docker compose exec pg psql -U lockplane -d lockplane -c "
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT NOW()
);
"
```

Then run the introspector again to see the schema:
```bash
go run main.go
```

## Migration Executor

Lockplane includes a transactional migration executor that safely applies schema changes.

### Plan Format

Migration plans are JSON files with a series of SQL steps:

```json
{
  "steps": [
    {
      "description": "Create posts table",
      "sql": "CREATE TABLE posts (id SERIAL PRIMARY KEY, title TEXT NOT NULL)"
    },
    {
      "description": "Add index on title",
      "sql": "CREATE INDEX idx_posts_title ON posts(title)"
    }
  ]
}
```

See example plans in `testdata/plans/`:
- `create_table.json` - Create a new table
- `add_column.json` - Add columns with constraints

### Using the Executor

The executor provides:
- **Transactional execution** - All steps succeed or all roll back
- **Shadow DB validation** - Test migrations before applying to main DB
- **Error tracking** - Detailed failure reporting

Example usage in Go:

```go
// Load migration plan
planBytes, _ := os.ReadFile("testdata/plans/create_table.json")
var plan Plan
json.Unmarshal(planBytes, &plan)

// Apply with shadow DB validation
shadowDB, _ := sql.Open("postgres", shadowConnStr)
result, err := applyPlan(ctx, mainDB, &plan, shadowDB)

if result.Success {
    fmt.Printf("Applied %d steps successfully\n", result.StepsApplied)
} else {
    fmt.Printf("Failed: %v\n", result.Errors)
}
```

### Testing Migrations

Run the test suite to see the executor in action:

```bash
go test -v -run TestApplyPlan
```

### Environment Variables

- `DATABASE_URL` - Main Postgres connection string (default: `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable`)
- `SHADOW_DATABASE_URL` - Shadow DB for dry-run validation (default: `postgres://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable`)

## Project Status

Currently implementing M1 (DSL & Planner). See `0001-design.md` for full design.

**Completed (M1 Foundation):**
- ✅ Schema introspector with JSON output
- ✅ Shadow DB setup for dry-run validation
- ✅ Transactional migration executor
- ✅ Golden test suite with fixtures

**In Progress:**
- DSL format for defining desired schema
- Diff engine to compare DSL vs live schema
- Plan generator to auto-create migration plans

**Planned:**
- Automatic rollback plan generation
- Durable execution for long-running operations
- MCP server interface for AI agents
- Catalog hash computation and ledger
- pgroll integration for zero-downtime migrations

## Current Limitations

**Manual plan creation:** You write migration plans as JSON with SQL steps. The DSL and plan generator are coming soon.

**No automatic rollbacks:** Transactions roll back on failure, but reverse migrations aren't generated yet.

**Introspector gaps:** Currently captures tables, columns, and indexes. Foreign keys, check constraints, and full index column parsing are not yet implemented.

**Development-only migrations:** All DDL runs in a transaction. Zero-downtime migrations with pgroll are planned for production use.
