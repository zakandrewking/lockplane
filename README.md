# Lockplane

Easy postgres schema management.

## 1. Install

```bash
go install github.com/lockplane/lockplane
```

## 2. Create a config file

The config file is a TOML file named `lockplane.toml` in the root of the project.  It should look like this:

```toml
[environments.local]
postgres_url = "postgresql://postgres:postgres@localhost:5432/postgres"
```

At this time, only a single environment called `local` is supported.

## 3. Create a schema file

Add to `schema/users.lp.sql`:

```sql
CREATE TABLE users (
  id BIGINT PRIMARY KEY,
  email TEXT NOT NULL,
  created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);
```

For now, schema files must be in the root of the `schema/` directory, and must
end in `.lp.sql`.

## 4. Check the schema for issues

```bash
# TODO
npx lockplane check-schema
```

## 4. Apply changes

```bash
# TODO
npx lockplane apply
```

## Postgres Feature Support

Feature | SQL Parsing | DB Introspection
-- | -- | --
CREATE TABLE | ✅ | ✅
Column types: SMALLINT, INTEGER, BIGINT, SMALLSERIAL, SERIAL, BIGSERIAL TIMESTAMP, VARCHAR, CHAR, REAL, DOUBLE PRECISION, TIMESTAMP [WITH TIME ZONE], TIME [WITH TIME ZONE], TEXT, NUMERIC, DECIMAL, BYTEA, JSON, JSONB | ✅ | ✅
DEFAULT: literals (numeric, string, boolean, NULL), SQL value functions (NOW(), CURRENT_TIMESTAMP, CURRENT_DATE, CURRENT_TIME, LOCALTIME, LOCALTIMESTAMP, CURRENT_USER, SESSION_USER) | ✅ | ✅
DEFAULT: expressions (arithmetic, type casts), UUID functions (gen_random_uuid(), uuid_generate_v4(), uuidv7()), sequence functions (nextval()), array/JSON literals, GENERATED, IDENTITY | ❌ | ❌
Enable RLS   | ❌ | ❌
