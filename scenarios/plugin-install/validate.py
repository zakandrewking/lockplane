#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation for plugin installation scenario.

Checks if Claude SUGGESTED installing the Lockplane plugin.

Checks:
1. Claude Code ran successfully
2. Claude's output mentions the Lockplane plugin
3. Claude suggests installing the plugin
4. Claude provides instructions
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
    """Validate the plugin installation scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    if not build_dir.exists():
        print("❌ Build directory not found", file=sys.stderr)
        return 1

    failures = 0

    print("=== Validating Plugin Suggestion (TDD) ===\n")

    # 1. Check Claude Code ran
    claude_output_file = build_dir / "claude_output.txt"
    if not check("Claude Code output exists", claude_output_file.exists()):
        failures += 1
        return 1

    # Read Claude's output
    claude_output = claude_output_file.read_text()
    output_lower = claude_output.lower()

    # 2. Check if Claude mentioned "plugin"
    if not check("Claude mentioned 'plugin'", "plugin" in output_lower):
        failures += 1

    # 3. Check if Claude mentioned "lockplane"
    if not check("Claude mentioned 'lockplane'", "lockplane" in output_lower):
        failures += 1

    # 4. Check if Claude suggested installing the plugin
    suggested_install = any([
        "/plugin" in output_lower and "install" in output_lower,
        "install the lockplane plugin" in output_lower,
        "/plugin install lockplane" in output_lower,
        "plugin install" in output_lower and "lockplane" in output_lower,
    ])

    install_msg = []
    if not suggested_install:
        install_msg.append("Expected: '/plugin install lockplane' or similar")
        install_msg.append("")
        install_msg.append("Claude's response preview:")
        for line in claude_output.split('\n')[:10]:
            install_msg.append(f"  {line[:80]}")

    if not check("Claude suggested installing plugin", suggested_install, '\n'.join(install_msg)):
        failures += 1

    # 5. Check if Claude explained benefits
    explained = any([
        "skill" in output_lower,
        "expert" in output_lower and "lockplane" in output_lower,
        "knowledge" in output_lower,
    ])

    if not check("Claude explained plugin benefits", explained):
        failures += 1

    print()
    print("=" * 60)
    if failures == 0:
        print("✅ PASS - Plugin suggestion works!")
        return 0
    else:
        print(f"❌ FAIL - {failures} check(s) failed")
        print()
        print("To make this pass:")
        print("  - Add plugin mention to Lockplane README.md")
        print("  - Add 'Claude Code Plugin' section")
        print("  - Ensure Claude recognizes the hint")
        return 1


if __name__ == "__main__":
    sys.exit(main())
