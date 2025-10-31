# Using Lockplane with SQLAlchemy

This guide shows you how to use Lockplane with SQLAlchemy, a popular Python ORM.

## Overview

With SQLAlchemy, your ORM models are the source of truth. Lockplane helps you safely migrate your database to match your models without manually writing migration scripts.

## Prerequisites

Before you begin, configure your database connections. You have three options:

**Option 1: Configuration File (Recommended)**

Create `lockplane.toml` in your project root:

```toml
# lockplane.toml
database_url = "postgresql://user:password@localhost:5432/myapp?sslmode=disable"
shadow_database_url = "postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable"
schema_path = "lockplane/schema/"
```

> **Important:** Add `?sslmode=disable` for local development databases. Remove it for production databases with SSL enabled.

**Option 2: Environment Variables**

```bash
# Production database (where migrations will be applied)
export DATABASE_URL="postgresql://user:password@localhost:5432/myapp?sslmode=disable"

# Shadow database (for testing migrations safely before applying to production)
export SHADOW_DATABASE_URL="postgresql://user:password@localhost:5433/myapp_shadow?sslmode=disable"
```

Add these to your shell profile (`~/.bashrc`, `~/.zshrc`, or `.env` file) to make them persistent.

**Option 3: CLI Flags**

```bash
lockplane apply migration.json \
  --target "postgresql://localhost:5432/myapp?sslmode=disable" \
  --shadow-db "postgresql://localhost:5433/myapp_shadow?sslmode=disable"
```

**Priority Order:** CLI flags > Environment variables > Config file > Defaults

**Important:** The `apply` command uses these settings to know where to execute migrations. Commands like `plan`, `diff`, and `introspect` can accept connection strings as arguments.

## Workflow

Instead of maintaining separate schema files, use SQLAlchemy's `create_all()` to generate your desired schema on demand:

```python
# models.py
from sqlalchemy import Column, Integer, String, Boolean, DateTime, ForeignKey, Text
from sqlalchemy.orm import declarative_base, relationship
from sqlalchemy.sql import func

Base = declarative_base()

class User(Base):
    __tablename__ = 'users'

    id = Column(Integer, primary_key=True)
    email = Column(String, nullable=False, unique=True)
    name = Column(String, nullable=False)
    created_at = Column(DateTime, server_default=func.now())
    is_active = Column(Boolean, default=True)

    posts = relationship("Post", back_populates="user")

class Post(Base):
    __tablename__ = 'posts'

    id = Column(Integer, primary_key=True)
    user_id = Column(Integer, ForeignKey('users.id', ondelete='CASCADE'), nullable=False)
    title = Column(String, nullable=False)
    content = Column(Text)
    published_at = Column(DateTime, nullable=True)
    created_at = Column(DateTime, server_default=func.now())

    user = relationship("User", back_populates="posts")
```

## Step 1: Generate Desired Schema from SQLAlchemy

Use your shadow database (already running) to extract the schema defined by your SQLAlchemy models:

```python
# generate_schema.py
from sqlalchemy import create_engine
from models import Base

# Connect to the shadow database (assuming default Lockplane setup)
engine = create_engine('postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow')

# Clean slate: drop existing tables, then create from models
Base.metadata.drop_all(engine)
Base.metadata.create_all(engine)

print("Created tables in shadow database from SQLAlchemy models")
```

Run the script:

```bash
python generate_schema.py
```

Now use Lockplane CLI to introspect the shadow database:

```bash
# Export the schema to JSON (using SHADOW_DATABASE_URL from environment)
lockplane introspect --db "$SHADOW_DATABASE_URL" > desired.json

# Or specify the connection string directly
lockplane introspect --db postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow > desired.json
```

**Note:** The shadow database gets cleaned automatically during migrations (each test runs in a rolled-back transaction), but if you need to manually clear it:

```bash
# Option 1: Restart the shadow container
docker compose restart shadow

# Option 2: Drop all tables via Python
python -c "from sqlalchemy import create_engine; from models import Base; engine = create_engine('postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow'); Base.metadata.drop_all(engine)"
```

Your `desired.json` now contains the schema from your SQLAlchemy models.

## Alternative: SQL DDL Approach

If you prefer SQL, use SQLAlchemy's DDL compiler:

