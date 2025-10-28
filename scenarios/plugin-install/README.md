# Plugin Installation Scenario

## Overview

**TDD Test - Expected to FAIL initially**

Tests that Claude Code automatically installs the Lockplane plugin when a user provides the GitHub repository link and asks for help with Lockplane.

## What This Tests

1. **Plugin Discovery**: Can Claude Code discover the Lockplane plugin from a GitHub link?
2. **Automatic Installation**: Does Claude recognize it needs the plugin and install it?
3. **Plugin System**: Is the plugin system working correctly?
4. **Skill Availability**: Is the Lockplane skill available after installation?

## User Journey

```
User: "I want to build an app with Lockplane. Here's the repo: https://github.com/zakandrewking/lockplane"
                         ↓
Claude: Recognizes need for Lockplane expertise
                         ↓
Claude: Runs `/plugin install lockplane` or `/plugin install github:zakandrewking/lockplane`
                         ↓
Claude: Plugin installed, skill available
                         ↓
Claude: Proceeds to help with Lockplane-specific guidance
```

## Why This Will Fail (TDD)

This test is **designed to fail** until:

1. ✅ Plugin is properly configured with `marketplace.json` (DONE)
2. ❌ Plugin is discoverable from the GitHub repo
3. ❌ Claude Code knows to install it when given the GitHub link
4. ❌ Plugin installation completes successfully
5. ❌ Lockplane skill becomes available

## Running the Scenario

### Standalone

```bash
./scenario.py
```

### Via Eval Runner

```bash
cd ../..
scenarios/run-evals.py plugin-install
```

## Validation

```bash
./validate.py
```

Expected failures:
- ❌ Lockplane plugin is installed
- ❌ Lockplane skill is available
- ❌ Claude attempted to install plugin

## Success Criteria (When Test Passes)

- [ ] Claude Code CLI is available
- [ ] Plugin system is accessible
- [ ] Lockplane plugin is installed
- [ ] Lockplane skill is available
- [ ] Claude's output shows plugin installation attempt

## Making This Test Pass

To make this test pass, we need to:

1. **Complete Plugin Configuration**
   - Ensure `marketplace.json` is correct
   - Verify plugin structure matches Claude Code requirements
   - Add proper plugin discovery metadata

2. **GitHub Repository Setup**
   - Add `/.claude-plugin/` directory to repo root
   - Ensure `marketplace.json` is discoverable
   - Add README section about the Claude Code plugin

3. **Claude Code Behavior**
   - Claude should recognize Lockplane mentions
   - Claude should check if plugin is installed
   - Claude should automatically install if needed
   - This may require prompting or hints in the repo

4. **Test the Flow**
   - Run this scenario
   - Verify each validation step passes
   - Confirm skill is available and works

## Related Files

- `/marketplace.json` - Plugin marketplace configuration
- `/.claude-plugin/` - Plugin directory structure
- `.claude/skills/lockplane.md` - The actual skill file

## Next Steps

1. Run this test (it will fail) ✅
2. Fix plugin configuration ⏳
3. Re-run test
4. Iterate until all validations pass
5. Document the working setup
