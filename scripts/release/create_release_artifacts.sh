#!/usr/bin/env sh
set -eu

VERSION="${1:-0.0.0-dev}"
OUT_DIR="${2:-dist/release}"

mkdir -p "$OUT_DIR"

ARCHIVE_BASE="diag-gateway-${VERSION}"
ARCHIVE_PATH="$OUT_DIR/${ARCHIVE_BASE}.tar.gz"
CHECKSUM_PATH="$OUT_DIR/${ARCHIVE_BASE}.sha256"

# Source snapshot archive for early-stage releases before binaries exist.
git archive --format=tar.gz --output "$ARCHIVE_PATH" HEAD

# Generate checksum manifest.
# shellcheck disable=SC2164
cd "$OUT_DIR"
if command -v sha256sum >/dev/null 2>&1; then
  sha256sum "$(basename "$ARCHIVE_PATH")" > "$(basename "$CHECKSUM_PATH")"
elif command -v shasum >/dev/null 2>&1; then
  shasum -a 256 "$(basename "$ARCHIVE_PATH")" > "$(basename "$CHECKSUM_PATH")"
else
  echo "No sha256 checksum tool available" >&2
  exit 1
fi

echo "Created release artifacts:"
echo "- $ARCHIVE_PATH"
echo "- $CHECKSUM_PATH"
