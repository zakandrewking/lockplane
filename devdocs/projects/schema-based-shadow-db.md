# Schema-Based Shadow Database Support

> **Design Decision**: SQLite shadow databases will use `:memory:` as the default instead of temporary files. This provides the fastest possible performance, automatic cleanup, and zero configuration. Users can still override with an explicit file path for debugging if needed.

## Progress Checklist

### Phase 1: Research & Design ✅
- [x] Analyze current shadow DB implementation
- [x] Design schema-based approach for PostgreSQL
- [x] Design approach for SQLite/libSQL
- [x] Identify security considerations
- [x] Design configuration UX
- [x] Document trade-offs

### Phase 2: Core Implementation
- [ ] Add schema support to PostgreSQL driver
- [ ] Implement schema introspection for PostgreSQL
- [ ] Implement schema-aware SQL generation
- [ ] Add schema configuration to environment config
- [ ] Update connection string parsing for schemas
- [ ] Add schema isolation for shadow operations

### Phase 3: SQLite/libSQL Implementation
- [ ] Design file-based shadow DB for SQLite
- [ ] Implement temporary file management
- [ ] Add cleanup mechanisms
- [ ] Handle in-memory databases appropriately

### Phase 4: Configuration & UX
- [ ] Update `lockplane init` wizard with schema options
- [ ] Add `--shadow-schema` CLI flag
- [ ] Update environment resolution logic
- [ ] Add validation for schema configurations
- [ ] Provide helpful error messages

### Phase 5: Testing
- [ ] Unit tests for schema operations
- [ ] Integration tests with real Postgres
- [ ] SQLite shadow DB tests
- [ ] Supabase-specific workflow tests
- [ ] Security isolation tests

### Phase 6: Documentation
- [ ] Update README with schema-based examples
- [ ] Update Supabase guides
- [ ] Create migration guide for existing users
- [ ] Document security model
- [ ] Add troubleshooting section

### Phase 7: Advanced Features (Future)
- [ ] Automatic schema cleanup
- [ ] Schema permission validation
- [ ] Multi-tenant support
- [ ] Schema templates

---

## Context

### The Problem

**Current state**: Users must run a separate database instance as a shadow database for testing migrations. This creates friction:

1. **Local development**: Running two PostgreSQL instances locally is cumbersome
   - Supabase users run `supabase start` (port 5432) but need a second instance (port 5433)
   - Requires Docker Compose or manual setup
   - Resource intensive (2x database processes)

2. **Production testing**: Users need a separate production shadow DB
   - Extra cost (another database instance)
   - More credentials to manage
   - Network complexity

3. **SQLite/libSQL**: Currently requires two database instances/files
   - SQLite: Two file paths with cleanup responsibilities
   - libSQL/Turso: Need a second remote database (extra cost)
   - File management overhead

**User pain points**:
```bash
# Current workflow - complex!
docker run -d -p 5433:5432 postgres  # Shadow DB
export SHADOW_DATABASE_URL=postgres://...@localhost:5433/...
lockplane apply migration.json
```

### The Solution

**Use database schemas (PostgreSQL) or temporary files (SQLite) for shadow databases.**

**PostgreSQL**: Use a dedicated schema within the same database
```bash
# New workflow - simple!
export SHADOW_SCHEMA=lockplane_shadow
lockplane apply migration.json  # Uses same DB, different schema
```

**SQLite**: Use temporary files that auto-cleanup
```bash
# Automatic - no configuration needed!
lockplane apply migration.json  # Creates temp shadow DB, cleans up after
```

---

## Goals

### Primary Goals

1. **Reduce friction**: Make shadow DB testing "just work" with minimal configuration
2. **Lower costs**: Eliminate need for separate database instances
3. **Maintain safety**: Preserve isolation between production and shadow operations
4. **Backward compatibility**: Existing configurations continue to work

### Non-Goals

1. **Replace separate DBs entirely**: Power users can still use separate instances
2. **Multi-database transactions**: Not attempting cross-database atomicity
3. **Schema management**: Not a general-purpose schema manager

---

## Design

### PostgreSQL: Schema-Based Shadow DB

#### How It Works

PostgreSQL schemas provide namespace isolation within a single database:

```sql
-- Main application schema (typically 'public')
CREATE TABLE public.users (id SERIAL PRIMARY KEY, name TEXT);

-- Shadow schema (isolated namespace)
CREATE SCHEMA lockplane_shadow;
CREATE TABLE lockplane_shadow.users (id SERIAL PRIMARY KEY, name TEXT);

-- Same DB, different schemas - no conflict!
```

**Benefits**:
- Single connection string
- No port conflicts
- Minimal resource overhead
- Native PostgreSQL feature

**Implementation approach**:
1. Connect to main database with regular credentials
2. Create shadow schema if it doesn't exist: `CREATE SCHEMA IF NOT EXISTS lockplane_shadow`
3. Set search path for shadow operations: `SET search_path TO lockplane_shadow`
4. Run migrations in shadow schema
5. Clean up or leave schema for next run (configurable)

#### Configuration Options

**Option 1: Same database, different schema (simple)**
```bash
# .env.local
DATABASE_URL=postgres://user:pass@localhost:5432/mydb
SHADOW_SCHEMA=lockplane_shadow
```

