#!/bin/bash
# Lockplane installation script
# Usage: curl -sSL https://raw.githubusercontent.com/lockplane/lockplane/main/install.sh | bash

set -e

# Detect OS and architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64)
        ARCH="x86_64"
        ;;
    arm64|aarch64)
        ARCH="arm64"
        ;;
    *)
        echo "Unsupported architecture: $ARCH"
        exit 1
        ;;
esac

case "$OS" in
    darwin)
        OS="Darwin"
        ;;
    linux)
        OS="Linux"
        ;;
    *)
        echo "Unsupported OS: $OS"
        exit 1
        ;;
esac

# Get latest version
LATEST_VERSION=$(curl -s https://api.github.com/repos/lockplane/lockplane/releases/latest | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST_VERSION" ]; then
    echo "Failed to get latest version"
    exit 1
fi

echo "Installing Lockplane $LATEST_VERSION for ${OS}_${ARCH}..."

# Download URL
DOWNLOAD_URL="https://github.com/lockplane/lockplane/releases/download/${LATEST_VERSION}/lockplane_${LATEST_VERSION#v}_${OS}_${ARCH}.tar.gz"

# Create temp directory
TMP_DIR=$(mktemp -d)
trap "rm -rf $TMP_DIR" EXIT

# Download and extract
echo "Downloading from $DOWNLOAD_URL..."
curl -sL "$DOWNLOAD_URL" | tar -xz -C "$TMP_DIR"

# Determine install location
if [ -w "/usr/local/bin" ]; then
    INSTALL_DIR="/usr/local/bin"
elif [ -w "$HOME/.local/bin" ]; then
    INSTALL_DIR="$HOME/.local/bin"
    mkdir -p "$INSTALL_DIR"
else
    INSTALL_DIR="$HOME/bin"
    mkdir -p "$INSTALL_DIR"
fi

# Install binary
echo "Installing lockplane to $INSTALL_DIR..."
mv "$TMP_DIR/lockplane" "$INSTALL_DIR/lockplane"
chmod +x "$INSTALL_DIR/lockplane"

# Verify installation
if command -v lockplane >/dev/null 2>&1; then
    echo "✅ Lockplane installed successfully!"
    lockplane --version 2>/dev/null || echo "Lockplane $(lockplane introspect -h 2>&1 | head -1)"
else
    echo "⚠️  Lockplane installed to $INSTALL_DIR/lockplane"
    echo "   Add $INSTALL_DIR to your PATH to use 'lockplane' command:"
    echo "   export PATH=\"$INSTALL_DIR:\$PATH\""
fi

echo ""
echo "Get started: https://github.com/lockplane/lockplane#quick-start"
