# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

## Why Lockplane?

**For AI agent builders:**
- AI can write SQL migrations but can't guarantee they're safe
- Lockplane makes schema changes deterministic and testable
- Every migration runs in a transaction and validates on shadow DB first
- Clear audit trail for every change

**For developers tired of migration anxiety:**
- Shadow database catches issues before production
- See exactly what SQL runs and in what order
- Atomic operations that succeed or roll back completely
- No surprises

**For teams wanting ease without limits:**
- Define desired schema state, not manual ALTER statements
- Keep Postgres power (joins, constraints, transactions)
- No vendor lock-in, runs on stock Postgres 14-16
- Works alongside existing tools

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

**Completed:**
- ✅ Schema introspector with JSON output
- ✅ Shadow DB setup for dry-run validation
- ✅ Transactional migration executor
- ✅ Golden test suite with fixtures

**In Progress:**
- DSL format for defining desired schema
- Diff engine to compare DSL vs live schema
- Plan generator to auto-create migration plans
