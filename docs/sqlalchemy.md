# Using Lockplane with SQLAlchemy

This guide shows you how to use Lockplane with SQLAlchemy, keeping your ORM models as the source of truth for your database schema.

## Overview

With SQLAlchemy, your ORM models define your desired database schema. Lockplane helps you safely migrate your database to match your models:

- **Declarative workflow**: Generate SQL DDL from your SQLAlchemy models
- **Shadow DB testing**: Validate migrations before touching production
- **Automatic rollback generation**: Every migration is reversible
- **Intelligent error reporting**: Get precise line numbers and helpful messages
- **No manual migration scripts**: Lockplane figures out the steps

## Prerequisites

- SQLAlchemy application with PostgreSQL
- Lockplane CLI installed (`npx lockplane` or `go install github.com/lockplane/lockplane@latest`)
- Docker (optional, for local shadow database)

## Quick Start

### 1. Initialize Lockplane

```bash
lockplane init
```

This interactive wizard will:
- Create `lockplane.toml` configuration
- Set up `.env.local` with database credentials
- Auto-configure a shadow database for safe testing
- Create a `schema/` directory for your SQL DDL files

Example `lockplane.toml`:

```toml
default_environment = "local"

[environments.local]
description = "Local development database"
schema_path = "schema/"
```

Example `.env.local` (auto-configured):

```bash
# Primary database
DATABASE_URL=postgresql://user:password@localhost:5432/myapp?sslmode=disable

# Shadow database (auto-configured by lockplane init)
SHADOW_DATABASE_URL=postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable
```

## Workflow: SQLAlchemy Models → Lockplane

### Your SQLAlchemy Models (Source of Truth)

```python
# models.py
from sqlalchemy import Column, Integer, String, Boolean, DateTime, ForeignKey, Text
from sqlalchemy.orm import declarative_base, relationship
from sqlalchemy.sql import func

Base = declarative_base()

class User(Base):
    __tablename__ = 'users'

    id = Column(Integer, primary_key=True)
    email = Column(String(255), nullable=False, unique=True)
    name = Column(String(255), nullable=False)
    created_at = Column(DateTime, server_default=func.now())
    is_active = Column(Boolean, default=True)

    posts = relationship("Post", back_populates="user")

class Post(Base):
    __tablename__ = 'posts'

    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey('users.id', ondelete='CASCADE'), nullable=False)
    title = Column(String(255), nullable=False)
    content = Column(Text)
    published_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, server_default=func.now())

    user = relationship("User", back_populates="posts")
```

### Step 1: Generate SQL DDL from SQLAlchemy

Create a script to export your SQLAlchemy models as SQL DDL:

```python
# generate_schema.py
from sqlalchemy import create_engine
from sqlalchemy.schema import CreateTable, CreateIndex
from models import Base
from sqlalchemy.dialects import postgresql

def generate_schema_sql(output_file='schema/schema.lp.sql'):
    """Generate SQL DDL from SQLAlchemy models"""

    with open(output_file, 'w') as f:
        # Write a header comment
        f.write("-- Auto-generated from SQLAlchemy models\n")
        f.write("-- DO NOT EDIT - Regenerate with: python generate_schema.py\n\n")

        # Generate CREATE TABLE statements
        for table in Base.metadata.sorted_tables:
            # Create table
            create_table = CreateTable(table).compile(dialect=postgresql.dialect())
            f.write(str(create_table))
            f.write(";\n\n")

            # Create indexes (that aren't part of constraints)
            for index in table.indexes:
                if not index.unique:  # Unique indexes are created with the table
                    create_index = CreateIndex(index).compile(dialect=postgresql.dialect())
                    f.write(str(create_index))
                    f.write(";\n\n")

    print(f"✓ Generated {output_file} from SQLAlchemy models")

if __name__ == "__main__":
    generate_schema_sql()
```

Run it:

```bash
python generate_schema.py
```

This creates `schema/schema.lp.sql`:

```sql
-- Auto-generated from SQLAlchemy models
-- DO NOT EDIT - Regenerate with: python generate_schema.py

CREATE TABLE users (
  id SERIAL NOT NULL,
  email VARCHAR(255) NOT NULL,
  name VARCHAR(255) NOT NULL,
  created_at TIMESTAMP DEFAULT now(),
  is_active BOOLEAN DEFAULT true,
  PRIMARY KEY (id),
  UNIQUE (email)
);

CREATE TABLE posts (
  id SERIAL NOT NULL,
  user_id INTEGER NOT NULL,
  title VARCHAR(255) NOT NULL,
  content TEXT,
  published_at TIMESTAMP,
  created_at TIMESTAMP DEFAULT now(),
  PRIMARY KEY (id),
  FOREIGN KEY(user_id) REFERENCES users (id) ON DELETE CASCADE
);
```

