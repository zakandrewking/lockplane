# Interactive Init Wizard Enhancement

## Progress Checklist

### Phase 1: Design and Planning
- [x] Document current init behavior
- [x] Define wizard flow and UX
- [x] Specify config file structure
- [ ] Review and approve design

### Phase 2: Core Implementation
- [ ] Implement multi-step wizard model
- [ ] Add environment collection flow
- [ ] Add database type detection/selection
- [ ] Add connection string validation
- [ ] Generate lockplane.toml with custom environments
- [ ] Generate corresponding .env files

### Phase 3: Enhanced Features
- [ ] Add "detect from existing database" flow
- [ ] Add connection string testing
- [ ] Add schema path customization
- [ ] Add templates for common setups (Supabase, local Postgres, SQLite)

### Phase 4: Testing
- [ ] Unit tests for wizard state machine
- [ ] Integration tests for config generation
- [ ] Manual testing of interactive flow
- [ ] Test edge cases (existing configs, invalid inputs)

### Phase 5: Documentation
- [ ] Update README.md with new init flow
- [ ] Update docs/getting_started.md
- [ ] Add examples for common scenarios
- [ ] Update CLI help text

## Context

Currently, `lockplane init` creates a basic setup with hardcoded PostgreSQL defaults:
- Creates `schema/` directory
- Generates `schema/lockplane.toml` with a single "local" environment
- Uses hardcoded connection strings for PostgreSQL

**Limitations:**
- No interactive prompts for custom environments
- No support for SQLite or other databases during init
- No guidance for users unfamiliar with connection strings
- No validation of connection strings
- Users must manually edit TOML after creation

**User Pain Points:**
- "How do I set up for SQLite?"
- "What connection string should I use for Supabase?"
- "Can I define multiple environments during init?"
- "How do I connect to my existing database?"

## Goals

1. **Interactive Environment Setup**: Guide users through defining one or more environments
2. **Database Type Selection**: Support PostgreSQL, SQLite, and libSQL/Turso out of the box
3. **Connection String Help**: Provide templates and validation for connection strings
4. **Config File Placement**: Create `lockplane.toml` in the current directory (not inside `schema/`)
5. **Secure Credential Handling**: Generate `.env.{name}` files for sensitive data
6. **Existing Config Detection**: Don't overwrite existing configs; offer to add environments
7. **Quick Start Options**: Provide templates for common scenarios (local dev, Supabase, SQLite)

## Design

### Wizard Flow

