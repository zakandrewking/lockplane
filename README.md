# Lockplane

A Postgres-first control plane for safe, AI-friendly schema management.

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

### Environment Variables

- `DATABASE_URL` - Postgres connection string (default: `postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable`)

## Project Status

Currently implementing M1 (DSL & Planner). See `0001-design.md` for full design.