**Important**: Use CREATE TABLE statements only. Do **not** include ALTER TABLE statements in your schema files:

```python
# ❌ DON'T DO THIS:
f.write("CREATE TABLE users (id INT);\n")
f.write("ALTER TABLE users ADD COLUMN email TEXT;\n")

# ✅ DO THIS:
f.write("CREATE TABLE users (\n  id INT,\n  email TEXT\n);\n")
```

Lockplane treats schema files as **declarative** (describing the desired end state), not imperative (describing migration steps). If you include ALTER TABLE, lockplane will warn you and merge it into the CREATE TABLE definition.

### Step 2: Validate Your Schema

Before generating a migration plan, validate your SQL syntax:

```bash
lockplane plan --check-schema schema/
```

This runs a fast syntax check and tests your schema on a clean shadow database. You'll get helpful error messages if there are issues:

```json
{
  "diagnostics": [{
    "code": "syntax_error",
    "file": "schema/schema.lp.sql",
    "line": 15,
    "column": 23,
    "message": "trailing comma not allowed here",
    "severity": "error"
  }]
}
```

### Step 3: Generate Migration Plan

Generate a plan to migrate from your current database to the desired schema:

```bash
lockplane plan --from-environment local --to schema/ > migration.json
```

Lockplane will:
- Introspect your current database automatically
- Compare it to your desired schema
- Generate SQL migration steps
- Output a migration plan in JSON format

Review the plan:

```bash
cat migration.json
```

Example output:

```json
{
  "source_hash": "a1b2c3d4...",
  "steps": [
    {
      "description": "Add column is_active to table users",
      "sql": ["ALTER TABLE users ADD COLUMN is_active BOOLEAN DEFAULT true"]
    },
    {
      "description": "Create index idx_posts_published_at on table posts",
      "sql": ["CREATE INDEX idx_posts_published_at ON posts (published_at)"]
    }
  ]
}
```

### Step 4: Apply Migration (with Shadow DB Testing)

```bash
lockplane apply
```

Lockplane will:
1. **Auto-detect** the `schema/` directory
2. **Introspect** your target database
3. **Generate** a migration plan
4. **Test** the migration on a shadow database first
5. **Show you** the plan and ask for confirmation
6. **Apply** the migration (only if shadow testing succeeds)
7. **Rollback** automatically if anything fails

**Interactive prompt:**

```
ℹ️ Auto-detected schema directory: supabase/schema/
🔍 Introspecting target database (local)...
📖 Loading desired schema from schema...
📋 Migration plan (2 steps):
  1. Add column is_active to table users
     SQL: ALTER TABLE users ADD COLUMN is_active BOOLEAN DEFAULT true
  2. Create index idx_posts_published_at on table posts
     SQL: CREATE INDEX idx_posts_published_at ON posts (published_at)

Do you want to perform these actions?
  Lockplane will perform the actions described above.
  Only 'yes' will be accepted to approve.
  Enter a value:
```

For CI/CD or development, auto-approve:

```bash
lockplane apply --auto-approve
```

## Development Workflow

### Daily Development Loop

```bash
# 1. Make changes to your SQLAlchemy models
vim models.py

# 2. Regenerate SQL DDL
python generate_schema.py

# 3. Validate syntax (fast, no database needed for syntax check)
lockplane plan --check-schema schema/

# 4. Apply to local database (with shadow DB testing)
lockplane apply --auto-approve
```

**Pro tip**: Add this to your Makefile:

```makefile
.PHONY: schema migrate

schema:
	@python generate_schema.py
	@lockplane plan --check-schema schema/

migrate: schema
	@lockplane apply --auto-approve
```

Then just run:

```bash
make migrate
```

## Production Deployments

### Generate Migration Plan for Review

```bash
# 1. Generate schema from models
python generate_schema.py

# 2. Create migration plan (against production database)
lockplane plan --from-environment production --to schema/ > migration.json

# 3. Review the plan
cat migration.json

# 4. Generate rollback plan
lockplane plan-rollback --plan migration.json --from-environment production > rollback.json

# 5. Commit both plans for audit trail
git add migration.json rollback.json
git commit -m "chore: migration plan for v2.1.0"
```

### Apply in Production

```bash
# Apply migration (tests on shadow DB first)
lockplane apply migration.json --target-environment production

# If something goes wrong, rollback:
lockplane apply rollback.json --target-environment production
```

## CI/CD Integration

### GitHub Actions Example

