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
#   dist/hero-telegram-daemon_<version>_linux_amd64
#   dist/hero-telegram-daemon_<version>_linux_arm64
#   dist/hero-telegram-daemon_<version>_darwin_amd64
#   dist/hero-telegram-daemon_<version>_darwin_arm64
#   dist/checksums.txt
#
# After build (linux/amd64 only), installs local dev copies:
#   /home/ricardo/installable/hero/hero
#   /home/ricardo/.workflow-hero/plugins/telegram/hero-telegram-daemon
# and updates /home/ricardo/.workflow-hero/plugins/telegram/manifest.json.
#
# Requirements: go, git, sha256sum (Linux) or shasum (macOS), python3 (manifest update)

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
DAEMON_MODULE_PATH="./cmd/hero-telegram-daemon"
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

  DAEMON_OUTPUT="${DIST}/hero-telegram-daemon_${VERSION}_${OS}_${ARCH}"
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
  (cd "${DIST}" && ${SHA_CMD} "hero_${VERSION}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
  (cd "${DIST}" && ${SHA_CMD} "hero-telegram-daemon_${VERSION}_${OS}_${ARCH}") >> "${CHECKSUMS_FILE}"
done

# Local install (linux/amd64 only).
HERO_INSTALL_PATH="/home/ricardo/installable/hero/hero"
LINUX_AMD64_HERO="${DIST}/hero_${VERSION}_linux_amd64"
LINUX_AMD64_DAEMON="${DIST}/hero-telegram-daemon_${VERSION}_linux_amd64"
TELEGRAM_PLUGIN_DIR="/home/ricardo/.workflow-hero/plugins/telegram"
TELEGRAM_DAEMON_PATH="${TELEGRAM_PLUGIN_DIR}/hero-telegram-daemon"
TELEGRAM_MANIFEST="${TELEGRAM_PLUGIN_DIR}/manifest.json"

echo ""
echo "→ Installing local dev binaries (linux/amd64)..."
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
echo "✓ Dev build artifacts in ${DIST}/"
ls -lh "${DIST}/"
echo ""
echo "✓ Checksums written to ${CHECKSUMS_FILE}"
cat "${CHECKSUMS_FILE}"
