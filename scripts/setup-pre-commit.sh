#!/bin/bash
# Setup pre-commit hooks for Lockplane

set -e

echo "Setting up pre-commit hooks for Lockplane..."
echo

# Check if pre-commit is installed
if ! command -v pre-commit &> /dev/null; then
    echo "pre-commit is not installed."
    echo
    echo "Install options:"
    echo "  1. Using pip:     pip install pre-commit"
    echo "  2. Using brew:    brew install pre-commit"
    echo "  3. Using pipx:    pipx install pre-commit"
    echo
    echo "See https://pre-commit.com/#install for more options."
    exit 1
fi

# Install the git hook scripts
echo "Installing pre-commit hooks..."
pre-commit install

echo
echo "✓ Pre-commit hooks installed!"
echo
echo "The hooks will run automatically on 'git commit'."
echo "To run manually: pre-commit run --all-files"
echo
echo "Hooks configured:"
echo "  • go fmt      - Format Go code"
echo "  • go vet      - Check for common errors"
echo "  • errcheck    - Ensure all errors are handled"
echo "  • staticcheck - Advanced linting"
echo "  • go test     - Run tests (disabled by default)"
echo
