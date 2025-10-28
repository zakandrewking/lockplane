#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation for plugin installation scenario.

This should FAIL until the Lockplane plugin is properly configured.

Checks:
1. Claude Code is available
2. Plugin system is working
3. Lockplane plugin was installed or attempted to be installed
4. Lockplane skill is available after installation
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

    os.chdir(build_dir)

    failures = 0

    print("=== Validating Plugin Installation (TDD - Expected to FAIL) ===\n")

    # 1. Check Claude Code is available
    try:
        result = subprocess.run(
            ["claude", "--version"],
            capture_output=True,
            text=True,
            check=False,
        )
        claude_available = result.returncode == 0
        if not check("Claude Code CLI is available", claude_available):
            failures += 1
            print("  Install Claude Code to run this test", file=sys.stderr)
    except FileNotFoundError:
        check("Claude Code CLI is available", False, "Command 'claude' not found")
        failures += 1

    # 2. Check if plugin system is working
    try:
        result = subprocess.run(
            ["claude", "/plugin", "list"],
            capture_output=True,
            text=True,
            check=False,
        )
        plugin_system_works = result.returncode == 0 or "plugin" in result.stdout.lower()
        if not check("Plugin system is accessible", plugin_system_works):
            failures += 1
    except FileNotFoundError:
        check("Plugin system is accessible", False)
        failures += 1

    # 3. Check if Lockplane plugin is installed
    try:
        result = subprocess.run(
            ["claude", "/plugin", "list"],
            capture_output=True,
            text=True,
            check=False,
        )

        # Look for lockplane in the installed plugins
        has_lockplane = "lockplane" in result.stdout.lower()

        if not check("Lockplane plugin is installed", has_lockplane, "Plugin not found in /plugin list"):
            failures += 1
            print("  Expected: Claude should have installed lockplane plugin", file=sys.stderr)
            print("  Actual: Plugin not found", file=sys.stderr)

    except FileNotFoundError:
        check("Lockplane plugin is installed", False)
        failures += 1

    # 4. Check if Lockplane skill is available
    try:
        result = subprocess.run(
            ["claude", "/skill", "list"],
            capture_output=True,
            text=True,
            check=False,
        )

        has_lockplane_skill = "lockplane" in result.stdout.lower()

        if not check("Lockplane skill is available", has_lockplane_skill, "Skill not found in /skill list"):
            failures += 1
            print("  Expected: lockplane skill should be available after plugin install", file=sys.stderr)
            print("  Actual: Skill not found", file=sys.stderr)

    except FileNotFoundError:
        check("Lockplane skill is available", False)
        failures += 1

    # 5. Check Claude's output/behavior (if available)
    if Path("claude_output.txt").exists():
        output = Path("claude_output.txt").read_text()

        # Check if Claude attempted to install the plugin
        install_attempted = any([
            "/plugin install" in output,
            "installing lockplane" in output.lower(),
            "plugin marketplace" in output.lower(),
        ])

        if not check("Claude attempted to install plugin", install_attempted):
            failures += 1
            print("  Expected: Claude should recognize need for Lockplane plugin", file=sys.stderr)
            print("  Expected: Claude should run '/plugin install lockplane'", file=sys.stderr)
            print("  Actual: No plugin installation attempt found in output", file=sys.stderr)

    print()
    print("=" * 60)
    print("TDD Status Report")
    print("=" * 60)

    if failures == 0:
        print("✅ All validations passed!")
        print("The Lockplane plugin system is working correctly.")
        return 0
    else:
        print(f"❌ {failures} validation(s) failed (EXPECTED for TDD)")
        print()
        print("This test is designed to FAIL until:")
        print("  1. The Lockplane plugin is properly published")
        print("  2. Claude Code can discover it from the GitHub link")
        print("  3. Claude Code recognizes when to install it")
        print("  4. The plugin installation completes successfully")
        print()
        print("Next steps:")
        print("  - Complete the plugin marketplace configuration")
        print("  - Add plugin discovery hints to the Lockplane repo")
        print("  - Test that Claude Code can find and install the plugin")
        print("  - Re-run this test until it passes")
        return 1


if __name__ == "__main__":
    sys.exit(main())
