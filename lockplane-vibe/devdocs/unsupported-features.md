# Unsupported SQL Features

**Status:** Draft - Living Document
**Created:** 2025-11-12
**Last Updated:** 2025-11-12

This document catalogs SQL standard features and database-specific features that Lockplane does not currently support. This serves as a roadmap for future development and helps users understand current limitations.

## Table of Contents

- [Current Support Summary](#current-support-summary)
- [SQL Standard Features](#sql-standard-features)
- [PostgreSQL-Specific Features](#postgresql-specific-features)
- [SQLite-Specific Features](#sqlite-specific-features)
- [Implementation Priority](#implementation-priority)

---

## Current Support Summary

### What Lockplane Currently Supports

**Basic DDL:**
- ✅ Tables (CREATE, DROP)
- ✅ Columns (ADD, DROP, ALTER TYPE/NULLABLE/DEFAULT for Postgres)
- ✅ Indexes (CREATE, DROP, UNIQUE indexes)
- ✅ Foreign Keys (CREATE, DROP, ON DELETE/ON UPDATE actions)
- ✅ Primary Keys
- ✅ Schema namespaces (PostgreSQL only)

**Introspection:**
- ✅ Table structure
- ✅ Column definitions with types, nullability, defaults
- ✅ Indexes including unique constraints
- ✅ Foreign key relationships
- ✅ Dialect detection (PostgreSQL, SQLite)

**Migration Operations:**
- ✅ Forward migration plans
- ✅ Rollback plan generation
- ✅ Shadow database validation
- ✅ Multi-phase migrations (for Postgres constraints)
- ✅ Table recreation for SQLite constraints

### Known Limitations

**SQLite:**
- ❌ ALTER COLUMN operations (requires table recreation workaround)
- ❌ ADD CONSTRAINT for foreign keys (must be in CREATE TABLE)
- ❌ DROP TABLE CASCADE

**Both Databases:**
- ❌ No support for most advanced SQL features listed below

---

## SQL Standard Features

These are features defined in the SQL standard (SQL:2016, SQL:2023) that Lockplane does not currently support.

### 1. Constraints

#### CHECK Constraints
**Status:** ❌ Not Supported
**SQL Standard:** SQL-92 and later
**Priority:** High

```sql
-- Table-level check constraint
CREATE TABLE products (
  price DECIMAL(10,2),
  discount_price DECIMAL(10,2),
  CONSTRAINT valid_discount CHECK (discount_price < price)
);

-- Column-level check constraint
CREATE TABLE users (
  age INTEGER CHECK (age >= 0 AND age <= 150)
);
```

**Impact:**
- Cannot enforce domain logic at database level
- Must rely on application-level validation
- Cannot introspect existing check constraints

**Complexity:** Medium
- Need to parse CHECK constraint expressions
- Need to handle constraint names
- Need to support both table-level and column-level constraints

---

#### UNIQUE Constraints (Named)
**Status:** ⚠️ Partial Support
**Current:** Only via UNIQUE indexes
**Priority:** Medium

```sql
-- Named unique constraint
ALTER TABLE users
ADD CONSTRAINT uk_users_email UNIQUE (email);

-- Composite unique constraint
ALTER TABLE posts
ADD CONSTRAINT uk_posts_user_title UNIQUE (user_id, title);
```

**Impact:**
- Cannot distinguish between UNIQUE indexes and UNIQUE constraints
- Constraint names may not match user intent
- Cannot use DEFERRABLE UNIQUE constraints (Postgres)

**Complexity:** Low-Medium
- Already support unique indexes
- Need to add constraint metadata
- Need to handle named vs unnamed constraints

---

#### EXCLUSION Constraints
**Status:** ❌ Not Supported
**Database:** PostgreSQL only
**Priority:** Low

```sql
-- Prevent overlapping date ranges
CREATE TABLE bookings (
  room_id INTEGER,
  during TSRANGE,
  EXCLUDE USING gist (room_id WITH =, during WITH &&)
);
```

**Impact:**
- Cannot enforce complex business rules like non-overlapping ranges
- Users must implement workarounds in application code

**Complexity:** High
- PostgreSQL-specific syntax
- Requires GiST/SP-GiST index support
- Complex expression parsing

---

#### DEFERRABLE Constraints
**Status:** ❌ Not Supported
**SQL Standard:** SQL-92 and later
**Database:** PostgreSQL only (for UNIQUE, PK, FK, EXCLUDE)
**Priority:** Medium

```sql
-- Deferrable foreign key
CREATE TABLE parent (
  id INTEGER PRIMARY KEY
);

CREATE TABLE child (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER,
  CONSTRAINT fk_child_parent
    FOREIGN KEY (parent_id) REFERENCES parent(id)
    DEFERRABLE INITIALLY DEFERRED
);

-- Can temporarily violate constraint within transaction
BEGIN;
INSERT INTO child VALUES (1, 100);  -- parent_id=100 doesn't exist yet
INSERT INTO parent VALUES (100);     -- Create it before commit
COMMIT;  -- Constraint checked here
```

**Impact:**
- Cannot handle circular dependencies in migrations
- Cannot temporarily violate constraints during data migration
- Must carefully order operations in migration plans

**Complexity:** Medium-High
- Need to track constraint timing (IMMEDIATE vs DEFERRED)
- Need to generate appropriate SET CONSTRAINTS statements
- Impacts migration plan generation logic

---

### 2. Advanced Column Features

#### Generated/Computed Columns
**Status:** ❌ Not Supported
**SQL Standard:** SQL:2003 and later
**Priority:** Medium

```sql
-- PostgreSQL (v12+)
CREATE TABLE products (
  price DECIMAL(10,2),
  tax_rate DECIMAL(5,4),
  price_with_tax DECIMAL(10,2) GENERATED ALWAYS AS (price * (1 + tax_rate)) STORED
);

-- SQLite (v3.31+)
CREATE TABLE rectangles (
  width REAL,
  height REAL,
  area REAL GENERATED ALWAYS AS (width * height) VIRTUAL
);
```

**Impact:**
- Cannot introspect generated column expressions
- Cannot preserve computed columns during migrations
- Users must manually recreate generated columns

**Complexity:** Medium-High
- Need to parse generation expressions
- Need to handle STORED vs VIRTUAL (SQLite)
- Need to handle dependencies between generated columns
- Column modification becomes more complex

---

#### IDENTITY Columns
**Status:** ❌ Not Supported
**SQL Standard:** SQL:2003 and later
**Database:** PostgreSQL 10+ (SQLite has AUTOINCREMENT)
**Priority:** Medium

```sql
-- PostgreSQL IDENTITY (modern approach)
CREATE TABLE products (
  id INTEGER GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  name TEXT
);

-- With options
CREATE TABLE orders (
  id INTEGER GENERATED BY DEFAULT AS IDENTITY (START WITH 1000 INCREMENT BY 1),
  order_date DATE
);
```

**Impact:**
- Currently only handle SERIAL/BIGSERIAL (Postgres) via default expressions
- Cannot distinguish IDENTITY from SERIAL
- Cannot modify IDENTITY parameters

**Complexity:** Medium
- Need to introspect IDENTITY metadata
- Need to handle ALWAYS vs BY DEFAULT
- Need to handle sequence options (start, increment, etc.)

---

#### Column Collations
**Status:** ❌ Not Supported
**SQL Standard:** SQL:1999 and later
**Priority:** Low

```sql
-- PostgreSQL
CREATE TABLE names (
  name TEXT COLLATE "en_US"
);

-- SQLite
CREATE TABLE names (
  name TEXT COLLATE NOCASE
);
```

**Impact:**
- Cannot preserve collation settings during migrations
- May cause unexpected sort/comparison behavior after migration

**Complexity:** Low-Medium
- Need to introspect collation settings
- Need to handle database-specific collation names
- Different collations available per database

---

### 3. Database Objects

#### Views
**Status:** ❌ Not Supported
**SQL Standard:** SQL-92 and later
**Priority:** Medium-High

```sql
CREATE VIEW active_users AS
SELECT id, email, created_at
FROM users
WHERE deleted_at IS NULL;

-- Updatable view (Postgres)
CREATE VIEW user_emails AS
SELECT id, email FROM users;

-- Materialized view (Postgres)
CREATE MATERIALIZED VIEW daily_stats AS
SELECT date, COUNT(*) FROM events GROUP BY date;
```

**Impact:**
- Views not included in schema introspection
- Views not preserved during migrations
- Cannot generate migrations for view changes

**Complexity:** Medium-High
- Need to introspect view definitions (SQL text)
- Need to handle view dependencies
- Need to detect view changes (tricky - text comparison?)
- Materialized views have additional complexity (refresh, indexes)

---

#### Sequences
**Status:** ⚠️ Partial Support
**Current:** Only implicit via SERIAL types
**Priority:** Medium

```sql
-- Explicit sequence
CREATE SEQUENCE order_numbers START 1000;

-- Use in table
CREATE TABLE orders (
  id INTEGER DEFAULT nextval('order_numbers'),
  order_date DATE
);

-- Shared sequence across tables
CREATE TABLE invoices (
  id INTEGER DEFAULT nextval('order_numbers'),
  invoice_date DATE
);
```

**Impact:**
- Cannot create standalone sequences
- Cannot share sequences across tables
- Cannot modify sequence parameters
- SERIAL/BIGSERIAL automatically create sequences, but not exposed

**Complexity:** Medium
- Need to introspect sequences separately from tables
- Need to track sequence dependencies
- Need to generate ALTER SEQUENCE statements
- SQLite doesn't have sequences (uses AUTOINCREMENT)

---

#### Triggers
**Status:** ❌ Not Supported
**SQL Standard:** SQL:1999 and later
**Priority:** Low-Medium

```sql
-- PostgreSQL trigger
CREATE TRIGGER update_modified_at
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_modified_at_column();

-- SQLite trigger
CREATE TRIGGER update_modified_at
AFTER UPDATE ON users
BEGIN
  UPDATE users SET modified_at = CURRENT_TIMESTAMP
  WHERE id = NEW.id;
END;
```

**Impact:**
- Triggers not included in schema introspection
- Triggers not preserved during migrations
- May break application logic after migration

**Complexity:** High
- Need to introspect trigger definitions
- Need to handle BEFORE/AFTER, INSERT/UPDATE/DELETE
- Need to handle FOR EACH ROW vs FOR EACH STATEMENT
- SQLite and Postgres have very different syntax
- Need to track trigger dependencies on functions (Postgres)

---

#### Stored Procedures and Functions
**Status:** ❌ Not Supported
**SQL Standard:** SQL:1999 and later
**Priority:** Low

```sql
-- PostgreSQL function
CREATE FUNCTION update_modified_at_column()
RETURNS TRIGGER AS $$
BEGIN
  NEW.modified_at = CURRENT_TIMESTAMP;
  RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- PostgreSQL procedure (v11+)
CREATE PROCEDURE transfer_funds(
  from_account INTEGER,
  to_account INTEGER,
  amount DECIMAL
)
LANGUAGE plpgsql
AS $$
BEGIN
  UPDATE accounts SET balance = balance - amount WHERE id = from_account;
  UPDATE accounts SET balance = balance + amount WHERE id = to_account;
  COMMIT;
END;
$$;
```

**Impact:**
- Functions not included in schema introspection
- Cannot migrate database logic
- Application may break if functions are expected

**Complexity:** Very High
- Need to introspect function definitions (PL/pgSQL, SQL, etc.)
- Need to handle parameters, return types, language
- Need to handle SECURITY DEFINER vs INVOKER
- Need to detect function changes
- SQLite has limited function support

---

### 4. Advanced Table Features

#### Table Inheritance
**Status:** ❌ Not Supported
**Database:** PostgreSQL only
**Priority:** Low

```sql
CREATE TABLE cities (
  name TEXT,
  population INTEGER
);

CREATE TABLE capitals (
  state TEXT
) INHERITS (cities);
```

**Impact:**
- Cannot migrate schemas using table inheritance
- Users must flatten inheritance manually

**Complexity:** High
- PostgreSQL-specific feature
- Need to track inheritance hierarchy
- Queries against parent table include child rows
- Complex implications for migrations

---

#### Table Partitioning
**Status:** ❌ Not Supported
**SQL Standard:** SQL:2003 and later
**Priority:** Medium

```sql
-- PostgreSQL declarative partitioning
CREATE TABLE measurements (
  city_id INTEGER,
  logdate DATE,
  temperature INTEGER
) PARTITION BY RANGE (logdate);

CREATE TABLE measurements_y2023
PARTITION OF measurements
FOR VALUES FROM ('2023-01-01') TO ('2024-01-01');

CREATE TABLE measurements_y2024
PARTITION OF measurements
FOR VALUES FROM ('2024-01-01') TO ('2025-01-01');
```

**Impact:**
- Cannot introspect partitioned tables properly
- Cannot generate migrations for partition changes
- Large table migrations may be inefficient

**Complexity:** Very High
- Need to introspect partition strategy (RANGE, LIST, HASH)
- Need to handle partition boundaries
- Need to track parent-child relationships
- Need to handle subpartitioning
- Different syntax between databases

---

#### Temporary Tables
**Status:** ❌ Not Supported
**SQL Standard:** SQL-92 and later
**Priority:** Low

```sql
-- Session-scoped temporary table
CREATE TEMPORARY TABLE temp_results (
  id INTEGER,
  result TEXT
);
```

**Impact:**
- Temporary tables not useful for declarative schema management
- Not a blocker for most use cases

**Complexity:** Low
- Should explicitly exclude from introspection
- May need flag to handle if user wants them

---

#### Unlogged Tables
**Status:** ❌ Not Supported
**Database:** PostgreSQL only
**Priority:** Low

```sql
CREATE UNLOGGED TABLE temp_data (
  id INTEGER,
  data TEXT
);
```

**Impact:**
- Cannot preserve unlogged table status
- May impact performance after migration

**Complexity:** Low
- Need to introspect table persistence
- Single boolean flag per table

---

### 5. Schema Management

#### Multiple Schemas
**Status:** ⚠️ Partial Support
**Current:** Basic CREATE/DROP/SET schema support for Postgres
**Priority:** Medium

```sql
-- PostgreSQL schemas
CREATE SCHEMA accounting;
CREATE SCHEMA sales;

CREATE TABLE accounting.invoices (...);
CREATE TABLE sales.orders (...);

-- Set search path
SET search_path TO accounting, public;
```

**Current Support:**
- ✅ Can create/drop schemas
- ✅ Can set search path
- ❌ Schema not included in table introspection
- ❌ Cannot introspect which schema tables belong to
- ❌ Cannot generate cross-schema migrations

**Impact:**
- Multi-schema databases not fully supported
- Must manually specify schema in all operations

**Complexity:** Medium
- Need to track schema per table
- Need to handle search_path in migrations
- Need to handle cross-schema references
- SQLite doesn't support schemas (but has ATTACH DATABASE)

---

#### Database-Level Collations
**Status:** ❌ Not Supported
**SQL Standard:** SQL:1999 and later
**Priority:** Low

```sql
CREATE DATABASE mydb LOCALE 'en_US.UTF-8';
```

**Impact:**
- Database-level settings not managed
- Out of scope for table-level schema management

**Complexity:** Low
- Could add as metadata
- Not critical for most use cases

---

### 6. Advanced Index Features

#### Partial Indexes
**Status:** ❌ Not Supported
**SQL Standard:** Not standard (PostgreSQL, SQLite extension)
**Priority:** Medium

```sql
-- PostgreSQL / SQLite
CREATE INDEX idx_active_users ON users (email)
WHERE deleted_at IS NULL;
```

**Impact:**
- Cannot preserve partial index predicates
- May create less efficient indexes after migration

**Complexity:** Medium
- Need to introspect WHERE clause
- Need to parse predicate expressions
- Need to include in index comparison logic

---

#### Expression Indexes
**Status:** ❌ Not Supported
**Database:** PostgreSQL, SQLite
**Priority:** Medium

```sql
-- Index on expression
CREATE INDEX idx_lower_email ON users (LOWER(email));

-- Complex expression
CREATE INDEX idx_full_name ON users ((first_name || ' ' || last_name));
```

**Impact:**
- Cannot preserve expression indexes
- May lose performance optimizations

**Complexity:** Medium-High
- Need to introspect index expressions
- Need to parse and compare expressions
- Need to handle expression changes

---

#### Index Types
**Status:** ⚠️ Partial Support
**Current:** Only B-tree (default)
**Database:** PostgreSQL has GiST, GIN, BRIN, SP-GiST, Hash
**Priority:** Low-Medium

```sql
-- PostgreSQL index types
CREATE INDEX idx_jsonb_data ON products USING gin (data);
CREATE INDEX idx_tsv ON documents USING gin (tsv);
CREATE INDEX idx_geom ON locations USING gist (geom);
CREATE INDEX idx_large_table ON large_table USING brin (created_at);
```

**Impact:**
- Cannot use specialized index types
- May have suboptimal query performance
- Cannot introspect index method

**Complexity:** Medium
- Need to introspect index type
- Need to handle type-specific options
- Need to generate CREATE INDEX with USING clause
- SQLite only supports B-tree

---

#### Index Include Columns
**Status:** ❌ Not Supported
**Database:** PostgreSQL 11+ (covering indexes)
**Priority:** Low

```sql
-- Include non-key columns in index
CREATE INDEX idx_users_email_include
ON users (email)
INCLUDE (first_name, last_name);
```

**Impact:**
- Cannot use covering indexes
- May have suboptimal query performance

**Complexity:** Low-Medium
- Need to introspect INCLUDE columns
- Need to separate key columns from included columns

---

#### Index Storage Parameters
**Status:** ❌ Not Supported
**Database:** PostgreSQL
**Priority:** Low

```sql
CREATE INDEX idx_large ON large_table (id)
WITH (fillfactor = 70);
```

**Impact:**
- Cannot preserve index tuning parameters
- Minor performance implications

**Complexity:** Low
- Need to introspect storage parameters
- Database-specific

---

### 7. Advanced Data Types

**Status:** ❌ Not Supported
**Priority:** Varies by type

Most advanced data types are not supported for introspection or migration:

#### PostgreSQL Types:
- Arrays (`INTEGER[]`, `TEXT[]`)
- JSON/JSONB
- UUID
- Network types (INET, CIDR, MACADDR)
- Geometric types (POINT, LINE, POLYGON, etc.)
- Range types (INT4RANGE, TSRANGE, etc.)
- Composite types (custom row types)
- Domain types (constrained base types)
- Enum types
- XML
- Full text search types (TSVECTOR, TSQUERY)
- Money
- Bit strings (BIT, VARBIT)

#### SQLite Types:
- SQLite has flexible typing (TEXT, INTEGER, REAL, BLOB, NULL)
- Type affinity rules
- No native date/time types (stored as TEXT/INTEGER/REAL)

**Current Support:**
- ✅ Basic types are preserved as-is
- ❌ No special handling for advanced types
- ❌ Type validation not performed

**Impact:**
- Advanced types stored as text representation
- Type-specific features not validated
- May lose type semantics during migrations

**Complexity:** High
- Each type has unique characteristics
- Need to handle type modifiers (length, precision, scale)
- Need to handle arrays, nested types
- Need to handle enums, domains separately
- Type compatibility between databases is complex

---

### 8. Miscellaneous

#### Comments
**Status:** ❌ Not Supported
**SQL Standard:** SQL:1999 and later
**Priority:** Low-Medium

```sql
COMMENT ON TABLE users IS 'Application user accounts';
COMMENT ON COLUMN users.email IS 'User email address (unique)';
COMMENT ON INDEX idx_email IS 'Fast email lookups';
```

**Impact:**
- Schema documentation lost during migrations
- Developers lose context

**Complexity:** Low
- Need to introspect comments
- Need to generate COMMENT ON statements
- SQLite has limited comment support (need to use pragma)

---

#### Row-Level Security (RLS)
**Status:** ❌ Not Supported
**Database:** PostgreSQL only
**Priority:** Low

```sql
ALTER TABLE accounts ENABLE ROW LEVEL SECURITY;

CREATE POLICY account_policy ON accounts
  FOR ALL
  TO app_user
  USING (user_id = current_user_id());
```

**Impact:**
- RLS policies not preserved during migrations
- Security model must be manually recreated

**Complexity:** High
- Need to introspect policies
- Need to handle policy expressions
- Need to handle roles and permissions
- Complex interaction with views and functions

---

#### Column Storage (TOAST)
**Status:** ❌ Not Supported
**Database:** PostgreSQL only
**Priority:** Very Low

```sql
ALTER TABLE documents
ALTER COLUMN content SET STORAGE EXTERNAL;
```

**Impact:**
- Storage optimization hints not preserved
- Minor performance implications

**Complexity:** Low
- Need to introspect storage settings
- PostgreSQL-specific optimization

---

## PostgreSQL-Specific Features

Features unique to PostgreSQL that are not in the SQL standard.

### 1. Extensions

**Status:** ❌ Not Supported
**Priority:** Medium-High

```sql
CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
CREATE EXTENSION IF NOT EXISTS "pg_trgm";
CREATE EXTENSION IF NOT EXISTS "postgis";
```

**Impact:**
- Cannot manage extension dependencies
- Schema may depend on extensions not installed in target
- Extension-dependent types/functions not validated

**Complexity:** Medium
- Need to introspect installed extensions
- Need to track version compatibility
- Need to handle extension dependencies
- Need to order CREATE EXTENSION before dependent objects

**Common Extensions:**
- uuid-ossp (UUID generation)
- pg_trgm (trigram text search)
- postgis (geographic data)
- hstore (key-value pairs)
- pgcrypto (cryptographic functions)
- citext (case-insensitive text)

---

### 2. Enum Types

**Status:** ❌ Not Supported
**Priority:** Medium

```sql
CREATE TYPE mood AS ENUM ('sad', 'ok', 'happy');

CREATE TABLE person (
  name TEXT,
  current_mood mood
);

-- Add enum value
ALTER TYPE mood ADD VALUE 'ecstatic';
```

**Impact:**
- Cannot introspect enum definitions
- Cannot preserve enum types during migrations
- Must use CHECK constraints or TEXT as workaround

**Complexity:** Medium-High
- Need to introspect enum types separately
- Need to introspect enum values and order
- Need to track enum dependencies on tables
- Need to handle ALTER TYPE ADD VALUE
- Cannot reorder or remove enum values easily

---

### 3. Domain Types

**Status:** ❌ Not Supported
**Priority:** Low-Medium

```sql
CREATE DOMAIN email AS TEXT
  CHECK (VALUE ~ '^[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z]{2,}$');

CREATE DOMAIN positive_integer AS INTEGER
  CHECK (VALUE > 0);

CREATE TABLE users (
  email email,
  age positive_integer
);
```

**Impact:**
- Cannot reuse constraint logic across tables
- Must duplicate CHECK constraints
- Cannot introspect domain definitions

**Complexity:** Medium
- Need to introspect domain definitions
- Need to track domain constraints
- Need to handle domain dependencies
- Need to distinguish domain from base type in columns

---

### 4. Composite Types

**Status:** ❌ Not Supported
**Priority:** Low

```sql
CREATE TYPE address AS (
  street TEXT,
  city TEXT,
  state CHAR(2),
  zip TEXT
);

CREATE TABLE companies (
  name TEXT,
  headquarters address
);
```

**Impact:**
- Cannot use structured column types
- Must flatten into multiple columns or use JSON

**Complexity:** Medium-High
- Need to introspect composite type definitions
- Need to handle nested types
- Need to track dependencies

---

### 5. Range Types

**Status:** ❌ Not Supported
**Priority:** Low

```sql
CREATE TABLE reservations (
  room_id INTEGER,
  during TSRANGE
);

-- Custom range type
CREATE TYPE float_range AS RANGE (
  subtype = FLOAT8,
  subtype_diff = float8mi
);
```

**Impact:**
- Cannot use range types for date ranges, numeric ranges, etc.
- Must use two separate columns (start, end)

**Complexity:** Medium
- Need to introspect range type definitions
- Need to handle built-in ranges (INT4RANGE, TSRANGE, etc.)
- Need to handle custom range types

---

### 6. Full Text Search

**Status:** ❌ Not Supported
**Priority:** Low

```sql
CREATE TABLE documents (
  title TEXT,
  body TEXT,
  tsv TSVECTOR
);

CREATE INDEX idx_tsv ON documents USING gin(tsv);

-- Trigger to maintain tsvector
CREATE TRIGGER tsvectorupdate
BEFORE INSERT OR UPDATE ON documents
FOR EACH ROW EXECUTE FUNCTION
tsvector_update_trigger(tsv, 'pg_catalog.english', title, body);
```

**Impact:**
- Cannot manage full text search indexes
- Must manually recreate FTS infrastructure

**Complexity:** High
- Need to handle TSVECTOR/TSQUERY types
- Need to handle text search configurations
- Need to handle GIN/GiST indexes for FTS
- Need to handle triggers for automatic updates

---

### 7. Array Types

**Status:** ❌ Not Supported
**Priority:** Medium

```sql
CREATE TABLE products (
  id INTEGER,
  name TEXT,
  tags TEXT[]
);

-- Multi-dimensional arrays
CREATE TABLE matrices (
  id INTEGER,
  data INTEGER[][]
);
```

**Impact:**
- Cannot use array columns
- Must use separate junction tables or JSON

**Complexity:** Medium
- Need to introspect array types
- Need to handle multi-dimensional arrays
- Need to handle array bounds
- Need to preserve array-specific constraints

---

### 8. JSONB Indexes and Operators

**Status:** ❌ Not Supported
**Priority:** Medium (if supporting JSONB type)

```sql
CREATE TABLE products (
  id INTEGER,
  data JSONB
);

-- GIN index on JSONB column
CREATE INDEX idx_data ON products USING gin(data);

-- Expression index on JSONB path
CREATE INDEX idx_category ON products ((data->>'category'));
```

**Impact:**
- If JSONB type is used, indexes not optimized
- JSON queries will be slow without proper indexes

**Complexity:** Medium-High (depends on JSONB type support)

---

### 9. Table Access Methods

**Status:** ❌ Not Supported
**Priority:** Very Low

```sql
-- PostgreSQL 12+ custom table access methods
CREATE TABLE audit_log (...) USING heap;  -- default
-- Future: columnar storage, other access methods
```

**Impact:**
- Cannot specify storage engine
- Not commonly used

**Complexity:** Low
- Need to introspect table AM
- Future PostgreSQL feature

---

### 10. Exclusion Constraints with Operators

**Status:** ❌ Not Supported (covered earlier but worth emphasizing)
**Priority:** Low

```sql
CREATE TABLE reservations (
  room_id INTEGER,
  during TSRANGE,
  EXCLUDE USING gist (
    room_id WITH =,
    during WITH &&
  )
);
```

**Complexity:** High (requires GiST index support)

---

## SQLite-Specific Features

Features unique to SQLite.

### 1. WITHOUT ROWID Tables

**Status:** ❌ Not Supported
**Priority:** Medium

```sql
CREATE TABLE config (
  key TEXT PRIMARY KEY,
  value TEXT
) WITHOUT ROWID;
```

**Impact:**
- Cannot optimize tables for clustered primary key
- May have suboptimal performance for some schemas
- Storage optimization lost

**Complexity:** Low
- Need to introspect WITHOUT ROWID flag
- Need to generate CREATE TABLE with WITHOUT ROWID
- Need to validate PRIMARY KEY requirement
- Cannot use AUTOINCREMENT with WITHOUT ROWID

**Benefits of WITHOUT ROWID:**
- ~50% storage reduction for appropriate tables
- ~2x faster queries on primary key
- Best for tables with non-integer primary keys or composite primary keys

---

### 2. STRICT Tables

**Status:** ❌ Not Supported
**Priority:** Medium-High

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY,
  email TEXT NOT NULL,
  age INTEGER
) STRICT;
```

**Impact:**
- Cannot enforce type safety at database level
- May insert wrong types silently
- Type coercion may cause unexpected behavior

**Complexity:** Low
- Need to introspect STRICT flag
- Need to generate CREATE TABLE with STRICT
- Need to validate column types (must be INT, INTEGER, REAL, TEXT, BLOB, or ANY)

**Benefits of STRICT:**
- Type safety (prevents inserting TEXT into INTEGER column)
- Better error messages
- SQL standard-like behavior
- Available since SQLite 3.37.0 (2021-11-27)

---

### 3. Generated Columns (STORED/VIRTUAL)

**Status:** ❌ Not Supported
**Priority:** Medium

```sql
CREATE TABLE rectangles (
  width REAL,
  height REAL,
  area REAL GENERATED ALWAYS AS (width * height) VIRTUAL,
  perimeter REAL GENERATED ALWAYS AS (2 * (width + height)) STORED
);
```

**Impact:**
- Cannot introspect generated column expressions
- Generated columns not preserved during migrations
- Must manually maintain computed values in application

**Complexity:** Medium
- Need to introspect generation expressions
- Need to distinguish STORED vs VIRTUAL
- VIRTUAL columns don't take disk space, STORED columns do
- Cannot ALTER TABLE ADD COLUMN a STORED generated column
- Available since SQLite 3.31.0 (2020-01-22)

---

### 4. Partial Indexes (WHERE clause)

**Status:** ❌ Not Supported
**Priority:** Medium (covered in SQL standard but worth noting for SQLite)

```sql
CREATE INDEX idx_active_users ON users (email)
WHERE deleted_at IS NULL;
```

**Impact:** Same as PostgreSQL partial indexes

**Complexity:** Medium (same as PostgreSQL)

---

### 5. AUTOINCREMENT

**Status:** ⚠️ Partial Support
**Current:** Default values may include AUTOINCREMENT-like behavior
**Priority:** Low

```sql
CREATE TABLE users (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT
);
```

**Impact:**
- AUTOINCREMENT prevents reuse of deleted rowids
- May not preserve AUTOINCREMENT flag exactly
- Usually not needed (regular INTEGER PRIMARY KEY is sufficient)

**Complexity:** Low
- Need to detect AUTOINCREMENT in PRIMARY KEY
- Need to generate with AUTOINCREMENT keyword
- Note: AUTOINCREMENT incompatible with WITHOUT ROWID

---

### 6. Foreign Key Actions: RESTRICT vs NO ACTION

**Status:** ⚠️ May Not Distinguish
**Priority:** Low

```sql
CREATE TABLE child (
  id INTEGER PRIMARY KEY,
  parent_id INTEGER,
  FOREIGN KEY (parent_id) REFERENCES parent(id)
    ON DELETE RESTRICT  -- vs NO ACTION
);
```

**Impact:**
- RESTRICT checks immediately, NO ACTION can be deferred
- SQLite treats them the same (both check immediately)
- Minor semantic difference

**Complexity:** Low
- Need to preserve exact ON DELETE/ON UPDATE action
- May already be preserved as text

---

### 7. Multiple Databases (ATTACH DATABASE)

**Status:** ❌ Not Supported
**Priority:** Low

```sql
ATTACH DATABASE 'other.db' AS other;

SELECT * FROM main.users u
JOIN other.orders o ON u.id = o.user_id;
```

**Impact:**
- Cannot introspect attached databases
- Cross-database queries not in schema
- Out of scope for single-database schema management

**Complexity:** N/A - Out of scope

---

### 8. Virtual Tables (FTS5, etc.)

**Status:** ❌ Not Supported
**Priority:** Low-Medium

```sql
-- Full-text search virtual table
CREATE VIRTUAL TABLE documents USING fts5(title, body);

-- R-Tree for spatial indexing
CREATE VIRTUAL TABLE spatial_index USING rtree(
  id,
  min_x, max_x,
  min_y, max_y
);
```

**Impact:**
- Cannot introspect virtual table definitions
- Cannot preserve full-text search tables
- Must manually recreate virtual tables

**Complexity:** High
- Need to introspect virtual table type (fts5, rtree, etc.)
- Need to introspect module-specific options
- Each module has different syntax
- Virtual tables don't appear like regular tables

---

### 9. Indexes on Expressions

**Status:** ❌ Not Supported
**Priority:** Medium (covered in SQL standard section)

```sql
CREATE INDEX idx_lower_email ON users (LOWER(email));
```

Same as PostgreSQL expression indexes.

---

### 10. ON CONFLICT Clauses

**Status:** ❌ Not Supported (insertion/update behavior, not DDL)
**Priority:** N/A

```sql
-- This is DML, not DDL
INSERT INTO users (id, email) VALUES (1, 'test@example.com')
ON CONFLICT (email) DO UPDATE SET email = excluded.email;
```

**Note:** ON CONFLICT is a DML feature, not DDL. Out of scope for schema management.

---

## Implementation Priority

Suggested order of implementation based on user impact and complexity:

### High Priority (P0)

1. **CHECK Constraints** - Essential for data integrity
   - Impact: High
   - Complexity: Medium
   - Benefits: Critical validation logic

2. **STRICT Tables (SQLite)** - Type safety
   - Impact: High (for SQLite users)
   - Complexity: Low
   - Benefits: Prevents type errors

3. **Views** - Very commonly used
   - Impact: High
   - Complexity: Medium-High
   - Benefits: Reusable queries, abstraction

### Medium-High Priority (P1)

4. **Generated/Computed Columns** - Modern SQL feature
   - Impact: Medium-High
   - Complexity: Medium-High
   - Benefits: Automatic calculations

5. **Named UNIQUE Constraints** - Better than indexes
   - Impact: Medium
   - Complexity: Low-Medium
   - Benefits: Clearer intent, deferrable

6. **WITHOUT ROWID (SQLite)** - Performance optimization
   - Impact: Medium (for SQLite users)
   - Complexity: Low
   - Benefits: 50% storage reduction, 2x speed

7. **PostgreSQL Extensions** - Enable advanced features
   - Impact: Medium-High
   - Complexity: Medium
   - Benefits: Unlocks many other features

8. **Enum Types (Postgres)** - Type safety
   - Impact: Medium
   - Complexity: Medium-High
   - Benefits: Constrained values

### Medium Priority (P2)

9. **Partial Indexes** - Performance optimization
   - Impact: Medium
   - Complexity: Medium
   - Benefits: Smaller, faster indexes

10. **Expression Indexes** - Performance optimization
    - Impact: Medium
    - Complexity: Medium-High
    - Benefits: Optimize computed queries

11. **Sequences** - Explicit ID management
    - Impact: Medium
    - Complexity: Medium
    - Benefits: Shared sequences, control

12. **IDENTITY Columns (Postgres)** - Modern auto-increment
    - Impact: Medium
    - Complexity: Medium
    - Benefits: Standard-compliant

13. **DEFERRABLE Constraints** - Complex migrations
    - Impact: Medium
    - Complexity: Medium-High
    - Benefits: Circular dependencies

14. **Table Partitioning** - Large table management
    - Impact: Medium (for large databases)
    - Complexity: Very High
    - Benefits: Query performance, maintenance

15. **Comments** - Documentation
    - Impact: Medium
    - Complexity: Low
    - Benefits: Self-documenting schema

### Lower Priority (P3)

16. **Domain Types (Postgres)** - Reusable constraints
    - Impact: Low-Medium
    - Complexity: Medium
    - Benefits: DRY constraints

17. **Materialized Views (Postgres)** - Performance
    - Impact: Low-Medium
    - Complexity: High
    - Benefits: Precomputed results

18. **Index Types (Postgres)** - Advanced indexing
    - Impact: Low-Medium
    - Complexity: Medium
    - Benefits: Specialized indexes

19. **Triggers** - Database logic
    - Impact: Low-Medium
    - Complexity: High
    - Benefits: Automatic actions

20. **Advanced Data Types** - Postgres arrays, JSON, etc.
    - Impact: Varies
    - Complexity: High
    - Benefits: Richer data model

### Very Low Priority (P4)

21. **Composite Types (Postgres)** - Rarely used
22. **Range Types (Postgres)** - Specialized use case
23. **Table Inheritance (Postgres)** - Legacy feature
24. **Full Text Search** - Specialized
25. **Row-Level Security (Postgres)** - Security policies
26. **Stored Procedures/Functions** - Database logic
27. **EXCLUSION Constraints (Postgres)** - Specialized
28. **Column Collations** - Rarely changed
29. **Unlogged Tables (Postgres)** - Performance tuning
30. **Storage Parameters** - Performance tuning

---

## Notes

- This document will be updated as features are implemented
- Priority may change based on user feedback and use cases
- Some features may be split into multiple phases
- Cross-database compatibility should be considered for each feature

---

## Contributing

If you need a feature listed here, please:
1. Open an issue on GitHub describing your use case
2. Include example SQL showing what you need
3. Explain the impact of not having this feature
4. Consider contributing a PR if you can!

---

## References

- [PostgreSQL Documentation](https://www.postgresql.org/docs/current/)
- [SQLite Documentation](https://www.sqlite.org/docs.html)
- [SQL:2016 Standard](https://en.wikipedia.org/wiki/SQL:2016)
- [Modern SQLite Blog](https://antonz.org/) - SQLite features
- [Postgres Weekly](https://postgresweekly.com/) - PostgreSQL updates
