#!/usr/bin/env bash
# scripts/release.sh — Cross-compile Hero for all V1 target platforms.
# Reads the version from the current git tag (ADR-010, DEPLOY.md §4.1).
#
# Usage:
#   ./scripts/release.sh
#
# Output:
#   dist/hero_<version>_linux_amd64
#   dist/hero_<version>_linux_arm64
#   dist/hero_<version>_darwin_amd64
#   dist/hero_<version>_darwin_arm64
#   dist/checksums.txt
#
# Requirements: go, git, sha256sum (Linux) or shasum (macOS)

set -euo pipefail

# Determine version from git tag.
VERSION=$(git describe --tags --abbrev=0 2>/dev/null || echo "dev")
echo "Building version: ${VERSION}"

DIST="dist"
mkdir -p "${DIST}"

TARGETS=(
  "linux/amd64"
  "linux/arm64"
  "darwin/amd64"
  "darwin/arm64"
)

MODULE_PATH="./cmd/hero"
LDFLAGS="-X main.version=${VERSION}"

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%/*}"
  ARCH="${TARGET#*/}"
  OUTPUT="${DIST}/hero_${VERSION}_${OS}_${ARCH}"
  echo "→ Building ${OUTPUT}..."
  GOOS="${OS}" GOARCH="${ARCH}" go build \
    -ldflags "${LDFLAGS}" \
    -o "${OUTPUT}" \
    "${MODULE_PATH}"
  echo "✓ Built ${OUTPUT}"
done

# Generate checksums.
CHECKSUMS_FILE="${DIST}/checksums.txt"
rm -f "${CHECKSUMS_FILE}"

if command -v sha256sum &>/dev/null; then
  SHA_CMD="sha256sum"
elif command -v shasum &>/dev/null; then
  SHA_CMD="shasum -a 256"
else
  echo "[ERROR] Neither sha256sum nor shasum found. Cannot generate checksums." >&2
  exit 1
fi

for TARGET in "${TARGETS[@]}"; do
  OS="${TARGET%/*}"
  ARCH="${TARGET#*/}"
  BIN="${DIST}/hero_${VERSION}_${OS}_${ARCH}"
  (cd "${DIST}" && ${SHA_CMD} "hero_${VERSION}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
done

echo ""
echo "✓ Release artifacts in ${DIST}/"
ls -lh "${DIST}/"
echo ""
echo "✓ Checksums written to ${CHECKSUMS_FILE}"
cat "${CHECKSUMS_FILE}"
