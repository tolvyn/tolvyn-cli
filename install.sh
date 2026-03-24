#!/bin/sh
set -e

REPO="tolvyn/tolvyn-cli"
INSTALL_DIR="/usr/local/bin"
BINARY_NAME="tolvyn"

OS="$(uname -s)"
ARCH="$(uname -m)"

case "$OS" in
  Linux)
    case "$ARCH" in
      x86_64) ASSET="tolvyn-linux-amd64" ;;
      aarch64|arm64) ASSET="tolvyn-linux-arm64" ;;
      *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  Darwin)
    case "$ARCH" in
      arm64) ASSET="tolvyn-darwin-arm64" ;;
      x86_64) ASSET="tolvyn-darwin-amd64" ;;
      *) echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  *)
    echo "Unsupported OS: $OS"
    echo "For Windows, download from https://github.com/$REPO/releases"
    exit 1
    ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | sed 's/.*"tag_name": "\(.*\)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release."
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$LATEST/$ASSET"

echo "Installing TOLVYN CLI $LATEST..."
TMP="$(mktemp)"
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "$INSTALL_DIR/$BINARY_NAME"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$TMP" "$INSTALL_DIR/$BINARY_NAME"
fi

echo ""
echo "✓ TOLVYN CLI installed"
echo ""
echo "  tolvyn init    — create your account"
echo "  tolvyn tail    — stream live AI requests"
echo "  tolvyn --help  — all commands"
echo ""
echo "Docs: https://docs.tolvyn.io"
