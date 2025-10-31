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
    print("Manually copying plugin files and registering...\n")

    # Create plugin directories
    plugins_dir = isolated_claude / "plugins"
    plugins_dir.mkdir(parents=True, exist_ok=True)

    marketplaces_dir = plugins_dir / "marketplaces"
    marketplaces_dir.mkdir(parents=True, exist_ok=True)

    marketplace_name = "lockplane-tools"
    marketplace_dir = marketplaces_dir / marketplace_name
    marketplace_dir.mkdir(parents=True, exist_ok=True)

    # Copy the entire lockplane repo to the marketplace directory
    print(f"Copying {lockplane_repo} to {marketplace_dir}...")
    shutil.copytree(
        lockplane_repo,
        marketplace_dir,
        dirs_exist_ok=True,
        ignore=shutil.ignore_patterns(
            '.git', '__pycache__', '*.pyc', 'node_modules',
            'dist', 'build', 'scenarios/*/build'
        )
    )

    # Create known_marketplaces.json
    marketplaces_config = {
        marketplace_name: {
            "source": {
                "source": "local",
                "path": str(lockplane_repo)
            },
            "installLocation": str(marketplace_dir),
            "lastUpdated": "2025-10-28T00:00:00.000Z"
        }
    }

    marketplaces_file = plugins_dir / "known_marketplaces.json"
    with open(marketplaces_file, "w") as f:
        json.dump(marketplaces_config, f, indent=2)

    print(f"✓ Created {marketplaces_file}")

    # Create installed_plugins.json
    installed_plugins = {
        "version": 1,
        "plugins": {
            f"lockplane@{marketplace_name}": {
                "version": "1.0.0",
                "installedAt": "2025-10-28T00:00:00.000Z",
                "lastUpdated": "2025-10-28T00:00:00.000Z",
                "installPath": str(marketplace_dir / "claude-plugin"),
                "isLocal": True
            }
        }
    }

    installed_file = plugins_dir / "installed_plugins.json"
    with open(installed_file, "w") as f:
        json.dump(installed_plugins, f, indent=2)

    print(f"✓ Created {installed_file}")

    # Save installation log
    install_log = f"""Plugin installed successfully!

Marketplace: {marketplace_name}
Location: {marketplace_dir}
Plugin path: {marketplace_dir / 'claude-plugin'}

Files created:
- {marketplaces_file}
- {installed_file}

Plugin structure:
"""
    # List key plugin files
    skill_file = marketplace_dir / "claude-plugin" / "skills" / "lockplane" / "SKILL.md"
    if skill_file.exists():
        install_log += f"- ✓ Skill file: {skill_file}\n"
    else:
        install_log += f"- ✗ Skill file not found\n"

    Path("install_output.txt").write_text(install_log)
    print("\n✓ Plugin installed successfully")
    print(f"  Marketplace: {marketplace_name}")
    print(f"  Location: {marketplace_dir}")

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
