#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Validation for basic migration scenario.
"""

import json
import os
import subprocess
import sys
from pathlib import Path


def main():
    """Validate the basic migration scenario results."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    if not build_dir.exists():
        print("❌ Build directory not found", file=sys.stderr)
        return 1

    os.chdir(build_dir)

    failures = 0

    print("=== Validating basic migration scenario ===")

    # Check files were created
    expected_files = ["current.json", "schema.lp.sql", "migration.json"]
    for filename in expected_files:
        if (build_dir / filename).exists():
            print(f"✓ {filename} exists")
        else:
            print(f"✗ {filename} missing", file=sys.stderr)
            failures += 1

    # Check migration.json has steps
    if (build_dir / "migration.json").exists():
        with open("migration.json") as f:
            plan = json.load(f)

        steps = plan.get("steps", [])
        if len(steps) > 0:
            print(f"✓ Migration plan has {len(steps)} step(s)")
        else:
            print("✗ Migration plan has no steps", file=sys.stderr)
            failures += 1

        # Check for table creation step
        has_create_table = any("CREATE TABLE" in step.get("sql", "") for step in steps)
        if has_create_table:
            print("✓ Migration includes CREATE TABLE")
        else:
            print("✗ Migration missing CREATE TABLE", file=sys.stderr)
            failures += 1

    # Check database state
    db_url = os.getenv("DATABASE_URL", "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable")

    try:
        result = subprocess.run(
            ["lockplane", "introspect", "--db", db_url],
            capture_output=True,
            text=True,
            check=True,
        )

        schema = json.loads(result.stdout)
        tables = schema.get("tables", [])

        if len(tables) > 0:
            print(f"✓ Database has {len(tables)} table(s)")
        else:
            print("✗ Database has no tables", file=sys.stderr)
            failures += 1

        # Check for users table
        has_users = any(t.get("name") == "users" for t in tables)
        if has_users:
            print("✓ Database has 'users' table")
        else:
            print("✗ Database missing 'users' table", file=sys.stderr)
            failures += 1

    except Exception as e:
        print(f"✗ Failed to introspect database: {e}", file=sys.stderr)
        failures += 1

    print()
    if failures == 0:
        print("✅ All validations passed")
        return 0
    else:
        print(f"❌ {failures} validation(s) failed", file=sys.stderr)
        return 1


if __name__ == "__main__":
    sys.exit(main())
