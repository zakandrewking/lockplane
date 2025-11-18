# Multi-Schema Support and RLS Policies

## Progress Checklist
- [ ] Phase 1: Add Policy type to schema
- [ ] Phase 2: Add config support for multiple schemas
- [ ] Phase 3: Update introspection for multi-schema
- [ ] Phase 4: Add policy introspection
- [ ] Phase 5: Add policy DDL generation
- [ ] Phase 6: Add schema-qualified table support
- [ ] Phase 7: Testing and documentation

## Context

Users need to manage tables and policies across multiple PostgreSQL schemas (e.g., `public`, `storage`, `auth`). Currently lockplane only works with `current_schema()`.

### Use Case Example

```sql
-- Supabase storage schema
CREATE POLICY "Public Access - Select" ON storage.objects
    FOR SELECT
    USING (bucket_id = 'genomes');
```

## Goals

1. **Multi-schema introspection**: Query multiple schemas in a single run
2. **Schema-qualified tables**: Support `schema.table` syntax
3. **RLS policy management**: Create, modify, and drop policies
4. **Config-driven**: Specify schemas to manage in `lockplane.toml`

## Implementation Phases

### Phase 1: Add Policy Type to Schema

Add `Policy` type to `database/interface.go`:

```go
// Policy represents a Row Level Security policy
type Policy struct {
    Name       string   `json:"name"`
    TableName  string   `json:"table_name"`
    Command    string   `json:"command"`    // SELECT, INSERT, UPDATE, DELETE, ALL
    Permissive bool     `json:"permissive"` // true = PERMISSIVE, false = RESTRICTIVE
    Roles      []string `json:"roles"`
    Using      *string  `json:"using,omitempty"`      // USING clause
    WithCheck  *string  `json:"with_check,omitempty"` // WITH CHECK clause
}

// Table struct - add Policies field
type Table struct {
    Name        string       `json:"name"`
    Schema      string       `json:"schema,omitempty"` // NEW: schema name
    Columns     []Column     `json:"columns"`
    Indexes     []Index      `json:"indexes"`
    ForeignKeys []ForeignKey `json:"foreign_keys,omitempty"`
    RLSEnabled  bool         `json:"rls_enabled,omitempty"`
    Policies    []Policy     `json:"policies,omitempty"` // NEW
}
```

### Phase 2: Add Config Support for Multiple Schemas

Update `internal/config/config.go`:

```go
type EnvironmentConfig struct {
    Description       string   `toml:"description"`
    DatabaseURL       string   `toml:"database_url"`
    ShadowDatabaseURL string   `toml:"shadow_database_url"`
    SchemaPath        string   `toml:"schema_path"`
    Schemas           []string `toml:"schemas"` // NEW: list of schemas to manage
}
```

Example `lockplane.toml`:

```toml
[environments.local]
description = "Local development with Supabase"
database_url = "postgresql://postgres:postgres@127.0.0.1:54322/postgres"
shadow_database_url = "postgresql://postgres:postgres@127.0.0.1:54322/postgres"
shadow_schema = "lockplane_shadow"
schemas = ["public", "storage", "auth"]  # NEW
```

### Phase 3: Update Introspection for Multi-Schema

Update `database/postgres/introspector.go`:

```go
// IntrospectSchema now accepts optional schema list
func (i *Introspector) IntrospectSchema(ctx context.Context, db *sql.DB) (*Schema, error) {
    return i.IntrospectSchemas(ctx, db, []string{}) // default: current_schema()
}

// IntrospectSchemas introspects multiple schemas
func (i *Introspector) IntrospectSchemas(ctx context.Context, db *sql.DB, schemas []string) (*Schema, error) {
    if len(schemas) == 0 {
        // Default: use current_schema()
        schemas = []string{getCurrentSchema(ctx, db)}
    }

    schema := &Schema{Tables: []Table{}, Dialect: DialectPostgres}

    for _, schemaName := range schemas {
        tables, err := i.GetTablesInSchema(ctx, db, schemaName)
        // ... collect tables from each schema
    }

    return schema, nil
}
```

### Phase 4: Add Policy Introspection

Add to `database/postgres/introspector.go`:

```go
// GetPolicies returns all policies for a given table
func (i *Introspector) GetPolicies(ctx context.Context, db *sql.DB, schemaName, tableName string) ([]Policy, error) {
    query := `
        SELECT
            pol.polname AS policy_name,
            pol.polcmd AS command,
            pol.polpermissive AS permissive,
            ARRAY(
                SELECT rolname FROM pg_roles
                WHERE oid = ANY(pol.polroles)
            ) AS roles,
            pg_get_expr(pol.polqual, pol.polrelid) AS using_expr,
            pg_get_expr(pol.polwithcheck, pol.polrelid) AS with_check_expr
        FROM pg_policy pol
        JOIN pg_class c ON c.oid = pol.polrelid
        JOIN pg_namespace n ON n.oid = c.relnamespace
        WHERE n.nspname = $1
          AND c.relname = $2
        ORDER BY pol.polname
    `

    rows, err := db.QueryContext(ctx, query, schemaName, tableName)
    // ... scan and return policies
}
```

