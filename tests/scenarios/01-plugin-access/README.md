# Plugin Access Scenario

## Overview

Tests that Claude Code can access and use an installed Lockplane plugin. This verifies the complete plugin workflow: install → access → use skills.

## What This Tests

1. **Plugin Installation**: Explicitly install Lockplane plugin in isolated environment
2. **Plugin Files**: Verify plugin files are in correct location
3. **Skill Access**: Claude can access the Lockplane skill
4. **Skill Usage**: Claude provides Lockplane-specific guidance
5. **Response Quality**: Skill improves response with expert knowledge

## How It Works

```
1. Create isolated Claude environment
        ↓
2. Install Lockplane plugin from local directory
   Command: /plugin install ./claude-plugin
        ↓
3. Ask Claude a Lockplane question
   "How do I safely add a column to a table with data?"
        ↓
4. Validate response shows Lockplane expertise
```

## Running the Scenario

```bash
scenarios/run-evals.py plugin-access
```

## Validation Checks

- ✅ Plugin installation attempted
- ✅ Plugins directory created
- ✅ Lockplane plugin files found
- ✅ Skill file (SKILL.md) found
- ✅ Response mentions Lockplane
- ✅ Response includes Lockplane commands
- ✅ Response includes safety guidance
- ✅ Response is detailed (>200 chars)

## Success Criteria

Claude's response should:
- Mention Lockplane specifically
- Reference Lockplane commands (`lockplane plan`, etc.)
- Discuss safety concepts (nullable, defaults, validation)
- Mention `.lp.sql` schema files
- Talk about shadow database testing
- Provide specific, accurate guidance

## Example Expected Response

```
To safely add an email column to your users table with Lockplane:

1. Update your schema file (schema.lp.sql) to include the new column:
   - Make it NULLABLE initially if the table has data
   - Or provide a DEFAULT value

2. Generate and validate the migration plan:
   lockplane plan --from current.json --to schema.lp.sql --validate

3. Apply with shadow database validation:
   lockplane apply --plan migration.json

The shadow database will test the migration before applying to production...
```

## Prerequisites

- Claude Code CLI installed
- Lockplane repository (for plugin files)
- Isolated test environment

## Debugging

If the test fails, check:
- `build/install_output.txt` - Plugin installation log
- `build/claude_output.txt` - Claude's response
- `build/isolated_home/.claude/plugins/` - Installed plugin files

## Related Scenarios

- `plugin-install` - Tests if Claude suggests installing the plugin
- `basic-migration` - Tests Lockplane CLI without plugins
