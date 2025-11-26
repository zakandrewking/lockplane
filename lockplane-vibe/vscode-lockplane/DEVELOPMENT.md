# Development Guide

## Setup

### Prerequisites

```bash
# Install Node.js (v18+)
brew install node

# Install TypeScript globally
npm install -g typescript

# Install dependencies
npm install
```

### Build

```bash
# Compile TypeScript
npm run compile

# Watch mode (recompile on changes)
npm run watch
```

## Testing the Extension

### Option 1: Run in VSCode Extension Development Host

1. Open this directory in VSCode:
   ```bash
   code vscode-lockplane
   ```

2. Press `F5` to launch Extension Development Host
   - This opens a new VSCode window with the extension loaded
   - The extension is automatically activated when you open a .lp.sql file

3. In the Extension Development Host window:
   - Create or open a `.lp.sql` file
   - Make changes and watch for validation errors
   - Check the Problems panel (View → Problems)

### Option 2: Install Locally

```bash
# Package the extension
npm install -g @vscode/vsce
vsce package

# This creates lockplane-0.1.0.vsix

# Install in VSCode
code --install-extension lockplane-0.1.0.vsix
```

## Testing Validation

### Create Test Files

```bash
# Create schema directory
mkdir -p test-workspace/lockplane/schema

# Create a valid schema
cat > test-workspace/lockplane/schema/001_users.lp.sql <<EOF
CREATE TABLE users (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  email TEXT NOT NULL UNIQUE,
  created_at TIMESTAMP DEFAULT NOW()
);
EOF

# Create an invalid schema (missing table reference)
cat > test-workspace/lockplane/schema/002_posts.lp.sql <<EOF
CREATE TABLE posts (
  id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES missing_table(id),
  title TEXT NOT NULL
);
EOF
```

### Open Test Workspace

```bash
code test-workspace
```

### Expected Behavior

1. **Valid file** (`001_users.lp.sql`):
   - No errors shown
   - No squiggly underlines

2. **Invalid file** (`002_posts.lp.sql`):
   - Error shown in Problems panel
   - Red squiggly underline on the invalid reference
   - Hover over error for details

## Debugging

### View Extension Logs

1. Open Output panel (View → Output)
2. Select "Lockplane" from dropdown
3. See validation command output and errors

### Debug TypeScript

1. Set breakpoints in `.ts` files
2. Press `F5` to launch Extension Development Host
3. Debugger will stop at breakpoints

### Common Issues

**"Lockplane CLI not found"**
- Make sure lockplane is installed and in PATH
- Test: `lockplane --version`
- Configure custom path in settings: `lockplane.cliPath`

**Extension not activating**
- Check that file has `.lp.sql` extension
- Check Output → Extension Host for errors

**Validation not working**
- Check `lockplane.enabled` setting is `true`
- Check `lockplane.schemaPath` points to correct directory
- View Output → Lockplane for validation command and results

## Project Structure

```
vscode-lockplane/
├── src/
│   ├── extension.ts      # Main entry point
│   ├── validator.ts      # CLI wrapper
│   └── diagnostics.ts    # Error reporting
├── out/                  # Compiled JavaScript (generated)
├── node_modules/         # Dependencies (generated)
├── package.json          # Extension manifest
├── tsconfig.json         # TypeScript config
├── README.md             # User documentation
└── DEVELOPMENT.md        # This file
```

## Making Changes

### Add a New Feature

1. Modify TypeScript files in `src/`
2. Compile: `npm run compile`
3. Test in Extension Development Host (`F5`)
4. Update README.md with user-facing changes
5. Update CHANGELOG.md

### Update Dependencies

```bash
npm update
npm audit fix
```

## Publishing

### To VSCode Marketplace

```bash
# Login to marketplace
vsce login lockplane

# Publish (increments version automatically)
vsce publish

# Or publish specific version
vsce publish 0.2.0
```

### To GitHub Releases

```bash
# Package
vsce package

# Upload lockplane-0.1.0.vsix to GitHub release
```

## Roadmap

See [docs/vscode_extension_design.md](../docs/vscode_extension_design.md) for the full roadmap.

### Phase 2: Language Server Protocol

Next major improvement will be migrating to LSP for:
- Better performance (persistent server process)
- Hover tooltips
- Go to definition
- Find references
- Multi-editor support (Vim, Emacs, etc.)

## Resources

- [VSCode Extension API](https://code.visualstudio.com/api)
- [Extension Guidelines](https://code.visualstudio.com/api/references/extension-guidelines)
- [Publishing Extensions](https://code.visualstudio.com/api/working-with-extensions/publishing-extension)
