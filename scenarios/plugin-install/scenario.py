#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Plugin installation scenario - Test that Claude Code installs the Lockplane plugin.

This is a TDD scenario - it should FAIL until the plugin is properly set up.

Tests that when a user:
1. Asks Claude Code to help set up an app with Lockplane
2. Provides a link to the Lockplane GitHub repo
3. Claude Code will first install the Lockplane plugin before proceeding

Expected behavior:
- Claude should recognize it needs Lockplane expertise
- Claude should install the plugin via /plugin install
- The Lockplane skill should become available
"""

import json
import os
import shutil
import subprocess
import sys
from pathlib import Path


def run_cmd(cmd: list[str], check: bool = True, **kwargs) -> subprocess.CompletedProcess:
    """Run a command and return the result."""
    print(f"$ {' '.join(cmd)}")
    return subprocess.run(cmd, capture_output=True, text=True, check=check, **kwargs)


def main():
    """Run the plugin installation scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    # Clean up
    if build_dir.exists():
        shutil.rmtree(build_dir)
    build_dir.mkdir(parents=True)

    os.chdir(build_dir)

    print("=== Plugin Installation Scenario ===\n")
    print("Testing TDD: This should FAIL until the plugin is complete.\n")

    # Initialize git repository
    print("🔧 Initializing git repository...")
    run_cmd(["git", "init"])
    run_cmd(["git", "config", "user.name", "Test User"])
    run_cmd(["git", "config", "user.email", "test@example.com"])

    # Create initial commit so there's a git history
    Path("README.md").write_text("# Test Project\n")
    run_cmd(["git", "add", "."])
    run_cmd(["git", "commit", "-m", "Initial commit"])

    print("\n🤖 Asking Claude Code to set up an app with Lockplane...")
    print("Providing GitHub link: https://github.com/zakandrewking/lockplane")

    # The key test: Ask Claude to help with Lockplane, providing the GitHub link
    # Claude should recognize it needs the plugin and install it first
    prompt = """I want to build a simple task tracking app with Next.js and Postgres.
I'd like to use Lockplane for schema management.

Here's the Lockplane repo: https://github.com/zakandrewking/lockplane

Can you help me set this up?"""

    # Write the prompt to a file for the test
    Path("prompt.txt").write_text(prompt)

    print("\nPrompt:")
    print("-" * 60)
    print(prompt)
    print("-" * 60)

    # Try to run Claude Code with the prompt
    try:
        # Use a non-interactive approach - write to a file and check later
        # In a real scenario, we'd use Claude Code's API or CLI
        result = run_cmd(
            ["claude", prompt],
            check=False,
            timeout=60,
        )

        if result.returncode == 0:
            print("\n✅ Claude Code executed successfully")
        else:
            print(f"\n⚠️  Claude Code returned exit code {result.returncode}")

        # Save the output for validation
        Path("claude_output.txt").write_text(result.stdout + "\n" + result.stderr)

    except subprocess.TimeoutExpired:
        print("\n⏱️  Command timed out (expected for interactive sessions)")
    except FileNotFoundError:
        print("\n⚠️  Claude Code CLI not found")
        print("This is okay - validation will check if plugin would be installed")

    print("\n📋 Scenario execution complete")
    print("Validation will check if Claude Code attempted to install the plugin\n")

    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        print(f"\n❌ Error: {e}")
        sys.exit(1)
