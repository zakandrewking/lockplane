# Lockplane Claude Plugin

A Claude Code plugin that provides expert knowledge for Lockplane, a Postgres-first control plane for safe, AI-friendly schema management.

## What's Included

This plugin provides:

- **Lockplane Skill**: Expert knowledge about Lockplane commands, workflows, and best practices
- **Automatic Invocation**: Claude will automatically use this skill when you discuss database migrations, schema management, or SQL validation

## What is Lockplane?

Lockplane is a declarative schema management tool that:

- Tests migrations on a shadow database before applying to production
- Generates rollback plans automatically
- Validates SQL for dangerous patterns (data loss, blocking operations)
- Supports both SQL DDL (`.lp.sql`) and JSON schema formats
- Works with PostgreSQL and SQLite

## Installation

### Option 1: Install from GitHub (Recommended)

```bash
# Add the marketplace
/plugin marketplace add zakandrewking/lockplane

# Install the plugin
/plugin install lockplane@lockplane-tools
```

### Option 2: Install from Local Directory

```bash
# From your Lockplane repository directory
/plugin install ./claude-plugin
```

### Option 3: Team-wide Configuration

Add to your `.claude/settings.json`:

```json
{
  "extraKnownMarketplaces": {
    "lockplane-tools": {
      "source": {
        "source": "github",
        "repo": "zakandrewking/lockplane",
        "path": "claude-plugin"
      }
    }
  }
}
```

## Usage

Once installed, the skill activates automatically when you:

- Ask about database schema migrations
- Need to create or modify database tables
- Want to generate migration plans
- Need to validate SQL safety
- Set up schema version control
- Plan database rollbacks

### Example Interactions

**"I need to add an email column to my users table"**

Claude will guide you through:
1. Updating your schema file
2. Validating the schema
3. Generating a safe migration plan
4. Applying with shadow DB testing

**"How do I set up Lockplane for my project?"**

Claude will provide the complete setup workflow with Docker Compose, configuration, and initial schema creation.

**"My validation is showing errors about DROP TABLE"**

Claude will explain the safety concern and guide you through proper handling of destructive operations.

## What the Skill Knows

The Lockplane skill provides expertise on:

- All Lockplane commands (`introspect`, `plan`, `apply`, `validate`, `convert`, `rollback`, `diff`)
- Configuration via `lockplane.toml` and environment variables
- Schema file formats (`.lp.sql` and JSON)
- Safety validations and dangerous pattern detection
- Shadow database testing workflows
- Integration with Prisma, Supabase, SQLAlchemy, and other tools
- Troubleshooting common issues
- Best practices for production deployments

## Plugin Structure

```
claude-plugin/
├── .claude-plugin/
│   ├── plugin.json           # Plugin metadata
│   └── marketplace.json      # Marketplace configuration
├── skills/
│   └── lockplane/
│       └── SKILL.md          # Lockplane expertise skill
└── README.md                 # This file
```

## Verifying Installation

After installation, you can verify the skill is available:

1. Ask Claude: "What skills do you have access to?"
2. The response should mention the "lockplane" skill
3. Try asking: "How do I use Lockplane to add a column to my table?"

## Development

To modify the skill:

1. Edit `skills/lockplane/SKILL.md`
2. Test locally: `/plugin install ./claude-plugin`
3. Commit and push changes
4. Users will get updates on next `/plugin update`

## License

MIT - See the main Lockplane repository for details.

## Support

For issues or questions:
- Lockplane documentation: Check the main repository
- Plugin issues: Open an issue in the Lockplane repository
- General help: Ask Claude!

## Version

Current version: 1.0.0

See the main Lockplane repository for changelog and release notes.
