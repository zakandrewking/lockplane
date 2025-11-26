#!/usr/bin/env -S uv run
# /// script
# requires-python = ">=3.11"
# dependencies = [
#     "pyyaml>=6.0",
#     "rich>=13.0",
# ]
# ///

"""
Lockplane Evaluation Runner

Discovers and runs scenario evaluations, validates outcomes, and generates reports.
"""

import argparse
import json
import subprocess
import sys
import time
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Optional

import yaml
from rich.console import Console
from rich.table import Table
from rich.progress import Progress, SpinnerColumn, TextColumn

console = Console()


@dataclass
class ScenarioConfig:
    """Configuration for a scenario."""
    name: str
    description: str
    timeout: int = 300
    tags: list[str] = None

    @classmethod
    def from_yaml(cls, path: Path) -> "ScenarioConfig":
        """Load scenario config from YAML file."""
        with open(path) as f:
            data = yaml.safe_load(f)
        return cls(**data)


@dataclass
class ScenarioResult:
    """Result of running a scenario."""
    name: str
    description: str
    passed: bool
    duration_seconds: float
    error_message: Optional[str] = None
    validation_output: str = ""
    tags: list[str] = None


def find_scenarios(scenarios_dir: Path, specific_scenario: str) -> list[Path]:
    """Find the specified scenario directory."""
    scenario_path = scenarios_dir / specific_scenario
    if not scenario_path.exists():
        console.print(f"[red]Error: Scenario '{specific_scenario}' not found[/red]")
        console.print(f"[yellow]Looking in: {scenarios_dir}[/yellow]")

        # List available scenarios
        available = []
        for path in scenarios_dir.iterdir():
            if path.is_dir() and (path / "scenario.yaml").exists():
                available.append(path.name)

        if available:
            console.print("\n[yellow]Available scenarios:[/yellow]")
            for name in sorted(available):
                console.print(f"  - {name}")

        sys.exit(1)

    if not (scenario_path / "scenario.yaml").exists():
        console.print(f"[red]Error: '{specific_scenario}' is not a valid scenario[/red]")
        console.print(f"[yellow]Missing: {scenario_path / 'scenario.yaml'}[/yellow]")
        sys.exit(1)

    return [scenario_path]


def run_script(script_path: Path, timeout: int, verbose: bool = False) -> tuple[bool, str]:
    """Run a Python or bash script and return success status and output."""
    if not script_path.exists():
        return False, f"Script not found: {script_path}"

    try:
        # Determine how to run the script
        if script_path.suffix == ".py":
            cmd = ["uv", "run", str(script_path)]
        else:
            cmd = ["bash", str(script_path)]

        result = subprocess.run(
            cmd,
            cwd=script_path.parent,
            capture_output=True,
            text=True,
            timeout=timeout,
        )

        output = result.stdout + result.stderr

        if verbose:
            console.print(output)

        return result.returncode == 0, output

    except subprocess.TimeoutExpired:
        return False, f"Timeout after {timeout} seconds"
    except Exception as e:
        return False, f"Error running script: {str(e)}"


