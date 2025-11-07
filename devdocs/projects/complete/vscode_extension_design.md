# VSCode Extension for Lockplane (.lp.sql)

## Design Document

This document outlines the design for a VSCode extension that provides validation for Lockplane `.lp.sql` schema files.

## Goals

1. **Validate .lp.sql files** - Show errors/warnings like `lockplane validate` command
2. **Delegate syntax features** - Let existing SQL extensions (sql-tools, etc.) handle:
   - Syntax highlighting
   - Autocomplete
   - Formatting
   - Code folding
3. **Real-time feedback** - Show validation errors as users type
4. **Integration** - Work seamlessly with existing SQL tooling

## Non-Goals

- **No custom syntax highlighting** - Use existing SQL extensions
- **No autocomplete** - Unless we can enhance existing SQL tools with schema awareness
- **No formatting** - Delegate to SQL formatters
- **No custom SQL language features** - Keep it simple, focused on validation

## Features

### 1. Schema Validation

**Real-time validation as you type:**
- Parse .lp.sql files using Lockplane parser
- Show diagnostics (errors/warnings) in Problems panel
- Inline error squiggles in editor
- Validation matches `lockplane validate` command output

**Validation types:**
- SQL syntax errors (via Lockplane parser)
- Schema consistency issues
- Foreign key reference validation
- Index validation
- Data type compatibility

**Example diagnostics:**
```
Error: Column 'user_id' references non-existent table 'users'
Warning: Adding NOT NULL column without DEFAULT may fail on existing data
Info: Consider adding an index on foreign key column 'user_id'
```

### 2. File Association

**Associate .lp.sql files with SQL:**
- Set language mode to `sql` (not custom language)
- This allows existing SQL extensions to work
- VSCode treats .lp.sql as SQL for syntax highlighting

### 3. Multi-file Schema Validation

**Validate entire schema directory:**
- When editing `lockplane/schema/001_users.lp.sql`
- Validate against all files in `lockplane/schema/`
- Show cross-file issues (missing references, etc.)

### 4. Configuration

**Extension settings:**
```json
{
  "lockplane.schemaPath": "lockplane/schema/",
  "lockplane.validateOnSave": true,
  "lockplane.validateOnType": true,
  "lockplane.validationDelay": 500,
  "lockplane.cliPath": "lockplane"
}
```

## Architecture

### Option A: Language Server Protocol (LSP)

**Pros:**
- Standard approach for language extensions
- Works with any editor (VSCode, Vim, Emacs, etc.)
- Clean separation of concerns
- Can provide hover info, goto definition, etc.

**Cons:**
- More complex to implement
- Need to implement LSP server in Go

**Implementation:**
```
┌─────────────────┐
│  VSCode Client  │
│   (TypeScript)  │
└────────┬────────┘
         │ LSP
         │
┌────────▼────────┐
│   LSP Server    │
│      (Go)       │
└────────┬────────┘
         │
┌────────▼────────┐
│ Lockplane Core  │
│    (Parser,     │
│   Validator)    │
└─────────────────┘
```

### Option B: CLI Integration

**Pros:**
- Simpler to implement
- Reuses existing `lockplane validate` command
- No new server to maintain

**Cons:**
- Higher latency (spawn process for each validation)
- Less extensible (harder to add features later)

**Implementation:**
```
┌─────────────────┐
│  VSCode Ext     │
│   (TypeScript)  │
└────────┬────────┘
         │ exec()
         │
┌────────▼────────┐
│ lockplane CLI   │
│   validate      │
└─────────────────┘
```

### Recommended: Hybrid Approach

**Start with CLI integration (Phase 1):**
- Quick to implement
- Validates extension concept
- Proves value to users

**Migrate to LSP (Phase 2):**
- Better performance
- More features (hover, goto def)
- Multi-editor support

## Implementation Plan

### Phase 1: MVP with CLI Integration

