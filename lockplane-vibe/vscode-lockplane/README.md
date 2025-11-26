# Lockplane VSCode Extension

Schema validation for Lockplane `.lp.sql` files in Visual Studio Code.

## Features

- **Real-time validation** - Validates schema files as you type and on save
- **Error highlighting** - Shows validation errors inline with squiggly underlines
- **Problems panel integration** - View all schema issues in VSCode's Problems panel
- **Multi-file validation** - Validates entire schema directory for cross-file references
- **Works with SQL tools** - Delegates syntax highlighting and formatting to existing SQL extensions

![Lockplane validation in action](https://raw.githubusercontent.com/zakandrewking/lockplane/main/docs/images/vscode-validation.png)

## Requirements

- **Lockplane CLI** must be installed and available in your PATH
- Install from: https://github.com/zakandrewking/lockplane

To verify installation:
```bash
lockplane --version
```

## Recommended Extensions

For the best experience with `.lp.sql` files, also install:

1. **SQLTools** - Syntax highlighting and autocomplete
2. **Prettier SQL** - Code formatting

## Usage

### 1. Create lockplane.toml (optional)

```toml
# lockplane.toml
database_url = "postgresql://localhost:5432/myapp?sslmode=disable"
shadow_database_url = "postgresql://localhost:5433/myapp_shadow?sslmode=disable"
schema_path = "lockplane/schema/"
```

### 2. Create .lp.sql files

```sql
-- lockplane/schema/001_users.lp.sql
CREATE TABLE users (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT NOW()
);
```

### 3. Get real-time validation

The extension automatically validates your schema files and shows:
- ✅ Valid schema (no errors)
- ❌ Syntax errors
- ⚠️ Validation warnings
- 💡 Suggestions and best practices

## Extension Settings

This extension contributes the following settings:

* `lockplane.enabled`: Enable/disable Lockplane validation (default: `true`)
* `lockplane.schemaPath`: Path to schema directory or file (default: `"lockplane/schema/"`)
* `lockplane.validateOnSave`: Validate on save (default: `true`)
* `lockplane.validateOnType`: Validate as you type (default: `true`)
* `lockplane.validationDelay`: Delay in ms before validating on type (default: `500`)
* `lockplane.cliPath`: Path to lockplane CLI (default: `"lockplane"`)

### Example .vscode/settings.json

```json
{
  "lockplane.schemaPath": "lockplane/schema/",
  "lockplane.validateOnSave": true,
  "lockplane.validateOnType": true,
  "lockplane.validationDelay": 500,

  // Associate .lp.sql with SQL for syntax highlighting
  "files.associations": {
    "*.lp.sql": "sql"
  },

  // Format on save (optional)
  "editor.formatOnSave": true,
  "[sql]": {
    "editor.defaultFormatter": "inferrinizzard.prettier-sql-vscode"
  }
}
```

## Validation

The extension validates:

- **SQL syntax** - Detects parse errors
- **Schema consistency** - Checks for missing tables, columns, etc.
- **Foreign keys** - Validates references exist
- **Data types** - Ensures type compatibility
- **Constraints** - Validates NOT NULL, UNIQUE, CHECK constraints
- **Indexes** - Checks index definitions
- **Migration safety** - Warns about potentially dangerous operations

## Troubleshooting

### "Lockplane CLI not found"

Make sure lockplane is installed and in your PATH:

```bash
# Install lockplane
go install github.com/zakandrewking/lockplane@latest

# Or download from releases
# https://github.com/zakandrewking/lockplane/releases

# Verify installation
lockplane --version
```

If lockplane is installed in a custom location, configure it:

```json
{
  "lockplane.cliPath": "/path/to/lockplane"
}
```

### Validation not working

1. Check that `.lp.sql` files are in the configured schema path
2. Verify `lockplane.enabled` is `true` in settings
3. Check the Output panel (View → Output → Lockplane) for errors

### Performance issues

If validation is slow:

1. Increase `lockplane.validationDelay` to reduce validation frequency
2. Disable `lockplane.validateOnType` to only validate on save
3. Reduce schema size or split into smaller files

## Known Limitations

- **Phase 1 (Current)**: Uses CLI-based validation (may have slight delay)
- **Future (LSP)**: Will use Language Server Protocol for better performance

## Roadmap

- [x] Phase 1: CLI-based validation
- [ ] Phase 2: Language Server Protocol (LSP) implementation
- [ ] Phase 3: Hover tooltips (show table/column info)
- [ ] Phase 3: Go to definition (jump to table definitions)
- [ ] Phase 3: Find references (find table usage)
- [ ] Phase 4: Schema-aware autocomplete
- [ ] Phase 4: Schema visualization

## Contributing

Issues and pull requests welcome at:
https://github.com/zakandrewking/lockplane

## License

Apache License 2.0 - see LICENSE file for details

## More Information

- [Lockplane Documentation](https://github.com/zakandrewking/lockplane)
- [Getting Started Guide](https://github.com/zakandrewking/lockplane/blob/main/docs/getting_started.md)
- [VSCode Extension Design](https://github.com/zakandrewking/lockplane/blob/main/docs/vscode_extension_design.md)

---

**Enjoy using Lockplane!**