**Option 2: Different database, different schema (production workflow)**
```bash
# .env.production
DATABASE_URL=postgres://user:pass@prod-db.supabase.co:5432/postgres  # Production
SHADOW_DATABASE_URL=postgres://postgres:postgres@localhost:54322/postgres  # Local Supabase
SHADOW_SCHEMA=lockplane_shadow  # Use schema in local Supabase
```

**Option 3: Different database with schema in connection string**
```bash
# .env.local
DATABASE_URL=postgres://user:pass@localhost:5432/mydb
SHADOW_DATABASE_URL=postgres://user:pass@localhost:5432/mydb?search_path=lockplane_shadow
```

**Option 4: CLI flag**
```bash
lockplane apply migration.json --shadow-schema lockplane_shadow
```

**Option 5: Config file**
```toml
# lockplane.toml
[environments.local]
description = "Local development"
shadow_schema = "lockplane_shadow"
```

#### Precedence Order
1. CLI flag `--shadow-schema`
2. `SHADOW_SCHEMA` environment variable
3. Config file `shadow_schema`
4. Schema in `SHADOW_DATABASE_URL` connection string (e.g., `?search_path=...`)
5. Default: no schema (use separate database as-is)

#### Configuration Combinations

**Case 1: Same database, use schema**
```bash
DATABASE_URL=postgres://localhost:5432/mydb
SHADOW_SCHEMA=lockplane_shadow
# Result: Uses same database, schema lockplane_shadow
```

**Case 2: Different database, use schema**
```bash
DATABASE_URL=postgres://prod:5432/mydb      # Production
SHADOW_DATABASE_URL=postgres://localhost:5432/test  # Local test DB
SHADOW_SCHEMA=lockplane_shadow              # Schema in local test DB
# Result: Uses different database (local test), schema lockplane_shadow
```

**Case 3: Different database, no schema**
```bash
DATABASE_URL=postgres://prod:5432/mydb
SHADOW_DATABASE_URL=postgres://localhost:5433/shadow
# Result: Uses different database entirely (traditional approach)
```

**Case 4: No configuration (fallback)**
```bash
DATABASE_URL=postgres://localhost:5432/mydb
# Result: Error - must provide shadow configuration
```

### SQLite/libSQL: In-Memory Shadow DB

SQLite and libSQL don't have schema namespaces, so we use an in-memory database for shadow testing. **This works the same for both local SQLite files and remote libSQL/Turso databases.**

#### How It Works

```go
// Connect to in-memory shadow database
// Same approach for both SQLite (local) and libSQL (remote)
shadowDB, err := sql.Open("sqlite3", ":memory:")
// ... test migrations ...
// Automatic cleanup when connection closes
```

**Benefits**:
- ✅ Fastest possible (no disk I/O, no network latency)
- ✅ Automatic cleanup (no file management)
- ✅ **Zero configuration** required (same for SQLite and libSQL)
- ✅ No disk space concerns
- ✅ Perfect for CI/CD
- ✅ Complete isolation from main database
- ✅ **For libSQL/Turso**: 50% cost savings (no remote shadow instance needed)

**Configuration options** (same for both SQLite and libSQL):

**Option 1: In-memory (default - recommended)**
```bash
# No configuration needed for either SQLite or libSQL!
lockplane apply migration.json
# Uses :memory: automatically
```

**Option 2: Explicit file path (for debugging)**
```bash
# .env.local (works for both SQLite and libSQL)
SHADOW_SQLITE_DB_PATH=/tmp/my-shadow.db
# Useful if you need to inspect shadow DB after run
# Even for libSQL, this creates a LOCAL file for shadow testing
```

**Option 3: Persistent file (for development)**
```bash
# .env.local (works for both SQLite and libSQL)
SHADOW_SQLITE_DB_PATH=./shadow.db
# Keeps shadow DB between runs for inspection
```

#### Why This Works for libSQL/Turso (Remote Databases)

libSQL is SQLite-compatible, so the same `:memory:` approach works perfectly:

```bash
# .env.local
LIBSQL_URL=libsql://mydb.turso.io?authToken=...
# No shadow config needed - uses :memory: automatically!
```

**Key point**: Shadow testing happens **locally** (`:memory:`) even though your main database is remote. This provides:
- ✅ 50% cost savings (no remote shadow instance)
- ✅ Fast testing (no network latency for shadow operations)
- ✅ Same validation guarantees

### Security Considerations

#### PostgreSQL Schema Isolation

**✅ Safe**:
- Schemas provide namespace isolation
- SQL in one schema doesn't affect another
- `DROP TABLE users` in shadow schema doesn't affect `public.users`
- PostgreSQL permissions model works at schema level

**Permissions required**:
```sql
-- User needs these permissions
CREATE SCHEMA lockplane_shadow;        -- CREATE privilege on database
SET search_path TO lockplane_shadow;   -- Can set own search_path
CREATE TABLE ...;                       -- CREATE privilege in schema
```

**Security best practices**:
1. **Development**: Use dedicated user with limited permissions
   ```sql
   CREATE USER lockplane_dev WITH PASSWORD '...';
   GRANT CREATE ON DATABASE mydb TO lockplane_dev;
   GRANT ALL ON SCHEMA lockplane_shadow TO lockplane_dev;
   ```