**1. Extension scaffold:**
```
vscode-lockplane/
├── package.json          # Extension manifest
├── src/
│   ├── extension.ts      # Entry point
│   ├── validator.ts      # CLI wrapper
│   └── diagnostics.ts    # Error reporting
├── .vscodeignore
└── README.md
```

**2. package.json configuration:**
```json
{
  "name": "lockplane",
  "displayName": "Lockplane",
  "description": "Validation for Lockplane .lp.sql schema files",
  "version": "0.1.0",
  "engines": {
    "vscode": "^1.80.0"
  },
  "activationEvents": [
    "onLanguage:sql"
  ],
  "contributes": {
    "languages": [
      {
        "id": "sql",
        "extensions": [".lp.sql"]
      }
    ],
    "configuration": {
      "title": "Lockplane",
      "properties": {
        "lockplane.schemaPath": {
          "type": "string",
          "default": "lockplane/schema/",
          "description": "Path to schema directory"
        },
        "lockplane.validateOnSave": {
          "type": "boolean",
          "default": true
        },
        "lockplane.validateOnType": {
          "type": "boolean",
          "default": true
        }
      }
    }
  }
}
```

**3. Validation flow:**
```typescript
// src/validator.ts
import * as vscode from 'vscode';
import * as cp from 'child_process';
import * as path from 'path';

export interface ValidationResult {
  file: string;
  line: number;
  column: number;
  severity: 'error' | 'warning' | 'info';
  message: string;
}

export async function validateSchema(
  schemaPath: string
): Promise<ValidationResult[]> {
  return new Promise((resolve, reject) => {
    const lockplanePath = vscode.workspace
      .getConfiguration('lockplane')
      .get<string>('cliPath', 'lockplane');

    // Run: lockplane validate schema <path>
    cp.exec(
      `${lockplanePath} validate schema ${schemaPath} --format json`,
      { cwd: vscode.workspace.rootPath },
      (error, stdout, stderr) => {
        if (error && error.code !== 1) {
          // Exit code 1 means validation failed (expected)
          // Other codes are actual errors
          reject(new Error(stderr));
          return;
        }

        try {
          const results = JSON.parse(stdout);
          resolve(parseValidationResults(results));
        } catch (e) {
          reject(e);
        }
      }
    );
  });
}
```

**4. Diagnostic reporting:**
```typescript
// src/diagnostics.ts
import * as vscode from 'vscode';
import { ValidationResult } from './validator';

const diagnosticCollection = vscode.languages.createDiagnosticCollection('lockplane');

export function updateDiagnostics(
  document: vscode.TextDocument,
  results: ValidationResult[]
) {
  const diagnostics: vscode.Diagnostic[] = results
    .filter(r => r.file === document.fileName)
    .map(r => {
      const range = new vscode.Range(
        r.line - 1,
        r.column - 1,
        r.line - 1,
        r.column + 10
      );

      const severity =
        r.severity === 'error' ? vscode.DiagnosticSeverity.Error :
        r.severity === 'warning' ? vscode.DiagnosticSeverity.Warning :
        vscode.DiagnosticSeverity.Information;

      return new vscode.Diagnostic(range, r.message, severity);
    });

  diagnosticCollection.set(document.uri, diagnostics);
}
```

**5. Extension activation:**
```typescript
// src/extension.ts
import * as vscode from 'vscode';
import { validateSchema } from './validator';
import { updateDiagnostics } from './diagnostics';

let validationTimeout: NodeJS.Timeout | undefined;

export function activate(context: vscode.ExtensionContext) {
  // Validate on save
  context.subscriptions.push(
    vscode.workspace.onDidSaveTextDocument(async (document) => {
      if (!document.fileName.endsWith('.lp.sql')) {
        return;
      }

      const schemaPath = getSchemaPath(document);
      const results = await validateSchema(schemaPath);
      updateDiagnostics(document, results);
    })
  );

  // Validate on type (debounced)
  context.subscriptions.push(
    vscode.workspace.onDidChangeTextDocument((event) => {
      const document = event.document;
      if (!document.fileName.endsWith('.lp.sql')) {
        return;
      }

      if (validationTimeout) {
        clearTimeout(validationTimeout);
      }

      const delay = vscode.workspace
        .getConfiguration('lockplane')
        .get<number>('validationDelay', 500);

      validationTimeout = setTimeout(async () => {
        const schemaPath = getSchemaPath(document);
        const results = await validateSchema(schemaPath);
        updateDiagnostics(document, results);
      }, delay);
    })
  );
}

function getSchemaPath(document: vscode.TextDocument): string {
  const workspaceFolder = vscode.workspace.getWorkspaceFolder(document.uri);
  if (!workspaceFolder) {
    return document.fileName;
  }

  const configPath = vscode.workspace
    .getConfiguration('lockplane')
    .get<string>('schemaPath', 'lockplane/schema/');

  return path.join(workspaceFolder.uri.fsPath, configPath);
}
```