### Phase 5: Add Policy DDL Generation

Add to `database/postgres/generator.go`:

```go
// CreatePolicy generates SQL to create a policy
func (g *Generator) CreatePolicy(schemaName, tableName string, policy Policy) (sql string, description string) {
    var sb strings.Builder

    sb.WriteString(fmt.Sprintf("CREATE POLICY %s ON %s.%s\n",
        quoteIdentifier(policy.Name),
        quoteIdentifier(schemaName),
        quoteIdentifier(tableName)))

    // AS PERMISSIVE/RESTRICTIVE
    if !policy.Permissive {
        sb.WriteString("    AS RESTRICTIVE\n")
    }

    // FOR command
    sb.WriteString(fmt.Sprintf("    FOR %s\n", policy.Command))

    // TO roles
    if len(policy.Roles) > 0 {
        sb.WriteString(fmt.Sprintf("    TO %s\n", strings.Join(policy.Roles, ", ")))
    }

    // USING clause
    if policy.Using != nil {
        sb.WriteString(fmt.Sprintf("    USING (%s)", *policy.Using))
    }

    // WITH CHECK clause
    if policy.WithCheck != nil {
        if policy.Using != nil {
            sb.WriteString("\n")
        }
        sb.WriteString(fmt.Sprintf("    WITH CHECK (%s)", *policy.WithCheck))
    }

    return sb.String(), fmt.Sprintf("Create policy %s on %s.%s", policy.Name, schemaName, tableName)
}

// DropPolicy generates SQL to drop a policy
func (g *Generator) DropPolicy(schemaName, tableName string, policy Policy) (sql string, description string) {
    return fmt.Sprintf("DROP POLICY %s ON %s.%s",
        quoteIdentifier(policy.Name),
        quoteIdentifier(schemaName),
        quoteIdentifier(tableName)),
        fmt.Sprintf("Drop policy %s from %s.%s", policy.Name, schemaName, tableName)
}

// EnableRLS generates SQL to enable RLS
func (g *Generator) EnableRLS(schemaName, tableName string) (sql string, description string) {
    return fmt.Sprintf("ALTER TABLE %s.%s ENABLE ROW LEVEL SECURITY",
        quoteIdentifier(schemaName),
        quoteIdentifier(tableName)),
        fmt.Sprintf("Enable RLS on %s.%s", schemaName, tableName)
}
```

### Phase 6: Schema-Qualified Table Support

Update parser to handle `schema.table`:

```go
// In internal/parser/sql.go

func parseCreateTable(stmt *pg_query.CreateStmt) (*database.Table, error) {
    if stmt.Relation == nil {
        return nil, fmt.Errorf("CREATE TABLE missing relation")
    }

    table := &database.Table{
        Name:        stmt.Relation.Relname,
        Schema:      stmt.Relation.Schemaname, // NEW: capture schema
        Columns:     []database.Column{},
        Indexes:     []database.Index{},
        ForeignKeys: []database.ForeignKey{},
    }

    // ... rest of parsing
}
```

### Phase 7: Testing Plan

1. **Unit tests**: Test policy parsing, generation, introspection
2. **Integration tests**: Test with real PostgreSQL with RLS
3. **Multi-schema tests**: Ensure policies work across schemas
4. **Supabase compatibility**: Test with Supabase storage schema

## Migration Path for Users

### Current (single schema)

```sql
CREATE TABLE genomes (id bigserial PRIMARY KEY);
```

### New (multi-schema)

```sql
-- public.genomes
CREATE TABLE public.genomes (id bigserial PRIMARY KEY);

-- storage.objects policies
ALTER TABLE storage.objects ENABLE ROW LEVEL SECURITY;

CREATE POLICY "Public Access - Select" ON storage.objects
    FOR SELECT
    USING (bucket_id = 'genomes');
```

### Config Migration

```toml
# Before
[environments.local]
database_url = "postgresql://..."

# After
[environments.local]
database_url = "postgresql://..."
schemas = ["public", "storage"]  # Specify which schemas to manage
```

## Questions

1. **Default behavior**: If `schemas` not specified, default to `current_schema()`?
2. **Schema creation**: Should lockplane create schemas or assume they exist?
3. **Policy diffing**: How to detect policy changes (full text comparison)?
4. **Backwards compatibility**: Ensure single-schema workflows still work

## Next Steps

1. Start with Phase 1: Add Policy type
2. Test policy introspection manually
3. Add policy DDL generation
4. Then tackle multi-schema support