2. **Production**: More restricted approach
   ```sql
   -- Read-only user for introspection
   CREATE USER lockplane_prod WITH PASSWORD '...';
   GRANT CONNECT ON DATABASE mydb TO lockplane_prod;
   GRANT USAGE ON SCHEMA public TO lockplane_prod;

   -- Separate shadow database for production testing
   -- (Don't use production DB for shadow operations)
   ```

**Risk mitigation**:
- **Accidental schema mix-up**: Validate schema before operations
- **Permission errors**: Graceful fallback to separate DB if schema creation fails
- **Production safety**: Recommend separate shadow DB for production environments

#### SQLite/libSQL Isolation

**✅ Safe**:
- Completely separate file = complete isolation
- Temporary files prevent accidental data loss
- Auto-cleanup prevents disk space issues

**Risks**:
- **Disk space**: Temporary files consume space (mitigated by cleanup)
- **File permissions**: Temp directory must be writable (standard requirement)

### Implementation Details

#### 1. Driver Interface Changes

Add schema awareness to `database.Driver`:

```go
// database/interface.go
type Driver interface {
    // ... existing methods ...

    // Schema support (PostgreSQL only)
    SupportsSchemas() bool
    CreateSchema(ctx context.Context, db *sql.DB, schemaName string) error
    DropSchema(ctx context.Context, db *sql.DB, schemaName string) error
    SetSchema(ctx context.Context, db *sql.DB, schemaName string) error
    ListSchemas(ctx context.Context, db *sql.DB) ([]string, error)
}
```

#### 2. PostgreSQL Implementation

```go
// database/postgres/driver.go
func (d *Driver) SupportsSchemas() bool {
    return true
}

func (d *Driver) CreateSchema(ctx context.Context, db *sql.DB, schemaName string) error {
    _, err := db.ExecContext(ctx, fmt.Sprintf("CREATE SCHEMA IF NOT EXISTS %s",
        pq.QuoteIdentifier(schemaName)))
    return err
}

func (d *Driver) SetSchema(ctx context.Context, db *sql.DB, schemaName string) error {
    _, err := db.ExecContext(ctx, fmt.Sprintf("SET search_path TO %s",
        pq.QuoteIdentifier(schemaName)))
    return err
}
```

#### 3. SQLite Implementation

```go
// database/sqlite/driver.go
func (d *Driver) SupportsSchemas() bool {
    return false  // SQLite doesn't support schemas
}

// SQLite uses :memory: as default shadow DB (fastest, auto-cleanup)
func (d *Driver) DefaultShadowURL() string {
    return ":memory:"
}
```

#### 4. Configuration Updates

```go
// internal/config/environment.go
type ResolvedEnvironment struct {
    Name              string
    DatabaseURL       string
    ShadowDatabaseURL string  // Existing
    ShadowSchema      string  // NEW: for PostgreSQL
    SchemaPath        string
    // ... rest ...
}

func ResolveEnvironment(config *Config, name string) (*ResolvedEnvironment, error) {
    // ... existing logic ...

    // NEW: Check for shadow schema configuration
    if shadowSchema := values["SHADOW_SCHEMA"]; shadowSchema != "" {
        resolved.ShadowSchema = shadowSchema
    }

    // Configuration logic supports multiple scenarios:
    // 1. SHADOW_SCHEMA only: Use same database with different schema
    // 2. SHADOW_DATABASE_URL + SHADOW_SCHEMA: Use different database with schema
    // 3. SHADOW_DATABASE_URL only: Use different database without schema (traditional)

    // If shadow schema is set but no separate shadow URL, use main DB with schema
    if resolved.ShadowSchema != "" && resolved.ShadowDatabaseURL == "" {
        resolved.ShadowDatabaseURL = resolved.DatabaseURL
    }
    // If both are set, that's valid too - use different DB with schema in that DB

    return resolved, nil
}
```

#### 5. Apply Command Updates

```go
// main.go - runApply()
func runApply(args []string) {
    // ... existing setup ...

    // NEW: Determine shadow strategy based on driver and config
    var shadowDB *sql.DB
    var cleanupShadow func()

    if !*skipShadow {
        if resolvedTarget.ShadowSchema != "" && mainDriverType == "postgres" {
            // Use schema-based shadow DB
            // This supports both:
            // 1. Same database (shadowDB URL == mainDB URL)
            // 2. Different database (shadowDB URL != mainDB URL)

            // Connect to shadow database (might be same as main DB)
            shadowDB, err = sql.Open(shadowDriverName, resolvedShadow.ShadowDatabaseURL)
            if err != nil {
                log.Fatalf("Failed to connect to shadow database: %v", err)
            }

            // Create shadow schema in the shadow database
            if err := mainDriver.CreateSchema(ctx, shadowDB, resolvedTarget.ShadowSchema); err != nil {
                log.Fatalf("Failed to create shadow schema: %v", err)
            }

            // Set search path for shadow operations
            if err := mainDriver.SetSchema(ctx, shadowDB, resolvedTarget.ShadowSchema); err != nil {
                log.Fatalf("Failed to set shadow schema: %v", err)
            }

            // Show clear message about what we're doing
            if resolvedShadow.ShadowDatabaseURL == resolvedTarget.DatabaseURL {
                fmt.Fprintf(os.Stderr, "🔍 Testing migration on shadow schema %q (same database)...\n",
                    resolvedTarget.ShadowSchema)
            } else {
                fmt.Fprintf(os.Stderr, "🔍 Testing migration on shadow schema %q in separate database...\n",
                    resolvedTarget.ShadowSchema)
            }
        } else if mainDriverType == "sqlite" || mainDriverType == "libsql" {
            // Use in-memory database for SQLite (fastest, auto-cleanup)
            shadowConnStr := resolvedShadow.ShadowDatabaseURL
            if shadowConnStr == "" {
                // Default to :memory: for SQLite
                shadowConnStr = ":memory:"
            }

            shadowDB, err = sql.Open(shadowDriverName, shadowConnStr)
            if err != nil {
                log.Fatalf("Failed to connect to shadow database: %v", err)
            }

            if shadowConnStr == ":memory:" {
                fmt.Fprintf(os.Stderr, "🔍 Testing migration on in-memory shadow DB...\n")
            } else {
                fmt.Fprintf(os.Stderr, "🔍 Testing migration on shadow DB: %s...\n", shadowConnStr)
            }
        } else {
            // Existing behavior: separate database
            shadowDB, err = sql.Open(...)
            fmt.Fprintf(os.Stderr, "🔍 Testing migration on shadow database...\n")
        }
    }

    // ... rest of apply logic ...
}
```