```python
# generate_schema_sql.py
from sqlalchemy import create_engine
from sqlalchemy.schema import CreateTable, CreateIndex
from models import Base

# Use a mock PostgreSQL dialect
from sqlalchemy.dialects import postgresql

# Generate CREATE TABLE statements
with open('desired.lp.sql', 'w') as f:
    for table in Base.metadata.sorted_tables:
        # Generate CREATE TABLE
        create_table = CreateTable(table).compile(dialect=postgresql.dialect())
        f.write(str(create_table) + ';\n\n')

        # Generate CREATE INDEX for indexes
        for index in table.indexes:
            from sqlalchemy.schema import CreateIndex
            create_index = CreateIndex(index).compile(dialect=postgresql.dialect())
            f.write(str(create_index) + ';\n')

        if table.indexes:
            f.write('\n')

print("Generated desired.lp.sql from SQLAlchemy models")
```

Then use Lockplane to diff:

```bash
lockplane diff $DATABASE_URL desired.lp.sql
```

## Step 2: Generate Migration Plan

Now you can generate a migration plan directly from your database:

```bash
# Generate a safe migration plan with validation (using database connection string)
lockplane plan --from $DATABASE_URL --to desired.json --validate > migration.json

# Or if you generated SQL DDL:
lockplane plan --from $DATABASE_URL --to desired.lp.sql --validate > migration.json
```

> **💡 Tip:** Lockplane automatically introspects your database when you provide a connection string, so you don't need to run `lockplane introspect` first!

Lockplane will:
- Compute the difference between schemas
- Generate SQL migration steps
- Validate the plan can execute without errors (using shadow DB)
- Ensure all operations are reversible

## Step 3: Review the Plan

```bash
cat migration.json
```

Example output:

```json
{
  "steps": [
    {
      "description": "Add column 'is_active' to table 'users'",
      "sql": "ALTER TABLE users ADD COLUMN is_active BOOLEAN DEFAULT true"
    },
    {
      "description": "Create index 'posts_published_at_idx' on table 'posts'",
      "sql": "CREATE INDEX posts_published_at_idx ON posts (published_at)"
    }
  ]
}
```

## Step 4: Apply Migration

```bash
# Apply to production database
lockplane apply migration.json
```

Lockplane will:
1. Test the migration on a shadow database first
2. Only apply to production if shadow testing succeeds
3. Execute all steps in a transaction
4. Automatically rollback on failure

## One-Step Migration (Auto-Approve)

For development environments, you can skip the intermediate plan file:

```bash
# 1. Generate desired schema from models (uses shadow database)
python generate_schema.py

# 2. Introspect to JSON
lockplane introspect --db "$SHADOW_DATABASE_URL" > desired.json

# 3. Apply in one command (directly from your actual database)
lockplane apply --auto-approve --target $DATABASE_URL --schema desired.json --validate
```

## Rollback Plan

Always generate a rollback before applying to production:

```bash
# Generate rollback plan (using database connection string)
lockplane rollback --plan migration.json --from $DATABASE_URL > rollback.json

# If something goes wrong, apply the rollback
lockplane apply rollback.json
```

## Integration with CI/CD

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
      - uses: actions/checkout@v3

      - name: Set up Python
        uses: actions/setup-python@v4
        with:
          python-version: '3.11'

      - name: Install dependencies
        run: |
          pip install sqlalchemy psycopg2-binary

      - name: Download Lockplane
        run: |
          wget https://github.com/zakandrewking/lockplane/releases/latest/download/lockplane_Linux_x86_64.tar.gz
          tar -xzf lockplane_Linux_x86_64.tar.gz
          chmod +x lockplane

      - name: Set up PostgreSQL
        uses: ikalnytskyi/action-setup-postgres@v4

      - name: Generate desired schema from SQLAlchemy models
        env:
          SHADOW_DATABASE_URL: ${{ secrets.SHADOW_DATABASE_URL }}
        run: |
          python generate_schema.py
          ./lockplane introspect --db "$SHADOW_DATABASE_URL" > desired.json

      - name: Generate migration plan
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
          SHADOW_DATABASE_URL: ${{ secrets.SHADOW_DATABASE_URL }}
        run: ./lockplane plan --from $DATABASE_URL --to desired.json --validate > migration.json

      - name: Generate rollback plan
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        run: ./lockplane rollback --plan migration.json --from $DATABASE_URL > rollback.json

      - name: Upload rollback plan
        uses: actions/upload-artifact@v3
        with:
          name: rollback-plan
          path: rollback.json

      - name: Apply migration
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
          SHADOW_DATABASE_URL: ${{ secrets.SHADOW_DATABASE_URL }}
        run: ./lockplane apply migration.json
