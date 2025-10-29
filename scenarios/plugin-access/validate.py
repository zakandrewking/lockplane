#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation for plugin access scenario.

Checks:
1. Plugin installation completed
2. Plugin files exist in isolated environment
3. Claude's response shows Lockplane expertise
4. Claude mentioned Lockplane commands/concepts
5. Response quality indicates skill usage
"""

import json
import os
import subprocess
import sys
from pathlib import Path


def check(name: str, condition: bool, error_msg: str = "") -> bool:
    """Run a validation check."""
    if condition:
        print(f"✓ {name}")
        return True
    else:
        print(f"✗ {name}", file=sys.stderr)
        if error_msg:
            for line in error_msg.split('\n'):
                print(f"  {line}", file=sys.stderr)
        return False


def main():
    """Validate the plugin access scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    if not build_dir.exists():
        print("❌ Build directory not found", file=sys.stderr)
        return 1

    isolated_claude = build_dir / "isolated_home" / ".claude"
    failures = 0

    print("=== Validating Plugin Access ===\n")

    # 1. Check plugin installation output exists
    install_output_file = build_dir / "install_output.txt"
    if not check("Plugin installation attempted", install_output_file.exists()):
        failures += 1

    # 2. Check for plugins directory
    isolated_plugins = isolated_claude / "plugins"
    if not check("Plugins directory created", isolated_plugins.exists()):
        failures += 1
        print("  Plugin may not have been installed", file=sys.stderr)

    # 3. Check for installed plugin files
    if isolated_plugins.exists():
        # Look for lockplane plugin files
        lockplane_files = list(isolated_plugins.glob("**/lockplane/**"))
        has_lockplane_files = len(lockplane_files) > 0

        if not check("Lockplane plugin files found", has_lockplane_files):
            failures += 1
            print(f"  Found {len(list(isolated_plugins.glob('**/*')))} total files in plugins/", file=sys.stderr)

        # Check for skill file specifically
        skill_files = list(isolated_plugins.glob("**/SKILL.md"))
        has_skill = len(skill_files) > 0

        if not check("Skill file found", has_skill):
            failures += 1
        else:
            print(f"  Found {len(skill_files)} skill file(s)")

    # 4. Check Claude's response
    claude_output_file = build_dir / "claude_output.txt"
    if not check("Claude output exists", claude_output_file.exists()):
        failures += 1
        return 1

    claude_output = claude_output_file.read_text()
    output_lower = claude_output.lower()

    # 5. Check for Lockplane-specific content
    mentioned_lockplane = "lockplane" in output_lower
    if not check("Response mentions Lockplane", mentioned_lockplane):
        failures += 1

    # 6. Check for Lockplane commands
    lockplane_commands = [
        "lockplane plan",
        "lockplane apply",
        "lockplane validate",
        "lockplane introspect",
        ".lp.sql",
        "schema file",
        "migration plan",
        "shadow db",
        "shadow database",
    ]

    found_commands = [cmd for cmd in lockplane_commands if cmd.lower() in output_lower]
    has_commands = len(found_commands) > 0

    cmd_msg = []
    if has_commands:
        cmd_msg.append(f"Found {len(found_commands)} Lockplane-specific terms:")
        for cmd in found_commands[:3]:
            cmd_msg.append(f"  - {cmd}")
    else:
        cmd_msg.append("Expected Lockplane commands like:")
        cmd_msg.append("  - 'lockplane plan'")
        cmd_msg.append("  - 'shadow database'")
        cmd_msg.append("  - '.lp.sql'")

    if not check("Response includes Lockplane commands/concepts", has_commands, '\n'.join(cmd_msg)):
        failures += 1

    # 7. Check for safety guidance (indicates skill knowledge)
    safety_terms = [
        "not null",
        "default",
        "nullable",
        "safe",
        "migration",
        "validate",
        "shadow",
    ]

    found_safety = [term for term in safety_terms if term.lower() in output_lower]
    has_safety_guidance = len(found_safety) >= 2  # At least 2 safety-related terms

    if not check("Response includes safety guidance", has_safety_guidance):
        failures += 1
        print(f"  Found safety terms: {found_safety if found_safety else 'none'}", file=sys.stderr)

    # 8. Check response quality (length indicates detailed response)
    response_length = len(claude_output)
    is_detailed = response_length > 200  # At least 200 characters

    if not check("Response is detailed", is_detailed):
        failures += 1
        print(f"  Response length: {response_length} characters", file=sys.stderr)

    print()
    print("=" * 60)
    print("Test Results")
    print("=" * 60)

    if failures == 0:
        print("✅ PASS - Plugin is working!")
        print()
        print("The Lockplane plugin:")
        print("  ✓ Was successfully installed")
        print("  ✓ Is accessible to Claude")
        print("  ✓ Provides Lockplane expertise")
        print("  ✓ Improves response quality")
        return 0
    else:
        print(f"❌ FAIL - {failures} check(s) failed")
        print()
        print("Possible issues:")
        print("  - Plugin installation may have failed")
        print("  - Plugin files not in correct location")
        print("  - Skill not being triggered")
        print("  - Claude not recognizing the skill")
        print()
        print("Debug:")
        print(f"  Check: {build_dir}/install_output.txt")
        print(f"  Check: {build_dir}/claude_output.txt")
        print(f"  Check: {isolated_plugins}")
        return 1


if __name__ == "__main__":
    sys.exit(main())
