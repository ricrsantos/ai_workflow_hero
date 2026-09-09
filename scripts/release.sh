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
# After build (linux/amd64 only), installs local release copies:
#   /home/ricardo/installable/hero/hero
#   /home/ricardo/.workflow-hero/plugins/telegram/hero-telegram-daemon
# and updates /home/ricardo/.workflow-hero/plugins/telegram/manifest.json.
#
# Requirements: go, git, sha256sum (Linux) or shasum (macOS), python3 (manifest update)

set -euo pipefail

# Require an exact tag on the current commit (no "dev" fallback).
# Latest: tag the release commit as v2.7.0 (or newer) before running.
TAG=$(git describe --tags --exact-match 2>/dev/null || true)
if [ -z "${TAG}" ]; then
  echo "[ERROR] Current commit is not tagged. Tag the release commit first (e.g. git tag v2.7.0)." >&2
  exit 1
fi

# Git tags use a leading "v" (v2.6.0); CLI version omits it (2.6.0).
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
DAEMON_MODULE_PATH="./cmd/hero-telegram-daemon"
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

  # Optional Telegram plugin daemon (ADR-059): publish a platform-matched
  # daemon binary so `hero plugin install telegram` can download it from GitHub
  # Releases for the matching Hero version.
  DAEMON_OUTPUT="${DIST}/hero-telegram-daemon_${TAG}_${OS}_${ARCH}"
  echo "→ Building ${DAEMON_OUTPUT}..."
  GOOS="${OS}" GOARCH="${ARCH}" go build \
    -ldflags "${LDFLAGS}" \
    -o "${DAEMON_OUTPUT}" \
    "${DAEMON_MODULE_PATH}"
  chmod +x "${DAEMON_OUTPUT}"
  echo "✓ Built ${DAEMON_OUTPUT}"
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
  (cd "${DIST}" && ${SHA_CMD} "hero_${TAG}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
  (cd "${DIST}" && ${SHA_CMD} "hero-telegram-daemon_${TAG}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
done

# Local install (linux/amd64 only).
HERO_INSTALL_PATH="/home/ricardo/installable/hero/hero"
LINUX_AMD64_HERO="${DIST}/hero_${TAG}_linux_amd64"
LINUX_AMD64_DAEMON="${DIST}/hero-telegram-daemon_${TAG}_linux_amd64"
TELEGRAM_PLUGIN_DIR="/home/ricardo/.workflow-hero/plugins/telegram"
TELEGRAM_DAEMON_PATH="${TELEGRAM_PLUGIN_DIR}/hero-telegram-daemon"
TELEGRAM_MANIFEST="${TELEGRAM_PLUGIN_DIR}/manifest.json"

echo ""
echo "→ Installing local release binaries (linux/amd64)..."
mkdir -p "$(dirname "${HERO_INSTALL_PATH}")" "${TELEGRAM_PLUGIN_DIR}"
cp -f "${LINUX_AMD64_HERO}" "${HERO_INSTALL_PATH}"
chmod +x "${HERO_INSTALL_PATH}"
echo "✓ Installed ${HERO_INSTALL_PATH}"

cp -f "${LINUX_AMD64_DAEMON}" "${TELEGRAM_DAEMON_PATH}"
chmod +x "${TELEGRAM_DAEMON_PATH}"
echo "✓ Installed ${TELEGRAM_DAEMON_PATH}"

if command -v python3 &>/dev/null; then
  python3 - "${TELEGRAM_MANIFEST}" "${VERSION}" "${TELEGRAM_DAEMON_PATH}" <<'PY'
import json
import os
import sys
from datetime import datetime, timezone

manifest_path, version, daemon_path = sys.argv[1:4]
manifest = {}
if os.path.exists(manifest_path):
    with open(manifest_path, encoding="utf-8") as f:
        manifest = json.load(f)

manifest.setdefault("name", "telegram")
manifest.setdefault("protocol_version", 1)
manifest["version"] = version
manifest["daemon_path"] = daemon_path
manifest["installed_at"] = datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")

with open(manifest_path, "w", encoding="utf-8") as f:
    json.dump(manifest, f, indent=2)
    f.write("\n")
PY
  echo "✓ Updated ${TELEGRAM_MANIFEST} (version=${VERSION})"
else
  echo "[WARN] python3 not found; skipped Telegram manifest update" >&2
fi

echo ""
echo "✓ Release artifacts in ${DIST}/"
ls -lh "${DIST}/"
echo ""
echo "✓ Checksums written to ${CHECKSUMS_FILE}"
cat "${CHECKSUMS_FILE}"
