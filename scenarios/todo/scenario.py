#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Todo app scenario - Generate a Next.js todo app with SQLite and Lockplane.
"""

import shutil
import subprocess
import sys
from pathlib import Path


def main():
    """Run the todo app scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    # Clean up previous build
    if build_dir.exists():
        print("🧹 Cleaning up previous build...")
        shutil.rmtree(build_dir)

    # Create fresh build directory
    build_dir.mkdir(parents=True)
    print(f"📁 Created build directory: {build_dir}")

    # Initialize git repository
    print("🔧 Initializing git repository...")
    subprocess.run(["git", "init"], cwd=build_dir, check=True)

    # Run Claude Code
    print("🤖 Running Claude Code...")
    print("Prompt: 'generate a new todo list app using next.js, sqlite, and lockplane'")

    try:
        result = subprocess.run(
            ["claude", "generate a new todo list app using next.js, sqlite, and lockplane"],
            cwd=build_dir,
            check=True,
            capture_output=False,  # Show output in real-time
        )

        if result.returncode == 0:
            print("✅ Scenario completed successfully")
            return 0
        else:
            print("❌ Scenario failed")
            return 1

    except subprocess.CalledProcessError as e:
        print(f"❌ Error running Claude Code: {e}")
        return 1
    except FileNotFoundError:
        print("❌ Error: 'claude' command not found. Is Claude Code installed?")
        return 1


if __name__ == "__main__":
    sys.exit(main())