### Phase 2: LSP Implementation

**Why LSP:**
- Better performance (persistent server process)
- More features:
  - Hover tooltips (show column types, constraints)
  - Go to definition (jump to table/column references)
  - Find references (where is this table used?)
  - Code actions (quick fixes for common issues)

**Implementation:**
1. Create Go-based LSP server
2. Reuse Lockplane parser and validator
3. Implement LSP protocol handlers
4. Update VSCode extension to use LSP client

**Example features:**
```typescript
// Hover: Show table info
// When hovering over "users" in: REFERENCES users(id)
// Show: Table 'users' (3 columns: id, email, created_at)

// Go to definition
// Ctrl+Click on "users" -> Jump to CREATE TABLE users

// Find references
// Right-click on table name -> Find all REFERENCES

// Code actions
// Problem: Missing NOT NULL
// Quick fix: Add NOT NULL constraint
```

## CLI Commands Needed

### 1. Validate with JSON output

```bash
lockplane validate schema lockplane/schema/ --format json
```

**Output:**
```json
{
  "valid": false,
  "errors": [
    {
      "file": "lockplane/schema/002_posts.lp.sql",
      "line": 5,
      "column": 15,
      "severity": "error",
      "message": "Foreign key references non-existent table 'users'"
    }
  ],
  "warnings": [
    {
      "file": "lockplane/schema/002_posts.lp.sql",
      "line": 8,
      "column": 3,
      "severity": "warning",
      "message": "Adding NOT NULL column without DEFAULT may fail on existing data"
    }
  ]
}
```

### 2. Parse and get AST (for LSP features)

```bash
lockplane parse lockplane/schema/001_users.lp.sql --format json
```

**Output:**
```json
{
  "tables": [
    {
      "name": "users",
      "location": {
        "file": "001_users.lp.sql",
        "line": 1,
        "column": 1
      },
      "columns": [
        {
          "name": "id",
          "location": { "line": 2, "column": 3 }
        }
      ]
    }
  ]
}
```

## Integration with Existing SQL Tools

### Recommended Extensions

**Let users install these for full SQL experience:**
1. **SQLTools** - Syntax highlighting, autocomplete, formatting
2. **PostgreSQL** - PostgreSQL-specific features
3. **Lockplane** - Our extension for validation only

### Configuration Recommendations

**In extension README:**
```markdown
## Recommended Setup

For the best experience with `.lp.sql` files, install:

1. **Lockplane** (this extension) - Schema validation
2. **SQLTools** - Syntax highlighting and autocomplete
3. **Prettier SQL** - Code formatting

### VSCode Settings

Add to your `.vscode/settings.json`:

```json
{
  // Associate .lp.sql with SQL
  "files.associations": {
    "*.lp.sql": "sql"
  },

  // Lockplane validation
  "lockplane.schemaPath": "lockplane/schema/",
  "lockplane.validateOnSave": true,

  // SQL formatting
  "editor.formatOnSave": true,
  "[sql]": {
    "editor.defaultFormatter": "inferrinizzard.prettier-sql-vscode"
  }
}
```
```

## Future Enhancements

### Schema-Aware Autocomplete (Phase 3)

