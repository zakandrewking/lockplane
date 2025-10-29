#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Plugin access scenario - Install plugin and verify Claude can use the skills.

Tests:
1. Explicitly install the Lockplane plugin in isolated environment
2. Ask Claude a Lockplane-specific question
3. Verify Claude has access to the plugin and skills
4. Verify Claude uses Lockplane expertise in the response
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
    """Run the plugin access scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"
    lockplane_repo = scenario_dir.parent.parent  # Go up to lockplane repo root

    # Clean up
    if build_dir.exists():
        shutil.rmtree(build_dir)
    build_dir.mkdir(parents=True)

    # Create isolated Claude config directory
    isolated_home = build_dir / "isolated_home"
    isolated_home.mkdir(parents=True)
    isolated_claude = isolated_home / ".claude"
    isolated_claude.mkdir(parents=True)

    print("=== Plugin Access Scenario ===\n")
    print("This tests that Claude can access and use an installed plugin.\n")
    print(f"📁 Isolated Claude config: {isolated_claude}")
    print(f"📦 Lockplane repo: {lockplane_repo}\n")

    # Change to build directory
    os.chdir(build_dir)

    # Initialize git repository
    print("🔧 Initializing git repository...")
    run_cmd(["git", "init"], check=True)
    run_cmd(["git", "config", "user.name", "Test User"], check=True)
    run_cmd(["git", "config", "user.email", "test@example.com"], check=True)

    # Create initial commit
    Path("README.md").write_text("# Test Project\n")
    run_cmd(["git", "add", "."], check=True)
    run_cmd(["git", "commit", "-m", "Initial commit"], check=True)

    # Set up isolated environment
    env = os.environ.copy()
    env["HOME"] = str(isolated_home)

    print("\n📦 Installing Lockplane plugin in isolated environment...")
    plugin_path = lockplane_repo / "claude-plugin"

    # Install the plugin using Claude CLI
    # Note: This uses the /plugin install command
    install_cmd = f"/plugin install {plugin_path}"
    print(f"Command: {install_cmd}\n")

    # Write a script that installs the plugin
    install_script = f"""
# Install the Lockplane plugin
claude "{install_cmd}" --print
"""

    Path("install_plugin.sh").write_text(install_script)

    # Try to run the install command
    try:
        result = run_cmd(
            ["bash", "install_plugin.sh"],
            check=False,
            env=env,
            timeout=30,
        )

        Path("install_output.txt").write_text(result.stdout)
        Path("install_stderr.txt").write_text(result.stderr)

        if result.returncode == 0:
            print("✓ Plugin installation command completed")
        else:
            print(f"⚠️  Plugin installation returned code {result.returncode}")

        print(f"Output: {result.stdout[:200]}...")

    except subprocess.TimeoutExpired:
        print("⏱️  Installation timed out")
        Path("install_timeout.txt").write_text("Timeout")

    print("\n🤖 Testing plugin access with Claude...")
    print("Asking a Lockplane-specific question...\n")

    # Ask a question that should trigger the Lockplane skill
    prompt = """I have a users table and I want to add a new email column.
The table already has data in it.

How should I do this safely with Lockplane?"""

    Path("prompt.txt").write_text(prompt)

    print("Prompt:")
    print("-" * 60)
    print(prompt)
    print("-" * 60)
    print()

    # Run Claude with the prompt
    try:
        print(f"Running Claude with HOME={isolated_home}\n")

        result = run_cmd(
            ["claude", "--print", prompt],
            check=False,
            env=env,
            timeout=60,
        )

        if result.returncode == 0:
            print("\n✅ Claude executed successfully")
        else:
            print(f"\n⚠️  Claude returned exit code {result.returncode}")

        # Save the output
        Path("claude_output.txt").write_text(result.stdout)
        Path("claude_stderr.txt").write_text(result.stderr)

        # Print FULL output
        print("\n" + "=" * 70)
        print("FULL CLAUDE OUTPUT:")
        print("=" * 70)
        print(result.stdout if result.stdout else "(no stdout)")

        if result.stderr:
            print("\n" + "=" * 70)
            print("STDERR:")
            print("=" * 70)
            print(result.stderr)

        print("=" * 70)

    except subprocess.TimeoutExpired:
        print("\n⏱️  Command timed out")
        Path("timeout.txt").write_text("Timeout")
    except FileNotFoundError:
        print("\n❌ Claude Code CLI not found")
        return 1

    print("\n📋 Scenario execution complete")
    print(f"Isolated config: {isolated_claude}")
    print("Validation will check if Claude used the Lockplane skill\n")

    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except Exception as e:
        print(f"\n❌ Error: {e}")
        import traceback
        traceback.print_exc()
        sys.exit(1)