#### 6. Introspection Updates

When introspecting, ensure we target the correct schema:

```go
// database/postgres/introspector.go
func (i *Introspector) GetTables(ctx context.Context, db *sql.DB, schema string) ([]string, error) {
    query := `
        SELECT table_name
        FROM information_schema.tables
        WHERE table_schema = $1
        ORDER BY table_name
    `

    if schema == "" {
        schema = "public"  // Default schema
    }

    rows, err := db.QueryContext(ctx, query, schema)
    // ... rest ...
}
```

### User Experience

#### Powerful Combination: Different Database + Schema

The most flexible approach combines both features:

```bash
# .env.production
DATABASE_URL=postgres://prod-db.supabase.co:5432/postgres          # Production
SHADOW_DATABASE_URL=postgres://postgres:postgres@localhost:54322/postgres  # Local Supabase
SHADOW_SCHEMA=lockplane_shadow                                     # Schema in local
```

**Benefits of this approach**:
- ✅ **Safe**: Shadow testing happens on completely different database
- ✅ **Clean**: Schema isolation within shadow database
- ✅ **Fast**: Local testing (no production network latency)
- ✅ **Cost-effective**: Reuses local Supabase instance
- ✅ **Easy cleanup**: Just drop the schema, not entire database
- ✅ **Multiple projects**: Each project can have its own schema in shared local DB

**Real-world use cases**:
1. **Production migrations**: Test against production structure locally before deploying
2. **Multiple environments**: Different production DBs, same local shadow with different schemas
3. **CI/CD**: Each pipeline run gets its own schema in shared test database
4. **Team development**: Shared local DB, per-developer schemas

#### Supabase Local Development (Common Case)

**Before (complex)**:
```bash
# Terminal 1
supabase start  # Port 5432

# Terminal 2
docker run -d -p 5433:5432 postgres  # Shadow DB
```

```bash
# .env.local
DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres
SHADOW_DATABASE_URL=postgres://postgres:postgres@localhost:5433/postgres
```

**After (simple)**:
```bash
# Terminal 1
supabase start  # Port 5432 only
```

```bash
# .env.local
DATABASE_URL=postgres://postgres:postgres@localhost:5432/postgres
SHADOW_SCHEMA=lockplane_shadow  # Same DB, different schema!
```

#### libSQL/Turso Projects

**Before (complex + costly)**:
```bash
# Need TWO Turso databases
turso db create myapp        # Production ($$$)
turso db create myapp-shadow # Shadow ($$$)
```

```bash
# .env.local
DATABASE_URL=libsql://myapp-user.turso.io?authToken=token1
SHADOW_DATABASE_URL=libsql://myapp-shadow-user.turso.io?authToken=token2
```

**After (simple + free)**:
```bash
# Only ONE Turso database needed
turso db create myapp  # Production only
```

```bash
# .env.local
DATABASE_URL=libsql://myapp-user.turso.io?authToken=token1
# No shadow config needed - uses local :memory: automatically!
```

**Why this is better**:
- ✅ 50% cost reduction (no shadow database needed)
- ✅ Zero configuration (automatic)
- ✅ Faster shadow testing (local, no network latency)
- ✅ Same validation guarantees

#### Supabase Production

**Recommended approach**: Use local Supabase with schema for production testing

```bash
# .env.production
DATABASE_URL=postgres://user:pass@db.supabase.co:5432/postgres     # Production (remote)
SHADOW_DATABASE_URL=postgres://postgres:postgres@localhost:54322/postgres  # Local Supabase
SHADOW_SCHEMA=lockplane_shadow                                     # Schema in local DB
```

**Why this is best**:
- ✅ Tests on separate database (safe)
- ✅ Uses schema for easy cleanup
- ✅ Local testing (fast)
- ✅ No risk to production

**Alternative 1**: Use local DB without schema
```bash
# .env.production
DATABASE_URL=postgres://user:pass@db.supabase.co:5432/postgres
SHADOW_DATABASE_URL=postgres://localhost:54322/postgres  # Local test DB
# No schema - uses whole database
```

**Alternative 2**: Use schema in production DB (not recommended)
```bash
# .env.production
DATABASE_URL=postgres://user:pass@db.supabase.co:5432/postgres
SHADOW_SCHEMA=lockplane_shadow_prod  # Schema in production database
```

