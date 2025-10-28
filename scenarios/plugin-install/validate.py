#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation for plugin installation scenario.

This should FAIL until the Lockplane plugin is properly configured.

Checks:
1. Isolated environment was created
2. Claude Code ran successfully
3. Plugin was installed in isolated environment
4. Lockplane skill is available in isolated environment
5. Claude's output shows plugin installation
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
            print(f"  {error_msg}", file=sys.stderr)
        return False


def main():
    """Validate the plugin installation scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    if not build_dir.exists():
        print("❌ Build directory not found", file=sys.stderr)
        return 1

    isolated_claude = build_dir / "isolated_home" / ".claude"

    failures = 0

    print("=== Validating Plugin Installation (TDD - Expected to FAIL) ===\n")

    # 1. Check isolated environment was created
    if not check("Isolated Claude directory created", isolated_claude.exists()):
        failures += 1
        print(f"  Expected: {isolated_claude}", file=sys.stderr)

    # 2. Check Claude Code ran
    claude_output_file = build_dir / "claude_output.txt"
    if not check("Claude Code output exists", claude_output_file.exists()):
        failures += 1
    else:
        # Read Claude's output
        claude_output = claude_output_file.read_text()

        # 3. Check if Claude mentioned plugins or installation
        mentioned_plugin = any([
            "plugin" in claude_output.lower(),
            "install" in claude_output.lower() and "lockplane" in claude_output.lower(),
            "/plugin" in claude_output,
        ])

        if not check("Claude mentioned plugin/installation", mentioned_plugin):
            failures += 1
            print("  Claude should recognize the need for a plugin", file=sys.stderr)
            print(f"  Output preview: {claude_output[:200]}...", file=sys.stderr)

    # 4. Check if plugins directory was created in isolated environment
    isolated_plugins = isolated_claude / "plugins"
    if not check("Isolated plugins directory created", isolated_plugins.exists()):
        failures += 1
        print("  Expected Claude to create ~/.claude/plugins", file=sys.stderr)

    # 5. Check if lockplane plugin was installed
    if isolated_plugins.exists():
        # Check for installed_plugins.json
        installed_file = isolated_plugins / "installed_plugins.json"

        if check("installed_plugins.json exists", installed_file.exists()):
            try:
                with open(installed_file) as f:
                    installed = json.load(f)

                has_lockplane = any(
                    "lockplane" in plugin_name.lower()
                    for plugin_name in installed.get("plugins", {}).keys()
                )

                if not check("Lockplane plugin is installed", has_lockplane):
                    failures += 1
                    print(f"  Installed plugins: {list(installed.get('plugins', {}).keys())}", file=sys.stderr)
                    print("  Expected: lockplane@lockplane-tools or similar", file=sys.stderr)
            except json.JSONDecodeError as e:
                check("installed_plugins.json is valid JSON", False, str(e))
                failures += 1
        else:
            failures += 1
            print("  Plugin should be recorded in installed_plugins.json", file=sys.stderr)

    # 6. Check if marketplace was added
    if isolated_plugins.exists():
        marketplaces_file = isolated_plugins / "known_marketplaces.json"

        if check("known_marketplaces.json exists", marketplaces_file.exists()):
            try:
                with open(marketplaces_file) as f:
                    marketplaces = json.load(f)

                has_lockplane_marketplace = "lockplane-tools" in marketplaces or any(
                    "lockplane" in name.lower() for name in marketplaces.keys()
                )

                if not check("Lockplane marketplace is registered", has_lockplane_marketplace):
                    failures += 1
                    print(f"  Registered marketplaces: {list(marketplaces.keys())}", file=sys.stderr)
                    print("  Expected: lockplane-tools", file=sys.stderr)
            except json.JSONDecodeError as e:
                check("known_marketplaces.json is valid JSON", False, str(e))
                failures += 1
        else:
            failures += 1
            print("  Marketplace should be registered in known_marketplaces.json", file=sys.stderr)

    # 7. Check if skill files exist
    if isolated_plugins.exists():
        # Look for skill files in the marketplaces directory
        skill_files = list(isolated_plugins.glob("**/SKILL.md"))
        lockplane_skills = [f for f in skill_files if "lockplane" in str(f).lower()]

        if not check("Lockplane skill files found", len(lockplane_skills) > 0):
            failures += 1
            print(f"  Found {len(skill_files)} total skill files", file=sys.stderr)
            print(f"  Found {len(lockplane_skills)} lockplane skill files", file=sys.stderr)

    print()
    print("=" * 60)
    print("TDD Status Report")
    print("=" * 60)

    if failures == 0:
        print("✅ All validations passed!")
        print("The Lockplane plugin auto-install is working correctly.")
        print()
        print("This means:")
        print("  ✓ Claude recognized the need for Lockplane expertise")
        print("  ✓ Claude installed the plugin automatically")
        print("  ✓ The plugin marketplace is properly configured")
        print("  ✓ The skill is available for use")
        return 0
    else:
        print(f"❌ {failures} validation(s) failed (EXPECTED for TDD)")
        print()
        print("This test is designed to FAIL until:")
        print("  1. Claude Code recognizes Lockplane mentions in prompts")
        print("  2. Claude Code automatically installs the plugin from GitHub")
        print("  3. The plugin marketplace configuration is complete")
        print("  4. The plugin installation completes successfully")
        print()
        print("Debugging tips:")
        print(f"  - Check Claude's output: {build_dir}/claude_output.txt")
        print(f"  - Check isolated config: {isolated_claude}")
        print(f"  - Run scenario manually for interactive debugging")
        print()
        print("Next steps:")
        print("  - Verify Claude Code can discover plugins from GitHub repos")
        print("  - Add hints/triggers for Claude to recognize Lockplane")
        print("  - Test plugin discovery and installation flow")
        print("  - Re-run this test until it passes")
        return 1


if __name__ == "__main__":
    sys.exit(main())
