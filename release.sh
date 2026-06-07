#!/usr/bin/env bash
# release.sh — reproducible cross-compile of the TOLVYN CLI for all published platforms.
#
# Builds the 5 binaries with the EXACT asset names install.sh expects
# (tolvyn-<os>-<arch>[.exe]) into ./dist. It does NOT publish — publishing is a
# separate, deliberate step (see the printed gh command at the end).
#
# Usage:
#   ./release.sh v1.4.0        # version as the first arg, or:
#   VERSION=v1.4.0 ./release.sh
#
# Notes:
#   - CGO_ENABLED=0 → static, portable binaries; cross-compiles cleanly from any host.
#   - -ldflags="-s -w" → stripped (smaller). The user-facing version comes from the
#     `cliVersion` const in cmd/tolvyn-cli/main.go — bump that BEFORE building so the
#     binary's `tolvyn version` matches the release tag. This script warns if they differ.
set -euo pipefail

VERSION="${1:-${VERSION:-}}"
if [ -z "$VERSION" ]; then
  echo "error: version required — usage: ./release.sh v1.4.0  (or VERSION=v1.4.0 ./release.sh)" >&2
  exit 1
fi

cd "$(dirname "$0")"
OUT="dist"
PKG="./cmd/tolvyn-cli"

# Sanity: the binary's embedded cliVersion should match the release tag (tag has a
# leading 'v', the const does not).
CONST_VER="$(grep -oE 'cliVersion = "[^"]+"' cmd/tolvyn-cli/main.go | sed 's/.*"\(.*\)"/\1/')"
if [ "v${CONST_VER}" != "$VERSION" ]; then
  echo "WARNING: cliVersion in main.go is '${CONST_VER}' but release is '${VERSION}'." >&2
  echo "         Bump the const so 'tolvyn version' matches the tag, then re-run." >&2
fi

rm -rf "$OUT"; mkdir -p "$OUT"

# os/arch → matches install.sh asset detection exactly.
TARGETS="linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64"
for t in $TARGETS; do
  os="${t%/*}"; arch="${t#*/}"
  ext=""; [ "$os" = "windows" ] && ext=".exe"
  asset="tolvyn-${os}-${arch}${ext}"
  echo "building ${asset} ..."
  CGO_ENABLED=0 GOOS="$os" GOARCH="$arch" \
    go build -trimpath -ldflags="-s -w" -o "${OUT}/${asset}" "$PKG"
done

echo ""
echo "built ${VERSION} → ${OUT}/:"
( cd "$OUT" && ls -la && echo "--- sha256 ---" && sha256sum tolvyn-* )

cat <<EOF

Next (publish — run manually after review; gh must be authed for tolvyn/tolvyn-cli):
  gh release create ${VERSION} --repo tolvyn/tolvyn-cli \\
    --title "${VERSION}" --notes-file RELEASE_NOTES.md \\
    ${OUT}/tolvyn-linux-amd64 ${OUT}/tolvyn-linux-arm64 \\
    ${OUT}/tolvyn-darwin-amd64 ${OUT}/tolvyn-darwin-arm64 ${OUT}/tolvyn-windows-amd64.exe

install.sh serves whatever GitHub marks as the LATEST release — no manifest to update.
EOF