⚠️ **Security note**: Using a schema in production DB requires careful permission management. The recommended approach (separate database + schema) is safer.

#### SQLite Projects

**Before**:
```bash
# .env.local
SQLITE_DB_PATH=./myapp.db
SHADOW_SQLITE_DB_PATH=./shadow.db  # Must manage manually
```

**After**:
```bash
# .env.local
SQLITE_DB_PATH=./myapp.db
# No shadow config needed - uses :memory: automatically!
```

**For debugging** (if you need to inspect shadow DB):
```bash
# .env.local
SQLITE_DB_PATH=./myapp.db
SHADOW_SQLITE_DB_PATH=./debug-shadow.db
# Shadow DB persists after run for inspection
```

#### libSQL/Turso Projects

```bash
# .env.local
LIBSQL_URL=libsql://mydb.turso.io?authToken=...
# Shadow automatically uses :memory: (local, fast)
```

**Why this works**: libSQL is SQLite-compatible, so `:memory:` provides a fast local shadow for testing remote database migrations.

### Configuration Migration

#### Backward Compatibility

All existing configurations continue to work, and new combinations are supported:

```bash
# OLD: Separate shadow database (still works!)
SHADOW_DATABASE_URL=postgres://...@localhost:5433/...

# NEW: Schema in same database (simple local dev)
SHADOW_SCHEMA=lockplane_shadow

# NEW: Both together (powerful production workflow)
DATABASE_URL=postgres://prod-db:5432/mydb
SHADOW_DATABASE_URL=postgres://localhost:5432/test
SHADOW_SCHEMA=lockplane_shadow
```

**Configuration combinations**:
1. **`SHADOW_DATABASE_URL` only** → Use separate database (traditional)
2. **`SHADOW_SCHEMA` only** → Use schema in same database (new, simple)
3. **Both `SHADOW_DATABASE_URL` + `SHADOW_SCHEMA`** → Use schema in different database (new, powerful)
4. **Neither** → Error (must provide shadow configuration)

#### Migration Guide for Existing Users

**PostgreSQL users**:

1. **Stop shadow database container** (if using Docker)
   ```bash
   docker stop lockplane-shadow  # or whatever you named it
   ```

2. **Update environment config**
   ```bash
   # .env.local - Remove or comment out SHADOW_DATABASE_URL
   # SHADOW_DATABASE_URL=postgres://...@localhost:5433/...

   # Add SHADOW_SCHEMA instead
   SHADOW_SCHEMA=lockplane_shadow
   ```

3. **Test**
   ```bash
   lockplane apply migration.json
   # Should see: "🔍 Testing migration on shadow schema 'lockplane_shadow'..."
   ```

**SQLite users**:

1. **Remove shadow configuration** (no longer needed)
   ```bash
   # .env.local - Remove this line
   # SHADOW_SQLITE_DB_PATH=./shadow.db
   ```

2. **Test**
   ```bash
   lockplane apply migration.json
   # Should see: "🔍 Testing migration on in-memory shadow DB..."
   ```

**Note**: The new default (`:memory:`) is actually **faster** than the old file-based approach!

---

## Testing Plan

### Unit Tests

1. **Schema operations** (`database/postgres/driver_test.go`)
   - Test `CreateSchema()`
   - Test `SetSchema()`
   - Test `DropSchema()`
   - Test `ListSchemas()`

2. **Configuration parsing** (`internal/config/environment_test.go`)
   - Test `SHADOW_SCHEMA` environment variable
   - Test precedence order
   - Test backward compatibility

3. **SQLite in-memory shadow** (`database/sqlite/driver_test.go`)
   - Test `:memory:` default
   - Test explicit file path override
   - Verify performance (should be fast!)

### Integration Tests

1. **PostgreSQL schema-based shadow** (`integration_test.go`)
   ```go
   func TestApplyPlan_WithSchemaShadow(t *testing.T) {
       // Use same DB, different schema
       mainDB := connectToPostgres(t)

       // Set shadow schema
       os.Setenv("SHADOW_SCHEMA", "lockplane_test_shadow")

       // Apply migration
       result := runApply(t, "testdata/plans-json/create_table.json")

       // Verify table exists in main schema
       assertTableExists(t, mainDB, "public", "posts")

       // Verify table does NOT exist in shadow schema
       assertTableNotExists(t, mainDB, "lockplane_test_shadow", "posts")
   }
   ```

2. **SQLite in-memory shadow** (`sqlite_integration_test.go`)
   ```go
   func TestApplyPlan_SQLiteInMemoryShadow(t *testing.T) {
       mainDB := connectToSQLite(t, "./test.db")

       // Don't set SHADOW_SQLITE_DB_PATH (should use :memory: automatically)
       os.Unsetenv("SHADOW_SQLITE_DB_PATH")

       // Apply should work with in-memory shadow
       start := time.Now()
       result := runApply(t, "testdata/plans-json/create_table.json")
       duration := time.Since(start)

       // Verify table exists in main DB only
       assertTableExists(t, mainDB, "posts")

       // Verify it's fast (in-memory should be < 1s for simple migrations)
       if duration > 2*time.Second {
           t.Errorf("In-memory shadow took too long: %v", duration)
       }
   }

   func TestApplyPlan_SQLiteFileShadow(t *testing.T) {
       mainDB := connectToSQLite(t, "./test.db")

       // Override with explicit file path
       os.Setenv("SHADOW_SQLITE_DB_PATH", "./test-shadow.db")
       defer os.Remove("./test-shadow.db")

       // Should use file instead of :memory:
       result := runApply(t, "testdata/plans-json/create_table.json")

       // Verify shadow file was created
       if _, err := os.Stat("./test-shadow.db"); os.IsNotExist(err) {
           t.Error("Shadow DB file was not created")
       }
   }
   ```