```
┌─────────────────────────────────────────────────────────────┐
│ Lockplane Init Wizard                                       │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│ Step 1: Check for existing config                          │
│   ├─ If lockplane.toml exists:                             │
│   │   └─ "Config exists. Add new environment? (y/N)"       │
│   │       ├─ Yes: Continue to Step 3                       │
│   │       └─ No: Exit                                       │
│   └─ If not: Continue to Step 2                            │
│                                                             │
│ Step 2: Quick Start or Custom?                             │
│   Choose an option:                                         │
│   • Local PostgreSQL (default ports)                       │
│   • Local SQLite (file-based)                              │
│   • Supabase                                                │
│   • Custom setup (manual)                                   │
│                                                             │
│ Step 3: Environment Details                                │
│   Environment name: [local]                                 │
│   Description: [Local development database]                │
│   Database type:                                            │
│     • PostgreSQL                                            │
│     • SQLite                                                │
│     • libSQL/Turso                                          │
│                                                             │
│ Step 4: Connection Details (varies by type)                │
│                                                             │
│   For PostgreSQL:                                           │
│     Host: [localhost]                                       │
│     Port: [5432]                                            │
│     Database: [lockplane]                                   │
│     User: [lockplane]                                       │
│     Password: [****]                                        │
│     SSL Mode: [disable/require/prefer]                      │
│     Shadow DB Port: [5433]                                  │
│                                                             │
│   For SQLite:                                               │
│     Database file: [lockplane.db]                           │
│     Use shadow DB? (y/N)                                    │
│       └─ If yes: Shadow file: [lockplane_shadow.db]        │
│                                                             │
│   For libSQL/Turso:                                         │
│     URL: [libsql://...turso.io]                             │
│     Auth token: [****]                                      │
│                                                             │
│ Step 5: Schema Path                                         │
│   Where should schema files live?                           │
│     • schema/ (recommended)                                 │
│     • . (current directory)                                 │
│     • custom path                                           │
│                                                             │
│ Step 6: Test Connection (optional)                          │
│   Test connection to database? (Y/n)                        │
│   [Testing...] ✓ Connection successful                      │
│                                                             │
│ Step 7: Add Another Environment?                            │
│   Add another environment? (y/N)                            │
│   ├─ Yes: Loop back to Step 3                              │
│   └─ No: Continue to Step 8                                │
│                                                             │
│ Step 8: Summary & Confirmation                              │
│   Will create:                                              │
│     ✓ lockplane.toml                                        │
│     ✓ .env.local                                            │
│     ✓ schema/ (if doesn't exist)                            │
│                                                             │
│   Environments:                                             │
│     • local (PostgreSQL, localhost:5432)                    │
│                                                             │
│   Proceed? (Y/n)                                            │
│                                                             │
│ Step 9: Create Files                                        │
│   [Creating...] ✓ Done!                                     │
│                                                             │
│   Next steps:                                               │
│     1. Review lockplane.toml                                │
│     2. Add .env.* to .gitignore                             │
│     3. Run: lockplane introspect                            │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

### File Structure

#### Generated `lockplane.toml` (in current directory)

```toml
# Lockplane Configuration
# Generated by: lockplane init

default_environment = "local"

[environments.local]
description = "Local development database"
schema_path = "schema"
# Connection details are in .env.local

[environments.staging]
description = "Staging database"
schema_path = "schema"
# Connection details are in .env.staging
```

#### Generated `.env.local`

```bash
# Lockplane Environment: local
# Generated by: lockplane init
# DO NOT COMMIT THIS FILE - Add .env.* to .gitignore

DATABASE_URL=postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable
SHADOW_DATABASE_URL=postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable
```

#### Generated `.gitignore` additions (if .gitignore exists)

```
# Lockplane environment files (added by lockplane init)
.env.*
!.env.*.example
```

### Config File Location Strategy

**Current behavior**: `schema/lockplane.toml`
**New behavior**: `lockplane.toml` in project root (current directory)

**Rationale:**
- Config files typically live at project root (`.eslintrc`, `tsconfig.json`, `Cargo.toml`)
- Easier to find and edit
- Separates config from schema files
- Allows schema files to live anywhere (not just `schema/`)

**Migration path:**
- Check both locations: `./lockplane.toml` first, then `./schema/lockplane.toml`
- If only old location exists, continue using it (backward compatible)
- If both exist, prefer `./lockplane.toml` and warn about duplicate
- `lockplane init` always creates in project root

### Quick Start Templates

#### Local PostgreSQL (Docker Compose)

```toml
default_environment = "local"

[environments.local]
description = "Local PostgreSQL via Docker Compose"
schema_path = "schema"
```

```bash
# .env.local
DATABASE_URL=postgresql://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable
SHADOW_DATABASE_URL=postgresql://lockplane:lockplane@localhost:5433/lockplane_shadow?sslmode=disable
```

#### SQLite File-Based

```toml
default_environment = "local"

[environments.local]
description = "Local SQLite database"
schema_path = "schema"
```

```bash
# .env.local
DATABASE_URL=sqlite://./lockplane.db
SHADOW_DATABASE_URL=sqlite://./lockplane_shadow.db
```

#### Supabase

```toml
default_environment = "local"

[environments.production]
description = "Supabase production database"
schema_path = "schema"
```

```bash
# .env.production
DATABASE_URL=postgresql://postgres.[PROJECT-REF]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres
SHADOW_DATABASE_URL=postgresql://postgres.[PROJECT-REF]:[PASSWORD]@aws-0-[REGION].pooler.supabase.com:6543/postgres_shadow
```

#### Turso/libSQL

```toml
default_environment = "production"

