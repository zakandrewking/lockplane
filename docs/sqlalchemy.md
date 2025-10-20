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

## Step 1: Export Current Schema

First, export your current database schema with Lockplane:

```bash
# Export current production schema
lockplane introspect > current.json
```

## Step 2: Generate Desired Schema from SQLAlchemy

Use a temporary in-memory SQLite database to extract the schema defined by your SQLAlchemy models:

```python
# generate_schema.py
from sqlalchemy import create_engine
from models import Base
import subprocess
import json

# Create a temporary in-memory database
engine = create_engine('sqlite:///:memory:')

# Create all tables from your models
Base.metadata.create_all(engine)

# For PostgreSQL-specific features, you can also use a temporary Postgres DB:
# engine = create_engine('postgresql://localhost/temp_schema_db')
# Base.metadata.create_all(engine)

# Use Lockplane to introspect the temp database
# For SQLite:
result = subprocess.run(
    ['lockplane', 'introspect'],
    env={'DATABASE_URL': 'sqlite://:memory:'},
    capture_output=True,
    text=True
)

# Or for the temp Postgres approach:
# result = subprocess.run(
#     ['lockplane', 'introspect'],
#     env={'DATABASE_URL': 'postgresql://localhost/temp_schema_db'},
#     capture_output=True,
#     text=True
# )

with open('desired.json', 'w') as f:
    f.write(result.stdout)

print("Generated desired.json from SQLAlchemy models")
```

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
lockplane diff current.json desired.lp.sql
```

## Step 3: Generate Migration Plan

```bash
# Generate a safe migration plan with validation
lockplane plan --from current.json --to desired.json --validate > migration.json
```

Lockplane will:
- Compute the difference between schemas
- Generate SQL migration steps
- Validate the plan can execute without errors (using shadow DB)
- Ensure all operations are reversible

## Step 4: Review the Plan

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

## Step 5: Apply Migration

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
# Generate current schema from production
lockplane introspect > current.json

# Generate desired schema from models
python generate_schema.py

# Apply in one command
lockplane apply --auto-approve --from current.json --to desired.json --validate
```

## Rollback Plan

Always generate a rollback before applying to production:

```bash
# Generate rollback plan
lockplane rollback --plan migration.json --from current.json > rollback.json

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

      - name: Export current schema
        env:
          DATABASE_URL: ${{ secrets.DATABASE_URL }}
        run: ./lockplane introspect > current.json

      - name: Generate desired schema from SQLAlchemy models
        run: python generate_schema.py

      - name: Generate migration plan
        env:
          SHADOW_DATABASE_URL: ${{ secrets.SHADOW_DATABASE_URL }}
        run: ./lockplane plan --from current.json --to desired.json --validate > migration.json

      - name: Generate rollback plan
        run: ./lockplane rollback --plan migration.json --from current.json > rollback.json

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

### 2. Version Control Your Schemas

Commit the generated schema files:

```bash
git add current.json desired.json
git commit -m "chore: update schema for v2.1.0"
```

This creates an audit trail of schema changes over time.

### 3. Generate Schemas in Development

In your local development workflow:

```bash
# Make changes to models.py
# ...

# Regenerate desired schema
python generate_schema.py

# See what changed
lockplane diff current.json desired.json

# Apply changes locally
lockplane apply --auto-approve --from current.json --to desired.json
```

### 4. Alembic Migration

If you're currently using Alembic, you can migrate to Lockplane gradually:

```bash
# Generate current state from your Alembic-managed database
lockplane introspect > alembic_current.json

# Generate desired state from SQLAlchemy models
python generate_schema.py

# Use Lockplane for future migrations
lockplane plan --from alembic_current.json --to desired.json --validate > migration.json
```

See also: [Migrating from Alembic](alembic.md)

### 5. Schema Snapshots

Save schema snapshots at each release:

```bash
lockplane introspect > schemas/v2.1.0.json
git add schemas/v2.1.0.json
git commit -m "chore: snapshot schema for v2.1.0"
```

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
├── current.json           # Current production schema
├── desired.json           # Desired schema from models
├── migration.json         # Generated migration plan
├── rollback.json          # Generated rollback plan
└── schemas/               # Schema version history
    ├── v1.0.0.json
    ├── v2.0.0.json
    └── v2.1.0.json
```

## Advanced: Handling Complex Migrations

For data migrations or complex schema changes, use Lockplane for DDL and a separate script for data:

```bash
# 1. Run DDL migration with Lockplane
lockplane apply --plan migration.json

# 2. Run data migration
python migrate_data.py

# 3. Verify
lockplane introspect > current.json
```

## Troubleshooting

### "Shadow DB validation failed"

Your migration might contain unsafe operations. Check the error message:

```bash
lockplane plan --from current.json --to desired.json --validate
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
