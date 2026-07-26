#!/usr/bin/env bash
# scripts/build_dev.sh — Cross-compile Hero for local/dev use without a release tag.
# Same outputs as release.sh, but version is derived from the latest repo tag and
# the current short commit: <latest-tag>_<short-commit> (e.g. 0.5.1_9a0e749).
#
# Usage:
#   ./scripts/build_dev.sh
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

LAST_TAG=$(git tag -l --sort=-creatordate | head -n1 || true)
COMMIT=$(git rev-parse --short HEAD)

if [ -n "${LAST_TAG}" ]; then
  VERSION="${LAST_TAG#v}_${COMMIT}"
else
  VERSION="dev_${COMMIT}"
fi

echo "Building dev version: ${VERSION}"

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
  OUTPUT="${DIST}/hero_${VERSION}_${OS}_${ARCH}"
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
  BIN="${DIST}/hero_${VERSION}_${OS}_${ARCH}"
  (cd "${DIST}" && ${SHA_CMD} "hero_${VERSION}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
done

echo ""
echo "✓ Dev build artifacts in ${DIST}/"
ls -lh "${DIST}/"
echo ""
echo "✓ Checksums written to ${CHECKSUMS_FILE}"
cat "${CHECKSUMS_FILE}"