**Hook into SQL autocomplete:**
- Provide table names from schema
- Provide column names for each table
- Suggest foreign key constraints based on existing tables

**Implementation:**
- Use VSCode's `vscode.languages.registerCompletionItemProvider`
- Read schema from `lockplane.toml` or introspect current files
- Provide intelligent suggestions

**Example:**
```sql
-- User types: CREATE TABLE posts (
--   user_id INTEGER REFERENCES |
--
-- Autocomplete suggests:
--   users(id)
--   accounts(id)
```

### Workspace Symbols (Phase 3)

**Navigate schema:**
- Ctrl+T -> Search for table names
- Quickly jump to any table definition

### Schema Visualization (Phase 4)

**Webview panel:**
- Show ER diagram
- Interactive schema explorer
- Click to jump to definition

## Development Setup

### Prerequisites

```bash
# Install Node.js and npm
brew install node

# Install VSCode extension dev tools
npm install -g yo generator-code

# Install TypeScript
npm install -g typescript
```

### Create Extension

```bash
# Generate extension scaffold
yo code

# Choose:
# - New Extension (TypeScript)
# - Name: lockplane
# - Identifier: lockplane
# - Description: Validation for Lockplane schema files
```

### Development

```bash
cd vscode-lockplane

# Install dependencies
npm install

# Open in VSCode
code .

# Press F5 to launch Extension Development Host
# Test with .lp.sql files
```

### Testing

```typescript
// src/test/extension.test.ts
import * as assert from 'assert';
import * as vscode from 'vscode';
import { validateSchema } from '../validator';

suite('Extension Test Suite', () => {
  test('Validates invalid schema', async () => {
    const results = await validateSchema('test/fixtures/invalid.lp.sql');
    assert.strictEqual(results.length > 0, true);
    assert.strictEqual(results[0].severity, 'error');
  });
});
```

### Publishing

```bash
# Install vsce
npm install -g @vscode/vsce

# Package extension
vsce package

# Publish to marketplace
vsce publish
```

## Next Steps

### Immediate (Phase 1 - MVP)

1. [ ] Create extension scaffold with `yo code`
2. [ ] Implement CLI-based validation
3. [ ] Add file association for .lp.sql
4. [ ] Implement diagnostic reporting
5. [ ] Add configuration options
6. [ ] Test with real .lp.sql files
7. [ ] Write README with setup instructions
8. [ ] Package and publish to VSCode marketplace

### Short-term (Phase 2 - LSP)

1. [ ] Design LSP protocol for Lockplane
2. [ ] Implement Go LSP server
3. [ ] Add hover tooltips
4. [ ] Add go to definition
5. [ ] Add find references
6. [ ] Migrate extension to use LSP client

### Long-term (Phase 3+ - Advanced Features)

1. [ ] Schema-aware autocomplete
2. [ ] Workspace symbols
3. [ ] Code actions / quick fixes
4. [ ] Schema visualization
5. [ ] Multi-editor support (Vim, Emacs)

## Resources

- [VSCode Extension API](https://code.visualstudio.com/api)
- [Language Server Protocol](https://microsoft.github.io/language-server-protocol/)
- [go-lsp](https://github.com/sourcegraph/go-lsp) - LSP library for Go
- [SQLTools Extension](https://github.com/mtxr/vscode-sqltools) - Reference for SQL tooling

## Questions / Decisions Needed

1. **CLI output format** - Should `lockplane validate` output JSON? What format?
2. **LSP timing** - When should we build the LSP server? Phase 1 or 2?
3. **Marketplace** - Publish to VSCode marketplace or GitHub only?
4. **Branding** - Extension name, icon, colors?
5. **Features** - Which validation features are most important?

## Notes

- **Keep it simple** - Start with validation only, add features incrementally
- **Delegate to SQL tools** - Don't reinvent syntax highlighting
- **Performance** - CLI approach may be slow, plan for LSP migration
- **Cross-platform** - Test on Windows, Mac, Linux
- **Error handling** - Gracefully handle missing `lockplane` CLI
