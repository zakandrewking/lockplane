#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = []
# ///

"""
Basic migration scenario - Test the core Lockplane workflow.

Tests:
1. introspect empty database
2. define a simple schema
3. generate migration plan
4. validate the plan
5. apply to database
"""

import json
import os
import subprocess
import sys
from pathlib import Path


def run_cmd(cmd: list[str], check: bool = True) -> subprocess.CompletedProcess:
    """Run a command and return the result."""
    print(f"$ {' '.join(cmd)}")
    return subprocess.run(cmd, capture_output=True, text=True, check=check)


def main():
    """Run the basic migration scenario."""
    scenario_dir = Path(__file__).parent
    build_dir = scenario_dir / "build"

    # Clean up
    if build_dir.exists():
        import shutil
        shutil.rmtree(build_dir)
    build_dir.mkdir(parents=True)

    os.chdir(build_dir)

    print("=== Basic Migration Scenario ===\n")

    # Check environment
    db_url = os.getenv("DATABASE_URL")
    shadow_db_url = os.getenv("SHADOW_DATABASE_URL")

    if not db_url:
        print("⚠️  DATABASE_URL not set, using default")
        db_url = "postgres://lockplane:lockplane@localhost:5432/lockplane?sslmode=disable"

    if not shadow_db_url:
        print("⚠️  SHADOW_DATABASE_URL not set, using default")
        shadow_db_url = "postgres://lockplane:lockplane@localhost:5433/lockplane?sslmode=disable"

    print(f"Database: {db_url}")
    print(f"Shadow DB: {shadow_db_url}\n")

    # Step 1: Introspect current (empty) database
    print("1️⃣  Introspecting current database state...")
    result = run_cmd(["lockplane", "introspect", "--db", db_url])

    current_schema = json.loads(result.stdout)
    with open("current.json", "w") as f:
        json.dump(current_schema, f, indent=2)

    print(f"   Tables found: {len(current_schema.get('tables', []))}")

    # Step 2: Create desired schema
    print("\n2️⃣  Creating desired schema...")
    schema_sql = """CREATE TABLE users (
    id BIGINT PRIMARY KEY,
    email TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW()
);

CREATE UNIQUE INDEX users_email_idx ON users(email);
"""

    with open("schema.lp.sql", "w") as f:
        f.write(schema_sql)

    print("   Created schema.lp.sql")

    # Step 3: Generate migration plan
    print("\n3️⃣  Generating migration plan...")
    result = run_cmd([
        "lockplane", "plan",
        "--from", "current.json",
        "--to", "schema.lp.sql",
    ])

    plan = json.loads(result.stdout)
    with open("migration.json", "w") as f:
        json.dump(plan, f, indent=2)

    print(f"   Generated {len(plan.get('steps', []))} migration step(s)")

    # Step 4: Validate the plan
    print("\n4️⃣  Validating migration plan...")
    result = run_cmd([
        "lockplane", "plan",
        "--from", "current.json",
        "--to", "schema.lp.sql",
        "--validate",
    ])

    print("   ✓ Plan validation passed")

    # Step 5: Apply the migration
    print("\n5️⃣  Applying migration...")
    result = run_cmd([
        "lockplane", "apply",
        "--plan", "migration.json",
        "--db", db_url,
        "--shadow-db", shadow_db_url,
    ])

    print("   ✓ Migration applied successfully")

    # Step 6: Verify the result
    print("\n6️⃣  Verifying database state...")
    result = run_cmd(["lockplane", "introspect", "--db", db_url])

    new_schema = json.loads(result.stdout)
    print(f"   Tables now: {len(new_schema.get('tables', []))}")

    for table in new_schema.get("tables", []):
        print(f"   - {table.get('name')}")

    print("\n✅ Basic migration scenario completed successfully")
    return 0


if __name__ == "__main__":
    try:
        sys.exit(main())
    except subprocess.CalledProcessError as e:
        print(f"\n❌ Command failed: {' '.join(e.cmd)}")
        print(f"   Exit code: {e.returncode}")
        if e.stdout:
            print(f"   stdout: {e.stdout}")
        if e.stderr:
            print(f"   stderr: {e.stderr}")
        sys.exit(1)
    except Exception as e:
        print(f"\n❌ Error: {e}")
        sys.exit(1)