[environments.production]
description = "Turso edge database"
schema_path = "schema"
```

```bash
# .env.production
DATABASE_URL=libsql://[db-name]-[org].turso.io?authToken=[token]
# Note: Turso doesn't support shadow databases yet
```

### Wizard State Machine

```go
type WizardState int

const (
    StateWelcome WizardState = iota
    StateCheckExisting
    StateQuickStartOrCustom
    StateEnvironmentName
    StateEnvironmentDescription
    StateDatabaseType
    StateConnectionDetails
    StateSchemaPath
    StateTestConnection
    StateAddAnother
    StateSummary
    StateCreating
    StateDone
)

type WizardModel struct {
    state           WizardState

    // Existing config detection
    existingConfig  *Config
    addingToExisting bool

    // Quick start selection
    quickStartChoice string // "postgres", "sqlite", "supabase", "custom"

    // Environment being configured
    currentEnv      EnvironmentInput
    environments    []EnvironmentInput

    // Connection testing
    testingConnection bool
    connectionTestResult string

    // Input fields (using bubbletea textinput)
    inputs          []textinput.Model
    focusIndex      int

    // Validation
    errors          map[string]string

    // Final output
    result          *InitResult
    err             error
}

type EnvironmentInput struct {
    Name              string
    Description       string
    DatabaseType      string // "postgres", "sqlite", "libsql"

    // PostgreSQL fields
    Host              string
    Port              string
    Database          string
    User              string
    Password          string
    SSLMode           string
    ShadowDBPort      string

    // SQLite fields
    FilePath          string
    ShadowFilePath    string

    // libSQL fields
    URL               string
    AuthToken         string

    // Common
    SchemaPath        string
}

type InitResult struct {
    ConfigPath        string
    ConfigCreated     bool
    ConfigUpdated     bool
    EnvFiles          []string
    SchemaDir         string
    SchemaDirCreated  bool
    GitignoreUpdated  bool
}
```

### Connection String Validation

```go
// ValidateConnectionString checks if a connection string is well-formed
func ValidateConnectionString(connStr string, dbType string) error {
    switch dbType {
    case "postgres":
        // Check for postgresql:// or postgres://
        // Validate basic structure
        if !strings.HasPrefix(connStr, "postgres://") &&
           !strings.HasPrefix(connStr, "postgresql://") {
            return fmt.Errorf("PostgreSQL connection string must start with postgres:// or postgresql://")
        }
        // Optional: Parse and validate host, port, database

    case "sqlite":
        // Check for sqlite:// or file path
        if !strings.HasPrefix(connStr, "sqlite://") &&
           !strings.HasPrefix(connStr, "./") &&
           !strings.HasPrefix(connStr, "/") {
            return fmt.Errorf("SQLite connection string must be sqlite:// or a file path")
        }

    case "libsql":
        // Check for libsql://
        if !strings.HasPrefix(connStr, "libsql://") {
            return fmt.Errorf("libSQL connection string must start with libsql://")
        }
    }
    return nil
}