```

## Tips and Best Practices

### 1. Always Use a Shadow Database

Lockplane's `apply` command uses environment variables to know where to execute migrations:

```bash
# Set these before running apply
export DATABASE_URL="postgresql://localhost:5432/myapp"
export SHADOW_DATABASE_URL="postgresql://localhost:5433/myapp_shadow"

# Then apply uses these automatically
lockplane apply migration.json
```

The shadow database is tested first - if the migration fails there, your production database is never touched.

### 2. Version Control Your Schemas (Optional)

You can optionally commit the generated schema files to create an audit trail:

```bash
git add desired.json
git commit -m "chore: update schema for v2.1.0"
```

However, with database connection strings, you can skip this and work directly with your live databases.

### 3. Generate Schemas in Development

In your local development workflow:

```bash
# Make changes to models.py
# ...

# Regenerate desired schema (using shadow database)
python generate_schema.py
lockplane introspect --db "$SHADOW_DATABASE_URL" > desired.json

# See what changed (directly from your database)
lockplane diff $DATABASE_URL desired.json

# Apply changes locally
lockplane apply --auto-approve --target $DATABASE_URL --schema desired.json
```

### 4. Alembic Migration

If you're currently using Alembic, you can migrate to Lockplane easily:

```bash
# Generate desired state from SQLAlchemy models (using shadow database)
python generate_schema.py
lockplane introspect --db "$SHADOW_DATABASE_URL" > desired.json

# Use Lockplane for future migrations (directly from your database)
lockplane plan --from $DATABASE_URL --to desired.json --validate > migration.json
```

See also: [Migrating from Alembic](alembic.md)

### 5. Schema Snapshots (Optional)

You can save schema snapshots at each release for audit purposes:

```bash
lockplane introspect > schemas/v2.1.0.json
git add schemas/v2.1.0.json
git commit -m "chore: snapshot schema for v2.1.0"
```

However, for most workflows, working directly with database connection strings is simpler.

## Comparison with Alembic

| Feature | Lockplane | Alembic |
|---------|-----------|---------|
| Schema as code | ✅ SQLAlchemy models | ✅ SQLAlchemy models |
| Auto-generate migrations | ✅ Yes (from models) | ✅ Yes (from models) |
| Manual migration scripts | ❌ Not needed | ✅ Supported |
| Shadow DB validation | ✅ Built-in | ❌ Manual |
| Automatic rollback generation | ✅ Yes | ❌ Manual |
| Database support | PostgreSQL, SQLite | PostgreSQL, MySQL, SQLite, others |
| AI-friendly | ✅ JSON schemas | ⚠️ Python scripts |

## Example Repository Structure

```
myproject/
├── models.py              # SQLAlchemy models (source of truth)
├── generate_schema.py     # Script to export desired schema
├── desired.json           # Desired schema from models
├── migration.json         # Generated migration plan (optional)
├── rollback.json          # Generated rollback plan (optional)
└── schemas/               # Schema version history (optional)
    ├── v1.0.0.json
    ├── v2.0.0.json
    └── v2.1.0.json
```

> **Note:** With database connection strings, you don't need to maintain `current.json` or schema version history - Lockplane introspects directly from your database when needed.

## Advanced: Handling Complex Migrations

For data migrations or complex schema changes, use Lockplane for DDL and a separate script for data:

```bash
# 1. Run DDL migration with Lockplane
lockplane apply migration.json

# 2. Run data migration
python migrate_data.py

# 3. Verify (introspect the database to check)
lockplane introspect
```

## Troubleshooting

### "Shadow DB validation failed"

Your migration might contain unsafe operations. Check the error message:

```bash
lockplane plan --from $DATABASE_URL --to desired.json --validate
```

Common issues:
- Dropping a NOT NULL constraint before setting a default
- Type changes that lose data
- Missing foreign key constraints

### "Type mismatch"

SQLAlchemy's `String` maps to different SQL types depending on dialect. Be explicit:

```python
# Instead of
name = Column(String)

# Use
from sqlalchemy import Text
name = Column(Text)  # or String(255) for VARCHAR(255)
```

### "Index already exists"

Lockplane detects indexes automatically. Make sure your SQLAlchemy models match the database state:

```python
class User(Base):
    __tablename__ = 'users'
    email = Column(String, nullable=False, unique=True)  # Creates an index
```

## Next Steps

- [Getting Started Guide](getting_started.md)
- [Prisma Integration](prisma.md)
- [Alembic Migration Guide](alembic.md)
- [Supabase Integration](supabase.md)