3. **Supabase workflow** (`integration_test.go`)
   ```go
   func TestSupabaseLocalWorkflow(t *testing.T) {
       if testing.Short() {
           t.Skip("Skipping Supabase integration test")
       }

       // Simulate Supabase local setup
       db := connectToPostgres(t, "postgres://postgres:postgres@localhost:54322/postgres")

       os.Setenv("SHADOW_SCHEMA", "lockplane_shadow")

       // Full workflow: plan → apply → verify
       // ...
   }
   ```

### Security Tests

1. **Schema isolation**
   ```go
   func TestSchemaIsolation(t *testing.T) {
       // Create table in public schema
       mainDB.Exec("CREATE TABLE public.users (id INT)")

       // Drop table in shadow schema shouldn't affect public
       shadowDB.Exec("SET search_path TO lockplane_shadow")
       shadowDB.Exec("DROP TABLE IF EXISTS users")

       // Verify public.users still exists
       assertTableExists(t, mainDB, "public", "users")
   }
   ```

2. **Permission validation**
   ```go
   func TestSchemPermissions(t *testing.T) {
       // Test with limited-privilege user
       limitedDB := connectWithUser(t, "lockplane_test_user")

       // Should fail gracefully if can't create schema
       err := driver.CreateSchema(ctx, limitedDB, "test_schema")
       if err != nil {
           // Should provide helpful error message
           assert.Contains(t, err.Error(), "permission denied")
       }
   }
   ```

---

## Documentation Updates

### 1. README.md

Add section: "Shadow Database Strategies"

```markdown
## Shadow Database Strategies

Lockplane tests migrations on a shadow database before applying to your production database.
You have several options:

### PostgreSQL: Use a Schema (Recommended)

Use a separate schema in the same database:

```bash
# .env.local
DATABASE_URL=postgres://user:pass@localhost:5432/mydb
SHADOW_SCHEMA=lockplane_shadow
```

**Benefits**: Simple setup, no extra database needed, cost-effective

### PostgreSQL: Use a Separate Database

For maximum isolation (recommended for production):

```bash
# .env.production
DATABASE_URL=postgres://user:pass@prod-db:5432/mydb
SHADOW_DATABASE_URL=postgres://user:pass@test-db:5432/mydb_shadow
```

### SQLite: Automatic (In-Memory)

Shadow database uses an in-memory database (`:memory:`). No configuration needed, and it's the fastest option!

### libSQL/Turso

Uses local in-memory database for shadow testing:

```bash
LIBSQL_URL=libsql://mydb.turso.io?authToken=...
# Shadow automatically uses :memory: (fast!)
```

### 2. docs/supabase-existing-project.md

Update shadow DB section:

```markdown
### Shadow Database Setup

**Option 1: Schema-based (Simplest)**

Use a schema in your Supabase database:

```bash
# .env.supabase
DATABASE_URL=postgres://postgres:<password>@<host>:5432/postgres
SHADOW_SCHEMA=lockplane_shadow
```

**Option 2: Separate Database (More Isolated)**

Use a local Postgres instance:

```bash
docker run -d -p 5433:5432 -e POSTGRES_PASSWORD=postgres postgres
```

