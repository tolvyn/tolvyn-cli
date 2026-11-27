#!/bin/sh
set -e

REPO="tolvyn/tolvyn-cli"
INSTALL_DIR="${TOLVYN_INSTALL_DIR:-/usr/local/bin}"
BINARY_NAME="tolvyn"

# verify_checksum <file> <SHA256SUMS> <asset name>: the file's SHA-256 must be the one SHA256SUMS lists for the asset.
verify_checksum() {
  want=$(awk -v a="$3" '$2 == a || $2 == "*"a { print $1; exit }' "$2")
  if [ -z "$want" ]; then
    echo "SHA256SUMS lists no $3."
    return 1
  fi
  if command -v sha256sum >/dev/null 2>&1; then
    got=$(sha256sum "$1" | cut -d' ' -f1)
  else
    got=$(shasum -a 256 "$1" | cut -d' ' -f1)
  fi
  if [ "$got" != "$want" ]; then
    echo "Checksum mismatch for $3: expected $want, got $got."
    return 1
  fi
}

# version_ok <v>: a release version, vN.N.N, on one line (a case pattern sees the whole string; grep sees lines).
version_ok() {
  case "$1" in *[!v0-9.]*) return 1 ;; esac
  printf '%s\n' "$1" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+$'
}

# cosign_ok <cosign version>: 3.1.3 or later, read as vMAJOR.MINOR.PATCH and compared as numbers (a suffix such as
# -rc.1 is ignored). 3.1.3 resolved GHSA-fx35-mq7g-6g98, "a verification bypass using an unexpected public key in a
# legacy bundle" (cosign's release notes); the bundle checked here comes from the same release it vouches for.
cosign_ok() {
  set -- $(printf '%s\n' "$1" | sed -n 's/^v\{0,1\}\([0-9][0-9]*\)\.\([0-9][0-9]*\)\.\([0-9][0-9]*\).*/\1 \2 \3/p')
  [ $# -eq 3 ] || return 1
  [ "$1" -gt 3 ] || { [ "$1" -eq 3 ] && { [ "$2" -gt 1 ] || { [ "$2" -eq 1 ] && [ "$3" -ge 3 ]; }; }; }
}

# install_binary <file> <dir> <name>: the binary installed 0755, and owned by root when sudo is needed. A move would
# keep the downloaded file's owner (the installing user) and mktemp's mode.
install_binary() {
  if [ -w "$2" ]; then
    install -m 0755 "$1" "$2/$3"
  else
    echo "Installing to $2 (requires sudo)..."
    sudo install -o 0 -g 0 -m 0755 "$1" "$2/$3"
  fi
}

# Sourced with TOLVYN_INSTALL_LIB=1 (the CLI's tests): the functions only.
if [ -n "${TOLVYN_INSTALL_LIB:-}" ]; then
  return 0
fi

# TOLVYN_REQUIRE_SIGNATURE=1: install only if the release's signature was verified (cosign 3.1.3 or later on PATH). Any
# other value but 0 is refused, so a mistyped "yes" cannot quietly mean "not required".
REQUIRE_SIGNATURE="${TOLVYN_REQUIRE_SIGNATURE:-0}"
case "$REQUIRE_SIGNATURE" in
  0|1) ;;
  *) echo "TOLVYN_REQUIRE_SIGNATURE must be 1 (or 0, or unset). Nothing was installed."; exit 1 ;;
esac

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

# TOLVYN_VERSION=v1.3.2 installs that release; otherwise the latest. TOLVYN_INSTALL_DIR installs elsewhere than
# /usr/local/bin.
LATEST="${TOLVYN_VERSION:-}"
if [ -z "$LATEST" ]; then
  LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
    | grep '"tag_name"' | sed 's/.*"tag_name": "\(.*\)".*/\1/')
fi

if [ -z "$LATEST" ]; then
  echo "Failed to fetch latest release."
  exit 1
fi
if ! version_ok "$LATEST"; then
  echo "Not a release version: $LATEST (TOLVYN_VERSION takes the form v1.3.2). Nothing was installed."
  exit 1
fi

BASE="https://github.com/$REPO/releases/download/$LATEST"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

echo "Installing TOLVYN CLI $LATEST..."
curl -fsSL "$BASE/$ASSET" -o "$WORK/$ASSET"
if ! curl -fsSL "$BASE/SHA256SUMS" -o "$WORK/SHA256SUMS"; then
  echo "This release has no SHA256SUMS, so the download cannot be checked. Nothing was installed."
  exit 1
fi
if ! verify_checksum "$WORK/$ASSET" "$WORK/SHA256SUMS" "$ASSET"; then
  echo "Nothing was installed."
  exit 1
fi
echo "Checksum verified (SHA256SUMS)."

# The checksums must also carry this repository's release workflow's signature, checked with cosign 3.1.3 or later
# (cosign_ok): the release is signed by cosign v3 in Sigstore's bundle format. No cosign, an older one, or one whose
# version cannot be read are all the same: the SHA256 checksum only (an older cosign is never asked to verify), unless
# TOLVYN_REQUIRE_SIGNATURE=1.
COSIGN_VERSION=""
if command -v cosign >/dev/null 2>&1; then
  COSIGN_VERSION=$(cosign version 2>&1 | sed -n '/^GitVersion:/{s/^GitVersion:[[:space:]]*//p;q;}')
fi
SIGNATURE_VERIFIED=no
if ! cosign_ok "$COSIGN_VERSION"; then
  echo "cosign 3.1.3 or later is needed to check the signature; checking the SHA256 checksum only"
else
  if ! curl -fsSL "$BASE/SHA256SUMS.sigstore.json" -o "$WORK/SHA256SUMS.sigstore.json" ||
     ! cosign verify-blob "$WORK/SHA256SUMS" --bundle "$WORK/SHA256SUMS.sigstore.json" \
       --certificate-identity "https://github.com/$REPO/.github/workflows/release.yml@refs/tags/$LATEST" \
       --certificate-oidc-issuer "https://token.actions.githubusercontent.com" >/dev/null 2>&1; then
    echo "The release's signature does not verify. Nothing was installed."
    exit 1
  fi
  echo "Signature verified (cosign)."
  SIGNATURE_VERIFIED=yes
fi
if [ "$REQUIRE_SIGNATURE" = 1 ] && [ "$SIGNATURE_VERIFIED" != yes ]; then
  echo "TOLVYN_REQUIRE_SIGNATURE=1, and the signature was not verified (cosign 3.1.3 or later is needed). Nothing was installed."
  exit 1
fi

install_binary "$WORK/$ASSET" "$INSTALL_DIR" "$BINARY_NAME"

echo ""
echo "✓ TOLVYN CLI installed"
echo ""
echo "  tolvyn init    — create your account"
echo "  tolvyn tail    — stream live AI requests"
echo "  tolvyn --help  — all commands"
echo ""
echo "Docs: https://docs.tolvyn.io"
