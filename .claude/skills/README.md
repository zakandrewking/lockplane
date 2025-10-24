# Lockplane Claude Skills

This directory contains Claude skills for working with Lockplane.

## Available Skills

### lockplane.md

A comprehensive skill that teaches Claude how to help users with Lockplane database migrations.

**Topics covered**:
- Core Lockplane workflows (introspect, plan, apply)
- All CLI commands with examples
- Configuration via lockplane.toml
- SQL safety validation
- Integration with Prisma, Supabase, SQLAlchemy
- Troubleshooting common issues
- Best practices

**When to use**: Any time you're working with database schema migrations, SQL validation, or helping users set up Lockplane.

## How to Use Skills

Claude Code automatically loads skills from the `.claude/skills/` directory. When you ask Claude for help with Lockplane-related tasks, it will use this skill to provide expert guidance.

Example prompts:
- "Help me add a new column to my users table"
- "I need to create a migration for my database"
- "How do I validate this SQL file for safety?"
- "Set up Lockplane for my project"

## Creating New Skills

To add more skills:

1. Create a new `.md` file in this directory
2. Document the skill's purpose and capabilities
3. Include examples and best practices
4. Test with Claude Code

See https://github.com/anthropics/claude-cookbooks/tree/main/skills for more information.