```yaml
name: Database Migration

on:
  push:
    branches: [main]

jobs:
  migrate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: pip install sqlalchemy psycopg2-binary

      - name: Install Lockplane
        run: npm install -g lockplane

      - name: Configure Lockplane environment
        run: |
          cat <<'EOF' > .env.production
          DATABASE_URL=${{ secrets.DATABASE_URL }}
          SHADOW_DATABASE_URL=${{ secrets.SHADOW_DATABASE_URL }}
          EOF

      - name: Generate desired schema from SQLAlchemy models
        run: python generate_schema.py

      - name: Validate schema syntax
        run: lockplane plan --check-schema schema/

      - name: Generate migration plan
        run: lockplane plan --from-environment production --to schema/ > migration.json

      - name: Generate rollback plan
        run: lockplane plan-rollback --plan migration.json --from-environment production > rollback.json

      - name: Upload rollback artifact
        uses: actions/upload-artifact@v4
        with:
          name: rollback-plan
          path: rollback.json

      - name: Apply migration (with shadow DB testing)
        run: lockplane apply migration.json --target-environment production
```

## Alternative: JSON Schema Format

If you prefer JSON over SQL DDL, you can introspect from a shadow database:

```python
# generate_schema.py (JSON approach)
from sqlalchemy import create_engine
from models import Base
import subprocess

# Connect to shadow database
engine = create_engine('postgresql://user:password@localhost:5433/myapp_shadow')

# Create all tables
Base.metadata.drop_all(engine)
Base.metadata.create_all(engine)

# Introspect to JSON
subprocess.run([
    'lockplane', 'introspect',
    '--db', 'postgresql://user:password@localhost:5433/myapp_shadow',
    '--output', 'schema.json'
])

print("✓ Generated schema.json from SQLAlchemy models")
```

Then use `schema.json` instead of `schema/` in lockplane commands.

**Recommendation**: Prefer SQL DDL (`.lp.sql`) files for better readability, easier diffs in git, and better IDE integration.

## Best Practices

### 1. Keep Models as Source of Truth

✅ **DO**: Update SQLAlchemy models, then regenerate schema

```bash
vim models.py              # Make changes
python generate_schema.py  # Regenerate SQL DDL
lockplane apply           # Apply changes
```

❌ **DON'T**: Manually edit schema files

```bash
vim schema/schema.lp.sql  # Don't do this!
```

### 2. Use CREATE TABLE Only

✅ **DO**: Write complete CREATE TABLE statements

```sql
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  email VARCHAR(255) NOT NULL,
  name VARCHAR(255) NOT NULL
);
```

❌ **DON'T**: Use ALTER TABLE in schema files

```sql
CREATE TABLE users (id SERIAL PRIMARY KEY);
ALTER TABLE users ADD COLUMN email VARCHAR(255);  -- Lockplane will warn!
```

### 3. Validate Before Applying

Always run `--check-schema` before applying:

```bash
# Fast syntax validation
lockplane plan --check-schema schema/

# Then apply
lockplane apply
```

### 4. Commit Schema Files (Optional)

For audit trail, commit generated schema files:

```bash
git add schema/schema.lp.sql
git commit -m "chore: update schema for user profile feature"
```

Add this reminder to your generation script:

```python
print(f"✓ Generated {output_file} from SQLAlchemy models")
print("  Remember to commit: git add schema/ && git commit -m 'chore: update schema'")
```

### 5. Test Migrations in Shadow DB

Never skip shadow DB testing. If `--validate` or shadow DB testing fails, **do not apply to production**.

```bash
# Shadow DB testing is automatic with apply
lockplane apply  # Tests on shadow DB first

# For plan command, add --validate to test
lockplane plan --from-environment local --to schema/ --validate
```

## Common Patterns

### Adding a New Table

```python
# models.py
class Comment(Base):
    __tablename__ = 'comments'

    id = Column(Integer, primary_key=True)
    post_id = Column(Integer, ForeignKey('posts.id', ondelete='CASCADE'))
    user_id = Column(Integer, ForeignKey('users.id', ondelete='CASCADE'))
    content = Column(Text, nullable=False)
    created_at = Column(DateTime, server_default=func.now())
```

```bash
python generate_schema.py
lockplane plan --check-schema schema/
lockplane apply
```

### Adding a Column with Default

```python
# models.py
class User(Base):
    # ... existing columns ...
    verified_at = Column(DateTime, nullable=True)  # New column
```

Lockplane will generate:

```sql
ALTER TABLE users ADD COLUMN verified_at TIMESTAMP
```

### Renaming a Column (Two-Phase)

SQLAlchemy doesn't track renames, so this requires a multi-phase migration:

**Phase 1: Add new column**

