# Using Lockplane with SQLAlchemy

This guide shows you how to use Lockplane with SQLAlchemy, a popular Python ORM.

## Overview

With SQLAlchemy, your ORM models are the source of truth. Lockplane helps you safely migrate your database to match your models without manually writing migration scripts.

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

First, create a temporary PostgreSQL database to extract the schema defined by your SQLAlchemy models:

```bash
# Create a temporary database for schema generation
createdb temp_schema_db

# Or using psql:
# psql -c "CREATE DATABASE temp_schema_db;"
```

Then use a simple Python script to create your tables:

```python
# generate_schema.py
from sqlalchemy import create_engine
from models import Base

# Connect to the temporary database
engine = create_engine('postgresql://localhost/temp_schema_db')

# Create all tables from your SQLAlchemy models
Base.metadata.create_all(engine)

print("Created tables in temp_schema_db from SQLAlchemy models")
```

Run the script:

```bash
python generate_schema.py
```

Now use Lockplane CLI to introspect the temporary database:

```bash
# Export the schema to JSON
lockplane introspect --db postgresql://localhost/temp_schema_db > desired.json

# Clean up the temporary database
dropdb temp_schema_db
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
lockplane apply --plan migration.json
```

Lockplane will:
1. Test the migration on a shadow database first
2. Only apply to production if shadow testing succeeds
3. Execute all steps in a transaction
4. Automatically rollback on failure

## One-Step Migration (Auto-Approve)

For development environments, you can skip the intermediate plan file:

```bash
# 1. Create temporary database
createdb temp_schema_db

# 2. Generate desired schema from models
python generate_schema.py

# 3. Introspect to JSON
lockplane introspect --db postgresql://localhost/temp_schema_db > desired.json

# 4. Clean up temporary database
dropdb temp_schema_db

# 5. Apply in one command (directly from your actual database)
lockplane apply --auto-approve --from $DATABASE_URL --to desired.json --validate
```

## Rollback Plan

Always generate a rollback before applying to production:

```bash
# Generate rollback plan (using database connection string)
lockplane rollback --plan migration.json --from $DATABASE_URL > rollback.json

# If something goes wrong, apply the rollback
lockplane apply --plan rollback.json
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
        run: |
          createdb temp_schema_db
          python generate_schema.py
          ./lockplane introspect --db postgresql://localhost/temp_schema_db > desired.json
          dropdb temp_schema_db

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
        run: ./lockplane apply --plan migration.json
```

## Tips and Best Practices

### 1. Always Use a Shadow Database

Set `SHADOW_DATABASE_URL` to test migrations before applying to production:

```bash
export DATABASE_URL="postgresql://localhost:5432/myapp"
export SHADOW_DATABASE_URL="postgresql://localhost:5433/myapp_shadow"
```

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

# Regenerate desired schema
createdb temp_schema_db
python generate_schema.py
lockplane introspect --db postgresql://localhost/temp_schema_db > desired.json
dropdb temp_schema_db

# See what changed (directly from your database)
lockplane diff $DATABASE_URL desired.json

# Apply changes locally
lockplane apply --auto-approve --from $DATABASE_URL --to desired.json
```

### 4. Alembic Migration

If you're currently using Alembic, you can migrate to Lockplane easily:

```bash
# Generate desired state from SQLAlchemy models
createdb temp_schema_db
python generate_schema.py
lockplane introspect --db postgresql://localhost/temp_schema_db > desired.json
dropdb temp_schema_db

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
lockplane apply --plan migration.json

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