def run_scenario(scenario_dir: Path, config: ScenarioConfig, verbose: bool = False) -> ScenarioResult:
    """Run a single scenario and validate it."""
    start_time = time.time()

    # Try scenario.py first, fall back to scenario.sh
    scenario_script = scenario_dir / "scenario.py"
    if not scenario_script.exists():
        scenario_script = scenario_dir / "scenario.sh"

    # Try validate.py first, fall back to validate.sh
    validate_script = scenario_dir / "validate.py"
    if not validate_script.exists():
        validate_script = scenario_dir / "validate.sh"

    # Prepare to capture all output
    full_output = []
    full_output.append(f"=== Scenario: {config.name} ===")
    full_output.append(f"Description: {config.description}")
    full_output.append(f"Started at: {time.strftime('%Y-%m-%d %H:%M:%S')}")
    full_output.append("")

    # Run the scenario
    if verbose:
        console.print(f"\n[bold]Running scenario: {config.name}[/bold]")

    full_output.append("--- Scenario Execution ---")
    success, output = run_script(scenario_script, config.timeout, verbose)
    full_output.append(output)
    full_output.append("")

    validation_output = ""

    if not success:
        duration = time.time() - start_time
        full_output.append(f"Status: FAILED")
        full_output.append(f"Duration: {duration:.1f}s")
        full_output.append(f"Error: Scenario execution failed")

        # Save output file
        output_file = scenario_dir / "latest-run.txt"
        output_file.write_text("\n".join(full_output))

        return ScenarioResult(
            name=config.name,
            description=config.description,
            passed=False,
            duration_seconds=duration,
            error_message=f"Scenario execution failed: {output}",
            tags=config.tags,
        )

    # Run validation
    if validate_script.exists():
        if verbose:
            console.print(f"[bold]Validating scenario: {config.name}[/bold]")

        full_output.append("--- Validation ---")
        success, validation_output = run_script(validate_script, 60, verbose)
        full_output.append(validation_output)
        full_output.append("")

        duration = time.time() - start_time
        full_output.append(f"Status: {'PASSED' if success else 'FAILED'}")
        full_output.append(f"Duration: {duration:.1f}s")

        # Save output file
        output_file = scenario_dir / "latest-run.txt"
        output_file.write_text("\n".join(full_output))

        return ScenarioResult(
            name=config.name,
            description=config.description,
            passed=success,
            duration_seconds=duration,
            validation_output=validation_output,
            tags=config.tags,
        )
    else:
        # No validation script - just check scenario ran
        duration = time.time() - start_time
        full_output.append("Status: PASSED (no validation)")
        full_output.append(f"Duration: {duration:.1f}s")

        # Save output file
        output_file = scenario_dir / "latest-run.txt"
        output_file.write_text("\n".join(full_output))

        return ScenarioResult(
            name=config.name,
            description=config.description,
            passed=True,
            duration_seconds=duration,
            validation_output="No validation script found",
            tags=config.tags,
        )


def print_results_table(results: list[ScenarioResult]):
    """Print results in a nice table."""
    table = Table(title="Evaluation Results")
    table.add_column("Scenario", style="cyan")
    table.add_column("Status", justify="center")
    table.add_column("Duration", justify="right")
    table.add_column("Description", style="dim")

    for result in results:
        status = "✅ PASS" if result.passed else "❌ FAIL"
        status_style = "green" if result.passed else "red"
        duration = f"{result.duration_seconds:.1f}s"

        table.add_row(
            result.name,
            f"[{status_style}]{status}[/{status_style}]",
            duration,
            result.description,
        )

    console.print(table)

    # Summary
    passed = sum(1 for r in results if r.passed)
    total = len(results)

    console.print()
    if passed == total:
        console.print(f"[bold green]✅ All {total} scenario(s) passed![/bold green]")
    else:
        console.print(f"[bold red]❌ {total - passed} of {total} scenario(s) failed[/bold red]")
        console.print()
        console.print("[bold]Failed scenarios:[/bold]")
        for result in results:
            if not result.passed:
                console.print(f"  • {result.name}: {result.error_message or 'Validation failed'}")
                if result.validation_output:
                    console.print(f"    {result.validation_output}")


def main():
    parser = argparse.ArgumentParser(
        description="Run a specific Lockplane evaluation scenario",
        epilog="Example: %(prog)s basic-migration"
    )
    parser.add_argument(
        "scenario",
        help="Name of the scenario to run (e.g., 'basic-migration', 'plugin-install')",
    )
    parser.add_argument(
        "--format",
        choices=["table", "json"],
        default="table",
        help="Output format",
    )
    parser.add_argument(
        "--verbose",
        "-v",
        action="store_true",
        help="Show detailed output",
    )
    parser.add_argument(
        "--scenarios-dir",
        type=Path,
        default=Path(__file__).parent,
        help="Directory containing scenarios",
    )

    args = parser.parse_args()

    # Find the specified scenario
    scenarios = find_scenarios(args.scenarios_dir, args.scenario)

    console.print(f"Running scenario: [bold cyan]{args.scenario}[/bold cyan]")

    # Run scenarios
    results = []

    with Progress(
        SpinnerColumn(),
        TextColumn("[progress.description]{task.description}"),
        console=console,
        transient=True,
    ) as progress:
        for scenario_dir in scenarios:
            config = ScenarioConfig.from_yaml(scenario_dir / "scenario.yaml")

            task = progress.add_task(f"Running {config.name}...", total=None)
            result = run_scenario(scenario_dir, config, args.verbose)
            results.append(result)
            progress.remove_task(task)

    # Output results
    if args.format == "json":
        print(json.dumps([asdict(r) for r in results], indent=2))
    else:
        print_results_table(results)

    # Exit with error if any failed
    if any(not r.passed for r in results):
        sys.exit(1)


if __name__ == "__main__":
    main()