```bash
# .env.supabase
DATABASE_URL=postgres://postgres:<password>@<host>:5432/postgres
SHADOW_DATABASE_URL=postgres://postgres:postgres@localhost:5433/postgres
```
```

### 3. New Guide: docs/shadow-database-guide.md

Create comprehensive guide covering:
- What is a shadow database and why?
- Schema vs separate database trade-offs
- Security considerations
- Per-database recommendations
- Troubleshooting

---

## Trade-offs & Considerations

### PostgreSQL Schema Approach

**Pros**:
- ✅ Simple setup (single connection string)
- ✅ No additional database instance needed
- ✅ Lower resource usage
- ✅ Cost-effective (no extra database)
- ✅ Fast (no network overhead between DBs)
- ✅ Native PostgreSQL feature (stable, well-tested)

**Cons**:
- ⚠️ Less isolated than separate database
- ⚠️ Requires CREATE SCHEMA permission
- ⚠️ Schema clutters database (minor issue)
- ⚠️ Risk of schema name conflicts (mitigated by naming)
- ⚠️ Not suitable for highest-security production environments

**When to use**:
- ✅ Local development
- ✅ Staging environments
- ✅ CI/CD pipelines
- ✅ Cost-sensitive deployments
- ❌ Highly-regulated production (use separate DB)

### SQLite In-Memory Approach

**Pros**:
- ✅ Zero configuration
- ✅ Fastest possible (no disk I/O)
- ✅ Automatic cleanup (no file management)
- ✅ Complete isolation
- ✅ No disk space concerns
- ✅ No file permission issues
- ✅ Perfect for CI/CD

**Cons**:
- ⚠️ Can't inspect shadow DB after run (gone when connection closes)
- ⚠️ Not realistic if you need to test disk-specific behavior (rare for schema migrations)

**When to use**:
- ✅ Always (default for SQLite)
- ✅ Can override with file path if you need to debug
- ✅ Especially good for CI/CD pipelines

### libSQL/Turso In-Memory Approach

**Pros**:
- ✅ Zero configuration
- ✅ 50% cost savings (no shadow database instance needed)
- ✅ Fastest possible (local testing, no network latency)
- ✅ Automatic cleanup (no file management)
- ✅ Complete isolation
- ✅ Works because libSQL is SQLite-compatible
- ✅ Tests SQL locally before applying to remote DB

**Cons**:
- ⚠️ Can't inspect shadow DB after run
- ⚠️ Shadow tests SQLite compatibility, not Turso-specific features (usually fine)

**When to use**:
- ✅ Always (default for libSQL/Turso)
- ✅ Saves money on Turso usage
- ✅ Perfect for CI/CD pipelines
- ✅ Override with file path only if debugging

### Separate Database (Existing Approach)

**Pros**:
- ✅ Maximum isolation
- ✅ No permission concerns
- ✅ Can inspect shadow DB independently
- ✅ Traditional approach (well understood)

**Cons**:
- ❌ Complex setup
- ❌ Extra cost (second database instance)
- ❌ More credentials to manage
- ❌ Higher resource usage

**When to use**:
- ✅ Production environments (highest security)
- ✅ Regulated industries
- ✅ When you already have a test database
- ❌ Local development (schema approach is simpler)

---

## Open Questions & Future Work

### Open Questions

1. **Schema cleanup**: Should we `DROP SCHEMA` after each run?
   - **Leaning toward**: Keep schema, just clean tables
   - **Reasoning**: Faster on subsequent runs, user can inspect

2. **Default schema name**: `lockplane_shadow` or something else?
   - **Leaning toward**: `lockplane_shadow`
   - **Reasoning**: Clear purpose, unlikely to conflict

3. **Production recommendations**: Should we warn when using schema in prod?
   - **Leaning toward**: Yes, log warning for production environments
   - **Reasoning**: Encourage best practices

4. **Permission checking**: Should we validate permissions before attempting?
   - **Leaning toward**: Try and fail gracefully with helpful message
   - **Reasoning**: Simpler implementation, clear error messages

### Future Enhancements

1. **Automatic cleanup command**
   ```bash
   lockplane cleanup-shadow --environment local
   ```

2. **Schema permission validation**
   ```bash
   lockplane check-shadow --environment production
   # Reports: "✓ Can create schema" or "❌ Missing CREATE privilege"
   ```

3. **Multi-tenant support**: Use schemas for multi-tenant apps
   ```bash
   SHADOW_SCHEMA=tenant_{{TENANT_ID}}_shadow
   ```

4. **Schema templates**: Pre-configure schemas with extensions
   ```toml
   [shadow_schema]
   name = "lockplane_shadow"
   extensions = ["uuid-ossp", "pg_trgm"]
   ```

5. **Smart fallback**: Auto-fallback to temp DB if schema creation fails
   ```go
   if err := createSchema(); err != nil {
       log.Warn("Can't create schema, using in-memory shadow DB...")
       return createTempDB()
   }
   ```

---

## Implementation Phases

### Phase 2: Core Implementation (2-3 days)

**Tasks**:
1. Add schema methods to `database.Driver` interface
2. Implement PostgreSQL schema support
3. Add `ShadowSchema` to `ResolvedEnvironment`
4. Update connection logic in `runApply()`
5. Basic testing

**Deliverables**:
- PostgreSQL schema-based shadow works end-to-end
- Backward compatibility maintained

### Phase 3: SQLite Implementation (0.5 days)

**Tasks**:
1. Update `runApply()` to use `:memory:` as default for SQLite
2. Allow override with explicit file path
3. Testing

**Deliverables**:
- SQLite in-memory shadow works end-to-end
- Faster than previous file-based approach

### Phase 4: Configuration & UX (1-2 days)

**Tasks**:
1. Update `lockplane init` wizard
2. Add CLI flags
3. Implement precedence logic
4. Add validation and error messages
5. Testing

**Deliverables**:
- Great UX for configuration
- Clear, helpful error messages

### Phase 5: Testing (2 days)

**Tasks**:
1. Write comprehensive unit tests
2. Write integration tests
3. Test Supabase workflow specifically
4. Security tests

**Deliverables**:
- High test coverage
- Confidence in security model

### Phase 6: Documentation (1 day)

**Tasks**:
1. Update README
2. Update Supabase guides
3. Create shadow DB guide
4. Update CLI help text
5. Create migration guide

**Deliverables**:
- Users can easily adopt new approach
- Clear guidance on which strategy to use

**Total estimate**: 6.5-8.5 days (slightly faster with `:memory:` approach for SQLite)

---

## Success Criteria

### Functional

- [ ] Schema-based shadow works for PostgreSQL
- [ ] In-memory shadow works for SQLite (`:memory:`)
- [ ] Backward compatibility maintained (existing configs work)
- [ ] Clear error messages for permission issues
- [ ] Graceful fallback when schema unavailable
- [ ] SQLite shadow is noticeably faster than file-based

### User Experience

- [ ] Supabase users can use single database for local dev
- [ ] Supabase users can combine different database + schema for production testing
- [ ] SQLite users need zero shadow configuration
- [ ] libSQL/Turso users need zero shadow configuration (saves 50% cost)
- [ ] `lockplane init` suggests appropriate shadow strategy
- [ ] Documentation clearly explains trade-offs and configuration combinations

### Quality

- [ ] Comprehensive test coverage
- [ ] Security model documented and validated
- [ ] Performance impact minimal (schemas are fast)
- [ ] No regressions in existing functionality

---

## Security Model

### PostgreSQL Schema Isolation

**What's isolated**:
- ✅ Table names (no conflicts)
- ✅ Data (completely separate)
- ✅ Indexes (separate per schema)
- ✅ Foreign keys (within schema only)

**What's shared**:
- Connection pool
- Database roles and permissions
- Extensions
- Background processes (VACUUM, etc.)

**Permission model**:
```sql
-- Minimal permissions for schema-based shadow
GRANT CONNECT ON DATABASE mydb TO lockplane_user;
GRANT CREATE ON DATABASE mydb TO lockplane_user;  -- For CREATE SCHEMA
GRANT USAGE ON SCHEMA lockplane_shadow TO lockplane_user;
GRANT ALL ON ALL TABLES IN SCHEMA lockplane_shadow TO lockplane_user;
```

**Attack vectors & mitigations**:

1. **SQL injection in schema name**
   - ✅ Mitigated: Use `pq.QuoteIdentifier()` for all schema names
   - ✅ Validate schema names match `^[a-zA-Z_][a-zA-Z0-9_]*$`

2. **Accidental operations on wrong schema**
   - ✅ Mitigated: Explicit `SET search_path` before shadow operations
   - ✅ Reset search path after shadow operations
   - ✅ Log schema being used

3. **Permission escalation**
   - ✅ Mitigated: Schema permissions don't grant database-level access
   - ✅ User can only affect schemas they have permissions for

4. **Resource exhaustion**
   - ⚠️ Potential: Shadow operations use same connection pool
   - ✅ Mitigated: Shadow testing is transactional (rolled back)
   - ✅ Brief duration (seconds to minutes)

### SQLite In-Memory Isolation

**What's isolated**:
- ✅ Everything (separate in-memory database = complete isolation)
- ✅ No filesystem concerns

**Permission model**:
- ✅ No special permissions required (pure memory)

**Attack vectors & mitigations**:

1. **Memory exhaustion**
   - ⚠️ Potential: Very large schemas could consume RAM
   - ✅ Mitigated: Shadow testing is brief (seconds)
   - ✅ Schema migrations rarely exceed 100 MB
   - ✅ OS will kill process if OOM (safe failure mode)

2. **Connection interference**
   - ✅ Mitigated: Each `:memory:` connection is independent
   - ✅ No shared state between shadow and main DB

3. **Data loss on crash**
   - ✅ Not a concern: Shadow DB is intentionally ephemeral
   - ✅ We only care that SQL is valid, not the data

### Production Recommendations

**Local/Staging**: Schema-based shadow is safe and recommended
```bash
SHADOW_SCHEMA=lockplane_shadow
```

**Production**: Separate database recommended for maximum isolation
```bash
# Use local test DB as shadow for production migrations
DATABASE_URL=postgres://prod-db:5432/myapp          # Production
SHADOW_DATABASE_URL=postgres://localhost:5432/test  # Local test instance
```

**High-security production**: Separate cloud database
```bash
DATABASE_URL=postgres://prod-db:5432/myapp
SHADOW_DATABASE_URL=postgres://test-db:5432/myapp_test  # Separate instance
```

---

## Conclusion

This feature makes Lockplane significantly more accessible while maintaining safety:

1. **Removes friction**: No more managing multiple database instances for local development
2. **Lowers costs**: Single database instance is enough (or reuse local DB for remote testing)
3. **Maintains safety**: PostgreSQL schemas provide strong isolation
4. **Great UX**: SQLite automatic shadow requires zero configuration
5. **Flexible**: Users can choose their isolation level based on needs
6. **Powerful combinations**: Can mix different databases with schema isolation

**Recommended configurations**:

| Use Case | Configuration | Benefits |
|----------|--------------|----------|
| **PostgreSQL local dev** | `SHADOW_SCHEMA=lockplane_shadow` | Simple, same DB |
| **PostgreSQL production** | `SHADOW_DATABASE_URL` (local) + `SHADOW_SCHEMA` | Safe, fast, clean |
| **SQLite local** | (automatic `:memory:`) | Zero config, fastest |
| **libSQL/Turso remote** | (automatic `:memory:`) | Zero config, cost savings |
| **CI/CD PostgreSQL** | `SHADOW_SCHEMA` per pipeline run | Isolated, parallel-safe |
| **Team development** | Same local DB, different `SHADOW_SCHEMA` per dev | Shared resources |

**Key innovation**: The combination of `SHADOW_DATABASE_URL` + `SHADOW_SCHEMA` provides the best of both worlds:
- Test production migrations locally (safe)
- Use schemas for easy cleanup (simple)
- Reuse local database infrastructure (cost-effective)
- Multiple projects/pipelines can share one local DB (efficient)

The implementation is straightforward, leveraging native PostgreSQL features and Go's standard library for SQLite. The main complexity is in the configuration resolution logic, which already exists and just needs extension.

**Next steps**: Begin Phase 2 implementation with PostgreSQL schema support.
