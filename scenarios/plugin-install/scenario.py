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


def run_cmd(cmd: list[str], check: bool = True, env: dict = None, **kwargs) -> subprocess.CompletedProcess:
    """Run a command and return the result."""
    cmd_str = ' '.join(str(c) for c in cmd)
    print(f"$ {cmd_str}")
    return subprocess.run(cmd, capture_output=True, text=True, check=check, env=env, **kwargs)


def main():
    """Run the plugin installation scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    # Clean up
    if build_dir.exists():
        shutil.rmtree(build_dir)
    build_dir.mkdir(parents=True)

    # Create isolated Claude config directory
    isolated_home = build_dir / "isolated_home"
    isolated_home.mkdir(parents=True)
    isolated_claude = isolated_home / ".claude"
    isolated_claude.mkdir(parents=True)

    print("=== Plugin Installation Scenario ===\n")
    print("Testing TDD: This should FAIL until the plugin is complete.\n")
    print(f"📁 Isolated Claude config: {isolated_claude}\n")

    # Change to build directory
    os.chdir(build_dir)

    # Initialize git repository
    print("🔧 Initializing git repository...")
    run_cmd(["git", "init"])
    run_cmd(["git", "config", "user.name", "Test User"])
    run_cmd(["git", "config", "user.email", "test@example.com"])

    # Create initial commit so there's a git history
    Path("README.md").write_text("# Test Project\n")
    run_cmd(["git", "add", "."])
    run_cmd(["git", "commit", "-m", "Initial commit"])

    print("\n🤖 Running Claude Code with isolated config...")
    print("Providing GitHub link: https://github.com/zakandrewking/lockplane\n")

    # The key test: Ask Claude to help with Lockplane, providing the GitHub link
    # Claude should recognize it needs the plugin and install it first
    prompt = """I want to build a simple task tracking app with Next.js and Postgres.
I'd like to use Lockplane for schema management.

Here's the Lockplane repo: https://github.com/zakandrewking/lockplane

Can you help me set this up? Please start by installing any necessary plugins."""

    # Write the prompt to a file for reference
    Path("prompt.txt").write_text(prompt)

    print("Prompt:")
    print("-" * 60)
    print(prompt)
    print("-" * 60)
    print()

    # Set up isolated environment
    # Use HOME to isolate Claude's config
    env = os.environ.copy()
    env["HOME"] = str(isolated_home)

    # Also save the original home for reference
    Path("original_home.txt").write_text(os.environ.get("HOME", ""))
    Path("isolated_home.txt").write_text(str(isolated_home))

    # Try to run Claude Code with the prompt in isolated environment
    try:
        print(f"Running Claude with HOME={isolated_home}")
        print("(This will use a fresh Claude config without existing plugins)\n")

        result = run_cmd(
            ["claude", "--print", prompt],
            check=False,
            env=env,
            timeout=90,
        )

        if result.returncode == 0:
            print("\n✅ Claude Code executed successfully")
        else:
            print(f"\n⚠️  Claude Code returned exit code {result.returncode}")

        # Save the output for validation
        Path("claude_output.txt").write_text(result.stdout)
        Path("claude_stderr.txt").write_text(result.stderr)

        # Print a sample of the output
        output_preview = result.stdout[:500] if result.stdout else "(no output)"
        print(f"\nClaude output preview:\n{output_preview}\n")

    except subprocess.TimeoutExpired:
        print("\n⏱️  Command timed out after 90 seconds")
        print("This might mean Claude is waiting for user input or taking too long")
        Path("timeout.txt").write_text("Command timed out")
    except FileNotFoundError:
        print("\n❌ Claude Code CLI not found")
        print("Install Claude Code CLI to run this test")
        return 1

    print("\n📋 Scenario execution complete")
    print(f"Check isolated config at: {isolated_claude}")
    print("Validation will check if Claude installed the plugin\n")

    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        print(f"\n❌ Error: {e}")
        sys.exit(1)
