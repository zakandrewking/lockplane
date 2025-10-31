---
name: lockplane
description: Use Lockplane for safe database schema management - define schemas in .lp.sql files, validate, and apply with shadow DB testing
---

# Lockplane Expert

Help users manage database schemas safely using Lockplane.

## What is Lockplane?

Lockplane tests migrations on a shadow database before applying to production, validates SQL for dangerous patterns, and works with PostgreSQL and SQLite.

## Core Workflow

1. **Create schema** - Write `.lp.sql` files with CREATE TABLE statements
2. **Validate** - `lockplane validate sql schema/`
3. **Apply** - `lockplane apply --auto-approve --target $DATABASE_URL --schema schema/`

## Commands

### Validate schema
```bash
lockplane validate sql schema/users.lp.sql
lockplane validate sql schema/  # validate entire directory
```

### Apply changes
```bash
# Set database connection
export DATABASE_URL="postgresql://user:password@localhost:5432/myapp?sslmode=disable"

# Apply changes (tests on shadow DB first)
lockplane apply --auto-approve --target $DATABASE_URL --schema schema/
```

## Configuration

```bash
export DATABASE_URL="postgresql://user:password@localhost:5432/myapp?sslmode=disable"
export SHADOW_DATABASE_URL="postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable"
```

Or use CLI flags: `--target` and `--shadow-db`

## Schema Format

Standard PostgreSQL DDL in `.lp.sql` files:

```sql
-- schema/users.lp.sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX users_email_idx ON users(email);
```

**Requirements:**
- Standard SQL DDL only (CREATE TABLE, CREATE INDEX, ALTER TABLE)
- No destructive operations (DROP TABLE, DROP COLUMN, TRUNCATE)
- No IF NOT EXISTS or CREATE OR REPLACE
- No transaction control (BEGIN/COMMIT/ROLLBACK)

## Safety Validations

**Data loss operations (ERROR):**
- DROP TABLE, DROP COLUMN, TRUNCATE TABLE

**Non-declarative patterns (ERROR):**
- IF NOT EXISTS clauses
- Transaction control (BEGIN, COMMIT, ROLLBACK)
- CREATE OR REPLACE

**Blocking operations (WARNING):**
- CREATE INDEX without CONCURRENTLY

## Example Interaction

**User: "I need to add an email column to my users table"**

Response:
```
I'll help you add an email column. Update your schema file:

```sql
-- schema/users.lp.sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  name TEXT NOT NULL,
  email TEXT,  -- Add this
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

Then validate and apply:
```bash
lockplane validate sql schema/users.lp.sql
lockplane apply --auto-approve --target $DATABASE_URL --schema schema/
```

Note: I made email nullable because adding a NOT NULL column to an existing table with data would fail. To make it NOT NULL:
1. Add as nullable first
2. Backfill data in your application
3. Add NOT NULL constraint in a second migration
```

## Best Practices

1. Always validate before applying
2. Use shadow DB validation (lockplane apply does this by default)
3. Keep schema files in version control
4. Use CREATE INDEX CONCURRENTLY for production databases
5. Test locally before production