```python
class User(Base):
    username = Column(String(255))  # Old
    display_name = Column(String(255))  # New
```

```bash
python generate_schema.py
lockplane apply  # Adds display_name column
```

**Phase 2: Copy data** (manual)

```python
# migrate_usernames.py
from sqlalchemy import create_engine
engine = create_engine('postgresql://...')
engine.execute("UPDATE users SET display_name = username WHERE display_name IS NULL")
```

**Phase 3: Remove old column**

```python
class User(Base):
    # username removed
    display_name = Column(String(255), nullable=False)
```

```bash
python generate_schema.py
lockplane apply  # Removes username column
```

## Troubleshooting

### "ALTER TABLE detected in schema file"

**Problem**: You have ALTER TABLE statements in your SQL DDL.

```
⚠️  Warning: ALTER TABLE users detected in schema file
   Lockplane treats schema files as declarative (desired end state).
   The ALTER TABLE will be merged into the CREATE TABLE definition.
   Recommendation: Use only CREATE TABLE statements with final desired columns.
```

**Solution**: Update your generate_schema.py to only output CREATE TABLE:

```python
# ✅ Correct
create_table = CreateTable(table).compile(dialect=postgresql.dialect())
f.write(str(create_table) + ';\n\n')

# ❌ Don't add ALTER TABLE
```

### "Trailing comma not allowed here"

**Problem**: SQL syntax error (common mistake).

```json
{
  "diagnostics": [{
    "file": "schema/schema.lp.sql",
    "line": 5,
    "column": 23,
    "message": "trailing comma not allowed here"
  }]
}
```

**Solution**: Fix the trailing comma:

```sql
-- ❌ Wrong
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255),  -- trailing comma!
);

-- ✅ Correct
CREATE TABLE users (
  id SERIAL PRIMARY KEY,
  name VARCHAR(255)   -- no comma
);
```

### "Type mismatch: expected text, got varchar"

**Problem**: SQLAlchemy `String` without length maps to TEXT, but your database has VARCHAR.

**Solution**: Be explicit with column types:

```python
# ❌ Ambiguous
email = Column(String)  # Becomes TEXT in PostgreSQL

# ✅ Explicit
from sqlalchemy import Text
email = Column(Text)  # or String(255) for VARCHAR(255)
```

### "relation already exists"

**Problem**: Runtime error during shadow DB testing.

```json
{
  "diagnostics": [{
    "code": "runtime_error",
    "file": "schema/schema.lp.sql",
    "line": 12,
    "message": "relation \"idx_users_email\" already exists"
  }]
}
```

**Solution**: Lockplane now shows you the exact line! Check for duplicate index definitions in your schema file.

### "Shadow DB validation failed"

**Problem**: Migration works in syntax check but fails when applied.

**Solution**: Run with `--verbose` to see details:

```bash
lockplane apply --verbose
```

Common issues:
- Trying to add NOT NULL column without a DEFAULT
- Type changes that would lose data
- Missing foreign key constraints

## Comparison with Alembic

| Feature | Lockplane | Alembic |
|---------|-----------|---------|
| Schema as code | ✅ SQLAlchemy models | ✅ SQLAlchemy models |
| Auto-generate DDL | ✅ Yes (via script) | ✅ Yes (built-in) |
| Migration files | ❌ Not needed | ✅ Python scripts |
| Shadow DB testing | ✅ Built-in | ❌ Manual |
| Auto rollback generation | ✅ Yes | ❌ Manual |
| Syntax validation | ✅ Pre-flight checks | ❌ Runtime only |
| Error line numbers | ✅ Precise location | ⚠️ Generic errors |
| Database support | PostgreSQL, SQLite | Postgres, MySQL, SQLite, others |
| AI-friendly | ✅ SQL DDL + JSON | ⚠️ Python scripts |

**Migration from Alembic**: See [Migrating from Alembic](alembic.md)

## Example Project Structure

```
myproject/
├── models.py                    # SQLAlchemy models (source of truth)
├── generate_schema.py           # Script to export SQL DDL
├── schema/
│   └── schema.lp.sql           # Generated SQL DDL (commit this)
├── lockplane.toml               # Lockplane configuration
├── .env.local                   # Database credentials (DO NOT commit)
├── .env.production              # Production credentials (DO NOT commit)
└── .gitignore                   # Include .env.*
```

**Recommended .gitignore**:

```gitignore
.env.*
!.env.example
*.pyc
__pycache__/
```

## Next Steps

- [Getting Started Guide](../README.md)
- [Migrating from Alembic](alembic.md)
- [Prisma Integration](prisma.md)
- [Supabase Integration](supabase.md)
