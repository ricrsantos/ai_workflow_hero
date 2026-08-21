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

# Require an exact tag on the current commit (no "dev" fallback).
# Hero 2.5.0 (C6 Codex adapter): tag the release commit as v2.5.0 before running.
TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -z "${TAG}" ]; then
  echo "[ERROR] Current commit is not tagged. Tag the release commit first (e.g. git tag v2.5.0)." >&2
  exit 1
fi

# Git tags use a leading "v" (v2.5.0); CLI version omits it (2.5.0).
# Injected via -ldflags "-X main.version=${VERSION}" — do not hardcode SemVer here.
VERSION="${TAG#v}"
echo "Building version: ${VERSION} (tag ${TAG})"

DIST="dist"
rm -rf "${DIST}"
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
  OUTPUT="${DIST}/hero_${TAG}_${OS}_${ARCH}"
  echo "→ Building ${OUTPUT}..."
  GOOS="${OS}" GOARCH="${ARCH}" go build \
    -ldflags "${LDFLAGS}" \
    -o "${OUTPUT}" \
    "${MODULE_PATH}"
  chmod +x "${OUTPUT}"
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
  BIN="${DIST}/hero_${TAG}_${OS}_${ARCH}"
  (cd "${DIST}" && ${SHA_CMD} "hero_${TAG}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
done

echo ""
echo "✓ Release artifacts in ${DIST}/"
ls -lh "${DIST}/"
echo ""
echo "✓ Checksums written to ${CHECKSUMS_FILE}"
cat "${CHECKSUMS_FILE}"