// TestConnection attempts to connect to the database
func TestConnection(connStr string, dbType string) error {
    driver, err := newDriver(dbType)
    if err != nil {
        return err
    }

    db, err := sql.Open(getSQLDriverName(dbType), connStr)
    if err != nil {
        return fmt.Errorf("failed to open connection: %w", err)
    }
    defer db.Close()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if err := db.PingContext(ctx); err != nil {
        return fmt.Errorf("failed to ping database: %w", err)
    }

    return nil
}
```

### UX Considerations

1. **Progressive Disclosure**: Show only relevant fields based on database type selection
2. **Sensible Defaults**: Pre-fill common values (localhost, 5432, etc.)
3. **Validation Feedback**: Show errors inline as user types
4. **Skip Options**: Allow users to skip optional steps (connection testing)
5. **Back Navigation**: Let users go back to correct mistakes
6. **Clear Next Steps**: After completion, show what to do next

### Error Handling

1. **Existing Config**: If `lockplane.toml` exists, offer to add environment or exit
2. **Invalid Input**: Show validation errors inline, don't proceed until fixed
3. **Connection Failure**: Allow retry or skip (with warning)
4. **File Write Errors**: Show clear error message, suggest fixes (permissions, disk space)
5. **Partial Completion**: If wizard is interrupted, don't leave partial files

### Security Considerations

1. **Password Masking**: Use masked input for passwords and tokens
2. **File Permissions**: Create `.env.*` files with 0600 permissions
3. **Gitignore Reminder**: Prompt user to add `.env.*` to `.gitignore`
4. **Example Files**: Optionally create `.env.*.example` with placeholder values
5. **No Defaults in Config**: Never write credentials to `lockplane.toml`, always use `.env.*`

## Implementation Plan

### Phase 1: Core Wizard Structure
- Implement multi-step wizard state machine
- Add navigation (next, back, quit)
- Add input validation framework

### Phase 2: Environment Collection
- Implement environment name/description inputs
- Add database type selection menu
- Build connection details forms for each DB type

### Phase 3: File Generation
- Implement TOML generation with multiple environments
- Implement `.env.*` file generation
- Add `.gitignore` update logic

### Phase 4: Enhanced Features
- Add connection testing
- Add quick start templates
- Add existing config detection and merging

### Phase 5: Polish
- Add help text and examples
- Improve error messages
- Add progress indicators

## Testing Strategy

### Unit Tests
- Wizard state transitions
- Connection string validation
- TOML generation with various inputs
- `.env` file generation

### Integration Tests
- Full wizard flow (automated with pre-filled inputs)
- Config file creation and loading
- Multiple environment scenarios

### Manual Testing
- Complete wizard with each database type
- Test with existing configs
- Test error cases (bad permissions, invalid input)
- Test on different platforms (Linux, macOS, Windows)

## Migration from Current Behavior

### Backward Compatibility
- Continue supporting `schema/lockplane.toml` location
- `--yes` flag still works (uses defaults, creates in new location)
- Existing `lockplane.toml` files continue to work without changes

### Upgrade Path
- No automatic migration (don't move existing files)
- Document that new `init` creates in project root
- Users can manually move `schema/lockplane.toml` to `./lockplane.toml` if desired

## Alternative Approaches Considered

### 1. Non-interactive flags approach
```bash
lockplane init --env local --type postgres --host localhost --port 5432
```
**Rejected**: Too many flags, poor UX for beginners

### 2. Config templates approach
```bash
lockplane init --template supabase
```
**Considered**: Good for quick start, but doesn't allow customization without editing files

**Decision**: Combine both - quick start templates in wizard + full customization

### 3. Keep everything in lockplane.toml (no .env files)
**Rejected**: Exposes credentials in config files, bad security practice

## Open Questions

1. **Should we support importing from other tools?**
   - Prisma's `schema.prisma`
   - Alembic's `alembic.ini`
   - Django's `settings.py`

2. **Should we create example migration files?**
   - `schema/001_initial.lp.sql` template

3. **Should we offer to run introspection immediately?**
   - After config creation, prompt: "Introspect database now? (Y/n)"

4. **Should we create docker-compose.yml for local dev?**
   - Offer to generate a local PostgreSQL setup

## Success Metrics

- Reduced time to first successful `lockplane introspect` after `init`
- Fewer GitHub issues about "how do I configure for X database?"
- Increased usage of multiple environments
- Better credential security (more users with `.env.*` in `.gitignore`)

## Future Enhancements

1. **Environment cloning**: Copy and modify existing environment
2. **Import from connection string**: Parse and populate fields
3. **Database detection**: Auto-detect running databases (localhost scanning)
4. **Cloud provider integration**: OAuth flows for Supabase, PlanetScale, etc.
5. **Configuration validation**: `lockplane config check` command
6. **Environment switching**: `lockplane use <env>` to change default
